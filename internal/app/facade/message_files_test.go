package facade

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/app/facade/mocks"
)

func photoFixture(fileID int32, size int64, width, height int32, photoType string) *client.PhotoSize {
	return &client.PhotoSize{
		Photo:  &client.File{Id: fileID, Size: size, ExpectedSize: size * 100},
		Type:   photoType,
		Width:  width,
		Height: height,
	}
}

func TestService_ListMessageFiles_PhotoSizesMetadataOnly(t *testing.T) {
	t.Parallel()

	gw := mocks.NewTelegramRepo(t)
	svc := New(gw)
	gw.EXPECT().GetMessage(&client.GetMessageRequest{ChatId: -100, MessageId: 7}).Return(&client.Message{
		Id:           7,
		ChatId:       -100,
		MediaAlbumId: 42,
		Content: &client.MessagePhoto{Photo: &client.Photo{Sizes: []*client.PhotoSize{
			photoFixture(101, 1000, 100, 100, "s"),
			photoFixture(102, 2000, 200, 200, "m"),
			photoFixture(103, 3000, 300, 300, "x"),
		}}},
	}, nil).Once()

	result, err := svc.ListMessageFiles(context.Background(), -100, 7)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int64(-100), result.ChatID)
	assert.Equal(t, int64(7), result.MessageID)
	assert.Equal(t, int64(42), result.MediaAlbumID)
	assert.True(t, result.ContentSupported)
	assert.Equal(t, int32(103), result.DefaultFileID)
	require.Len(t, result.Items, 1)
	item := result.Items[0]
	assert.Equal(t, int32(0), item.ItemIndex)
	assert.Equal(t, MessageMediaKindPhoto, item.Kind)
	assert.Equal(t, MessageMediaStateFilesPresent, item.State)
	require.Len(t, item.Files, 3)
	for index, file := range item.Files {
		assert.Equal(t, int32(101+index), file.FileID)
		assert.Equal(t, MessageFileRolePhotoSize, file.Role)
		assert.Equal(t, int32(index), file.SourceIndex)
		assert.Equal(t, "image/jpeg", file.MIMEType)
		assert.True(t, file.SizeKnown)
		assert.Equal(t, int64(1000*(index+1)), file.Size)
		assert.Equal(t, int32(100*(index+1)), file.Width)
		assert.Equal(t, int32(100*(index+1)), file.Height)
		assert.Equal(t, []string{"s", "m", "x"}[index], file.PhotoSizeType)
		assert.Empty(t, file.FileName)
		assert.Empty(t, file.Codec)
	}
}

func TestService_ListMessageFiles_PhotoMutationBindsExtraction(t *testing.T) {
	t.Parallel()

	gw := mocks.NewTelegramRepo(t)
	svc := New(gw)
	gw.EXPECT().GetMessage(mock.Anything).Return(&client.Message{
		Content: &client.MessagePhoto{Photo: &client.Photo{Sizes: []*client.PhotoSize{
			photoFixture(101, 1000, 100, 100, "s"),
			photoFixture(102, 2000, 200, 200, "m"),
			photoFixture(103, 3000, 300, 300, "x"),
		}}},
	}, nil).Once()

	result, err := svc.ListMessageFiles(context.Background(), 100, 7)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	require.Len(t, result.Items[0].Files, 3)
	assert.Equal(t, int32(103), result.DefaultFileID)
}

