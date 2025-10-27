/**
 * Announcements Page Module
 * Manages the announcements page functionality including loading, filtering, and displaying announcements
 */

class AnnouncementsPage {
    constructor() {
        this.announcements = [];
        this.filteredAnnouncements = [];
        this.currentStatus = 'draft'; // Default to drafts
        this.filters = {
            type: 'all',
            sort: 'newest',
            customer: 'all',
            dateRange: '14days',
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

        // Check URL parameters for initial status filter
        this.checkUrlParameters();

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
     * Check URL parameters and set initial filter state
     */
    checkUrlParameters() {
        const params = new URLSearchParams(window.location.search);
        const statusParam = params.get('status');

        if (statusParam) {
            console.log('URL parameter status:', statusParam);
            this.currentStatus = statusParam;
        }
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
            let archiveData;
            if (!this.userContext.isAdmin && this.userContext.customerCode) {
                // Customer user - fetch only their announcements
                console.log(`📥 Fetching announcements for customer: ${this.userContext.customerCode}`);
                archiveData = await this.s3Client.fetchCustomerAnnouncements(this.userContext.customerCode);
            } else {
                // Admin user - fetch all announcements
                console.log('📥 Fetching all announcements (admin view)');
                archiveData = await this.s3Client.fetchAnnouncements();
            }

            // Also fetch drafts
            let draftData = [];
            try {
                console.log('📥 Fetching announcement drafts...');
                const response = await fetch(`${window.location.origin}/drafts`, {
                    credentials: 'same-origin'
                });
                if (response.ok) {
                    const allDrafts = await response.json();
                    // Filter to only announcement drafts
                    draftData = allDrafts.filter(draft => {
                        const isAnnouncement = draft.announcement_id || draft.object_type?.startsWith('announcement_');
                        return isAnnouncement;
                    });
                    console.log(`✅ Loaded ${draftData.length} announcement drafts`);
                }
            } catch (error) {
                console.warn('⚠️  Could not load drafts:', error);
            }

            // Merge archive and draft data
            const allData = [...archiveData, ...draftData];

            // Filter by object_type starting with "announcement_"
            this.announcements = this.s3Client.filterByObjectType(allData, 'announcement_*');

            console.log(`✅ Loaded ${this.announcements.length} announcements`);

            // Show user context banner
            this.showUserContextBanner();

            // Populate customer filter dropdown (for admin users)
            this.populateCustomerFilter();

            // Apply filters and render
            this.applyFilters();

            // Start polling for meeting details on approved announcements without meeting info
            this.startPollingForApprovedAnnouncements();

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
                case '14days':
                    filterDate = new Date(now.getTime() - 14 * 24 * 60 * 60 * 1000);
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
    /**
     * Apply date filter to announcements (helper for counting)
     */
    applyDateFilter(announcements) {
        if (!this.filters.dateRange) {
            return announcements;
        }

        const now = new Date();
        let filterDate;

        switch (this.filters.dateRange) {
            case 'today':
                filterDate = new Date(now.getFullYear(), now.getMonth(), now.getDate());
                break;
            case 'week':
                filterDate = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000);
                break;
            case '14days':
                filterDate = new Date(now.getTime() - 14 * 24 * 60 * 60 * 1000);
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
            return announcements.filter(a => {
                const announcementDate = new Date(a.posted_date || a.created_date);
                return announcementDate >= filterDate;
            });
        }

        return announcements;
    }

    updateStatusCounts() {
        // Apply date filter to get the base set of announcements for counting
        const dateFilteredAnnouncements = this.applyDateFilter(this.announcements);
        
        const counts = {
            all: dateFilteredAnnouncements.length,
            draft: dateFilteredAnnouncements.filter(a => a.status === 'draft').length,
            submitted: dateFilteredAnnouncements.filter(a => a.status === 'submitted').length,
            approved: dateFilteredAnnouncements.filter(a => a.status === 'approved').length,
            completed: dateFilteredAnnouncements.filter(a => a.status === 'completed').length,
            cancelled: dateFilteredAnnouncements.filter(a => a.status === 'cancelled').length
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

        console.log('Status counts updated with date filter:', counts);
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
        // Backend uses nested meeting_metadata object
        const meetingMetadata = announcement.meeting_metadata;
        const joinUrl = meetingMetadata?.join_url;
        const isCompleted = announcement.status === 'completed';

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
            // Show edit button for owner to modify before approval
            buttons.push(`
                <a href="edit-announcement.html?announcementId=${announcement.announcement_id}" class="action-btn edit" onclick="event.stopPropagation()">
                    ✏️ Edit
                </a>
            `);
            // Show cancel button for owner (matching my-changes pattern)
            buttons.push(`
                <button class="action-btn cancel" onclick="event.stopPropagation(); announcementsPage.cancelAnnouncement('${announcement.announcement_id}')">
                    💣 Cancel
                </button>
            `);
            // Show approve button for admins (matching my-changes pattern)
            if (isAdmin) {
                buttons.push(`
                    <button class="action-btn approve" onclick="event.stopPropagation(); announcementsPage.approveAnnouncement('${announcement.announcement_id}')">
                        ✅ Approve
                    </button>
                `);
            }
            // Add Join button last (right-most) if meeting details exist
            if (joinUrl && !isCompleted) {
                buttons.push(`
                    <a href="${joinUrl}" 
                       target="_blank" 
                       rel="noopener noreferrer"
                       class="action-btn join-meeting" 
                       onclick="event.stopPropagation()"
                       aria-label="Join meeting for announcement">
                        🎥 Join
                    </a>
                `);
            }
        } else if (announcement.status === 'approved') {
            // Approved announcements can be cancelled or completed (matching changes pattern)
            buttons.push(`
                <button class="action-btn cancel" onclick="event.stopPropagation(); announcementsPage.cancelAnnouncement('${announcement.announcement_id}')">
                    💣 Cancel
                </button>
            `);
            buttons.push(`
                <button class="action-btn complete" onclick="event.stopPropagation(); announcementsPage.completeAnnouncement('${announcement.announcement_id}')">
                    🎯 Complete
                </button>
            `);
            // Add Join button last (right-most) if meeting details exist
            if (joinUrl && !isCompleted) {
                buttons.push(`
                    <a href="${joinUrl}" 
                       target="_blank" 
                       rel="noopener noreferrer"
                       class="action-btn join-meeting" 
                       onclick="event.stopPropagation()"
                       aria-label="Join meeting for announcement">
                        🎥 Join
                    </a>
                `);
            }
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
     * Update a single announcement in the local data and re-render only that card
     */
    updateSingleAnnouncement(updatedAnnouncement) {
        const announcementId = updatedAnnouncement.announcement_id || updatedAnnouncement.id;
        
        // Find and update the announcement in our local array
        const index = this.announcements.findIndex(a => (a.announcement_id || a.id) === announcementId);
        if (index !== -1) {
            this.announcements[index] = updatedAnnouncement;
            
            // Update filtered announcements if this announcement is in the filtered list
            const filteredIndex = this.filteredAnnouncements.findIndex(a => (a.announcement_id || a.id) === announcementId);
            if (filteredIndex !== -1) {
                this.filteredAnnouncements[filteredIndex] = updatedAnnouncement;
            }
            
            // Re-render only the affected card
            this.updateAnnouncementCard(updatedAnnouncement);
            
            return true;
        }
        return false;
    }

    /**
     * Update a specific announcement card in the DOM without full page refresh
     */
    updateAnnouncementCard(announcement) {
        const announcementId = announcement.announcement_id || announcement.id;
        
        // Find the card in the DOM
        const container = document.getElementById('announcementsList');
        if (!container) return;
        
        const cards = container.querySelectorAll('.announcement-card');
        let cardToUpdate = null;
        
        // Find the card by checking the announcement ID in the card content
        cards.forEach(card => {
            const idElement = card.querySelector('.change-id');
            if (idElement && idElement.textContent === announcementId) {
                cardToUpdate = card;
            }
        });
        
        if (!cardToUpdate) {
            console.log('Card not found in DOM, may be filtered out');
            return;
        }
        
        // Create new card and replace
        const newCard = this.renderAnnouncementCard(announcement);
        cardToUpdate.replaceWith(newCard);
        
        console.log(`✅ Updated card for announcement ${announcementId}`);
    }

    /**
     * Start watching for meeting details to be added after approval
     * Uses ETag-based polling to efficiently detect when backend adds meeting invite
     */
    startMeetingDetailsWatch(announcementId, options = {}) {
        const {
            initialIntervalMs = 2000,
            laterIntervalMs = 5000,
            maxDurationMs = 120000, // 2 minutes
            transitionTimeMs = 20000,
            initialDelayMs = 0 // Start immediately
        } = options;

        let lastETag = null;
        let elapsedMs = 0;
        let intervalMs = initialIntervalMs;
        let pollInterval = null;

        const pollForMeetingDetails = async () => {
            try {
                elapsedMs += intervalMs;

                // Switch to slower polling after transition time
                if (elapsedMs > transitionTimeMs && intervalMs === initialIntervalMs) {
                    intervalMs = laterIntervalMs;
                    clearInterval(pollInterval);
                    pollInterval = setInterval(pollForMeetingDetails, intervalMs);
                }

                // Stop polling after max duration
                if (elapsedMs >= maxDurationMs) {
                    clearInterval(pollInterval);
                    console.log('Meeting details watch timed out after', maxDurationMs, 'ms');
                    return;
                }

                // Fetch with ETag for conditional request
                const headers = {};
                if (lastETag) {
                    headers['If-None-Match'] = lastETag;
                }

                const response = await fetch(`${window.location.origin}/announcements/${announcementId}`, {
                    headers,
                    credentials: 'same-origin'
                });

                // 304 Not Modified - no changes yet
                if (response.status === 304) {
                    console.log('No changes detected (304), continuing to poll...');
                    return;
                }

                // 200 OK - data changed
                if (response.status === 200) {
                    lastETag = response.headers.get('ETag');
                    const updatedAnnouncement = await response.json();

                    // Check if meeting details were added (backend uses nested meeting_metadata)
                    if (updatedAnnouncement.meeting_metadata?.join_url) {
                        clearInterval(pollInterval);
                        console.log('Meeting details detected! Updating card...');

                        // Show success message
                        if (typeof showSuccess === 'function') {
                            showSuccess('statusContainer', 'Meeting scheduled! Join button is now available.');
                        }

                        // Update only this specific announcement (efficient, no full refresh)
                        this.updateSingleAnnouncement(updatedAnnouncement);
                    }
                }

            } catch (error) {
                console.error('Error polling for meeting details:', error);
                // Don't stop polling on error, just log it
            }
        };

        // Delay start by 1 minute to give backend time to process
        console.log(`Scheduling meeting details watch for announcement ${announcementId} (starting in ${initialDelayMs}ms)`);
        setTimeout(() => {
            console.log(`Starting meeting details watch for announcement ${announcementId}`);
            pollInterval = setInterval(pollForMeetingDetails, intervalMs);
            // Also do an immediate check after delay
            pollForMeetingDetails();
        }, initialDelayMs);
    }

    /**
     * Start polling for meeting details on all approved announcements that don't have them yet
     */
    startPollingForApprovedAnnouncements() {
        // Find all approved announcements without meeting details
        const approvedWithoutMeeting = this.announcements.filter(announcement => {
            if (announcement.status !== 'approved') return false;
            
            // Backend uses nested meeting_metadata object
            const meetingMetadata = announcement.meeting_metadata;
            const hasJoinUrl = meetingMetadata?.join_url;
            
            return !hasJoinUrl;
        });

        if (approvedWithoutMeeting.length > 0) {
            console.log(`📊 Found ${approvedWithoutMeeting.length} approved announcements without meeting details, starting polling`);
            
            approvedWithoutMeeting.forEach(announcement => {
                this.startMeetingDetailsWatch(announcement.announcement_id, {
                    initialIntervalMs: 2000,
                    laterIntervalMs: 5000,
                    maxDurationMs: 120000,
                    transitionTimeMs: 20000,
                    initialDelayMs: 0
                });
            });
        } else {
            console.log('✅ All approved announcements have meeting details or no approved announcements found');
        }
    }

    /**
     * Approve announcement
     */
    async approveAnnouncement(announcementId) {
        try {
            // CRITICAL: Reload announcement from S3 to get latest backend updates
            console.log('🔄 Reloading announcement from S3 before approval:', announcementId);

            let freshAnnouncement;
            try {
                const response = await fetch(`${window.location.origin}/announcements/${announcementId}`, {
                    method: 'GET',
                    credentials: 'same-origin'
                });

                if (!response.ok) {
                    throw new Error(`Failed to reload announcement: ${response.statusText}`);
                }

                freshAnnouncement = await response.json();
                console.log('📋 Fresh announcement status:', freshAnnouncement.status);
            } catch (error) {
                console.error('❌ Failed to reload announcement from S3:', error);
                throw new Error('Failed to reload announcement data');
            }

            // Check if already approved
            if (freshAnnouncement.status === 'approved') {
                console.log('✅ Announcement already approved, skipping approval');
                // Clear cache and reload to show current state
                this.s3Client.clearCache('/announcements');
                await this.loadAnnouncements();
                this.filterByStatus('approved');
                return;
            }

            // Use announcement actions module with fresh data
            const actions = new AnnouncementActions(announcementId, freshAnnouncement.status, freshAnnouncement);
            await actions.approveAnnouncement();

            // Update local array immediately
            const announcementIndex = this.announcements.findIndex(a => a.announcement_id === announcementId);
            if (announcementIndex !== -1) {
                this.announcements[announcementIndex].status = 'approved';
            }

            // Start watching for meeting details to be added by backend
            this.startMeetingDetailsWatch(announcementId, {
                initialIntervalMs: 2000,
                laterIntervalMs: 5000,
                maxDurationMs: 120000,
                transitionTimeMs: 20000,
                initialDelayMs: 0
            });

            // Clear cache and force fresh reload
            this.s3Client.clearCache('/announcements');
            
            // Wait a moment for backend to process
            await new Promise(resolve => setTimeout(resolve, 1000));
            
            // Force refresh without cache to get the latest data
            let archiveData;
            if (!this.userContext.isAdmin && this.userContext.customerCode) {
                archiveData = await this.s3Client.fetchCustomerAnnouncements(this.userContext.customerCode, { skipCache: true });
            } else {
                archiveData = await this.s3Client.fetchAnnouncements({ skipCache: true });
            }
            
            // Also reload drafts
            let draftData = [];
            try {
                const response = await fetch(`${window.location.origin}/drafts`, {
                    credentials: 'same-origin'
                });
                if (response.ok) {
                    const allDrafts = await response.json();
                    draftData = allDrafts.filter(draft => {
                        const isAnnouncement = draft.announcement_id || draft.object_type?.startsWith('announcement_');
                        return isAnnouncement;
                    });
                }
            } catch (error) {
                console.warn('Could not load drafts:', error);
            }
            
            // Update announcements and apply filters
            const allData = [...archiveData, ...draftData];
            this.announcements = this.s3Client.filterByObjectType(allData, 'announcement_*');
            this.applyFilters();
            this.filterByStatus('approved');
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

            // Clear cache and force fresh reload
            this.s3Client.clearCache('/announcements');
            
            // Wait a moment for backend to process
            await new Promise(resolve => setTimeout(resolve, 1000));
            
            // Force refresh without cache to get the latest data
            let archiveData;
            if (!this.userContext.isAdmin && this.userContext.customerCode) {
                archiveData = await this.s3Client.fetchCustomerAnnouncements(this.userContext.customerCode, { skipCache: true });
            } else {
                archiveData = await this.s3Client.fetchAnnouncements({ skipCache: true });
            }
            
            // Also reload drafts
            let draftData = [];
            try {
                const response = await fetch(`${window.location.origin}/drafts`, {
                    credentials: 'same-origin'
                });
                if (response.ok) {
                    const allDrafts = await response.json();
                    draftData = allDrafts.filter(draft => {
                        const isAnnouncement = draft.announcement_id || draft.object_type?.startsWith('announcement_');
                        return isAnnouncement;
                    });
                }
            } catch (error) {
                console.warn('Could not load drafts:', error);
            }
            
            // Update announcements and apply filters
            const allData = [...archiveData, ...draftData];
            this.announcements = this.s3Client.filterByObjectType(allData, 'announcement_*');
            this.applyFilters();
            this.filterByStatus('submitted');
        } catch (error) {
            console.error('Error submitting announcement:', error);
        }
    }

    /**
     * Delete announcement (draft or cancelled only)
     */
    async deleteAnnouncement(announcementId) {
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
                    console.log('Announcement already deleted or not found, removing from list...');
                }
                else {
                    throw new Error(`Failed to delete announcement: ${response.statusText}`);
                }
            }

            // Remove from local arrays
            this.announcements = this.announcements.filter(a => a.announcement_id !== announcementId);
            this.filteredAnnouncements = this.filteredAnnouncements.filter(a => a.announcement_id !== announcementId);

            // Update counts and re-render
            this.updateStatusCounts();
            this.render();

            // Clear cache for next load
            this.s3Client.clearCache('/announcements');

            console.log(`✅ Announcement ${announcementId} deleted and removed from list`);
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

            // Update local array immediately
            const announcementIndex = this.announcements.findIndex(a => a.announcement_id === announcementId);
            if (announcementIndex !== -1) {
                this.announcements[announcementIndex].status = 'cancelled';
            }

            // Clear cache and force fresh reload
            this.s3Client.clearCache('/announcements');
            
            // Wait a moment for backend to process
            await new Promise(resolve => setTimeout(resolve, 1000));
            
            // Force refresh without cache to get the latest data
            let archiveData;
            if (!this.userContext.isAdmin && this.userContext.customerCode) {
                archiveData = await this.s3Client.fetchCustomerAnnouncements(this.userContext.customerCode, { skipCache: true });
            } else {
                archiveData = await this.s3Client.fetchAnnouncements({ skipCache: true });
            }
            
            // Also reload drafts
            let draftData = [];
            try {
                const response = await fetch(`${window.location.origin}/drafts`, {
                    credentials: 'same-origin'
                });
                if (response.ok) {
                    const allDrafts = await response.json();
                    draftData = allDrafts.filter(draft => {
                        const isAnnouncement = draft.announcement_id || draft.object_type?.startsWith('announcement_');
                        return isAnnouncement;
                    });
                }
            } catch (error) {
                console.warn('Could not load drafts:', error);
            }
            
            // Update announcements and apply filters
            const allData = [...archiveData, ...draftData];
            this.announcements = this.s3Client.filterByObjectType(allData, 'announcement_*');
            this.applyFilters();
            this.filterByStatus('cancelled');
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

            // Update local array immediately
            const announcementIndex = this.announcements.findIndex(a => a.announcement_id === announcementId);
            if (announcementIndex !== -1) {
                this.announcements[announcementIndex].status = 'completed';
            }

            // Clear cache and force fresh reload
            this.s3Client.clearCache('/announcements');
            
            // Wait a moment for backend to process
            await new Promise(resolve => setTimeout(resolve, 1000));
            
            // Force refresh without cache to get the latest data
            let archiveData;
            if (!this.userContext.isAdmin && this.userContext.customerCode) {
                archiveData = await this.s3Client.fetchCustomerAnnouncements(this.userContext.customerCode, { skipCache: true });
            } else {
                archiveData = await this.s3Client.fetchAnnouncements({ skipCache: true });
            }
            
            // Also reload drafts
            let draftData = [];
            try {
                const response = await fetch(`${window.location.origin}/drafts`, {
                    credentials: 'same-origin'
                });
                if (response.ok) {
                    const allDrafts = await response.json();
                    draftData = allDrafts.filter(draft => {
                        const isAnnouncement = draft.announcement_id || draft.object_type?.startsWith('announcement_');
                        return isAnnouncement;
                    });
                }
            } catch (error) {
                console.warn('Could not load drafts:', error);
            }
            
            // Update announcements and apply filters
            const allData = [...archiveData, ...draftData];
            this.announcements = this.s3Client.filterByObjectType(allData, 'announcement_*');
            this.applyFilters();
            this.filterByStatus('completed');
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
                posted_date: new Date().toISOString(), // For date filtering

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
        const announcementId = announcement.announcement_id || announcement.id;
        console.log('📖 Showing announcement details:', announcementId);

        // Always get the latest announcement data from our array (in case it was updated by polling)
        const freshAnnouncement = this.announcements.find(a => 
            (a.announcement_id || a.id) === announcementId
        );

        // Use the AnnouncementDetailsModal for consistency with approvals page
        if (typeof AnnouncementDetailsModal !== 'undefined') {
            // Store as global so close button can access it
            window.announcementDetailsModal = new AnnouncementDetailsModal(freshAnnouncement || announcement);
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

        // Status-specific messages with clickable create action
        const statusMessages = {
            draft: {
                icon: '📝',
                title: 'No drafts found',
                message: 'You haven\'t saved any announcement drafts yet.',
                action: 'Create your first announcement to get started.',
                clickable: true
            },
            submitted: {
                icon: '📋',
                title: 'No submitted announcements',
                message: 'You haven\'t submitted any announcements for approval yet.',
                action: 'Create an announcement and submit it for approval.',
                clickable: true
            },
            approved: {
                icon: '✅',
                title: 'No approved announcements',
                message: 'You don\'t have any approved announcements waiting to be posted.',
                action: 'Create and submit an announcement to get it approved.',
                clickable: true
            },
            completed: {
                icon: '🎉',
                title: 'No completed announcements',
                message: 'You haven\'t posted any announcements yet.',
                action: 'Create an announcement and complete the posting.',
                clickable: true
            },
            cancelled: {
                icon: '❌',
                title: 'No cancelled announcements',
                message: 'You don\'t have any cancelled announcements.',
                action: 'Create a new announcement to get started.',
                clickable: true
            }
        };

        // Check if we have a status-specific message
        if (this.currentStatus && statusMessages[this.currentStatus]) {
            const msg = statusMessages[this.currentStatus];
            container.innerHTML = `
                <div class="empty-state clickable" onclick="window.location.href='create-announcement.html'" role="button" tabindex="0" onkeydown="if(event.key==='Enter'||event.key===' '){window.location.href='create-announcement.html'}">
                    <div class="empty-state-icon">${msg.icon}</div>
                    <h3>${msg.title}</h3>
                    <p>${msg.message}</p>
                    <p class="text-muted">${msg.action}</p>
                    <div class="btn-primary" style="display: inline-block; margin-top: 10px;">Create New Announcement</div>
                </div>
            `;
            return;
        }

        // Default empty state for "All" or other filters
        const defaultMessage = this.filters.type !== 'all'
            ? `No ${this.getTypeName(this.filters.type)} announcements found.`
            : 'No announcements available at this time.';

        container.innerHTML = `
            <div class="empty-state clickable" onclick="window.location.href='create-announcement.html'" role="button" tabindex="0" onkeydown="if(event.key==='Enter'||event.key===' '){window.location.href='create-announcement.html'}">
                <div class="empty-state-icon">📢</div>
                <h3>${message || defaultMessage}</h3>
                <p>Create your first announcement to get started.</p>
                <div class="btn-primary" style="display: inline-block; margin-top: 10px;">Create New Announcement</div>
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
