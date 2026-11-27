package facade

import (
	"bytes"
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/app/facade/mocks"
)

func TestService_GetMessageFile_TypedNilMedia(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content client.MessageContent
	}{
		{name: "nil content", content: nil},
		{name: "nil photo", content: (*client.MessagePhoto)(nil)},
		{name: "nil video", content: (*client.MessageVideo)(nil)},
		{name: "nil document", content: (*client.MessageDocument)(nil)},
		{name: "nil audio", content: (*client.MessageAudio)(nil)},
		{name: "nil voice note", content: (*client.MessageVoiceNote)(nil)},
		{name: "nil animation", content: (*client.MessageAnimation)(nil)},
		{name: "nil video note", content: (*client.MessageVideoNote)(nil)},
		{name: "nil sticker", content: (*client.MessageSticker)(nil)},
		{name: "nil video wrapper", content: &client.MessageVideo{}},
		{name: "nil document wrapper", content: &client.MessageDocument{}},
		{name: "nil audio wrapper", content: &client.MessageAudio{}},
		{name: "nil voice wrapper", content: &client.MessageVoiceNote{}},
		{name: "nil animation wrapper", content: &client.MessageAnimation{}},
		{name: "nil video note wrapper", content: &client.MessageVideoNote{}},
		{name: "nil sticker wrapper", content: &client.MessageSticker{}},
		{name: "nil photo wrapper", content: &client.MessagePhoto{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gw := mocks.NewTelegramRepo(t)
			svc := New(gw)
			gw.EXPECT().GetMessage(mock.Anything).Return(&client.Message{Content: tt.content}, nil)

			err := svc.GetMessageFile(context.Background(), 100, 7, func(FileDownloadEvent) error {
				return errors.New("unexpected emit")
			})

			require.Error(t, err)
			var notFound *MessageFileNotFoundError
			require.ErrorAs(t, err, &notFound)
			assert.Equal(t, int64(100), notFound.ChatID)
			assert.Equal(t, int64(7), notFound.MessageID)
		})
	}
}