func TestService_ListMessageFiles_VideoAlternativesAndHLS(t *testing.T) {
	t.Parallel()

	gw := mocks.NewTelegramRepo(t)
	svc := New(gw)
	gw.EXPECT().GetMessage(mock.Anything).Return(&client.Message{
		Content: &client.MessageVideo{
			Video: &client.Video{
				Video:    &client.File{Id: 201, Size: 10000},
				FileName: "main.mp4",
				MimeType: "video/mp4",
				Width:    1920,
				Height:   1080,
				Thumbnail: &client.Thumbnail{
					File:   &client.File{Id: 202, Size: 100},
					Width:  100,
					Height: 50,
				},
			},
			AlternativeVideos: []*client.AlternativeVideo{
				nil,
				{
					Video:   &client.File{Id: 203, Size: 5000},
					HlsFile: &client.File{Id: 204},
					Width:   640,
					Height:  360,
					Codec:   "h264",
				},
			},
		},
	}, nil).Once()

	result, err := svc.ListMessageFiles(context.Background(), 100, 7)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	item := result.Items[0]
	assert.Equal(t, MessageMediaKindVideo, item.Kind)
	assert.Equal(t, MessageMediaStateFilesPresent, item.State)
	require.Len(t, item.Files, 4)

	assert.Equal(t, int32(201), item.Files[0].FileID)
	assert.Equal(t, MessageFileRoleMain, item.Files[0].Role)
	assert.Equal(t, "main.mp4", item.Files[0].FileName)
	assert.Equal(t, "video/mp4", item.Files[0].MIMEType)
	assert.Empty(t, item.Files[0].Codec)
	assert.Equal(t, int32(1920), item.Files[0].Width)

	assert.Equal(t, int32(202), item.Files[1].FileID)
	assert.Equal(t, MessageFileRoleThumbnail, item.Files[1].Role)
	assert.Equal(t, int32(100), item.Files[1].Width)
	assert.Empty(t, item.Files[1].MIMEType)

	assert.Equal(t, int32(203), item.Files[2].FileID)
	assert.Equal(t, MessageFileRoleAlternativeVideo, item.Files[2].Role)
	assert.Equal(t, int32(1), item.Files[2].SourceIndex)
	assert.Equal(t, int32(640), item.Files[2].Width)
	assert.Equal(t, "h264", item.Files[2].Codec)
	assert.Empty(t, item.Files[2].MIMEType)

	assert.Equal(t, int32(204), item.Files[3].FileID)
	assert.Equal(t, MessageFileRoleHLSPlaylist, item.Files[3].Role)
	assert.Equal(t, int32(1), item.Files[3].SourceIndex)
	assert.Equal(t, "h264", item.Files[3].Codec)

	assert.Equal(t, int32(201), result.DefaultFileID)
}

