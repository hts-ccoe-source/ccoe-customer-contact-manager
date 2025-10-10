# DateTime Utilities Migration Guide

## Overview

This guide covers the migration from manual date/time handling to the new centralized datetime utilities. **This is a breaking change** that affects all components using date/time operations.

## Breaking Changes Summary

### 🚨 Critical Breaking Changes

1. **Node.js Dependencies**: Removed `date-fns` and `date-fns-tz`, now uses `dayjs` exclusively
2. **API Changes**: All manual date/time parsing must be replaced with centralized utilities
3. **Data Structures**: New `time.Time` fields added alongside existing string fields
4. **Validation**: Stricter validation rules now enforced
5. **Error Handling**: New structured error types replace generic errors

## Critical Migration Steps

### Step 1: Update Dependencies

#### Node.js Projects
```bash
# Remove old dependencies
npm uninstall date-fns date-fns-tz moment

# Install new dependency
npm install dayjs

# Update package.json
```

**Before:**
```json
{
  "dependencies": {
    "date-fns": "^3.0.0",
    "date-fns-tz": "^2.0.0"
  }
}
```

**After:**
```json
{
  "dependencies": {
    "dayjs": "^1.11.0"
  }
}
```

#### Go Projects
No dependency changes needed - uses standard library `time` package.

### Step 2: Replace Direct Library Usage

#### Node.js - Replace date-fns Imports

**Before:**
```javascript
import { format, parseISO, isValid, addDays } from 'date-fns';
import { zonedTimeToUtc, utcToZonedTime } from 'date-fns-tz';

// Manual formatting
const formatted = format(new Date(), 'yyyy-MM-dd HH:mm:ss');

// Manual parsing
const parsed = parseISO('2025-01-15T10:00:00Z');

// Manual validation
if (!isValid(someDate)) {
    throw new Error('Invalid date');
}

// Manual timezone conversion
const utcDate = zonedTimeToUtc(localDate, 'America/New_York');
```

**After:**
```javascript
import { Parser, Formatter, Validator } from './datetime/index.js';

const parser = new Parser();
const formatter = new Formatter();
const validator = new Validator();

// Centralized formatting
const formatted = formatter.toRFC3339(new Date());

// Centralized parsing
const parsed = parser.parseDateTime('2025-01-15T10:00:00Z');

// Centralized validation
try {
    validator.validateDateTime(someDate);
} catch (err) {
    if (err instanceof DateTimeError) {
        console.log(`Validation failed: ${err.message}`);
    }
}

// Centralized timezone conversion
const converted = formatter.toTimezone(date, 'America/New_York');
```

### Step 3: Update Manual Date/Time Parsing

#### Go - Replace Manual Parsing

**Before:**
```go
// Manual string concatenation
timestamp := beginDate + "T" + beginTime + ":00-05:00"
parsed, err := time.Parse(time.RFC3339, timestamp)

// Manual format attempts
formats := []string{"2006-01-02", "01/02/2006"}
var parsed time.Time
for _, format := range formats {
    if t, err := time.Parse(format, input); err == nil {
        parsed = t
        break
    }
}

// Manual Microsoft Graph formatting
graphTime := meetingTime.UTC().Format("2006-01-02T15:04:05.0000000")
```

**After:**
```go
import "your-project/internal/datetime"

parser := datetime.NewParser(nil)
formatter := datetime.NewFormatter(nil)

// Centralized parsing with multiple format support
parsed, err := parser.ParseDateTime(input)
if err != nil {
    return err
}

// Legacy field parsing
combined, err := parser.ParseLegacyDateTimeFields(beginDate, beginTime, timezone)
if err != nil {
    return err
}

// Centralized Microsoft Graph formatting
graphTime := formatter.ToMicrosoftGraph(meetingTime)
```

#### Node.js - Replace Manual Parsing

