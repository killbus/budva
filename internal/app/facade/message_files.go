package facade

import (
	"context"

	"github.com/zelenin/go-tdlib/client"
)

type MessageMediaKind int32

const (
	MessageMediaKindUnspecified MessageMediaKind = 0
	MessageMediaKindPhoto       MessageMediaKind = 1
	MessageMediaKindVideo       MessageMediaKind = 2
	MessageMediaKindDocument    MessageMediaKind = 3
	MessageMediaKindAudio       MessageMediaKind = 4
	MessageMediaKindVoiceNote   MessageMediaKind = 5
	MessageMediaKindAnimation   MessageMediaKind = 6
	MessageMediaKindVideoNote   MessageMediaKind = 7
	MessageMediaKindSticker     MessageMediaKind = 8
	MessageMediaKindPreview     MessageMediaKind = 9
	MessageMediaKindUnsupported MessageMediaKind = 10
)

type MessageMediaState int32

const (
	MessageMediaStateUnspecified  MessageMediaState = 0
	MessageMediaStateFilesPresent MessageMediaState = 1
	MessageMediaStateLocked       MessageMediaState = 2
	MessageMediaStateUnsupported  MessageMediaState = 3
	MessageMediaStateNoFiles      MessageMediaState = 4
)

type MessageFileRole int32

const (
	MessageFileRoleUnspecified        MessageFileRole = 0
	MessageFileRoleMain               MessageFileRole = 1
	MessageFileRolePhotoSize          MessageFileRole = 2
	MessageFileRoleAlternativeVideo   MessageFileRole = 3
	MessageFileRoleHLSPlaylist        MessageFileRole = 4
	MessageFileRoleThumbnail          MessageFileRole = 5
	MessageFileRoleAlbumCover         MessageFileRole = 6
	MessageFileRoleExternalAlbumCover MessageFileRole = 7
	MessageFileRolePremiumAnimation   MessageFileRole = 8
)

type MessageFileDescriptor struct {
	FileID        int32
	Role          MessageFileRole
	SourceIndex   int32
	FileName      string
	MIMEType      string
	Codec         string
	PhotoSizeType string
	Size          int64
	SizeKnown     bool
	Width         int32
	Height        int32
}

type MessageMediaItem struct {
	ItemIndex int32
	Kind      MessageMediaKind
	State     MessageMediaState
	Files     []MessageFileDescriptor
}

type MessageFiles struct {
	ChatID           int64
	MessageID        int64
	MediaAlbumID     int64
	ContentSupported bool
	Items            []MessageMediaItem
	DefaultFileID    int32
}

func (s *Service) ListMessageFiles(ctx context.Context, chatID, messageID int64) (*MessageFiles, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	msg, err := s.telegramRepo.GetMessage(&client.GetMessageRequest{ChatId: chatID, MessageId: messageID})
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, err
	}
	if msg == nil {
		return nil, NewMessageNotFoundError(chatID, messageID)
	}
	result := enumerateMessageFiles(msg, chatID, messageID)
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return result, nil
}

func enumerateMessageFiles(msg *client.Message, chatID, messageID int64) *MessageFiles {
	result := &MessageFiles{
		ChatID:    chatID,
		MessageID: messageID,
	}
	if msg == nil {
		return result
	}
	result.MediaAlbumID = int64(msg.MediaAlbumId)
	result.Items, result.ContentSupported = messageMediaItems(msg.Content)
	if result.ContentSupported {
		defaultFileID := messageDefaultFileID(msg)
		if hasFileID(result.Items, defaultFileID) {
			result.DefaultFileID = defaultFileID
		}
	}
	return result
}