func TestService_DownloadFile_ProgressiveBoundaries(t *testing.T) {
	t.Parallel()

	source := make([]byte, progressiveChunkSize*2+5)
	for index := range source {
		source[index] = byte(index % 251)
	}
	gw := mocks.NewTelegramRepo(t)
	svc := New(gw)

	gw.EXPECT().DownloadFile(mock.MatchedBy(func(req *client.DownloadFileRequest) bool {
		return req.FileId == 42 && req.Priority == progressivePriority && req.Offset == 0 &&
			req.Limit == 0 && !req.Synchronous
	})).Return(&client.File{Id: 42, Size: 0, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()

	available := int64(70000)
	upstreamComplete := false
	gw.EXPECT().GetFile(&client.GetFileRequest{FileId: 42}).RunAndReturn(func(*client.GetFileRequest) (*client.File, error) {
		return &client.File{Id: 42, Size: int64(len(source)), Local: &client.LocalFile{IsDownloadingActive: !upstreamComplete, IsDownloadingCompleted: upstreamComplete}}, nil
	})
	gw.EXPECT().GetFileDownloadedPrefixSize(mock.Anything).RunAndReturn(func(req *client.GetFileDownloadedPrefixSizeRequest) (*client.FileDownloadedPrefixSize, error) {
		return &client.FileDownloadedPrefixSize{Size: available - req.Offset}, nil
	})
	gw.EXPECT().ReadFilePart(mock.Anything).RunAndReturn(func(req *client.ReadFilePartRequest) (*client.FilePart, error) {
		if req.Count <= 0 || req.Count > progressiveChunkSize || req.Offset < 0 || req.Offset+req.Count > available {
			return nil, errors.New("read exceeds safe range or chunk cap")
		}
		return &client.FilePart{Data: source[req.Offset : req.Offset+req.Count]}, nil
	})

	var got bytes.Buffer
	var chunks []FileDownloadChunk
	var metadata *FileDownloadMetadata
	var complete *FileDownloadComplete
	err := svc.DownloadFile(context.Background(), 42, func(event FileDownloadEvent) error {
		switch {
		case event.Metadata != nil:
			metadata = event.Metadata
		case event.Chunk != nil:
			require.Equal(t, int64(got.Len()), event.Chunk.Offset)
			chunks = append(chunks, *event.Chunk)
			got.Write(event.Chunk.Data)
			available = int64(len(source))
			upstreamComplete = got.Len() == len(source)
		case event.Complete != nil:
			complete = event.Complete
		}
		return nil
	})

	require.NoError(t, err)
	require.NotNil(t, metadata)
	assert.Equal(t, int32(42), metadata.FileID)
	assert.False(t, metadata.SizeKnown)
	assert.Equal(t, int64(0), metadata.Size)
	assert.Len(t, chunks, 3)
	assert.Len(t, chunks[1].Data, progressiveChunkSize)
	for _, chunk := range chunks {
		assert.LessOrEqual(t, len(chunk.Data), progressiveChunkSize)
	}
	assert.True(t, bytes.Equal(source, got.Bytes()))
	require.NotNil(t, complete)
	assert.Equal(t, int64(len(source)), complete.Size)
}

func TestService_DownloadFile_UnknownSizeUsesRemainingBytes(t *testing.T) {
	t.Parallel()

	source := []byte("0123456789abcdef")
	gw := mocks.NewTelegramRepo(t)
	svc := New(gw)
	gw.EXPECT().DownloadFile(mock.Anything).Return(&client.File{Id: 42, Size: 0, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
	gw.EXPECT().GetFile(&client.GetFileRequest{FileId: 42}).Return(&client.File{Id: 42, Size: 0, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
	gw.EXPECT().GetFileDownloadedPrefixSize(&client.GetFileDownloadedPrefixSizeRequest{FileId: 42, Offset: 0}).Return(&client.FileDownloadedPrefixSize{Size: 6}, nil).Once()
	gw.EXPECT().ReadFilePart(&client.ReadFilePartRequest{FileId: 42, Offset: 0, Count: 6}).Return(&client.FilePart{Data: source[:6]}, nil).Once()
	gw.EXPECT().GetFile(&client.GetFileRequest{FileId: 42}).Return(&client.File{Id: 42, Size: 0, Local: &client.LocalFile{IsDownloadingCompleted: true}}, nil).Once()
	gw.EXPECT().GetFileDownloadedPrefixSize(&client.GetFileDownloadedPrefixSizeRequest{FileId: 42, Offset: 6}).Return(&client.FileDownloadedPrefixSize{Size: 10}, nil).Once()
	gw.EXPECT().ReadFilePart(&client.ReadFilePartRequest{FileId: 42, Offset: 6, Count: 10}).Return(&client.FilePart{Data: source[6:]}, nil).Once()
	gw.EXPECT().GetFile(&client.GetFileRequest{FileId: 42}).Return(&client.File{Id: 42, Size: 0, Local: &client.LocalFile{IsDownloadingCompleted: true}}, nil).Once()
	gw.EXPECT().GetFileDownloadedPrefixSize(&client.GetFileDownloadedPrefixSizeRequest{FileId: 42, Offset: 16}).Return(&client.FileDownloadedPrefixSize{Size: 0}, nil).Once()

	var got bytes.Buffer
	var complete *FileDownloadComplete
	err := svc.DownloadFile(context.Background(), 42, func(event FileDownloadEvent) error {
		if event.Chunk != nil {
			got.Write(event.Chunk.Data)
		}
		if event.Complete != nil {
			complete = event.Complete
		}
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, source, got.Bytes())
	require.NotNil(t, complete)
	assert.Equal(t, int64(len(source)), complete.Size)
}

func TestService_DownloadFile_CompleteEmptyFile(t *testing.T) {
	t.Parallel()

	gw := mocks.NewTelegramRepo(t)
	svc := New(gw)
	gw.EXPECT().DownloadFile(mock.Anything).Return(&client.File{Id: 42, Size: 0, Local: &client.LocalFile{IsDownloadingCompleted: true}}, nil).Once()
	gw.EXPECT().GetFile(&client.GetFileRequest{FileId: 42}).Return(&client.File{Id: 42, Size: 0, Local: &client.LocalFile{IsDownloadingCompleted: true}}, nil).Once()
	gw.EXPECT().GetFileDownloadedPrefixSize(&client.GetFileDownloadedPrefixSizeRequest{FileId: 42, Offset: 0}).Return(&client.FileDownloadedPrefixSize{Size: 0}, nil).Once()

	var events []FileDownloadEvent
	require.NoError(t, svc.DownloadFile(context.Background(), 42, func(event FileDownloadEvent) error {
		events = append(events, event)
		return nil
	}))
	require.Len(t, events, 2)
	assert.True(t, events[0].Metadata.SizeKnown)
	assert.Equal(t, int64(0), events[0].Metadata.Size)
	require.NotNil(t, events[1].Complete)
	assert.Equal(t, int64(0), events[1].Complete.Size)
}

func TestService_DownloadFile_TerminalFailures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		arrange func(gw *mocks.TelegramRepo)
	}{
		{
			name: "invalid start state",
			arrange: func(gw *mocks.TelegramRepo) {
				gw.EXPECT().DownloadFile(mock.Anything).Return(nil, nil).Once()
			},
		},
		{
			name: "invalid file state",
			arrange: func(gw *mocks.TelegramRepo) {
				gw.EXPECT().DownloadFile(mock.Anything).Return(&client.File{Id: 42, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
				gw.EXPECT().GetFile(mock.Anything).Return(nil, nil).Once()
			},
		},
		{
			name: "inactive incomplete",
			arrange: func(gw *mocks.TelegramRepo) {
				gw.EXPECT().DownloadFile(mock.Anything).Return(&client.File{Id: 42, Local: &client.LocalFile{}}, nil).Once()
				gw.EXPECT().GetFile(mock.Anything).Return(&client.File{Id: 42, Local: &client.LocalFile{}}, nil).Once()
				gw.EXPECT().GetFileDownloadedPrefixSize(mock.Anything).Return(&client.FileDownloadedPrefixSize{Size: 0}, nil).Once()
			},
		},
		{
			name: "final size mismatch",
			arrange: func(gw *mocks.TelegramRepo) {
				gw.EXPECT().DownloadFile(mock.Anything).Return(&client.File{Id: 42, Size: 100, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
				gw.EXPECT().GetFile(mock.Anything).Return(&client.File{Id: 42, Size: 100, Local: &client.LocalFile{IsDownloadingCompleted: true}}, nil).Once()
				gw.EXPECT().GetFileDownloadedPrefixSize(mock.Anything).Return(&client.FileDownloadedPrefixSize{Size: 0}, nil).Once()
			},
		},
		{
			name: "prefix exceeds complete size",
			arrange: func(gw *mocks.TelegramRepo) {
				gw.EXPECT().DownloadFile(mock.Anything).Return(&client.File{Id: 42, Size: 100, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
				gw.EXPECT().GetFile(mock.Anything).Return(&client.File{Id: 42, Size: 100, Local: &client.LocalFile{IsDownloadingCompleted: true}}, nil).Once()
				gw.EXPECT().GetFileDownloadedPrefixSize(mock.Anything).Return(&client.FileDownloadedPrefixSize{Size: 101}, nil).Once()
			},
		},
		{
			name: "nil prefix",
			arrange: func(gw *mocks.TelegramRepo) {
				gw.EXPECT().DownloadFile(mock.Anything).Return(&client.File{Id: 42, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
				gw.EXPECT().GetFile(mock.Anything).Return(&client.File{Id: 42, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
				gw.EXPECT().GetFileDownloadedPrefixSize(mock.Anything).Return(nil, nil).Once()
			},
		},
		{
			name: "negative prefix",
			arrange: func(gw *mocks.TelegramRepo) {
				gw.EXPECT().DownloadFile(mock.Anything).Return(&client.File{Id: 42, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
				gw.EXPECT().GetFile(mock.Anything).Return(&client.File{Id: 42, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
				gw.EXPECT().GetFileDownloadedPrefixSize(mock.Anything).Return(&client.FileDownloadedPrefixSize{Size: -1}, nil).Once()
			},
		},
		{
			name: "nil part",
			arrange: func(gw *mocks.TelegramRepo) {
				gw.EXPECT().DownloadFile(mock.Anything).Return(&client.File{Id: 42, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
				gw.EXPECT().GetFile(mock.Anything).Return(&client.File{Id: 42, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
				gw.EXPECT().GetFileDownloadedPrefixSize(mock.Anything).Return(&client.FileDownloadedPrefixSize{Size: 3}, nil).Once()
				gw.EXPECT().ReadFilePart(mock.Anything).Return(nil, nil).Once()
			},
		},
		{
			name: "short read",
			arrange: func(gw *mocks.TelegramRepo) {
				gw.EXPECT().DownloadFile(mock.Anything).Return(&client.File{Id: 42, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
				gw.EXPECT().GetFile(mock.Anything).Return(&client.File{Id: 42, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
				gw.EXPECT().GetFileDownloadedPrefixSize(mock.Anything).Return(&client.FileDownloadedPrefixSize{Size: 10}, nil).Once()
				gw.EXPECT().ReadFilePart(mock.Anything).Return(&client.FilePart{Data: []byte("abc")}, nil).Once()
			},
		},
		{
			name: "oversized read",
			arrange: func(gw *mocks.TelegramRepo) {
				gw.EXPECT().DownloadFile(mock.Anything).Return(&client.File{Id: 42, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
				gw.EXPECT().GetFile(mock.Anything).Return(&client.File{Id: 42, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
				gw.EXPECT().GetFileDownloadedPrefixSize(mock.Anything).Return(&client.FileDownloadedPrefixSize{Size: 3}, nil).Once()
				gw.EXPECT().ReadFilePart(mock.Anything).Return(&client.FilePart{Data: []byte("abcd")}, nil).Once()
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gw := mocks.NewTelegramRepo(t)
			svc := New(gw)
			tt.arrange(gw)

			err := svc.DownloadFile(context.Background(), 42, func(event FileDownloadEvent) error {
				if event.Complete != nil {
					return errors.New("complete emitted after failure")
				}
				return nil
			})

			require.Error(t, err)
			var failed *FileDownloadFailedError
			require.ErrorAs(t, err, &failed)
			assert.Equal(t, int32(42), failed.FileID)
		})
	}
}

func TestService_DownloadFile_ReadError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("tdlib read failed")
	gw := mocks.NewTelegramRepo(t)
	svc := New(gw)
	gw.EXPECT().DownloadFile(mock.Anything).Return(&client.File{Id: 42, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
	gw.EXPECT().GetFile(mock.Anything).Return(&client.File{Id: 42, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
	gw.EXPECT().GetFileDownloadedPrefixSize(mock.Anything).Return(&client.FileDownloadedPrefixSize{Size: 3}, nil).Once()
	gw.EXPECT().ReadFilePart(mock.Anything).Return(nil, wantErr).Once()

	err := svc.DownloadFile(context.Background(), 42, func(FileDownloadEvent) error { return nil })
	require.ErrorIs(t, err, wantErr)
	var failed *FileDownloadFailedError
	require.ErrorAs(t, err, &failed)
}

func TestService_GetMessageFile_PhotoTieAndMissingMessage(t *testing.T) {
	t.Parallel()
	first := &client.File{Id: 1, Size: 20}
	last := &client.File{Id: 2, Size: 20}
	file, mimeType := resolveMessageFile(&client.Message{Content: &client.MessagePhoto{Photo: &client.Photo{Sizes: []*client.PhotoSize{
		nil, {Photo: first}, {Photo: last}, {Photo: &client.File{Id: 3, Size: 10}},
	}}}})
	assert.Same(t, last, file)
	assert.Equal(t, "image/jpeg", mimeType)

	repo := mocks.NewTelegramRepo(t)
	missing := NewMessageNotFoundError(100, 7)
	repo.EXPECT().GetMessage(mock.Anything).Return(nil, missing).Once()
	err := New(repo).GetMessageFile(context.Background(), 100, 7, func(FileDownloadEvent) error {
		t.Error("missing message emitted an event")
		return nil
	})
	require.ErrorIs(t, err, missing)
}

func TestService_DownloadFile_CancelAfterCompleteEmit(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repo := mocks.NewTelegramRepo(t)
	file := &client.File{Id: 42, Local: &client.LocalFile{IsDownloadingCompleted: true}}
	repo.EXPECT().DownloadFile(mock.Anything).Return(file, nil).Once()
	repo.EXPECT().GetFile(mock.Anything).Return(file, nil).Once()
	repo.EXPECT().GetFileDownloadedPrefixSize(mock.Anything).Return(&client.FileDownloadedPrefixSize{}, nil).Once()
	err := New(repo).DownloadFile(ctx, 42, func(event FileDownloadEvent) error {
		if event.Complete != nil {
			cancel()
		}
		return nil
	})
	require.ErrorIs(t, err, context.Canceled)
}

func TestService_DownloadFile_EmitErrorStopsReads(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("callback failed")
	gw := mocks.NewTelegramRepo(t)
	svc := New(gw)
	gw.EXPECT().DownloadFile(mock.Anything).Return(&client.File{Id: 42, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
	gw.EXPECT().GetFile(mock.Anything).Return(&client.File{Id: 42, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
	gw.EXPECT().GetFileDownloadedPrefixSize(mock.Anything).Return(&client.FileDownloadedPrefixSize{Size: 3}, nil).Once()
	gw.EXPECT().ReadFilePart(mock.Anything).Return(&client.FilePart{Data: []byte("abc")}, nil).Once()

	err := svc.DownloadFile(context.Background(), 42, func(event FileDownloadEvent) error {
		if event.Chunk != nil {
			return wantErr
		}
		return nil
	})
	require.ErrorIs(t, err, wantErr)
	gw.AssertExpectations(t)
}

func TestService_DownloadFile_ContextSemantics(t *testing.T) {
	t.Run("already cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)

		err := svc.DownloadFile(ctx, 42, func(FileDownloadEvent) error { return nil })
		require.ErrorIs(t, err, context.Canceled)
	})

	t.Run("cancel during empty-prefix wait", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			gw := mocks.NewTelegramRepo(t)
			svc := New(gw)
			gw.EXPECT().DownloadFile(mock.Anything).Return(&client.File{Id: 42, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
			gw.EXPECT().GetFile(mock.Anything).Return(&client.File{Id: 42, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
			gw.EXPECT().GetFileDownloadedPrefixSize(mock.Anything).Return(&client.FileDownloadedPrefixSize{Size: 0}, nil).Once()

			done := make(chan error, 1)
			go func() {
				done <- svc.DownloadFile(ctx, 42, func(FileDownloadEvent) error { return nil })
			}()
			synctest.Wait()
			cancel()
			synctest.Wait()

			require.ErrorIs(t, <-done, context.Canceled)
			gw.AssertExpectations(t)
		})
	})

	t.Run("cancel after chunk", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			gw := mocks.NewTelegramRepo(t)
			svc := New(gw)
			gw.EXPECT().DownloadFile(mock.Anything).Return(&client.File{Id: 42, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
			gw.EXPECT().GetFile(mock.Anything).Return(&client.File{Id: 42, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
			gw.EXPECT().GetFileDownloadedPrefixSize(mock.Anything).Return(&client.FileDownloadedPrefixSize{Size: 3}, nil).Once()
			gw.EXPECT().ReadFilePart(mock.Anything).Return(&client.FilePart{Data: []byte("abc")}, nil).Once()

			err := svc.DownloadFile(ctx, 42, func(event FileDownloadEvent) error {
				if event.Chunk != nil {
					cancel()
				}
				return nil
			})
			require.ErrorIs(t, err, context.Canceled)
			gw.AssertExpectations(t)
		})
	})

	t.Run("cancel during in-flight SDK call", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			gw := mocks.NewTelegramRepo(t)
			svc := New(gw)
			gw.EXPECT().DownloadFile(mock.Anything).Return(&client.File{Id: 42, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
			gw.EXPECT().GetFile(mock.Anything).RunAndReturn(func(*client.GetFileRequest) (*client.File, error) {
				cancel()
				synctest.Wait()
				return &client.File{Id: 42, Local: &client.LocalFile{IsDownloadingActive: true}}, nil
			}).Once()

			err := svc.DownloadFile(ctx, 42, func(FileDownloadEvent) error { return nil })
			require.ErrorIs(t, err, context.Canceled)
			gw.AssertExpectations(t)
		})
	})

	t.Run("cancel during prefix query", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			gw := mocks.NewTelegramRepo(t)
			svc := New(gw)
			gw.EXPECT().DownloadFile(mock.Anything).Return(&client.File{Id: 42, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
			gw.EXPECT().GetFile(mock.Anything).Return(&client.File{Id: 42, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
			gw.EXPECT().GetFileDownloadedPrefixSize(mock.Anything).RunAndReturn(func(*client.GetFileDownloadedPrefixSizeRequest) (*client.FileDownloadedPrefixSize, error) {
				cancel()
				synctest.Wait()
				return &client.FileDownloadedPrefixSize{Size: 3}, nil
			}).Once()

			err := svc.DownloadFile(ctx, 42, func(FileDownloadEvent) error { return nil })
			require.ErrorIs(t, err, context.Canceled)
			gw.AssertExpectations(t)
		})
	})

	t.Run("cancel after metadata emit", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			gw := mocks.NewTelegramRepo(t)
			svc := New(gw)
			gw.EXPECT().DownloadFile(mock.Anything).Return(&client.File{Id: 42, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
			err := svc.DownloadFile(ctx, 42, func(FileDownloadEvent) error {
				cancel()
				synctest.Wait()
				return nil
			})
			require.ErrorIs(t, err, context.Canceled)
			gw.AssertExpectations(t)
		})
	})

	t.Run("deadline during prefix query", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(1*time.Second))
			defer cancel()
			gw := mocks.NewTelegramRepo(t)
			svc := New(gw)
			gw.EXPECT().DownloadFile(mock.Anything).Return(&client.File{Id: 42, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
			gw.EXPECT().GetFile(mock.Anything).Return(&client.File{Id: 42, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
			gw.EXPECT().GetFileDownloadedPrefixSize(mock.Anything).RunAndReturn(func(*client.GetFileDownloadedPrefixSizeRequest) (*client.FileDownloadedPrefixSize, error) {
				<-ctx.Done()
				return &client.FileDownloadedPrefixSize{Size: 0}, nil
			}).Once()

			err := svc.DownloadFile(ctx, 42, func(FileDownloadEvent) error { return nil })
			require.ErrorIs(t, err, context.DeadlineExceeded)
			gw.AssertExpectations(t)
		})
	})

	t.Run("deadline before start", func(t *testing.T) {
		ctx, cancel := context.WithDeadline(context.Background(), time.Unix(0, 0))
		defer cancel()
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)

		err := svc.DownloadFile(ctx, 42, func(FileDownloadEvent) error { return nil })
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})
}

func TestService_DownloadFile_CompletionRace(t *testing.T) {
	t.Parallel()

	t.Run("zero prefix after completion", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			var complete bool
			gw := mocks.NewTelegramRepo(t)
			svc := New(gw)
			gw.EXPECT().DownloadFile(mock.Anything).Return(&client.File{Id: 42, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
			gw.EXPECT().GetFile(mock.Anything).Return(&client.File{Id: 42, Size: 0, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
			gw.EXPECT().GetFileDownloadedPrefixSize(mock.Anything).RunAndReturn(func(*client.GetFileDownloadedPrefixSizeRequest) (*client.FileDownloadedPrefixSize, error) {
				complete = true
				return &client.FileDownloadedPrefixSize{Size: 0}, nil
			}).Once()
			gw.EXPECT().GetFile(mock.Anything).Return(&client.File{Id: 42, Size: 0, Local: &client.LocalFile{IsDownloadingCompleted: true}}, nil).Once()
			gw.EXPECT().GetFileDownloadedPrefixSize(mock.Anything).Return(&client.FileDownloadedPrefixSize{Size: 0}, nil).Once()

			var completeSize int64
			require.NoError(t, svc.DownloadFile(context.Background(), 42, func(event FileDownloadEvent) error {
				if event.Complete != nil {
					completeSize = event.Complete.Size
				}
				return nil
			}))
			assert.True(t, complete)
			assert.Equal(t, int64(0), completeSize)
		})
	})

	t.Run("final data after completion", func(t *testing.T) {
		var complete bool
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		gw.EXPECT().DownloadFile(mock.Anything).Return(&client.File{Id: 42, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
		gw.EXPECT().GetFile(mock.Anything).Return(&client.File{Id: 42, Size: 20, Local: &client.LocalFile{IsDownloadingActive: true}}, nil).Once()
		gw.EXPECT().GetFileDownloadedPrefixSize(mock.Anything).RunAndReturn(func(*client.GetFileDownloadedPrefixSizeRequest) (*client.FileDownloadedPrefixSize, error) {
			complete = true
			return &client.FileDownloadedPrefixSize{Size: 20}, nil
		}).Once()
		gw.EXPECT().ReadFilePart(mock.Anything).Return(&client.FilePart{Data: []byte("01234567890123456789")}, nil).Once()
		gw.EXPECT().GetFile(mock.Anything).Return(&client.File{Id: 42, Size: 20, Local: &client.LocalFile{IsDownloadingCompleted: true}}, nil).Once()
		gw.EXPECT().GetFileDownloadedPrefixSize(mock.Anything).Return(&client.FileDownloadedPrefixSize{Size: 0}, nil).Once()

		var got bytes.Buffer
		require.NoError(t, svc.DownloadFile(context.Background(), 42, func(event FileDownloadEvent) error {
			if event.Chunk != nil {
				got.Write(event.Chunk.Data)
			}
			return nil
		}))
		assert.Equal(t, 20, got.Len())
		assert.True(t, complete)
	})
}

func TestService_DownloadFile_CallbackBackpressure(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		payload := []byte("abc")
		var blocked atomic.Bool
		var stateCalls atomic.Int64
		errReadWhileBlocked := errors.New("SDK read while callback was blocked")

		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		gw.EXPECT().DownloadFile(mock.Anything).Return(&client.File{
			Id:    42,
			Size:  int64(len(payload)),
			Local: &client.LocalFile{IsDownloadingActive: true},
		}, nil).Once()
		gw.EXPECT().GetFile(mock.Anything).RunAndReturn(func(*client.GetFileRequest) (*client.File, error) {
			stateCalls.Add(1)
			if blocked.Load() {
				return nil, errReadWhileBlocked
			}
			return &client.File{
				Id:    42,
				Size:  int64(len(payload)),
				Local: &client.LocalFile{IsDownloadingCompleted: true},
			}, nil
		})
		gw.EXPECT().GetFileDownloadedPrefixSize(&client.GetFileDownloadedPrefixSizeRequest{FileId: 42, Offset: 0}).
			Return(&client.FileDownloadedPrefixSize{Size: int64(len(payload))}, nil).Once()
		gw.EXPECT().GetFileDownloadedPrefixSize(&client.GetFileDownloadedPrefixSizeRequest{FileId: 42, Offset: int64(len(payload))}).
			Return(&client.FileDownloadedPrefixSize{Size: 0}, nil).Once()
		gw.EXPECT().ReadFilePart(&client.ReadFilePartRequest{FileId: 42, Offset: 0, Count: int64(len(payload))}).
			Return(&client.FilePart{Data: payload}, nil).Once()

		release := make(chan struct{})
		var releaseOnce sync.Once
		releaseCallback := func() { releaseOnce.Do(func() { close(release) }) }
		defer releaseCallback()
		chunkReceived := make(chan struct{})
		done := make(chan error, 1)
		go func() {
			done <- svc.DownloadFile(context.Background(), 42, func(event FileDownloadEvent) error {
				if event.Chunk == nil {
					return nil
				}
				blocked.Store(true)
				close(chunkReceived)
				<-release
				blocked.Store(false)
				return nil
			})
		}()

		select {
		case <-chunkReceived:
		case <-time.After(5 * time.Second):
			t.Fatal("chunk was not emitted before completion was observed")
		}
		synctest.Wait()
		assert.True(t, blocked.Load())
		assert.Equal(t, int64(1), stateCalls.Load())
		releaseCallback()
		require.NoError(t, <-done)
	})
}

type sharedDownloadRepo struct {
	*mocks.TelegramRepo

	mu       sync.Mutex
	arrived  chan struct{}
	release  chan struct{}
	payload  []byte
	prefix   int
	complete bool
	active   bool
}

func (r *sharedDownloadRepo) DownloadFile(req *client.DownloadFileRequest) (*client.File, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return &client.File{
		Id:    42,
		Size:  0,
		Local: &client.LocalFile{IsDownloadingActive: r.active, IsDownloadingCompleted: r.complete},
	}, nil
}

func (r *sharedDownloadRepo) GetFile(req *client.GetFileRequest) (*client.File, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	size := int64(0)
	if r.complete {
		size = int64(len(r.payload))
	}
	return &client.File{
		Id:    42,
		Size:  size,
		Local: &client.LocalFile{IsDownloadingActive: r.active, IsDownloadingCompleted: r.complete},
	}, nil
}

func (r *sharedDownloadRepo) GetFileDownloadedPrefixSize(req *client.GetFileDownloadedPrefixSizeRequest) (*client.FileDownloadedPrefixSize, error) {
	select {
	case r.arrived <- struct{}{}:
	default:
	}
	<-r.release
	r.mu.Lock()
	defer r.mu.Unlock()
	remaining := int64(r.prefix) - req.Offset
	if remaining < 0 {
		remaining = 0
	}
	return &client.FileDownloadedPrefixSize{Size: remaining}, nil
}

func (r *sharedDownloadRepo) ReadFilePart(req *client.ReadFilePartRequest) (*client.FilePart, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if req.Offset < 0 || req.Count <= 0 || req.Offset+req.Count > int64(len(r.payload)) ||
		req.Offset+req.Count > int64(r.prefix) {
		return nil, errors.New("read outside available prefix")
	}
	data := make([]byte, req.Count)
	copy(data, r.payload[req.Offset:req.Offset+req.Count])
	return &client.FilePart{Data: data}, nil
}

func (r *sharedDownloadRepo) setPrefix(prefix int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prefix = prefix
}

func (r *sharedDownloadRepo) finish() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prefix = len(r.payload)
	r.complete = true
	r.active = false
}

func TestService_DownloadFile_SharedFileConcurrentReaders(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		payload := []byte("0123456789abcdef")
		repo := &sharedDownloadRepo{TelegramRepo: mocks.NewTelegramRepo(t), payload: payload, active: true}
		repo.arrived = make(chan struct{}, 2)
		repo.release = make(chan struct{})
		svc := New(repo)

		ctxA, cancelA := context.WithCancel(t.Context())
		defer cancelA()
		var gotB bytes.Buffer
		doneA := make(chan error, 1)
		doneB := make(chan error, 1)
		go func() {
			doneA <- svc.DownloadFile(ctxA, 42, func(FileDownloadEvent) error { return nil })
		}()
		go func() {
			doneB <- svc.DownloadFile(t.Context(), 42, func(event FileDownloadEvent) error {
				if event.Chunk != nil {
					gotB.Write(event.Chunk.Data)
				}
				return nil
			})
		}()

		// Both readers must have reached the prefix query before advancing it.
		<-repo.arrived
		<-repo.arrived
		repo.setPrefix(3)
		close(repo.release)
		// Let both readers drain the first prefix and block in their poll timers.
		synctest.Wait()
		assert.Equal(t, payload[:3], gotB.Bytes())

		cancelA()
		synctest.Wait()
		require.ErrorIs(t, <-doneA, context.Canceled)

		repo.finish()
		synctest.Wait()
		require.NoError(t, <-doneB)
		assert.Equal(t, payload, gotB.Bytes())
	})
}
