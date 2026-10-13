package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPushTarget_EffectiveSchemaVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  int
		want    int
		wantErr bool
	}{
		{name: "absent selects v1", config: 0, want: 1},
		{name: "explicit v1", config: 1, want: 1},
		{name: "explicit v2", config: 2, want: 2},
		{name: "negative rejected", config: -1, wantErr: true},
		{name: "future version rejected", config: 3, wantErr: true},
		{name: "large version rejected", config: 99, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			target := &PushTarget{URL: "https://example.test", SchemaVersion: tt.config}
			got, err := target.EffectiveSchemaVersion()
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
