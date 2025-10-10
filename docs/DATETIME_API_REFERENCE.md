# DateTime Utilities API Reference

## Overview

This document provides a complete API reference for the datetime utilities used across all system components. The utilities are implemented in both Go and Node.js with identical functionality and behavior.

## Package Structure

### Go Package: `internal/datetime`
```
internal/datetime/
├── parser.go      # DateTime parsing functions
├── formatter.go   # DateTime formatting functions  
├── validator.go   # DateTime validation functions
├── types.go       # Types, constants, and configuration
└── errors.go      # Error types and handling
```

### Node.js Package: `datetime`
```
datetime/
├── parser.js      # DateTime parsing functions
├── formatter.js   # DateTime formatting functions
├── validator.js   # DateTime validation functions
├── types.js       # Types, constants, and configuration
├── errors.js      # Error types and handling
└── index.js       # Package exports
```

## Configuration

### DateTimeConfig

Configuration object used by all datetime utilities.

#### Go Implementation
```go
type DateTimeConfig struct {
    DefaultTimezone string        // Default timezone for parsing (default: "America/New_York")
    AllowPastDates  bool         // Allow dates in the past (default: false)
    FutureTolerance time.Duration // Tolerance for "future" validation (default: 5 minutes)
}
```

#### Node.js Implementation
```javascript
class DateTimeConfig {
    constructor(options = {}) {
        this.defaultTimezone = options.defaultTimezone || 'America/New_York';
        this.allowPastDates = options.allowPastDates || false;
        this.futureTolerance = options.futureTolerance || 300000; // 5 minutes in ms
    }
}
```

## Parser Module

### Go API

#### Constructor
```go
func NewParser(config *DateTimeConfig) *Parser
```
Creates a new Parser instance with the given configuration. If config is nil, uses default configuration.

#### Methods

##### ParseDateTime
```go
func (p *Parser) ParseDateTime(input string) (time.Time, error)
```
Parses a date/time string in various formats and returns a time.Time object.

**Parameters:**
- `input`: Date/time string in supported format

**Returns:**
- `time.Time`: Parsed date/time
- `error`: DateTimeError if parsing fails

**Supported Formats:**
- RFC3339: `2025-01-15T10:00:00-05:00`
- ISO8601: `2025-01-15T10:00:00Z`
- Combined: `2025-01-15 10:00:00`
- US Format: `01/15/2025 10:00 AM`
- Natural: `January 15, 2025 at 10:00 AM`

##### ParseDate
```go
func (p *Parser) ParseDate(input string) (time.Time, error)
```
Parses a date-only string and returns a time.Time object at midnight in the default timezone.

**Parameters:**
- `input`: Date string in supported format

**Supported Formats:**
- ISO: `2025-01-15`
- US: `01/15/2025`, `1/15/2025`
- Natural: `January 15, 2025`, `Jan 15, 2025`
- European: `15 January 2025`, `15 Jan 2025`

##### ParseTime
```go
func (p *Parser) ParseTime(input string, baseDate time.Time) (time.Time, error)
```
Parses a time-only string and combines it with the given base date.

**Parameters:**
- `input`: Time string in supported format
- `baseDate`: Base date to combine with the time

**Supported Formats:**
- 24-hour: `15:04:05`, `15:04`
- 12-hour: `3:04:05 PM`, `3:04 PM`, `3:04PM`

##### ParseWithFormats
```go
func (p *Parser) ParseWithFormats(input string, formats []string) (time.Time, error)
```
Attempts to parse input using the provided format strings in order.

**Parameters:**
- `input`: Date/time string to parse
- `formats`: Array of Go time format strings to try

##### ParseLegacyDateTimeFields
```go
func (p *Parser) ParseLegacyDateTimeFields(beginDate, beginTime, timezone string) (time.Time, error)
```
Parses legacy separate date/time fields into a single time.Time object.

**Parameters:**
- `beginDate`: Date string (e.g., "2025-01-15")
- `beginTime`: Time string (e.g., "10:00")
- `timezone`: Timezone string (e.g., "America/New_York")

##### ParseDateTimeWithTimezone
```go
func (p *Parser) ParseDateTimeWithTimezone(input, timezone string) (time.Time, error)
```
Parses a date/time string and applies the specified timezone.

