package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestIsChannelOrSupergroup checks runtime TDLib IDs after ruleset loading.
func TestIsChannelOrSupergroup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		id   ChatID
		want bool
	}{
		// Канал (сохранённый ключ, отрицательный, модуль > 1e12). Фикстура
		// 1002641439846 = 1e12 + 2641439846 (ruleset/repo_test.go).
		{name: "stored_channel_key_opens", id: -1002641439846, want: true},
		{name: "just_above_boundary_opens", id: -(channelIDMagnitude + 1), want: true},

		// Граничные: м==1e12 включён в «не канал» (<=), м==1e12+1 → канал.
		{name: "boundary_equals_magnitude_not_channel", id: -channelIDMagnitude, want: false},
		{name: "one_past_boundary_is_channel", id: -(channelIDMagnitude + 1), want: true},

		// Базовая группа (модуль <= 1e12, отрицательный ключ) → не открывать.
		{name: "basic_group_not_opened", id: -999999999, want: false},
		{name: "small_basic_group_not_opened", id: -12345, want: false},

		// RED: magnitude-only classification mistakes large private IDs for channels.
		{name: "large_private_id_not_opened", id: 1002641439846, want: false},
		{name: "private_chat_not_opened", id: 12345, want: false},
		{name: "bot_private_chat_not_opened", id: 7131987336, want: false},

		{name: "zero_not_channel", id: 0, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, IsChannelOrSupergroup(tt.id))
		})
	}
}
