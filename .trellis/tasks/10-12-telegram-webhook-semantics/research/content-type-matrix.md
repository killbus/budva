# TDLib Message Content Policy Matrix

Status: M01 semantic mapping under verification; this matrix is research evidence only.
This is not an approved wire schema, runtime-support claim or M03 start
approval. M02 remains responsible for contract and packaging decisions.

## Evidence And Boundary

The inventory comes from pinned
`github.com/zelenin/go-tdlib@v0.7.6/client/type.go`. Exactly 74 concrete
`MessageContent` implementations are mapped once each below.

Message projections contain only message facts. Attachment entities, file
representations, access/download/cache state, relation collections or
completeness, Delivery state and Album completeness are excluded from Message.
Resource notes are evidence for later independent Attachment analysis, not
proposed Message fields. Source fields are exact top-level TDLib fields.

## Per-Type Mapping

### MessageText

- Source: `type.go:23742`
- Exact source fields: `text:Text *FormattedText`, `link_preview:LinkPreview *LinkPreview`, `link_preview_options:LinkPreviewOptions *LinkPreviewOptions`.
- Proposed Message fact projection: `text`, `link_preview`, `link_preview_options`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-text` or a covering scenario below.
### MessageAnimation

- Source: `type.go:23773`
- Exact source fields: `animation:Animation *Animation`, `caption:Caption *FormattedText`, `show_caption_above_media:ShowCaptionAboveMedia bool`, `has_spoiler:HasSpoiler bool`, `is_secret:IsSecret bool`.
- Proposed Message fact projection: `animation`, `caption`, `show_caption_above_media`, `has_spoiler`, `is_secret`.
- Direct/nested resource semantics: direct Animation; thumbnail and main File are representations of one resource.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-animation` or a covering scenario below.
### MessageAudio

- Source: `type.go:23808`
- Exact source fields: `audio:Audio *Audio`, `caption:Caption *FormattedText`.
- Proposed Message fact projection: `audio`, `caption`.
- Direct/nested resource semantics: direct Audio; cover thumbnails and main File are representations.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-audio` or a covering scenario below.
### MessageDocument

- Source: `type.go:23837`
- Exact source fields: `document:Document *Document`, `caption:Caption *FormattedText`.
- Proposed Message fact projection: `document`, `caption`.
- Direct/nested resource semantics: direct Document; thumbnail and main File are representations.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-document` or a covering scenario below.
### MessagePaidMedia

- Source: `type.go:23866`
- Exact source fields: `star_count:StarCount int64`, `media:Media []PaidMedia`, `caption:Caption *FormattedText`, `show_caption_above_media:ShowCaptionAboveMedia bool`.
- Proposed Message fact projection: content type plus exclusion reason only.
- Direct/nested resource semantics: nested PaidMedia is authority-bound and not exported.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `excluded`; reason: `authority_bound_paid_content`.
- Representative fixture: `fx-paidmedia` or a covering scenario below.
### MessagePhoto

- Source: `type.go:23922`
- Exact source fields: `photo:Photo *Photo`, `caption:Caption *FormattedText`, `show_caption_above_media:ShowCaptionAboveMedia bool`, `has_spoiler:HasSpoiler bool`, `is_secret:IsSecret bool`.
- Proposed Message fact projection: `photo`, `caption`, `show_caption_above_media`, `has_spoiler`, `is_secret`.
- Direct/nested resource semantics: direct Photo; all sizes are representations of one photo resource.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-photo` or a covering scenario below.
### MessageSticker

- Source: `type.go:23957`
- Exact source fields: `sticker:Sticker *Sticker`, `is_premium:IsPremium bool`.
- Proposed Message fact projection: `sticker`, `is_premium`.
- Direct/nested resource semantics: direct Sticker; thumbnail, animation and main files are representations.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-sticker` or a covering scenario below.
### MessageVideo

