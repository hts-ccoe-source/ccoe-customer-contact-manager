/**
 * Announcements Page Module
 * Manages the announcements page functionality including loading, filtering, and displaying announcements
 */

class AnnouncementsPage {
    constructor() {
        this.announcements = [];
        this.filteredAnnouncements = [];
        this.currentStatus = ''; // Empty string means 'all'
        this.filters = {
            type: 'all',
            sort: 'newest',
            customer: 'all',
            dateRange: '',
            search: ''
        };
        this.s3Client = new S3Client();
        this.loadingManager = new LoadingManager({ container: '#announcementsList' });
        this.userContext = null; // Will store user role and customer info

        this.init();
    }

    /**
     * Initialize the page
     */
    async init() {
        console.log('🎯 Initializing Announcements Page');

        // Detect user context (admin vs customer user)
        await this.detectUserContext();

        // Generate status filter buttons
        this.generateStatusButtons();

        // Set up event listeners
        this.setupEventListeners();

        // Load announcements
        await this.loadAnnouncements();
    }

    /**
     * Generate status filter buttons
     */
    generateStatusButtons() {
        const container = document.getElementById('statusFilters');
        if (!container) return;

        try {
            container.innerHTML = `
                <button class="status-btn ${this.currentStatus === '' ? 'active' : ''}" data-status="" onclick="announcementsPage.filterByStatus('')">
                    📋 All Announcements (<span id="allCount">0</span>)
                </button>
                <button class="status-btn ${this.currentStatus === 'draft' ? 'active' : ''}" data-status="draft" onclick="announcementsPage.filterByStatus('draft')">
                    📝 Drafts (<span id="draftsCount">0</span>)
                </button>
                <button class="status-btn ${this.currentStatus === 'submitted' ? 'active' : ''}" data-status="submitted" onclick="announcementsPage.filterByStatus('submitted')">
                    📋 Submitted (<span id="pendingCount">0</span>)
                </button>
                <button class="status-btn ${this.currentStatus === 'approved' ? 'active' : ''}" data-status="approved" onclick="announcementsPage.filterByStatus('approved')">
                    ✅ Approved (<span id="approvedCount">0</span>)
                </button>
                <button class="status-btn ${this.currentStatus === 'completed' ? 'active' : ''}" data-status="completed" onclick="announcementsPage.filterByStatus('completed')">
                    🎉 Completed (<span id="completedCount">0</span>)
                </button>
                <button class="status-btn ${this.currentStatus === 'cancelled' ? 'active' : ''}" data-status="cancelled" onclick="announcementsPage.filterByStatus('cancelled')">
                    ❌ Cancelled (<span id="cancelledCount">0</span>)
                </button>
            `;
        } catch (error) {
            console.error('Error generating status buttons:', error);
        }
    }

    /**
     * Filter announcements by status
     */
    filterByStatus(status) {
        this.currentStatus = status;

        // Update active button
        document.querySelectorAll('.status-btn').forEach(btn => btn.classList.remove('active'));
        const activeBtn = document.querySelector(`[data-status="${status}"]`);
        if (activeBtn) {
            activeBtn.classList.add('active');
        }

        // Apply filters
        this.applyFilters();
    }

    /**
     * Detect user context from authentication
     * Determines if user is admin or customer-specific
     */
    async detectUserContext() {
        try {
            // Try to get user context from authentication
            const response = await fetch(`${window.location.origin}/api/user/context`, {
                method: 'GET',
                credentials: 'same-origin'
            });

            if (response.ok) {
                this.userContext = await response.json();
                console.log('👤 User context detected:', this.userContext);
            } else {
                // Fallback: check if user info is available in window.portal
                if (window.portal && window.portal.currentUser) {
                    this.userContext = this.inferUserContext(window.portal.currentUser);
                } else {
                    // Default to admin for demo/development
                    this.userContext = {
                        isAdmin: true,
                        customerCode: null,
                        email: 'demo.user@hearst.com'
                    };
                }
            }
        } catch (error) {
            console.warn('⚠️ Could not detect user context, defaulting to admin:', error);
            this.userContext = {
                isAdmin: true,
                customerCode: null,
                email: window.portal?.currentUser || 'unknown'
            };
        }

        // Apply customer filter if user is not admin
        if (!this.userContext.isAdmin && this.userContext.customerCode) {
            this.filters.customer = this.userContext.customerCode;
            console.log(`🔒 Customer user detected, filtering to: ${this.userContext.customerCode}`);
        }
    }

