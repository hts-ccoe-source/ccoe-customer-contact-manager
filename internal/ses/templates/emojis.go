package templates

// NotificationType represents the type of notification being sent
type NotificationType string

const (
	NotificationApprovalRequest NotificationType = "approval_request"
	NotificationApproved        NotificationType = "approved"
	NotificationCompleted       NotificationType = "completed"
	NotificationCancelled       NotificationType = "cancelled"
	NotificationMeeting         NotificationType = "meeting"
)

// CategoryType represents the category of the event
type CategoryType string

const (
	CategoryCIC         CategoryType = "cic"
	CategoryFinOps      CategoryType = "finops"
	CategoryInnerSource CategoryType = "innersource"
	CategoryGeneral     CategoryType = "general"
	CategoryChange      CategoryType = "change"
)

// Emoji constants for different notification types and categories
const (
	EmojiApprovalRequest = "⚠️" // Yellow yield sign
	EmojiApprovedChange  = "🟢"  // Green circle (approved/go-ahead)
	EmojiCompleted       = "✅"  // Green checkmark
	EmojiCancelled       = "❌"  // Red X
	EmojiCIC             = "☁️" // Cloud
	EmojiFinOps          = "💰"  // Money bag
	EmojiInnerSource     = "🔧"  // Wrench
	EmojiGeneral         = "📢"  // Megaphone
	EmojiMeeting         = "📅"  // Calendar
	EmojiDefault         = "📧"  // Email (fallback)
)

// GetEmojiForNotification returns the appropriate emoji based on notification type and category
func GetEmojiForNotification(notificationType NotificationType, category CategoryType) string {
	// For approval requests, always use the warning emoji
	if notificationType == NotificationApprovalRequest {
		return EmojiApprovalRequest
	}

	// For completed notifications, always use the checkmark
	if notificationType == NotificationCompleted {
		return EmojiCompleted
	}

	// For cancelled notifications, always use the red X
	if notificationType == NotificationCancelled {
		return EmojiCancelled
	}

	// For meeting invitations, always use the calendar
	if notificationType == NotificationMeeting {
		return EmojiMeeting
	}

	// For approved notifications, use category-specific emojis for announcements
	// and green circle for changes
	if notificationType == NotificationApproved {
		switch category {
		case CategoryCIC:
			return EmojiCIC
		case CategoryFinOps:
			return EmojiFinOps
		case CategoryInnerSource:
			return EmojiInnerSource
		case CategoryGeneral:
			return EmojiGeneral
		case CategoryChange:
			return EmojiApprovedChange
		default:
			return EmojiDefault
		}
	}

	// Fallback to default emoji
	return EmojiDefault
}
