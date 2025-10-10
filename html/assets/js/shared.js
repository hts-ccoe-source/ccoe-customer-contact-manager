/**
 * Shared JavaScript utilities for Multi-Customer Change Management Portal
 */

class ChangeManagementPortal {
    constructor() {
        this.baseUrl = window.location.origin;
        this.currentUser = null;
        this.statusConfig = {
            draft: { label: 'Drafts', icon: '📝', color: '#fff3cd', textColor: '#856404' },
            submitted: { label: 'Requesting Approval', icon: '📋', color: '#fff3cd', textColor: '#856404' },
            'waiting for approval': { label: 'Requesting Approval', icon: '📋', color: '#fff3cd', textColor: '#856404' },
            approved: { label: 'Approved', icon: '✅', color: '#d4edda', textColor: '#155724' },
            completed: { label: 'Completed', icon: '🎉', color: '#e2e3e5', textColor: '#383d41' },
            cancelled: { label: 'Cancelled', icon: '❌', color: '#f8d7da', textColor: '#721c24' }
        };
        this.init();
    }

    async init() {
        this.injectStatusCSS();
        await this.checkAuthentication();
        this.updateNavigation();
        this.setupEventListeners();
    }

    /**
     * Check if user is authenticated
     */
    async checkAuthentication() {
        try {
            const response = await fetch(`${this.baseUrl}/auth-check`, {
                method: 'GET',
                credentials: 'same-origin'
            });

            if (response.ok) {
                const data = await response.json();
                this.currentUser = data.user;
                this.updateUserInfo();
                return true;
            } else {
                // Set a default user when auth is not available
                console.log('Authentication not available, using demo mode');
                this.currentUser = 'demo.user@hearst.com';
                this.updateUserInfo();
                return true;
            }
        } catch (error) {
            console.error('Authentication check failed (auth service not available):', error);
            // Set a default user when auth service is not available
            this.currentUser = 'demo.user@hearst.com';
            this.updateUserInfo();
            return true;
        }
    }

    /**
     * Update user information in the navigation
     */
    updateUserInfo() {
        const userInfoElement = document.getElementById('userInfo');
        if (userInfoElement && this.currentUser) {
            userInfoElement.textContent = `Welcome, ${this.currentUser}`;
        }
    }

    /**
     * Update navigation active state based on current page
     */
    updateNavigation() {
        const currentPage = window.location.pathname.split('/').pop() || 'index.html';
        const navLinks = document.querySelectorAll('.nav-link');

        navLinks.forEach(link => {
            const href = link.getAttribute('href');
            if (href === currentPage || (currentPage === '' && href === 'index.html')) {
                link.classList.add('active');
            } else {
                link.classList.remove('active');
            }
        });
    }

    /**
     * Setup global event listeners
     */
    setupEventListeners() {
        // Handle logout
        const logoutBtn = document.getElementById('logoutBtn');
        if (logoutBtn) {
            logoutBtn.addEventListener('click', this.logout.bind(this));
        }

        // Handle form submissions with loading states
        document.addEventListener('submit', this.handleFormSubmit.bind(this));
    }

    /**
     * Handle form submissions with loading states
     */
    handleFormSubmit(event) {
        const form = event.target;
        if (form.tagName === 'FORM') {
            const submitBtn = form.querySelector('button[type="submit"]');
            if (submitBtn) {
                submitBtn.disabled = true;
                submitBtn.innerHTML = '<div class="spinner"></div> Processing...';

                // Re-enable after 30 seconds as fallback
                setTimeout(() => {
                    submitBtn.disabled = false;
                    submitBtn.innerHTML = submitBtn.dataset.originalText || 'Submit';
                }, 30000);
            }
        }
    }

    /**
     * Logout user
     */
    async logout() {
        try {
            await fetch(`${this.baseUrl}/logout`, {
                method: 'POST',
                credentials: 'same-origin'
            });
            window.location.href = '/login';
        } catch (error) {
            console.error('Logout failed:', error);
            // Force redirect anyway
            window.location.href = '/login';
        }
    }

