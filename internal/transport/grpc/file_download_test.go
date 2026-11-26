package grpc

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/zelenin/go-tdlib/client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"

	"github.com/pure-golang/budva-claude/internal/app/facade"
	facademocks "github.com/pure-golang/budva-claude/internal/app/facade/mocks"
	"github.com/pure-golang/budva-claude/internal/transport/grpc/mocks"
	"github.com/pure-golang/budva-claude/internal/transport/grpc/pb"
)

const progressivePayloadDiagnostic = "PROGRESSIVE_PAYLOAD_BEFORE_COMPLETION"

type progressiveFile struct {
	*facademocks.TelegramRepo
	mu sync.Mutex

	id        int32
	size      int64
	unknown   bool
	available int
	prefix    []byte
	active    bool
	complete  bool

	started     chan struct{}
	release     chan struct{}
	releaseOnce sync.Once
	requests    []client.DownloadFileRequest
}

func (f *progressiveFile) assertDownloadRequests(t *testing.T) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	require.Len(t, f.requests, 1)
	request := f.requests[0]
	assert.False(t, request.Synchronous)
}

func (f *progressiveFile) markStarted() {
	if f.started != nil {
		close(f.started)
	}
}

func (f *progressiveFile) waitCompletion() {
	<-f.release
}

func (f *progressiveFile) releaseCompletion() {
	f.releaseOnce.Do(func() {
		f.mu.Lock()
		f.complete = true
		f.active = false
		f.available = len(f.prefix)
		f.size = int64(f.available)
		f.mu.Unlock()
		close(f.release)
	})
}

func newProgressiveFile(t *testing.T, payload []byte, barrier bool) *progressiveFile {
	t.Helper()
	available := len(payload)
	if barrier {
		available = 1
	}
	file := &progressiveFile{
		TelegramRepo: facademocks.NewTelegramRepo(t),
		id:           42,
		prefix:       payload,
		available:    available,
		active:       true,
		complete:     false,
	}
	file.size = int64(len(payload))
	if barrier {
		file.started = make(chan struct{})
		file.release = make(chan struct{})
	}
	return file
}

func (f *progressiveFile) DownloadFile(req *client.DownloadFileRequest) (*client.File, error) {
	f.mu.Lock()
	f.requests = append(f.requests, *req)
	f.mu.Unlock()
	if req.Synchronous {
		f.markStarted()
		f.waitCompletion()
	} else {
		f.markStarted()
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	size := f.size
	if f.unknown {
		size = 0
	}
	return &client.File{Id: f.id, Size: size, Local: &client.LocalFile{IsDownloadingActive: f.active, IsDownloadingCompleted: f.complete}}, nil
}

func (f *progressiveFile) GetFile(req *client.GetFileRequest) (*client.File, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	size := f.size
	if f.unknown {
		size = 0
	}
	return &client.File{Id: f.id, Size: size, Local: &client.LocalFile{IsDownloadingActive: f.active, IsDownloadingCompleted: f.complete}}, nil
}

func (f *progressiveFile) GetFileDownloadedPrefixSize(req *client.GetFileDownloadedPrefixSizeRequest) (*client.FileDownloadedPrefixSize, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	remaining := int64(f.available) - req.Offset
	if remaining < 0 {
		remaining = 0
	}
	return &client.FileDownloadedPrefixSize{Size: remaining}, nil
}

func (f *progressiveFile) ReadFilePart(req *client.ReadFilePartRequest) (*client.FilePart, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if req.Count <= 0 || req.Offset+req.Count > int64(f.available) {
		return nil, errors.New("read outside available prefix")
	}
	data := make([]byte, req.Count)
	copy(data, f.prefix[req.Offset:req.Offset+req.Count])
	return &client.FilePart{Data: data}, nil
}

func (f *progressiveFile) GetMessage(req *client.GetMessageRequest) (*client.Message, error) {
	return &client.Message{Id: req.MessageId, Content: &client.MessageDocument{Document: &client.Document{
		Document: &client.File{Id: f.id},
		MimeType: "application/octet-stream",
	}}}, nil
}

func newBufconnServer(t *testing.T, service pb.FacadeGRPCServer) pb.FacadeGRPCClient {
	t.Helper()
	listener := bufconn.Listen(1024 * 1024)
	t.Cleanup(func() { _ = listener.Close() })
	server := grpc.NewServer()
	pb.RegisterFacadeGRPCServer(server, service)
	go server.Serve(listener)
	t.Cleanup(server.Stop)
	conn, err := grpc.NewClient("passthrough:///bufnet", grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
		return listener.DialContext(ctx)
	}), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return pb.NewFacadeGRPCClient(conn)
}