func TestService_ListMessageFiles_AllSupportedTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		content        client.MessageContent
		kind           MessageMediaKind
		expectedFiles  []MessageFileDescriptor
		defaultFileID  int32
		contentSupport bool
	}{
		{
			name:           "typed-nil photo",
			content:        (*client.MessagePhoto)(nil),
			kind:           MessageMediaKindPhoto,
			defaultFileID:  0,
			contentSupport: true,
		},
		{
			name: "document",
			content: &client.MessageDocument{Document: &client.Document{
				Document:  &client.File{Id: 301, Size: 100},
				FileName:  "doc.pdf",
				MimeType:  "application/pdf",
				Thumbnail: &client.Thumbnail{File: &client.File{Id: 302, Size: 10}, Width: 64, Height: 64},
			}},
			kind: MessageMediaKindDocument,
			expectedFiles: []MessageFileDescriptor{
				{FileID: 301, Role: MessageFileRoleMain, SourceIndex: 0, FileName: "doc.pdf", MIMEType: "application/pdf", Size: 100, SizeKnown: true},
				{FileID: 302, Role: MessageFileRoleThumbnail, SourceIndex: 0, Width: 64, Height: 64, Size: 10, SizeKnown: true},
			},
			defaultFileID:  301,
			contentSupport: true,
		},
		{
			name: "audio",
			content: &client.MessageAudio{Audio: &client.Audio{
				Audio:    &client.File{Id: 401, Size: 1000},
				FileName: "song.mp3",
				MimeType: "audio/mpeg",
				AlbumCoverThumbnail: &client.Thumbnail{
					File:   &client.File{Id: 402, Size: 100},
					Width:  100,
					Height: 100,
				},
				ExternalAlbumCovers: []*client.Thumbnail{
					nil,
					{File: &client.File{Id: 403, Size: 200}, Width: 200, Height: 200},
				},
			}},
			kind: MessageMediaKindAudio,
			expectedFiles: []MessageFileDescriptor{
				{FileID: 401, Role: MessageFileRoleMain, SourceIndex: 0, FileName: "song.mp3", MIMEType: "audio/mpeg", Size: 1000, SizeKnown: true},
				{FileID: 402, Role: MessageFileRoleAlbumCover, SourceIndex: 0, Width: 100, Height: 100, Size: 100, SizeKnown: true},
				{FileID: 403, Role: MessageFileRoleExternalAlbumCover, SourceIndex: 1, Width: 200, Height: 200, Size: 200, SizeKnown: true},
			},
			defaultFileID:  401,
			contentSupport: true,
		},
		{
			name:    "voice note",
			content: &client.MessageVoiceNote{VoiceNote: &client.VoiceNote{Voice: &client.File{Id: 501}, MimeType: "audio/ogg"}},
			kind:    MessageMediaKindVoiceNote,
			expectedFiles: []MessageFileDescriptor{
				{FileID: 501, Role: MessageFileRoleMain, SourceIndex: 0, MIMEType: "audio/ogg"},
			},
			defaultFileID:  501,
			contentSupport: true,
		},
		{
			name: "animation",
			content: &client.MessageAnimation{Animation: &client.Animation{
				Animation: &client.File{Id: 601, Size: 500},
				FileName:  "anim.gif",
				MimeType:  "image/gif",
				Width:     200,
				Height:    100,
				Thumbnail: &client.Thumbnail{File: &client.File{Id: 602, Size: 60}, Width: 90, Height: 45},
			}},
			kind: MessageMediaKindAnimation,
			expectedFiles: []MessageFileDescriptor{
				{FileID: 601, Role: MessageFileRoleMain, SourceIndex: 0, FileName: "anim.gif", MIMEType: "image/gif", Width: 200, Height: 100, Size: 500, SizeKnown: true},
				{FileID: 602, Role: MessageFileRoleThumbnail, SourceIndex: 0, Width: 90, Height: 45, Size: 60, SizeKnown: true},
			},
			defaultFileID:  601,
			contentSupport: true,
		},
		{
			name: "video note",
			content: &client.MessageVideoNote{VideoNote: &client.VideoNote{
				Video:     &client.File{Id: 701},
				Length:    240,
				Thumbnail: &client.Thumbnail{File: &client.File{Id: 702, Size: 70}, Width: 80, Height: 80},
			}},
			kind: MessageMediaKindVideoNote,
			expectedFiles: []MessageFileDescriptor{
				{FileID: 701, Role: MessageFileRoleMain, SourceIndex: 0, Width: 240, Height: 240},
				{FileID: 702, Role: MessageFileRoleThumbnail, SourceIndex: 0, Width: 80, Height: 80, Size: 70, SizeKnown: true},
			},
			defaultFileID:  701,
			contentSupport: true,
		},
		{
			name: "sticker with premium animation",
			content: &client.MessageSticker{Sticker: &client.Sticker{
				Sticker:   &client.File{Id: 801, Size: 50},
				Width:     512,
				Height:    512,
				FullType:  &client.StickerFullTypeRegular{PremiumAnimation: &client.File{Id: 802}},
				Thumbnail: &client.Thumbnail{File: &client.File{Id: 803}, Width: 128, Height: 128},
			}},
			kind: MessageMediaKindSticker,
			expectedFiles: []MessageFileDescriptor{
				{FileID: 801, Role: MessageFileRoleMain, SourceIndex: 0, Width: 512, Height: 512, Size: 50, SizeKnown: true},
				{FileID: 803, Role: MessageFileRoleThumbnail, SourceIndex: 0, Width: 128, Height: 128},
				{FileID: 802, Role: MessageFileRolePremiumAnimation, SourceIndex: 0},
			},
			defaultFileID:  801,
			contentSupport: true,
		},
		{
			name: "video without primary wrapper keeps alternatives",
			content: &client.MessageVideo{
				Video: nil,
				AlternativeVideos: []*client.AlternativeVideo{
					{Video: &client.File{Id: 251, Size: 4000}, HlsFile: &client.File{Id: 252}, Width: 640, Height: 360, Codec: "av1"},
				},
			},
			kind: MessageMediaKindVideo,
			expectedFiles: []MessageFileDescriptor{
				{FileID: 251, Role: MessageFileRoleAlternativeVideo, SourceIndex: 0, Width: 640, Height: 360, Codec: "av1", Size: 4000, SizeKnown: true},
				{FileID: 252, Role: MessageFileRoleHLSPlaylist, SourceIndex: 0, Width: 640, Height: 360, Codec: "av1"},
			},
			defaultFileID:  0,
			contentSupport: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gw := mocks.NewTelegramRepo(t)
			svc := New(gw)
			gw.EXPECT().GetMessage(mock.Anything).Return(&client.Message{Content: tt.content}, nil).Once()
			result, err := svc.ListMessageFiles(context.Background(), 100, 7)
			require.NoError(t, err)
			assert.Equal(t, tt.contentSupport, result.ContentSupported)
			require.Len(t, result.Items, 1)
			item := result.Items[0]
			assert.Equal(t, tt.kind, item.Kind)
			if len(tt.expectedFiles) == 0 {
				assert.Equal(t, MessageMediaStateNoFiles, item.State)
				assert.Empty(t, item.Files)
			} else {
				assert.Equal(t, MessageMediaStateFilesPresent, item.State)
				require.Len(t, item.Files, len(tt.expectedFiles))
				assert.Equal(t, tt.expectedFiles, item.Files)
			}
			assert.Equal(t, tt.defaultFileID, result.DefaultFileID)
		})
	}
}

