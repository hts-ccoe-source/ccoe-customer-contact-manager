/**
 * Parser handles parsing of various date/time input formats
 * Equivalent to internal/datetime/parser.go
 */

import dayjs from 'dayjs';
import utc from 'dayjs/plugin/utc.js';
import timezone from 'dayjs/plugin/timezone.js';
import customParseFormat from 'dayjs/plugin/customParseFormat.js';
import { DateTimeConfig, defaultConfig, DateTimeError, newDateTimeError, ERROR_TYPES, COMMON_INPUT_FORMATS } from './types.js';

// Configure Day.js plugins
dayjs.extend(utc);
dayjs.extend(timezone);
dayjs.extend(customParseFormat);

export class Parser {
    constructor(config) {
        this.config = config || defaultConfig();
    }
    
    /**
     * Attempts to parse a date/time string using multiple formats
     * @param {string} input - The date/time string to parse
     * @returns {Date} - Parsed Date object
     * @throws {DateTimeError} - If parsing fails
     */
    parseDateTime(input) {
        if (!input || typeof input !== 'string') {
            throw newDateTimeError(
                ERROR_TYPES.INVALID_FORMAT,
                'empty date/time input',
                input,
                null
            );
        }
        
        // Clean up the input
        input = input.trim();
        
        // Try parsing with Day.js
        let parsed;
        try {
            parsed = this._parseWithDayjs(input);
        } catch (err) {
            throw newDateTimeError(
                ERROR_TYPES.INVALID_FORMAT,
                `unable to parse date/time: expected formats like '2006-01-02T15:04:05Z' or '01/02/2006 3:04 PM', got '${input}'`,
                input,
                err
            );
        }
        
        // Check if input explicitly specified timezone
        const hasTimezone = input.includes('Z') || input.includes('+') || 
                           (input.includes('T') && input.lastIndexOf('-') > input.lastIndexOf('T')) ||
                           input.toUpperCase().endsWith('UTC');
        
        if (!hasTimezone) {
            // Apply default timezone - interpret the parsed time as being in the default timezone
            try {
                // Use Day.js to create a time in the default timezone
                parsed = dayjs.tz(parsed.format('YYYY-MM-DD HH:mm:ss'), this.config.defaultTimezone);
            } catch (err) {
                throw newDateTimeError(
                    ERROR_TYPES.INVALID_TIMEZONE,
                    `invalid default timezone: ${this.config.defaultTimezone}`,
                    input,
                    err
                );
            }
        }
        
        // Convert to native Date object and store timezone info
        const result = parsed.toDate();
        
        // Store original timezone info for formatting
        if (input.includes('Z')) {
            result._originalTimezone = 'UTC';
        } else if (hasTimezone && input.includes('T') && input.lastIndexOf('-') > input.lastIndexOf('T')) {
            // For timezone offsets like -05:00, we need to map them to timezone names
            // This matches Go's behavior which shows "EST" instead of "-05:00"
            const tzPart = input.substring(input.lastIndexOf('-'));
            result._originalTimezone = this._mapOffsetToTimezone(tzPart);
        } else if (hasTimezone && input.includes('+')) {
            // For positive timezone offsets like +08:00
            const tzPart = input.substring(input.lastIndexOf('+'));
            result._originalTimezone = this._mapOffsetToTimezone('+' + tzPart);
        } else if (!hasTimezone) {
            result._originalTimezone = this.config.defaultTimezone;
        }
        
        return result;
    }
    
    /**
     * Parses a date-only string and returns a Date at midnight in the default timezone
     * @param {string} input - The date string to parse
     * @returns {Date} - Parsed Date object at midnight
     * @throws {DateTimeError} - If parsing fails
     */
    parseDate(input) {
        if (!input || typeof input !== 'string') {
            throw newDateTimeError(
                ERROR_TYPES.INVALID_FORMAT,
                'empty date input',
                input,
                null
            );
        }
        
        input = input.trim();
        
        // Try parsing with Day.js using various date formats
        let parsed;
        const dateFormats = [
            'YYYY-MM-DD',
            'MM/DD/YYYY',
            'M/D/YYYY',
            'MMMM D, YYYY',
            'MMM D, YYYY',
            'D MMMM YYYY',
            'D MMM YYYY'
        ];
        
        try {
            // Try each format
            for (const format of dateFormats) {
                parsed = dayjs(input, format, true); // strict parsing
                if (parsed.isValid()) {
                    break;
                }
            }
            
            if (!parsed || !parsed.isValid()) {
                throw new Error('No valid format found');
            }
        } catch (err) {
            throw newDateTimeError(
                ERROR_TYPES.INVALID_FORMAT,
                `unable to parse date: expected formats like '2006-01-02' or '01/02/2006', got '${input}'`,
                input,
                err
            );
        }
        
        // Set to midnight in the default timezone
        try {
            // Create a date at midnight in the default timezone using Day.js
            const result = dayjs.tz(parsed.format('YYYY-MM-DD') + ' 00:00:00', this.config.defaultTimezone).toDate();
            result._originalTimezone = this.config.defaultTimezone;
            return result;
        } catch (err) {
            throw newDateTimeError(
                ERROR_TYPES.INVALID_TIMEZONE,
                `invalid default timezone: ${this.config.defaultTimezone}`,
                input,
                err
            );
        }
    }
    
