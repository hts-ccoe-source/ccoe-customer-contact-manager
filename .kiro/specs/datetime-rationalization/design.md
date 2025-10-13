# Design Document

## Overview

This design establishes a unified date/time handling strategy across all system components: Go Lambda backend, Node.js frontend API, and Node.js authorizer Lambda at edge. The solution provides consistent parsing, storage, validation, and formatting of temporal data throughout the entire application stack.

## Architecture

### Core Principles

1. **Single Source of Truth**: RFC3339/ISO8601 with timezone as the canonical internal format
2. **Language-Agnostic Standards**: Identical behavior across Go and Node.js implementations
3. **Centralized Utilities**: Dedicated date/time modules in each language with equivalent functionality
4. **Graceful Input Handling**: Accept multiple input formats, normalize to standard format
5. **Context-Aware Output**: Format appropriately for different consumers (APIs, humans, logs)

### Component Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Go Lambda     │    │  Node.js API    │    │ Node.js Edge    │
│   Backend       │    │   Frontend      │    │   Authorizer    │
├─────────────────┤    ├─────────────────┤    ├─────────────────┤
│ datetime/       │    │ datetime/       │    │ datetime/       │
│ ├─ parser.go    │    │ ├─ parser.js    │    │ ├─ parser.js    │
│ ├─ formatter.go │    │ ├─ formatter.js │    │ ├─ formatter.js │
│ ├─ validator.go │    │ ├─ validator.js │    │ ├─ validator.js │
│ └─ types.go     │    │ └─ types.js     │    │ └─ types.js     │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌─────────────────┐
                    │ Shared Standards│
                    │ - RFC3339 Format│
                    │ - Timezone Rules│
                    │ - Validation    │
                    │ - Error Messages│
                    └─────────────────┘
```

## Components and Interfaces

### 1. Standard Internal Format

**Canonical Format**: RFC3339 with timezone
```
2025-01-15T10:00:00-05:00  (with timezone offset)
2025-01-15T15:00:00Z       (UTC)
```

**Internal Storage**: 
- Go: `time.Time` type
- Node.js: `Date` object with timezone metadata

### 2. Parser Module

#### Go Implementation (`internal/datetime/parser.go`)
```go
type Parser struct {
    DefaultTimezone *time.Location
}

func (p *Parser) ParseDateTime(input string) (time.Time, error)
func (p *Parser) ParseDate(input string) (time.Time, error)
func (p *Parser) ParseTime(input string, date time.Time) (time.Time, error)
func (p *Parser) ParseWithFormats(input string, formats []string) (time.Time, error)
```

#### Node.js Implementation (`datetime/parser.js`)
```javascript
class Parser {
    constructor(defaultTimezone = 'America/New_York') {
        this.defaultTimezone = defaultTimezone;
    }
    
    parseDateTime(input) { /* equivalent to Go */ }
    parseDate(input) { /* equivalent to Go */ }
    parseTime(input, date) { /* equivalent to Go */ }
    parseWithFormats(input, formats) { /* equivalent to Go */ }
}
```

#### Supported Input Formats
1. **ISO8601/RFC3339**: `2025-01-15T10:00:00-05:00`
2. **Date Only**: `2025-01-15`, `01/15/2025`, `January 15, 2025`
3. **Time Only**: `10:00`, `10:00:00`, `10:00 AM`, `22:00`
4. **Combined**: `2025-01-15 10:00`, `01/15/2025 10:00 AM`
5. **Legacy Formats**: Handle existing application formats gracefully

### 3. Formatter Module

#### Go Implementation (`internal/datetime/formatter.go`)
```go
type Formatter struct {
    DefaultTimezone *time.Location
}

func (f *Formatter) ToRFC3339(t time.Time) string
func (f *Formatter) ToMicrosoftGraph(t time.Time) string
func (f *Formatter) ToHumanReadable(t time.Time, timezone string) string
func (f *Formatter) ToICS(t time.Time) string
func (f *Formatter) ToLogFormat(t time.Time) string
```

#### Node.js Implementation (`datetime/formatter.js`)
```javascript
class Formatter {
    constructor(defaultTimezone = 'America/New_York') {
        this.defaultTimezone = defaultTimezone;
    }
    
