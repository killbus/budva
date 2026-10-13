package handler

import (
	"time"

	"github.com/zelenin/go-tdlib/client"

	"github.com/pure-golang/budva-claude/internal/service/push/messagev2"
)

// Projection is a pure function of the source message: it captures
// transport-independent facts only. It never inspects endpoint
// configuration and never serializes webhook JSON; endpoint schema
// selection happens later at the delivery boundary.
//
// Pinned content types are the exact 74 MessageContent types of the
// pinned go-tdlib v0.7.6 inventory (research/content-type-matrix.md);
// every other runtime discriminator is represented as unknown with a
// bounded source_type. The set is frozen for M03 and grows only with a
// deliberate research and pin update.
var pinnedContentTypes = map[string]struct{}{
	client.TypeMessageText:                         {},
	client.TypeMessageAnimation:                    {},
	client.TypeMessageAudio:                        {},
	client.TypeMessageDocument:                     {},
	client.TypeMessagePaidMedia:                    {},
	client.TypeMessagePhoto:                        {},
	client.TypeMessageSticker:                      {},
	client.TypeMessageVideo:                        {},
	client.TypeMessageVideoNote:                    {},
	client.TypeMessageVoiceNote:                    {},
	client.TypeMessageExpiredPhoto:                 {},
	client.TypeMessageExpiredVideo:                 {},
	client.TypeMessageExpiredVideoNote:             {},
	client.TypeMessageExpiredVoiceNote:             {},
	client.TypeMessageLocation:                     {},
	client.TypeMessageVenue:                        {},
	client.TypeMessageContact:                      {},
	client.TypeMessageAnimatedEmoji:                {},
	client.TypeMessageDice:                         {},
	client.TypeMessageGame:                         {},
	client.TypeMessagePoll:                         {},
	client.TypeMessageStory:                        {},
	client.TypeMessageInvoice:                      {},
	client.TypeMessageCall:                         {},
	client.TypeMessageVideoChatScheduled:           {},
	client.TypeMessageVideoChatStarted:             {},
	client.TypeMessageVideoChatEnded:               {},
	client.TypeMessageInviteVideoChatParticipants:  {},
	client.TypeMessageBasicGroupChatCreate:         {},
	client.TypeMessageSupergroupChatCreate:         {},
	client.TypeMessageChatChangeTitle:              {},
	client.TypeMessageChatChangePhoto:              {},
	client.TypeMessageChatDeletePhoto:              {},
	client.TypeMessageChatAddMembers:               {},
	client.TypeMessageChatJoinByLink:               {},
	client.TypeMessageChatJoinByRequest:            {},
	client.TypeMessageChatDeleteMember:             {},
	client.TypeMessageChatUpgradeTo:                {},
	client.TypeMessageChatUpgradeFrom:              {},
	client.TypeMessagePinMessage:                   {},
	client.TypeMessageScreenshotTaken:              {},
	client.TypeMessageChatSetBackground:            {},
	client.TypeMessageChatSetTheme:                 {},
	client.TypeMessageChatSetMessageAutoDeleteTime: {},
	client.TypeMessageChatBoost:                    {},
	client.TypeMessageForumTopicCreated:            {},
	client.TypeMessageForumTopicEdited:             {},
	client.TypeMessageForumTopicIsClosedToggled:    {},
	client.TypeMessageForumTopicIsHiddenToggled:    {},
	client.TypeMessageSuggestProfilePhoto:          {},
	client.TypeMessageCustomServiceAction:          {},
	client.TypeMessageGameScore:                    {},
	client.TypeMessagePaymentSuccessful:            {},
	client.TypeMessagePaymentSuccessfulBot:         {},
	client.TypeMessagePaymentRefunded:              {},
	client.TypeMessageGiftedPremium:                {},
	client.TypeMessagePremiumGiftCode:              {},
	client.TypeMessageGiveawayCreated:              {},
	client.TypeMessageGiveaway:                     {},
	client.TypeMessageGiveawayCompleted:            {},
	client.TypeMessageGiveawayWinners:              {},
	client.TypeMessageGiftedStars:                  {},
	client.TypeMessageGiveawayPrizeStars:           {},
	client.TypeMessageGift:                         {},
	client.TypeMessageContactRegistered:            {},
	client.TypeMessageUsersShared:                  {},
	client.TypeMessageChatShared:                   {},
	client.TypeMessageBotWriteAccessAllowed:        {},
	client.TypeMessageWebAppDataSent:               {},
	client.TypeMessageWebAppDataReceived:           {},
	client.TypeMessagePassportDataSent:             {},
	client.TypeMessagePassportDataReceived:         {},
	client.TypeMessageProximityAlertTriggered:      {},
	client.TypeMessageUnsupported:                  {},
}