    /**
     * Parses a time-only string and combines it with the given date
     * @param {string} input - The time string to parse
     * @param {Date} date - The date to combine with
     * @returns {Date} - Combined Date object
     * @throws {DateTimeError} - If parsing fails
     */
    parseTime(input, date) {
        if (!input || typeof input !== 'string') {
            throw newDateTimeError(
                ERROR_TYPES.INVALID_FORMAT,
                'empty time input',
                input,
                null
            );
        }
        
        if (!date || !(date instanceof Date)) {
            throw newDateTimeError(
                ERROR_TYPES.INVALID_FORMAT,
                'invalid date parameter',
                input,
                null
            );
        }
        
        input = input.trim();
        
        // Time-only formats for Day.js
        const timeFormats = [
            'HH:mm:ss',
            'HH:mm',
            'h:mm:ss A',
            'h:mm A',
            'h:mm:ssA',
            'h:mmA'
        ];
        
        let parsed;
        try {
            // Try each time format
            for (const format of timeFormats) {
                parsed = dayjs(input, format, true); // strict parsing
                if (parsed.isValid()) {
                    break;
                }
            }
            
            if (!parsed || !parsed.isValid()) {
                throw new Error('No valid format found');
            }
        } catch (err) {
            throw newDateTimeError(
                ERROR_TYPES.INVALID_FORMAT,
                `unable to parse time: expected formats like '15:04' or '3:04 PM', got '${input}'`,
                input,
                err
            );
        }
        
        // Combine with the provided date, preserving the date's timezone context
        const baseDayjs = dayjs(date);
        const combined = baseDayjs
            .hour(parsed.hour())
            .minute(parsed.minute())
            .second(parsed.second())
            .millisecond(parsed.millisecond());
        
        const result = combined.toDate();
        result._originalTimezone = date._originalTimezone;
        return result;
    }
    
    /**
     * Parse input using Day.js with multiple format attempts
     * @param {string} input - The input string to parse
     * @returns {dayjs.Dayjs} - Parsed Day.js object
     * @throws {Error} - If all formats fail
     */
    _parseWithDayjs(input) {
        // First try standard Day.js parsing (handles ISO formats automatically)
        let parsed = dayjs(input);
        if (parsed.isValid()) {
            return parsed;
        }
        
        // Try specific formats
        const formats = [
            // ISO8601/RFC3339 formats
            'YYYY-MM-DDTHH:mm:ssZ',
            'YYYY-MM-DDTHH:mm:ss',
            'YYYY-MM-DDTHH:mm',
            
            // Date only formats
            'YYYY-MM-DD',
            'MM/DD/YYYY',
            'M/D/YYYY',
            'MMMM D, YYYY',
            'MMM D, YYYY',
            'D MMMM YYYY',
            'D MMM YYYY',
            
            // Combined date/time formats
            'YYYY-MM-DD HH:mm:ss',
            'YYYY-MM-DD HH:mm',
            'MM/DD/YYYY HH:mm:ss',
            'MM/DD/YYYY HH:mm',
            'MM/DD/YYYY h:mm A',
            'MMMM D, YYYY h:mm A',
            'MMMM D, YYYY [at] h:mm A'
        ];
        
        for (const format of formats) {
            parsed = dayjs(input, format, true); // strict parsing
            if (parsed.isValid()) {
                return parsed;
            }
        }
        
        throw new Error('No valid format found');
    }
    