- Source: `type.go:23986`
- Exact source fields: `video:Video *Video`, `alternative_videos:AlternativeVideos []*AlternativeVideo`, `caption:Caption *FormattedText`, `show_caption_above_media:ShowCaptionAboveMedia bool`, `has_spoiler:HasSpoiler bool`, `is_secret:IsSecret bool`.
- Proposed Message fact projection: `video`, `alternative_videos`, `caption`, `show_caption_above_media`, `has_spoiler`, `is_secret`.
- Direct/nested resource semantics: direct Video; thumbnails/main File are representations; AlternativeVideo values are alternate representations.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-video` or a covering scenario below.
### MessageVideoNote

- Source: `type.go:24023`
- Exact source fields: `video_note:VideoNote *VideoNote`, `is_viewed:IsViewed bool`, `is_secret:IsSecret bool`.
- Proposed Message fact projection: `video_note`, `is_viewed`, `is_secret`.
- Direct/nested resource semantics: direct VideoNote; thumbnail and main File are representations.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-videonote` or a covering scenario below.
### MessageVoiceNote

- Source: `type.go:24054`
- Exact source fields: `voice_note:VoiceNote *VoiceNote`, `caption:Caption *FormattedText`, `is_listened:IsListened bool`.
- Proposed Message fact projection: `voice_note`, `caption`, `is_listened`.
- Direct/nested resource semantics: direct VoiceNote; File access is separate; waveform/transcription are bounded media semantics.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-voicenote` or a covering scenario below.
### MessageExpiredPhoto

- Source: `type.go:24085`
- Exact source fields: none.
- Proposed Message fact projection: content type only.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-expiredphoto` or a covering scenario below.
### MessageExpiredVideo

- Source: `type.go:24110`
- Exact source fields: none.
- Proposed Message fact projection: content type only.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-expiredvideo` or a covering scenario below.
### MessageExpiredVideoNote

- Source: `type.go:24135`
- Exact source fields: none.
- Proposed Message fact projection: content type only.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-expiredvideonote` or a covering scenario below.
### MessageExpiredVoiceNote

- Source: `type.go:24160`
- Exact source fields: none.
- Proposed Message fact projection: content type only.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-expiredvoicenote` or a covering scenario below.
### MessageLocation

- Source: `type.go:24185`
- Exact source fields: `location:Location *Location`, `live_period:LivePeriod int32`, `expires_in:ExpiresIn int32`, `heading:Heading int32`, `proximity_alert_radius:ProximityAlertRadius int32`.
- Proposed Message fact projection: `location`, `live_period`, `heading`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: omit sender-only proximity_alert_radius and observation-local expires_in.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-location` or a covering scenario below.
### MessageVenue

- Source: `type.go:24220`
- Exact source fields: `venue:Venue *Venue`.
- Proposed Message fact projection: `venue`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-venue` or a covering scenario below.
### MessageContact

- Source: `type.go:24247`
- Exact source fields: `contact:Contact *Contact`.
- Proposed Message fact projection: `contact`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-contact` or a covering scenario below.
### MessageAnimatedEmoji

- Source: `type.go:24274`
- Exact source fields: `animated_emoji:AnimatedEmoji *AnimatedEmoji`, `emoji:Emoji string`.
- Proposed Message fact projection: `animated_emoji`, `emoji`.
- Direct/nested resource semantics: nested AnimatedEmoji may reference Sticker resources.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-animatedemoji` or a covering scenario below.
### MessageDice

- Source: `type.go:24303`
- Exact source fields: `initial_state:InitialState DiceStickers`, `final_state:FinalState DiceStickers`, `emoji:Emoji string`, `value:Value int32`, `success_animation_frame_number:SuccessAnimationFrameNumber int32`.
- Proposed Message fact projection: `initial_state`, `final_state`, `emoji`, `value`, `success_animation_frame_number`.
- Direct/nested resource semantics: initial/final DiceStickers are presentation resources.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-dice` or a covering scenario below.
### MessageGame

- Source: `type.go:24365`
- Exact source fields: `game:Game *Game`.
- Proposed Message fact projection: `game`.
- Direct/nested resource semantics: nested Game may contain Photo and Animation resources.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-game` or a covering scenario below.
### MessagePoll

- Source: `type.go:24392`
- Exact source fields: `poll:Poll *Poll`.
- Proposed Message fact projection: `poll`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: omit recent-voter identities and account-local answer state from nested Poll.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-poll` or a covering scenario below.
### MessageStory