**Parameters:**
- `input`: Date/time string
- `timezone`: IANA timezone identifier

### Node.js API

#### Constructor
```javascript
class Parser {
    constructor(config = null)
}
```
Creates a new Parser instance with the given configuration. If config is null, uses default configuration.

#### Methods

All methods have the same signatures and behavior as the Go implementation, with JavaScript naming conventions:

##### parseDateTime
```javascript
parseDateTime(input)
```
Returns a Date object or throws DateTimeError.

##### parseDate
```javascript
parseDate(input)
```
Returns a Date object set to midnight in the default timezone.

##### parseTime
```javascript
parseTime(input, baseDate)
```
Combines time with base date and returns Date object.

##### parseWithFormats
```javascript
parseWithFormats(input, formats)
```
Attempts parsing with provided format strings (using Day.js format syntax).

##### parseLegacyDateTimeFields
```javascript
parseLegacyDateTimeFields(beginDate, beginTime, timezone)
```
Parses legacy separate fields into Date object.

##### parseDateTimeWithTimezone
```javascript
parseDateTimeWithTimezone(input, timezone)
```
Parses with explicit timezone application.

## Formatter Module

### Go API

#### Constructor
```go
func NewFormatter(config *DateTimeConfig) *Formatter
```

#### Methods

##### ToRFC3339
```go
func (f *Formatter) ToRFC3339(t time.Time) string
```
Formats time as RFC3339 string with timezone offset.
**Output:** `2025-01-15T10:00:00-05:00`

##### ToMicrosoftGraph
```go
func (f *Formatter) ToMicrosoftGraph(t time.Time) string
```
Formats time for Microsoft Graph API (UTC with specific precision).
**Output:** `2025-01-15T15:00:00.0000000`

##### ToHumanReadable
```go
func (f *Formatter) ToHumanReadable(t time.Time, timezone string) string
```
Formats time in human-readable format for the specified timezone.
**Output:** `January 15, 2025 at 10:00 AM EST`

##### ToICS
```go
func (f *Formatter) ToICS(t time.Time) string
```
Formats time for ICS calendar files (UTC).
**Output:** `20250115T150000Z`

##### ToLogFormat
```go
func (f *Formatter) ToLogFormat(t time.Time) string
```
Formats time for logging (UTC with milliseconds).
**Output:** `2025-01-15T15:00:00.000Z`

##### ToDateOnly
```go
func (f *Formatter) ToDateOnly(t time.Time) string
```
Formats only the date portion.
**Output:** `2025-01-15`

##### ToTimeOnly
```go
func (f *Formatter) ToTimeOnly(t time.Time) string
```
Formats only the time portion (24-hour).
**Output:** `15:04:05`

##### ToTimeOnly12Hour
```go
func (f *Formatter) ToTimeOnly12Hour(t time.Time) string
```
Formats only the time portion (12-hour).
**Output:** `3:04:05 PM`

##### ToEmailTemplate
```go
func (f *Formatter) ToEmailTemplate(t time.Time, timezone string) string
```
Formats time for email templates with day of week.
**Output:** `Monday, January 15, 2025 at 10:00 AM EST`

##### ToScheduleWindow
```go
func (f *Formatter) ToScheduleWindow(start, end time.Time, timezone string) string
```
Formats a time range for schedule displays.
**Output:** `January 15, 2025 from 10:00 AM EST to 2:00 PM EST`

##### FormatDuration
```go
func (f *Formatter) FormatDuration(d time.Duration) string
```
Formats a duration in human-readable format.
**Output:** `2 hours 30 minutes`

##### ToTimezone
```go
func (f *Formatter) ToTimezone(t time.Time, timezone string) time.Time
```
Converts time to the specified timezone.

### Node.js API

#### Constructor
```javascript
class Formatter {
    constructor(config = null)
}
```

#### Methods

All methods have the same signatures and behavior as the Go implementation:

- `toRFC3339(date)`
- `toMicrosoftGraph(date)`
- `toHumanReadable(date, timezone)`
- `toICS(date)`
- `toLogFormat(date)`
- `toDateOnly(date)`
- `toTimeOnly(date)`
- `toTimeOnly12Hour(date)`
- `toEmailTemplate(date, timezone)`
- `toScheduleWindow(start, end, timezone)`
- `formatDuration(milliseconds)` - Takes milliseconds instead of Duration
- `toTimezone(date, timezone)`

