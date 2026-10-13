package ruleset

import (
	"testing"

	"github.com/pure-golang/budva-claude/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestValidate_PushTargetSchemaVersion(t *testing.T) {
	t.Parallel()

	t.Run("rejection", func(t *testing.T) {
		t.Parallel()
		// RED: without schemaVersion validation these invalid candidates
		// would load successfully.
		for _, tc := range []struct {
			name   string
			config int
		}{
			{name: "negative", config: -1},
			{name: "future version", config: 3},
			{name: "large version", config: 20},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				rs := &domain.RuleSet{PushTargets: map[string]*domain.PushTarget{"endpoint": {URL: "https://example.test", SchemaVersion: tc.config}}}
				err := validate(rs)
				require.Error(t, err)
				require.Contains(t, err.Error(), "schemaVersion")
			})
		}
	})

	t.Run("acceptance", func(t *testing.T) {
		t.Parallel()
		// Absent and explicit v1 remain valid; explicit v2 is the only
		// opt-in into Message-only v2.
		for _, tc := range []struct {
			name   string
			config int
		}{
			{name: "absent", config: 0},
			{name: "explicit v1", config: 1},
			{name: "explicit v2", config: 2},
		} {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()
				rs := &domain.RuleSet{PushTargets: map[string]*domain.PushTarget{"endpoint": {URL: "https://example.test", SchemaVersion: tc.config}}}
				require.NoError(t, validate(rs))
			})
		}
	})
}

func TestValidate_SchemaVersionErrorNamesTarget(t *testing.T) {
	t.Parallel()

	rs := &domain.RuleSet{PushTargets: map[string]*domain.PushTarget{
		"archive": {URL: "https://example.test", SchemaVersion: 2},
		"bad":     {URL: "https://example.test", SchemaVersion: 7},
	}}
	err := validate(rs)
	require.Error(t, err)
	require.Contains(t, err.Error(), `"bad"`)
	require.NotContains(t, err.Error(), `"archive"`)
}