func receiveDownloadEvents(t *testing.T, stream grpc.ServerStreamingClient[pb.FileDownloadEvent]) []*pb.FileDownloadEvent {
	t.Helper()
	var events []*pb.FileDownloadEvent
	for {
		event, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return events
		}
		require.NoError(t, err)
		events = append(events, event)
	}
}

func TestProgressiveDownloadBeforeCompletion(t *testing.T) {
	for _, mode := range []string{"message", "file-id"} {
		t.Run(mode, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				payload := []byte("PROGRESSIVE-PAYLOAD")
				file := newProgressiveFile(t, payload, true)
				service := facade.New(file)
				facadeClient := newBufconnServer(t, New(service))
				t.Cleanup(file.releaseCompletion)
				ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
				defer cancel()
				var stream grpc.ServerStreamingClient[pb.FileDownloadEvent]
				var err error
				if mode == "message" {
					stream, err = facadeClient.GetMessageFile(ctx, &pb.GetMessageFileRequest{ChatId: 100, MessageId: 7})
				} else {
					stream, err = facadeClient.DownloadFile(ctx, &pb.DownloadFileRequest{FileId: 42})
				}
				require.NoError(t, err)

				firstEvent, err := stream.Recv()
				require.NoError(t, err, progressivePayloadDiagnostic)
				require.NotNil(t, firstEvent.GetMetadata())

				chunk, err := stream.Recv()
				require.NoError(t, err, progressivePayloadDiagnostic)
				require.True(t, len(chunk.GetChunk().GetData()) > 0, progressivePayloadDiagnostic)
				assert.Equal(t, payload[:len(chunk.GetChunk().GetData())], chunk.GetChunk().GetData())
				assert.False(t, chunk.GetChunk().GetUpstreamComplete())
				select {
				case <-file.release:
					t.Fatal("completion barrier opened before client payload")
				default:
				}

				<-file.started
				file.assertDownloadRequests(t)
				file.releaseCompletion()
				events := receiveDownloadEvents(t, stream)
				require.NotEmpty(t, events)
				require.NotNil(t, events[len(events)-1].GetComplete())
				assert.Equal(t, int64(len(payload)), events[len(events)-1].GetComplete().GetSize())
				received := append([]byte(nil), chunk.GetChunk().GetData()...)
				for _, event := range events {
					if next := event.GetChunk(); next != nil {
						assert.Equal(t, int64(len(received)), next.Offset)
						received = append(received, next.Data...)
					}
				}
				assert.Equal(t, payload, received)
			})
		})
	}
}

func TestStreamingTransportValidationAndSendFailure(t *testing.T) {
	t.Parallel()

	t.Run("message validation", func(t *testing.T) {
		facadeMock := mocks.NewFacadeService(t)
		tr := New(facadeMock)
		err := tr.GetMessageFile(&pb.GetMessageFileRequest{ChatId: 0, MessageId: 1}, fakeEventStream{})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
		err = tr.GetMessageFile(&pb.GetMessageFileRequest{ChatId: -100, MessageId: 0}, fakeEventStream{})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})

	t.Run("send failure stops app", func(t *testing.T) {
		wantErr := errors.New("send failed")
		facadeMock := mocks.NewFacadeService(t)
		var sends int
		facadeMock.EXPECT().GetMessageFile(mock.Anything, int64(-100), int64(1), mock.Anything).
			RunAndReturn(func(_ context.Context, _ int64, _ int64, emit func(facade.FileDownloadEvent) error) error {
				err := emit(facade.FileDownloadEvent{Metadata: &facade.FileDownloadMetadata{FileID: 42}})
				require.NoError(t, err)
				err = emit(facade.FileDownloadEvent{Chunk: &facade.FileDownloadChunk{Offset: 0, Data: []byte("abc")}})
				require.ErrorIs(t, err, wantErr)
				return nil
			})
		tr := New(facadeMock)
		err := tr.GetMessageFile(&pb.GetMessageFileRequest{ChatId: -100, MessageId: 1}, fakeEventStream{emit: func(*pb.FileDownloadEvent) error {
			sends++
			if sends == 1 {
				return nil
			}
			return wantErr
		}})
		require.ErrorIs(t, err, wantErr)
		assert.Equal(t, 2, sends)
	})

	t.Run("download failed stable internal message", func(t *testing.T) {
		facadeMock := mocks.NewFacadeService(t)
		facadeMock.EXPECT().DownloadFile(mock.Anything, int32(42), mock.Anything).
			Return(&facade.FileDownloadFailedError{FileID: 42, Err: errors.New("/tmp/td/cache")})
		tr := New(facadeMock)
		err := tr.DownloadFile(&pb.DownloadFileRequest{FileId: 42}, fakeEventStream{})
		require.Error(t, err)
		assert.Equal(t, codes.Internal, status.Code(err))
		assert.Contains(t, err.Error(), "download file 42 failed")
		assert.NotContains(t, err.Error(), "/tmp/td/cache")
	})
}
