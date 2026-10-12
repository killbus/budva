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
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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

	err := sink.Push(context.Background(), env)

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

	require.NoError(t, sink.Push(context.Background(), Envelope{
		SchemaVersion: SchemaVersion,
		Event:         EventNew,
		ChatID:        -1001,
		MessageID:     7,
		OccurredAt:    time.Unix(1, 0).UTC(),
		Text:          "message",
	}))

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
			err := sink.Push(context.Background(), Envelope{})

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
	err := sink.Push(context.Background(), Envelope{})

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

			err := sink.Push(context.Background(), Envelope{})

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

	err := sink.Push(ctx, Envelope{})

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