**Before:**
```javascript
// Manual string concatenation
const timestamp = `${beginDate}T${beginTime}:00-05:00`;
const parsed = new Date(timestamp);

// Manual format attempts
function parseDate(input) {
    const formats = ['YYYY-MM-DD', 'MM/DD/YYYY'];
    for (const format of formats) {
        const parsed = moment(input, format);
        if (parsed.isValid()) {
            return parsed.toDate();
        }
    }
    throw new Error('Invalid format');
}

// Manual Microsoft Graph formatting
const graphTime = meetingTime.toISOString().replace(/\.\d{3}Z$/, '.0000000');
```

**After:**
```javascript
import { Parser, Formatter } from './datetime/index.js';

const parser = new Parser();
const formatter = new Formatter();

// Centralized parsing with multiple format support
const parsed = parser.parseDateTime(input);

// Legacy field parsing
const combined = parser.parseLegacyDateTimeFields(beginDate, beginTime, timezone);

// Centralized Microsoft Graph formatting
const graphTime = formatter.toMicrosoftGraph(meetingTime);
```

### Step 4: Update Data Structures

#### Go - Add time.Time Fields

**Before:**
```go
type ScheduleInfo struct {
    BeginDate string `json:"beginDate"`
    BeginTime string `json:"beginTime"`
    EndDate   string `json:"endDate"`
    EndTime   string `json:"endTime"`
    Timezone  string `json:"timezone"`
}

type MeetingInvite struct {
    Title           string `json:"title"`
    StartTimeString string `json:"startTime"`
    Duration        int    `json:"duration"`
}
```

**After:**
```go
type ScheduleInfo struct {
    // New time.Time fields (primary)
    ImplementationStart time.Time `json:"implementationStart"`
    ImplementationEnd   time.Time `json:"implementationEnd"`
    
    // Keep old fields for backward compatibility
    BeginDate string `json:"beginDate"`
    BeginTime string `json:"beginTime"`
    EndDate   string `json:"endDate"`
    EndTime   string `json:"endTime"`
    Timezone  string `json:"timezone"`
}

type MeetingInvite struct {
    Title     string    `json:"title"`
    StartTime time.Time `json:"startTime"` // Now time.Time instead of string
    Duration  int       `json:"duration"`
}

// Add custom JSON marshaling for backward compatibility
func (s *ScheduleInfo) MarshalJSON() ([]byte, error) {
    formatter := datetime.NewFormatter(nil)
    
    // Populate legacy fields from time.Time fields
    if !s.ImplementationStart.IsZero() {
        s.BeginDate = formatter.ToDateOnly(s.ImplementationStart)
        s.BeginTime = formatter.ToTimeOnly(s.ImplementationStart)
    }
    
    type Alias ScheduleInfo
    return json.Marshal((*Alias)(s))
}
```

#### Node.js - Update Schemas

**Before:**
```javascript
const ScheduleInfoSchema = {
    beginDate: String,
    beginTime: String,
    endDate: String,
    endTime: String,
    timezone: String
};

const MeetingInviteSchema = {
    title: String,
    startTime: String, // Was string
    duration: Number
};
```

**After:**
```javascript
const ScheduleInfoSchema = {
    // New Date fields (primary)
    implementationStart: Date,
    implementationEnd: Date,
    
    // Keep old fields for backward compatibility
    beginDate: String,
    beginTime: String,
    endDate: String,
    endTime: String,
    timezone: String
};

const MeetingInviteSchema = {
    title: String,
    startTime: Date, // Now Date object instead of string
    duration: Number
};

// Helper function to populate legacy fields
function populateLegacyFields(scheduleInfo) {
    const formatter = new Formatter();
    
    if (scheduleInfo.implementationStart) {
        scheduleInfo.beginDate = formatter.toDateOnly(scheduleInfo.implementationStart);
        scheduleInfo.beginTime = formatter.toTimeOnly(scheduleInfo.implementationStart);
    }
    
    return scheduleInfo;
}
```

### Step 5: Update Meeting Scheduling Code

#### Go - Meeting Creation