func TestService_ListMessageFiles_PaidMedia(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		media         []client.PaidMedia
		expectedItems []MessageMediaItem
		defaultFileID int32
	}{
		{
			name: "photo+video+preview+unsupported",
			media: []client.PaidMedia{
				&client.PaidMediaPhoto{Photo: &client.Photo{Sizes: []*client.PhotoSize{photoFixture(901, 100, 100, 100, "s")}}},
				&client.PaidMediaVideo{Video: &client.Video{
					Video:     &client.File{Id: 902, Size: 1000},
					MimeType:  "video/mp4",
					Thumbnail: &client.Thumbnail{File: &client.File{Id: 903, Size: 90}, Width: 120, Height: 68},
				}},
				&client.PaidMediaPreview{Width: 100, Height: 100},
				&client.PaidMediaUnsupported{},
			},
			expectedItems: []MessageMediaItem{
				{ItemIndex: 0, Kind: MessageMediaKindPhoto, State: MessageMediaStateFilesPresent, Files: []MessageFileDescriptor{
					{FileID: 901, Role: MessageFileRolePhotoSize, SourceIndex: 0, MIMEType: "image/jpeg", Width: 100, Height: 100, Size: 100, SizeKnown: true, PhotoSizeType: "s"},
				}},
				{ItemIndex: 1, Kind: MessageMediaKindVideo, State: MessageMediaStateFilesPresent, Files: []MessageFileDescriptor{
					{FileID: 902, Role: MessageFileRoleMain, SourceIndex: 0, MIMEType: "video/mp4", Size: 1000, SizeKnown: true},
					{FileID: 903, Role: MessageFileRoleThumbnail, SourceIndex: 0, Width: 120, Height: 68, Size: 90, SizeKnown: true},
				}},
				{ItemIndex: 2, Kind: MessageMediaKindPreview, State: MessageMediaStateLocked},
				{ItemIndex: 3, Kind: MessageMediaKindUnsupported, State: MessageMediaStateUnsupported},
			},
			defaultFileID: 0,
		},
		{
			name: "nil and typed-nil slots",
			media: []client.PaidMedia{
				nil,
				(*client.PaidMediaPhoto)(nil),
				(*client.PaidMediaVideo)(nil),
				(*client.PaidMediaPreview)(nil),
				(*client.PaidMediaUnsupported)(nil),
			},
			expectedItems: []MessageMediaItem{
				{ItemIndex: 0, Kind: MessageMediaKindUnsupported, State: MessageMediaStateUnsupported},
				{ItemIndex: 1, Kind: MessageMediaKindUnsupported, State: MessageMediaStateUnsupported},
				{ItemIndex: 2, Kind: MessageMediaKindUnsupported, State: MessageMediaStateUnsupported},
				{ItemIndex: 3, Kind: MessageMediaKindUnsupported, State: MessageMediaStateUnsupported},
				{ItemIndex: 4, Kind: MessageMediaKindUnsupported, State: MessageMediaStateUnsupported},
			},
			defaultFileID: 0,
		},
		{
			name: "empty nested wrappers",
			media: []client.PaidMedia{
				&client.PaidMediaPhoto{},
				&client.PaidMediaVideo{},
			},
			expectedItems: []MessageMediaItem{
				{ItemIndex: 0, Kind: MessageMediaKindPhoto, State: MessageMediaStateNoFiles},
				{ItemIndex: 1, Kind: MessageMediaKindVideo, State: MessageMediaStateNoFiles},
			},
			defaultFileID: 0,
		},
		{
			name:          "unknown paid subtype",
			media:         []client.PaidMedia{unknownPaidMedia{}},
			expectedItems: []MessageMediaItem{{ItemIndex: 0, Kind: MessageMediaKindUnsupported, State: MessageMediaStateUnsupported}},
			defaultFileID: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gw := mocks.NewTelegramRepo(t)
			svc := New(gw)
			gw.EXPECT().GetMessage(mock.Anything).Return(&client.Message{Content: &client.MessagePaidMedia{Media: tt.media}}, nil).Once()
			result, err := svc.ListMessageFiles(context.Background(), 100, 7)
			require.NoError(t, err)
			assert.True(t, result.ContentSupported)
			assert.Equal(t, tt.expectedItems, result.Items)
			assert.Equal(t, tt.defaultFileID, result.DefaultFileID)
		})
	}
}

