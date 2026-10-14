package push

import (
	"errors"
	"fmt"
	"time"
)

// defaultWebhookTimeout bounds one delivery attempt when no explicit timeout
// is configured.
const defaultWebhookTimeout = 5 * time.Second

var errRedirectLimit = errors.New("webhook redirect policy rejected request")

// FailureClass identifies the terminal outcome of one webhook delivery attempt.
type FailureClass string

const (
	FailureHTTP4xx       FailureClass = "http_4xx"
	FailureHTTP5xx       FailureClass = "http_5xx"
	FailureRedirect      FailureClass = "redirect"
	FailureTimeout       FailureClass = "timeout"
	FailureTLSTransport  FailureClass = "tls_transport"
	FailureSerialization FailureClass = "serialization"
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
