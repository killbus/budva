package push

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// RED: HTTPS redirects previously allowed sending private content over HTTP.
func TestWebhookSink_RejectsPlaintextAndDowngrade(t *testing.T) {
	for _, initial := range []string{"http://push.test/secret", "https://push.test/secret"} {
		t.Run(initial, func(t *testing.T) {
			transport := &redirectTransport{}
			sink := NewWebhookSink(WebhookConfig{URL: initial, Client: &http.Client{Transport: transport}})
			err := sink.Push(context.Background(), RenderedEnvelope{Envelope: testV1Envelope()})
			require.Error(t, err)
			require.NotContains(t, err.Error(), "secret")
			want := 0
			if strings.HasPrefix(initial, "https:") {
				want = 1
			}
			require.Equal(t, want, transport.attempts)
		})
	}
}

// RED: static headers could previously overwrite the canonical content type.
func TestWebhookSink_CanonicalHeadersAndNoSecretError(t *testing.T) {
	transport := &inspectTransport{}
	sink := NewWebhookSink(WebhookConfig{
		URL: "https://push.test/bearer-secret", Headers: map[string]string{"Content-Type": "text/plain"},
		Client: &http.Client{Transport: transport, Timeout: -1},
	})
	err := sink.Push(context.Background(), RenderedEnvelope{Envelope: testV1Envelope()})
	require.Error(t, err)
	require.Equal(t, "application/json", transport.contentType)
	require.NotContains(t, err.Error(), "bearer-secret")
	require.Equal(t, defaultWebhookTimeout, sink.client.Timeout)
}

// RED: replayable POST bodies let net/http retry when an idempotency header is configured.
func TestWebhookSink_TransportCannotReplayRequest(t *testing.T) {
	transport := &replayInspectTransport{}
	sink := NewWebhookSink(WebhookConfig{
		URL: "https://push.test/events", Headers: map[string]string{"Idempotency-Key": "external-key"},
		Client: &http.Client{Transport: transport},
	})
	require.NoError(t, sink.Push(context.Background(), RenderedEnvelope{Envelope: testV1Envelope()}))
	require.False(t, transport.replayable)
}

type replayInspectTransport struct{ replayable bool }

func (t *replayInspectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.replayable = req.GetBody != nil
	return &http.Response{StatusCode: http.StatusNoContent, Header: make(http.Header), Body: http.NoBody, Request: req}, nil
}

// RED: clearing GetBody on the client's original request prevents 307 redirects.
func TestWebhookSink_PreservesBodyAcrossHTTPSRedirect(t *testing.T) {
	var body string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/" {
			http.Redirect(w, req, "/final", http.StatusTemporaryRedirect)
			return
		}
		data, _ := io.ReadAll(req.Body)
		body = string(data)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	sink := NewWebhookSink(WebhookConfig{URL: server.URL, Client: server.Client()})
	require.NoError(t, sink.Push(context.Background(), RenderedEnvelope{Envelope: Envelope{Event: EventNew, Text: "full source"}}))
	require.Contains(t, body, `"text":"full source"`)
}

type redirectTransport struct{ attempts int }

func (t *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.attempts++
	return &http.Response{StatusCode: 307, Header: http.Header{"Location": {"http://push.test/secret"}}, Body: io.NopCloser(strings.NewReader("")), Request: req}, nil
}

type inspectTransport struct{ contentType string }

func (t *inspectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.contentType = req.Header.Get("Content-Type")
	return nil, errors.New("transport failed for bearer-secret")
}
