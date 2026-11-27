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
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/pure-golang/budva-claude/internal/app/facade"
	facademocks "github.com/pure-golang/budva-claude/internal/app/facade/mocks"
	"github.com/pure-golang/budva-claude/internal/infra/telegram"
	"github.com/pure-golang/budva-claude/internal/transport/grpc/mocks"
	"github.com/pure-golang/budva-claude/internal/transport/grpc/pb"
)

func newBufconnFacadeClient(t *testing.T, service pb.FacadeGRPCServer) pb.FacadeGRPCClient {
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

func TestListMessageFiles_Validation(t *testing.T) {
	t.Parallel()

	mockService := mocks.NewFacadeService(t)
	tr := New(mockService)

	_, err := tr.ListMessageFiles(context.Background(), &pb.ListMessageFilesRequest{ChatId: 0, MessageId: 1})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))

	_, err = tr.ListMessageFiles(context.Background(), &pb.ListMessageFilesRequest{ChatId: -100, MessageId: 0})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestListMessageFiles_Mapping(t *testing.T) {
	t.Parallel()

	appResult := &facade.MessageFiles{
		ChatID:           -100,
		MessageID:        7,
		MediaAlbumID:     42,
		ContentSupported: true,
		DefaultFileID:    103,
		Items: []facade.MessageMediaItem{
			{
				ItemIndex: 0,
				Kind:      facade.MessageMediaKindPhoto,
				State:     facade.MessageMediaStateFilesPresent,
				Files: []facade.MessageFileDescriptor{
					{FileID: 101, Role: facade.MessageFileRolePhotoSize, SourceIndex: 0, MIMEType: "image/jpeg", Width: 100, Height: 100, Size: 1000, SizeKnown: true, PhotoSizeType: "s"},
				},
			},
		},
	}
	mockService := mocks.NewFacadeService(t)
	mockService.EXPECT().ListMessageFiles(mock.Anything, int64(-100), int64(7)).Return(appResult, nil).Once()
	tr := New(mockService)

	response, err := tr.ListMessageFiles(context.Background(), &pb.ListMessageFilesRequest{ChatId: -100, MessageId: 7})
	require.NoError(t, err)
	require.NotNil(t, response)
	assert.Equal(t, int64(-100), response.ChatId)
	assert.Equal(t, int64(7), response.MessageId)
	assert.Equal(t, int64(42), response.MediaAlbumId)
	assert.True(t, response.ContentSupported)
	assert.Equal(t, int64(103), response.DefaultFileId)
	require.Len(t, response.Items, 1)
	item := response.Items[0]
	assert.Equal(t, int32(0), item.ItemIndex)
	assert.Equal(t, pb.MessageMediaKind_MESSAGE_MEDIA_KIND_PHOTO, item.Kind)
	assert.Equal(t, pb.MessageMediaState_MESSAGE_MEDIA_STATE_FILES_PRESENT, item.State)
	require.Len(t, item.Files, 1)
	file := item.Files[0]
	assert.Equal(t, int64(101), file.FileId)
	assert.Equal(t, pb.MessageFileRole_MESSAGE_FILE_ROLE_PHOTO_SIZE, file.Role)
	assert.Equal(t, int32(0), file.SourceIndex)
	assert.Equal(t, "image/jpeg", file.MimeType)
	assert.Equal(t, int64(1000), file.Size)
	assert.True(t, file.SizeKnown)
	assert.Equal(t, int32(100), file.Width)
	assert.Equal(t, "s", file.PhotoSizeType)
}

