# Design Document

## Overview

This design standardizes all email communication templates in the CCOE Customer Contact Manager system. The system currently has announcement templates (CIC, FinOps, InnerSource, General) in `internal/ses/announcement_templates.go` but lacks standardization across all notification types (approval requests, approved notifications, meeting invitations, completions, and cancellations) for both announcements and changes.

The design introduces:

- Centralized configuration management for email addresses
- Consistent emoji usage in subject lines
- Standardized HTML/text template structure
- Unified footer with SES macros and taglines
- Mobile-optimized subject lines and content
- Minimal image usage for email client compatibility

## Architecture

### Configuration Layer

**Config Structure Enhancement**

Add new fields to `config.json`:

```json
{
  "email_config": {
    "sender_address": "ccoe@nonprod.ccoe.hearst.com",
    "meeting_organizer": "ccoe@hearst.com",
    "portal_base_url": "https://portal.example.com"
  }
}
```

**Config Type Definition** (`internal/config/config.go`):

```go
type EmailConfig struct {
    SenderAddress     string `json:"sender_address"`
    MeetingOrganizer  string `json:"meeting_organizer"`
    PortalBaseURL     string `json:"portal_base_url"`
}

type Config struct {
    // ... existing fields ...
    EmailConfig EmailConfig `json:"email_config"`
}
```

### Template Layer

**Unified Template Package** (`internal/ses/templates/`)

Create a new subpackage structure:

```
internal/ses/templates/
├── base.go           # Base template structures and common functions
├── emojis.go         # Emoji constants and mapping
├── announcements.go  # Announcement-specific templates
├── changes.go        # Change-specific templates
└── shared.go         # Shared template components (header, footer)
```

**Template Type Hierarchy**:

```go
// Base template data structure
type BaseTemplateData struct {
    EventID          string
    EventType        string    // "announcement" or "change"
    Category         string    // "cic", "finops", "innersource", "general", "change"
    Status           string    // Workflow state: "pending_approval", "approved", "completed", "cancelled", etc.
    Title            string
    Summary          string
    Content          string
    SenderAddress    string
    Timestamp        time.Time
    Attachments      []string  // URLs to attachments
}

// Approval record for tracking who approved and when
type ApprovalRecord struct {
    ApprovedBy       string
    ApprovedAt       time.Time
    ApproverEmail    string
}

// Notification type-specific data
type ApprovalRequestData struct {
    BaseTemplateData
    ApprovalURL      string
    Customers        []string
}

type ApprovedNotificationData struct {
    BaseTemplateData
    Approvals        []ApprovalRecord  // Multiple approvers with timestamps
}

type MeetingData struct {
    BaseTemplateData
    MeetingMetadata  *types.MeetingMetadata
    OrganizerEmail   string
}

type CompletionData struct {
    BaseTemplateData
    CompletedBy      string
    CompletedByEmail string
    CompletedAt      time.Time
}

type CancellationData struct {
    BaseTemplateData
    CancelledBy      string
    CancelledByEmail string
    CancelledAt      time.Time
}
```

## Components and Interfaces

### 1. Emoji Manager (`internal/ses/templates/emojis.go`)

```go
type NotificationType string

const (
    NotificationApprovalRequest NotificationType = "approval_request"
    NotificationApproved        NotificationType = "approved"
    NotificationCompleted       NotificationType = "completed"
    NotificationCancelled       NotificationType = "cancelled"
    NotificationMeeting         NotificationType = "meeting"
)

type CategoryType string

const (
    CategoryCIC         CategoryType = "cic"
    CategoryFinOps      CategoryType = "finops"
    CategoryInnerSource CategoryType = "innersource"
    CategoryGeneral     CategoryType = "general"
    CategoryChange      CategoryType = "change"
)

// Emoji constants
const (
    EmojiApprovalRequest = "⚠️"   // Yellow yield sign
    EmojiApprovedChange  = "🟢"   // Green circle (approved/go-ahead)
    EmojiCompleted       = "✅"   // Green checkmark
    EmojiCancelled       = "❌"   // Red X
    EmojiCIC             = "☁️"   // Cloud
    EmojiFinOps          = "💰"   // Money bag
    EmojiInnerSource     = "🔧"   // Wrench
    EmojiGeneral         = "📢"   // Megaphone
    EmojiMeeting         = "📅"   // Calendar
)

func GetEmojiForNotification(notificationType NotificationType, category CategoryType) string {
    // Returns appropriate emoji based on notification type and category
}
```

