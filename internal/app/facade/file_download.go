package facade

import (
	"context"
	"errors"
	"time"

	"github.com/zelenin/go-tdlib/client"
)

const (
	progressiveChunkSize = 256 * 1024
	progressivePollDelay = 250 * time.Millisecond
	progressivePriority  = 16
)

type FileDownloadMetadata struct {
	FileID    int32
	MIMEType  string
	Size      int64
	SizeKnown bool
}

type FileDownloadChunk struct {
	Offset           int64
	Data             []byte
	UpstreamComplete bool
}

type FileDownloadComplete struct {
	Size int64
}

type FileDownloadEvent struct {
	Metadata *FileDownloadMetadata
	Chunk    *FileDownloadChunk
	Complete *FileDownloadComplete
}

// GetMessageFile progressively streams the file contained in a message.
func (s *Service) GetMessageFile(ctx context.Context, chatID, messageID int64, emit func(FileDownloadEvent) error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	msg, err := s.telegramRepo.GetMessage(&client.GetMessageRequest{ChatId: chatID, MessageId: messageID})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if msg == nil {
		return NewMessageFileNotFoundError(chatID, messageID)
	}
	file, mimeType := resolveMessageFile(msg)
	if file == nil {
		return NewMessageFileNotFoundError(chatID, messageID)
	}
	return s.progressiveDownload(ctx, file.Id, mimeType, emit)
}

// DownloadFile progressively streams a caller-supplied TDLib file ID.
func (s *Service) DownloadFile(ctx context.Context, fileID int32, emit func(FileDownloadEvent) error) error {
	return s.progressiveDownload(ctx, fileID, "", emit)
}

func resolveMessageFile(msg *client.Message) (*client.File, string) {
	if msg == nil {
		return nil, ""
	}
	var file *client.File
	var mimeType string
	switch content := msg.Content.(type) {
	case *client.MessagePhoto:
		if content != nil {
			mimeType = "image/jpeg"
			if content.Photo != nil {
				for _, size := range content.Photo.Sizes {
					if size != nil && size.Photo != nil && (file == nil || size.Photo.Size >= file.Size) {
						file = size.Photo
					}
				}
			}
		}
	case *client.MessageVideo:
		if content != nil && content.Video != nil {
			file = content.Video.Video
			mimeType = content.Video.MimeType
		}
	case *client.MessageDocument:
		if content != nil && content.Document != nil {
			file = content.Document.Document
			mimeType = content.Document.MimeType
		}
	case *client.MessageAudio:
		if content != nil && content.Audio != nil {
			file = content.Audio.Audio
			mimeType = content.Audio.MimeType
		}
	case *client.MessageVoiceNote:
		if content != nil && content.VoiceNote != nil {
			file = content.VoiceNote.Voice
			mimeType = content.VoiceNote.MimeType
		}
	case *client.MessageAnimation:
		if content != nil && content.Animation != nil {
			file = content.Animation.Animation
			mimeType = content.Animation.MimeType
		}
	case *client.MessageVideoNote:
		if content != nil && content.VideoNote != nil {
			file = content.VideoNote.Video
		}
	case *client.MessageSticker:
		if content != nil && content.Sticker != nil {
			file = content.Sticker.Sticker
		}
	}
	if file == nil {
		return nil, ""
	}
	return file, mimeType
}

func (s *Service) progressiveDownload(ctx context.Context, requestedFileID int32, mimeType string, emit func(FileDownloadEvent) error) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	started, err := s.telegramRepo.DownloadFile(&client.DownloadFileRequest{
		FileId:      requestedFileID,
		Priority:    progressivePriority,
		Offset:      0,
		Limit:       0,
		Synchronous: false,
	})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return NewFileDownloadFailedError(requestedFileID, err)
	}
	if started == nil || started.Local == nil || started.Id <= 0 || started.Size < 0 {
		return NewFileDownloadFailedError(requestedFileID, errors.New("download did not return a valid file state"))
	}
	fileID := started.Id

	metadataSize, metadataSizeKnown := downloadMetadataSize(started)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err := emit(FileDownloadEvent{Metadata: &FileDownloadMetadata{
		FileID:    fileID,
		MIMEType:  mimeType,
		Size:      metadataSize,
		SizeKnown: metadataSizeKnown,
	}}); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}

	var cursor int64
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		file, err := s.telegramRepo.GetFile(&client.GetFileRequest{FileId: fileID})
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return NewFileDownloadFailedError(fileID, err)
		}
		if file == nil || file.Local == nil || file.Size < 0 {
			return NewFileDownloadFailedError(fileID, errors.New("file state is invalid"))
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		complete := file.Local.IsDownloadingCompleted
		if complete && file.Size > 0 && (cursor > file.Size || cursor < 0) {
			return NewFileDownloadFailedError(fileID, errors.New("cursor is outside the completed file"))
		}

		prefix, err := s.telegramRepo.GetFileDownloadedPrefixSize(&client.GetFileDownloadedPrefixSizeRequest{FileId: fileID, Offset: cursor})
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return NewFileDownloadFailedError(fileID, err)
		}
		if prefix == nil || prefix.Size < 0 {
			return NewFileDownloadFailedError(fileID, errors.New("downloaded prefix state is invalid"))
		}
		if complete && file.Size > 0 && cursor+prefix.Size > file.Size {
			return NewFileDownloadFailedError(fileID, errors.New("downloaded prefix exceeds completed file"))
		}
		if cursor+prefix.Size < cursor {
			return NewFileDownloadFailedError(fileID, errors.New("downloaded prefix overflows"))
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		if prefix.Size > 0 {
			count := min(prefix.Size, int64(progressiveChunkSize))
			part, err := s.telegramRepo.ReadFilePart(&client.ReadFilePartRequest{FileId: fileID, Offset: cursor, Count: count})
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err != nil {
				return NewFileDownloadFailedError(fileID, err)
			}
			if part == nil || int64(len(part.Data)) != count {
				return NewFileDownloadFailedError(fileID, errors.New("file part length does not match requested range"))
			}
			chunk := FileDownloadChunk{Offset: cursor, Data: part.Data, UpstreamComplete: complete}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err := emit(FileDownloadEvent{Chunk: &chunk}); err != nil {
				return err
			}
			cursor += int64(len(chunk.Data))
			if ctx.Err() != nil {
				return ctx.Err()
			}
			continue
		}

		if complete {
			if file.Size > 0 && cursor != file.Size {
				return NewFileDownloadFailedError(fileID, errors.New("completed file size does not match streamed bytes"))
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err := emit(FileDownloadEvent{Complete: &FileDownloadComplete{Size: cursor}}); err != nil {
				return err
			}
			return ctx.Err()
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if file.Local.IsDownloadingActive {
			timer := time.NewTimer(progressivePollDelay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			continue
		}
		return NewFileDownloadFailedError(fileID, errors.New("download stopped before any readable remainder"))
	}
}

func downloadMetadataSize(file *client.File) (int64, bool) {
	if file.Size > 0 {
		return file.Size, true
	}
	if file.Local.IsDownloadingCompleted {
		return file.Size, true
	}
	return 0, false
}
