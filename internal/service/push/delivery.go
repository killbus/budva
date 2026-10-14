package push

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"

	"github.com/pure-golang/budva-claude/internal/service/push/event"
	"github.com/pure-golang/budva-claude/internal/service/push/messagev2"
)

// DeliverySchemaVersion identifies the schema of one payload rendered at the
// delivery boundary.
type DeliverySchemaVersion int

const (
	// DeliverySchemaV1 is the canonical six-key envelope, the default.
	DeliverySchemaV1 DeliverySchemaVersion = 1
	// DeliverySchemaV2 is the Message-only v2 notification.
	DeliverySchemaV2 DeliverySchemaVersion = 2
)

// ErrV2Unavailable means the source projection could not form a valid v2
// notification; a v2-selected endpoint submission fails explicitly instead of
// falling back to v1, raw source data or text-only JSON.
var ErrV2Unavailable = errors.New("push: v2 projection unavailable")

// ErrV1Unavailable means the carrier has no v1 representation; a v1-selected
// endpoint submission fails explicitly instead of emitting an empty or
// text-only envelope for a source fact that only the v2 schema carries.
var ErrV1Unavailable = errors.New("push: v1 representation unavailable")

// RenderPayload renders the source facts for one endpoint-selected schema.
// It never falls back to another schema: an unprojectable v2 fact fails with
// ErrV2Unavailable, a missing v1 fact fails with ErrV1Unavailable, and
// unsupported versions are rejected.
func RenderPayload(rend event.RenderedEnvelope, schema DeliverySchemaVersion) ([]byte, error) {
	switch schema {
	case DeliverySchemaV1:
		// A legitimate v1 fact always carries its event kind (new, edited or
		// deleted). An empty event marks a v2-only carrier: the fact has no v1
		// representation, so the v1 endpoint stays silent rather than
		// marshalling an empty-text envelope.
		if rend.Envelope.Event == "" {
			return nil, ErrV1Unavailable
		}
		return json.Marshal(rend.Envelope)
	case DeliverySchemaV2:
		if rend.V2 == nil {
			return nil, ErrV2Unavailable
		}
		return messagev2.MarshalNotification(rend.V2)
	default:
		return nil, fmt.Errorf("push: unsupported delivery schema version %d", int(schema))
	}
}

// WebhookConfig configures a canonical JSON webhook sink.
type WebhookConfig struct {
	URL        string
	Headers    map[string]string
	HMACSecret string
	Client     *http.Client
	// Schema selects the payload rendered per delivery. Zero keeps v1.
	Schema DeliverySchemaVersion
}

// WebhookSink performs one bounded HTTP delivery attempt per Push call.
type WebhookSink struct {
	url        string
	headers    map[string]string
	hmacSecret string
	client     *http.Client
	schema     DeliverySchemaVersion
}

// NewWebhookSink creates a sink without mutating the caller's HTTP client.
func NewWebhookSink(config WebhookConfig) *WebhookSink {
	client := http.DefaultClient
	if config.Client != nil {
		client = config.Client
	}
	clientCopy := *client
	if clientCopy.Timeout <= 0 {
		clientCopy.Timeout = defaultWebhookTimeout
	}
	transport := clientCopy.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	clientCopy.Transport = singleAttemptTransport{base: transport}
	previousRedirectPolicy := clientCopy.CheckRedirect
	clientCopy.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > 3 || req.URL.Scheme != "https" {
			return errRedirectLimit
		}
		if previousRedirectPolicy != nil {
			return previousRedirectPolicy(req, via)
		}
		return nil
	}

	headers := make(map[string]string, len(config.Headers))
	for name, value := range config.Headers {
		headers[name] = value
	}

	schema := config.Schema
	if schema == 0 {
		schema = DeliverySchemaV1
	}

	return &WebhookSink{
		url:        config.URL,
		headers:    headers,
		hmacSecret: config.HMACSecret,
		client:     &clientCopy,
		schema:     schema,
	}
}

type singleAttemptTransport struct{ base http.RoundTripper }

func (t singleAttemptTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// The client retains GetBody for bounded 307/308 redirects, but Transport
	// must not treat a POST with a static idempotency header as retryable.
	attempt := req.Clone(req.Context())
	attempt.GetBody = nil
	return t.base.RoundTrip(attempt)
}

// Push posts the rendered endpoint payload once. Redirect following is part of
// the same bounded delivery flow and internal retries are deliberately absent.
func (s *WebhookSink) Push(ctx context.Context, rend event.RenderedEnvelope) error {
	parsed, err := url.ParseRequestURI(s.url)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return &DeliveryError{Class: FailureTLSTransport}
	}
	body, err := RenderPayload(rend, s.schema)
	if err != nil {
		return &DeliveryError{Class: FailureSerialization, Err: err}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.url, bytes.NewReader(body))
	if err != nil {
		return &DeliveryError{Class: FailureTLSTransport, Err: err}
	}
	for name, value := range s.headers {
		req.Header.Set(name, value)
	}
	req.Header.Set("Content-Type", "application/json")
	if s.hmacSecret != "" {
		mac := hmac.New(sha256.New, []byte(s.hmacSecret))
		_, _ = mac.Write(body)
		req.Header.Set("X-Budva-Signature", "sha256="+hex.EncodeToString(mac.Sum(nil)))
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return classifyDeliveryError(ctx, err)
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		return classifyDeliveryError(ctx, err)
	}

	switch {
	case resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices:
		return nil
	case resp.StatusCode >= http.StatusBadRequest && resp.StatusCode < http.StatusInternalServerError:
		return &DeliveryError{Class: FailureHTTP4xx, StatusCode: resp.StatusCode}
	case resp.StatusCode >= http.StatusMultipleChoices && resp.StatusCode < http.StatusBadRequest:
		return &DeliveryError{Class: FailureRedirect, StatusCode: resp.StatusCode}
	default:
		return &DeliveryError{Class: FailureHTTP5xx, StatusCode: resp.StatusCode}
	}
}

func classifyDeliveryError(ctx context.Context, err error) error {
	if errors.Is(err, errRedirectLimit) {
		return &DeliveryError{Class: FailureRedirect, Err: err}
	}
	if ctx.Err() != nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return &DeliveryError{Class: FailureTimeout, Err: err}
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return &DeliveryError{Class: FailureTimeout, Err: err}
	}
	return &DeliveryError{Class: FailureTLSTransport, Err: err}
}