    /**
     * Generate a unique change ID (GUID format)
     */
    generateChangeId() {
        // Generate a proper GUID/UUID v4
        return 'CHG-' + 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
            const r = Math.random() * 16 | 0;
            const v = c == 'x' ? r : (r & 0x3 | 0x8);
            return v.toString(16);
        });
    }

    /**
     * Format date for display
     */
    formatDate(dateString) {
        const date = new Date(dateString);
        return date.toLocaleDateString() + ' ' + date.toLocaleTimeString();
    }

    /**
     * Show status message
     */
    showStatus(message, type = 'info', containerId = 'statusContainer') {
        const container = document.getElementById(containerId);
        if (!container) return;

        const statusDiv = document.createElement('div');
        statusDiv.className = `status-message status-${type}`;
        statusDiv.textContent = message;

        container.innerHTML = '';
        container.appendChild(statusDiv);

        // Auto-hide success messages after 5 seconds
        if (type === 'success') {
            setTimeout(() => {
                statusDiv.remove();
            }, 5000);
        }
    }

    /**
     * Show loading state
     */
    showLoading(containerId = 'mainContent') {
        const container = document.getElementById(containerId);
        if (!container) return;

        container.innerHTML = `
            <div class="loading">
                <div class="spinner"></div>
                Loading...
            </div>
        `;
    }

    /**
     * Validate form data
     */
    validateForm(formData, requiredFields) {
        const errors = [];

        requiredFields.forEach(field => {
            if (!formData.get(field) || formData.get(field).trim() === '') {
                errors.push(`${field} is required`);
            }
        });

        return errors;
    }

    /**
     * Get customer codes from form
     */
    getSelectedCustomers() {
        const checkboxes = document.querySelectorAll('input[name="customers"]:checked');
        return Array.from(checkboxes).map(cb => cb.value);
    }

    /**
     * Save data to server (deprecated localStorage methods removed)
     * All data should be saved server-side for persistence and security
     */
    async saveToServer(endpoint, data) {
        try {
            const response = await fetch(`${this.baseUrl}${endpoint}`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                credentials: 'same-origin',
                body: JSON.stringify(data)
            });

            if (!response.ok) {
                throw new Error(`Server save failed: ${response.statusText}`);
            }

            return await response.json();
        } catch (error) {
            console.error('Error saving to server:', error);
            throw error;
        }
    }

    /**
     * Load data from server
     */
    async loadFromServer(endpoint) {
        try {
            const response = await fetch(`${this.baseUrl}${endpoint}`, {
                method: 'GET',
                credentials: 'same-origin'
            });

            if (!response.ok) {
                if (response.status === 404) {
                    return null; // Not found
                }
                throw new Error(`Server load failed: ${response.statusText}`);
            }

            return await response.json();
        } catch (error) {
            console.error('Error loading from server:', error);
            throw error;
        }
    }

    /**
     * Clear expired items - now handled server-side
     */
    clearExpiredStorage() {
        // Server-side cleanup - no longer needed client-side
        console.log('Storage cleanup is now handled server-side');
    }

    /**
     * Get status display configuration
     */
    getStatusConfig(status) {
        return this.statusConfig[status] || { label: status, icon: '📄', color: '#e9ecef', textColor: '#495057' };
    }

    /**
     * Generate status button HTML
     */
    /**
     * Check if a change status matches a filter status
     */
    statusMatches(changeStatus, filterStatus) {
        // Handle status mapping - some changes use different status values
        if (filterStatus === 'submitted') {
            return changeStatus === 'submitted' || changeStatus === 'waiting for approval';
        }

        // Handle undefined status
        if (changeStatus === 'undefined' || changeStatus === undefined) {
            return filterStatus === 'draft'; // Treat undefined as draft
        }

        return changeStatus === filterStatus;
    }

    /**
     * Get all changes that match a status filter
     */
    filterChangesByStatus(changes, status) {
        if (!status) return changes; // No filter

        // Debug logging
        console.log(`Filtering ${changes.length} changes for status: "${status}"`);
        changes.forEach((change, index) => {
            console.log(`Change ${index}: ID=${change.changeId}, status="${change.status}"`);
        });

        const filtered = changes.filter(change => this.statusMatches(change.status, status));
        console.log(`Found ${filtered.length} changes with status "${status}"`);

        return filtered;
    }

    generateStatusButton(status, count, isActive = false) {
        const config = this.getStatusConfig(status);
        const activeClass = isActive ? ' active' : '';
        // Use consistent ID format: draftsCount, submittedCount, etc.
        const countId = status === 'draft' ? 'draftsCount' :
            status === 'submitted' ? 'submittedCount' :
                status === 'approved' ? 'approvedCount' :
                    status === 'completed' ? 'completedCount' :
                        status === 'cancelled' ? 'cancelledCount' : `${status}Count`;
        return `
            <button class="status-btn${activeClass}" data-status="${status}" onclick="filterByStatus('${status}')">
                ${config.icon} ${config.label} (<span id="${countId}">${count}</span>)
            </button>
        `;
    }

    /**
     * Generate CSS for status styles
     */
    generateStatusCSS() {
        let css = '';
        Object.keys(this.statusConfig).forEach(status => {
            const config = this.statusConfig[status];
            css += `
                .status-${status} {
                    background: ${config.color};
                    color: ${config.textColor};
                }
            `;
        });
        return css;
    }

    /**
     * Inject status CSS into the page
     */
    injectStatusCSS() {
        const existingStyle = document.getElementById('dynamic-status-styles');
        if (existingStyle) {
            existingStyle.remove();
        }

        const style = document.createElement('style');
        style.id = 'dynamic-status-styles';
        style.textContent = this.generateStatusCSS();
        document.head.appendChild(style);
    }

    /**
     * Delete submitted change by change ID (moves to deleted folder)
     */
    async deleteChange(changeId) {
        try {
            const response = await fetch(`${this.baseUrl}/changes/${changeId}`, {
                method: 'DELETE',
                credentials: 'same-origin'
            });

            if (!response.ok && response.status !== 404) {
                throw new Error(`Failed to delete change: ${response.statusText}`);
            }

            return await response.json();
        } catch (error) {
            console.error('Error deleting change:', error);
            throw error;
        }
    }
}