func messageMediaItems(content client.MessageContent) ([]MessageMediaItem, bool) {
	switch media := content.(type) {
	case *client.MessagePhoto:
		if media == nil {
			return emptyMediaItems(MessageMediaKindPhoto), true
		}
		item := photoMediaItem(media.Photo)
		return []MessageMediaItem{item}, true
	case *client.MessageVideo:
		if media == nil {
			return emptyMediaItems(MessageMediaKindVideo), true
		}
		return []MessageMediaItem{videoMediaItem(media.Video, media.AlternativeVideos)}, true
	case *client.MessageDocument:
		if media == nil {
			return emptyMediaItems(MessageMediaKindDocument), true
		}
		return []MessageMediaItem{documentMediaItem(media.Document)}, true
	case *client.MessageAudio:
		if media == nil {
			return emptyMediaItems(MessageMediaKindAudio), true
		}
		return []MessageMediaItem{audioMediaItem(media.Audio)}, true
	case *client.MessageVoiceNote:
		if media == nil {
			return emptyMediaItems(MessageMediaKindVoiceNote), true
		}
		return []MessageMediaItem{voiceNoteMediaItem(media.VoiceNote)}, true
	case *client.MessageAnimation:
		if media == nil {
			return emptyMediaItems(MessageMediaKindAnimation), true
		}
		return []MessageMediaItem{animationMediaItem(media.Animation)}, true
	case *client.MessageVideoNote:
		if media == nil {
			return emptyMediaItems(MessageMediaKindVideoNote), true
		}
		return []MessageMediaItem{videoNoteMediaItem(media.VideoNote)}, true
	case *client.MessageSticker:
		if media == nil {
			return emptyMediaItems(MessageMediaKindSticker), true
		}
		return []MessageMediaItem{stickerMediaItem(media.Sticker)}, true
	case *client.MessagePaidMedia:
		if media == nil {
			return nil, true
		}
		return paidMediaItems(media.Media), true
	case nil:
		return nil, false
	default:
		return nil, false
	}
}

func emptyMediaItems(kind MessageMediaKind) []MessageMediaItem {
	return []MessageMediaItem{{Kind: kind, State: MessageMediaStateNoFiles}}
}

func paidMediaItems(media []client.PaidMedia) []MessageMediaItem {
	items := make([]MessageMediaItem, 0, len(media))
	for index, paid := range media {
		var item MessageMediaItem
		switch paid := paid.(type) {
		case *client.PaidMediaPhoto:
			if paid == nil {
				item = unsupportedMediaItem(index)
			} else {
				item = photoMediaItem(paid.Photo)
				item.ItemIndex = int32(index)
			}
		case *client.PaidMediaVideo:
			if paid == nil {
				item = unsupportedMediaItem(index)
			} else {
				item = paidVideoMediaItem(paid.Video)
				item.ItemIndex = int32(index)
			}
		case *client.PaidMediaPreview:
			if paid == nil {
				item = unsupportedMediaItem(index)
			} else {
				item = MessageMediaItem{
					ItemIndex: int32(index),
					Kind:      MessageMediaKindPreview,
					State:     MessageMediaStateLocked,
				}
			}
		case *client.PaidMediaUnsupported:
			if paid == nil {
				item = unsupportedMediaItem(index)
			} else {
				item = MessageMediaItem{
					ItemIndex: int32(index),
					Kind:      MessageMediaKindUnsupported,
					State:     MessageMediaStateUnsupported,
				}
			}
		case nil:
			item = unsupportedMediaItem(index)
		default:
			item = unsupportedMediaItem(index)
		}
		items = append(items, item)
	}
	return items
}

func photoMediaItem(photo *client.Photo) MessageMediaItem {
	item := MessageMediaItem{Kind: MessageMediaKindPhoto}
	if photo != nil {
		for index, size := range photo.Sizes {
			if size == nil {
				continue
			}
			item.Files = appendFileReference(item.Files, size.Photo, MessageFileRolePhotoSize, int32(index), fileMetadata{
				width: size.Width, height: size.Height, photoSizeType: size.Type, mimeType: "image/jpeg",
			})
		}
	}
	if len(item.Files) > 0 {
		item.State = MessageMediaStateFilesPresent
	} else {
		item.State = MessageMediaStateNoFiles
	}
	return item
}