- Source: `type.go:24419`
- Exact source fields: `story_sender_chat_id:StorySenderChatId int64`, `story_id:StoryId int32`, `via_mention:ViaMention bool`.
- Proposed Message fact projection: `story_sender_chat_id`, `story_id`, `via_mention`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-story` or a covering scenario below.
### MessageInvoice

- Source: `type.go:24450`
- Exact source fields: `product_info:ProductInfo *ProductInfo`, `currency:Currency string`, `total_amount:TotalAmount int64`, `start_parameter:StartParameter string`, `is_test:IsTest bool`, `need_shipping_address:NeedShippingAddress bool`, `receipt_message_id:ReceiptMessageId int64`, `paid_media:PaidMedia PaidMedia`, `paid_media_caption:PaidMediaCaption *FormattedText`.
- Proposed Message fact projection: `product_info`, `currency`, `total_amount`, `start_parameter`, `is_test`, `need_shipping_address`, `receipt_message_id`, `paid_media`, `paid_media_caption`.
- Direct/nested resource semantics: ProductInfo may contain Photo; PaidMedia remains authority-bound.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-invoice` or a covering scenario below.
### MessageCall

- Source: `type.go:24526`
- Exact source fields: `is_video:IsVideo bool`, `discard_reason:DiscardReason CallDiscardReason`, `duration:Duration int32`.
- Proposed Message fact projection: `is_video`, `discard_reason`, `duration`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-call` or a covering scenario below.
### MessageVideoChatScheduled

- Source: `type.go:24578`
- Exact source fields: `group_call_id:GroupCallId int32`, `start_date:StartDate int32`.
- Proposed Message fact projection: `group_call_id`, `start_date`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-videochatscheduled` or a covering scenario below.
### MessageVideoChatStarted

- Source: `type.go:24607`
- Exact source fields: `group_call_id:GroupCallId int32`.
- Proposed Message fact projection: `group_call_id`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-videochatstarted` or a covering scenario below.
### MessageVideoChatEnded

- Source: `type.go:24634`
- Exact source fields: `duration:Duration int32`.
- Proposed Message fact projection: `duration`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-videochatended` or a covering scenario below.
### MessageInviteVideoChatParticipants

- Source: `type.go:24661`
- Exact source fields: `group_call_id:GroupCallId int32`, `user_ids:UserIds []int64`.
- Proposed Message fact projection: `group_call_id`, `user_ids`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-invitevideochatparticipants` or a covering scenario below.
### MessageBasicGroupChatCreate

- Source: `type.go:24690`
- Exact source fields: `title:Title string`, `member_user_ids:MemberUserIds []int64`.
- Proposed Message fact projection: `title`, `member_user_ids`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-basicgroupchatcreate` or a covering scenario below.
### MessageSupergroupChatCreate

- Source: `type.go:24719`
- Exact source fields: `title:Title string`.
- Proposed Message fact projection: `title`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-supergroupchatcreate` or a covering scenario below.
### MessageChatChangeTitle

- Source: `type.go:24746`
- Exact source fields: `title:Title string`.
- Proposed Message fact projection: `title`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-chatchangetitle` or a covering scenario below.
### MessageChatChangePhoto

- Source: `type.go:24773`
- Exact source fields: `photo:Photo *ChatPhoto`.
- Proposed Message fact projection: `photo`.
- Direct/nested resource semantics: nested ChatPhoto contains photo representations.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-chatchangephoto` or a covering scenario below.
### MessageChatDeletePhoto

- Source: `type.go:24800`
- Exact source fields: none.
- Proposed Message fact projection: content type only.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-chatdeletephoto` or a covering scenario below.
### MessageChatAddMembers

- Source: `type.go:24825`
- Exact source fields: `member_user_ids:MemberUserIds []int64`.
- Proposed Message fact projection: `member_user_ids`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-chataddmembers` or a covering scenario below.
### MessageChatJoinByLink

- Source: `type.go:24852`
- Exact source fields: none.
- Proposed Message fact projection: content type only.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-chatjoinbylink` or a covering scenario below.
### MessageChatJoinByRequest

- Source: `type.go:24877`
- Exact source fields: none.
- Proposed Message fact projection: content type only.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-chatjoinbyrequest` or a covering scenario below.
### MessageChatDeleteMember

- Source: `type.go:24902`
- Exact source fields: `user_id:UserId int64`.
- Proposed Message fact projection: `user_id`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-chatdeletemember` or a covering scenario below.
### MessageChatUpgradeTo

