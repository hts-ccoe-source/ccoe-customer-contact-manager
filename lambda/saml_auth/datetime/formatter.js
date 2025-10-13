/**
 * Formatter handles formatting of Date values for different output contexts
 * Equivalent to internal/datetime/formatter.go
 */

import dayjs from 'dayjs';
import utc from 'dayjs/plugin/utc.js';
import timezone from 'dayjs/plugin/timezone.js';
import { DateTimeConfig, defaultConfig, DateTimeError, newDateTimeError, ERROR_TYPES } from './types.js';

// Configure Day.js plugins
dayjs.extend(utc);
dayjs.extend(timezone);

export class Formatter {
    constructor(config) {
        this.config = config || defaultConfig();
    }
    
    /**
     * Formats a Date to the canonical RFC3339 format with timezone
     * @param {Date} date - The Date object to format
     * @returns {string} - RFC3339 formatted string
     */
    toRFC3339(date) {
        if (!date || !(date instanceof Date)) {
            throw new Error('Invalid date parameter');
        }
        
        const dayjsDate = dayjs(date);
        
        // Check if original timezone was specified
        if (date._originalTimezone === 'UTC') {
            // Format as UTC
            return dayjsDate.utc().format('YYYY-MM-DDTHH:mm:ss[Z]');
        } else if (date._originalTimezone && date._originalTimezone !== this.config.defaultTimezone) {
            // Format in the original timezone
            try {
                return dayjsDate.tz(date._originalTimezone).format('YYYY-MM-DDTHH:mm:ssZ');
            } catch (err) {
                // Fallback to default formatting
            }
        }
        
        // Default RFC3339 formatting
        return dayjsDate.format('YYYY-MM-DDTHH:mm:ssZ');
    }
    
    /**
     * Formats a Date for Microsoft Graph API compatibility
     * Graph API expects UTC time in a specific format without timezone offset
     * @param {Date} date - The Date object to format
     * @returns {string} - Microsoft Graph formatted string
     */
    toMicrosoftGraph(date) {
        if (!date || !(date instanceof Date)) {
            throw new Error('Invalid date parameter');
        }
        
        // Convert to UTC and format without timezone info
        const utcDate = dayjs(date).utc();
        return utcDate.format('YYYY-MM-DDTHH:mm:ss.0000000');
    }
    
    /**
     * Formats a Date for human-readable display
     * @param {Date} date - The Date object to format
     * @param {string} timezone - Optional timezone for display
     * @returns {string} - Human-readable formatted string
     */
    toHumanReadable(date, timezone) {
        if (!date || !(date instanceof Date)) {
            throw new Error('Invalid date parameter');
        }
        
        // Use the specified timezone or the original timezone stored on the date
        const targetTimezone = timezone || date._originalTimezone || this.config.defaultTimezone;
        
        const dayjsDate = dayjs(date);
        let displayDate;
        
        try {
            // Convert to target timezone for display
            if (targetTimezone === 'UTC') {
                displayDate = dayjsDate.utc();
                const timeStr = displayDate.format('h:mm A') + ' UTC';
                const dateStr = displayDate.format('MMMM D, YYYY');
                return `${dateStr} at ${timeStr}`;
            } else {
                displayDate = dayjsDate.tz(targetTimezone);
                
                // Get timezone abbreviation using Intl API
                const formatter = new Intl.DateTimeFormat('en-US', {
                    timeZone: targetTimezone,
                    timeZoneName: 'short'
                });
                
                const parts = formatter.formatToParts(date);
                const timeZonePart = parts.find(part => part.type === 'timeZoneName');
                const tzAbbr = timeZonePart ? timeZonePart.value : this._getTimezoneAbbreviation(targetTimezone);
                
                const timeStr = displayDate.format('h:mm A') + ' ' + tzAbbr;
                const dateStr = displayDate.format('MMMM D, YYYY');
                return `${dateStr} at ${timeStr}`;
            }
        } catch (err) {
            // Fallback to basic formatting
            const timeZoneName = this._getTimezoneAbbreviation(targetTimezone);
            const timeStr = dayjsDate.format('h:mm A') + ' ' + timeZoneName;
            const dateStr = dayjsDate.format('MMMM D, YYYY');
            return `${dateStr} at ${timeStr}`;
        }
    }
    
