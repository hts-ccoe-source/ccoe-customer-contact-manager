# Filter Button Live Count Recalculation

## Overview
Implemented live recalculation of status filter button counts based on date filter changes on both the `announcements.html` and `my-changes.html` pages.

## Problem
The status filter buttons (All, Drafts, Submitted, Approved, Completed, Cancelled) displayed static counts that didn't update when users changed the date filter. This made it unclear how many items of each status existed within the selected date range.

## Solution

### My Changes Page (`html/my-changes.html`)

**Added `applyDateFilterToChanges()` helper method:**
- Extracts date filtering logic into a reusable method
- Reads the current date filter value from the dropdown
- Returns filtered changes based on the selected date range (today, week, 14 days, month, quarter)
- Uses `created_date`, `submittedDate`, or `modifiedDate` fields for filtering

**Updated `updateCounts()` method:**
- Now calls `applyDateFilterToChanges()` before counting
- Counts are calculated from date-filtered changes instead of all changes
- All status counts (draft, submitted, approved, completed, cancelled) reflect the current date filter

**Updated `applyFilters()` method:**
- Added call to `this.updateCounts()` after applying filters
- Ensures counts update whenever filters change (status, date, or search)

### Announcements Page (`html/assets/js/announcements-page.js`)

**Already had `applyDateFilter()` helper method:**
- Existing method that filters announcements by date range
- Uses `this.filters.dateRange` property

**Updated `updateStatusCounts()` method:**
- Already called `applyDateFilter()` before counting
- Added console.log for debugging
- Counts reflect the current date filter selection

**Event listener already configured:**
- Date filter change event properly updates `this.filters.dateRange`
- Automatically calls `applyFilters()` which calls `updateStatusCounts()`

## User Experience

### Before
- Status button counts showed total items across all time
- Changing date filter updated the displayed items but not the counts
- Users couldn't see how many items of each status existed in the selected timeframe

### After
- Status button counts dynamically update when date filter changes
- Counts accurately reflect items within the selected date range
- Users can see at a glance how many drafts, submitted items, etc. exist in the current timeframe
- Example: Selecting "Past 14 Days" shows only items from the last 14 days in both the list AND the counts

## Technical Details

### Date Filter Options
Both pages support the same date ranges:
- **Today**: Items from today only
- **This Week**: Items from the last 7 days
- **Past 14 Days**: Items from the last 14 days (default)
- **This Month**: Items from the first day of the current month
- **This Quarter**: Items from the first day of the current quarter

### Count Update Triggers
Counts are recalculated when:
1. Date filter dropdown changes
2. Status filter button is clicked
3. Search filter is modified
4. Page initially loads
5. Data is refreshed

### Performance
- Filtering is performed in-memory on already-loaded data
- No additional API calls required
- Counts update instantly when filters change

## Files Modified
1. `html/my-changes.html` - Added date filtering to count calculation
2. `html/assets/js/announcements-page.js` - Enhanced existing date filtering for counts

## Testing Recommendations
1. Load my-changes.html and verify counts update when changing date filter
2. Load announcements.html and verify counts update when changing date filter
3. Test with different date ranges (today, week, 14 days, month, quarter)
4. Verify counts match the number of items displayed in each status
5. Test combination of date filter + status filter + search filter
6. Verify counts reset correctly when clearing filters
