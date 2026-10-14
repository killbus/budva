package push

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pure-golang/budva-claude/internal/service/push/messagev2"
)

func TestWebhookSink_PushCanonicalJSON(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int64
	var gotBody []byte
	var gotHeader http.Header
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		require.Equal(t, http.MethodPost, r.Method)
		gotHeader = r.Header.Clone()
		var err error
		gotBody, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	sink := NewWebhookSink(WebhookConfig{
		URL:     server.URL,
		Headers: map[string]string{"Authorization": "Bearer token"},
		Client:  server.Client(),
	})
	env := Envelope{
		SchemaVersion: SchemaVersion,
		Event:         EventEdited,
		ChatID:        -1001,
		MessageID:     42,
		OccurredAt:    time.Date(2026, time.October, 11, 12, 0, 0, 0, time.UTC),
		Text:          "full edited text",
	}

	err := sink.Push(context.Background(), RenderedEnvelope{Envelope: env})

	require.NoError(t, err)
	require.Equal(t, int64(1), attempts.Load())
	require.Equal(t, "application/json", gotHeader.Get("Content-Type"))
	require.Equal(t, "Bearer token", gotHeader.Get("Authorization"))
	var got Envelope
	require.NoError(t, json.Unmarshal(gotBody, &got))
	require.Equal(t, env, got)
}

func TestWebhookSink_PushSignsRawBody(t *testing.T) {
	t.Parallel()

	const secret = "signing-secret"
	var body []byte
	var signature string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		body, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		signature = r.Header.Get("X-Budva-Signature")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	sink := NewWebhookSink(WebhookConfig{
		URL:        server.URL,
		HMACSecret: secret,
		Client:     server.Client(),
	})

	require.NoError(t, sink.Push(context.Background(), RenderedEnvelope{Envelope: Envelope{
		SchemaVersion: SchemaVersion,
		Event:         EventNew,
		ChatID:        -1001,
		MessageID:     7,
		OccurredAt:    time.Unix(1, 0).UTC(),
		Text:          "message",
	}}))

	mac := hmac.New(sha256.New, []byte(secret))
	_, err := mac.Write(body)
	require.NoError(t, err)
	require.Equal(t, "sha256="+hex.EncodeToString(mac.Sum(nil)), signature)
}

func TestWebhookSink_PushClassifiesFailureWithoutRetry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		statusCode int
		wantClass  FailureClass
	}{
		{name: "http 4xx", statusCode: http.StatusBadRequest, wantClass: FailureHTTP4xx},
		{name: "http 5xx", statusCode: http.StatusServiceUnavailable, wantClass: FailureHTTP5xx},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var attempts atomic.Int64
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				attempts.Add(1)
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			sink := NewWebhookSink(WebhookConfig{URL: server.URL, Client: server.Client()})
			err := sink.Push(context.Background(), RenderedEnvelope{Envelope: testV1Envelope()})

			var deliveryErr *DeliveryError
			require.ErrorAs(t, err, &deliveryErr)
			require.Equal(t, tt.wantClass, deliveryErr.Class)
			require.Equal(t, int64(1), attempts.Load())
		})
	}
}

func TestWebhookSink_PushClassifiesRedirectLimit(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int64
	var server *httptest.Server
	server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		http.Redirect(w, r, server.URL+r.URL.Path+"x", http.StatusFound)
	}))
	defer server.Close()

	sink := NewWebhookSink(WebhookConfig{URL: server.URL, Client: server.Client()})
	err := sink.Push(context.Background(), RenderedEnvelope{Envelope: testV1Envelope()})

	var deliveryErr *DeliveryError
	require.ErrorAs(t, err, &deliveryErr)
	require.Equal(t, FailureRedirect, deliveryErr.Class)
	require.Equal(t, int64(4), attempts.Load())
}