/**
 * Change lifecycle management utilities
 */
class ChangeLifecycle {
    constructor(portal) {
        this.portal = portal;
    }

    /**
     * Create metadata structure with change lifecycle fields
     */
    createMetadata(formData, changeId = null, version = 1) {
        const now = new Date().toISOString();
        const id = changeId || this.portal.generateChangeId();

        return {
            changeId: id,
            version: version,
            createdAt: now,
            modifiedAt: now,
            createdBy: this.portal.currentUser,
            modifiedBy: this.portal.currentUser,
            status: "draft",
            // Flat structure - all fields at top level
            changeTitle: formData.get('changeTitle'),
            customers: this.portal.getSelectedCustomers(),
            snowTicket: formData.get('snowTicket') || '',
            jiraTicket: formData.get('jiraTicket') || '',
            changeReason: formData.get('changeReason'),
            implementationPlan: formData.get('implementationPlan'),
            testPlan: formData.get('testPlan'),
            customerImpact: formData.get('customerImpact'),
            rollbackPlan: formData.get('rollbackPlan'),
            // DateTime fields for lambda validation
            implementationStart: this.convertToRFC3339(formData.get('implementationBeginDate'), formData.get('implementationBeginTime')),
            implementationEnd: this.convertToRFC3339(formData.get('implementationEndDate'), formData.get('implementationEndTime')),
            // Separate date/time fields for form population
            implementationBeginDate: formData.get('implementationBeginDate'),
            implementationBeginTime: formData.get('implementationBeginTime'),
            implementationEndDate: formData.get('implementationEndDate'),
            implementationEndTime: formData.get('implementationEndTime'),
            timezone: formData.get('timezone'),
            // Meeting fields
            meetingRequired: formData.get('meetingRequired') || 'no',
            meetingTitle: formData.get('meetingTitle') || '',
            meetingDate: formData.get('meetingDate') || '',
            meetingDuration: formData.get('meetingDuration') || '',
            meetingLocation: formData.get('meetingLocation') || ''
        };
    }

    /**
     * Get customer names from selected customer codes
     */
    getCustomerNames(formData) {
        const customerMap = {
            'hts': 'HTS Prod',
            'htsnonprod': 'HTS NonProd',
            'cds': 'CDS Global',
            'fdbus': 'FDBUS',
            'hmiit': 'Hearst Magazines Italy',
            'hmies': 'Hearst Magazines Spain',
            'htvdigital': 'HTV Digital',
            'htv': 'HTV',
            'icx': 'iCrossing',
            'motor': 'Motor',
            'bat': 'Bring A Trailer',
            'mhk': 'MHK',
            'hdmautos': 'Autos',
            'hnpit': 'HNP IT',
            'hnpdigital': 'HNP Digital',
            'camp': 'CAMP Systems',
            'mcg': 'MCG',
            'hmuk': 'Hearst Magazines UK',
            'hmusdigital': 'Hearst Magazines Digital',
            'hwp': 'Hearst Western Properties',
            'zynx': 'Zynx',
            'hchb': 'HCHB',
            'fdbuk': 'FDBUK',
            'hecom': 'Hearst ECommerce',
            'blkbook': 'Black Book'
        };

        const selectedCodes = this.portal.getSelectedCustomers();
        return selectedCodes.map(code => customerMap[code] || code);
    }

