package ruleset

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pure-golang/budva-claude/internal/config"
)

// RED: unconditional negation rejects signed group IDs and loses positive bot IDs.
func TestRepo_Load_TDLibChatIDs(t *testing.T) {
	t.Parallel()
	path := writeRuleset(t, `
chatIdFormat: tdlib
sources:
  7131987336:
    translate: {lang: en, for: [12345, -12345, -1002641439846]}
    sign: {for: [12345, -12345, -1002641439846]}
    link: {for: [12345, -12345, -1002641439846]}
    prev: {for: [12345, -12345, -1002641439846]}
    next: {for: [12345, -12345, -1002641439846]}
destinations:
  12345: {}
  -12345: {}
  -1002641439846: {}
forwardRules:
  bot:
    from: 7131987336
    to: [12345, -12345, -1002641439846]
    check: 54321
    other: -54321
  channel:
    from: -1002641439846
    to: [7131987336]
  group:
    from: -12345
    to: [12345]
`)
	rs, err := New(config.RulesetConfig{Path: path}).Load()
	require.NoError(t, err)
	rule := rs.ForwardRules["bot"]
	require.Equal(t, int64(7131987336), rule.From)
	wantDestinations := []int64{12345, -12345, -1002641439846}
	require.Equal(t, wantDestinations, rule.To)
	require.Equal(t, int64(54321), rule.Check)
	require.Equal(t, int64(-54321), rule.Other)
	require.Zero(t, rs.ForwardRules["channel"].Check)
	require.Zero(t, rs.ForwardRules["channel"].Other)
	for _, id := range []int64{7131987336, -12345, -1002641439846} {
		require.Contains(t, rs.UniqueSources, id)
		require.Equal(t, id, rs.Sources[id].ChatID)
	}
	for _, id := range wantDestinations {
		require.Contains(t, rs.UniqueDestinations, id)
		require.Equal(t, id, rs.Destinations[id].ChatID)
	}
	src := rs.Sources[7131987336]
	for _, ids := range [][]int64{src.Translate.For, src.Sign.For, src.Link.For, src.Prev.For, src.Next.For} {
		require.Equal(t, wantDestinations, ids)
	}
}

func TestRepo_Load_ChatIDFormatValidation(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, format, wantError string
		wantID                  int64
	}{
		{name: "default legacy", wantID: -1001000},
		{name: "explicit legacy", format: "legacy", wantID: -1001000},
		{name: "native private", format: "tdlib", wantID: 1001000},
		{name: "unknown format", format: "tdlb", wantError: "chatIdFormat"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := writeRuleset(t, "chatIdFormat: "+tt.format+"\n"+webhookOnlyRuleset)
			rs, err := New(config.RulesetConfig{Path: path}).Load()
			if tt.wantError != "" {
				require.ErrorContains(t, err, tt.wantError)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantID, rs.ForwardRules["rule1"].From)
			require.Contains(t, rs.UniqueSources, tt.wantID)
		})
	}
}

// A native private chat and basic group may share a magnitude, but not an ID.
func TestRepo_Load_TDLibFromEqualsTo(t *testing.T) {
	t.Parallel()
	for _, id := range []string{"7131987336", "-1002641439846"} {
		t.Run(id, func(t *testing.T) {
			t.Parallel()
			path := writeRuleset(t, "chatIdFormat: tdlib\nforwardRules:\n  loop:\n    from: "+id+"\n    to: ["+id+"]\n")
			_, err := New(config.RulesetConfig{Path: path}).Load()
			require.ErrorContains(t, err, "must differ from From")
		})
	}
}

func TestRepo_Load_ChatIDFormatReload(t *testing.T) {
	t.Parallel()
	path := writeRuleset(t, webhookOnlyRuleset)
	r := New(config.RulesetConfig{Path: path})
	legacy, err := r.Load()
	require.NoError(t, err)
	appendRulesetContent(t, path, "\nchatIdFormat: tdlib\n")
	native, err := r.Load()
	require.NoError(t, err)
	require.Contains(t, legacy.UniqueSources, int64(-1001000))
	require.NotContains(t, native.UniqueSources, int64(-1001000))
	require.Contains(t, native.UniqueSources, int64(1001000))

	// Explicit legacy mode retains every existing transformation.
	explicit := New(config.RulesetConfig{Path: writeRuleset(t, "chatIdFormat: legacy\n"+testRuleset)})
	rs, err := explicit.Load()
	require.NoError(t, err)
	require.Equal(t, int64(-1002641439846), rs.ForwardRules["CopyRule"].From)
	require.Equal(t, []int64{-1002667730628}, rs.Sources[-1002641439846].Sign.For)
}