### 2. Template Builder (`internal/ses/templates/base.go`)

```go
type EmailTemplate struct {
    Subject  string
    HTMLBody string
    TextBody string
}

type TemplateBuilder interface {
    BuildApprovalRequest(data ApprovalRequestData) EmailTemplate
    BuildApprovedNotification(data ApprovedNotificationData) EmailTemplate
    BuildMeetingInvitation(data MeetingData) EmailTemplate
    BuildCompletion(data CompletionData) EmailTemplate
    BuildCancellation(data CancellationData) EmailTemplate
}

// Concrete implementations
type AnnouncementTemplateBuilder struct {
    config EmailConfig
}

type ChangeTemplateBuilder struct {
    config EmailConfig
}
```

### 3. Shared Components (`internal/ses/templates/shared.go`)

```go
// Common HTML structure
func renderHiddenMetadata(eventID string, eventType string, notificationType string) string  // Hidden HTML fields for tracking
func renderHTMLHeader(emoji string, title string, backgroundColor string) string
func renderStatusSubtitle(status string) string  // Renders workflow state subtitle
func renderAttachments(attachments []string) string  // Renders attachment links
func renderHTMLFooter(eventID string, timestamp time.Time) string
func renderSESMacro() string

// Common text structure
func renderTextHeader(emoji string, title string) string
func renderTextStatusLine(status string) string  // Renders workflow state for text emails
func renderTextAttachments(attachments []string) string  // Renders attachment URLs for text emails
func renderTextFooter(eventID string, timestamp time.Time) string

// Status display mapping
func getStatusDisplay(status string) string {
    // Maps internal status codes to user-friendly display text
    // Examples:
    // "pending_approval" -> "Pending Approval"
    // "approved" -> "Approved"
    // "completed" -> "Completed"
    // "cancelled" -> "Cancelled"
}

// Mobile-optimized subject line builder
func buildSubject(emoji string, title string) string {
    // Builds mobile-friendly subject line
    // Format: "{emoji} {title}"
    // Example: "⚠️ Q4 Cloud Planning Session"
    // The emoji conveys the notification type, no action verb needed
    // Note: Title length is enforced by portal UI, no truncation needed here
}

// Tagline builder with hyperlinked event ID
func buildTagline(eventID string, eventType string, baseURL string) string {
    // Returns HTML: "event ID <a href='{url}'>{eventID}</a> sent by the CCOE customer contact manager"
    // The event ID is a clickable hyperlink that takes users directly to the event
    // URL format based on event type:
    // - Announcements: {baseURL}/edit-announcement.html?announcementId={eventID}
    // - Changes: {baseURL}/edit-change.html?changeId={eventID}
    // Example HTML: event ID <a href="https://portal.example.com/edit-announcement.html?announcementId=CIC-123456">CIC-123456</a> sent by the CCOE customer contact manager
}

// Tagline builder for plain text emails
func buildTaglineText(eventID string, eventType string, baseURL string) string {
    // Returns plain text with full URL
    // Example: "event ID CIC-123456 (https://portal.example.com/edit-announcement.html?announcementId=CIC-123456) sent by the CCOE customer contact manager"
}
```

## Data Models

### Email Configuration Model

```go
type EmailConfig struct {
    SenderAddress    string
    MeetingOrganizer string
    PortalBaseURL    string
}

func (c *EmailConfig) Validate() error {
    if c.SenderAddress == "" {
        return errors.New("sender_address is required in email_config")
    }
    if c.MeetingOrganizer == "" {
        return errors.New("meeting_organizer is required in email_config")
    }
    if c.PortalBaseURL == "" {
        return errors.New("portal_base_url is required in email_config")
    }
    // Validate email format
    // Validate URL format
    return nil
}
```

