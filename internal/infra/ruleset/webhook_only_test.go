package ruleset

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pure-golang/budva-claude/internal/config"
	"github.com/pure-golang/budva-claude/internal/domain"
)

const webhookOnlyRuleset = `
sources:
  1001000: {}
pushTargets:
  external:
    url: https://push.example.test/events
forwardRules:
  rule1:
    from: 1001000
    pushTo: [external]
`

func writeWebhookOnlyRuleset(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "ruleset.yml")
	err := os.WriteFile(path, []byte(webhookOnlyRuleset), 0o600)
	require.NoError(t, err)
	return path
}

// TestRepo_Load_WebhookOnlyRule pins that a webhook-only ruleset loads:
// the rule has pushTo and no to, the file must load, and the unique
// source / rule / targets are preserved. An empty to produces no unique
// destination but is not an ErrEmptyConfig.
func TestRepo_Load_WebhookOnlyRule(t *testing.T) {
	t.Parallel()

	r := New(config.RulesetConfig{Path: writeWebhookOnlyRuleset(t)})

	rs, err := r.Load()
	require.NoError(t, err)

	require.Contains(t, rs.UniqueSources, domain.ChatID(-1001000))
	require.Empty(t, rs.UniqueDestinations)
	require.Equal(t, []domain.ForwardRuleID{"rule1"}, rs.OrderedForwardRules)
	require.Equal(t, []string{"external"}, rs.ForwardRules["rule1"].PushTo)
	require.Empty(t, rs.ForwardRules["rule1"].To)
}

// TestRepo_Load_WebhookOnlyRuleHotReload pins the hot-reload path: Load()
// inside the fsnotify callback succeeds for a webhook-only file, so the
// engine reload branch goes through loadSnapshot instead of reload_failed.
func TestRepo_Load_WebhookOnlyRuleHotReload(t *testing.T) {
	t.Parallel()

	path := writeWebhookOnlyRuleset(t)

	r := New(config.RulesetConfig{Path: path})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	loadErr := make(chan error, 1)
	called := make(chan struct{}, 1)
	err := r.WatchContext(ctx, func() {
		select {
		case called <- struct{}{}:
		default:
		}
		_, lerr := r.Load()
		loadErr <- lerr
	})
	require.NoError(t, err)
	t.Cleanup(func() { r.Close() })

	err = os.WriteFile(path, []byte(webhookOnlyRuleset+"\n"), 0o600)
	require.NoError(t, err)

	select {
	case <-called:
	case <-time.After(5 * time.Second):
		t.Fatal("watcher did not fire on in-place write")
	}
	require.NoError(t, <-loadErr)
}
