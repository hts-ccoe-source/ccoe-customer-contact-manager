/**
 * Filter Utilities Module - Filtering and sorting functionality
 * Provides debounced filtering, sorting, and search utilities
 */

class FilterManager {
    constructor(options = {}) {
        this.options = {
            debounceDelay: options.debounceDelay || 300,
            onFilterChange: options.onFilterChange || null,
            onSortChange: options.onSortChange || null
        };

        this.filters = {};
        this.sortConfig = {
            field: null,
            direction: 'asc'
        };
        this.debounceTimers = {};
    }

    /**
     * Set a filter value with debouncing
     */
    setFilter(key, value, immediate = false) {
        if (immediate) {
            this.applyFilter(key, value);
        } else {
            // Clear existing timer
            if (this.debounceTimers[key]) {
                clearTimeout(this.debounceTimers[key]);
            }

            // Set new timer
            this.debounceTimers[key] = setTimeout(() => {
                this.applyFilter(key, value);
            }, this.options.debounceDelay);
        }
    }

    /**
     * Apply filter immediately
     */
    applyFilter(key, value) {
        this.filters[key] = value;
        
        if (this.options.onFilterChange) {
            this.options.onFilterChange(this.filters);
        }
    }

    /**
     * Remove a filter
     */
    removeFilter(key) {
        delete this.filters[key];
        
        if (this.options.onFilterChange) {
            this.options.onFilterChange(this.filters);
        }
    }

    /**
     * Clear all filters
     */
    clearFilters() {
        this.filters = {};
        
        if (this.options.onFilterChange) {
            this.options.onFilterChange(this.filters);
        }
    }

    /**
     * Get current filters
     */
    getFilters() {
        return { ...this.filters };
    }

    /**
     * Set sort configuration
     */
    setSort(field, direction = 'asc') {
        this.sortConfig = { field, direction };
        
        if (this.options.onSortChange) {
            this.options.onSortChange(this.sortConfig);
        }
    }

    /**
     * Toggle sort direction for a field
     */
    toggleSort(field) {
        if (this.sortConfig.field === field) {
            // Toggle direction
            this.sortConfig.direction = this.sortConfig.direction === 'asc' ? 'desc' : 'asc';
        } else {
            // New field, default to ascending
            this.sortConfig.field = field;
            this.sortConfig.direction = 'asc';
        }
        
        if (this.options.onSortChange) {
            this.options.onSortChange(this.sortConfig);
        }
    }

    /**
     * Get current sort configuration
     */
    getSort() {
        return { ...this.sortConfig };
    }
}

/**
 * Filter by status
 */
function filterByStatus(items, status) {
    if (!status || status === 'all') {
        return items;
    }
    
    return items.filter(item => item.status === status);
}

/**
 * Filter by customer
 */
function filterByCustomer(items, customerCode) {
    if (!customerCode || customerCode === 'all') {
        return items;
    }
    
    return items.filter(item => {
        if (Array.isArray(item.customers)) {
            return item.customers.includes(customerCode);
        }
        return item.customer === customerCode;
    });
}

/**
 * Filter by type (for announcements)
 */
function filterByType(items, type) {
    if (!type || type === 'all') {
        return items;
    }
    
    return items.filter(item => {
        if (type.endsWith('*')) {
            // Wildcard matching
            const prefix = type.slice(0, -1);
            return item.object_type && item.object_type.startsWith(prefix);
        }
        return item.object_type === type;
    });
}

/**
 * Filter by date range
 */
function filterByDateRange(items, dateField, startDate, endDate) {
    if (!startDate && !endDate) {
        return items;
    }
    
    return items.filter(item => {
        const itemDate = new Date(item[dateField]);
        
        if (startDate && itemDate < new Date(startDate)) {
            return false;
        }
        
        if (endDate && itemDate > new Date(endDate)) {
            return false;
        }
        
        return true;
    });
}

/**
 * Search items by text query
 */
function searchItems(items, query, searchFields = ['title', 'changeTitle', 'description']) {
    if (!query || query.trim() === '') {
        return items;
    }
    
    const lowerQuery = query.toLowerCase().trim();
    
    return items.filter(item => {
        return searchFields.some(field => {
            const value = item[field];
            if (typeof value === 'string') {
                return value.toLowerCase().includes(lowerQuery);
            }
            return false;
        });
    });
}

/**
 * Sort items by field
 */