// v2SnapshotFor projects one current message snapshot (new or edited event).
// It returns nil only when the identity is unusable; every other unknown or
// out-of-bounds fact is represented within the notification, and a resulting
// serializer rejection is the delivery boundary's explicit failure, never a
// v1 fallback.
func v2SnapshotFor(msg *client.Message, ev messagev2.Event, occurredAt time.Time) *messagev2.Notification {
	if msg == nil || msg.ChatId == 0 || msg.Id == 0 {
		return nil
	}
	m := &messagev2.Message{
		ChatID:                  msg.ChatId,
		MessageID:               msg.Id,
		Sender:                  v2SenderFor(msg.SenderId),
		SentAt:                  unixToTime(int64(msg.Date)),
		EditedAt:                unixToTime(int64(msg.EditDate)),
		Content:                 v2ContentFor(msg.Content),
		ReplyTo:                 v2ReplyFor(msg.ReplyTo),
		MessageThreadID:         msg.MessageThreadId,
		SavedMessagesTopicID:    msg.SavedMessagesTopicId,
		MediaAlbumID:            int64(msg.MediaAlbumId),
		ViaBotUserID:            msg.ViaBotUserId,
		SenderBusinessBotUserID: msg.SenderBusinessBotUserId,
		AuthorSignature:         msg.AuthorSignature,
		HasSensitiveContent:     msg.HasSensitiveContent,
		RestrictionReason:       msg.RestrictionReason,
		Forward:                 v2ForwardFor(msg.ForwardInfo),
	}
	return &messagev2.Notification{
		SchemaVersion: messagev2.SchemaVersion,
		Event:         ev,
		OccurredAt:    occurredAt,
		Message:       m,
	}
}

// v2DeleteFor projects one deleted-message notification. It carries identity
// and the two delete-update facts only; no snapshot fact is synthesized.
func v2DeleteFor(chatID, messageID int64, isPermanent, fromCache bool, occurredAt time.Time) *messagev2.Notification {
	if chatID == 0 || messageID == 0 {
		return nil
	}
	return &messagev2.Notification{
		SchemaVersion: messagev2.SchemaVersion,
		Event:         messagev2.EventDeleted,
		OccurredAt:    occurredAt,
		Message: &messagev2.Message{
			ChatID:    chatID,
			MessageID: messageID,
			Delete: &messagev2.Delete{
				IsPermanent: isPermanent,
				FromCache:   fromCache,
			},
		},
	}
}

// v2SenderFor maps the bounded sender variants; unsupported variants are
// omitted rather than serialized raw.
func v2SenderFor(s client.MessageSender) *messagev2.Sender {
	switch sender := s.(type) {
	case *client.MessageSenderUser:
		id := sender.UserId
		return &messagev2.Sender{Type: client.TypeMessageSenderUser, UserID: &id}
	case *client.MessageSenderChat:
		id := sender.ChatId
		return &messagev2.Sender{Type: client.TypeMessageSenderChat, ChatID: &id}
	default:
		return nil
	}
}

// v2ContentFor projects the pinned content discriminator, the message text or
// caption and the safe media flags. Text-bearing content is derived from the
// same content variants the source message service extracts.
func v2ContentFor(c client.MessageContent) *messagev2.Content {
	if c == nil {
		return &messagev2.Content{Type: messagev2.UnknownContentType}
	}
	disc := c.MessageContentType()
	content := &messagev2.Content{Type: disc}
	if _, pinned := pinnedContentTypes[disc]; !pinned {
		content.Type = messagev2.UnknownContentType
		if messagev2.ValidateDiscriminator(disc) == nil {
			content.SourceType = disc
		}
	}
	switch typed := c.(type) {
	case *client.MessageText:
		content.FormattedText = v2FormattedTextFor(typed.Text)
	case *client.MessagePhoto:
		content.FormattedText = v2FormattedTextFor(typed.Caption)
		content.Flags = &messagev2.ContentFlags{
			ShowCaptionAboveMedia: &typed.ShowCaptionAboveMedia,
			HasSpoiler:            &typed.HasSpoiler,
		}
	case *client.MessageVideo:
		content.FormattedText = v2FormattedTextFor(typed.Caption)
		content.Flags = &messagev2.ContentFlags{
			ShowCaptionAboveMedia: &typed.ShowCaptionAboveMedia,
			HasSpoiler:            &typed.HasSpoiler,
		}
	case *client.MessageAnimation:
		content.FormattedText = v2FormattedTextFor(typed.Caption)
		content.Flags = &messagev2.ContentFlags{
			ShowCaptionAboveMedia: &typed.ShowCaptionAboveMedia,
			HasSpoiler:            &typed.HasSpoiler,
		}
	case *client.MessageAudio:
		content.FormattedText = v2FormattedTextFor(typed.Caption)
	case *client.MessageDocument:
		content.FormattedText = v2FormattedTextFor(typed.Caption)
	case *client.MessageVoiceNote:
		content.FormattedText = v2FormattedTextFor(typed.Caption)
	}
	return content
}