func TestService_ListMessageFiles_PaidMediaSeparateItems(t *testing.T) {
	t.Parallel()

	gw := mocks.NewTelegramRepo(t)
	svc := New(gw)
	gw.EXPECT().GetMessage(mock.Anything).Return(&client.Message{
		Content: &client.MessagePaidMedia{Media: []client.PaidMedia{
			&client.PaidMediaPhoto{Photo: &client.Photo{Sizes: []*client.PhotoSize{photoFixture(901, 100, 100, 100, "s")}}},
			&client.PaidMediaVideo{Video: &client.Video{Video: &client.File{Id: 902, Size: 1000}}},
		}},
	}, nil).Once()

	result, err := svc.ListMessageFiles(context.Background(), 100, 7)
	require.NoError(t, err)
	require.Len(t, result.Items, 2)
	assert.Equal(t, int32(0), result.Items[0].ItemIndex)
	assert.Equal(t, MessageMediaKindPhoto, result.Items[0].Kind)
	assert.Equal(t, int32(1), result.Items[1].ItemIndex)
	assert.Equal(t, MessageMediaKindVideo, result.Items[1].Kind)
	assert.Zero(t, result.DefaultFileID)
}

type unknownPaidMedia struct{}

func (unknownPaidMedia) PaidMediaType() string { return "unknown" }

