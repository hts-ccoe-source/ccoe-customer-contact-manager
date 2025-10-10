/**
 * Standard format constants and types for consistent date/time handling
 * Equivalent to internal/datetime/types.go
 */

// Standard format constants for consistent date/time handling
export const FORMATS = {
    // RFC3339 format is the canonical internal format for all date/time storage
    RFC3339: 'YYYY-MM-DDTHH:mm:ssZ',
    
    // Graph format is the format required by Microsoft Graph API
    GRAPH: 'YYYY-MM-DDTHH:mm:ss.0000000',
    
    // ICS format is the format used for calendar files (iCalendar)
    ICS: 'YYYYMMDDTHHmmssZ',
    
    // Log format is the format used for structured logging with milliseconds
    LOG: 'YYYY-MM-DDTHH:mm:ss.SSSZ',
    
    // Human-readable formats
    HUMAN_DATE: 'MMMM d, yyyy',
    HUMAN_TIME: 'h:mm a zzz'
};

// Common input formats that the parser should accept
export const COMMON_INPUT_FORMATS = [
    // ISO8601/RFC3339 formats
    "yyyy-MM-dd'T'HH:mm:ssXXX",
    "yyyy-MM-dd'T'HH:mm:ss'Z'",
    "yyyy-MM-dd'T'HH:mm:ss",
    
    // Date only formats
    "yyyy-MM-dd",
    "MM/dd/yyyy",
    "M/d/yyyy",
    "MMMM d, yyyy",
    "MMM d, yyyy",
    "d MMMM yyyy",
    "d MMM yyyy",
    
    // Time only formats
    "HH:mm:ss",
    "HH:mm",
    "h:mm:ss a",
    "h:mm a",
    "h:mm:ssa",
    "h:mma",
    
    // Combined date/time formats
    "yyyy-MM-dd HH:mm:ss",
    "yyyy-MM-dd HH:mm",
    "MM/dd/yyyy HH:mm:ss",
    "MM/dd/yyyy HH:mm",
    "MM/dd/yyyy h:mm a",
    "MMMM d, yyyy h:mm a",
    "MMMM d, yyyy 'at' h:mm a"
];

// Error types for standardized error handling
export const ERROR_TYPES = {
    INVALID_FORMAT: 'INVALID_FORMAT',
    INVALID_TIMEZONE: 'INVALID_TIMEZONE',
    INVALID_RANGE: 'INVALID_RANGE',
    PAST_DATE: 'PAST_DATE',
    FUTURE_DATE: 'FUTURE_DATE'
};

/**
 * DateTimeConfig holds configuration for date/time operations
 */
export class DateTimeConfig {
    constructor(options = {}) {
        // Default timezone is used when no timezone is specified in input
        this.defaultTimezone = options.defaultTimezone || 'America/New_York';
        
        // Allow past dates controls whether past dates are allowed in validation
        this.allowPastDates = options.allowPastDates || false;
        
        // Future tolerance is the grace period for "future" date validation
        // (e.g., allow dates up to 5 minutes in the past for meeting times)
        this.futureTolerance = options.futureTolerance || 300000; // 5 minutes in milliseconds
    }
}

/**
 * Returns a sensible default configuration
 */
export function defaultConfig() {
    return new DateTimeConfig({
        defaultTimezone: 'America/New_York', // EST/EDT
        allowPastDates: false,
        futureTolerance: 300000 // 5 minutes
    });
}

/**
 * DateTimeError represents a standardized date/time error
 */
export class DateTimeError extends Error {
    constructor(type, message, input, cause) {
        super(message);
        this.name = 'DateTimeError';
        this.type = type;
        this.input = input;
        this.cause = cause;
        
        // Maintain proper stack trace for where our error was thrown (only available on V8)
        if (Error.captureStackTrace) {
            Error.captureStackTrace(this, DateTimeError);
        }
    }
    
    toString() {
        if (this.cause) {
            return `${this.message}: ${this.cause.message}`;
        }
        return this.message;
    }
}

/**
 * Creates a new DateTimeError
 */
export function newDateTimeError(errorType, message, input, cause) {
    return new DateTimeError(errorType, message, input, cause);
}