func TestListMessageFiles_ErrorMapping(t *testing.T) {
	t.Parallel()

	floodWait := &telegram.FloodWaitError{Delay: 3 * time.Second}
	notReady := &telegram.ChatNotReadyError{ChatID: 100}
	tests := []struct {
		name      string
		err       error
		wantCode  codes.Code
		wantDelay time.Duration
	}{
		{name: "canceled", err: context.Canceled, wantCode: codes.Canceled},
		{name: "deadline", err: context.DeadlineExceeded, wantCode: codes.DeadlineExceeded},
		{name: "not found", err: facade.NewMessageNotFoundError(100, 7), wantCode: codes.NotFound},
		{name: "flood wait", err: floodWait, wantCode: codes.ResourceExhausted, wantDelay: 3 * time.Second},
		{name: "chat not ready", err: notReady, wantCode: codes.Unavailable, wantDelay: chatNotReadyRetryDelay},
		{name: "other internal", err: errors.New("boom"), wantCode: codes.Internal},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			mockService := mocks.NewFacadeService(t)
			mockService.EXPECT().ListMessageFiles(mock.Anything, mock.Anything, mock.Anything).Return(nil, tt.err).Once()
			tr := New(mockService)
			_, err := tr.ListMessageFiles(context.Background(), &pb.ListMessageFilesRequest{ChatId: 100, MessageId: 7})
			require.Error(t, err)
			assert.Equal(t, tt.wantCode, status.Code(err))
			if tt.wantDelay > 0 {
				st := status.Convert(err)
				require.Len(t, st.Details(), 1)
				retryInfo, ok := st.Details()[0].(*errdetails.RetryInfo)
				require.True(t, ok)
				assert.Equal(t, durationpb.New(tt.wantDelay), retryInfo.RetryDelay)
			}
		})
	}
}

func TestListMessageFiles_RawResponseErrorStaysInternal(t *testing.T) {
	t.Parallel()

	sharedErr := client.ResponseError{Err: &client.Error{Code: 400, Message: "bad request"}}
	mockService := mocks.NewFacadeService(t)
	mockService.EXPECT().ListMessageFiles(mock.Anything, mock.Anything, mock.Anything).Return(nil, sharedErr).Once()
	tr := New(mockService)

	_, err := tr.ListMessageFiles(context.Background(), &pb.ListMessageFilesRequest{ChatId: 100, MessageId: 7})
	require.Error(t, err)
	assert.Equal(t, codes.Internal, status.Code(err))
	assert.Contains(t, err.Error(), "bad request")
}

type selectableFileRepo struct {
	*facademocks.TelegramRepo
	mu sync.Mutex

	payloads        map[int32][]byte
	requests        []client.DownloadFileRequest
	available       int
	active          bool
	complete        bool
	downloadAllowed bool

	started     chan struct{}
	release     chan struct{}
	releaseOnce sync.Once
}

func newSelectableFileRepo(t *testing.T, payloads map[int32][]byte, barrier bool) *selectableFileRepo {
	t.Helper()
	repo := &selectableFileRepo{
		TelegramRepo: facademocks.NewTelegramRepo(t),
		payloads:     payloads,
		active:       true,
	}
	if barrier {
		repo.available = 1
		repo.started = make(chan struct{})
		repo.release = make(chan struct{})
	}
	return repo
}

func (r *selectableFileRepo) releaseCompletion() {
	r.releaseOnce.Do(func() {
		r.mu.Lock()
		r.complete = true
		r.active = false
		r.mu.Unlock()
		close(r.release)
	})
}

func (r *selectableFileRepo) DownloadFile(req *client.DownloadFileRequest) (*client.File, error) {
	r.mu.Lock()
	if !r.downloadAllowed {
		r.mu.Unlock()
		return nil, errors.New("download not allowed")
	}
	r.markStartedLocked()
	r.requests = append(r.requests, *req)
	size := len(r.payloads[req.FileId])
	active, complete := r.active, r.complete
	wait := r.release
	r.mu.Unlock()
	if req.Synchronous && wait != nil {
		<-wait
		r.mu.Lock()
		active, complete = r.active, r.complete
		r.mu.Unlock()
	}
	return &client.File{Id: req.FileId, Size: int64(size), Local: &client.LocalFile{IsDownloadingActive: active, IsDownloadingCompleted: complete}}, nil
}

func (r *selectableFileRepo) markStartedLocked() {
	if r.started != nil {
		select {
		case <-r.started:
		default:
			close(r.started)
		}
	}
}

func (r *selectableFileRepo) allowDownload(t *testing.T) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	require.Empty(t, r.requests)
	r.downloadAllowed = true
}

