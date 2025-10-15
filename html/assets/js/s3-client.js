/**
 * S3 Client Module - S3 data operations with retry logic and caching
 * Provides methods for fetching objects from S3 with exponential backoff
 */

class S3Client {
    constructor(options = {}) {
        this.baseUrl = options.baseUrl || window.location.origin;
        this.cache = new Map();
        this.cacheTimeout = options.cacheTimeout || 5 * 60 * 1000; // 5 minutes default
        this.maxRetries = options.maxRetries || 3;
        this.retryDelay = options.retryDelay || 1000; // 1 second initial delay
    }

    /**
     * Fetch objects from S3 with retry logic and exponential backoff
     */
    async fetchObjects(path, options = {}) {
        const cacheKey = this.getCacheKey(path, options);
        
        // Check cache first
        if (!options.skipCache) {
            const cached = this.getFromCache(cacheKey);
            if (cached) {
                console.log(`📦 Cache hit for: ${path}`);
                return cached;
            }
        }

        // Fetch with retry logic
        let lastError = null;
        for (let attempt = 0; attempt < this.maxRetries; attempt++) {
            try {
                console.log(`🔄 Fetching from S3 (attempt ${attempt + 1}/${this.maxRetries}): ${path}`);
                
                const response = await fetch(`${this.baseUrl}${path}`, {
                    method: 'GET',
                    credentials: 'same-origin',
                    headers: {
                        'Cache-Control': 'no-cache',
                        ...options.headers
                    }
                });

                if (!response.ok) {
                    throw new Error(`HTTP ${response.status}: ${response.statusText}`);
                }

                const data = await response.json();
                
                // Store in cache
                this.setCache(cacheKey, data);
                
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
     * Note: This endpoint may not exist yet, will return empty array on 404
     */
    async fetchAnnouncements(options = {}) {
        const path = '/announcements';
        try {
            return await this.fetchObjects(path, options);
        } catch (error) {
            console.warn('Announcements endpoint not available:', error);
            return [];
        }
    }

    /**
     * Fetch customer-specific announcements
     * Note: This endpoint may not exist yet, will return empty array on 404
     */
    async fetchCustomerAnnouncements(customerCode, options = {}) {
        const path = `/announcements/customer/${customerCode}`;
        try {
            return await this.fetchObjects(path, options);
        } catch (error) {
            console.warn(`Customer announcements endpoint not available for ${customerCode}:`, error);
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
     * Get data from cache if not expired
     */
    getFromCache(key) {
        const cached = this.cache.get(key);
        if (!cached) return null;

        const now = Date.now();
        if (now - cached.timestamp > this.cacheTimeout) {
            // Cache expired
            this.cache.delete(key);
            return null;
        }

        return cached.data;
    }

    /**
     * Store data in cache
     */
    setCache(key, data) {
        this.cache.set(key, {
            data,
            timestamp: Date.now()
        });
    }

    /**
     * Clear cache
     */
    clearCache(pattern = null) {
        if (pattern) {
            // Clear specific pattern
            for (const key of this.cache.keys()) {
                if (key.includes(pattern)) {
                    this.cache.delete(key);
                }
            }
        } else {
            // Clear all cache
            this.cache.clear();
        }
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
        const now = Date.now();
        let valid = 0;
        let expired = 0;

        for (const [key, value] of this.cache.entries()) {
            if (now - value.timestamp > this.cacheTimeout) {
                expired++;
            } else {
                valid++;
            }
        }

        return {
            total: this.cache.size,
            valid,
            expired,
            timeout: this.cacheTimeout
        };
    }
}

// Create a singleton instance for global use
const s3Client = new S3Client();

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
    module.exports = { S3Client, s3Client };
}
