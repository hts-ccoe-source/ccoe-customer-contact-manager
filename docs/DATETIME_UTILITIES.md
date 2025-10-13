# DateTime Utilities Documentation

## Overview

The DateTime utilities provide standardized date/time handling across all system components (Go Lambda backend, Node.js frontend API, and Node.js authorizer Lambda). This ensures consistent parsing, storage, validation, and formatting of temporal data throughout the entire application stack.

## Core Principles

1. **Single Source of Truth**: RFC3339/ISO8601 with timezone as the canonical internal format
2. **Language-Agnostic Standards**: Identical behavior across Go and Node.js implementations
3. **Centralized Utilities**: Dedicated date/time modules in each language with equivalent functionality
4. **Graceful Input Handling**: Accept multiple input formats, normalize to standard format
5. **Context-Aware Output**: Format appropriately for different consumers (APIs, humans, logs)

## Architecture

### Go Implementation
- **Location**: `internal/datetime/`
- **Package**: `datetime`
- **Dependencies**: Standard Go `time` package

### Node.js Implementation
- **Location**: `datetime/`
- **Package**: `datetime`
- **Dependencies**: `dayjs` (with `utc` and `timezone` plugins)

## Standard Formats

### Internal Format (RFC3339)
```
2025-01-15T10:00:00-05:00  (with timezone offset)
2025-01-15T15:00:00Z       (UTC)
```

### Output Formats
- **Microsoft Graph**: `2025-01-15T15:00:00.0000000` (UTC, specific precision)
- **Human Readable**: `January 15, 2025 at 10:00 AM EST`
- **ICS Calendar**: `20250115T150000Z` (UTC for calendar files)
- **Log Format**: `2025-01-15T15:00:00.000Z` (UTC with milliseconds)

## Configuration

### DateTimeConfig

Both Go and Node.js implementations use a configuration object:

**Go:**
```go
type DateTimeConfig struct {
    DefaultTimezone string        // "America/New_York"
    AllowPastDates  bool         // false
    FutureTolerance time.Duration // 5 * time.Minute
}
```

**Node.js:**
```javascript
class DateTimeConfig {
    constructor(options = {}) {
        this.defaultTimezone = options.defaultTimezone || 'America/New_York';
        this.allowPastDates = options.allowPastDates || false;
        this.futureTolerance = options.futureTolerance || 300000; // 5 minutes
    }
}
```

## Parser Module

### Go API

```go
// Create parser with default config
parser := datetime.NewParser(nil)

// Create parser with custom config
config := &datetime.DateTimeConfig{
    DefaultTimezone: "America/Los_Angeles",
    AllowPastDates:  true,
    FutureTolerance: 10 * time.Minute,
}
parser := datetime.NewParser(config)

// Parse various formats
parsed, err := parser.ParseDateTime("2025-01-15T10:00:00-05:00")
parsed, err := parser.ParseDateTime("01/15/2025 10:00 AM")
parsed, err := parser.ParseDate("2025-01-15")
parsed, err := parser.ParseTime("10:00 AM", baseDate)
```

### Node.js API

```javascript
import { Parser, DateTimeConfig } from './datetime/index.js';

// Create parser with default config
const parser = new Parser();

// Create parser with custom config
const config = new DateTimeConfig({
    defaultTimezone: 'America/Los_Angeles',
    allowPastDates: true,
    futureTolerance: 600000 // 10 minutes
});
const parser = new Parser(config);

// Parse various formats
const parsed = parser.parseDateTime('2025-01-15T10:00:00-05:00');
const parsed = parser.parseDateTime('01/15/2025 10:00 AM');
const parsed = parser.parseDate('2025-01-15');
const parsed = parser.parseTime('10:00 AM', baseDate);
```

### Supported Input Formats

#### Date Formats
- `2025-01-15` (ISO date)
- `01/15/2025` (US format)
- `1/15/2025` (US format, no leading zeros)
- `January 15, 2025` (full month name)
- `Jan 15, 2025` (abbreviated month)
- `15 January 2025` (European style)
- `15 Jan 2025` (European abbreviated)