    /**
     * Infer user context from email or user attributes
     * This is a fallback when API is not available
     */
    inferUserContext(email) {
        // Check if email contains admin indicators
        const adminPatterns = [
            '@hearst.com',
            'admin',
            'ccoe',
            'cloudops'
        ];

        const isAdmin = adminPatterns.some(pattern =>
            email.toLowerCase().includes(pattern.toLowerCase())
        );

        // Try to extract customer code from email
        // Example: user@hts.hearst.com -> customer code 'hts'
        let customerCode = null;
        const emailParts = email.split('@');
        if (emailParts.length > 1) {
            const domain = emailParts[1];
            const domainParts = domain.split('.');
            if (domainParts.length > 1 && domainParts[0] !== 'hearst') {
                customerCode = domainParts[0];
            }
        }

        return {
            isAdmin,
            customerCode,
            email
        };
    }

    /**
     * Set up event listeners
     */
    setupEventListeners() {
        // Date filter
        const dateFilter = document.getElementById('dateFilter');
        if (dateFilter) {
            dateFilter.addEventListener('change', (e) => {
                this.filters.dateRange = e.target.value;
                this.applyFilters();
            });
        }

        // Search filter
        const searchFilter = document.getElementById('searchFilter');
        if (searchFilter) {
            searchFilter.addEventListener('input', (e) => {
                this.filters.search = e.target.value;
                this.applyFilters();
            });
        }

        // Type filter
        const typeFilter = document.getElementById('typeFilter');
        if (typeFilter) {
            typeFilter.addEventListener('change', (e) => {
                this.filters.type = e.target.value;
                this.applyFilters();
            });
        }

        // Sort filter
        const sortFilter = document.getElementById('sortFilter');
        if (sortFilter) {
            sortFilter.addEventListener('change', (e) => {
                this.filters.sort = e.target.value;
                this.applyFilters();
            });
        }

        // Customer filter (for admin users)
        const customerFilter = document.getElementById('customerFilter');
        if (customerFilter) {
            customerFilter.addEventListener('change', (e) => {
                this.filters.customer = e.target.value;
                this.applyFilters();
            });
        }
    }

    /**
     * Clear all filters
     */
    clearFilters() {
        this.currentStatus = '';
        this.filters.dateRange = '';
        this.filters.search = '';

        // Reset UI
        const dateFilter = document.getElementById('dateFilter');
        if (dateFilter) dateFilter.value = '';

        const searchFilter = document.getElementById('searchFilter');
        if (searchFilter) searchFilter.value = '';

        // Update active button
        document.querySelectorAll('.status-btn').forEach(btn => btn.classList.remove('active'));
        const allBtn = document.querySelector('[data-status=""]');
        if (allBtn) allBtn.classList.add('active');

        this.applyFilters();
    }

    /**
     * Load announcements from S3
     */
    async loadAnnouncements() {
        const container = document.getElementById('announcementsList');
        if (!container) return;

        try {
            console.log('📥 Loading announcements from S3...');

            // Show loading state
            container.innerHTML = `
                <div class="loading-container">
                    <div class="loading-spinner spinner-medium"></div>
                    <div class="loading-message">Loading announcements...</div>
                </div>
            `;

            // Fetch announcements based on user context
            let data;
            if (!this.userContext.isAdmin && this.userContext.customerCode) {
                // Customer user - fetch only their announcements
                console.log(`📥 Fetching announcements for customer: ${this.userContext.customerCode}`);
                data = await this.s3Client.fetchCustomerAnnouncements(this.userContext.customerCode);
            } else {
                // Admin user - fetch all announcements
                console.log('📥 Fetching all announcements (admin view)');
                data = await this.s3Client.fetchAnnouncements();
            }

            // Filter by object_type starting with "announcement_"
            this.announcements = this.s3Client.filterByObjectType(data, 'announcement_*');

            console.log(`✅ Loaded ${this.announcements.length} announcements`);

            // Show user context banner
            this.showUserContextBanner();

            // Populate customer filter dropdown (for admin users)
            this.populateCustomerFilter();

            // Apply filters and render
            this.applyFilters();

        } catch (error) {
            console.error('❌ Error loading announcements:', error);

            container.innerHTML = '';
            showError(container, `Failed to load announcements: ${error.message}`, {
                duration: 0,
                dismissible: true
            });

            // Show empty state
            this.renderEmptyState('Error loading announcements. Please try again.');
        }
    }

