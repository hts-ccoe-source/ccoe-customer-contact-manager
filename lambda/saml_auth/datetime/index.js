/**
 * Main entry point for the datetime utilities package
 * Provides a unified interface equivalent to internal/datetime/datetime.go
 */

import { Parser } from './parser.js';
import { Formatter } from './formatter.js';
import { Validator } from './validator.js';
import { 
    DateTimeConfig, 
    defaultConfig, 
    DateTimeError, 
    newDateTimeError, 
    ERROR_TYPES, 
    FORMATS, 
    COMMON_INPUT_FORMATS 
} from './types.js';

/**
 * DateTime provides a unified interface for all date/time operations
 * This is the main class that applications should use
 */
export class DateTime {
    constructor(config) {
        this.config = config || defaultConfig();
        this.parser = new Parser(this.config);
        this.formatter = new Formatter(this.config);
        this.validator = new Validator(this.config);
    }
    
    // Parser methods
    parseDateTime(input) {
        return this.parser.parseDateTime(input);
    }
    
    parseDate(input) {
        return this.parser.parseDate(input);
    }
    
    parseTime(input, date) {
        return this.parser.parseTime(input, date);
    }
    
    parseWithFormats(input, formats) {
        return this.parser.parseWithFormats(input, formats);
    }
    
    parseDateTimeWithTimezone(input, timezone) {
        return this.parser.parseDateTimeWithTimezone(input, timezone);
    }
    
    parseLegacyDateTimeFields(date, timeStr, timezone) {
        return this.parser.parseLegacyDateTimeFields(date, timeStr, timezone);
    }
    
    // Formatter methods
    toRFC3339(date) {
        return this.formatter.toRFC3339(date);
    }
    
    toMicrosoftGraph(date) {
        return this.formatter.toMicrosoftGraph(date);
    }
    
    toHumanReadable(date, timezone) {
        return this.formatter.toHumanReadable(date, timezone);
    }
    
    toICS(date) {
        return this.formatter.toICS(date);
    }
    
    toLogFormat(date) {
        return this.formatter.toLogFormat(date);
    }
    
    toDateOnly(date) {
        return this.formatter.toDateOnly(date);
    }
    
    toTimeOnly(date) {
        return this.formatter.toTimeOnly(date);
    }
    
    toTimeOnly12Hour(date) {
        return this.formatter.toTimeOnly12Hour(date);
    }
    
    toEmailTemplate(date, timezone) {
        return this.formatter.toEmailTemplate(date, timezone);
    }
    
    toScheduleWindow(start, end, timezone) {
        return this.formatter.toScheduleWindow(start, end, timezone);
    }
    
    toTimezone(date, timezone) {
        return this.formatter.toTimezone(date, timezone);
    }
    
    formatDuration(milliseconds) {
        return this.formatter.formatDuration(milliseconds);
    }
    
    toLegacyFields(date) {
        return this.formatter.toLegacyFields(date);
    }
    
    // Validator methods
    validateDateTime(date) {
        return this.validator.validateDateTime(date);
    }
    
    validateDateRange(start, end) {
        return this.validator.validateDateRange(start, end);
    }
    
    validateTimezone(timezone) {
        return this.validator.validateTimezone(timezone);
    }
    
    validateMeetingTime(date) {
        return this.validator.validateMeetingTime(date);
    }
    
    validateBusinessHours(date, timezone) {
        return this.validator.validateBusinessHours(date, timezone);
    }
    
    validateScheduleWindow(start, end) {
        return this.validator.validateScheduleWindow(start, end);
    }
    
    validateMeetingDuration(durationMs) {
        return this.validator.validateMeetingDuration(durationMs);
    }
    
    validateTimezonePair(date1, date2) {
        return this.validator.validateTimezonePair(date1, date2);
    }
}

// Export individual classes for direct use
export { Parser, Formatter, Validator };

// Export types and constants
export { 
    DateTimeConfig, 
    defaultConfig, 
    DateTimeError, 
    newDateTimeError, 
    ERROR_TYPES, 
    FORMATS, 
    COMMON_INPUT_FORMATS 
};

// Export a default instance for convenience
export const defaultDateTime = new DateTime();

// Convenience functions using the default instance
export function parseDateTime(input) {
    return defaultDateTime.parseDateTime(input);
}

export function parseDate(input) {
    return defaultDateTime.parseDate(input);
}

export function parseTime(input, date) {
    return defaultDateTime.parseTime(input, date);
}

export function toRFC3339(date) {
    return defaultDateTime.toRFC3339(date);
}

export function toMicrosoftGraph(date) {
    return defaultDateTime.toMicrosoftGraph(date);
}

export function toHumanReadable(date, timezone) {
    return defaultDateTime.toHumanReadable(date, timezone);
}

export function toLogFormat(date) {
    return defaultDateTime.toLogFormat(date);
}

export function validateDateTime(date) {
    return defaultDateTime.validateDateTime(date);
}

export function validateMeetingTime(date) {
    return defaultDateTime.validateMeetingTime(date);
}