#### Time Formats
- `15:04:05` (24-hour with seconds)
- `15:04` (24-hour)
- `3:04:05 PM` (12-hour with seconds)
- `3:04 PM` (12-hour)
- `3:04:05PM` (12-hour no space)
- `3:04PM` (12-hour no space)

#### Combined DateTime Formats
- `2025-01-15T10:00:00-05:00` (RFC3339)
- `2025-01-15T10:00:00Z` (UTC)
- `2025-01-15 10:00:00` (space separator)
- `01/15/2025 3:04 PM` (US format with time)
- `January 15, 2025 at 3:04 PM` (natural language)

## Formatter Module

### Go API

```go
formatter := datetime.NewFormatter(nil)

// Standard formats
rfc3339 := formatter.ToRFC3339(time.Now())
graph := formatter.ToMicrosoftGraph(time.Now())
human := formatter.ToHumanReadable(time.Now(), "America/New_York")
ics := formatter.ToICS(time.Now())
log := formatter.ToLogFormat(time.Now())

// Utility formats
dateOnly := formatter.ToDateOnly(time.Now())
timeOnly := formatter.ToTimeOnly(time.Now())
time12Hour := formatter.ToTimeOnly12Hour(time.Now())
email := formatter.ToEmailTemplate(time.Now(), "America/New_York")

// Schedule formatting
window := formatter.ToScheduleWindow(start, end, "America/New_York")
duration := formatter.FormatDuration(2 * time.Hour)
```

### Node.js API

```javascript
import { Formatter } from './datetime/index.js';

const formatter = new Formatter();

// Standard formats
const rfc3339 = formatter.toRFC3339(new Date());
const graph = formatter.toMicrosoftGraph(new Date());
const human = formatter.toHumanReadable(new Date(), 'America/New_York');
const ics = formatter.toICS(new Date());
const log = formatter.toLogFormat(new Date());

// Utility formats
const dateOnly = formatter.toDateOnly(new Date());
const timeOnly = formatter.toTimeOnly(new Date());
const time12Hour = formatter.toTimeOnly12Hour(new Date());
const email = formatter.toEmailTemplate(new Date(), 'America/New_York');

// Schedule formatting
const window = formatter.toScheduleWindow(start, end, 'America/New_York');
const duration = formatter.formatDuration(7200000); // 2 hours in ms
```

### Format Examples

| Method | Output Example |
|--------|----------------|
| `ToRFC3339()` | `2025-01-15T10:00:00-05:00` |
| `ToMicrosoftGraph()` | `2025-01-15T15:00:00.0000000` |
| `ToHumanReadable()` | `January 15, 2025 at 10:00 AM EST` |
| `ToICS()` | `20250115T150000Z` |
| `ToLogFormat()` | `2025-01-15T15:00:00.000Z` |
| `ToEmailTemplate()` | `Monday, January 15, 2025 at 10:00 AM EST` |
| `ToScheduleWindow()` | `January 15, 2025 from 10:00 AM EST to 2:00 PM EST` |

## Validator Module

### Go API

```go
validator := datetime.NewValidator(nil)

// Basic validation
err := validator.ValidateDateTime(time.Now())
err := validator.ValidateDateRange(start, end)
err := validator.ValidateTimezone("America/New_York")
err := validator.ValidateMeetingTime(meetingTime)

// Business rules validation
err := validator.ValidateBusinessHours(meetingTime, "America/New_York")
err := validator.ValidateScheduleWindow(start, end)
err := validator.ValidateMeetingDuration(2 * time.Hour)
```

### Node.js API

```javascript
import { Validator } from './datetime/index.js';

const validator = new Validator();

// Basic validation
validator.validateDateTime(new Date());
validator.validateDateRange(start, end);
validator.validateTimezone('America/New_York');
validator.validateMeetingTime(meetingTime);

// Business rules validation
validator.validateBusinessHours(meetingTime, 'America/New_York');
validator.validateScheduleWindow(start, end);
validator.validateMeetingDuration(7200000); // 2 hours in ms
```

### Validation Rules