    /**
     * Show user context banner to indicate viewing mode
     */
    showUserContextBanner() {
        const bannerContainer = document.getElementById('userContextBanner');
        if (!bannerContainer) return;

        if (!this.userContext.isAdmin && this.userContext.customerCode) {
            // Show customer-specific banner
            bannerContainer.innerHTML = `
                <div class="info-banner">
                    <span class="info-icon">ℹ️</span>
                    <span>Viewing announcements for: <strong>${this.getCustomerName(this.userContext.customerCode)}</strong></span>
                </div>
            `;
        } else if (this.userContext.isAdmin) {
            // Show admin banner
            bannerContainer.innerHTML = `
                <div class="info-banner admin-banner">
                    <span class="info-icon">👤</span>
                    <span>Admin View: You can see announcements for all customers</span>
                </div>
            `;
        }
    }

    /**
     * Populate customer filter dropdown with unique customers
     */
    populateCustomerFilter() {
        const customerFilter = document.getElementById('customerFilter');
        if (!customerFilter) return;

        // Get unique customers from announcements
        const customers = new Set();
        this.announcements.forEach(announcement => {
            if (Array.isArray(announcement.customers)) {
                announcement.customers.forEach(customer => customers.add(customer));
            } else if (announcement.customer) {
                customers.add(announcement.customer);
            } else if (announcement.target_customers) {
                // Some announcements might use target_customers field
                if (Array.isArray(announcement.target_customers)) {
                    announcement.target_customers.forEach(customer => customers.add(customer));
                }
            }
        });

        // Sort customers alphabetically
        const sortedCustomers = Array.from(customers).sort();

        // If user is not admin, disable the filter and show only their customer
        if (!this.userContext.isAdmin && this.userContext.customerCode) {
            customerFilter.innerHTML = `<option value="${this.userContext.customerCode}">${this.getCustomerName(this.userContext.customerCode)}</option>`;
            customerFilter.disabled = true;
            customerFilter.title = 'You can only view announcements for your organization';
            return;
        }

        // Admin users get full dropdown
        const currentValue = customerFilter.value;
        customerFilter.innerHTML = '<option value="all">All Customers</option>';
        customerFilter.disabled = false;
        customerFilter.title = '';

        sortedCustomers.forEach(customer => {
            const option = document.createElement('option');
            option.value = customer;
            option.textContent = this.getCustomerName(customer);
            customerFilter.appendChild(option);
        });

        // Restore previous selection if it still exists
        if (currentValue && (currentValue === 'all' || sortedCustomers.includes(currentValue))) {
            customerFilter.value = currentValue;
        }
    }

    /**
     * Get customer friendly name
     */
    getCustomerName(customerCode) {
        // Map customer codes to friendly names
        const names = {
            'hts': 'HTS Prod',
            'cds': 'CDS Global',
            'fdbus': 'FDBUS',
            'unknown': 'Unknown Customer'
        };
        return names[customerCode?.toLowerCase()] || customerCode?.toUpperCase() || 'Unknown';
    }

    /**
     * Apply filters to announcements
     */
    applyFilters() {
        console.log('🔍 Applying filters:', { status: this.currentStatus, ...this.filters });

        let filtered = [...this.announcements];

        // Filter by status
        if (this.currentStatus) {
            if (this.currentStatus === 'submitted') {
                // Filter for submitted status
                filtered = filtered.filter(a => a.status === 'submitted');
            } else {
                filtered = filtered.filter(a => a.status === this.currentStatus);
            }
        }

        // Filter by date range
        if (this.filters.dateRange) {
            const now = new Date();
            let filterDate;

            switch (this.filters.dateRange) {
                case 'today':
                    filterDate = new Date(now.getFullYear(), now.getMonth(), now.getDate());
                    break;
                case 'week':
                    filterDate = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000);
                    break;
                case 'month':
                    filterDate = new Date(now.getFullYear(), now.getMonth(), 1);
                    break;
                case 'quarter':
                    const quarter = Math.floor(now.getMonth() / 3);
                    filterDate = new Date(now.getFullYear(), quarter * 3, 1);
                    break;
            }

            if (filterDate) {
                filtered = filtered.filter(a => {
                    const announcementDate = new Date(a.posted_date || a.created_date);
                    return announcementDate >= filterDate;
                });
            }
        }