- Source: `type.go:24929`
- Exact source fields: `supergroup_id:SupergroupId int64`.
- Proposed Message fact projection: `supergroup_id`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-chatupgradeto` or a covering scenario below.
### MessageChatUpgradeFrom

- Source: `type.go:24956`
- Exact source fields: `title:Title string`, `basic_group_id:BasicGroupId int64`.
- Proposed Message fact projection: `title`, `basic_group_id`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-chatupgradefrom` or a covering scenario below.
### MessagePinMessage

- Source: `type.go:24985`
- Exact source fields: `message_id:MessageId int64`.
- Proposed Message fact projection: `message_id`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-pinmessage` or a covering scenario below.
### MessageScreenshotTaken

- Source: `type.go:25012`
- Exact source fields: none.
- Proposed Message fact projection: content type only.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-screenshottaken` or a covering scenario below.
### MessageChatSetBackground

- Source: `type.go:25037`
- Exact source fields: `old_background_message_id:OldBackgroundMessageId int64`, `background:Background *ChatBackground`, `only_for_self:OnlyForSelf bool`.
- Proposed Message fact projection: `old_background_message_id`, `background`, `only_for_self`.
- Direct/nested resource semantics: nested ChatBackground may contain background/document representations.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-chatsetbackground` or a covering scenario below.
### MessageChatSetTheme

- Source: `type.go:25068`
- Exact source fields: `theme_name:ThemeName string`.
- Proposed Message fact projection: `theme_name`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-chatsettheme` or a covering scenario below.
### MessageChatSetMessageAutoDeleteTime

- Source: `type.go:25095`
- Exact source fields: `message_auto_delete_time:MessageAutoDeleteTime int32`, `from_user_id:FromUserId int64`.
- Proposed Message fact projection: `message_auto_delete_time`, `from_user_id`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-chatsetmessageautodeletetime` or a covering scenario below.
### MessageChatBoost

- Source: `type.go:25124`
- Exact source fields: `boost_count:BoostCount int32`.
- Proposed Message fact projection: `boost_count`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-chatboost` or a covering scenario below.
### MessageForumTopicCreated

- Source: `type.go:25151`
- Exact source fields: `name:Name string`, `icon:Icon *ForumTopicIcon`.
- Proposed Message fact projection: `name`, `icon`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-forumtopiccreated` or a covering scenario below.
### MessageForumTopicEdited

- Source: `type.go:25180`
- Exact source fields: `name:Name string`, `edit_icon_custom_emoji_id:EditIconCustomEmojiId bool`, `icon_custom_emoji_id:IconCustomEmojiId JsonInt64`.
- Proposed Message fact projection: `name`, `edit_icon_custom_emoji_id`, `icon_custom_emoji_id`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-forumtopicedited` or a covering scenario below.
### MessageForumTopicIsClosedToggled

- Source: `type.go:25211`
- Exact source fields: `is_closed:IsClosed bool`.
- Proposed Message fact projection: `is_closed`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-forumtopicisclosedtoggled` or a covering scenario below.
### MessageForumTopicIsHiddenToggled

- Source: `type.go:25238`
- Exact source fields: `is_hidden:IsHidden bool`.
- Proposed Message fact projection: `is_hidden`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-forumtopicishiddentoggled` or a covering scenario below.
### MessageSuggestProfilePhoto

- Source: `type.go:25265`
- Exact source fields: `photo:Photo *ChatPhoto`.
- Proposed Message fact projection: `photo`.
- Direct/nested resource semantics: nested ChatPhoto contains photo representations.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-suggestprofilephoto` or a covering scenario below.
### MessageCustomServiceAction

- Source: `type.go:25292`
- Exact source fields: `text:Text string`.
- Proposed Message fact projection: `text`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-customserviceaction` or a covering scenario below.
### MessageGameScore