#### DateTime Validation
- Must be a valid date/time value
- Cannot be more than 50 years in the past
- Cannot be more than 10 years in the future

#### Meeting Time Validation
- Must be in the future (with configurable tolerance, default 5 minutes)
- Cannot be more than 2 years in the future
- Respects `AllowPastDates` configuration

#### Date Range Validation
- Start time must be before end time
- Range cannot exceed 1 year
- Both dates must pass individual validation

#### Business Hours Validation
- Must be Monday-Friday
- Must be between 8 AM and 6 PM in specified timezone

#### Duration Validation
- Meeting duration: 15 minutes to 8 hours
- Schedule window: 15 minutes to 24 hours

## Error Handling

### Error Types

Both implementations use standardized error types:

- `INVALID_FORMAT`: Parsing or format errors
- `INVALID_TIMEZONE`: Invalid timezone identifiers
- `INVALID_RANGE`: Date range or duration errors
- `PAST_DATE`: Date is in the past when future required
- `FUTURE_DATE`: Date is too far in the future

### Go Error Handling

```go
parsed, err := parser.ParseDateTime(input)
if err != nil {
    if dtErr, ok := err.(*datetime.DateTimeError); ok {
        switch dtErr.Type {
        case datetime.ErrInvalidFormat:
            // Handle format error
        case datetime.ErrInvalidTimezone:
            // Handle timezone error
        }
    }
}
```

### Node.js Error Handling

```javascript
try {
    const parsed = parser.parseDateTime(input);
} catch (err) {
    if (err instanceof DateTimeError) {
        switch (err.type) {
            case ERROR_TYPES.INVALID_FORMAT:
                // Handle format error
                break;
            case ERROR_TYPES.INVALID_TIMEZONE:
                // Handle timezone error
                break;
        }
    }
}
```

## Usage Examples

### Basic Parsing and Formatting

**Go:**
```go
package main

import (
    "fmt"
    "your-project/internal/datetime"
)

func main() {
    parser := datetime.NewParser(nil)
    formatter := datetime.NewFormatter(nil)
    
    // Parse user input
    parsed, err := parser.ParseDateTime("01/15/2025 10:00 AM")
    if err != nil {
        panic(err)
    }
    
    // Format for different contexts
    fmt.Println("RFC3339:", formatter.ToRFC3339(parsed))
    fmt.Println("Human:", formatter.ToHumanReadable(parsed, "America/New_York"))
    fmt.Println("Graph API:", formatter.ToMicrosoftGraph(parsed))
}
```

**Node.js:**
```javascript
import { Parser, Formatter } from './datetime/index.js';

const parser = new Parser();
const formatter = new Formatter();

// Parse user input
const parsed = parser.parseDateTime('01/15/2025 10:00 AM');

// Format for different contexts
console.log('RFC3339:', formatter.toRFC3339(parsed));
console.log('Human:', formatter.toHumanReadable(parsed, 'America/New_York'));
console.log('Graph API:', formatter.toMicrosoftGraph(parsed));
```

### Meeting Scheduling

**Go:**
```go
func scheduleMeeting(startInput, timezone string) error {
    parser := datetime.NewParser(nil)
    validator := datetime.NewValidator(nil)
    formatter := datetime.NewFormatter(nil)
    
    // Parse meeting time with timezone
    meetingTime, err := parser.ParseDateTimeWithTimezone(startInput, timezone)
    if err != nil {
        return err
    }
    
    // Validate meeting time
    if err := validator.ValidateMeetingTime(meetingTime); err != nil {
        return err
    }
    
    // Format for Microsoft Graph API
    graphTime := formatter.ToMicrosoftGraph(meetingTime)
    
    // Create meeting via Graph API
    return createGraphMeeting(graphTime)
}
```