    /**
     * Parses a date/time string and applies the specified timezone
     * @param {string} input - The date/time string to parse
     * @param {string} timezone - The timezone to apply
     * @returns {Date} - Parsed Date object in specified timezone
     * @throws {DateTimeError} - If parsing fails
     */
    parseDateTimeWithTimezone(input, timezone) {
        // If no timezone specified, use normal parsing
        if (!timezone) {
            return this.parseDateTime(input);
        }
        
        try {
            // Validate timezone first using Intl API (more reliable than Day.js)
            try {
                new Intl.DateTimeFormat('en-US', { timeZone: timezone });
            } catch (err) {
                throw newDateTimeError(
                    ERROR_TYPES.INVALID_TIMEZONE,
                    `invalid timezone: ${timezone}: unknown time zone ${timezone}`,
                    input,
                    err
                );
            }
            
            // Check if input explicitly specified timezone
            const cleaned = input.trim();
            const hasTimezone = cleaned.includes('Z') || cleaned.includes('+') || 
                               (cleaned.includes('T') && cleaned.lastIndexOf('-') > cleaned.lastIndexOf('T')) ||
                               cleaned.toUpperCase().endsWith('UTC');
            
            if (hasTimezone) {
                // Input already has timezone, parse normally then convert to target timezone
                const parsed = dayjs(cleaned);
                if (!parsed.isValid()) {
                    throw newDateTimeError(
                        ERROR_TYPES.INVALID_FORMAT,
                        `unable to parse date/time: expected formats like '2006-01-02T15:04:05Z' or '01/02/2006 3:04 PM', got '${cleaned}'`,
                        cleaned,
                        null
                    );
                }
                
                const converted = parsed.tz(timezone);
                const result = converted.toDate();
                result._originalTimezone = timezone;
                return result;
            } else {
                // Input has no timezone, interpret it as being in the specified timezone
                // This is the key: we want to treat the wall clock time as being in the target timezone
                
                let parsed;
                try {
                    parsed = this._parseWithDayjs(cleaned);
                } catch (err) {
                    throw newDateTimeError(
                        ERROR_TYPES.INVALID_FORMAT,
                        `unable to parse date/time: expected formats like '2006-01-02T15:04:05Z' or '01/02/2006 3:04 PM', got '${cleaned}'`,
                        cleaned,
                        err
                    );
                }
                
                // Use Day.js to interpret the wall clock time as being in the target timezone
                // This matches Go's time.Date() behavior exactly
                const result = dayjs.tz(parsed.format('YYYY-MM-DD HH:mm:ss'), timezone).toDate();
                result._originalTimezone = timezone;
                return result;
            }
        } catch (err) {
            if (err instanceof DateTimeError) {
                throw err;
            }
            throw newDateTimeError(
                ERROR_TYPES.INVALID_TIMEZONE,
                `invalid timezone: ${timezone}: ${err.message}`,
                input,
                err
            );
        }
    }
    
    /**
     * Parses separate date and time fields (for backward compatibility)
     * @param {string} date - The date string
     * @param {string} timeStr - The time string
     * @param {string} timezone - The timezone string
     * @returns {Date} - Combined Date object
     * @throws {DateTimeError} - If parsing fails
     */
    parseLegacyDateTimeFields(date, timeStr, timezone) {
        if (!date) {
            throw newDateTimeError(
                ERROR_TYPES.INVALID_FORMAT,
                'date field is required',
                date,
                null
            );
        }
        
        // Parse the date
        let parsedDate = this.parseDate(date);
        
        // If time is provided, parse and combine it
        if (timeStr) {
            parsedDate = this.parseTime(timeStr, parsedDate);
        }
        
        // Apply timezone if specified
        if (timezone) {
            try {
                // Interpret the parsed date as being in the specified timezone using Day.js
                const dayjsDate = dayjs(parsedDate);
                const result = dayjs.tz(dayjsDate.format('YYYY-MM-DD HH:mm:ss'), timezone).toDate();
                result._originalTimezone = timezone;
                parsedDate = result;
            } catch (err) {
                throw newDateTimeError(
                    ERROR_TYPES.INVALID_TIMEZONE,
                    `invalid timezone: ${timezone}`,
                    timezone,
                    err
                );
            }
        }
        
        return parsedDate;
    }
    
    /**
     * Map timezone offset to timezone name (to match Go's behavior)
     * @private
     * @param {string} offset - Timezone offset like "-05:00" or "+08:00"
     * @returns {string} - Timezone name or the original offset
     */
    _mapOffsetToTimezone(offset) {
        // Common timezone offset mappings (for January - EST/PST are in standard time)
        const offsetMap = {
            '-05:00': 'America/New_York',  // EST
            '-08:00': 'America/Los_Angeles', // PST
            '-06:00': 'America/Chicago',   // CST
            '-07:00': 'America/Denver',    // MST
            '+00:00': 'UTC',
            'Z': 'UTC'
        };
        
        return offsetMap[offset] || offset;
    }

}