        // Filter by search text
        if (this.filters.search) {
            const searchLower = this.filters.search.toLowerCase();
            filtered = filtered.filter(a => {
                const title = (a.title || '').toLowerCase();
                const summary = (a.summary || '').toLowerCase();
                return title.includes(searchLower) || summary.includes(searchLower);
            });
        }

        // Filter by type
        if (this.filters.type !== 'all') {
            const typeMap = {
                'finops': 'announcement_finops',
                'innersourcing': 'announcement_innersourcing',
                'cic': 'announcement_cic',
                'general': 'announcement_general'
            };

            const objectType = typeMap[this.filters.type];
            filtered = filtered.filter(a => a.object_type === objectType);
        }

        // Filter by customer (for admin users)
        if (this.filters.customer && this.filters.customer !== 'all') {
            filtered = filtered.filter(announcement => {
                // Check various customer fields
                if (Array.isArray(announcement.customers)) {
                    return announcement.customers.includes(this.filters.customer);
                } else if (announcement.customer) {
                    return announcement.customer === this.filters.customer;
                } else if (Array.isArray(announcement.target_customers)) {
                    return announcement.target_customers.includes(this.filters.customer);
                }
                // If no customer info, include in "all" view only
                return false;
            });
        }

        // Sort by date
        const dateField = 'posted_date';
        filtered = this.s3Client.sortByDate(
            filtered,
            dateField,
            this.filters.sort === 'newest'
        );

        this.filteredAnnouncements = filtered;

        // Update status counts
        this.updateStatusCounts();