func TestWebhookSink_PushClassifiesTimeoutAndTransport(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		err       error
		wantClass FailureClass
	}{
		{name: "timeout", err: timeoutError{}, wantClass: FailureTimeout},
		{name: "transport", err: errors.New("connection refused"), wantClass: FailureTLSTransport},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			transport := &errorTransport{err: tt.err}
			sink := NewWebhookSink(WebhookConfig{
				URL:    "https://push.example.test/events",
				Client: &http.Client{Transport: transport},
			})

			err := sink.Push(context.Background(), RenderedEnvelope{Envelope: testV1Envelope()})

			var deliveryErr *DeliveryError
			require.ErrorAs(t, err, &deliveryErr)
			require.Equal(t, tt.wantClass, deliveryErr.Class)
			require.Equal(t, int64(1), transport.attempts.Load())
		})
	}
}

func TestWebhookSink_PushContextCancellationIsTimeout(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	transport := &contextTransport{}
	sink := NewWebhookSink(WebhookConfig{
		URL:    "https://push.example.test/events",
		Client: &http.Client{Transport: transport},
	})

	err := sink.Push(ctx, RenderedEnvelope{Envelope: testV1Envelope()})

	var deliveryErr *DeliveryError
	require.ErrorAs(t, err, &deliveryErr)
	require.Equal(t, FailureTimeout, deliveryErr.Class)
	require.Equal(t, int64(1), transport.attempts.Load())
}

type errorTransport struct {
	err      error
	attempts atomic.Int64
}

func (t *errorTransport) RoundTrip(*http.Request) (*http.Response, error) {
	t.attempts.Add(1)
	return nil, t.err
}

type contextTransport struct {
	attempts atomic.Int64
}

func (t *contextTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.attempts.Add(1)
	return nil, req.Context().Err()
}

type timeoutError struct{}

func (timeoutError) Error() string   { return "timeout" }
func (timeoutError) Timeout() bool   { return true }
func (timeoutError) Temporary() bool { return true }

// RED 7: endpoints in one generation select their own schema; the default
// endpoint keeps the exact v1 wire and the v2 endpoint gets the v2
// notification only.
func TestWebhookSink_MixedSchemasRenderPerEndpoint(t *testing.T) {
	t.Parallel()

	occurredAt := time.Date(2026, time.October, 13, 8, 0, 0, 0, time.UTC)
	env := Envelope{
		SchemaVersion: SchemaVersion,
		Event:         EventNew,
		ChatID:        100,
		MessageID:     1,
		OccurredAt:    occurredAt,
		Text:          "full source text",
	}
	v2 := &messagev2.Notification{
		SchemaVersion: messagev2.SchemaVersion,
		Event:         messagev2.EventNew,
		OccurredAt:    occurredAt,
		Message: &messagev2.Message{
			ChatID:    100,
			MessageID: 1,
			SentAt:    &occurredAt,
			Content:   &messagev2.Content{Type: "messageText"},
		},
	}

	var v1s, v2s chan capturedTest
	mkSink := func(schema DeliverySchemaVersion, store chan capturedTest) *WebhookSink {
		transport := &schemaCaptureTransport{store: store}
		return NewWebhookSink(WebhookConfig{
			URL:    "https://push.example.test/events",
			Schema: schema,
			Client: &http.Client{Transport: transport},
		})
	}

	t.Run("v1 default", func(t *testing.T) {
		v1s = make(chan capturedTest, 1)
		err := mkSink(DeliverySchemaV1, v1s).Push(context.Background(), RenderedEnvelope{Envelope: env, V2: v2})
		require.NoError(t, err)
		got := <-v1s
		require.Equal(t, 1, got.schema)
		var decoded Envelope
		require.NoError(t, json.Unmarshal(got.body, &decoded))
		require.Equal(t, env, decoded)
	})

	t.Run("v2 explicit", func(t *testing.T) {
		v2s = make(chan capturedTest, 1)
		err := mkSink(DeliverySchemaV2, v2s).Push(context.Background(), RenderedEnvelope{Envelope: env, V2: v2})
		require.NoError(t, err)
		got := <-v2s
		require.Equal(t, 2, got.schema)
		require.NotContains(t, string(got.body), "full source text")
	})
}