### Template Registry

```go
type TemplateRegistry struct {
    announcementBuilder TemplateBuilder
    changeBuilder       TemplateBuilder
    config              EmailConfig
}

func NewTemplateRegistry(config EmailConfig) *TemplateRegistry {
    return &TemplateRegistry{
        announcementBuilder: &AnnouncementTemplateBuilder{config: config},
        changeBuilder:       &ChangeTemplateBuilder{config: config},
        config:              config,
    }
}

func (r *TemplateRegistry) GetTemplate(
    eventType string,
    notificationType NotificationType,
    data interface{},
) (EmailTemplate, error) {
    // Routes to appropriate builder based on event type
}
```

## HTML Template Structure

### Standard HTML Layout

All HTML emails will follow this structure:

```html
<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        /* Inline CSS for email client compatibility */
        body { 
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Arial, sans-serif;
            line-height: 1.6; 
            color: #333;
            margin: 0;
            padding: 0;
        }
        .email-container {
            max-width: 600px;
            margin: 0 auto;
        }
        .header {
            padding: 20px;
            color: white;
            /* Background color varies by category */
        }
        .content {
            padding: 20px;
            background-color: #ffffff;
        }
        .footer {
            background-color: #f5f5f5;
            padding: 15px 20px;
            font-size: 0.9em;
            color: #666;
        }
        .unsubscribe {
            background-color: #e9ecef;
            padding: 15px 20px;
            margin-top: 20px;
        }
        /* Mobile-responsive */
        @media only screen and (max-width: 600px) {
            .email-container {
                width: 100% !important;
            }
        }
    </style>
</head>
<body>
    <!-- Hidden metadata for email threading and tracking -->
    <div style="display:none; max-height:0px; overflow:hidden;">
        <span id="event-id">{EVENT_ID}</span>
        <span id="event-type">{EVENT_TYPE}</span>
        <span id="notification-type">{NOTIFICATION_TYPE}</span>
    </div>
    
    <div class="email-container">
        <!-- Header with emoji and title -->
        <div class="header">
            <h1>{emoji} {Category Name}</h1>
        </div>
        
        <!-- Main content -->
        <div class="content">
            <!-- Status subtitle based on workflow state -->
            <div class="status-subtitle" style="color: #6c757d; font-size: 0.9em; margin-bottom: 15px;">
                Status: {STATUS_DISPLAY}
            </div>
            
            <!-- Notification-specific content -->
            
            <!-- Attachments section (if any) -->
            <div class="attachments" style="margin-top: 20px;">
                <h3 style="font-size: 1em; color: #495057;">📎 Attachments</h3>
                <!-- List of attachment links -->
            </div>
        </div>
        
        <!-- Footer with tagline -->
        <div class="footer">
            <!-- The event ID is a clickable hyperlink back to the event -->
            <p>event ID <a href="{EVENT_URL}" style="color: #007bff; text-decoration: none;">{EVENT_ID}</a> sent by the CCOE customer contact manager</p>
            <!-- EVENT_URL format:
                 Announcements: {PORTAL_BASE_URL}/edit-announcement.html?announcementId={EVENT_ID}
                 Changes: {PORTAL_BASE_URL}/edit-change.html?changeId={EVENT_ID}
            -->
        </div>
        
        <!-- SES unsubscribe macro -->
        <div class="unsubscribe">
            <p>Notification sent at {timestamp}</p>
            <p><a href="{{amazonSESUnsubscribeUrl}}">📧 Manage Email Preferences or Unsubscribe</a></p>
        </div>
    </div>
</body>
</html>
```

### Color Scheme by Category