- Source: `type.go:25319`
- Exact source fields: `game_message_id:GameMessageId int64`, `game_id:GameId JsonInt64`, `score:Score int32`.
- Proposed Message fact projection: `game_message_id`, `game_id`, `score`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-gamescore` or a covering scenario below.
### MessagePaymentSuccessful

- Source: `type.go:25350`
- Exact source fields: `invoice_chat_id:InvoiceChatId int64`, `invoice_message_id:InvoiceMessageId int64`, `currency:Currency string`, `total_amount:TotalAmount int64`, `subscription_until_date:SubscriptionUntilDate int32`, `is_recurring:IsRecurring bool`, `is_first_recurring:IsFirstRecurring bool`, `invoice_name:InvoiceName string`.
- Proposed Message fact projection: `invoice_chat_id`, `invoice_message_id`, `currency`, `total_amount`, `subscription_until_date`, `is_recurring`, `is_first_recurring`, `invoice_name`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: omit private transaction and authority state pending M02 review.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-paymentsuccessful` or a covering scenario below.
### MessagePaymentSuccessfulBot

- Source: `type.go:25391`
- Exact source fields: `currency:Currency string`, `total_amount:TotalAmount int64`, `subscription_until_date:SubscriptionUntilDate int32`, `is_recurring:IsRecurring bool`, `is_first_recurring:IsFirstRecurring bool`, `invoice_payload:InvoicePayload []byte`, `shipping_option_id:ShippingOptionId string`, `order_info:OrderInfo *OrderInfo`, `telegram_payment_charge_id:TelegramPaymentChargeId string`, `provider_payment_charge_id:ProviderPaymentChargeId string`.
- Proposed Message fact projection: content type plus exclusion reason only.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `excluded`; reason: `transaction_private`.
- Representative fixture: `fx-paymentsuccessfulbot` or a covering scenario below.
### MessagePaymentRefunded

- Source: `type.go:25436`
- Exact source fields: `owner_id:OwnerId MessageSender`, `currency:Currency string`, `total_amount:TotalAmount int64`, `invoice_payload:InvoicePayload []byte`, `telegram_payment_charge_id:TelegramPaymentChargeId string`, `provider_payment_charge_id:ProviderPaymentChargeId string`.
- Proposed Message fact projection: content type plus exclusion reason only.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `excluded`; reason: `transaction_private`.
- Representative fixture: `fx-paymentrefunded` or a covering scenario below.
### MessageGiftedPremium

- Source: `type.go:25500`
- Exact source fields: `gifter_user_id:GifterUserId int64`, `receiver_user_id:ReceiverUserId int64`, `text:Text *FormattedText`, `currency:Currency string`, `amount:Amount int64`, `cryptocurrency:Cryptocurrency string`, `cryptocurrency_amount:CryptocurrencyAmount JsonInt64`, `month_count:MonthCount int32`, `sticker:Sticker *Sticker`.
- Proposed Message fact projection: `gifter_user_id`, `receiver_user_id`, `text`, `currency`, `amount`, `cryptocurrency`, `cryptocurrency_amount`, `month_count`, `sticker`.
- Direct/nested resource semantics: nested Sticker is a presentation resource.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-giftedpremium` or a covering scenario below.
### MessagePremiumGiftCode

- Source: `type.go:25543`
- Exact source fields: `creator_id:CreatorId MessageSender`, `text:Text *FormattedText`, `is_from_giveaway:IsFromGiveaway bool`, `is_unclaimed:IsUnclaimed bool`, `currency:Currency string`, `amount:Amount int64`, `cryptocurrency:Cryptocurrency string`, `cryptocurrency_amount:CryptocurrencyAmount JsonInt64`, `month_count:MonthCount int32`, `sticker:Sticker *Sticker`, `code:Code string`.
- Proposed Message fact projection: `creator_id`, `text`, `is_from_giveaway`, `is_unclaimed`, `currency`, `amount`, `cryptocurrency`, `cryptocurrency_amount`, `month_count`, `sticker`.
- Direct/nested resource semantics: nested Sticker is a presentation resource.
- Privacy/local-state exclusions: omit code.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-premiumgiftcode` or a covering scenario below.
### MessageGiveawayCreated

