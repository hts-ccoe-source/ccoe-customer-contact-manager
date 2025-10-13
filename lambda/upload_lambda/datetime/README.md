# Node.js DateTime Utilities

This package provides standardized date/time handling utilities for Node.js components, designed to be equivalent to the Go `internal/datetime` package. It ensures consistent parsing, formatting, and validation of temporal data across all Node.js components in the system.

## Features

- **Consistent Parsing**: Handle multiple input formats gracefully
- **Standardized Formatting**: Output in various formats (RFC3339, Microsoft Graph, human-readable, etc.)
- **Robust Validation**: Business rule validation for meetings, schedules, and date ranges
- **Timezone Support**: Proper timezone handling using IANA timezone identifiers
- **Cross-Language Compatibility**: Produces identical results to the Go implementation

## Installation

```bash
npm install
```

## Dependencies

- `date-fns`: Core date manipulation and formatting
- `date-fns-tz`: Timezone support

## Quick Start

```javascript
import { DateTime, parseDateTime, toRFC3339, validateMeetingTime } from './index.js';

// Using convenience functions
const date = parseDateTime('2025-01-15 10:00 AM');
const formatted = toRFC3339(date);
validateMeetingTime(date);

// Using the main DateTime class
const dt = new DateTime();
const parsed = dt.parseDateTime('January 15, 2025 at 10:00 AM');
const humanReadable = dt.toHumanReadable(parsed, 'America/New_York');
```

## API Reference

### DateTime Class

The main class that provides all date/time operations:

```javascript
import { DateTime, DateTimeConfig } from './index.js';

const config = new DateTimeConfig({
    defaultTimezone: 'America/New_York',
    allowPastDates: false,
    futureTolerance: 300000 // 5 minutes in milliseconds
});

const dt = new DateTime(config);
```

### Parser Methods

#### `parseDateTime(input)`
Parses various date/time formats into a Date object.

```javascript
const date1 = dt.parseDateTime('2025-01-15T10:00:00-05:00');
const date2 = dt.parseDateTime('01/15/2025 10:00 AM');
const date3 = dt.parseDateTime('January 15, 2025 at 10:00 AM');
```

#### `parseDate(input)`
Parses date-only strings, returning midnight in the default timezone.

```javascript
const date = dt.parseDate('2025-01-15');
const date2 = dt.parseDate('January 15, 2025');
```

#### `parseTime(input, date)`
Parses time-only strings and combines with a given date.

```javascript
const baseDate = new Date('2025-01-15');
const combined = dt.parseTime('10:00 AM', baseDate);
```

#### `parseLegacyDateTimeFields(date, time, timezone)`
Parses separate date/time/timezone fields for backward compatibility.

```javascript
const combined = dt.parseLegacyDateTimeFields(
    '2025-01-15', 
    '10:00:00', 
    'America/New_York'
);
```

### Formatter Methods

#### `toRFC3339(date)`
Formats to canonical RFC3339 format.

```javascript
const formatted = dt.toRFC3339(new Date()); // "2025-01-15T10:00:00-05:00"
```

#### `toMicrosoftGraph(date)`
Formats for Microsoft Graph API compatibility.

```javascript
const graphFormat = dt.toMicrosoftGraph(new Date()); // "2025-01-15T15:00:00.0000000"
```

#### `toHumanReadable(date, timezone)`
Formats for human display.

```javascript
const readable = dt.toHumanReadable(new Date(), 'America/New_York');
// "January 15, 2025 at 10:00 AM EST"
```

#### `toICS(date)`
Formats for iCalendar files.

```javascript
const icsFormat = dt.toICS(new Date()); // "20250115T150000Z"
```

#### `toLogFormat(date)`
Formats for structured logging.

```javascript
const logFormat = dt.toLogFormat(new Date()); // "2025-01-15T15:00:00.000Z"
```

#### `toEmailTemplate(date, timezone)`
Formats for email templates.

```javascript
const emailFormat = dt.toEmailTemplate(new Date(), 'America/New_York');
// "Monday, January 15, 2025 at 10:00 AM EST"
```

#### `toScheduleWindow(start, end, timezone)`
Formats date ranges for schedule display.

```javascript
const window = dt.toScheduleWindow(startDate, endDate, 'America/New_York');
// "January 15, 2025 from 10:00 AM EST to 2:00 PM EST"
```

### Validator Methods

#### `validateDateTime(date)`
Basic date/time validation.

```javascript
dt.validateDateTime(new Date()); // throws DateTimeError if invalid
```

