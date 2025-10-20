/**
 * S3 Client Module - S3 data operations with retry logic and caching
 * Provides methods for fetching objects from S3 with exponential backoff
 */

class S3Client {
    constructor(options = {}) {
        this.baseUrl = options.baseUrl || window.location.origin;
        this.cache = new Map(); // Store data for 304 responses
        this.etagCache = new Map(); // Store ETags for conditional requests
        this.maxRetries = options.maxRetries || 3;
        this.retryDelay = options.retryDelay || 1000; // 1 second initial delay
        this.localStorageKey = 's3-client-cache';
        this.localStorageETagKey = 's3-client-etags';
        
        // Load cache from localStorage on init
        this.loadFromLocalStorage();
    }

    /**
     * Fetch objects from S3 with retry logic, exponential backoff, and ETag-based caching
     */
    async fetchObjects(path, options = {}) {
        const cacheKey = this.getCacheKey(path, options);
        
        // Get stored ETag for conditional request (unless skipCache is true)
        const storedETag = options.skipCache ? null : this.etagCache.get(cacheKey);

        // Fetch with retry logic
        let lastError = null;
        for (let attempt = 0; attempt < this.maxRetries; attempt++) {
            try {
                const headers = {
                    'Cache-Control': 'no-cache',
                    ...options.headers
                };

                // Add If-None-Match header if we have an ETag
                if (storedETag && !options.skipCache) {
                    headers['If-None-Match'] = storedETag;
                    console.log(`🔄 Fetching with ETag (attempt ${attempt + 1}/${this.maxRetries}): ${path}`);
                } else {
                    console.log(`🔄 Fetching from S3 (attempt ${attempt + 1}/${this.maxRetries}): ${path}`);
                }
                
                const response = await fetch(`${this.baseUrl}${path}`, {
                    method: 'GET',
                    credentials: 'same-origin',
                    headers
                });

                // Handle 304 Not Modified - use cached data
                if (response.status === 304) {
                    const cachedData = this.cache.get(cacheKey);
                    if (cachedData) {
                        console.log(`✅ 304 Not Modified - using cached data for: ${path}`);
                        return cachedData;
                    }
                    // If we don't have cached data, fall through to retry without ETag
                    console.warn(`⚠️  Got 304 but no cached data, retrying without ETag`);
                    this.etagCache.delete(cacheKey);
                    continue;
                }

                if (!response.ok) {
                    throw new Error(`HTTP ${response.status}: ${response.statusText}`);
                }

                const data = await response.json();
                
                // Get and store ETag from response
                const newETag = response.headers.get('ETag');
                if (newETag) {
                    this.etagCache.set(cacheKey, newETag);
                    console.log(`💾 Stored ETag for: ${path}`);
                }
                
                // Store in cache
                this.setCache(cacheKey, data, newETag);
                
                console.log(`✅ Successfully fetched: ${path}`);
                return data;

            } catch (error) {
                lastError = error;
                console.warn(`⚠️  Fetch attempt ${attempt + 1} failed:`, error.message);

                // Don't retry on certain errors
                if (error.message.includes('401') || error.message.includes('403')) {
                    throw new Error('Authentication required. Please refresh the page and log in again.');
                }

                // Wait before retrying (exponential backoff)
                if (attempt < this.maxRetries - 1) {
                    const delay = this.retryDelay * Math.pow(2, attempt);
                    console.log(`⏳ Waiting ${delay}ms before retry...`);
                    await this.sleep(delay);
                }
            }
        }

        // All retries failed
        throw new Error(`Failed to fetch after ${this.maxRetries} attempts: ${lastError.message}`);
    }

    /**
     * Fetch multiple objects in parallel
     */
    async fetchMultiple(paths, options = {}) {
        const promises = paths.map(path => 
            this.fetchObjects(path, options).catch(error => ({
                error: true,
                path,
                message: error.message
            }))
        );

        return await Promise.all(promises);
    }

    /**
     * Fetch all changes for a customer
     * Uses the /changes endpoint with filtering on the client side
     * since there's no customer-specific endpoint
     */
    async fetchCustomerChanges(customerCode, options = {}) {
        // Fetch all changes and filter client-side
        const allChanges = await this.fetchAllChanges(options);
        
        // Filter to only changes for this customer
        return allChanges.filter(change => {
            if (Array.isArray(change.customers)) {
                return change.customers.includes(customerCode);
            } else if (change.customer) {
                return change.customer === customerCode;
            }
            return false;
        });
    }

    /**
     * Fetch all changes (admin view)
     * Uses the /changes endpoint which returns all changes
     */
    async fetchAllChanges(options = {}) {
        const path = '/changes';
        return await this.fetchObjects(path, options);
    }

    /**
     * Fetch a specific change by ID
     */
    async fetchChange(changeId, options = {}) {
        const path = `/changes/${changeId}`;
        return await this.fetchObjects(path, options);
    }

