/**
 * Validator handles validation of Date values according to business rules
 * Equivalent to internal/datetime/validator.go
 */

import dayjs from 'dayjs';
import utc from 'dayjs/plugin/utc.js';
import timezone from 'dayjs/plugin/timezone.js';
import { DateTimeConfig, defaultConfig, DateTimeError, newDateTimeError, ERROR_TYPES } from './types.js';
import { Formatter } from './formatter.js';

// Configure Day.js plugins
dayjs.extend(utc);
dayjs.extend(timezone);

export class Validator {
    constructor(config) {
        this.config = config || defaultConfig();
        this.formatter = new Formatter(this.config);
    }
    
    /**
     * Performs basic validation on a Date value
     * @param {Date} date - The Date object to validate
     * @throws {DateTimeError} - If validation fails
     */
    validateDateTime(date) {
        // Check if date is valid
        if (!date || !(date instanceof Date) || !dayjs(date).isValid()) {
            throw newDateTimeError(
                ERROR_TYPES.INVALID_FORMAT,
                'date/time cannot be invalid or null',
                '',
                null
            );
        }
        
        const now = dayjs();
        const dateJs = dayjs(date);
        
        // Check if time is too far in the future (sanity check)
        const maxFuture = now.add(10, 'year'); // 10 years from now
        if (dateJs.isAfter(maxFuture)) {
            throw newDateTimeError(
                ERROR_TYPES.FUTURE_DATE,
                `date/time is too far in the future: ${this.formatter.toRFC3339(date)}`,
                this.formatter.toRFC3339(date),
                null
            );
        }
        
        // Check if time is too far in the past (sanity check)
        const minPast = now.subtract(50, 'year'); // 50 years ago
        if (dateJs.isBefore(minPast)) {
            throw newDateTimeError(
                ERROR_TYPES.PAST_DATE,
                `date/time is too far in the past: ${this.formatter.toRFC3339(date)}`,
                this.formatter.toRFC3339(date),
                null
            );
        }
    }
    
    /**
     * Ensures that start time is before end time
     * @param {Date} start - The start Date object
     * @param {Date} end - The end Date object
     * @throws {DateTimeError} - If validation fails
     */
    validateDateRange(start, end) {
        // Validate individual times first
        this.validateDateTime(start);
        this.validateDateTime(end);
        
        const startJs = dayjs(start);
        const endJs = dayjs(end);
        
        // Check that start is before end
        if (!startJs.isBefore(endJs)) {
            throw newDateTimeError(
                ERROR_TYPES.INVALID_RANGE,
                `start time (${this.formatter.toRFC3339(start)}) must be before end time (${this.formatter.toRFC3339(end)})`,
                `${this.formatter.toRFC3339(start)} to ${this.formatter.toRFC3339(end)}`,
                null
            );
        }
        
        // Check that the range is reasonable (not more than 1 year)
        const maxDuration = 365 * 24 * 60 * 60 * 1000; // 1 year in milliseconds
        const duration = endJs.diff(startJs); // Day.js diff returns milliseconds
        
        if (duration > maxDuration) {
            throw newDateTimeError(
                ERROR_TYPES.INVALID_RANGE,
                `date range is too long: maximum allowed is 1 year, got ${this.formatter.formatDuration(duration)}`,
                `${this.formatter.toRFC3339(start)} to ${this.formatter.toRFC3339(end)}`,
                null
            );
        }
    }
    
    /**
     * Verifies that a timezone string is valid
     * @param {string} timezone - The timezone string to validate
     * @throws {DateTimeError} - If validation fails
     */
    validateTimezone(timezone) {
        if (!timezone || typeof timezone !== 'string') {
            throw newDateTimeError(
                ERROR_TYPES.INVALID_TIMEZONE,
                'timezone cannot be empty',
                timezone,
                null
            );
        }
        
        try {
            // Test if timezone is valid using Day.js
            const testDate = dayjs();
            testDate.tz(timezone);
            
            // Additional check using Intl.DateTimeFormat
            new Intl.DateTimeFormat('en-US', { timeZone: timezone });
        } catch (err) {
            throw newDateTimeError(
                ERROR_TYPES.INVALID_TIMEZONE,
                `invalid timezone: ${timezone}: ${err.message}`,
                timezone,
                err
            );
        }
    }
    
    /**
     * Validates that a meeting time is appropriate
     * @param {Date} date - The meeting Date object to validate
     * @throws {DateTimeError} - If validation fails
     */
    validateMeetingTime(date) {
        // Basic validation first
        this.validateDateTime(date);
        
        const now = dayjs();
        const dateJs = dayjs(date);
        
        // Check if meeting is in the past (with tolerance)
        if (!this.config.allowPastDates) {
            const earliestAllowed = now.subtract(this.config.futureTolerance, 'millisecond');
            if (dateJs.isBefore(earliestAllowed)) {
                throw newDateTimeError(
                    ERROR_TYPES.PAST_DATE,
                    `meeting time cannot be in the past (tolerance: ${this.formatter.formatDuration(this.config.futureTolerance)}): ${this.formatter.toRFC3339(date)}`,
                    this.formatter.toRFC3339(date),
                    null
                );
            }
        }
        
        // Check if meeting is too far in the future (business rule)
        const maxFuture = now.add(2, 'year'); // 2 years from now
        if (dateJs.isAfter(maxFuture)) {
            throw newDateTimeError(
                ERROR_TYPES.FUTURE_DATE,
                `meeting time is too far in the future (maximum: 2 years): ${this.formatter.toRFC3339(date)}`,
                this.formatter.toRFC3339(date),
                null
            );
        }
    }
    