func TestService_ListMessageFiles_UnsupportedContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content client.MessageContent
	}{
		{name: "text", content: &client.MessageText{Text: &client.FormattedText{Text: "hello"}}},
		{name: "nil content", content: nil},
		{name: "location", content: &client.MessageLocation{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gw := mocks.NewTelegramRepo(t)
			svc := New(gw)
			gw.EXPECT().GetMessage(mock.Anything).Return(&client.Message{Content: tt.content}, nil).Once()
			result, err := svc.ListMessageFiles(context.Background(), 100, 7)
			require.NoError(t, err)
			assert.False(t, result.ContentSupported)
			assert.Empty(t, result.Items)
			assert.Zero(t, result.DefaultFileID)
		})
	}
}

func TestService_ListMessageFiles_TypedNilPaidMedia(t *testing.T) {
	t.Parallel()

	gw := mocks.NewTelegramRepo(t)
	svc := New(gw)
	gw.EXPECT().GetMessage(mock.Anything).Return(&client.Message{Content: (*client.MessagePaidMedia)(nil)}, nil).Once()
	result, err := svc.ListMessageFiles(context.Background(), 100, 7)
	require.NoError(t, err)
	assert.True(t, result.ContentSupported)
	assert.Empty(t, result.Items)
	assert.Zero(t, result.DefaultFileID)
}

func TestService_ListMessageFiles_DuplicateIDs(t *testing.T) {
	t.Parallel()

	t.Run("across roles in one video item", func(t *testing.T) {
		t.Parallel()
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		gw.EXPECT().GetMessage(mock.Anything).Return(&client.Message{
			Content: &client.MessageVideo{
				Video: &client.Video{Video: &client.File{Id: 999, Size: 1000}},
				AlternativeVideos: []*client.AlternativeVideo{
					{Video: &client.File{Id: 999}, HlsFile: &client.File{Id: 999}, Codec: "av1"},
				},
			},
		}, nil).Once()

		result, err := svc.ListMessageFiles(context.Background(), 100, 7)
		require.NoError(t, err)
		require.Len(t, result.Items, 1)
		require.Len(t, result.Items[0].Files, 3)
		assert.Equal(t, int32(999), result.Items[0].Files[0].FileID)
		assert.Equal(t, int32(999), result.Items[0].Files[1].FileID)
		assert.Equal(t, int32(999), result.Items[0].Files[2].FileID)
		assert.Equal(t, int32(999), result.DefaultFileID)
	})

	t.Run("across separate paid items", func(t *testing.T) {
		t.Parallel()
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		gw.EXPECT().GetMessage(mock.Anything).Return(&client.Message{
			Content: &client.MessagePaidMedia{Media: []client.PaidMedia{
				&client.PaidMediaPhoto{Photo: &client.Photo{Sizes: []*client.PhotoSize{photoFixture(999, 100, 100, 100, "s")}}},
				&client.PaidMediaVideo{Video: &client.Video{Video: &client.File{Id: 999, Size: 1000}}},
			}},
		}, nil).Once()

		result, err := svc.ListMessageFiles(context.Background(), 100, 7)
		require.NoError(t, err)
		require.Len(t, result.Items, 2)
		require.Len(t, result.Items[0].Files, 1)
		require.Len(t, result.Items[1].Files, 1)
		assert.Equal(t, int32(0), result.Items[0].ItemIndex)
		assert.Equal(t, int32(999), result.Items[0].Files[0].FileID)
		assert.Equal(t, MessageFileRolePhotoSize, result.Items[0].Files[0].Role)
		assert.Equal(t, int32(1), result.Items[1].ItemIndex)
		assert.Equal(t, int32(999), result.Items[1].Files[0].FileID)
		assert.Equal(t, MessageFileRoleMain, result.Items[1].Files[0].Role)
		assert.Zero(t, result.DefaultFileID)
	})
}