```go
var categoryColors = map[CategoryType]string{
    CategoryCIC:         "#0066cc", // Blue
    CategoryFinOps:      "#28a745", // Green
    CategoryInnerSource: "#6f42c1", // Purple
    CategoryGeneral:     "#007bff", // Light blue
    CategoryChange:      "#fd7e14", // Orange
}
```

## Subject Line Patterns

### Mobile-Optimized Format

All subject lines follow this pattern to ensure mobile preview compatibility:

```
{emoji} {event_title}
```

The emoji conveys the notification type, eliminating the need for action verbs and allowing the full subject to focus on the event title.

**Examples**:

- `⚠️ Q4 Cloud Planning Session` (approval request)
- `🟢 Database Migration` (approved change)
- `✅ FinOps Cost Review Meeting` (completed)
- `❌ Code Review Session` (cancelled)
- `☁️ Cloud Architecture Workshop` (CIC announcement)
- `💰 Q3 Cost Optimization Review` (FinOps announcement)
- `🔧 InnerSource Guild Kickoff` (InnerSource announcement)

**Implementation**:

- Emoji always first character
- Event title immediately after emoji (no action verbs needed)
- Event IDs are NOT included in subject lines (they appear in the email body and tagline)
- Title length is enforced by the portal UI (48-50 character maximum)
- Email templates use the title as-is without truncation

## Error Handling

### Configuration Validation

```go
func ValidateEmailConfig(config Config) error {
    if config.EmailConfig.SenderAddress == "" {
        return fmt.Errorf("email_config.sender_address is required")
    }
    if config.EmailConfig.MeetingOrganizer == "" {
        return fmt.Errorf("email_config.meeting_organizer is required")
    }
    if config.EmailConfig.PortalBaseURL == "" {
        return fmt.Errorf("email_config.portal_base_url is required")
    }
    
    // Validate email format
    if !isValidEmail(config.EmailConfig.SenderAddress) {
        return fmt.Errorf("invalid sender_address format: %s", config.EmailConfig.SenderAddress)
    }
    if !isValidEmail(config.EmailConfig.MeetingOrganizer) {
        return fmt.Errorf("invalid meeting_organizer format: %s", config.EmailConfig.MeetingOrganizer)
    }
    
    // Validate URL format
    if !isValidURL(config.EmailConfig.PortalBaseURL) {
        return fmt.Errorf("invalid portal_base_url format: %s", config.EmailConfig.PortalBaseURL)
    }
    
    return nil
}
```

### Template Rendering Errors

```go
type TemplateError struct {
    TemplateType string
    Field        string
    Err          error
}

func (e *TemplateError) Error() string {
    return fmt.Sprintf("template error in %s.%s: %v", e.TemplateType, e.Field, e.Err)
}
```

### Fallback Behavior

- If emoji lookup fails, use default emoji (📧)
- If template rendering fails, log error and use plain text fallback
- If config values missing, fail fast at startup (no runtime failures)

## Testing Strategy

### Unit Tests

**Config Validation Tests** (`internal/config/config_test.go`):

- Test valid email configuration
- Test missing sender_address
- Test missing meeting_organizer
- Test missing portal_base_url
- Test invalid email formats
- Test invalid URL format
- Test config loading from JSON

**Emoji Manager Tests** (`internal/ses/templates/emojis_test.go`):

- Test emoji selection for each notification type
- Test emoji selection for each category
- Test fallback emoji behavior

**Template Builder Tests** (`internal/ses/templates/base_test.go`):

- Test subject line generation (length limits, truncation)
- Test HTML template rendering
- Test text template rendering
- Test footer generation with correct taglines
- Test SES macro inclusion

**Shared Components Tests** (`internal/ses/templates/shared_test.go`):

- Test mobile-optimized subject line builder
- Test tagline format for different event IDs
- Test HTML sanitization
- Test text formatting

### Integration Tests

**End-to-End Template Tests** (`internal/ses/templates/integration_test.go`):

- Test complete email generation for each notification type
- Test announcement templates (all categories)
- Test change templates
- Verify all templates include SES macro
- Verify all templates include correct tagline
- Verify all templates have emoji in subject
- Verify mobile preview compatibility (subject length)