- Source: `type.go:25627`
- Exact source fields: `star_count:StarCount int64`.
- Proposed Message fact projection: `star_count`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-giveawaycreated` or a covering scenario below.
### MessageGiveaway

- Source: `type.go:25654`
- Exact source fields: `parameters:Parameters *GiveawayParameters`, `winner_count:WinnerCount int32`, `prize:Prize GiveawayPrize`, `sticker:Sticker *Sticker`.
- Proposed Message fact projection: `parameters`, `winner_count`, `prize`, `sticker`.
- Direct/nested resource semantics: nested Sticker is a presentation resource.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-giveaway` or a covering scenario below.
### MessageGiveawayCompleted

- Source: `type.go:25710`
- Exact source fields: `giveaway_message_id:GiveawayMessageId int64`, `winner_count:WinnerCount int32`, `is_star_giveaway:IsStarGiveaway bool`, `unclaimed_prize_count:UnclaimedPrizeCount int32`.
- Proposed Message fact projection: `giveaway_message_id`, `winner_count`, `is_star_giveaway`, `unclaimed_prize_count`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-giveawaycompleted` or a covering scenario below.
### MessageGiveawayWinners

- Source: `type.go:25743`
- Exact source fields: `boosted_chat_id:BoostedChatId int64`, `giveaway_message_id:GiveawayMessageId int64`, `additional_chat_count:AdditionalChatCount int32`, `actual_winners_selection_date:ActualWinnersSelectionDate int32`, `only_new_members:OnlyNewMembers bool`, `was_refunded:WasRefunded bool`, `prize:Prize GiveawayPrize`, `prize_description:PrizeDescription string`, `winner_count:WinnerCount int32`, `winner_user_ids:WinnerUserIds []int64`, `unclaimed_prize_count:UnclaimedPrizeCount int32`.
- Proposed Message fact projection: `boosted_chat_id`, `giveaway_message_id`, `additional_chat_count`, `actual_winners_selection_date`, `only_new_members`, `was_refunded`, `prize`, `prize_description`, `winner_count`, `winner_user_ids`, `unclaimed_prize_count`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-giveawaywinners` or a covering scenario below.
### MessageGiftedStars

- Source: `type.go:25827`
- Exact source fields: `gifter_user_id:GifterUserId int64`, `receiver_user_id:ReceiverUserId int64`, `currency:Currency string`, `amount:Amount int64`, `cryptocurrency:Cryptocurrency string`, `cryptocurrency_amount:CryptocurrencyAmount JsonInt64`, `star_count:StarCount int64`, `transaction_id:TransactionId string`, `sticker:Sticker *Sticker`.
- Proposed Message fact projection: `gifter_user_id`, `receiver_user_id`, `currency`, `amount`, `cryptocurrency`, `cryptocurrency_amount`, `star_count`, `sticker`.
- Direct/nested resource semantics: nested Sticker is a presentation resource.
- Privacy/local-state exclusions: omit transaction_id.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-giftedstars` or a covering scenario below.
### MessageGiveawayPrizeStars

- Source: `type.go:25870`
- Exact source fields: `star_count:StarCount int64`, `transaction_id:TransactionId string`, `boosted_chat_id:BoostedChatId int64`, `giveaway_message_id:GiveawayMessageId int64`, `is_unclaimed:IsUnclaimed bool`, `sticker:Sticker *Sticker`.
- Proposed Message fact projection: `star_count`, `boosted_chat_id`, `giveaway_message_id`, `is_unclaimed`, `sticker`.
- Direct/nested resource semantics: nested Sticker is a presentation resource.
- Privacy/local-state exclusions: omit transaction_id.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-giveawayprizestars` or a covering scenario below.
### MessageGift

- Source: `type.go:25907`
- Exact source fields: `gift:Gift *Gift`, `text:Text *FormattedText`, `sell_star_count:SellStarCount int64`, `is_private:IsPrivate bool`, `is_saved:IsSaved bool`, `was_converted:WasConverted bool`.
- Proposed Message fact projection: `gift`, `text`, `sell_star_count`, `is_private`.
- Direct/nested resource semantics: nested Gift may reference Sticker resources.
- Privacy/local-state exclusions: omit account-local is_saved and was_converted.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-gift` or a covering scenario below.
### MessageContactRegistered

- Source: `type.go:25944`
- Exact source fields: none.
- Proposed Message fact projection: content type only.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-contactregistered` or a covering scenario below.
### MessageUsersShared

