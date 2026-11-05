package telegram

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zelenin/go-tdlib/client"
)

func TestFloodWaitFromError(t *testing.T) {
	t.Parallel()

	t.Run("tdlib 429 becomes typed flood wait", func(t *testing.T) {
		t.Parallel()

		// Act
		got := FloodWaitFromError(client.ResponseError{Err: &client.Error{
			Code:    429,
			Message: "Too Many Requests: retry after 7",
		}})

		// Assert
		var floodWait *FloodWaitError
		require.ErrorAs(t, got, &floodWait)
		assert.Equal(t, 7*time.Second+500*time.Millisecond, floodWait.Delay)
	})

	t.Run("wrapped flood text becomes typed flood wait", func(t *testing.T) {
		t.Parallel()

		// Act
		got := FloodWaitFromError(errors.New("get user: 429 Too Many Requests: retry after 7"))

		// Assert
		var floodWait *FloodWaitError
		require.ErrorAs(t, got, &floodWait)
		assert.Equal(t, 7*time.Second+500*time.Millisecond, floodWait.Delay)
	})

	t.Run("non flood error passes through", func(t *testing.T) {
		t.Parallel()

		// Arrange
		wantErr := errors.New("not a flood")

		// Act
		got := FloodWaitFromError(wantErr)

		// Assert
		assert.ErrorIs(t, got, wantErr)
		var floodWait *FloodWaitError
		assert.NotErrorAs(t, got, &floodWait)
	})
}
