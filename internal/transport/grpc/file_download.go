package grpc

import (
	"context"
	"errors"
	"log/slog"
	"math"

	alogger "github.com/pure-golang/adapters/logger"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pure-golang/budva-claude/internal/app/facade"
	"github.com/pure-golang/budva-claude/internal/infra/telegram"
	"github.com/pure-golang/budva-claude/internal/transport/grpc/pb"
)

func mapStreamingError(err error) error {
	var floodWait *telegram.FloodWaitError
	if errors.As(err, &floodWait) {
		return mapFacadeError(err)
	}
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, err.Error())
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, err.Error())
	}
	var fileDownloadFailed *facade.FileDownloadFailedError
	if errors.As(err, &fileDownloadFailed) {
		return status.Errorf(codes.Internal, "download file %d failed", fileDownloadFailed.FileID)
	}
	return mapFacadeError(err)
}

var _ pb.FacadeGRPCServer = (*Transport)(nil)

func mapFileDownloadEvent(event facade.FileDownloadEvent) (*pb.FileDownloadEvent, error) {
	nonNil := 0
	if event.Metadata != nil {
		nonNil++
	}
	if event.Chunk != nil {
		nonNil++
	}
	if event.Complete != nil {
		nonNil++
	}
	if nonNil != 1 {
		return nil, status.Error(codes.Internal, "file download produced an invalid event")
	}
	switch {
	case event.Metadata != nil:
		return &pb.FileDownloadEvent{
			Event: &pb.FileDownloadEvent_Metadata{
				Metadata: &pb.FileDownloadMetadata{
					FileId:    int64(event.Metadata.FileID),
					MimeType:  event.Metadata.MIMEType,
					Size:      event.Metadata.Size,
					SizeKnown: event.Metadata.SizeKnown,
				},
			},
		}, nil
	case event.Chunk != nil:
		return &pb.FileDownloadEvent{
			Event: &pb.FileDownloadEvent_Chunk{
				Chunk: &pb.FileDownloadChunk{
					Offset:           event.Chunk.Offset,
					Data:             event.Chunk.Data,
					UpstreamComplete: event.Chunk.UpstreamComplete,
				},
			},
		}, nil
	default:
		return &pb.FileDownloadEvent{
			Event: &pb.FileDownloadEvent_Complete{
				Complete: &pb.FileDownloadComplete{Size: event.Complete.Size},
			},
		}, nil
	}
}

func streamFileDownload(
	ctx context.Context,
	send func(*pb.FileDownloadEvent) error,
	start func(emit func(facade.FileDownloadEvent) error) error,
) error {
	var sendErr error
	err := start(func(event facade.FileDownloadEvent) error {
		if sendErr != nil {
			return errors.New("stream send already failed")
		}
		pbEvent, mapErr := mapFileDownloadEvent(event)
		if mapErr != nil {
			return mapErr
		}
		sendErr = send(pbEvent)
		return sendErr
	})
	if sendErr != nil {
		return sendErr
	}
	if err == nil {
		return nil
	}
	return mapStreamingError(err)
}

func (t *Transport) GetMessageFile(req *pb.GetMessageFileRequest, stream grpc.ServerStreamingServer[pb.FileDownloadEvent]) error {
	ctx := stream.Context()
	if req.GetChatId() == 0 || req.GetMessageId() <= 0 {
		return status.Error(codes.InvalidArgument, "chat_id must be non-zero and message_id must be positive")
	}
	err := streamFileDownload(ctx, stream.Send, func(emit func(facade.FileDownloadEvent) error) error {
		return t.facadeService.GetMessageFile(ctx, req.GetChatId(), req.GetMessageId(), emit)
	})
	if err != nil {
		alogger.FromContext(ctx).Error("Failed to stream message file", slog.Any("err", err))
	}
	return err
}

func (t *Transport) DownloadFile(req *pb.DownloadFileRequest, stream grpc.ServerStreamingServer[pb.FileDownloadEvent]) error {
	ctx := stream.Context()
	if req.GetFileId() <= 0 || req.GetFileId() > math.MaxInt32 {
		return status.Error(codes.InvalidArgument, "file_id must be within [1, MaxInt32]")
	}
	err := streamFileDownload(ctx, stream.Send, func(emit func(facade.FileDownloadEvent) error) error {
		return t.facadeService.DownloadFile(ctx, int32(req.GetFileId()), emit)
	})
	if err != nil {
		alogger.FromContext(ctx).Error("Failed to stream file", slog.Any("err", err))
	}
	return err
}