## Validator Module

### Go API

#### Constructor
```go
func NewValidator(config *DateTimeConfig) *Validator
```

#### Methods

##### ValidateDateTime
```go
func (v *Validator) ValidateDateTime(t time.Time) error
```
Validates that the time is a valid date/time value within acceptable bounds.

**Validation Rules:**
- Not more than 50 years in the past
- Not more than 10 years in the future
- Must be a valid time value

##### ValidateDateRange
```go
func (v *Validator) ValidateDateRange(start, end time.Time) error
```
Validates that start time is before end time and range is reasonable.

**Validation Rules:**
- Start must be before end
- Range cannot exceed 1 year
- Both times must pass individual validation

##### ValidateTimezone
```go
func (v *Validator) ValidateTimezone(tz string) error
```
Validates that the timezone string is a valid IANA timezone identifier.

##### ValidateMeetingTime
```go
func (v *Validator) ValidateMeetingTime(t time.Time) error
```
Validates that the time is appropriate for scheduling meetings.

**Validation Rules:**
- Must be in the future (respects FutureTolerance config)
- Cannot be more than 2 years in the future
- Respects AllowPastDates configuration

##### ValidateBusinessHours
```go
func (v *Validator) ValidateBusinessHours(t time.Time, timezone string) error
```
Validates that the time falls within business hours.

**Validation Rules:**
- Must be Monday-Friday
- Must be between 8 AM and 6 PM in specified timezone

##### ValidateScheduleWindow
```go
func (v *Validator) ValidateScheduleWindow(start, end time.Time) error
```
Validates a schedule window for meetings.

**Validation Rules:**
- Combines ValidateDateRange and ValidateMeetingTime rules
- Window must be at least 15 minutes
- Window cannot exceed 24 hours

##### ValidateMeetingDuration
```go
func (v *Validator) ValidateMeetingDuration(d time.Duration) error
```
Validates meeting duration is within acceptable bounds.

**Validation Rules:**
- Minimum: 15 minutes
- Maximum: 8 hours

### Node.js API

#### Constructor
```javascript
class Validator {
    constructor(config = null)
}
```

#### Methods

All methods have the same signatures and behavior as the Go implementation:

- `validateDateTime(date)`
- `validateDateRange(start, end)`
- `validateTimezone(tz)`
- `validateMeetingTime(date)`
- `validateBusinessHours(date, timezone)`
- `validateScheduleWindow(start, end)`
- `validateMeetingDuration(milliseconds)` - Takes milliseconds instead of Duration

## Error Types

### DateTimeError

Structured error type used by all datetime utilities.

#### Go Implementation
```go
type DateTimeError struct {
    Type    string // Error type constant
    Message string // Human-readable error message
    Input   string // Original input that caused the error
    Cause   error  // Underlying error (if any)
}

func (e *DateTimeError) Error() string {
    return e.Message
}
```

#### Node.js Implementation
```javascript
class DateTimeError extends Error {
    constructor(type, message, input, cause) {
        super(message);
        this.name = 'DateTimeError';
        this.type = type;
        this.input = input;
        this.cause = cause;
    }
}
```

### Error Type Constants

#### Go Constants
```go
const (
    ErrInvalidFormat   = "INVALID_FORMAT"
    ErrInvalidTimezone = "INVALID_TIMEZONE"
    ErrInvalidRange    = "INVALID_RANGE"
    ErrPastDate        = "PAST_DATE"
    ErrFutureDate      = "FUTURE_DATE"
)
```

#### Node.js Constants
```javascript
const ERROR_TYPES = {
    INVALID_FORMAT: 'INVALID_FORMAT',
    INVALID_TIMEZONE: 'INVALID_TIMEZONE',
    INVALID_RANGE: 'INVALID_RANGE',
    PAST_DATE: 'PAST_DATE',
    FUTURE_DATE: 'FUTURE_DATE'
};
```

## Format Constants