    /**
     * Checks if a time falls within business hours
     * @param {Date} date - The Date object to check
     * @param {string} timezone - The timezone for business hours check
     * @throws {DateTimeError} - If validation fails
     */
    validateBusinessHours(date, timezone) {
        // Convert to specified timezone using Day.js
        let displayDate = dayjs(date);
        if (timezone) {
            try {
                displayDate = displayDate.tz(timezone);
            } catch (err) {
                throw newDateTimeError(
                    ERROR_TYPES.INVALID_TIMEZONE,
                    `invalid timezone: ${timezone}`,
                    timezone,
                    err
                );
            }
        }
        
        const hour = displayDate.hour();
        const weekday = displayDate.day(); // 0 = Sunday, 6 = Saturday
        
        // Check if it's a weekend
        if (weekday === 0 || weekday === 6) {
            throw newDateTimeError(
                ERROR_TYPES.INVALID_RANGE,
                `meeting time falls on weekend: ${this.formatter.toEmailTemplate(date, timezone)}`,
                this.formatter.toRFC3339(date),
                null
            );
        }
        
        // Check if it's outside business hours (8 AM to 6 PM)
        if (hour < 8 || hour >= 18) {
            throw newDateTimeError(
                ERROR_TYPES.INVALID_RANGE,
                `meeting time is outside business hours (8 AM - 6 PM): ${this.formatter.toEmailTemplate(date, timezone)}`,
                this.formatter.toRFC3339(date),
                null
            );
        }
    }
    
    /**
     * Validates an implementation schedule window
     * @param {Date} start - The start Date object
     * @param {Date} end - The end Date object
     * @throws {DateTimeError} - If validation fails
     */
    validateScheduleWindow(start, end) {
        // Basic range validation
        this.validateDateRange(start, end);
        
        const startJs = dayjs(start);
        const endJs = dayjs(end);
        const duration = endJs.diff(startJs); // Day.js diff returns milliseconds
        
        // Check minimum duration (at least 15 minutes)
        const minDuration = 15 * 60 * 1000; // 15 minutes in milliseconds
        if (duration < minDuration) {
            throw newDateTimeError(
                ERROR_TYPES.INVALID_RANGE,
                `schedule window is too short: minimum ${this.formatter.formatDuration(minDuration)}, got ${this.formatter.formatDuration(duration)}`,
                `${this.formatter.toRFC3339(start)} to ${this.formatter.toRFC3339(end)}`,
                null
            );
        }
        
        // Check maximum duration (no more than 24 hours for a single window)
        const maxDuration = 24 * 60 * 60 * 1000; // 24 hours in milliseconds
        if (duration > maxDuration) {
            throw newDateTimeError(
                ERROR_TYPES.INVALID_RANGE,
                `schedule window is too long: maximum ${this.formatter.formatDuration(maxDuration)}, got ${this.formatter.formatDuration(duration)}`,
                `${this.formatter.toRFC3339(start)} to ${this.formatter.toRFC3339(end)}`,
                null
            );
        }
    }
    
    /**
     * Validates that a meeting duration is reasonable
     * @param {number} durationMs - Duration in milliseconds
     * @throws {DateTimeError} - If validation fails
     */
    validateMeetingDuration(durationMs) {
        if (typeof durationMs !== 'number' || durationMs <= 0) {
            throw newDateTimeError(
                ERROR_TYPES.INVALID_RANGE,
                'meeting duration must be a positive number',
                String(durationMs),
                null
            );
        }
        
        // Minimum 15 minutes
        const minDuration = 15 * 60 * 1000; // 15 minutes in milliseconds
        if (durationMs < minDuration) {
            throw newDateTimeError(
                ERROR_TYPES.INVALID_RANGE,
                `meeting duration is too short: minimum 15 minutes, got ${this.formatter.formatDuration(durationMs)}`,
                String(durationMs),
                null
            );
        }
        
        // Maximum 8 hours
        const maxDuration = 8 * 60 * 60 * 1000; // 8 hours in milliseconds
        if (durationMs > maxDuration) {
            throw newDateTimeError(
                ERROR_TYPES.INVALID_RANGE,
                `meeting duration is too long: maximum 8 hours, got ${this.formatter.formatDuration(durationMs)}`,
                String(durationMs),
                null
            );
        }
    }
    
    /**
     * Validates that two times are in compatible timezones
     * @param {Date} date1 - First Date object
     * @param {Date} date2 - Second Date object
     * @throws {DateTimeError} - If validation fails
     */
    validateTimezonePair(date1, date2) {
        // Both times should be valid Date objects
        if (!date1 || !(date1 instanceof Date) || !dayjs(date1).isValid()) {
            throw newDateTimeError(
                ERROR_TYPES.INVALID_FORMAT,
                'first date must be a valid Date object',
                String(date1),
                null
            );
        }
        
        if (!date2 || !(date2 instanceof Date) || !dayjs(date2).isValid()) {
            throw newDateTimeError(
                ERROR_TYPES.INVALID_FORMAT,
                'second date must be a valid Date object',
                String(date2),
                null
            );
        }
        
        // This is mainly a sanity check - times can be in different timezones,
        // but we want to ensure they're both valid
        this.validateDateTime(date1);
        this.validateDateTime(date2);
    }
}