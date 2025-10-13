# DateTime Utilities Package

This package provides standardized date/time handling utilities for consistent parsing, formatting, and validation across all system components (Go Lambda backend, Node.js frontend API, and Node.js edge authorizer).

## Features

- **Standardized Internal Format**: RFC3339 with timezone as the canonical format
- **Multiple Input Formats**: Accepts ISO8601, common date formats, and human-readable inputs
- **Context-Aware Output**: Formats appropriately for Microsoft Graph API, human display, logging, etc.
- **Comprehensive Validation**: Business rule validation with configurable parameters
- **Backward Compatibility**: Supports legacy separate date/time/timezone fields
- **Timezone Handling**: Robust timezone conversion and validation

## Quick Start

```go
import "github.com/your-org/your-repo/internal/datetime"

// Create a datetime manager
dt := datetime.New(nil) // uses default config

// Parse various formats
parsed, err := dt.Parse("2025-01-15 10:00 AM")
if err != nil {
    log.Fatal(err)
}

// Format for different contexts
rfc3339 := dt.Format(parsed).ToRFC3339()
graphAPI := dt.Format(parsed).ToMicrosoftGraph()
human := dt.Format(parsed).ToHumanReadable("America/New_York")

// Validate business rules
if err := dt.Validate(parsed).MeetingTime(); err != nil {
    log.Printf("Invalid meeting time: %v", err)
}
```

## Configuration

```go
config := &datetime.DateTimeConfig{
    DefaultTimezone: "America/New_York",
    AllowPastDates:  false,
    FutureTolerance: 5 * time.Minute,
}

dt := datetime.New(config)
```

## Supported Input Formats

### Date/Time Combined
- `2025-01-15T10:00:00-05:00` (RFC3339 with timezone)
- `2025-01-15T10:00:00Z` (UTC)
- `2025-01-15 10:00:00` (local time)
- `01/15/2025 10:00 AM` (US format with 12-hour time)
- `January 15, 2025 at 10:00 AM` (human-readable)

### Date Only
- `2025-01-15` (ISO format)
- `01/15/2025` (US format)
- `January 15, 2025` (human-readable)

### Time Only
- `10:00:00` (24-hour with seconds)
- `10:00` (24-hour)
- `10:00 AM` (12-hour)
- `10:00:00 PM` (12-hour with seconds)

## Output Formats

### Standard Formats
- **RFC3339**: `2025-01-15T10:00:00-05:00` (canonical internal format)
- **Microsoft Graph**: `2025-01-15T15:00:00.0000000` (UTC, specific precision)
- **ICS Calendar**: `20250115T150000Z` (UTC for calendar files)
- **Log Format**: `2025-01-15T15:00:00.000Z` (UTC with milliseconds)

### Human-Readable Formats
- **Human Readable**: `January 15, 2025 at 10:00 AM EST`
- **Email Template**: `Monday, January 15, 2025 at 10:00 AM EST`
- **Schedule Window**: `January 15, 2025 from 10:00 AM to 2:00 PM EST`

## Common Use Cases

### Meeting Scheduling

```go
dt := datetime.New(nil)

// Parse meeting time from user input
startTime, err := dt.Parse("01/15/2025 2:00 PM")
if err != nil {
    return err
}

// Validate it's a valid meeting time
if err := dt.Validate(startTime).MeetingTime(); err != nil {
    return fmt.Errorf("invalid meeting time: %w", err)
}

// Format for Microsoft Graph API
graphStart := dt.Format(startTime).ToMicrosoftGraph()

// Format for email notification
emailTime := dt.Format(startTime).ToEmailTemplate("America/New_York")
```

### Legacy Data Migration

```go
dt := datetime.New(nil)

// Parse existing separate fields
date := "2025-01-15"
timeStr := "14:00:00"
timezone := "America/New_York"

parsed, err := dt.ParseLegacy(date, timeStr, timezone)
if err != nil {
    return err
}

// Convert to new standardized format
standardized := dt.Format(parsed).ToRFC3339()

// Or convert back to legacy fields for backward compatibility
newDate, newTime, newTZ := dt.Format(parsed).ToLegacyFields()
```