type schemaCaptureTransport struct {
	store chan capturedTest
}

func (t *schemaCaptureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	var probe struct {
		SchemaVersion int `json:"schema_version"`
	}
	_ = json.Unmarshal(body, &probe)
	t.store <- capturedTest{schema: probe.SchemaVersion, body: body}
	resp := &http.Response{
		StatusCode: http.StatusAccepted,
		Status:     "202 Accepted",
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("")),
		Request:    req,
	}
	return resp, nil
}

// RED 8: an unprojectable or invalid v2 fact fails only the v2-selected
// endpoint with the serialization class; no v1, raw-TDLib or text-only body
// is ever sent, and the v1 sibling is unaffected.
func TestWebhookSink_V2FailureIsEndpointIsolated(t *testing.T) {
	t.Parallel()

	occurredAt := time.Date(2026, time.October, 13, 8, 0, 0, 0, time.UTC)
	env := Envelope{
		SchemaVersion: SchemaVersion,
		Event:         EventNew,
		ChatID:        100,
		MessageID:     1,
		OccurredAt:    occurredAt,
		Text:          "full source text",
	}

	attempts := make(chan []byte, 4)
	transport := &bodyCaptureTransport{attempts: attempts}

	cases := map[string]RenderedEnvelope{
		"projection unavailable": {Envelope: env, V2: nil},
		"invalid v2 fact": {
			Envelope: env,
			V2: &messagev2.Notification{
				SchemaVersion: messagev2.SchemaVersion,
				Event:         messagev2.EventNew,
				OccurredAt:    occurredAt,
				Message:       nil,
			},
		},
	}

	for name, rend := range cases {
		t.Run(name, func(t *testing.T) {
			sink := NewWebhookSink(WebhookConfig{
				URL:    "https://push.example.test/events",
				Schema: DeliverySchemaV2,
				Client: &http.Client{Transport: transport},
			})
			err := sink.Push(context.Background(), rend)

			var deliveryErr *DeliveryError
			require.ErrorAs(t, err, &deliveryErr)
			require.Equal(t, FailureSerialization, deliveryErr.Class)

			select {
			case body := <-attempts:
				t.Fatalf("v2 failure must not send any body, sent %s", body)
			default:
			}
		})
	}

	t.Run("v1 sibling unaffected", func(t *testing.T) {
		sink := NewWebhookSink(WebhookConfig{
			URL:    "https://push.example.test/events",
			Schema: DeliverySchemaV1,
			Client: &http.Client{Transport: &bodyCaptureTransport{attempts: attempts}},
		})
		require.NoError(t, sink.Push(context.Background(), RenderedEnvelope{Envelope: env, V2: nil}))
		body := <-attempts
		var decoded Envelope
		require.NoError(t, json.Unmarshal(body, &decoded))
		require.Equal(t, env, decoded)
	})
}

// testV1Envelope is a minimal valid v1 source fact. A v1 envelope always
// carries a real event kind; the empty-event envelope is now reserved as the
// v2-only carrier sentinel, so throwaway v1 fixtures use an explicit event.
func testV1Envelope() Envelope {
	return Envelope{SchemaVersion: SchemaVersion, Event: EventNew, ChatID: 100, MessageID: 1}
}

// textlessV2 is a valid captionless-photo v2 fact: the canonical content
// type is present but there is no text, exactly the webhook-only carrier the
// source handler submits via SubmitV2Only.
func textlessV2() *messagev2.Notification {
	occurredAt := time.Date(2026, time.October, 13, 8, 0, 0, 0, time.UTC)
	return &messagev2.Notification{
		SchemaVersion: messagev2.SchemaVersion,
		Event:         messagev2.EventNew,
		OccurredAt:    occurredAt,
		Message: &messagev2.Message{
			ChatID:    100,
			MessageID: 1,
			SentAt:    &occurredAt,
			Content:   &messagev2.Content{Type: "messagePhoto"},
		},
	}
}