func (r *selectableFileRepo) assertDownloadRequest(t *testing.T, fileID int64) {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	require.Len(t, r.requests, 1)
	assert.False(t, r.requests[0].Synchronous)
	assert.Equal(t, fileID, int64(r.requests[0].FileId))
}

func (r *selectableFileRepo) GetFile(req *client.GetFileRequest) (*client.File, error) {
	r.mu.Lock()
	if !r.downloadAllowed {
		r.mu.Unlock()
		return nil, errors.New("download not allowed")
	}
	defer r.mu.Unlock()
	return &client.File{Id: req.FileId, Size: int64(len(r.payloads[req.FileId])), Local: &client.LocalFile{IsDownloadingActive: r.active, IsDownloadingCompleted: r.complete}}, nil
}

func (r *selectableFileRepo) GetFileDownloadedPrefixSize(req *client.GetFileDownloadedPrefixSizeRequest) (*client.FileDownloadedPrefixSize, error) {
	r.mu.Lock()
	if !r.downloadAllowed {
		r.mu.Unlock()
		return nil, errors.New("download not allowed")
	}
	readableEnd := int64(len(r.payloads[req.FileId]))
	if !r.complete && readableEnd > int64(r.available) {
		readableEnd = int64(r.available)
	}
	remaining := readableEnd - req.Offset
	if remaining < 0 {
		remaining = 0
	}
	defer r.mu.Unlock()
	return &client.FileDownloadedPrefixSize{Size: remaining}, nil
}

func (r *selectableFileRepo) ReadFilePart(req *client.ReadFilePartRequest) (*client.FilePart, error) {
	r.mu.Lock()
	if !r.downloadAllowed {
		r.mu.Unlock()
		return nil, errors.New("download not allowed")
	}
	payload := r.payloads[req.FileId]
	readableEnd := int64(len(payload))
	if !r.complete && readableEnd > int64(r.available) {
		readableEnd = int64(r.available)
	}
	bound := req.Offset + req.Count
	r.mu.Unlock()
	if req.Count <= 0 || bound > readableEnd {
		return nil, errors.New("read outside available prefix")
	}
	data := make([]byte, req.Count)
	copy(data, payload[req.Offset:req.Offset+req.Count])
	return &client.FilePart{Data: data}, nil
}