    /**
     * Fetch announcements
     * Uses the /announcements endpoint which filters by ID prefix (CIC-, FIN-, INN-)
     */
    async fetchAnnouncements(options = {}) {
        const path = '/announcements';
        try {
            return await this.fetchObjects(path, options);
        } catch (error) {
            console.warn('Error fetching announcements:', error);
            return [];
        }
    }

    /**
     * Fetch customer-specific announcements
     * Uses the /announcements/customer/{customerCode} endpoint
     */
    async fetchCustomerAnnouncements(customerCode, options = {}) {
        const path = `/announcements/customer/${customerCode}`;
        try {
            return await this.fetchObjects(path, options);
        } catch (error) {
            console.warn(`Error fetching customer announcements for ${customerCode}:`, error);
            return [];
        }
    }

    /**
     * Update a change object in S3
     */
    async updateChange(changeId, changeData, options = {}) {
        const path = `/changes/${changeId}`;
        
        let lastError = null;
        for (let attempt = 0; attempt < this.maxRetries; attempt++) {
            try {
                console.log(`🔄 Updating change (attempt ${attempt + 1}/${this.maxRetries}): ${changeId}`);
                
                const response = await fetch(`${this.baseUrl}${path}`, {
                    method: 'PUT',
                    credentials: 'same-origin',
                    headers: {
                        'Content-Type': 'application/json',
                        ...options.headers
                    },
                    body: JSON.stringify(changeData)
                });

                if (!response.ok) {
                    throw new Error(`HTTP ${response.status}: ${response.statusText}`);
                }

                const data = await response.json();
                
                // Invalidate cache for this change
                this.clearCache(`/api/changes/${changeId}`);
                this.clearCache('/api/changes/all');
                
                console.log(`✅ Successfully updated change: ${changeId}`);
                return data;

            } catch (error) {
                lastError = error;
                console.warn(`⚠️  Update attempt ${attempt + 1} failed:`, error.message);

                // Don't retry on certain errors
                if (error.message.includes('401') || error.message.includes('403')) {
                    throw new Error('Authentication required. Please refresh the page and log in again.');
                }

                // Wait before retrying (exponential backoff)
                if (attempt < this.maxRetries - 1) {
                    const delay = this.retryDelay * Math.pow(2, attempt);
                    console.log(`⏳ Waiting ${delay}ms before retry...`);
                    await this.sleep(delay);
                }
            }
        }

        // All retries failed
        throw new Error(`Failed to update change after ${this.maxRetries} attempts: ${lastError.message}`);
    }

    /**
     * Get cache key for request
     */
    getCacheKey(path, options) {
        return `${path}:${JSON.stringify(options)}`;
    }

    /**
     * Store data in cache (used for 304 responses)
     */
    setCache(key, data, etag = null) {
        this.cache.set(key, data);
        
        if (etag) {
            this.etagCache.set(key, etag);
        }
        
        // Persist to localStorage
        this.saveToLocalStorage();
    }

    /**
     * Clear cache and ETags
     */
    clearCache(pattern = null) {
        if (pattern) {
            // Clear specific pattern
            for (const key of this.cache.keys()) {
                if (key.includes(pattern)) {
                    this.cache.delete(key);
                    this.etagCache.delete(key);
                }
            }
        } else {
            // Clear all cache
            this.cache.clear();
            this.etagCache.clear();
        }
        
        // Persist to localStorage
        this.saveToLocalStorage();
        console.log('🗑️  Cache cleared');
    }

    /**
     * Sleep utility for retry delays
     */
    sleep(ms) {
        return new Promise(resolve => setTimeout(resolve, ms));
    }

    /**
     * Filter objects by object_type
     */
    filterByObjectType(objects, objectType) {
        if (!Array.isArray(objects)) return [];
        
        return objects.filter(obj => {
            if (objectType.endsWith('*')) {
                // Wildcard matching (e.g., "announcement_*")
                const prefix = objectType.slice(0, -1);
                return obj.object_type && obj.object_type.startsWith(prefix);
            }
            
            // For 'change' type, include objects without object_type field
            // This handles legacy changes created before object_type was added
            // All new changes will have object_type: "change" set by createMetadata()
            if (objectType === 'change') {
                return !obj.object_type || obj.object_type === 'change';
            }
            
            return obj.object_type === objectType;
        });
    }

    /**
     * Sort objects by date field
     */
    sortByDate(objects, dateField = 'createdAt', descending = true) {
        if (!Array.isArray(objects)) return [];
        
        return objects.sort((a, b) => {
            const dateA = new Date(a[dateField] || 0);
            const dateB = new Date(b[dateField] || 0);
            return descending ? dateB - dateA : dateA - dateB;
        });
    }

    /**
     * Group objects by customer
     */
    groupByCustomer(objects) {
        if (!Array.isArray(objects)) return {};
        
        const grouped = {};
        objects.forEach(obj => {
            if (obj.customers && Array.isArray(obj.customers)) {
                obj.customers.forEach(customer => {
                    if (!grouped[customer]) {
                        grouped[customer] = [];
                    }
                    grouped[customer].push(obj);
                });
            }
        });
        
        return grouped;
    }