**Before:**
```go
func createMeeting(beginDate, beginTime, timezone string) error {
    // Manual timestamp creation
    timestamp := beginDate + "T" + beginTime + ":00"
    if timezone == "EST" {
        timestamp += "-05:00"
    }
    
    meetingTime, err := time.Parse(time.RFC3339, timestamp)
    if err != nil {
        return err
    }
    
    // Manual Graph API formatting
    graphTime := meetingTime.UTC().Format("2006-01-02T15:04:05.0000000")
    
    return callGraphAPI(graphTime)
}
```

**After:**
```go
func createMeeting(beginDate, beginTime, timezone string) error {
    parser := datetime.NewParser(nil)
    validator := datetime.NewValidator(nil)
    formatter := datetime.NewFormatter(nil)
    
    // Parse using centralized utilities
    meetingTime, err := parser.ParseLegacyDateTimeFields(beginDate, beginTime, timezone)
    if err != nil {
        return fmt.Errorf("failed to parse meeting time: %w", err)
    }
    
    // Validate meeting time
    if err := validator.ValidateMeetingTime(meetingTime); err != nil {
        return fmt.Errorf("invalid meeting time: %w", err)
    }
    
    // Format for Graph API
    graphTime := formatter.ToMicrosoftGraph(meetingTime)
    
    return callGraphAPI(graphTime)
}
```

#### Node.js - Meeting Creation

**Before:**
```javascript
async function createMeeting(beginDate, beginTime, timezone) {
    // Manual timestamp creation
    let timestamp = `${beginDate}T${beginTime}:00`;
    if (timezone === 'EST') {
        timestamp += '-05:00';
    }
    
    const meetingTime = new Date(timestamp);
    
    // Manual Graph API formatting
    const graphTime = meetingTime.toISOString().replace(/\.\d{3}Z$/, '.0000000');
    
    return await callGraphAPI(graphTime);
}
```

**After:**
```javascript
import { Parser, Validator, Formatter } from './datetime/index.js';

async function createMeeting(beginDate, beginTime, timezone) {
    const parser = new Parser();
    const validator = new Validator();
    const formatter = new Formatter();
    
    try {
        // Parse using centralized utilities
        const meetingTime = parser.parseLegacyDateTimeFields(beginDate, beginTime, timezone);
        
        // Validate meeting time
        validator.validateMeetingTime(meetingTime);
        
        // Format for Graph API
        const graphTime = formatter.toMicrosoftGraph(meetingTime);
        
        return await callGraphAPI(graphTime);
    } catch (err) {
        if (err instanceof DateTimeError) {
            throw new Error(`Meeting creation failed: ${err.message}`);
        }
        throw err;
    }
}
```

### Step 6: Update Error Handling

#### Go - Structured Error Handling

**Before:**
```go
parsed, err := time.Parse("2006-01-02", input)
if err != nil {
    return fmt.Errorf("invalid date format")
}
```

**After:**
```go
parser := datetime.NewParser(nil)
parsed, err := parser.ParseDate(input)
if err != nil {
    if dtErr, ok := err.(*datetime.DateTimeError); ok {
        switch dtErr.Type {
        case datetime.ErrInvalidFormat:
            return fmt.Errorf("invalid date format: expected YYYY-MM-DD or MM/DD/YYYY, got '%s'", input)
        case datetime.ErrInvalidTimezone:
            return fmt.Errorf("invalid timezone: %s", dtErr.Message)
        default:
            return fmt.Errorf("date parsing error: %s", dtErr.Message)
        }
    }
    return err
}
```

#### Node.js - Structured Error Handling

**Before:**
```javascript
try {
    const date = new Date(input);
    if (isNaN(date.getTime())) {
        throw new Error('Invalid date');
    }
} catch (err) {
    console.log('Date parsing failed');
}
```

**After:**
```javascript
import { Parser, DateTimeError, ERROR_TYPES } from './datetime/index.js';

const parser = new Parser();

try {
    const date = parser.parseDateTime(input);
} catch (err) {
    if (err instanceof DateTimeError) {
        switch (err.type) {
            case ERROR_TYPES.INVALID_FORMAT:
                console.log(`Invalid date format: expected YYYY-MM-DD or MM/DD/YYYY, got '${input}'`);
                break;
            case ERROR_TYPES.INVALID_TIMEZONE:
                console.log(`Invalid timezone: ${err.message}`);
                break;
            default:
                console.log(`Date parsing error: ${err.message}`);
        }
    } else {
        console.log('Unexpected error:', err);
    }
}
```