**Node.js:**
```javascript
import { Parser, Validator, Formatter } from './datetime/index.js';

async function scheduleMeeting(startInput, timezone) {
    const parser = new Parser();
    const validator = new Validator();
    const formatter = new Formatter();
    
    // Parse meeting time with timezone
    const meetingTime = parser.parseDateTimeWithTimezone(startInput, timezone);
    
    // Validate meeting time
    validator.validateMeetingTime(meetingTime);
    
    // Format for Microsoft Graph API
    const graphTime = formatter.toMicrosoftGraph(meetingTime);
    
    // Create meeting via Graph API
    return await createGraphMeeting(graphTime);
}
```

### Legacy Data Migration

**Go:**
```go
func migrateLegacySchedule(beginDate, beginTime, timezone string) (time.Time, error) {
    parser := datetime.NewParser(nil)
    
    // Parse legacy separate fields
    date, err := parser.ParseDate(beginDate)
    if err != nil {
        return time.Time{}, err
    }
    
    combined, err := parser.ParseTime(beginTime, date)
    if err != nil {
        return time.Time{}, err
    }
    
    // Apply timezone if specified
    if timezone != "" {
        return parser.ParseDateTimeWithTimezone(
            combined.Format("2006-01-02 15:04:05"), 
            timezone,
        )
    }
    
    return combined, nil
}
```

**Node.js:**
```javascript
import { Parser } from './datetime/index.js';

function migrateLegacySchedule(beginDate, beginTime, timezone) {
    const parser = new Parser();
    
    // Parse legacy separate fields
    const date = parser.parseDate(beginDate);
    const combined = parser.parseTime(beginTime, date);
    
    // Apply timezone if specified
    if (timezone) {
        const formatter = new Formatter();
        const dateTimeStr = formatter.toDateOnly(combined) + ' ' + formatter.toTimeOnly(combined);
        return parser.parseDateTimeWithTimezone(dateTimeStr, timezone);
    }
    
    return combined;
}
```

## Cross-Language Compatibility

### Ensuring Consistency

Both implementations are designed to produce identical results:

1. **Same Input Formats**: Both parsers accept the same input format strings
2. **Same Output Formats**: Both formatters produce identical output strings
3. **Same Validation Rules**: Both validators apply identical business rules
4. **Same Error Messages**: Both implementations use the same error messages

### Testing Compatibility

Use shared test data to verify consistency:

```json
{
  "testCases": [
    {
      "input": "2025-01-15T10:00:00-05:00",
      "expectedRFC3339": "2025-01-15T10:00:00-05:00",
      "expectedGraph": "2025-01-15T15:00:00.0000000",
      "expectedHuman": "January 15, 2025 at 10:00 AM EST"
    }
  ]
}
```

## Best Practices

### 1. Always Use Centralized Utilities
```go
// ❌ Don't do manual parsing
parsed, _ := time.Parse("2006-01-02", input)

// ✅ Use centralized parser
parser := datetime.NewParser(nil)
parsed, err := parser.ParseDate(input)
```

### 2. Handle Timezones Explicitly
```go
// ❌ Don't assume timezone
meetingTime := time.Now()

// ✅ Parse with explicit timezone
parser := datetime.NewParser(nil)
meetingTime, err := parser.ParseDateTimeWithTimezone(input, userTimezone)
```

### 3. Validate Before Processing
```go
// ✅ Always validate meeting times
validator := datetime.NewValidator(nil)
if err := validator.ValidateMeetingTime(meetingTime); err != nil {
    return err
}
```

### 4. Use Appropriate Output Formats
```go
formatter := datetime.NewFormatter(nil)

// For APIs
graphTime := formatter.ToMicrosoftGraph(meetingTime)

// For users
humanTime := formatter.ToHumanReadable(meetingTime, userTimezone)

// For logs
logTime := formatter.ToLogFormat(meetingTime)
```

### 5. Handle Errors Gracefully
```javascript
try {
    const parsed = parser.parseDateTime(userInput);
    return formatter.toHumanReadable(parsed, userTimezone);
} catch (err) {
    if (err instanceof DateTimeError && err.type === ERROR_TYPES.INVALID_FORMAT) {
        return 'Please enter a valid date format like "01/15/2025" or "January 15, 2025"';
    }
    throw err;
}
```

## Migration Guide

### From Manual Parsing

