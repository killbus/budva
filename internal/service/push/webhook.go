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
	"time"
)

const defaultWebhookTimeout = 5 * time.Second

var errRedirectLimit = errors.New("webhook redirect policy rejected request")

// FailureClass identifies the terminal outcome of one webhook delivery attempt.
type FailureClass string

const (
	FailureHTTP4xx      FailureClass = "http_4xx"
	FailureHTTP5xx      FailureClass = "http_5xx"
	FailureRedirect     FailureClass = "redirect"
	FailureTimeout      FailureClass = "timeout"
	FailureTLSTransport FailureClass = "tls_transport"
)

// DeliveryError describes a failed webhook delivery attempt.
type DeliveryError struct {
	Class      FailureClass
	StatusCode int
	Err        error
}

func (e *DeliveryError) Error() string {
	if e.StatusCode != 0 {
		return fmt.Sprintf("webhook delivery failed: class=%s status=%d", e.Class, e.StatusCode)
	}
	return fmt.Sprintf("webhook delivery failed: class=%s", e.Class)
}

func (e *DeliveryError) Unwrap() error {
	return e.Err
}

// WebhookConfig configures a canonical JSON webhook sink.
type WebhookConfig struct {
	URL        string
	Headers    map[string]string
	HMACSecret string
	Client     *http.Client
}

// WebhookSink performs one bounded HTTP delivery attempt per Push call.
type WebhookSink struct {
	url        string
	headers    map[string]string
	hmacSecret string
	client     *http.Client
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

	return &WebhookSink{
		url:        config.URL,
		headers:    headers,
		hmacSecret: config.HMACSecret,
		client:     &clientCopy,
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

// Push posts the canonical envelope once. Redirect following is part of the
// same bounded delivery flow and internal retries are deliberately absent.
func (s *WebhookSink) Push(ctx context.Context, env Envelope) error {
	parsed, err := url.ParseRequestURI(s.url)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return &DeliveryError{Class: FailureTLSTransport}
	}
	body, err := json.Marshal(env)
	if err != nil {
		return &DeliveryError{Class: FailureTLSTransport, Err: err}
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