**Config Integration Tests** (`internal/config/integration_test.go`):

- Test loading config.json with email_config
- Test config validation at startup
- Test template registry initialization with config

### Visual Regression Tests

**Email Rendering Tests**:

- Generate sample HTML for each template type
- Save to `testing/email-samples/` directory
- Manual review in multiple email clients:
  - Gmail (web, mobile)
  - Outlook (web, desktop)
  - Apple Mail (macOS, iOS)
  - Verify images are minimal/absent
  - Verify mobile preview shows key information

## Migration Strategy

No backwards compatibility required - this is a clean replacement of existing templates.

### Phase 1: Add Configuration

1. Update `config.json` with `email_config` section
2. Update `internal/config/config.go` with new types
3. Add validation logic
4. Update application initialization to validate config

### Phase 2: Create Template Infrastructure

1. Create `internal/ses/templates/` package structure
2. Implement emoji manager
3. Implement shared components (header, footer, tagline, hidden metadata)
4. Implement base template structures

### Phase 3: Replace Announcement Templates

1. Create `internal/ses/templates/announcements.go`
2. Implement all announcement notification templates with new structure:
   - Approval request
   - Approved notification
   - Meeting invitation
   - Completion
   - Cancellation
3. Replace existing `announcement_templates.go` functions

### Phase 4: Create Change Templates

1. Create `internal/ses/templates/changes.go`
2. Implement all change notification templates:
   - Approval request
   - Approved notification
   - Meeting invitation
   - Completion
   - Cancellation
3. Use consistent structure with announcement templates

### Phase 5: Update Callers

1. Update `internal/ses/operations.go` to use new template registry
2. Update `internal/lambda/handlers.go` to pass config to templates
3. Update any other code that generates emails

### Phase 6: Remove Old Code

1. Delete old `announcement_templates.go` file
2. Remove any deprecated template functions
3. Update documentation

## Performance Considerations

### Template Caching

```go
type TemplateCache struct {
    mu        sync.RWMutex
    templates map[string]*template.Template
}

func (c *TemplateCache) Get(key string) (*template.Template, bool)
func (c *TemplateCache) Set(key string, tmpl *template.Template)
```

### Lazy Initialization

- Template registry initialized once at startup
- HTML templates parsed once and cached
- Config loaded once and validated at startup

### Memory Efficiency

- Use string builders for HTML generation
- Avoid unnecessary string concatenation
- Reuse template structures where possible

## Security Considerations

### Input Sanitization

```go
func sanitizeHTML(input string) string {
    // Escape HTML special characters
    // Prevent XSS in email content
}

func sanitizeSubject(input string) string {
    // Remove newlines and control characters
    // Prevent header injection
}
```

### Email Address Validation

```go
func isValidEmail(email string) bool {
    // RFC 5322 compliant email validation
    // Prevent email injection attacks
}
```

### Content Security

- All user-provided content HTML-escaped
- No JavaScript in HTML emails
- No external resource loading (images, CSS)
- Inline CSS only for styling

## Monitoring and Observability

### Logging

```go
// Log template generation
log.Info("generating email template",
    "notification_type", notificationType,
    "category", category,
    "event_id", eventID)

// Log config validation
log.Info("email configuration validated",
    "sender_address", config.SenderAddress,
    "meeting_organizer", config.MeetingOrganizer)

// Log template errors
log.Error("template generation failed",
    "error", err,
    "notification_type", notificationType,
    "event_id", eventID)
```

### Metrics

- Count of emails generated by type
- Count of template errors
- Template generation duration
- Config validation failures

## Documentation Requirements

### Code Documentation

- Godoc comments for all public functions
- Examples for template builders
- Configuration schema documentation

### User Documentation

- Update deployment guide with new config requirements
- Document email template structure
- Provide examples of each notification type
- Document emoji usage patterns

### Developer Documentation

- Template development guide
- How to add new notification types
- How to customize templates
- Testing guide for email templates