**Before:**
```go
// Manual parsing with potential errors
parsed, err := time.Parse("2006-01-02 15:04:05", input)
```

**After:**
```go
// Centralized parsing with multiple format support
parser := datetime.NewParser(nil)
parsed, err := parser.ParseDateTime(input)
```

### From String Concatenation

**Before:**
```go
// Manual string building
timestamp := beginDate + "T" + beginTime + ":00-05:00"
```

**After:**
```go
// Proper parsing and formatting
parser := datetime.NewParser(nil)
date, _ := parser.ParseDate(beginDate)
combined, _ := parser.ParseTime(beginTime, date)
formatter := datetime.NewFormatter(nil)
timestamp := formatter.ToRFC3339(combined)
```

### From Hardcoded Formats

**Before:**
```go
// Hardcoded Graph API format
graphTime := meetingTime.UTC().Format("2006-01-02T15:04:05.0000000")
```

**After:**
```go
// Centralized formatting
formatter := datetime.NewFormatter(nil)
graphTime := formatter.ToMicrosoftGraph(meetingTime)
```

## Troubleshooting

### Common Issues

#### 1. Timezone Parsing Errors
**Problem**: `invalid timezone: America/New_York`
**Solution**: Ensure timezone names are valid IANA identifiers. Use `America/New_York`, not `EST`.

#### 2. Format Not Recognized
**Problem**: `unable to parse date/time: got '15/01/2025'`
**Solution**: The parser expects US format (`01/15/2025`) or ISO format (`2025-01-15`). European format (`15/01/2025`) is not supported.

#### 3. Past Date Validation
**Problem**: `meeting time cannot be in the past`
**Solution**: Check the `AllowPastDates` configuration or adjust the `FutureTolerance` setting.

#### 4. Cross-Language Inconsistency
**Problem**: Go and Node.js produce different outputs
**Solution**: Ensure both implementations use the same configuration and input formats. Check timezone handling.

### Debugging Tips

1. **Enable Verbose Logging**: Use `ToLogFormat()` to see exact timestamps
2. **Check Timezone Configuration**: Verify default timezone settings
3. **Test with Known Values**: Use RFC3339 format for testing
4. **Compare Implementations**: Run the same input through both Go and Node.js

## Performance Considerations

### Go Implementation
- Uses standard library `time` package (highly optimized)
- Minimal memory allocation for parsing
- Efficient timezone handling with location caching

### Node.js Implementation
- Uses Day.js exclusively for all date/time operations (lightweight and performant)
- Leverages Day.js timezone plugin for robust timezone handling
- Caches timezone data where possible
- Optimized for common use cases

### Optimization Tips
1. **Reuse Parser/Formatter Instances**: Create once, use multiple times
2. **Cache Timezone Locations**: Avoid repeated timezone lookups
3. **Use Appropriate Precision**: Don't use millisecond precision if not needed
4. **Batch Operations**: Process multiple dates in a single call when possible

## API Reference Summary

### Go Package: `internal/datetime`

#### Types
- `Parser` - Handles parsing various input formats
- `Formatter` - Handles formatting for different outputs
- `Validator` - Handles validation according to business rules
- `DateTimeConfig` - Configuration for all operations
- `DateTimeError` - Standardized error type

#### Constants
- `RFC3339Format`, `GraphFormat`, `ICSFormat`, `LogFormat`
- `ErrInvalidFormat`, `ErrInvalidTimezone`, `ErrInvalidRange`, `ErrPastDate`, `ErrFutureDate`

### Node.js Package: `datetime`

#### Classes
- `Parser` - Handles parsing various input formats
- `Formatter` - Handles formatting for different outputs  
- `Validator` - Handles validation according to business rules
- `DateTimeConfig` - Configuration for all operations
- `DateTimeError` - Standardized error type

#### Constants
- `FORMATS` - Object with format constants
- `ERROR_TYPES` - Object with error type constants
- `COMMON_INPUT_FORMATS` - Array of supported input formats

This documentation provides a comprehensive guide to using the datetime utilities consistently across all system components. For specific implementation details, refer to the source code and inline documentation in each module.