// v2FormattedTextFor copies the text and the bounded entity discriminators.
// Entity offsets and lengths are TDLib UTF-16 code units and are copied
// verbatim; an out-of-bounds entity type is omitted.
func v2FormattedTextFor(ft *client.FormattedText) *messagev2.FormattedText {
	if ft == nil {
		return nil
	}
	out := &messagev2.FormattedText{Text: ft.Text, Entities: make([]messagev2.Entity, 0, len(ft.Entities))}
	for _, e := range ft.Entities {
		if e == nil {
			continue
		}
		t := e.Type.TextEntityTypeType()
		if t == "" || messagev2.ValidateDiscriminator(t) != nil {
			continue
		}
		out.Entities = append(out.Entities, messagev2.Entity{Offset: e.Offset, Length: e.Length, Type: t})
	}
	return out
}

// v2ReplyFor keeps the bounded reply discriminator; only the message variant
// additionally carries its chat and message identifiers. Quote, origin and
// nested content are excluded.
func v2ReplyFor(r client.MessageReplyTo) *messagev2.ReplyTo {
	if r == nil {
		return nil
	}
	t := r.MessageReplyToType()
	if t == "" || messagev2.ValidateDiscriminator(t) != nil {
		return nil
	}
	out := &messagev2.ReplyTo{Type: t}
	if m, ok := r.(*client.MessageReplyToMessage); ok {
		out.ChatID = m.ChatId
		out.MessageID = m.MessageId
	}
	return out
}

// v2ForwardFor projects the shallow sanitized forward provenance: origin,
// send time, source identities and the bounded announcement type.
func v2ForwardFor(f *client.MessageForwardInfo) *messagev2.Forward {
	if f == nil {
		return nil
	}
	out := &messagev2.Forward{
		Origin: v2OriginFor(f.Origin),
		SentAt: unixToTime(int64(f.Date)),
	}
	if f.Source != nil {
		source := &messagev2.ForwardSource{
			ChatID:    f.Source.ChatId,
			MessageID: f.Source.MessageId,
			Sender:    v2SenderFor(f.Source.SenderId),
			SentAt:    unixToTime(int64(f.Source.Date)),
		}
		if err := messagev2.ValidateBoundedString(f.Source.SenderName); err == nil {
			source.SenderName = f.Source.SenderName
		}
		out.Source = source
	}
	if f.PublicServiceAnnouncementType != "" && messagev2.ValidateDiscriminator(f.PublicServiceAnnouncementType) == nil {
		out.PublicServiceAnnouncementType = f.PublicServiceAnnouncementType
	}
	return out
}

// v2OriginFor maps the bounded origin variants; unsupported variants are
// omitted. Bounded source strings that cannot be represented are omitted
// rather than failing the whole projection.
func v2OriginFor(o client.MessageOrigin) *messagev2.Origin {
	if o == nil {
		return nil
	}
	t := o.MessageOriginType()
	if t == "" || messagev2.ValidateDiscriminator(t) != nil {
		return nil
	}
	out := &messagev2.Origin{Type: t}
	switch origin := o.(type) {
	case *client.MessageOriginUser:
		id := origin.SenderUserId
		out.SenderUserID = &id
	case *client.MessageOriginHiddenUser:
		if err := messagev2.ValidateBoundedString(origin.SenderName); err == nil {
			out.SenderName = origin.SenderName
		}
	case *client.MessageOriginChat:
		id := origin.SenderChatId
		out.SenderChatID = &id
		out.AuthorSignature = origin.AuthorSignature
	case *client.MessageOriginChannel:
		id := origin.ChatId
		out.ChatID = &id
		mid := origin.MessageId
		out.MessageID = &mid
		out.AuthorSignature = origin.AuthorSignature
	}
	return out
}

// unixToTime converts a TDLib Unix second to UTC; a zero timestamp is absent
// rather than rendered as the Unix epoch.
func unixToTime(secs int64) *time.Time {
	if secs == 0 {
		return nil
	}
	t := time.Unix(secs, 0).UTC()
	return &t
}