### Step 7: Update Lambda Functions

#### Go Lambda - Handler Updates

**Before:**
```go
func handleScheduleRequest(ctx context.Context, event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    var req ScheduleRequest
    json.Unmarshal([]byte(event.Body), &req)
    
    // Manual parsing
    startTime := req.BeginDate + "T" + req.BeginTime + ":00-05:00"
    parsed, _ := time.Parse(time.RFC3339, startTime)
    
    // Process...
}
```

**After:**
```go
func handleScheduleRequest(ctx context.Context, event events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
    var req ScheduleRequest
    json.Unmarshal([]byte(event.Body), &req)
    
    parser := datetime.NewParser(nil)
    validator := datetime.NewValidator(nil)
    
    // Centralized parsing
    startTime, err := parser.ParseLegacyDateTimeFields(req.BeginDate, req.BeginTime, req.Timezone)
    if err != nil {
        return events.APIGatewayProxyResponse{
            StatusCode: 400,
            Body:       fmt.Sprintf(`{"error": "Invalid date/time: %s"}`, err.Error()),
        }, nil
    }
    
    // Validate
    if err := validator.ValidateMeetingTime(startTime); err != nil {
        return events.APIGatewayProxyResponse{
            StatusCode: 400,
            Body:       fmt.Sprintf(`{"error": "Invalid meeting time: %s"}`, err.Error()),
        }, nil
    }
    
    // Process...
}
```

#### Node.js Lambda - Handler Updates

**Before:**
```javascript
exports.handler = async (event) => {
    const req = JSON.parse(event.body);
    
    // Manual parsing
    const startTime = new Date(`${req.beginDate}T${req.beginTime}:00-05:00`);
    
    // Process...
};
```

**After:**
```javascript
import { Parser, Validator, DateTimeError } from './datetime/index.js';

export const handler = async (event) => {
    try {
        const req = JSON.parse(event.body);
        
        const parser = new Parser();
        const validator = new Validator();
        
        // Centralized parsing
        const startTime = parser.parseLegacyDateTimeFields(req.beginDate, req.beginTime, req.timezone);
        
        // Validate
        validator.validateMeetingTime(startTime);
        
        // Process...
        
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

## Migration Checklist

### Pre-Migration
- [ ] Backup existing data
- [ ] Identify all datetime usage in codebase
- [ ] Plan rollback strategy
- [ ] Set up monitoring for datetime errors

### Dependencies
- [ ] Update Node.js package.json
- [ ] Remove date-fns imports
- [ ] Install dayjs
- [ ] Update import statements

### Code Updates
- [ ] Replace manual parsing with Parser class
- [ ] Replace manual formatting with Formatter class
- [ ] Add validation using Validator class
- [ ] Update data structures with time.Time fields
- [ ] Update error handling for DateTimeError
- [ ] Update Lambda handlers
- [ ] Update API endpoints

### Testing
- [ ] Unit tests pass with new utilities
- [ ] Integration tests pass
- [ ] Cross-language compatibility tests pass
- [ ] Backward compatibility tests pass
- [ ] Performance tests show no regression

### Deployment
- [ ] Deploy to staging environment
- [ ] Verify existing data still works
- [ ] Test new datetime operations
- [ ] Monitor for errors
- [ ] Deploy to production

### Post-Migration
- [ ] Monitor error rates
- [ ] Verify Microsoft Graph API integration
- [ ] Check meeting creation functionality
- [ ] Validate timezone handling
- [ ] Clean up deprecated code (after grace period)

## Rollback Plan

If issues occur during migration:

1. **Immediate Rollback**: Revert to previous version
2. **Partial Rollback**: Keep new utilities but restore old data structures
3. **Data Recovery**: Restore from backup if data corruption occurs

## Timeline

- **Week 1**: Update dependencies and basic parsing
- **Week 2**: Update data structures and Lambda handlers
- **Week 3**: Full integration testing and deployment
- **Week 4**: Monitor and clean up deprecated code

## Support

For migration issues:
1. Check troubleshooting section in main documentation
2. Review error messages for specific guidance
3. Test with known good datetime values
4. Verify timezone configuration

This migration is critical for system reliability and must be completed carefully with thorough testing at each step.
## T
roubleshooting Common Migration Issues

### 1. Dependency Issues

#### Problem: `Cannot find module 'date-fns'`
```
Error: Cannot find module 'date-fns'
```

**Solution:**
```bash
# Remove all date-fns references
npm uninstall date-fns date-fns-tz
# Install dayjs
npm install dayjs
# Update imports in code
```

#### Problem: `dayjs is not a function`
```
TypeError: dayjs is not a function
```

**Solution:**
```javascript
// Wrong import
import dayjs from 'dayjs';