        this.render();
    }

    /**
     * Update status counts in filter buttons
     */
    updateStatusCounts() {
        const counts = {
            all: this.announcements.length,
            draft: this.announcements.filter(a => a.status === 'draft').length,
            submitted: this.announcements.filter(a => a.status === 'submitted').length,
            approved: this.announcements.filter(a => a.status === 'approved').length,
            completed: this.announcements.filter(a => a.status === 'completed').length,
            cancelled: this.announcements.filter(a => a.status === 'cancelled').length
        };

        // Update count displays
        const updateCount = (id, count) => {
            const el = document.getElementById(id);
            if (el) el.textContent = count;
        };

        updateCount('allCount', counts.all);
        updateCount('draftsCount', counts.draft);
        updateCount('pendingCount', counts.submitted);
        updateCount('approvedCount', counts.approved);
        updateCount('completedCount', counts.completed);
        updateCount('cancelledCount', counts.cancelled);
    }

    /**
     * Render announcements
     */
    render() {
        const container = document.getElementById('announcementsList');
        if (!container) return;

        // Clear container
        container.innerHTML = '';

        // Check if we have announcements
        if (this.filteredAnnouncements.length === 0) {
            this.renderEmptyState();
            return;
        }

        // Create grid container
        const grid = document.createElement('div');
        grid.className = 'announcements-grid';

        // Render each announcement
        this.filteredAnnouncements.forEach(announcement => {
            const card = this.renderAnnouncementCard(announcement);
            grid.appendChild(card);
        });

        container.appendChild(grid);

        console.log(`✅ Rendered ${this.filteredAnnouncements.length} announcements`);
    }

    /**
     * Render a single announcement card
     */
    renderAnnouncementCard(announcement) {
        const card = document.createElement('div');
        card.className = 'announcement-card';
        card.setAttribute('role', 'article');
        card.setAttribute('tabindex', '0');

        // Extract type from object_type (e.g., "announcement_finops" -> "finops")
        const type = this.getAnnouncementType(announcement.object_type);
        const icon = this.getTypeIcon(type);
        const typeName = this.getTypeName(type);
        const announcementTitle = this.escapeHtml(announcement.title || announcement.changeTitle || 'Untitled Announcement');
        const announcementId = announcement.announcement_id || announcement.changeId || announcement.id;

        // Format date
        const postedDate = announcement.posted_date
            ? new Date(announcement.posted_date).toLocaleDateString('en-US', {
                year: 'numeric',
                month: 'short',
                day: 'numeric'
            })
            : 'Unknown date';

        card.setAttribute('aria-label', `${typeName} announcement: ${announcementTitle}, posted ${postedDate}`);

        // Get current user for ownership check
        const currentUser = window.portal?.currentUser || '';
        const isOwner = announcement.created_by === currentUser || announcement.author === currentUser || announcement.submittedBy === currentUser;

        // Get author display name (use actual email if available)
        const authorDisplay = announcement.submittedBy || announcement.created_by || announcement.author || 'Unknown';

        // Get status label
        const statusLabel = announcement.status ? announcement.status.replace('_', ' ').toUpperCase() : 'UNKNOWN';

        card.innerHTML = `
            <div class="change-header">
                <div class="change-info">
                    <div class="change-title">${announcementTitle}</div>
                    <div class="change-id">${announcementId}</div>
                    <div class="change-meta">
                        <span>📅 ${postedDate}</span>
                        <span>👤 ${this.escapeHtml(authorDisplay)}</span>
                        <span>${icon} ${typeName}</span>
                    </div>
                </div>
                <div class="change-status status-${announcement.status || 'unknown'}">
                    ${statusLabel}
                </div>
            </div>
            
            ${announcement.summary ? `
                <div class="change-summary">
                    ${this.escapeHtml(announcement.summary)}
                </div>
            ` : ''}
            
            <div class="change-actions" onclick="event.stopPropagation()">
                <a href="edit-announcement.html?announcementId=${announcementId}&duplicate=true" class="action-btn" onclick="event.stopPropagation()">
                    📋 Duplicate
                </a>
                ${isOwner ? this.renderWorkflowButtons(announcement) : ''}
            </div>
        `;

        // Make entire card clickable to view details
        card.addEventListener('click', () => {
            this.showAnnouncementDetails(announcement);
        });

        // Add keyboard support
        card.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault();
                this.showAnnouncementDetails(announcement);
            }
        });

        return card;
    }

    /**
     * Get announcement type from object_type
     */
    getAnnouncementType(objectType) {
        if (!objectType || !objectType.startsWith('announcement_')) {
            return 'general';
        }
        return objectType.replace('announcement_', '');
    }

    /**
     * Get icon for announcement type
     */
    getTypeIcon(type) {
        const icons = {
            'finops': '💰',
            'innersourcing': '🔧',
            'cic': '☁️',
            'general': '📢'
        };
        return icons[type] || '📢';
    }

    /**
     * Get display name for announcement type
     */
    getTypeName(type) {
        const names = {
            'finops': 'FinOps',
            'innersourcing': 'InnerSourcing Guild',
            'cic': 'CIC / Cloud Innovator Community',
            'general': 'General'
        };
        return names[type] || 'General';
    }

    /**
     * Render workflow buttons for announcements
     */
    renderWorkflowButtons(announcement) {
        const buttons = [];
        const currentUser = window.portal?.currentUser || '';
        const isAdmin = this.userContext?.isAdmin || false;

        if (announcement.status === 'draft') {
            buttons.push(`
                <a href="edit-announcement.html?announcementId=${announcement.announcement_id}" class="action-btn edit" onclick="event.stopPropagation()">
                    ✏️ Edit
                </a>
                <button class="action-btn danger" onclick="event.stopPropagation(); announcementsPage.deleteAnnouncement('${announcement.announcement_id}')">
                    🗑️ Delete
                </button>
                <button class="action-btn success" onclick="event.stopPropagation(); announcementsPage.submitAnnouncement('${announcement.announcement_id}')">
                    🚀 Submit
                </button>
            `);
        } else if (announcement.status === 'submitted') {
            // Show cancel button for owner (first, matching my-changes pattern)
            buttons.push(`
                <button class="action-btn cancel" onclick="event.stopPropagation(); announcementsPage.cancelAnnouncement('${announcement.announcement_id}')">
                    💣 Cancel
                </button>
            `);
            // Show approve button for admins (second, matching my-changes pattern)
            if (isAdmin) {
                buttons.push(`
                    <button class="action-btn approve" onclick="event.stopPropagation(); announcementsPage.approveAnnouncement('${announcement.announcement_id}')">
                        ✅ Approve
                    </button>
                `);
            }
        } else if (announcement.status === 'approved') {
            // Approved announcements can be cancelled or completed (matching changes pattern)
            buttons.push(`
                <button class="action-btn cancel" onclick="event.stopPropagation(); announcementsPage.cancelAnnouncement('${announcement.announcement_id}')">
                    💣 Cancel
                </button>
                <button class="action-btn complete" onclick="event.stopPropagation(); announcementsPage.completeAnnouncement('${announcement.announcement_id}')">
                    🎯 Complete
                </button>
            `);
        } else if (announcement.status === 'cancelled') {
            // Cancelled announcements can only be deleted (matching changes pattern)
            buttons.push(`
                <button class="action-btn danger" onclick="event.stopPropagation(); announcementsPage.deleteAnnouncement('${announcement.announcement_id}')">
                    🗑️ Delete
                </button>
            `);
        }

        return buttons.join('');
    }

    /**
     * Approve announcement
     */
    async approveAnnouncement(announcementId) {
        try {
            const announcement = this.announcements.find(a => a.announcement_id === announcementId);
            if (!announcement) {
                throw new Error('Announcement not found');
            }

            // Use announcement actions module
            const actions = new AnnouncementActions(announcementId, announcement.status, announcement);
            await actions.approveAnnouncement();

            // Reload announcements after action completes
            setTimeout(() => {
                this.loadAnnouncements();
            }, 2000);
        } catch (error) {
            console.error('Error approving announcement:', error);
        }
    }

    /**
     * Get customer code from announcement
     */
    getAnnouncementCustomer(announcement) {
        // Check various customer fields
        if (announcement.customer) {
            return announcement.customer;
        }
        if (Array.isArray(announcement.customers) && announcement.customers.length > 0) {
            return announcement.customers[0];
        }
        if (Array.isArray(announcement.target_customers) && announcement.target_customers.length > 0) {
            return announcement.target_customers[0];
        }
        return null;
    }

    /**
     * Render tags
     */
    renderTags(tags) {
        if (!tags || !Array.isArray(tags) || tags.length === 0) {
            return '';
        }

        return tags.slice(0, 3).map(tag =>
            `<span class="tag" role="listitem">${this.escapeHtml(tag)}</span>`
        ).join('');
    }

    /**
     * Submit announcement for approval
     */
    async submitAnnouncement(announcementId) {
        try {
            const announcement = this.announcements.find(a => a.announcement_id === announcementId);
            if (!announcement) {
                throw new Error('Announcement not found');
            }

            // Only submit if status is draft
            if (announcement.status !== 'draft') {
                console.log(`Cannot submit announcement with status: ${announcement.status}`);
                return;
            }

            // Update status to submitted
            const actions = new AnnouncementActions(announcementId, announcement.status, announcement);
            await actions.updateAnnouncementStatus('submitted', 'submitted');

            // Clear cache and reload announcements
            this.s3Client.clearCache('/announcements');
            setTimeout(() => {
                this.loadAnnouncements();
            }, 1000);
        } catch (error) {
            console.error('Error submitting announcement:', error);
        }
    }

    /**
     * Delete announcement (draft or cancelled only)
     */
    async deleteAnnouncement(announcementId) {
        if (!confirm('Are you sure you want to delete this announcement?')) {
            return;
        }

        try {
            const announcement = this.announcements.find(a => a.announcement_id === announcementId);
            if (!announcement) {
                throw new Error('Announcement not found');
            }

            // Only allow deleting drafts or cancelled announcements (per state machine)
            if (announcement.status !== 'draft' && announcement.status !== 'cancelled') {
                console.error(`Cannot delete announcement with status: ${announcement.status}. Only draft or cancelled announcements can be deleted.`);
                return;
            }

            // Delete via API
            const response = await fetch(`${window.location.origin}/announcements/${announcementId}`, {
                method: 'DELETE',
                credentials: 'same-origin'
            });

            if (!response.ok) {
                if (response.status === 404) {
                    // Announcement already deleted or doesn't exist
                    console.log('Announcement already deleted or not found, refreshing list...');
                    // Clear cache and reload to get fresh data
                    this.s3Client.clearCache('/announcements');
                    await this.loadAnnouncements();
                    return;
                }
                throw new Error(`Failed to delete announcement: ${response.statusText}`);
            }

            // Clear cache and reload announcements
            this.s3Client.clearCache('/announcements');
            setTimeout(() => {
                this.loadAnnouncements();
            }, 1000);
        } catch (error) {
            console.error('Error deleting announcement:', error);
            showError(document.getElementById('announcementsList'), error.message, {
                duration: 5000,
                dismissible: true
            });
        }
    }

    /**
     * Cancel announcement
     */
    async cancelAnnouncement(announcementId) {
        try {
            const announcement = this.announcements.find(a => a.announcement_id === announcementId);
            if (!announcement) {
                throw new Error('Announcement not found');
            }

            // Use announcement actions module
            const actions = new AnnouncementActions(announcementId, announcement.status, announcement);
            await actions.cancelAnnouncement();

            // Reload announcements after action completes
            setTimeout(() => {
                this.loadAnnouncements();
            }, 2000);
        } catch (error) {
            console.error('Error cancelling announcement:', error);
        }
    }

    /**
     * Complete announcement
     */
    async completeAnnouncement(announcementId) {
        try {
            const announcement = this.announcements.find(a => a.announcement_id === announcementId);
            if (!announcement) {
                throw new Error('Announcement not found');
            }

            // Use announcement actions module
            const actions = new AnnouncementActions(announcementId, announcement.status, announcement);
            await actions.completeAnnouncement();

            // Reload announcements after action completes
            setTimeout(() => {
                this.loadAnnouncements();
            }, 2000);
        } catch (error) {
            console.error('Error completing announcement:', error);
        }
    }

    /**
     * Duplicate announcement
     */
    async duplicateAnnouncement(announcementId) {
        try {
            const announcement = this.announcements.find(a => a.announcement_id === announcementId);
            if (!announcement) {
                console.error('Announcement not found');
                return;
            }

            console.log('Duplicating announcement:', announcement);

            // Generate new announcement ID (format: TYPE-uuid)
            const announcementType = this.getAnnouncementType(announcement.object_type);
            const typePrefix = announcementType.toUpperCase();
            const uuid = this.generateUUID();
            const newAnnouncementId = `${typePrefix}-${uuid}`;

            // Create duplicated announcement with new ID and draft status
            const duplicated = {
                announcement_id: newAnnouncementId,
                object_type: announcement.object_type,
                status: 'draft',
                created_at: new Date().toISOString(),
                created_by: window.portal?.currentUser || 'Unknown',

                // Copy content fields
                title: announcement.title || '',
                summary: announcement.summary || '',
                content: announcement.content || '',

                // Copy customer targeting
                customers: announcement.customers || [],

                // Copy meeting details - INTENTIONALLY EXCLUDE meeting_id and join_url
                // Same reasoning as changes: backend generates these when approved
                meetingRequired: announcement.meetingRequired || 'no',
                meetingTitle: announcement.meetingTitle || '',
                meetingDate: announcement.meetingDate || '',
                meetingDuration: announcement.meetingDuration || '',
                meetingLocation: announcement.meetingLocation || '',
                attendees: announcement.attendees || '',

                // Copy metadata if exists
                metadata: announcement.metadata ? { ...announcement.metadata } : {},

                // Fresh modifications array
                modifications: [
                    {
                        timestamp: new Date().toISOString(),
                        user_id: window.portal?.currentUser || 'Unknown',
                        modification_type: 'created'
                    }
                ]
            };

            console.log('Saving duplicated announcement:', duplicated);

            // Save as draft via drafts API (same as changes)
            const response = await fetch(`${window.location.origin}/drafts`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                credentials: 'same-origin',
                body: JSON.stringify(duplicated)
            });

            if (!response.ok) {
                const errorBody = await response.text();
                console.error('Failed to save duplicated announcement:', response.status, errorBody);
                throw new Error(`Failed to save duplicated announcement: ${response.statusText} - ${errorBody}`);
            }

            console.log('Announcement duplicated successfully');

            // Redirect to edit page
            setTimeout(() => {
                window.location.href = `create-announcement.html?announcementId=${newAnnouncementId}&duplicate=true`;
            }, 500);

        } catch (error) {
            console.error('Error duplicating announcement:', error);
        }
    }

    /**
     * Generate UUID for announcement ID
     */
    generateUUID() {
        return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
            const r = Math.random() * 16 | 0;
            const v = c === 'x' ? r : (r & 0x3 | 0x8);
            return v.toString(16);
        });
    }

    /**
     * Show announcement details in modal
     */
    showAnnouncementDetails(announcement) {
        console.log('📖 Showing announcement details:', announcement.announcement_id);

        // Use the AnnouncementDetailsModal for consistency with approvals page
        if (typeof AnnouncementDetailsModal !== 'undefined') {
            // Store as global so close button can access it
            window.announcementDetailsModal = new AnnouncementDetailsModal(announcement);
            window.announcementDetailsModal.show();
        } else {
            console.error('AnnouncementDetailsModal not available');
            alert('Announcement details modal not available');
        }
    }

    /**
     * Format content (handle markdown or HTML)
     */
    formatContent(content) {
        if (!content) return '';

        // For now, just escape HTML and preserve line breaks
        // In a real implementation, you might want to use a markdown parser
        return this.escapeHtml(content).replace(/\n/g, '<br>');
    }

    /**
     * Render attachments section
     */
    renderAttachments(attachments) {
        if (!attachments || !Array.isArray(attachments) || attachments.length === 0) {
            return '';
        }

        const attachmentsList = attachments.map(att => `
            <li>
                <a href="${this.escapeHtml(att.url)}" target="_blank" rel="noopener noreferrer" style="color: #667eea; text-decoration: none;">
                    📎 ${this.escapeHtml(att.name)}
                </a>
            </li>
        `).join('');

        return `
            <div style="margin-bottom: 20px;">
                <strong>Attachments:</strong>
                <ul style="margin: 8px 0 0 20px;">
                    ${attachmentsList}
                </ul>
            </div>
        `;
    }

    /**
     * Render links section
     */
    renderLinks(links) {
        if (!links || !Array.isArray(links) || links.length === 0) {
            return '';
        }

        const linksList = links.map(link => `
            <li>
                <a href="${this.escapeHtml(link.url)}" target="_blank" rel="noopener noreferrer" style="color: #667eea; text-decoration: none;">
                    🔗 ${this.escapeHtml(link.text)}
                </a>
            </li>
        `).join('');

        return `
            <div style="margin-bottom: 20px;">
                <strong>Related Links:</strong>
                <ul style="margin: 8px 0 0 20px;">
                    ${linksList}
                </ul>
            </div>
        `;
    }

    /**
     * Render full tags list
     */
    renderFullTags(tags) {
        if (!tags || !Array.isArray(tags) || tags.length === 0) {
            return '';
        }

        const tagsList = tags.map(tag =>
            `<span class="tag">${this.escapeHtml(tag)}</span>`
        ).join('');

        return `
            <div style="margin-top: 20px; padding-top: 20px; border-top: 1px solid #e9ecef;">
                <strong>Tags:</strong>
                <div style="display: flex; gap: 6px; flex-wrap: wrap; margin-top: 8px;">
                    ${tagsList}
                </div>
            </div>
        `;
    }

    /**
     * Render empty state
     */
    renderEmptyState(message = null) {
        const container = document.getElementById('announcementsList');
        if (!container) return;

        const defaultMessage = this.filters.type !== 'all'
            ? `No ${this.getTypeName(this.filters.type)} announcements found.`
            : 'No announcements available at this time.';

        container.innerHTML = `
            <div class="empty-state">
                <div class="empty-state-icon">📢</div>
                <h3>${message || defaultMessage}</h3>
                <p>Check back later for updates and important communications.</p>
                ${this.filters.type !== 'all' ? '<button class="btn-primary" onclick="announcementsPage.clearFilters()">View All Announcements</button>' : ''}
            </div>
        `;
    }

    /**
     * Clear filters
     */
    clearFilters() {
        console.log('🧹 Clearing filters');

        // Reset filters, but preserve customer filter for non-admin users
        const customerFilter = (!this.userContext.isAdmin && this.userContext.customerCode)
            ? this.userContext.customerCode
            : 'all';

        this.filters = {
            type: 'all',
            sort: 'newest',
            customer: customerFilter
        };

        // Reset UI
        const typeFilter = document.getElementById('typeFilter');
        if (typeFilter) typeFilter.value = 'all';

        const sortFilter = document.getElementById('sortFilter');
        if (sortFilter) sortFilter.value = 'newest';

        const customerFilterEl = document.getElementById('customerFilter');
        if (customerFilterEl && this.userContext.isAdmin) {
            customerFilterEl.value = 'all';
        }

        // Re-apply filters
        this.applyFilters();
    }

    /**
     * Refresh announcements
     */
    async refresh() {
        console.log('🔄 Refreshing announcements');

        // Clear cache
        this.s3Client.clearCache('/announcements');

        // Reload
        await this.loadAnnouncements();
    }

    /**
     * Escape HTML to prevent XSS
     */
    escapeHtml(text) {
        if (!text) return '';

        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
}

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
    module.exports = { AnnouncementsPage };
}