// RED: a v1-selected endpoint given a v2-only carrier (no v1 representation)
// must fail its submission with the serialization class and send zero HTTP
// attempts, so the endpoint stays silent instead of emitting an empty or
// text-only envelope. Silence is an explicit carrier signal (empty event),
// never inferred from empty text: a legitimate v1 delete with empty text
// still renders.
func TestWebhookSink_V1SilentForV2OnlyCarrier(t *testing.T) {
	t.Parallel()

	// A v1 endpoint receiving the textless v2-only carrier is silent.
	v1attempts := make(chan []byte, 1)
	v1sink := NewWebhookSink(WebhookConfig{
		URL:    "https://push.example.test/events",
		Schema: DeliverySchemaV1,
		Client: &http.Client{Transport: &bodyCaptureTransport{attempts: v1attempts}},
	})
	err := v1sink.Push(context.Background(), RenderedEnvelope{V2: textlessV2()})
	var deliveryErr *DeliveryError
	require.ErrorAs(t, err, &deliveryErr)
	require.Equal(t, FailureSerialization, deliveryErr.Class)
	require.ErrorIs(t, deliveryErr.Err, ErrV1Unavailable)
	select {
	case body := <-v1attempts:
		t.Fatalf("silent v1 endpoint must send no body, sent %s", body)
	default:
	}

	// The same fact rendered by a v2 endpoint is delivered as the v2 wire.
	v2attempts := make(chan []byte, 1)
	v2sink := NewWebhookSink(WebhookConfig{
		URL:    "https://push.example.test/events",
		Schema: DeliverySchemaV2,
		Client: &http.Client{Transport: &bodyCaptureTransport{attempts: v2attempts}},
	})
	require.NoError(t, v2sink.Push(context.Background(), RenderedEnvelope{V2: textlessV2()}))
	var probe struct {
		SchemaVersion int `json:"schema_version"`
		Event         string
		Message       map[string]any `json:"message"`
	}
	require.NoError(t, json.Unmarshal(<-v2attempts, &probe))
	require.Equal(t, 2, probe.SchemaVersion)
	require.Equal(t, "new", probe.Event)
	require.Equal(t, "messagePhoto", probe.Message["content"].(map[string]any)["type"])
}

// RED: v1 silence is keyed on the absent v1 representation (empty event), not
// on empty text. A legitimate v1 delete has empty text and must still render.
func TestRenderPayload_V1SilenceIsExplicit(t *testing.T) {
	t.Parallel()

	occurredAt := time.Date(2026, time.October, 13, 8, 0, 0, 0, time.UTC)

	// v2-only carrier: no v1 representation, so the v1 render fails.
	_, err := RenderPayload(RenderedEnvelope{V2: textlessV2()}, DeliverySchemaV1)
	require.ErrorIs(t, err, ErrV1Unavailable)

	// A legitimate v1 delete carries an empty Text but a non-empty event, so
	// it must still render the default wire.
	deleted := Envelope{
		SchemaVersion: SchemaVersion,
		Event:         EventDeleted,
		ChatID:        100,
		MessageID:     1,
		OccurredAt:    occurredAt,
		Text:          "",
	}
	body, err := RenderPayload(RenderedEnvelope{Envelope: deleted}, DeliverySchemaV1)
	require.NoError(t, err)
	var decoded Envelope
	require.NoError(t, json.Unmarshal(body, &decoded))
	require.Equal(t, deleted, decoded)
}

// capturedTest is one captured delivery attempt for schema-selection tests.
type capturedTest struct {
	schema int
	body   []byte
}

type bodyCaptureTransport struct {
	attempts chan []byte
}

func (t *bodyCaptureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	t.attempts <- body
	resp := &http.Response{
		StatusCode: http.StatusAccepted,
		Status:     "202 Accepted",
		Proto:      "HTTP/1.1",
		ProtoMajor: 1,
		ProtoMinor: 1,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("")),
		Request:    req,
	}
	return resp, nil
}