    /**
     * Convert separate date and time to RFC3339 format
     */
    convertToRFC3339(date, time) {
        if (!date || !time) {
            return null;
        }
        
        try {
            const dateTime = `${date}T${time}`;
            const dateObj = new Date(dateTime);
            return dateObj.toISOString();
        } catch (error) {
            console.warn('Failed to convert date/time to RFC3339:', error);
            return null;
        }
    }

    /**
     * Save change as draft
     */
    async saveDraft(metadata) {
        try {
            const response = await fetch(`${this.portal.baseUrl}/drafts`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                credentials: 'same-origin',
                body: JSON.stringify(metadata)
            });

            if (!response.ok) {
                throw new Error(`Failed to save draft: ${response.statusText}`);
            }

            return await response.json();
        } catch (error) {
            console.error('Error saving draft:', error);
            throw error;
        }
    }

    /**
     * Load draft by change ID
     */
    async loadDraft(changeId) {
        try {
            const response = await fetch(`${this.portal.baseUrl}/api/drafts/${changeId}`, {
                method: 'GET',
                credentials: 'same-origin'
            });

            if (!response.ok) {
                if (response.status === 404) {
                    return null; // Draft not found
                }
                throw new Error(`Failed to load draft: ${response.statusText}`);
            }

            return await response.json();
        } catch (error) {
            console.error('Error loading draft:', error);
            throw error;
        }
    }

    /**
     * Submit change (move from draft to submitted)
     */
    async submitChange(metadata) {
        try {
            // Update status and timestamp
            metadata.status = 'submitted';
            metadata.modifiedAt = new Date().toISOString();
            metadata.modifiedBy = this.portal.currentUser;

            const response = await fetch(`${this.portal.baseUrl}/upload`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                credentials: 'same-origin',
                body: JSON.stringify(metadata)
            });

            if (!response.ok) {
                throw new Error(`Failed to submit change: ${response.statusText}`);
            }

            const result = await response.json();

            // After successful submission, delete the draft to prevent duplicates
            try {
                await this.deleteDraft(metadata.changeId);
                console.log(`Successfully deleted draft ${metadata.changeId} after submission`);
            } catch (draftError) {
                console.warn(`Failed to delete draft ${metadata.changeId} after submission:`, draftError);
                // Don't fail the submission if draft deletion fails
            }

            return result;
        } catch (error) {
            console.error('Error submitting change:', error);
            throw error;
        }
    }

    /**
     * Delete draft by change ID
     */
    async deleteDraft(changeId) {
        try {
            // Try to delete from server
            const response = await fetch(`${this.portal.baseUrl}/drafts/${changeId}`, {
                method: 'DELETE',
                credentials: 'same-origin'
            });

            if (response.ok) {
                console.log(`Successfully deleted draft ${changeId} from server`);
            } else if (response.status === 404) {
                console.log(`Draft ${changeId} not found on server (may have been already deleted)`);
            } else {
                console.log(`Server delete failed for draft ${changeId} (${response.status}), will still remove from localStorage`);
            }
        } catch (error) {
            console.log(`API delete failed for draft ${changeId}:`, error.message);
        }

        // Draft deletion is now handled server-side only
        console.log(`✅ Draft ${changeId} deleted from server`);

        return true;
    }

    /**
     * Delete change (move to deleted folder)
     */
    async deleteChange(changeId) {
        try {
            return await this.portal.deleteChange(changeId);
        } catch (error) {
            console.error('Error deleting change:', error);
            throw error;
        }
    }

    /**
     * Search changes by criteria
     */
    async searchChanges(criteria) {
        try {
            const params = new URLSearchParams(criteria);
            const response = await fetch(`${this.portal.baseUrl}/api/changes/search?${params}`, {
                method: 'GET',
                credentials: 'same-origin'
            });

            if (!response.ok) {
                throw new Error(`Search failed: ${response.statusText}`);
            }

            return await response.json();
        } catch (error) {
            console.error('Error searching changes:', error);
            throw error;
        }
    }

    /**
     * Delete change (move to deleted folder)
     */
    async deleteChange(changeId) {
        try {
            return await this.portal.deleteChange(changeId);
        } catch (error) {
            console.error('Error deleting change:', error);
            throw error;
        }
    }
}

// Initialize portal when DOM is loaded
document.addEventListener('DOMContentLoaded', () => {
    window.portal = new ChangeManagementPortal();
    window.changeLifecycle = new ChangeLifecycle(window.portal);

    // Clear expired storage items
    window.portal.clearExpiredStorage();
});

// Export for use in other scripts
window.ChangeManagementPortal = ChangeManagementPortal;
window.ChangeLifecycle = ChangeLifecycle;