### Go Constants
```go
const (
    RFC3339Format = "2006-01-02T15:04:05Z07:00"
    GraphFormat   = "2006-01-02T15:04:05.0000000"
    ICSFormat     = "20060102T150405Z"
    LogFormat     = "2006-01-02T15:04:05.000Z"
    DateOnlyFormat = "2006-01-02"
    TimeOnlyFormat = "15:04:05"
    Time12HourFormat = "3:04:05 PM"
)
```

### Node.js Constants
```javascript
const FORMATS = {
    RFC3339: 'YYYY-MM-DDTHH:mm:ssZ',
    GRAPH: 'YYYY-MM-DDTHH:mm:ss.0000000',
    ICS: 'YYYYMMDDTHHmmssZ',
    LOG: 'YYYY-MM-DDTHH:mm:ss.SSSZ',
    DATE_ONLY: 'YYYY-MM-DD',
    TIME_ONLY: 'HH:mm:ss',
    TIME_12_HOUR: 'h:mm:ss A'
};
```

## Usage Examples

### Basic Usage

#### Go Example
```go
package main

import (
    "fmt"
    "your-project/internal/datetime"
)

func main() {
    // Create utilities with default config
    parser := datetime.NewParser(nil)
    formatter := datetime.NewFormatter(nil)
    validator := datetime.NewValidator(nil)
    
    // Parse various formats
    parsed, err := parser.ParseDateTime("01/15/2025 10:00 AM")
    if err != nil {
        panic(err)
    }
    
    // Validate
    if err := validator.ValidateMeetingTime(parsed); err != nil {
        panic(err)
    }
    
    // Format for different contexts
    fmt.Println("RFC3339:", formatter.ToRFC3339(parsed))
    fmt.Println("Human:", formatter.ToHumanReadable(parsed, "America/New_York"))
    fmt.Println("Graph API:", formatter.ToMicrosoftGraph(parsed))
}
```

#### Node.js Example
```javascript
import { Parser, Formatter, Validator } from './datetime/index.js';

// Create utilities with default config
const parser = new Parser();
const formatter = new Formatter();
const validator = new Validator();

try {
    // Parse various formats
    const parsed = parser.parseDateTime('01/15/2025 10:00 AM');
    
    // Validate
    validator.validateMeetingTime(parsed);
    
    // Format for different contexts
    console.log('RFC3339:', formatter.toRFC3339(parsed));
    console.log('Human:', formatter.toHumanReadable(parsed, 'America/New_York'));
    console.log('Graph API:', formatter.toMicrosoftGraph(parsed));
} catch (err) {
    if (err instanceof DateTimeError) {
        console.error(`DateTime error: ${err.message}`);
    } else {
        throw err;
    }
}
```

### Custom Configuration

#### Go Example
```go
// Custom configuration
config := &datetime.DateTimeConfig{
    DefaultTimezone: "America/Los_Angeles",
    AllowPastDates:  true,
    FutureTolerance: 10 * time.Minute,
}

parser := datetime.NewParser(config)
validator := datetime.NewValidator(config)

// Now allows past dates and has 10-minute tolerance
pastDate, _ := parser.ParseDateTime("2020-01-01T10:00:00Z")
err := validator.ValidateMeetingTime(pastDate) // Won't error due to AllowPastDates
```

#### Node.js Example
```javascript
import { Parser, Validator, DateTimeConfig } from './datetime/index.js';

// Custom configuration
const config = new DateTimeConfig({
    defaultTimezone: 'America/Los_Angeles',
    allowPastDates: true,
    futureTolerance: 600000 // 10 minutes in milliseconds
});

const parser = new Parser(config);
const validator = new Validator(config);

// Now allows past dates and has 10-minute tolerance
const pastDate = parser.parseDateTime('2020-01-01T10:00:00Z');
validator.validateMeetingTime(pastDate); // Won't throw due to allowPastDates
```

### Error Handling

#### Go Example
```go
parsed, err := parser.ParseDateTime(userInput)
if err != nil {
    if dtErr, ok := err.(*datetime.DateTimeError); ok {
        switch dtErr.Type {
        case datetime.ErrInvalidFormat:
            return fmt.Errorf("invalid date format: expected YYYY-MM-DD or MM/DD/YYYY, got '%s'", userInput)
        case datetime.ErrInvalidTimezone:
            return fmt.Errorf("invalid timezone: %s", dtErr.Message)
        case datetime.ErrPastDate:
            return fmt.Errorf("meeting time cannot be in the past")
        default:
            return fmt.Errorf("date parsing error: %s", dtErr.Message)
        }
    }
    return err
}
```

