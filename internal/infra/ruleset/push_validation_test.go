package ruleset

import (
	"testing"

	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestPushTarget_RejectsInvalidTimeoutAndHeaders(t *testing.T) {
	// RED: removing timeout/header validation allows these invalid candidates.
	for _, tc := range []struct {
		name    string
		timeout int
		headers map[string]string
	}{
		{name: "negative timeout", timeout: -1},
		{name: "overflow timeout", timeout: 9223372037},
		{name: "empty header", headers: map[string]string{"": "value"}},
		{name: "invalid name", headers: map[string]string{"bad name": "value"}},
		{name: "newline value", headers: map[string]string{"Authorization": "token\r\ninjected: yes"}},
		{name: "NUL value", headers: map[string]string{"X-Test": "\x00"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rs := &domain.RuleSet{PushTargets: map[string]*domain.PushTarget{"endpoint": {URL: "https://example.test", TimeoutSeconds: tc.timeout, Headers: tc.headers}}}
			require.Error(t, validate(rs))
		})
	}
}