#### `validateMeetingTime(date)`
Validates meeting times (must be in future with tolerance).

```javascript
dt.validateMeetingTime(futureDate); // throws if in past or too far future
```

#### `validateDateRange(start, end)`
Validates that start is before end and range is reasonable.

```javascript
dt.validateDateRange(startDate, endDate);
```

#### `validateTimezone(timezone)`
Validates IANA timezone identifiers.

```javascript
dt.validateTimezone('America/New_York'); // OK
dt.validateTimezone('Invalid/Timezone'); // throws DateTimeError
```

#### `validateBusinessHours(date, timezone)`
Validates that time falls within business hours (8 AM - 6 PM, weekdays).

```javascript
dt.validateBusinessHours(workdayDate, 'America/New_York');
```

## Supported Input Formats

The parser accepts various input formats:

### Date Formats
- ISO 8601: `2025-01-15`
- US Format: `01/15/2025`, `1/15/2025`
- Written: `January 15, 2025`, `Jan 15, 2025`
- European: `15 January 2025`, `15 Jan 2025`

### Time Formats
- 24-hour: `10:00`, `10:00:00`
- 12-hour: `10:00 AM`, `10:00:00 PM`
- Compact: `10:00AM`, `10:00PM`

### Combined Formats
- ISO 8601: `2025-01-15T10:00:00-05:00`
- Space separated: `2025-01-15 10:00:00`
- Natural: `January 15, 2025 at 10:00 AM`

## Error Handling

All methods throw `DateTimeError` objects with standardized error types:

```javascript
import { DateTimeError, ERROR_TYPES } from './index.js';

try {
    dt.parseDateTime('invalid-date');
} catch (error) {
    if (error instanceof DateTimeError) {
        console.log('Error type:', error.type); // ERROR_TYPES.INVALID_FORMAT
        console.log('Message:', error.message);
        console.log('Input:', error.input);
    }
}
```

### Error Types
- `INVALID_FORMAT`: Input format not recognized
- `INVALID_TIMEZONE`: Invalid IANA timezone identifier
- `INVALID_RANGE`: Invalid date range (start after end, too long, etc.)
- `PAST_DATE`: Date is in the past when future required
- `FUTURE_DATE`: Date is too far in the future

## Configuration

```javascript
import { DateTimeConfig } from './index.js';

const config = new DateTimeConfig({
    defaultTimezone: 'America/New_York',  // Default timezone for ambiguous inputs
    allowPastDates: false,                // Whether to allow past dates in validation
    futureTolerance: 300000               // Grace period in milliseconds (5 minutes)
});
```

## Cross-Language Compatibility

This Node.js implementation is designed to produce identical results to the Go `internal/datetime` package:

- Same supported input formats
- Identical output formats
- Equivalent validation rules
- Consistent error messages and types

## Examples

### Meeting Scheduling
```javascript
import { DateTime } from './index.js';

const dt = new DateTime();

// Parse user input
const meetingTime = dt.parseDateTime('January 15, 2025 at 2:00 PM');

// Validate it's appropriate for a meeting
dt.validateMeetingTime(meetingTime);
dt.validateBusinessHours(meetingTime, 'America/New_York');

// Format for different outputs
const graphFormat = dt.toMicrosoftGraph(meetingTime);  // For API calls
const emailFormat = dt.toEmailTemplate(meetingTime, 'America/New_York');  // For invites
const logFormat = dt.toLogFormat(meetingTime);  // For logging
```

### Schedule Window Processing
```javascript
// Parse schedule window
const start = dt.parseLegacyDateTimeFields('2025-01-15', '09:00:00', 'America/New_York');
const end = dt.parseLegacyDateTimeFields('2025-01-15', '17:00:00', 'America/New_York');

// Validate the window
dt.validateScheduleWindow(start, end);

// Format for display
const windowDisplay = dt.toScheduleWindow(start, end, 'America/New_York');
console.log(windowDisplay); // "January 15, 2025 from 9:00 AM EST to 5:00 PM EST"
```

### Timezone Conversion
```javascript
const utcTime = dt.parseDateTime('2025-01-15T15:00:00Z');
const estTime = dt.toTimezone(utcTime, 'America/New_York');
const pstTime = dt.toTimezone(utcTime, 'America/Los_Angeles');
```

## Testing

The package includes comprehensive tests that verify:
- Cross-language compatibility with Go implementation
- All supported input formats
- Timezone handling accuracy
- Error conditions and messages
- Business rule validation

```bash
npm test
```