    toRFC3339(date) { /* equivalent to Go */ }
    toMicrosoftGraph(date) { /* equivalent to Go */ }
    toHumanReadable(date, timezone) { /* equivalent to Go */ }
    toICS(date) { /* equivalent to Go */ }
    toLogFormat(date) { /* equivalent to Go */ }
}
```

#### Output Formats
1. **RFC3339**: `2025-01-15T10:00:00-05:00` (internal standard)
2. **Microsoft Graph**: `2025-01-15T15:00:00.0000000` (UTC, specific precision)
3. **Human Readable**: `January 15, 2025 at 10:00 AM EST`
4. **ICS Calendar**: `20250115T150000Z` (UTC for calendar files)
5. **Log Format**: `2025-01-15T15:00:00.000Z` (UTC with milliseconds)

### 4. Validator Module

#### Go Implementation (`internal/datetime/validator.go`)
```go
type Validator struct {
    AllowPastDates bool
    FutureTolerance time.Duration
}

func (v *Validator) ValidateDateTime(t time.Time) error
func (v *Validator) ValidateDateRange(start, end time.Time) error
func (v *Validator) ValidateTimezone(tz string) error
func (v *Validator) ValidateMeetingTime(t time.Time) error
```

#### Node.js Implementation (`datetime/validator.js`)
```javascript
class Validator {
    constructor(options = {}) {
        this.allowPastDates = options.allowPastDates || false;
        this.futureTolerance = options.futureTolerance || 300000; // 5 minutes
    }
    
    validateDateTime(date) { /* equivalent to Go */ }
    validateDateRange(start, end) { /* equivalent to Go */ }
    validateTimezone(tz) { /* equivalent to Go */ }
    validateMeetingTime(date) { /* equivalent to Go */ }
}
```

### 5. Types and Constants

#### Go Implementation (`internal/datetime/types.go`)
```go
const (
    RFC3339Format = "2006-01-02T15:04:05Z07:00"
    GraphFormat   = "2006-01-02T15:04:05.0000000"
    ICSFormat     = "20060102T150405Z"
    LogFormat     = "2006-01-02T15:04:05.000Z"
)

type DateTimeConfig struct {
    DefaultTimezone string
    AllowPastDates  bool
    FutureTolerance time.Duration
}
```

#### Node.js Implementation (`datetime/types.js`)
```javascript
const FORMATS = {
    RFC3339: 'YYYY-MM-DDTHH:mm:ssZ',
    GRAPH: 'YYYY-MM-DDTHH:mm:ss.0000000',
    ICS: 'YYYYMMDDTHHmmssZ',
    LOG: 'YYYY-MM-DDTHH:mm:ss.SSSZ'
};

class DateTimeConfig {
    constructor(options = {}) {
        this.defaultTimezone = options.defaultTimezone || 'America/New_York';
        this.allowPastDates = options.allowPastDates || false;
        this.futureTolerance = options.futureTolerance || 300000;
    }
}
```

## Data Models

### Internal DateTime Representation

#### Go Struct Updates
```go
// Update existing types to use time.Time consistently
type ScheduleInfo struct {
    ImplementationStart time.Time `json:"implementationStart"`
    ImplementationEnd   time.Time `json:"implementationEnd"`
    BeginDate          string    `json:"beginDate"` // Keep for backward compatibility
    BeginTime          string    `json:"beginTime"` // Keep for backward compatibility
    EndDate            string    `json:"endDate"`   // Keep for backward compatibility
    EndTime            string    `json:"endTime"`   // Keep for backward compatibility
    Timezone           string    `json:"timezone"`
}