func TestService_ListMessageFiles_InvalidFileRefs(t *testing.T) {
	t.Parallel()

	gw := mocks.NewTelegramRepo(t)
	svc := New(gw)
	gw.EXPECT().GetMessage(mock.Anything).Return(&client.Message{
		Content: &client.MessageVideo{
			Video: &client.Video{
				Video:     &client.File{Id: 1001, Size: 1000},
				Thumbnail: &client.Thumbnail{File: &client.File{Id: -5}},
			},
			AlternativeVideos: []*client.AlternativeVideo{
				{Video: nil, HlsFile: &client.File{Id: 0}},
				nil,
			},
		},
	}, nil).Once()

	result, err := svc.ListMessageFiles(context.Background(), 100, 7)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	item := result.Items[0]
	assert.Equal(t, MessageMediaStateFilesPresent, item.State)
	require.Len(t, item.Files, 1)
	assert.Equal(t, int32(1001), item.Files[0].FileID)
}

func TestService_ListMessageFiles_SizeAndMetadataUnknowns(t *testing.T) {
	t.Parallel()

	gw := mocks.NewTelegramRepo(t)
	svc := New(gw)
	gw.EXPECT().GetMessage(mock.Anything).Return(&client.Message{
		Content: &client.MessagePhoto{Photo: &client.Photo{Sizes: []*client.PhotoSize{
			{Photo: &client.File{Id: 1101, Size: 0, ExpectedSize: 999}, Type: "", Width: 0, Height: 0},
		}}},
	}, nil).Once()

	result, err := svc.ListMessageFiles(context.Background(), 100, 7)
	require.NoError(t, err)
	require.Len(t, result.Items, 1)
	require.Len(t, result.Items[0].Files, 1)
	file := result.Items[0].Files[0]
	assert.False(t, file.SizeKnown)
	assert.Zero(t, file.Size)
	assert.Empty(t, file.PhotoSizeType)
	assert.Zero(t, file.Width)
	assert.Zero(t, file.Height)
	assert.Equal(t, "image/jpeg", file.MIMEType)
}

func TestService_ListMessageFiles_TextVsSupportedEmpty(t *testing.T) {
	t.Parallel()

	t.Run("text", func(t *testing.T) {
		t.Parallel()
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		gw.EXPECT().GetMessage(mock.Anything).Return(&client.Message{Content: &client.MessageText{}}, nil).Once()
		result, err := svc.ListMessageFiles(context.Background(), 100, 7)
		require.NoError(t, err)
		assert.False(t, result.ContentSupported)
	})
	t.Run("supported-empty photo", func(t *testing.T) {
		t.Parallel()
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		gw.EXPECT().GetMessage(mock.Anything).Return(&client.Message{Content: &client.MessagePhoto{}}, nil).Once()
		result, err := svc.ListMessageFiles(context.Background(), 100, 7)
		require.NoError(t, err)
		assert.True(t, result.ContentSupported)
		require.Len(t, result.Items, 1)
		assert.Equal(t, MessageMediaStateNoFiles, result.Items[0].State)
	})
}

func TestService_ListMessageFiles_AlbumNoSiblingCalls(t *testing.T) {
	t.Parallel()

	gw := mocks.NewTelegramRepo(t)
	svc := New(gw)
	gw.EXPECT().GetMessage(mock.Anything).Return(&client.Message{MediaAlbumId: 777}, nil).Once()

	result, err := svc.ListMessageFiles(context.Background(), 100, 7)
	require.NoError(t, err)
	assert.Equal(t, int64(777), result.MediaAlbumID)
	assert.False(t, result.ContentSupported)
	assert.Empty(t, result.Items)
}