func TestListMessageFiles_RealBufconnPhotoThenSelectNonDefaultDownload(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const defaultPhotoFileID = 103
		const selectedPhotoFileID = 102
		defaultPayload := []byte("DEFAULT-LARGEST-PHOTO-BYTES")
		selectedPayload := []byte("SELECTED-PHOTO-PAYLOAD")
		defaultSize := int64(len(defaultPayload))
		selectedSize := int64(len(selectedPayload))
		require.Greater(t, defaultSize, selectedSize)
		payloads := map[int32][]byte{
			101:                 []byte("small-photo"),
			selectedPhotoFileID: selectedPayload,
			defaultPhotoFileID:  defaultPayload,
		}

		repo := newSelectableFileRepo(t, payloads, true)
		defer repo.releaseCompletion()
		repo.EXPECT().GetMessage(&client.GetMessageRequest{ChatId: -100, MessageId: 7}).Return(&client.Message{
			Content: &client.MessagePhoto{Photo: &client.Photo{Sizes: []*client.PhotoSize{
				{Photo: &client.File{Id: 101, Size: int64(len(payloads[101]))}, Type: "s"},
				{Photo: &client.File{Id: selectedPhotoFileID, Size: selectedSize}, Type: "m"},
				{Photo: &client.File{Id: defaultPhotoFileID, Size: defaultSize}, Type: "x"},
			}}},
		}, nil).Once()

		service := facade.New(repo)
		clientConn := newBufconnFacadeClient(t, New(service))
		t.Cleanup(repo.releaseCompletion)

		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		response, err := clientConn.ListMessageFiles(ctx, &pb.ListMessageFilesRequest{ChatId: -100, MessageId: 7})
		require.NoError(t, err)
		require.Len(t, response.Items, 1)
		descriptors := response.Items[0].Files
		require.Len(t, descriptors, 3)
		assert.Equal(t, int64(101), descriptors[0].FileId)
		assert.Equal(t, int64(selectedPhotoFileID), descriptors[1].FileId)
		assert.Equal(t, int64(defaultPhotoFileID), descriptors[2].FileId)
		assert.Equal(t, int64(defaultPhotoFileID), response.DefaultFileId)

		var selectedDescriptor *pb.MessageFileDescriptor
		for _, descriptor := range descriptors {
			if descriptor.GetFileId() == int64(selectedPhotoFileID) {
				require.Nil(t, selectedDescriptor)
				require.Equal(t, pb.MessageFileRole_MESSAGE_FILE_ROLE_PHOTO_SIZE, descriptor.GetRole())
				require.Equal(t, int32(1), descriptor.GetSourceIndex())
				require.NotEqual(t, int64(response.DefaultFileId), descriptor.GetFileId())
				selectedDescriptor = descriptor
			}
		}
		require.NotNil(t, selectedDescriptor)

		repo.allowDownload(t)
		stream, err := clientConn.DownloadFile(ctx, &pb.DownloadFileRequest{FileId: selectedDescriptor.GetFileId()})
		require.NoError(t, err)

		firstEvent, err := stream.Recv()
		require.NoError(t, err, progressivePayloadDiagnostic)
		require.NotNil(t, firstEvent.GetMetadata())
		assert.Equal(t, selectedDescriptor.GetFileId(), firstEvent.GetMetadata().GetFileId())
		assert.True(t, firstEvent.GetMetadata().GetSizeKnown())
		assert.Equal(t, selectedSize, firstEvent.GetMetadata().GetSize())

		chunk, err := stream.Recv()
		require.NoError(t, err, progressivePayloadDiagnostic)
		require.NotNil(t, chunk.GetChunk())
		require.NotEmpty(t, chunk.GetChunk().GetData())
		assert.False(t, chunk.GetChunk().GetUpstreamComplete(), progressivePayloadDiagnostic)
		assert.Equal(t, int64(0), chunk.GetChunk().GetOffset())
		assert.Equal(t, selectedPayload[:len(chunk.GetChunk().GetData())], chunk.GetChunk().GetData())

		select {
		case <-repo.release:
			t.Fatal("completion barrier opened before client payload")
		default:
		}

		<-repo.started
		repo.assertDownloadRequest(t, selectedDescriptor.GetFileId())
		repo.releaseCompletion()

		received := append([]byte(nil), chunk.GetChunk().GetData()...)
		received = assertReceivedDownload(t, stream, received, selectedSize)
		assert.Equal(t, selectedPayload, received)
	})
}