### Timezone Conversion

```go
dt := datetime.New(nil)

// Parse time in Eastern timezone
eastTime, err := dt.ParseWithTimezone("2025-01-15 14:00", "America/New_York")
if err != nil {
    return err
}

// Convert to Pacific timezone
pacificTime, err := dt.Format(eastTime).ToTimezone("America/Los_Angeles")
if err != nil {
    return err
}

// Display in human-readable format
display := dt.Format(eastTime).ToHumanReadable("America/Los_Angeles")
```

### Schedule Window Validation

```go
dt := datetime.New(nil)

startTime, _ := dt.Parse("2025-01-15 14:00")
endTime, _ := dt.Parse("2025-01-15 16:00")

// Validate the range
if err := dt.ValidateRange(startTime, endTime); err != nil {
    return fmt.Errorf("invalid schedule: %w", err)
}

// Format for display
window := dt.FormatRange(startTime, endTime).ToScheduleWindow("America/New_York")
fmt.Printf("Implementation window: %s", window)
```

## Error Handling

The package provides standardized error types:

```go
if err != nil {
    if dtErr, ok := err.(*datetime.DateTimeError); ok {
        switch dtErr.Type {
        case datetime.ErrInvalidFormat:
            // Handle parsing errors
        case datetime.ErrInvalidTimezone:
            // Handle timezone errors
        case datetime.ErrInvalidRange:
            // Handle range validation errors
        case datetime.ErrPastDate:
            // Handle past date errors
        }
    }
}
```

## Validation Rules

### Meeting Time Validation
- Must be in the future (with configurable tolerance)
- Cannot be more than 2 years in the future
- Must have valid timezone information

### Business Hours Validation
- Monday through Friday only
- 8:00 AM to 6:00 PM in specified timezone

### Schedule Window Validation
- Minimum duration: 15 minutes
- Maximum duration: 24 hours
- Start time must be before end time

### Date Range Validation
- Both dates must be valid
- Start must be before end
- Maximum range: 1 year

## Integration with Existing Code

### Updating Existing Structs

```go
// Before
type ScheduleInfo struct {
    BeginDate string `json:"beginDate"`
    BeginTime string `json:"beginTime"`
    Timezone  string `json:"timezone"`
}

// After (with backward compatibility)
type ScheduleInfo struct {
    // New standardized fields
    ImplementationStart time.Time `json:"implementationStart"`
    ImplementationEnd   time.Time `json:"implementationEnd"`
    
    // Legacy fields (for backward compatibility)
    BeginDate string `json:"beginDate"`
    BeginTime string `json:"beginTime"`
    EndDate   string `json:"endDate"`
    EndTime   string `json:"endTime"`
    Timezone  string `json:"timezone"`
}

// Migration helper
func (s *ScheduleInfo) MigrateToStandardized(dt *datetime.Manager) error {
    start, err := dt.ParseLegacy(s.BeginDate, s.BeginTime, s.Timezone)
    if err != nil {
        return err
    }
    s.ImplementationStart = start
    
    end, err := dt.ParseLegacy(s.EndDate, s.EndTime, s.Timezone)
    if err != nil {
        return err
    }
    s.ImplementationEnd = end
    
    return nil
}
```

## Performance Considerations

- Timezone data is cached by Go's time package
- Parser tries formats in order of likelihood
- Formatter methods are optimized for common use cases
- Validation is designed to fail fast on obvious errors

## Thread Safety

All datetime utilities are thread-safe and can be used concurrently across goroutines. The Manager struct is immutable after creation.

## Testing

The package includes comprehensive test coverage. Run tests with:

```bash
go test ./internal/datetime/...
```

See `example_test.go` for usage examples and test patterns.