// Correct import for ES modules
import dayjs from 'dayjs';
// Or for CommonJS
const dayjs = require('dayjs');
```

### 2. Parsing Issues

#### Problem: `Invalid date format` errors
```
DateTimeError: unable to parse date/time: expected formats like '2006-01-02T15:04:05Z' or '01/02/2006 3:04 PM', got '15/01/2025'
```

**Solution:**
European date format (`15/01/2025`) is not supported. Use:
- US format: `01/15/2025`
- ISO format: `2025-01-15`
- Natural format: `January 15, 2025`

#### Problem: Timezone parsing failures
```
DateTimeError: invalid timezone: EST
```

**Solution:**
Use IANA timezone identifiers instead of abbreviations:
```javascript
// Wrong
const timezone = 'EST';

// Correct
const timezone = 'America/New_York';
```

### 3. Data Structure Issues

#### Problem: `time.Time` serialization errors
```
json: error calling MarshalJSON for type time.Time
```

**Solution:**
Implement custom JSON marshaling:
```go
func (s *ScheduleInfo) MarshalJSON() ([]byte, error) {
    formatter := datetime.NewFormatter(nil)
    
    type Alias ScheduleInfo
    aux := &struct {
        ImplementationStartStr string `json:"implementationStart"`
        *Alias
    }{
        ImplementationStartStr: formatter.ToRFC3339(s.ImplementationStart),
        Alias: (*Alias)(s),
    }
    
    return json.Marshal(aux)
}
```

#### Problem: Legacy field population
```
Error: beginDate is undefined after migration
```

**Solution:**
Ensure legacy fields are populated from time.Time fields:
```go
// In your conversion function
if !schedule.ImplementationStart.IsZero() {
    formatter := datetime.NewFormatter(nil)
    schedule.BeginDate = formatter.ToDateOnly(schedule.ImplementationStart)
    schedule.BeginTime = formatter.ToTimeOnly(schedule.ImplementationStart)
}
```

### 4. Validation Issues

#### Problem: `meeting time cannot be in the past`
```
DateTimeError: meeting time cannot be in the past (tolerance: 5m0s)
```

**Solution:**
Adjust configuration or check input:
```go
// Increase tolerance
config := &datetime.DateTimeConfig{
    FutureTolerance: 10 * time.Minute,
    AllowPastDates:  true, // For testing
}
parser := datetime.NewParser(config)
```

#### Problem: Business hours validation failures
```
DateTimeError: meeting time falls on weekend
```

**Solution:**
Either fix the input or skip business hours validation:
```go
// Skip business hours validation for system meetings
validator := datetime.NewValidator(nil)
// Don't call ValidateBusinessHours for automated meetings
```

### 5. Microsoft Graph API Issues

#### Problem: Graph API rejects datetime format
```
Graph API Error: Invalid datetime format
```

**Solution:**
Ensure you're using the correct Graph format:
```go
// Wrong - using RFC3339
graphTime := formatter.ToRFC3339(meetingTime)