type MeetingInvite struct {
    Title           string    `json:"title"`
    StartTime       time.Time `json:"startTime"`
    Duration        int       `json:"duration"`
    DurationMinutes int       `json:"durationMinutes"`
    Location        string    `json:"location"`
}
```

#### Node.js Schema Updates
```javascript
// Equivalent schemas for Node.js components
const ScheduleInfoSchema = {
    implementationStart: Date,
    implementationEnd: Date,
    beginDate: String, // backward compatibility
    beginTime: String, // backward compatibility
    endDate: String,   // backward compatibility
    endTime: String,   // backward compatibility
    timezone: String
};

const MeetingInviteSchema = {
    title: String,
    startTime: Date,
    duration: Number,
    durationMinutes: Number,
    location: String
};
```

### Migration Strategy

1. **Phase 1**: Add new time.Time fields alongside existing string fields
2. **Phase 2**: Update all parsing to populate both old and new fields
3. **Phase 3**: Update all consumers to use new fields
4. **Phase 4**: Remove old string fields (breaking change, major version)

## Error Handling

### Standardized Error Types

#### Go Errors
```go
type DateTimeError struct {
    Type    string
    Message string
    Input   string
    Cause   error
}

const (
    ErrInvalidFormat   = "INVALID_FORMAT"
    ErrInvalidTimezone = "INVALID_TIMEZONE"
    ErrInvalidRange    = "INVALID_RANGE"
    ErrPastDate        = "PAST_DATE"
)
```

#### Node.js Errors
```javascript
class DateTimeError extends Error {
    constructor(type, message, input, cause) {
        super(message);
        this.type = type;
        this.input = input;
        this.cause = cause;
    }
}

const ERROR_TYPES = {
    INVALID_FORMAT: 'INVALID_FORMAT',
    INVALID_TIMEZONE: 'INVALID_TIMEZONE',
    INVALID_RANGE: 'INVALID_RANGE',
    PAST_DATE: 'PAST_DATE'
};
```

### Error Messages

Standardized error messages across all components:
- `"Invalid date format: expected YYYY-MM-DD, got '${input}'"`
- `"Invalid timezone: '${timezone}' is not a valid IANA timezone"`
- `"Invalid date range: start time must be before end time"`
- `"Meeting time cannot be in the past (tolerance: 5 minutes)"`

## Testing Strategy

### Unit Tests

1. **Parser Tests**: Test all supported input formats across both languages
2. **Formatter Tests**: Verify identical output formats between Go and Node.js
3. **Validator Tests**: Ensure consistent validation rules
4. **Cross-Language Tests**: Compare outputs between Go and Node.js implementations

### Integration Tests

1. **End-to-End Flow**: Test date/time data flow from frontend → API → Lambda
2. **Microsoft Graph Integration**: Test meeting creation with proper timestamps
3. **Email Template Integration**: Test human-readable formatting
4. **Calendar Integration**: Test ICS format generation

### Test Data

Shared test cases across all implementations:
```json
{
  "validInputs": [
    "2025-01-15T10:00:00-05:00",
    "2025-01-15",
    "01/15/2025",
    "10:00 AM",
    "January 15, 2025 10:00 AM"
  ],
  "invalidInputs": [
    "invalid-date",
    "2025-13-01",
    "25:00:00"
  ],
  "expectedOutputs": {
    "rfc3339": "2025-01-15T10:00:00-05:00",
    "microsoftGraph": "2025-01-15T15:00:00.0000000",
    "humanReadable": "January 15, 2025 at 10:00 AM EST"
  }
}
```

## Implementation Notes

### Go-Specific Considerations

1. Use `time.Time` consistently throughout the codebase
2. Leverage Go's built-in timezone handling with `time.LoadLocation()`
3. Use `time.Parse()` with multiple format attempts
4. Implement custom JSON marshaling/unmarshaling for backward compatibility

### Node.js-Specific Considerations

1. Use libraries like `date-fns-tz` or `luxon` for robust timezone handling
2. Ensure consistent behavior with Go's time parsing
3. Handle timezone conversion carefully to match Go's behavior
4. Use ISO string methods for serialization

### Edge Function Considerations

1. Keep datetime utilities lightweight for edge deployment
2. Cache timezone data to avoid repeated lookups
3. Minimize dependencies for faster cold starts
4. Focus on validation and basic formatting only