#### Node.js Example
```javascript
try {
    const parsed = parser.parseDateTime(userInput);
} catch (err) {
    if (err instanceof DateTimeError) {
        switch (err.type) {
            case ERROR_TYPES.INVALID_FORMAT:
                throw new Error(`Invalid date format: expected YYYY-MM-DD or MM/DD/YYYY, got '${userInput}'`);
            case ERROR_TYPES.INVALID_TIMEZONE:
                throw new Error(`Invalid timezone: ${err.message}`);
            case ERROR_TYPES.PAST_DATE:
                throw new Error('Meeting time cannot be in the past');
            default:
                throw new Error(`Date parsing error: ${err.message}`);
        }
    }
    throw err;
}
```

## Integration Patterns

### Lambda Handler Pattern

#### Go Lambda
```go
func handleScheduleRequest(ctx context.Context, event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    parser := datetime.NewParser(nil)
    validator := datetime.NewValidator(nil)
    formatter := datetime.NewFormatter(nil)
    
    var req ScheduleRequest
    if err := json.Unmarshal([]byte(event.Body), &req); err != nil {
        return errorResponse(400, "Invalid request body")
    }
    
    // Parse meeting time
    meetingTime, err := parser.ParseLegacyDateTimeFields(req.BeginDate, req.BeginTime, req.Timezone)
    if err != nil {
        return errorResponse(400, fmt.Sprintf("Invalid date/time: %s", err.Error()))
    }
    
    // Validate
    if err := validator.ValidateMeetingTime(meetingTime); err != nil {
        return errorResponse(400, fmt.Sprintf("Invalid meeting time: %s", err.Error()))
    }
    
    // Format for Graph API
    graphTime := formatter.ToMicrosoftGraph(meetingTime)
    
    // Process meeting creation...
    
    return successResponse(map[string]interface{}{
        "meetingTime": formatter.ToRFC3339(meetingTime),
        "graphTime":   graphTime,
    })
}
```

#### Node.js Lambda
```javascript
import { Parser, Validator, Formatter, DateTimeError } from './datetime/index.js';

export const handler = async (event) => {
    const parser = new Parser();
    const validator = new Validator();
    const formatter = new Formatter();
    
    try {
        const req = JSON.parse(event.body);
        
        // Parse meeting time
        const meetingTime = parser.parseLegacyDateTimeFields(req.beginDate, req.beginTime, req.timezone);
        
        // Validate
        validator.validateMeetingTime(meetingTime);
        
        // Format for Graph API
        const graphTime = formatter.toMicrosoftGraph(meetingTime);
        
        // Process meeting creation...
        
        return {
            statusCode: 200,
            body: JSON.stringify({
                meetingTime: formatter.toRFC3339(meetingTime),
                graphTime: graphTime
            })
        };
    } catch (err) {
        if (err instanceof DateTimeError) {
            return {
                statusCode: 400,
                body: JSON.stringify({ error: `Invalid date/time: ${err.message}` })
            };
        }
        throw err;
    }
};
```

### Data Migration Pattern

#### Go Migration
```go
func migrateLegacySchedules() error {
    parser := datetime.NewParser(&datetime.DateTimeConfig{
        AllowPastDates: true, // For historical data
    })
    formatter := datetime.NewFormatter(nil)
    
    schedules, err := loadLegacySchedules()
    if err != nil {
        return err
    }
    
    for _, schedule := range schedules {
        // Parse legacy fields
        startTime, err := parser.ParseLegacyDateTimeFields(
            schedule.BeginDate, 
            schedule.BeginTime, 
            schedule.Timezone,
        )
        if err != nil {
            log.Printf("Failed to parse schedule %s: %v", schedule.ID, err)
            continue
        }
        
        // Update with new time.Time field
        schedule.ImplementationStart = startTime
        
        // Keep legacy fields for backward compatibility
        schedule.BeginDate = formatter.ToDateOnly(startTime)
        schedule.BeginTime = formatter.ToTimeOnly(startTime)
        
        if err := saveSchedule(schedule); err != nil {
            log.Printf("Failed to save schedule %s: %v", schedule.ID, err)
        }
    }
    
    return nil
}
```