func videoMediaItem(video *client.Video, alternatives []*client.AlternativeVideo) MessageMediaItem {
	item := MessageMediaItem{Kind: MessageMediaKindVideo}
	if video != nil {
		item.Files = appendFileReference(item.Files, video.Video, MessageFileRoleMain, 0, fileMetadata{
			fileName: video.FileName, mimeType: video.MimeType, width: video.Width, height: video.Height,
		})
		if video.Thumbnail != nil {
			item.Files = appendThumbnail(item.Files, video.Thumbnail, 0)
		}
	}
	for index, alternative := range alternatives {
		if alternative == nil {
			continue
		}
		metadata := fileMetadata{width: alternative.Width, height: alternative.Height, codec: alternative.Codec}
		item.Files = appendFileReference(item.Files, alternative.Video, MessageFileRoleAlternativeVideo, int32(index), metadata)
		item.Files = appendFileReference(item.Files, alternative.HlsFile, MessageFileRoleHLSPlaylist, int32(index), metadata)
	}
	return itemState(item)
}

func documentMediaItem(document *client.Document) MessageMediaItem {
	item := MessageMediaItem{Kind: MessageMediaKindDocument}
	if document != nil {
		item.Files = appendFileReference(item.Files, document.Document, MessageFileRoleMain, 0, fileMetadata{
			fileName: document.FileName, mimeType: document.MimeType,
		})
		if document.Thumbnail != nil {
			item.Files = appendThumbnail(item.Files, document.Thumbnail, 0)
		}
	}
	return itemState(item)
}

func audioMediaItem(audio *client.Audio) MessageMediaItem {
	item := MessageMediaItem{Kind: MessageMediaKindAudio}
	if audio != nil {
		item.Files = appendFileReference(item.Files, audio.Audio, MessageFileRoleMain, 0, fileMetadata{
			fileName: audio.FileName, mimeType: audio.MimeType,
		})
		if audio.AlbumCoverThumbnail != nil {
			item.Files = appendFileReference(item.Files, audio.AlbumCoverThumbnail.File, MessageFileRoleAlbumCover, 0, thumbnailMetadata(audio.AlbumCoverThumbnail))
		}
		for index, cover := range audio.ExternalAlbumCovers {
			if cover == nil {
				continue
			}
			item.Files = appendFileReference(item.Files, cover.File, MessageFileRoleExternalAlbumCover, int32(index), thumbnailMetadata(cover))
		}
	}
	return itemState(item)
}

func voiceNoteMediaItem(voiceNote *client.VoiceNote) MessageMediaItem {
	item := MessageMediaItem{Kind: MessageMediaKindVoiceNote}
	if voiceNote != nil {
		item.Files = appendFileReference(item.Files, voiceNote.Voice, MessageFileRoleMain, 0, fileMetadata{mimeType: voiceNote.MimeType})
	}
	return itemState(item)
}

func animationMediaItem(animation *client.Animation) MessageMediaItem {
	item := MessageMediaItem{Kind: MessageMediaKindAnimation}
	if animation != nil {
		item.Files = appendFileReference(item.Files, animation.Animation, MessageFileRoleMain, 0, fileMetadata{
			fileName: animation.FileName, mimeType: animation.MimeType, width: animation.Width, height: animation.Height,
		})
		if animation.Thumbnail != nil {
			item.Files = appendThumbnail(item.Files, animation.Thumbnail, 0)
		}
	}
	return itemState(item)
}

func videoNoteMediaItem(videoNote *client.VideoNote) MessageMediaItem {
	item := MessageMediaItem{Kind: MessageMediaKindVideoNote}
	if videoNote != nil {
		item.Files = appendFileReference(item.Files, videoNote.Video, MessageFileRoleMain, 0, fileMetadata{
			width: videoNote.Length, height: videoNote.Length,
		})
		if videoNote.Thumbnail != nil {
			item.Files = appendThumbnail(item.Files, videoNote.Thumbnail, 0)
		}
	}
	return itemState(item)
}

