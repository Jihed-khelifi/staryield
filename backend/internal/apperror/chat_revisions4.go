package apperror

import "net/http"

// Replies, likes and client nicknames (revisions 4.0, §23.2–23.4).
const (
	// CodeReplyTargetInvalid: reply_to_id is not a text or image message of
	// the same conversation.
	CodeReplyTargetInvalid Code = "reply_target_invalid"
	// CodeMessageNotLikeable: system notices cannot be liked.
	CodeMessageNotLikeable Code = "message_not_likeable"
)

// ReplyTargetInvalid refuses a reply whose target cannot be quoted.
func ReplyTargetInvalid() *Error {
	return New(CodeReplyTargetInvalid, http.StatusUnprocessableEntity, "You can only reply to a text or image message of this conversation.")
}

// MessageNotLikeable refuses a like on a system notice.
func MessageNotLikeable() *Error {
	return New(CodeMessageNotLikeable, http.StatusUnprocessableEntity, "System notices can't be liked.")
}