- Source: `type.go:25969`
- Exact source fields: `users:Users []*SharedUser`, `button_id:ButtonId int32`.
- Proposed Message fact projection: content type plus exclusion reason only.
- Direct/nested resource semantics: SharedUser photos are identity representations and excluded by default.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `excluded_by_default`; reason: `account_authority_private`.
- Representative fixture: `fx-usersshared` or a covering scenario below.
### MessageChatShared

- Source: `type.go:25998`
- Exact source fields: `chat:Chat *SharedChat`, `button_id:ButtonId int32`.
- Proposed Message fact projection: content type plus exclusion reason only.
- Direct/nested resource semantics: SharedChat photo is an identity representation and excluded by default.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `excluded_by_default`; reason: `account_authority_private`.
- Representative fixture: `fx-chatshared` or a covering scenario below.
### MessageBotWriteAccessAllowed

- Source: `type.go:26027`
- Exact source fields: `reason:Reason BotWriteAccessAllowReason`.
- Proposed Message fact projection: content type plus exclusion reason only.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `excluded_by_default`; reason: `account_authority_private`.
- Representative fixture: `fx-botwriteaccessallowed` or a covering scenario below.
### MessageWebAppDataSent

- Source: `type.go:26070`
- Exact source fields: `button_text:ButtonText string`.
- Proposed Message fact projection: `button_text`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-webappdatasent` or a covering scenario below.
### MessageWebAppDataReceived

- Source: `type.go:26097`
- Exact source fields: `button_text:ButtonText string`, `data:Data string`.
- Proposed Message fact projection: content type plus exclusion reason only.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `excluded`; reason: `private_application_payload`.
- Representative fixture: `fx-webappdatareceived` or a covering scenario below.
### MessagePassportDataSent

- Source: `type.go:26126`
- Exact source fields: `types:Types []PassportElementType`.
- Proposed Message fact projection: `types`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: retain only bounded element-type discriminants.
- Proposed support status: `supported`; reason: `none`.
- Representative fixture: `fx-passportdatasent` or a covering scenario below.
### MessagePassportDataReceived

- Source: `type.go:26169`
- Exact source fields: `elements:Elements []*EncryptedPassportElement`, `credentials:Credentials *EncryptedCredentials`.
- Proposed Message fact projection: content type plus exclusion reason only.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `excluded`; reason: `identity_secret`.
- Representative fixture: `fx-passportdatareceived` or a covering scenario below.
### MessageProximityAlertTriggered

- Source: `type.go:26198`
- Exact source fields: `traveler_id:TravelerId MessageSender`, `watcher_id:WatcherId MessageSender`, `distance:Distance int32`.
- Proposed Message fact projection: `traveler_id`, `watcher_id`, `distance`.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `metadata_only_opt_in`; reason: `service_policy`.
- Representative fixture: `fx-proximityalerttriggered` or a covering scenario below.
### MessageUnsupported

- Source: `type.go:26252`
- Exact source fields: none.
- Proposed Message fact projection: stable unsupported type plus reason only.
- Direct/nested resource semantics: none identified by the top-level mapping.
- Privacy/local-state exclusions: global raw-object, credential, local-path and account-local-state exclusions only.
- Proposed support status: `unsupported`; reason: `tdlib_unsupported`.
- Representative fixture: `fx-unsupported` or a covering scenario below.

## Cross-Update And Aggregate Findings

- `UpdateNewMessage` supplies a complete `Message` observation at ingress.
- `UpdateMessageContent` supplies only chat ID, message ID and new content;
  it does not repeat sender, original time, reply, forwarding or album facts.
- `UpdateMessageEdited` supplies chat ID, message ID, edit date and reply
  markup; it does not supply replacement content.
- `UpdateDeleteMessages` supplies chat ID, message IDs, `is_permanent` and
  `from_cache`; deletion remains meaningful without a known predecessor.
- These updates provide no unified native event ID.
- `media_album_id` groups multiple Messages; it is not an attachment
  collection or evidence of complete Album membership.

## Representative Fixture Coverage

Research fixtures are in `m01-representative-fixtures.jsonl`. They are
non-approved semantic examples, not a wire schema. Every Message example
excludes Attachment, Relation, Delivery and Album-completeness state.