    /**
     * Get cache statistics
     */
    getCacheStats() {
        let withETag = 0;

        for (const key of this.cache.keys()) {
            if (this.etagCache.has(key)) {
                withETag++;
            }
        }

        return {
            total: this.cache.size,
            withETag,
            etagCacheSize: this.etagCache.size,
            persistedToLocalStorage: this.isLocalStorageAvailable()
        };
    }

    /**
     * Check if localStorage is available
     */
    isLocalStorageAvailable() {
        try {
            const test = '__localStorage_test__';
            localStorage.setItem(test, test);
            localStorage.removeItem(test);
            return true;
        } catch (e) {
            return false;
        }
    }

    /**
     * Save cache and ETags to localStorage
     */
    saveToLocalStorage() {
        if (!this.isLocalStorageAvailable()) return;

        try {
            // Convert Maps to objects for JSON serialization
            const cacheObj = {};
            for (const [key, value] of this.cache.entries()) {
                cacheObj[key] = value;
            }

            const etagObj = {};
            for (const [key, value] of this.etagCache.entries()) {
                etagObj[key] = value;
            }

            localStorage.setItem(this.localStorageKey, JSON.stringify(cacheObj));
            localStorage.setItem(this.localStorageETagKey, JSON.stringify(etagObj));
        } catch (error) {
            console.warn('Failed to save cache to localStorage:', error);
            // If localStorage is full, clear it and try again
            if (error.name === 'QuotaExceededError') {
                this.clearLocalStorage();
            }
        }
    }

    /**
     * Load cache and ETags from localStorage
     */
    loadFromLocalStorage() {
        if (!this.isLocalStorageAvailable()) return;

        try {
            const cacheStr = localStorage.getItem(this.localStorageKey);
            const etagStr = localStorage.getItem(this.localStorageETagKey);

            if (cacheStr) {
                const cacheObj = JSON.parse(cacheStr);
                let loadedCount = 0;

                // Load all cached data
                for (const [key, value] of Object.entries(cacheObj)) {
                    this.cache.set(key, value);
                    loadedCount++;
                }

                console.log(`📦 Loaded ${loadedCount} cached items from localStorage`);
            }

            if (etagStr) {
                const etagObj = JSON.parse(etagStr);
                for (const [key, value] of Object.entries(etagObj)) {
                    this.etagCache.set(key, value);
                }
            }
        } catch (error) {
            console.warn('Failed to load cache from localStorage:', error);
            // Clear corrupted data
            this.clearLocalStorage();
        }
    }

    /**
     * Clear localStorage cache
     */
    clearLocalStorage() {
        if (!this.isLocalStorageAvailable()) return;

        try {
            localStorage.removeItem(this.localStorageKey);
            localStorage.removeItem(this.localStorageETagKey);
            console.log('🗑️  localStorage cache cleared');
        } catch (error) {
            console.warn('Failed to clear localStorage:', error);
        }
    }

    /**
     * Update an announcement object in S3
     * Announcements are stored per customer, so we need the customer code
     */
    async updateAnnouncement(announcementId, announcementData, customerCode, options = {}) {
        const path = `/customers/${customerCode}/announcements/${announcementId}`;
        
        let lastError = null;
        for (let attempt = 0; attempt < this.maxRetries; attempt++) {
            try {
                console.log(`🔄 Updating announcement (attempt ${attempt + 1}/${this.maxRetries}): ${announcementId} for customer ${customerCode}`);
                
                const response = await fetch(`${this.baseUrl}${path}`, {
                    method: 'PUT',
                    credentials: 'same-origin',
                    headers: {
                        'Content-Type': 'application/json',
                        ...options.headers
                    },
                    body: JSON.stringify(announcementData)
                });

                if (!response.ok) {
                    throw new Error(`HTTP ${response.status}: ${response.statusText}`);
                }

                const data = await response.json();
                
                // Invalidate cache for this announcement
                this.clearCache(`/api/customers/${customerCode}/announcements/${announcementId}`);
                this.clearCache('/api/changes/all'); // Also clear the all changes cache since it includes announcements
                
                console.log(`✅ Successfully updated announcement: ${announcementId} for customer ${customerCode}`);
                return data;

            } catch (error) {
                lastError = error;
                console.warn(`⚠️  Update attempt ${attempt + 1} failed:`, error.message);

                // Don't retry on certain errors
                if (error.message.includes('401') || error.message.includes('403')) {
                    throw new Error('Authentication required. Please refresh the page and log in again.');
                }

                // Wait before retrying (exponential backoff)
                if (attempt < this.maxRetries - 1) {
                    const delay = this.retryDelay * Math.pow(2, attempt);
                    console.log(`⏳ Waiting ${delay}ms before retry...`);
                    await this.sleep(delay);
                }
            }
        }

        // All retries failed
        throw new Error(`Failed to update announcement after ${this.maxRetries} attempts: ${lastError.message}`);
    }
}

// Create a singleton instance for global use
const s3Client = new S3Client();

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
    module.exports = { S3Client, s3Client };
}