func TestListMessageFiles_RealBufconnAlternativeVideoThenSelectDownload(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		const mainVideoFileID = 201
		const selectedAlternativeFileID = 203
		const hlsFileID = 204
		mainPayload := []byte("MAIN-VIDEO-PAYLOAD")
		selectedPayload := []byte("ALTERNATIVE-VIDEO-PAYLOAD")
		payloads := map[int32][]byte{
			mainVideoFileID:           mainPayload,
			selectedAlternativeFileID: selectedPayload,
			hlsFileID:                 []byte("hls-placeholder"),
		}
		selectedSize := int64(len(selectedPayload))

		repo := newSelectableFileRepo(t, payloads, true)
		defer repo.releaseCompletion()
		repo.EXPECT().GetMessage(&client.GetMessageRequest{ChatId: -100, MessageId: 7}).Return(&client.Message{
			Content: &client.MessageVideo{
				Video: &client.Video{Video: &client.File{Id: mainVideoFileID, Size: int64(len(mainPayload))}, MimeType: "video/mp4"},
				AlternativeVideos: []*client.AlternativeVideo{
					{Video: &client.File{Id: selectedAlternativeFileID, Size: selectedSize}, HlsFile: &client.File{Id: hlsFileID}, Codec: "h265"},
				},
			},
		}, nil).Once()

		service := facade.New(repo)
		clientConn := newBufconnFacadeClient(t, New(service))
		t.Cleanup(repo.releaseCompletion)

		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
		defer cancel()
		response, err := clientConn.ListMessageFiles(ctx, &pb.ListMessageFilesRequest{ChatId: -100, MessageId: 7})
		require.NoError(t, err)
		require.Len(t, response.Items, 1)
		descriptors := response.Items[0].Files
		require.Len(t, descriptors, 3)
		assert.Equal(t, int64(mainVideoFileID), descriptors[0].FileId)
		assert.Equal(t, pb.MessageFileRole_MESSAGE_FILE_ROLE_MAIN, descriptors[0].Role)
		assert.Equal(t, int64(hlsFileID), descriptors[2].FileId)
		assert.Equal(t, pb.MessageFileRole_MESSAGE_FILE_ROLE_HLS_PLAYLIST, descriptors[2].Role)
		assert.NotEqual(t, int64(response.DefaultFileId), int64(selectedAlternativeFileID))

		var selectedDescriptor *pb.MessageFileDescriptor
		for _, descriptor := range descriptors {
			if descriptor.GetFileId() == int64(selectedAlternativeFileID) {
				require.Nil(t, selectedDescriptor)
				require.Equal(t, pb.MessageFileRole_MESSAGE_FILE_ROLE_ALTERNATIVE_VIDEO, descriptor.GetRole())
				require.Equal(t, int32(0), descriptor.GetSourceIndex())
				require.Equal(t, "h265", descriptor.GetCodec())
				require.NotEqual(t, int64(response.DefaultFileId), descriptor.GetFileId())
				selectedDescriptor = descriptor
			}
		}
		require.NotNil(t, selectedDescriptor)

		repo.allowDownload(t)
		stream, err := clientConn.DownloadFile(ctx, &pb.DownloadFileRequest{FileId: selectedDescriptor.GetFileId()})
		require.NoError(t, err)

		firstEvent, err := stream.Recv()
		require.NoError(t, err, progressivePayloadDiagnostic)
		require.NotNil(t, firstEvent.GetMetadata())
		assert.Equal(t, selectedDescriptor.GetFileId(), firstEvent.GetMetadata().GetFileId())
		assert.True(t, firstEvent.GetMetadata().GetSizeKnown())
		assert.Equal(t, selectedSize, firstEvent.GetMetadata().GetSize())

		chunk, err := stream.Recv()
		require.NoError(t, err, progressivePayloadDiagnostic)
		require.NotNil(t, chunk.GetChunk())
		require.NotEmpty(t, chunk.GetChunk().GetData())
		assert.False(t, chunk.GetChunk().GetUpstreamComplete(), progressivePayloadDiagnostic)
		assert.Equal(t, int64(0), chunk.GetChunk().GetOffset())
		assert.Equal(t, selectedPayload[:len(chunk.GetChunk().GetData())], chunk.GetChunk().GetData())

		select {
		case <-repo.release:
			t.Fatal("completion barrier opened before client payload")
		default:
		}

		<-repo.started
		repo.assertDownloadRequest(t, selectedDescriptor.GetFileId())
		repo.releaseCompletion()

		received := append([]byte(nil), chunk.GetChunk().GetData()...)
		received = assertReceivedDownload(t, stream, received, selectedSize)
		assert.Equal(t, selectedPayload, received)
	})
}

func assertReceivedDownload(t *testing.T, stream grpc.ServerStreamingClient[pb.FileDownloadEvent], received []byte, size int64) []byte {
	t.Helper()
	completeReceived := false
	for {
		event, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err, progressivePayloadDiagnostic)
		if completeReceived {
			t.Fatal("event received after complete")
		}
		switch {
		case event.GetMetadata() != nil:
			t.Fatal("unexpected metadata event after first event")
		case event.GetChunk() != nil:
			require.NotEmpty(t, event.GetChunk().GetData())
			assert.Equal(t, int64(len(received)), event.GetChunk().GetOffset())
			received = append(received, event.GetChunk().GetData()...)
		case event.GetComplete() != nil:
			completeReceived = true
			assert.Equal(t, size, event.GetComplete().GetSize())
		default:
			t.Fatal("unexpected download event type")
		}
	}
	require.True(t, completeReceived, "missing terminal complete event")
	assert.Equal(t, size, int64(len(received)))
	return received
}