    /**
     * Formats a Date for iCalendar (ICS) files
     * ICS format requires UTC time in compact format
     * @param {Date} date - The Date object to format
     * @returns {string} - ICS formatted string
     */
    toICS(date) {
        if (!date || !(date instanceof Date)) {
            throw new Error('Invalid date parameter');
        }
        
        // Convert to UTC and format in compact format
        const utcDate = dayjs(date).utc();
        return utcDate.format('YYYYMMDDTHHmmss[Z]');
    }
    
    /**
     * Formats a Date for structured logging
     * Uses UTC with millisecond precision
     * @param {Date} date - The Date object to format
     * @returns {string} - Log formatted string
     */
    toLogFormat(date) {
        if (!date || !(date instanceof Date)) {
            throw new Error('Invalid date parameter');
        }
        
        // Convert to UTC and format with milliseconds
        const utcDate = dayjs(date).utc();
        return utcDate.format('YYYY-MM-DDTHH:mm:ss.SSS[Z]');
    }
    
    /**
     * Formats a Date to date-only string
     * @param {Date} date - The Date object to format
     * @returns {string} - Date-only string
     */
    toDateOnly(date) {
        if (!date || !(date instanceof Date)) {
            throw new Error('Invalid date parameter');
        }
        return dayjs(date).format('YYYY-MM-DD');
    }
    
    /**
     * Formats a Date to time-only string in 24-hour format
     * @param {Date} date - The Date object to format
     * @returns {string} - Time-only string
     */
    toTimeOnly(date) {
        if (!date || !(date instanceof Date)) {
            throw new Error('Invalid date parameter');
        }
        return dayjs(date).format('HH:mm:ss');
    }
    
    /**
     * Formats a Date to time-only string in 12-hour format
     * @param {Date} date - The Date object to format
     * @returns {string} - Time-only string in 12-hour format
     */
    toTimeOnly12Hour(date) {
        if (!date || !(date instanceof Date)) {
            throw new Error('Invalid date parameter');
        }
        return dayjs(date).format('h:mm:ss A');
    }
    
    /**
     * Formats a Date for email templates with timezone context
     * @param {Date} date - The Date object to format
     * @param {string} timezone - Optional timezone for display
     * @returns {string} - Email template formatted string
     */
    toEmailTemplate(date, timezone) {
        if (!date || !(date instanceof Date)) {
            throw new Error('Invalid date parameter');
        }
        
        // Use specified timezone or default
        const targetTimezone = timezone || this.config.defaultTimezone;
        
        try {
            // Format as "Monday, January 15, 2025 at 10:00 AM EST"
            const dayjsDate = dayjs(date).tz(targetTimezone);
            const tzAbbr = this._getTimezoneAbbreviation(targetTimezone);
            return dayjsDate.format('dddd, MMMM D, YYYY [at] h:mm A') + ' ' + tzAbbr;
        } catch (err) {
            // Fallback to basic format if timezone conversion fails
            return dayjs(date).format('dddd, MMMM D, YYYY [at] h:mm A');
        }
    }
    
    /**
     * Formats start and end times for schedule display
     * @param {Date} start - The start Date object
     * @param {Date} end - The end Date object
     * @param {string} timezone - Optional timezone for display
     * @returns {string} - Schedule window formatted string
     */
    toScheduleWindow(start, end, timezone) {
        if (!start || !(start instanceof Date) || !end || !(end instanceof Date)) {
            throw new Error('Invalid date parameters');
        }
        
        const startFormatted = this.toHumanReadable(start, timezone);
        const endFormatted = this.toHumanReadable(end, timezone);
        
        // Check if same date
        const startDate = timezone ? dayjs(start).tz(timezone) : dayjs(start);
        const endDate = timezone ? dayjs(end).tz(timezone) : dayjs(end);
        
        const sameDay = startDate.format('YYYY-MM-DD') === endDate.format('YYYY-MM-DD');
        
        if (sameDay) {
            // If same date, show date once: "January 15, 2025 from 10:00 AM to 2:00 PM EST"
            const dateStr = startDate.format('MMMM D, YYYY');
            const tzAbbr = timezone ? this._getTimezoneAbbreviation(timezone) : '';
            const startTimeStr = startDate.format('h:mm A') + (tzAbbr ? ' ' + tzAbbr : '');
            const endTimeStr = endDate.format('h:mm A') + (tzAbbr ? ' ' + tzAbbr : '');
            
            return `${dateStr} from ${startTimeStr} to ${endTimeStr}`;
        }
        
        // Different dates: show full format for both
        return `${startFormatted} to ${endFormatted}`;
    }
    