func stickerMediaItem(sticker *client.Sticker) MessageMediaItem {
	item := MessageMediaItem{Kind: MessageMediaKindSticker}
	if sticker != nil {
		item.Files = appendFileReference(item.Files, sticker.Sticker, MessageFileRoleMain, 0, fileMetadata{
			width: sticker.Width, height: sticker.Height,
		})
		if sticker.Thumbnail != nil {
			item.Files = appendThumbnail(item.Files, sticker.Thumbnail, 0)
		}
		if regular, ok := sticker.FullType.(*client.StickerFullTypeRegular); ok && regular != nil {
			item.Files = appendFileReference(item.Files, regular.PremiumAnimation, MessageFileRolePremiumAnimation, 0, fileMetadata{})
		}
	}
	return itemState(item)
}

func paidVideoMediaItem(video *client.Video) MessageMediaItem {
	item := MessageMediaItem{Kind: MessageMediaKindVideo}
	if video != nil {
		item.Files = appendFileReference(item.Files, video.Video, MessageFileRoleMain, 0, fileMetadata{
			fileName: video.FileName, mimeType: video.MimeType, width: video.Width, height: video.Height,
		})
		if video.Thumbnail != nil {
			item.Files = appendThumbnail(item.Files, video.Thumbnail, 0)
		}
	}
	return itemState(item)
}

func unsupportedMediaItem(index int) MessageMediaItem {
	return MessageMediaItem{
		ItemIndex: int32(index),
		Kind:      MessageMediaKindUnsupported,
		State:     MessageMediaStateUnsupported,
	}
}

func itemState(item MessageMediaItem) MessageMediaItem {
	if len(item.Files) > 0 {
		item.State = MessageMediaStateFilesPresent
	} else {
		item.State = MessageMediaStateNoFiles
	}
	return item
}

func appendThumbnail(files []MessageFileDescriptor, thumbnail *client.Thumbnail, sourceIndex int32) []MessageFileDescriptor {
	if thumbnail == nil {
		return files
	}
	return appendFileReference(files, thumbnail.File, MessageFileRoleThumbnail, sourceIndex, thumbnailMetadata(thumbnail))
}

type fileMetadata struct {
	fileName      string
	mimeType      string
	codec         string
	photoSizeType string
	width         int32
	height        int32
}

func thumbnailMetadata(thumbnail *client.Thumbnail) fileMetadata {
	if thumbnail == nil {
		return fileMetadata{}
	}
	return fileMetadata{width: thumbnail.Width, height: thumbnail.Height}
}

func appendFileReference(files []MessageFileDescriptor, file *client.File, role MessageFileRole, sourceIndex int32, metadata fileMetadata) []MessageFileDescriptor {
	if file == nil || file.Id <= 0 {
		return files
	}
	size, sizeKnown := int64(0), false
	if file.Size > 0 {
		size, sizeKnown = file.Size, true
	}
	return append(files, MessageFileDescriptor{
		FileID:        file.Id,
		Role:          role,
		SourceIndex:   sourceIndex,
		FileName:      metadata.fileName,
		MIMEType:      metadata.mimeType,
		Codec:         metadata.codec,
		PhotoSizeType: metadata.photoSizeType,
		Size:          size,
		SizeKnown:     sizeKnown,
		Width:         max(metadata.width, 0),
		Height:        max(metadata.height, 0),
	})
}

func hasFileID(items []MessageMediaItem, fileID int32) bool {
	if fileID <= 0 {
		return false
	}
	for _, item := range items {
		for _, file := range item.Files {
			if file.FileID == fileID {
				return true
			}
		}
	}
	return false
}

func messageDefaultFileID(msg *client.Message) int32 {
	file, _ := resolveMessageFile(msg)
	if file == nil {
		return 0
	}
	return file.Id
}