function sortItems(items, field, direction = 'asc') {
    if (!field) {
        return items;
    }
    
    const sorted = [...items].sort((a, b) => {
        let aVal = a[field];
        let bVal = b[field];
        
        // Handle nested fields (e.g., 'schedule.start')
        if (field.includes('.')) {
            const parts = field.split('.');
            aVal = parts.reduce((obj, key) => obj?.[key], a);
            bVal = parts.reduce((obj, key) => obj?.[key], b);
        }
        
        // Handle null/undefined
        if (aVal == null && bVal == null) return 0;
        if (aVal == null) return 1;
        if (bVal == null) return -1;
        
        // Handle dates
        if (aVal instanceof Date || bVal instanceof Date || 
            (typeof aVal === 'string' && !isNaN(Date.parse(aVal)))) {
            aVal = new Date(aVal);
            bVal = new Date(bVal);
            return direction === 'asc' ? aVal - bVal : bVal - aVal;
        }
        
        // Handle numbers
        if (typeof aVal === 'number' && typeof bVal === 'number') {
            return direction === 'asc' ? aVal - bVal : bVal - aVal;
        }
        
        // Handle strings
        const aStr = String(aVal).toLowerCase();
        const bStr = String(bVal).toLowerCase();
        
        if (direction === 'asc') {
            return aStr.localeCompare(bStr);
        } else {
            return bStr.localeCompare(aStr);
        }
    });
    
    return sorted;
}

/**
 * Apply multiple filters to items
 */
function applyFilters(items, filters) {
    let filtered = [...items];
    
    // Apply status filter
    if (filters.status) {
        filtered = filterByStatus(filtered, filters.status);
    }
    
    // Apply customer filter
    if (filters.customer) {
        filtered = filterByCustomer(filtered, filters.customer);
    }
    
    // Apply type filter
    if (filters.type) {
        filtered = filterByType(filtered, filters.type);
    }
    
    // Apply date range filter
    if (filters.dateField && (filters.startDate || filters.endDate)) {
        filtered = filterByDateRange(
            filtered, 
            filters.dateField, 
            filters.startDate, 
            filters.endDate
        );
    }
    
    // Apply search query
    if (filters.query) {
        filtered = searchItems(filtered, filters.query, filters.searchFields);
    }
    
    // Apply sorting
    if (filters.sortField) {
        filtered = sortItems(filtered, filters.sortField, filters.sortDirection);
    }
    
    return filtered;
}

/**
 * Debounce function utility
 */
function debounce(func, delay = 300) {
    let timeoutId;
    
    return function debounced(...args) {
        clearTimeout(timeoutId);
        
        timeoutId = setTimeout(() => {
            func.apply(this, args);
        }, delay);
    };
}

/**
 * Create a debounced search handler
 */
function createDebouncedSearch(callback, delay = 300) {
    return debounce((query) => {
        callback(query);
    }, delay);
}

/**
 * Get unique values from items for a field (useful for filter dropdowns)
 */
function getUniqueValues(items, field) {
    const values = new Set();
    
    items.forEach(item => {
        const value = item[field];
        
        if (Array.isArray(value)) {
            value.forEach(v => values.add(v));
        } else if (value != null) {
            values.add(value);
        }
    });
    
    return Array.from(values).sort();
}

/**
 * Group items by field value
 */
function groupBy(items, field) {
    const groups = {};
    
    items.forEach(item => {
        const value = item[field];
        
        if (Array.isArray(value)) {
            // Handle array fields (like customers)
            value.forEach(v => {
                if (!groups[v]) {
                    groups[v] = [];
                }
                groups[v].push(item);
            });
        } else {
            const key = value || 'unknown';
            if (!groups[key]) {
                groups[key] = [];
            }
            groups[key].push(item);
        }
    });
    
    return groups;
}

/**
 * Paginate items
 */
function paginate(items, page = 1, pageSize = 20) {
    const startIndex = (page - 1) * pageSize;
    const endIndex = startIndex + pageSize;
    
    return {
        items: items.slice(startIndex, endIndex),
        page,
        pageSize,
        totalItems: items.length,
        totalPages: Math.ceil(items.length / pageSize),
        hasNext: endIndex < items.length,
        hasPrev: page > 1
    };
}

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
    module.exports = {
        FilterManager,
        filterByStatus,
        filterByCustomer,
        filterByType,
        filterByDateRange,
        searchItems,
        sortItems,
        applyFilters,
        debounce,
        createDebouncedSearch,
        getUniqueValues,
        groupBy,
        paginate
    };
}