// Correct - using Graph format
graphTime := formatter.ToMicrosoftGraph(meetingTime)
// Produces: 2025-01-15T15:00:00.0000000
```

#### Problem: Timezone issues with Graph API
```
Graph API Error: Meeting time doesn't match expected timezone
```

**Solution:**
Graph API expects UTC time:
```go
// Ensure time is converted to UTC for Graph API
formatter := datetime.NewFormatter(nil)
graphTime := formatter.ToMicrosoftGraph(meetingTime) // Automatically converts to UTC
```

### 6. Cross-Language Consistency Issues

#### Problem: Go and Node.js produce different outputs
```
Go output:    2025-01-15T10:00:00-05:00
Node.js output: 2025-01-15T15:00:00Z
```

**Solution:**
Ensure both use the same configuration:
```go
// Go
config := &datetime.DateTimeConfig{
    DefaultTimezone: "America/New_York",
}

// Node.js
const config = new DateTimeConfig({
    defaultTimezone: 'America/New_York'
});
```

### 7. Performance Issues

#### Problem: Slow datetime operations
```
Timeout: datetime parsing taking too long
```

**Solution:**
Reuse parser instances and cache timezone data:
```go
// Create once, reuse many times
var globalParser = datetime.NewParser(nil)

func parseDateTime(input string) (time.Time, error) {
    return globalParser.ParseDateTime(input)
}
```

### 8. Legacy Data Migration

#### Problem: Existing data doesn't work with new validation
```
DateTimeError: date/time is too far in the past
```

**Solution:**
Use migration-specific configuration:
```go
// For data migration only
migrationConfig := &datetime.DateTimeConfig{
    AllowPastDates:  true,
    FutureTolerance: 0,
}
parser := datetime.NewParser(migrationConfig)
```

### 9. Testing Issues

#### Problem: Tests fail with new validation
```
Test failed: meeting time validation
```

**Solution:**
Update test data to use valid future dates:
```go
// Wrong - hardcoded past date
testTime := time.Date(2020, 1, 1, 10, 0, 0, 0, time.UTC)

// Correct - relative future date
testTime := time.Now().Add(24 * time.Hour)
```

### 10. Import/Export Issues

#### Problem: Cannot import datetime utilities
```
Error: Cannot resolve module './datetime/index.js'
```

**Solution:**
Check file structure and exports:
```javascript
// Ensure datetime/index.js exists and exports correctly
export { Parser } from './parser.js';
export { Formatter } from './formatter.js';
export { Validator } from './validator.js';
export { DateTimeConfig, DateTimeError, ERROR_TYPES } from './types.js';
```

## Emergency Rollback Procedures

### If Critical Issues Occur:

1. **Immediate Actions:**
   ```bash
   # Revert to previous version
   git revert <migration-commit>
   
   # Restore dependencies
   npm install date-fns@^3.0.0 date-fns-tz@^2.0.0
   
   # Redeploy previous version
   ```

2. **Data Recovery:**
   ```bash
   # Restore from backup
   aws s3 cp s3://backup-bucket/data-backup.json ./
   
   # Verify data integrity
   node verify-data.js
   ```

3. **Monitoring:**
   ```bash
   # Check error rates
   aws logs filter-log-events --log-group-name /aws/lambda/function-name
   
   # Monitor datetime operations
   grep "DateTimeError" /var/log/application.log
   ```

## Getting Help

1. **Check Documentation**: Review the main datetime utilities documentation
2. **Error Messages**: DateTimeError messages include specific guidance
3. **Test with Known Values**: Use RFC3339 format for testing
4. **Cross-Reference**: Compare Go and Node.js outputs for consistency
5. **Gradual Migration**: Migrate one component at a time to isolate issues

## Post-Migration Validation

After completing migration, verify:

- [ ] All datetime operations use centralized utilities
- [ ] No direct date-fns imports remain
- [ ] Microsoft Graph API integration works
- [ ] Meeting scheduling functions correctly
- [ ] Timezone handling is consistent
- [ ] Error handling provides useful messages
- [ ] Performance is acceptable
- [ ] Cross-language outputs match
- [ ] Legacy data still works
- [ ] New validation rules are enforced