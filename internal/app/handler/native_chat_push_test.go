package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/app/handler/mocks"
	"github.com/pure-golang/budva-claude/internal/config"
	"github.com/pure-golang/budva-claude/internal/infra/queue"
	"github.com/pure-golang/budva-claude/internal/infra/ruleset"
	"github.com/pure-golang/budva-claude/internal/service/dedup"
	"github.com/pure-golang/budva-claude/internal/service/filters"
	"github.com/pure-golang/budva-claude/internal/service/message"
	"github.com/pure-golang/budva-claude/internal/service/push"
	"github.com/pure-golang/budva-claude/internal/service/push/event"
)

// RED: load real YAML before routing; negating a bot ID silently drops its
// updates. Both schemas must deliver the lifecycle with the original chat ID.
func TestPush_LoadedChatIDs(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, format       string
		configured, actual int64
	}{
		{"bot private", "tdlib", 7131987336, 7131987336},
		{"basic group", "tdlib", -12345, -12345},
		{"channel", "tdlib", -1002641439846, -1002641439846},
		{"legacy group", "", 12345, -12345},
		{"legacy channel", "", 1002641439846, -1002641439846},
	} {
		for _, schema := range []int{1, 2} {
			t.Run(fmt.Sprintf("%s/v%d", tt.name, schema), func(t *testing.T) {
				t.Parallel()
				var mu sync.Mutex
				var bodies [][]byte
				server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Errorf("read webhook: %v", err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					mu.Lock()
					bodies = append(bodies, body)
					mu.Unlock()
					w.WriteHeader(http.StatusAccepted)
				}))
				defer server.Close()
				path := filepath.Join(t.TempDir(), "ruleset.yml")
				require.NoError(t, os.WriteFile(path, []byte(fmt.Sprintf(`
chatIdFormat: %s
sources:
  %d: {}
pushTargets:
  endpoint:
    url: %s
    schemaVersion: %d
forwardRules:
  source:
    from: %d
    pushTo: [endpoint]
`, tt.format, tt.configured, server.URL, schema, tt.configured)), 0o600))
				rs, err := ruleset.New(config.RulesetConfig{Path: path}).Load()
				require.NoError(t, err)
				target := rs.PushTargets["endpoint"]
				g := push.NewGeneration(map[string]push.EndpointConfig{
					"endpoint": {Sink: push.NewWebhookSink(push.WebhookConfig{
						URL: target.URL, Client: server.Client(), Schema: push.DeliverySchemaVersion(target.SchemaVersion),
					}), BufferCapacity: 8, Timeout: 5 * time.Second},
				})
				defer g.Retire()
				state, q := mocks.NewStateRepo(t), queue.New()
				h := New(mocks.NewTelegramRepo(t), state, message.New(), filters.New(),
					mocks.NewTransformService(t), mocks.NewAlbumService(t), q, mocks.NewRateLimiter(t),
					func(ids []int64) DedupTracker { return dedup.NewTracker(ids) })
				h.SetRuleSetWithPush(rs, g)
				msg := &client.Message{ChatId: tt.actual, Id: 1, Date: 1791868800, CanBeSaved: true,
					SenderId: &client.MessageSenderUser{UserId: 7131987336},
					Content:  &client.MessageText{Text: &client.FormattedText{Text: "bot reply"}}}
				state.EXPECT().GetCopiedMessageIDs(tt.actual, int64(1)).Return(nil).Twice()
				ctx := context.Background()
				h.handleUpdate(ctx, &client.UpdateNewMessage{Message: msg})
				h.OnEditedMessage(ctx, msg)
				h.OnDeletedMessages(ctx, tt.actual, []int64{1}, true, false)
				q.ProcessAll()
				foreign := *msg
				foreign.ChatId = -tt.actual
				h.handleUpdate(ctx, &client.UpdateNewMessage{Message: &foreign})
				g.Retire()
				require.Zero(t, q.Len())
				mu.Lock()
				defer mu.Unlock()
				require.Len(t, bodies, 3)
				for i, kind := range []string{"new", "edited", "deleted"} {
					if schema == 1 {
						var env event.Envelope
						require.NoError(t, json.Unmarshal(bodies[i], &env))
						require.Equal(t, 1, env.SchemaVersion)
						require.Equal(t, kind, string(env.Event))
						require.Equal(t, tt.actual, env.ChatID)
						require.Equal(t, int64(1), env.MessageID)
					} else {
						var env v2Probe
						require.NoError(t, json.Unmarshal(bodies[i], &env))
						require.Equal(t, 2, env.SchemaVersion)
						require.Equal(t, kind, env.Event)
						require.Equal(t, float64(tt.actual), env.Message["chat_id"])
						require.Equal(t, float64(1), env.Message["message_id"])
					}
				}
			})
		}
	}
}