func TestService_ListMessageFiles_RepeatSameSnapshot(t *testing.T) {
	t.Parallel()

	gw := mocks.NewTelegramRepo(t)
	svc := New(gw)
	msg := &client.Message{
		Content: &client.MessageVideo{
			Video: &client.Video{Video: &client.File{Id: 2001}},
			AlternativeVideos: []*client.AlternativeVideo{
				{Video: &client.File{Id: 2002}, HlsFile: &client.File{Id: 2003}, Codec: "h265"},
			},
		},
	}
	gw.EXPECT().GetMessage(mock.Anything).Return(msg, nil).Once()
	gw.EXPECT().GetMessage(mock.Anything).Return(msg, nil).Once()

	first, err := svc.ListMessageFiles(context.Background(), 100, 7)
	require.NoError(t, err)
	second, err := svc.ListMessageFiles(context.Background(), 100, 7)
	require.NoError(t, err)
	assert.Equal(t, first, second)
}

func TestService_ListMessageFiles_ContextAndErrors(t *testing.T) {
	t.Parallel()

	t.Run("cancel before call", func(t *testing.T) {
		t.Parallel()
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		result, err := svc.ListMessageFiles(ctx, 100, 7)
		require.ErrorIs(t, err, context.Canceled)
		assert.Nil(t, result)
	})
	t.Run("cancel after controlled return wins", func(t *testing.T) {
		t.Parallel()
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		gw.EXPECT().GetMessage(mock.Anything).RunAndReturn(func(*client.GetMessageRequest) (*client.Message, error) {
			cancel()
			return &client.Message{Content: &client.MessagePhoto{}}, nil
		}).Once()
		result, err := svc.ListMessageFiles(ctx, 100, 7)
		require.ErrorIs(t, err, context.Canceled)
		assert.Nil(t, result)
	})
	t.Run("cancel after controlled error wins", func(t *testing.T) {
		t.Parallel()
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		upstream := errors.New("controlled upstream")
		gw.EXPECT().GetMessage(mock.Anything).RunAndReturn(func(*client.GetMessageRequest) (*client.Message, error) {
			cancel()
			return nil, upstream
		}).Once()
		result, err := svc.ListMessageFiles(ctx, 100, 7)
		require.ErrorIs(t, err, context.Canceled)
		require.NotErrorIs(t, err, upstream)
		assert.Nil(t, result)
	})
	t.Run("nil message is NotFound", func(t *testing.T) {
		t.Parallel()
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		gw.EXPECT().GetMessage(mock.Anything).Return(nil, nil).Once()
		result, err := svc.ListMessageFiles(context.Background(), 100, 7)
		require.Error(t, err)
		var notFound *MessageNotFoundError
		require.ErrorAs(t, err, &notFound)
		assert.Nil(t, result)
	})
	t.Run("upstream error unchanged", func(t *testing.T) {
		t.Parallel()
		gw := mocks.NewTelegramRepo(t)
		svc := New(gw)
		upstream := errors.New("upstream")
		gw.EXPECT().GetMessage(mock.Anything).Return(nil, upstream).Once()
		result, err := svc.ListMessageFiles(context.Background(), 100, 7)
		require.ErrorIs(t, err, upstream)
		assert.Nil(t, result)
	})
}

func TestService_GetMessageFile_PaidMediaUnsupported(t *testing.T) {
	t.Parallel()

	repo := mocks.NewTelegramRepo(t)
	service := New(repo)
	repo.EXPECT().GetMessage(&client.GetMessageRequest{ChatId: 100, MessageId: 7}).Return(&client.Message{
		Content: &client.MessagePaidMedia{Media: []client.PaidMedia{
			&client.PaidMediaPhoto{Photo: &client.Photo{Sizes: []*client.PhotoSize{photoFixture(901, 100, 100, 100, "s")}}},
		}},
	}, nil).Once()

	err := service.GetMessageFile(context.Background(), 100, 7, func(FileDownloadEvent) error {
		return errors.New("emit called")
	})
	require.Error(t, err)
	var notFound *MessageFileNotFoundError
	require.ErrorAs(t, err, &notFound)
	assert.Equal(t, int64(100), notFound.ChatID)
	assert.Equal(t, int64(7), notFound.MessageID)
}