    /**
     * Converts a Date to a specific timezone and returns the formatted string
     * @param {Date} date - The Date object to convert
     * @param {string} timezone - The target timezone
     * @returns {string} - RFC3339 formatted string in target timezone
     * @throws {DateTimeError} - If timezone is invalid
     */
    toTimezone(date, timezone) {
        if (!date || !(date instanceof Date)) {
            throw new Error('Invalid date parameter');
        }
        
        try {
            const converted = dayjs(date).tz(timezone);
            const result = converted.toDate();
            result._originalTimezone = timezone;
            return this.toRFC3339(result);
        } catch (err) {
            throw newDateTimeError(
                ERROR_TYPES.INVALID_TIMEZONE,
                `invalid timezone: ${timezone}`,
                timezone,
                err
            );
        }
    }
    
    /**
     * Formats a duration in a human-readable way
     * @param {number} milliseconds - Duration in milliseconds
     * @returns {string} - Human-readable duration string
     */
    formatDuration(milliseconds) {
        if (typeof milliseconds !== 'number' || milliseconds < 0) {
            throw new Error('Invalid duration parameter');
        }
        
        const seconds = Math.floor(milliseconds / 1000);
        const minutes = Math.floor(seconds / 60);
        const hours = Math.floor(minutes / 60);
        
        if (seconds < 60) {
            return seconds === 1 ? '1 second' : `${seconds} seconds`;
        }
        
        if (minutes < 60) {
            return minutes === 1 ? '1 minute' : `${minutes} minutes`;
        }
        
        const remainingMinutes = minutes % 60;
        
        if (remainingMinutes === 0) {
            return hours === 1 ? '1 hour' : `${hours} hours`;
        }
        
        if (hours === 1) {
            return remainingMinutes === 1 ? '1 hour 1 minute' : `1 hour ${remainingMinutes} minutes`;
        }
        
        if (remainingMinutes === 1) {
            return `${hours} hours 1 minute`;
        }
        
        return `${hours} hours ${remainingMinutes} minutes`;
    }
    
    /**
     * Converts a Date back to separate date/time/timezone fields
     * for backward compatibility with existing data structures
     * @param {Date} date - The Date object to convert
     * @returns {Object} - Object with date, time, and timezone fields
     */
    toLegacyFields(date) {
        if (!date || !(date instanceof Date)) {
            throw new Error('Invalid date parameter');
        }
        
        const dayjsDate = dayjs(date);
        return {
            date: dayjsDate.format('YYYY-MM-DD'),
            time: dayjsDate.format('HH:mm:ss'),
            timezone: Intl.DateTimeFormat().resolvedOptions().timeZone
        };
    }
    
    /**
     * Get timezone abbreviation for a timezone
     * @private
     * @param {string} timezone - The timezone name
     * @returns {string} - Timezone abbreviation
     */
    _getTimezoneAbbreviation(timezone) {
        if (timezone === 'UTC') {
            return 'UTC';
        }
        
        try {
            // Try to get abbreviation for the timezone using current date
            const now = new Date();
            const formatter = new Intl.DateTimeFormat('en-US', {
                timeZoneName: 'short',
                timeZone: timezone
            });
            
            const parts = formatter.formatToParts(now);
            const timeZonePart = parts.find(part => part.type === 'timeZoneName');
            
            if (timeZonePart) {
                return timeZonePart.value;
            }
        } catch (err) {
            // Fallback
        }
        
        // Fallback to offset format using Day.js
        try {
            const now = dayjs().tz(timezone);
            const offset = now.utcOffset();
            
            if (offset === 0) {
                return 'UTC';
            }
            
            // Convert minutes to hours and minutes for offset display
            const absOffset = Math.abs(offset);
            const hours = Math.floor(absOffset / 60);
            const minutes = absOffset % 60;
            const sign = offset >= 0 ? '+' : '-';
            
            // Format as +/-HHMM (like Go's -0500 format)
            return `${sign}${hours.toString().padStart(2, '0')}${minutes.toString().padStart(2, '0')}`;
        } catch (err) {
            return 'UTC';
        }
    }

    /**
     * Converts a Date to UTC and returns RFC3339 formatted string
     * @param {Date} date - The Date object to convert
     * @returns {string} - RFC3339 formatted string in UTC
     */
    toUTC(date) {
        if (!date || !(date instanceof Date)) {
            throw new Error('Invalid date parameter');
        }
        
        return dayjs(date).utc().format('YYYY-MM-DDTHH:mm:ss[Z]');
    }
}