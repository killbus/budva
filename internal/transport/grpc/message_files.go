package grpc

import (
	"context"
	"errors"
	"log/slog"

	alogger "github.com/pure-golang/adapters/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/pure-golang/budva-claude/internal/app/facade"
	"github.com/pure-golang/budva-claude/internal/transport/grpc/pb"
)

func mapListMessageFilesError(err error) error {
	if errors.Is(err, context.Canceled) {
		return status.Error(codes.Canceled, err.Error())
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return status.Error(codes.DeadlineExceeded, err.Error())
	}
	return mapFacadeError(err)
}

func mapMessageMediaKind(kind facade.MessageMediaKind) pb.MessageMediaKind {
	switch kind {
	case facade.MessageMediaKindPhoto:
		return pb.MessageMediaKind_MESSAGE_MEDIA_KIND_PHOTO
	case facade.MessageMediaKindVideo:
		return pb.MessageMediaKind_MESSAGE_MEDIA_KIND_VIDEO
	case facade.MessageMediaKindDocument:
		return pb.MessageMediaKind_MESSAGE_MEDIA_KIND_DOCUMENT
	case facade.MessageMediaKindAudio:
		return pb.MessageMediaKind_MESSAGE_MEDIA_KIND_AUDIO
	case facade.MessageMediaKindVoiceNote:
		return pb.MessageMediaKind_MESSAGE_MEDIA_KIND_VOICE_NOTE
	case facade.MessageMediaKindAnimation:
		return pb.MessageMediaKind_MESSAGE_MEDIA_KIND_ANIMATION
	case facade.MessageMediaKindVideoNote:
		return pb.MessageMediaKind_MESSAGE_MEDIA_KIND_VIDEO_NOTE
	case facade.MessageMediaKindSticker:
		return pb.MessageMediaKind_MESSAGE_MEDIA_KIND_STICKER
	case facade.MessageMediaKindPreview:
		return pb.MessageMediaKind_MESSAGE_MEDIA_KIND_PREVIEW
	case facade.MessageMediaKindUnsupported:
		return pb.MessageMediaKind_MESSAGE_MEDIA_KIND_UNSUPPORTED
	default:
		return pb.MessageMediaKind_MESSAGE_MEDIA_KIND_UNSPECIFIED
	}
}

func mapMessageMediaState(state facade.MessageMediaState) pb.MessageMediaState {
	switch state {
	case facade.MessageMediaStateFilesPresent:
		return pb.MessageMediaState_MESSAGE_MEDIA_STATE_FILES_PRESENT
	case facade.MessageMediaStateLocked:
		return pb.MessageMediaState_MESSAGE_MEDIA_STATE_LOCKED
	case facade.MessageMediaStateUnsupported:
		return pb.MessageMediaState_MESSAGE_MEDIA_STATE_UNSUPPORTED
	case facade.MessageMediaStateNoFiles:
		return pb.MessageMediaState_MESSAGE_MEDIA_STATE_NO_FILES
	default:
		return pb.MessageMediaState_MESSAGE_MEDIA_STATE_UNSPECIFIED
	}
}

func mapMessageFileRole(role facade.MessageFileRole) pb.MessageFileRole {
	switch role {
	case facade.MessageFileRoleMain:
		return pb.MessageFileRole_MESSAGE_FILE_ROLE_MAIN
	case facade.MessageFileRolePhotoSize:
		return pb.MessageFileRole_MESSAGE_FILE_ROLE_PHOTO_SIZE
	case facade.MessageFileRoleAlternativeVideo:
		return pb.MessageFileRole_MESSAGE_FILE_ROLE_ALTERNATIVE_VIDEO
	case facade.MessageFileRoleHLSPlaylist:
		return pb.MessageFileRole_MESSAGE_FILE_ROLE_HLS_PLAYLIST
	case facade.MessageFileRoleThumbnail:
		return pb.MessageFileRole_MESSAGE_FILE_ROLE_THUMBNAIL
	case facade.MessageFileRoleAlbumCover:
		return pb.MessageFileRole_MESSAGE_FILE_ROLE_ALBUM_COVER
	case facade.MessageFileRoleExternalAlbumCover:
		return pb.MessageFileRole_MESSAGE_FILE_ROLE_EXTERNAL_ALBUM_COVER
	case facade.MessageFileRolePremiumAnimation:
		return pb.MessageFileRole_MESSAGE_FILE_ROLE_PREMIUM_ANIMATION
	default:
		return pb.MessageFileRole_MESSAGE_FILE_ROLE_UNSPECIFIED
	}
}

func messageFileDescriptorToProto(descriptor facade.MessageFileDescriptor) *pb.MessageFileDescriptor {
	return &pb.MessageFileDescriptor{
		FileId:        int64(descriptor.FileID),
		Role:          mapMessageFileRole(descriptor.Role),
		SourceIndex:   descriptor.SourceIndex,
		FileName:      descriptor.FileName,
		MimeType:      descriptor.MIMEType,
		Size:          descriptor.Size,
		SizeKnown:     descriptor.SizeKnown,
		Width:         descriptor.Width,
		Height:        descriptor.Height,
		Codec:         descriptor.Codec,
		PhotoSizeType: descriptor.PhotoSizeType,
	}
}

func messageMediaItemToProto(item facade.MessageMediaItem) *pb.MessageMediaItem {
	files := make([]*pb.MessageFileDescriptor, 0, len(item.Files))
	for _, file := range item.Files {
		files = append(files, messageFileDescriptorToProto(file))
	}
	return &pb.MessageMediaItem{
		ItemIndex: item.ItemIndex,
		Kind:      mapMessageMediaKind(item.Kind),
		State:     mapMessageMediaState(item.State),
		Files:     files,
	}
}

func messageFilesToProto(files *facade.MessageFiles) *pb.ListMessageFilesResponse {
	if files == nil {
		return nil
	}
	items := make([]*pb.MessageMediaItem, 0, len(files.Items))
	for _, item := range files.Items {
		items = append(items, messageMediaItemToProto(item))
	}
	return &pb.ListMessageFilesResponse{
		ChatId:           files.ChatID,
		MessageId:        files.MessageID,
		MediaAlbumId:     files.MediaAlbumID,
		ContentSupported: files.ContentSupported,
		Items:            items,
		DefaultFileId:    int64(files.DefaultFileID),
	}
}

func (t *Transport) ListMessageFiles(ctx context.Context, req *pb.ListMessageFilesRequest) (*pb.ListMessageFilesResponse, error) {
	if req.GetChatId() == 0 || req.GetMessageId() <= 0 {
		return nil, status.Error(codes.InvalidArgument, "chat_id must be non-zero and message_id must be positive")
	}
	files, err := t.facadeService.ListMessageFiles(ctx, req.GetChatId(), req.GetMessageId())
	if err != nil {
		alogger.FromContext(ctx).Error("Failed to list message files", slog.Any("err", err))
		return nil, mapListMessageFilesError(err)
	}
	return messageFilesToProto(files), nil
}