#### Node.js Migration
```javascript
import { Parser, Formatter, DateTimeConfig } from './datetime/index.js';

async function migrateLegacySchedules() {
    const parser = new Parser(new DateTimeConfig({
        allowPastDates: true // For historical data
    }));
    const formatter = new Formatter();
    
    const schedules = await loadLegacySchedules();
    
    for (const schedule of schedules) {
        try {
            // Parse legacy fields
            const startTime = parser.parseLegacyDateTimeFields(
                schedule.beginDate,
                schedule.beginTime,
                schedule.timezone
            );
            
            // Update with new Date field
            schedule.implementationStart = startTime;
            
            // Keep legacy fields for backward compatibility
            schedule.beginDate = formatter.toDateOnly(startTime);
            schedule.beginTime = formatter.toTimeOnly(startTime);
            
            await saveSchedule(schedule);
        } catch (err) {
            console.error(`Failed to migrate schedule ${schedule.id}:`, err);
        }
    }
}
```

## Performance Considerations

### Instance Reuse
Create parser, formatter, and validator instances once and reuse them:

```go
// Global instances (Go)
var (
    globalParser    = datetime.NewParser(nil)
    globalFormatter = datetime.NewFormatter(nil)
    globalValidator = datetime.NewValidator(nil)
)
```

```javascript
// Module-level instances (Node.js)
const parser = new Parser();
const formatter = new Formatter();
const validator = new Validator();

export { parser, formatter, validator };
```

### Timezone Caching
Both implementations cache timezone data automatically, but you can optimize by:

1. Using consistent timezone strings
2. Avoiding repeated timezone lookups
3. Reusing formatter instances with the same default timezone

### Batch Operations
For processing multiple dates, reuse instances and handle errors gracefully:

```go
func processDates(inputs []string) []time.Time {
    parser := datetime.NewParser(nil)
    results := make([]time.Time, 0, len(inputs))
    
    for _, input := range inputs {
        if parsed, err := parser.ParseDateTime(input); err == nil {
            results = append(results, parsed)
        }
    }
    
    return results
}
```

## Cross-Language Compatibility

Both implementations are designed to produce identical results. To ensure compatibility:

1. **Use Same Configuration**: Ensure both Go and Node.js use identical DateTimeConfig values
2. **Test with Shared Data**: Use the same test cases for both implementations
3. **Validate Outputs**: Compare outputs from both implementations for the same inputs
4. **Handle Timezones Consistently**: Use IANA timezone identifiers in both

### Compatibility Testing
```go
// Go test
func TestCrossLanguageCompatibility(t *testing.T) {
    parser := datetime.NewParser(nil)
    formatter := datetime.NewFormatter(nil)
    
    input := "2025-01-15T10:00:00-05:00"
    parsed, err := parser.ParseDateTime(input)
    require.NoError(t, err)
    
    rfc3339 := formatter.ToRFC3339(parsed)
    graph := formatter.ToMicrosoftGraph(parsed)
    
    // These should match Node.js outputs exactly
    assert.Equal(t, "2025-01-15T10:00:00-05:00", rfc3339)
    assert.Equal(t, "2025-01-15T15:00:00.0000000", graph)
}
```

```javascript
// Node.js test
import { Parser, Formatter } from './datetime/index.js';

test('cross-language compatibility', () => {
    const parser = new Parser();
    const formatter = new Formatter();
    
    const input = '2025-01-15T10:00:00-05:00';
    const parsed = parser.parseDateTime(input);
    
    const rfc3339 = formatter.toRFC3339(parsed);
    const graph = formatter.toMicrosoftGraph(parsed);
    
    // These should match Go outputs exactly
    expect(rfc3339).toBe('2025-01-15T10:00:00-05:00');
    expect(graph).toBe('2025-01-15T15:00:00.0000000');
});
```

This API reference provides complete documentation for all datetime utility functions across both Go and Node.js implementations, ensuring consistent usage throughout the system.