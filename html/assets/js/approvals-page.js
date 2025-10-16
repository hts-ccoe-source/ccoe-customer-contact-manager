/**
 * Approvals Page Module - Manage change approvals organized by customer
 * Provides functionality for viewing, filtering, and approving changes
 */

class ApprovalsPage {
    constructor() {
        this.changes = [];
        this.announcements = [];
        this.filteredChanges = [];
        this.filteredAnnouncements = [];
        this.customerGroups = {};
        this.filters = {
            status: 'pending',
            customer: 'all',
            dateRange: '',
            objectType: 'all' // 'all', 'change', 'announcement'
        };
        this.expandedCustomers = new Set();
        this.userContext = null; // Will store user role and customer info
        
        this.init();
    }

    async init() {
        console.log('ApprovalsPage initializing...');
        
        // Detect user context (admin vs customer user)
        await this.detectUserContext();
        
        // Setup event listeners
        this.setupEventListeners();
        
        // Load changes
        await this.loadChanges();
    }

    /**
     * Detect user context from authentication
     * Determines if user is admin or customer-specific
     */
    async detectUserContext() {
        // Note: /api/user/context endpoint doesn't exist yet
        // For now, use window.portal.currentUser and infer context
        
        try {
            // Check if user info is available in window.portal
            if (window.portal && window.portal.currentUser) {
                // Try to infer from email domain or user attributes
                this.userContext = this.inferUserContext(window.portal.currentUser);
                console.log('User context inferred from portal:', this.userContext);
            } else {
                // Default to admin for demo/development
                console.log('No user context available, defaulting to admin');
                this.userContext = {
                    isAdmin: true,
                    customerCode: null,
                    email: 'demo.user@hearst.com'
                };
            }
        } catch (error) {
            console.warn('Could not detect user context, defaulting to admin:', error);
            // Default to admin if detection fails
            this.userContext = {
                isAdmin: true,
                customerCode: null,
                email: window.portal?.currentUser || 'unknown'
            };
        }

        // Apply customer filter if user is not admin
        if (!this.userContext.isAdmin && this.userContext.customerCode) {
            this.filters.customer = this.userContext.customerCode;
            console.log(`Customer user detected, filtering to: ${this.userContext.customerCode}`);
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

    setupEventListeners() {
        // Status filter
        const statusFilter = document.getElementById('statusFilter');
        if (statusFilter) {
            statusFilter.addEventListener('change', (e) => {
                this.filters.status = e.target.value;
                this.applyFilters();
            });
        }

        // Customer filter
        const customerFilter = document.getElementById('customerFilter');
        if (customerFilter) {
            customerFilter.addEventListener('change', (e) => {
                this.filters.customer = e.target.value;
                this.applyFilters();
            });
        }

        // Date range filter
        const dateRangeFilter = document.getElementById('dateRangeFilter');
        if (dateRangeFilter) {
            dateRangeFilter.addEventListener('change', (e) => {
                this.filters.dateRange = e.target.value;
                this.applyFilters();
            });
        }

        // Object type filter
        const objectTypeFilter = document.getElementById('objectTypeFilter');
        if (objectTypeFilter) {
            objectTypeFilter.addEventListener('change', (e) => {
                this.filters.objectType = e.target.value;
                this.applyFilters();
            });
        }
    }

    /**
     * Load changes and announcements from S3
     */
    async loadChanges() {
        const container = document.getElementById('approvalsList');
        if (!container) return;

        try {
            // Show loading state
            container.innerHTML = `
                <div class="loading">
                    <div class="spinner"></div>
                    Loading changes and announcements...
                </div>
            `;

            console.log('Fetching objects from S3...');
            
            // Fetch objects based on user context
            let objects;
            if (!this.userContext.isAdmin && this.userContext.customerCode) {
                // Customer user - fetch only their objects
                console.log(`Fetching objects for customer: ${this.userContext.customerCode}`);
                objects = await s3Client.fetchCustomerChanges(this.userContext.customerCode);
            } else {
                // Admin user - fetch all objects
                console.log('Fetching all objects (admin view)');
                objects = await s3Client.fetchAllChanges();
            }
            
            console.log(`Loaded ${objects.length} objects from S3`);

            // Separate changes and announcements by object_type
            this.changes = s3Client.filterByObjectType(objects, 'change');
            
            // Filter announcements (object_type starts with "announcement_")
            this.announcements = objects.filter(obj => 
                obj.object_type && obj.object_type.startsWith('announcement_')
            );
            
            console.log(`Filtered to ${this.changes.length} change objects and ${this.announcements.length} announcement objects`);

            // Show user context banner
            this.showUserContextBanner();

            // Populate customer filter dropdown
            this.populateCustomerFilter();

            // Apply filters and render
            this.applyFilters();

        } catch (error) {
            console.error('Error loading objects:', error);
            
            container.innerHTML = `
                <div class="empty-state">
                    <div class="empty-state-icon">⚠️</div>
                    <h3>Error Loading Data</h3>
                    <p>${error.message}</p>
                    <button class="btn-primary" onclick="approvalsPage.refresh()">Try Again</button>
                </div>
            `;
        }
    }

    /**
     * Show user context banner to indicate viewing mode
     */
    showUserContextBanner() {
        const statusContainer = document.getElementById('statusContainer');
        if (!statusContainer) return;

        if (!this.userContext.isAdmin && this.userContext.customerCode) {
            // Show customer-specific banner
            statusContainer.innerHTML = `
                <div class="info-banner">
                    <span class="info-icon">ℹ️</span>
                    <span>Viewing changes for: <strong>${this.getCustomerName(this.userContext.customerCode)}</strong></span>
                </div>
            `;
        } else if (this.userContext.isAdmin) {
            // Show admin banner
            statusContainer.innerHTML = `
                <div class="info-banner admin-banner">
                    <span class="info-icon">👤</span>
                    <span>Admin View: You can see changes for all customers</span>
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

        // Get unique customers from changes
        const customers = new Set();
        this.changes.forEach(change => {
            if (Array.isArray(change.customers)) {
                change.customers.forEach(customer => customers.add(customer));
            } else if (change.customer) {
                customers.add(change.customer);
            }
        });

        // Sort customers alphabetically
        const sortedCustomers = Array.from(customers).sort();

        // If user is not admin, disable the filter and show only their customer
        if (!this.userContext.isAdmin && this.userContext.customerCode) {
            customerFilter.innerHTML = `<option value="${this.userContext.customerCode}">${this.getCustomerName(this.userContext.customerCode)}</option>`;
            customerFilter.disabled = true;
            customerFilter.title = 'You can only view changes for your organization';
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
     * Apply filters to changes and announcements
     */
    applyFilters() {
        // Determine which objects to show based on objectType filter
        let objectsToFilter = [];
        
        if (this.filters.objectType === 'change') {
            objectsToFilter = [...this.changes];
        } else if (this.filters.objectType === 'announcement') {
            objectsToFilter = [...this.announcements];
        } else {
            // 'all' - combine both
            objectsToFilter = [...this.changes, ...this.announcements];
        }

        // Filter by status
        if (this.filters.status && this.filters.status !== 'all') {
            if (this.filters.status === 'pending') {
                // Pending means submitted but not approved
                objectsToFilter = objectsToFilter.filter(obj => 
                    obj.status === 'submitted' || obj.status === 'pending' || obj.status === 'pending_approval'
                );
            } else {
                objectsToFilter = filterByStatus(objectsToFilter, this.filters.status);
            }
        }

        // Filter by customer
        if (this.filters.customer && this.filters.customer !== 'all') {
            objectsToFilter = filterByCustomer(objectsToFilter, this.filters.customer);
        }

        // Filter by date range
        if (this.filters.dateRange) {
            objectsToFilter = this.filterByDateRange(objectsToFilter, this.filters.dateRange);
        }

        // Separate filtered results back into changes and announcements
        this.filteredChanges = objectsToFilter.filter(obj => obj.object_type === 'change');
        this.filteredAnnouncements = objectsToFilter.filter(obj => 
            obj.object_type && obj.object_type.startsWith('announcement_')
        );

        // Group by customer and render
        this.groupByCustomer();
        this.render();
    }

    /**
     * Filter changes by date range
     */
    filterByDateRange(changes, range) {
        if (!range) return changes;

        const now = new Date();
        let startDate;

        switch (range) {
            case 'today':
                startDate = new Date(now.getFullYear(), now.getMonth(), now.getDate());
                break;
            case 'week':
                startDate = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000);
                break;
            case 'month':
                startDate = new Date(now.getFullYear(), now.getMonth(), 1);
                break;
            case 'quarter':
                const quarter = Math.floor(now.getMonth() / 3);
                startDate = new Date(now.getFullYear(), quarter * 3, 1);
                break;
            default:
                return changes;
        }

        return changes.filter(change => {
            const submittedDate = new Date(change.submittedAt || change.createdAt);
            return submittedDate >= startDate;
        });
    }

    /**
     * Group filtered changes and announcements by customer
     */
    groupByCustomer() {
        this.customerGroups = {};

        // Group changes
        this.filteredChanges.forEach(change => {
            const customers = Array.isArray(change.customers) 
                ? change.customers 
                : (change.customer ? [change.customer] : ['unknown']);

            customers.forEach(customer => {
                if (!this.customerGroups[customer]) {
                    this.customerGroups[customer] = [];
                }
                this.customerGroups[customer].push(change);
            });
        });

        // Group announcements
        this.filteredAnnouncements.forEach(announcement => {
            const customers = Array.isArray(announcement.customers) 
                ? announcement.customers 
                : (announcement.customer ? [announcement.customer] : ['unknown']);

            customers.forEach(customer => {
                if (!this.customerGroups[customer]) {
                    this.customerGroups[customer] = [];
                }
                this.customerGroups[customer].push(announcement);
            });
        });

        // Sort objects within each customer group by submission date (newest first)
        Object.keys(this.customerGroups).forEach(customer => {
            this.customerGroups[customer].sort((a, b) => {
                const dateA = new Date(a.submittedAt || a.created_at || a.createdAt || 0);
                const dateB = new Date(b.submittedAt || b.created_at || b.createdAt || 0);
                return dateB - dateA;
            });
        });
    }

    /**
     * Render the approvals list
     */
    render() {
        const container = document.getElementById('approvalsList');
        if (!container) return;

        // Check if there are any objects to display
        const totalObjects = this.filteredChanges.length + this.filteredAnnouncements.length;
        if (totalObjects === 0) {
            container.innerHTML = this.renderEmptyState();
            return;
        }

        // Render customer sections
        const customerCodes = Object.keys(this.customerGroups).sort();
        const html = customerCodes.map(customer => 
            this.renderCustomerSection(customer, this.customerGroups[customer])
        ).join('');

        container.innerHTML = html;
    }

    /**
     * Render empty state
     */
    renderEmptyState() {
        let message = 'No items found';
        let description = 'There are no changes or announcements matching your current filters.';

        if (this.filters.objectType === 'change') {
            message = 'No changes found';
            description = 'There are no changes matching your current filters.';
        } else if (this.filters.objectType === 'announcement') {
            message = 'No announcements found';
            description = 'There are no announcements matching your current filters.';
        }

        if (this.filters.status === 'pending') {
            message = 'No Pending Approvals';
            description = 'All items have been reviewed. Great job!';
        } else if (this.filters.status === 'approved') {
            message = 'No Approved Items';
            description = 'There are no approved items matching your filters.';
        }

        return `
            <div class="empty-state">
                <div class="empty-state-icon">📋</div>
                <h3>${message}</h3>
                <p>${description}</p>
                <button class="btn-secondary" onclick="approvalsPage.clearFilters()">Clear Filters</button>
            </div>
        `;
    }

    /**
     * Render a customer section with collapsible items (changes and announcements)
     */
    renderCustomerSection(customerCode, items) {
        const isExpanded = this.expandedCustomers.has(customerCode);
        const customerName = this.getCustomerName(customerCode);
        const pendingCount = items.filter(item => 
            item.status === 'submitted' || item.status === 'pending' || item.status === 'pending_approval'
        ).length;

        return `
            <div class="customer-section" role="region" aria-label="${customerName} items">
                <button class="customer-header ${isExpanded ? '' : 'collapsed'}" 
                     onclick="approvalsPage.toggleCustomer('${customerCode}')"
                     aria-expanded="${isExpanded}"
                     aria-controls="customer-changes-${customerCode}"
                     tabindex="0">
                    <div class="customer-info">
                        <span class="toggle-icon ${isExpanded ? 'expanded' : ''}" aria-hidden="true">▶</span>
                        <span class="customer-name">${customerName}</span>
                        <span class="customer-code">${customerCode}</span>
                        <span class="customer-count">(${pendingCount} pending)</span>
                    </div>
                </button>
                <div class="customer-changes ${isExpanded ? 'expanded' : ''}" 
                     id="customer-changes-${customerCode}"
                     role="list"
                     aria-label="Items for ${customerName}">
                    ${items.map(item => this.renderItemCard(item)).join('')}
                </div>
            </div>
        `;
    }

    /**
     * Render an item card (change or announcement)
     */
    renderItemCard(item) {
        const isAnnouncement = item.object_type && item.object_type.startsWith('announcement_');
        
        if (isAnnouncement) {
            return this.renderAnnouncementCard(item);
        } else {
            return this.renderChangeCard(item);
        }
    }

    /**
     * Render a change card
     */
    renderChangeCard(change) {
        const statusClass = this.getStatusClass(change.status);
        const statusLabel = this.getStatusLabel(change.status);
        const submittedDate = this.formatDate(change.submittedAt || change.createdAt);
        const submittedBy = change.submittedBy || change.createdBy || 'Unknown';
        const changeTitle = this.escapeHtml(change.title || change.changeTitle || 'Untitled Change');

        return `
            <div class="change-card" role="listitem" onclick="approvalsPage.viewDetails('${change.changeId || change.id}', event)" style="cursor: pointer;">
                <div class="change-header">
                    <div class="change-info">
                        <div class="change-title">
                            ${changeTitle}
                        </div>
                        <div class="change-id" aria-label="Change ID">${change.changeId || change.id || 'N/A'}</div>
                        <div class="change-meta">
                            <span><span aria-hidden="true">📅</span> <span class="sr-only">Submitted:</span> ${submittedDate}</span>
                            <span><span aria-hidden="true">👤</span> <span class="sr-only">By:</span> ${submittedBy}</span>
                        </div>
                    </div>
                    <div class="change-status ${statusClass}" role="status" aria-label="Status: ${statusLabel}">${statusLabel}</div>
                </div>
                ${this.renderChangeActions(change)}
            </div>
        `;
    }

    /**
     * Render an announcement card
     */
    renderAnnouncementCard(announcement) {
        const statusClass = this.getStatusClass(announcement.status);
        const statusLabel = this.getStatusLabel(announcement.status);
        const submittedDate = this.formatDate(announcement.submittedAt || announcement.created_at || announcement.createdAt);
        const submittedBy = announcement.submittedBy || announcement.created_by || announcement.createdBy || 'Unknown';
        const announcementTitle = this.escapeHtml(announcement.title || 'Untitled Announcement');
        const announcementType = this.getAnnouncementTypeLabel(announcement.announcement_type || announcement.object_type);
        const typeIcon = this.getAnnouncementTypeIcon(announcement.announcement_type || announcement.object_type);

        return `
            <div class="change-card announcement-card" role="listitem" onclick="approvalsPage.viewAnnouncementDetails('${announcement.announcement_id || announcement.id}', event)" style="cursor: pointer;">
                <div class="change-header">
                    <div class="change-info">
                        <div class="change-title">
                            <span class="announcement-type-icon" aria-hidden="true">${typeIcon}</span>
                            ${announcementTitle}
                        </div>
                        <div class="change-id" aria-label="Announcement ID">${announcement.announcement_id || announcement.id || 'N/A'}</div>
                        <div class="change-meta">
                            <span><span aria-hidden="true">📢</span> <span class="sr-only">Type:</span> ${announcementType}</span>
                            <span><span aria-hidden="true">📅</span> <span class="sr-only">Submitted:</span> ${submittedDate}</span>
                            <span><span aria-hidden="true">👤</span> <span class="sr-only">By:</span> ${submittedBy}</span>
                        </div>
                    </div>
                    <div class="change-status ${statusClass}" role="status" aria-label="Status: ${statusLabel}">${statusLabel}</div>
                </div>
                ${this.renderAnnouncementActions(announcement)}
            </div>
        `;
    }

    /**
     * Render action buttons for a change
     */
    renderChangeActions(change) {
        const isPending = change.status === 'submitted' || change.status === 'pending';
        const isApproved = change.status === 'approved';
        const changeTitle = this.escapeHtml(change.title || change.changeTitle || 'this change');

        return `
            <div class="change-actions" role="group" aria-label="Actions for ${changeTitle}" onclick="event.stopPropagation()">
                <button class="action-btn primary" 
                        onclick="approvalsPage.viewDetails('${change.changeId || change.id}')"
                        aria-label="View details for ${changeTitle}">
                    View Details
                </button>
                ${isPending ? `
                    <button class="action-btn cancel" 
                            onclick="approvalsPage.cancelChange('${change.changeId || change.id}')"
                            aria-label="Cancel ${changeTitle}">
                        💣 Cancel
                    </button>
                    <button class="action-btn approve" 
                            onclick="approvalsPage.approveChange('${change.changeId || change.id}')"
                            aria-label="Approve ${changeTitle}">
                        ✅ Approve
                    </button>
                ` : ''}
                ${isApproved ? `
                    <span style="color: #28a745; font-size: 0.9rem;" role="status" aria-label="Approved by ${change.approvedBy || 'Unknown'} on ${this.formatDate(change.approvedAt)}">
                        <span aria-hidden="true">✓</span> Approved by ${change.approvedBy || 'Unknown'} on ${this.formatDate(change.approvedAt)}
                    </span>
                ` : ''}
            </div>
        `;
    }

    /**
     * Render action buttons for an announcement using AnnouncementActions module
     */
    renderAnnouncementActions(announcement) {
        const announcementId = announcement.announcement_id || announcement.id;
        const announcementTitle = this.escapeHtml(announcement.title || 'this announcement');
        
        // Create AnnouncementActions instance
        const actions = new AnnouncementActions(
            announcementId,
            announcement.status,
            announcement
        );
        
        // Register global instance for onclick handlers
        actions.registerGlobal();
        
        // Get action buttons HTML
        const actionButtons = actions.renderActionButtons();
        
        return `
            <div class="change-actions" role="group" aria-label="Actions for ${announcementTitle}" onclick="event.stopPropagation()">
                <button class="action-btn primary" 
                        onclick="approvalsPage.viewAnnouncementDetails('${announcementId}')"
                        aria-label="View details for ${announcementTitle}">
                    View Details
                </button>
                ${actionButtons}
            </div>
        `;
    }

    /**
     * Toggle customer section expand/collapse
     */
    toggleCustomer(customerCode) {
        if (this.expandedCustomers.has(customerCode)) {
            this.expandedCustomers.delete(customerCode);
        } else {
            this.expandedCustomers.add(customerCode);
        }
        this.render();
    }

    /**
     * View change details in modal
     */
    async viewDetails(changeId, event) {
        // Stop event propagation if event is provided (from button clicks)
        if (event) {
            event.stopPropagation();
        }
        
        try {
            // Find the change in our data
            const change = this.changes.find(c => 
                (c.changeId || c.id) === changeId
            );

            if (!change) {
                console.error('Change not found:', changeId);
                return;
            }

            // Use the ChangeDetailsModal from change-details-modal.js
            if (typeof ChangeDetailsModal !== 'undefined') {
                const modal = new ChangeDetailsModal(change);
                modal.show();
            } else {
                console.error('ChangeDetailsModal not available');
                alert('Change details modal not available');
            }
        } catch (error) {
            console.error('Error viewing change details:', error);
            alert('Error loading change details');
        }
    }

    /**
     * Approve a change
     */
    async approveChange(changeId) {
        if (!confirm('Are you sure you want to approve this change?')) {
            return;
        }

        try {
            console.log('Approving change:', changeId);

            // Show loading state
            showInfo('statusContainer', 'Approving change...', { duration: 0 });

            // Find the change
            const change = this.changes.find(c => 
                (c.changeId || c.id) === changeId
            );

            if (!change) {
                throw new Error('Change not found');
            }

            // Create updated change object
            const updatedChange = {
                ...change,
                status: 'approved',
                approvedAt: new Date().toISOString(),
                approvedBy: window.portal?.currentUser || 'Unknown'
            };

            // Add modification entry
            if (!updatedChange.modifications) {
                updatedChange.modifications = [];
            }
            updatedChange.modifications.push({
                timestamp: updatedChange.approvedAt,
                user_id: updatedChange.approvedBy,
                modification_type: 'approved'
            });

            // Call the approve endpoint to trigger backend processing
            const response = await fetch(`${window.location.origin}/changes/${changeId}/approve`, {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                credentials: 'same-origin',
                body: JSON.stringify(updatedChange)
            });

            if (!response.ok) {
                const errorData = await response.json();
                throw new Error(errorData.error || 'Failed to approve change');
            }

            // Clear messages and show success
            clearMessages('statusContainer');
            showSuccess('statusContainer', 'Change approved successfully! Approval notification will be sent.');

            // Refresh the view
            await this.refresh();

        } catch (error) {
            console.error('Error approving change:', error);
            clearMessages('statusContainer');
            showError('statusContainer', `Error approving change: ${error.message}`);
        }
    }

    /**
     * Cancel a change
     */
    async cancelChange(changeId) {
        if (!confirm('Are you sure you want to cancel this change?')) {
            return;
        }

        try {
            console.log('Cancelling change:', changeId);

            // Show loading state
            showInfo('statusContainer', 'Cancelling change...', { duration: 0 });

            // Find the change
            const change = this.changes.find(c => 
                (c.changeId || c.id) === changeId
            );

            if (!change) {
                throw new Error('Change not found');
            }

            // Create updated change object
            const updatedChange = {
                ...change,
                status: 'cancelled',
                cancelledAt: new Date().toISOString(),
                cancelledBy: window.portal?.currentUser || 'Unknown'
            };

            // Add modification entry
            if (!updatedChange.modifications) {
                updatedChange.modifications = [];
            }
            updatedChange.modifications.push({
                timestamp: updatedChange.cancelledAt,
                user_id: updatedChange.cancelledBy,
                modification_type: 'cancelled'
            });

            // Update S3 object with new status
            await s3Client.updateChange(changeId, updatedChange);

            // Clear messages and show success
            clearMessages('statusContainer');
            showSuccess('statusContainer', 'Change cancelled successfully!');

            // Refresh the view
            await this.refresh();

        } catch (error) {
            console.error('Error cancelling change:', error);
            clearMessages('statusContainer');
            showError('statusContainer', `Error cancelling change: ${error.message}`);
        }
    }

    /**
     * View announcement details in modal
     */
    async viewAnnouncementDetails(announcementId, event) {
        // Stop event propagation if event is provided (from button clicks)
        if (event) {
            event.stopPropagation();
        }
        
        try {
            // Find the announcement in our data
            const announcement = this.announcements.find(a => 
                (a.announcement_id || a.id) === announcementId
            );

            if (!announcement) {
                console.error('Announcement not found:', announcementId);
                return;
            }

            // Use the AnnouncementDetailsModal
            if (typeof AnnouncementDetailsModal !== 'undefined') {
                announcementDetailsModal = new AnnouncementDetailsModal(announcement);
                announcementDetailsModal.show();
            } else {
                console.error('AnnouncementDetailsModal not available');
                alert('Announcement details modal not available');
            }
        } catch (error) {
            console.error('Error viewing announcement details:', error);
            alert('Error loading announcement details');
        }
    }

    /**
     * Note: Announcement action methods (approve, cancel, complete) are now handled
     * by the AnnouncementActions class in announcement-actions.js
     * The methods have been removed to avoid duplication and ensure consistency
     */

    /**
     * Get announcement type label
     */
    getAnnouncementTypeLabel(type) {
        if (!type) return 'General';
        
        // Handle both announcement_type field and object_type field
        const cleanType = type.replace('announcement_', '');
        
        const labels = {
            'cic': 'CIC (Cloud Innovation Center)',
            'finops': 'FinOps',
            'innersource': 'InnerSource Guild',
            'general': 'General'
        };
        
        return labels[cleanType.toLowerCase()] || cleanType;
    }

    /**
     * Get announcement type icon
     */
    getAnnouncementTypeIcon(type) {
        if (!type) return '📢';
        
        // Handle both announcement_type field and object_type field
        const cleanType = type.replace('announcement_', '');
        
        const icons = {
            'cic': '☁️',
            'finops': '💰',
            'innersource': '🔧',
            'general': '📢'
        };
        
        return icons[cleanType.toLowerCase()] || '📢';
    }

    /**
     * Clear all filters
     */
    clearFilters() {
        this.filters = {
            status: 'pending',
            customer: 'all',
            dateRange: '',
            objectType: 'all'
        };

        // Reset filter controls
        const statusFilter = document.getElementById('statusFilter');
        const customerFilter = document.getElementById('customerFilter');
        const dateRangeFilter = document.getElementById('dateRangeFilter');
        const objectTypeFilter = document.getElementById('objectTypeFilter');

        if (statusFilter) statusFilter.value = 'pending';
        if (customerFilter) customerFilter.value = 'all';
        if (dateRangeFilter) dateRangeFilter.value = '';
        if (objectTypeFilter) objectTypeFilter.value = 'all';

        this.applyFilters();
    }

    /**
     * Refresh the page data
     */
    async refresh() {
        // Clear cache and reload
        s3Client.clearCache();
        await this.loadChanges();
    }

    /**
     * Get customer friendly name
     */
    getCustomerName(customerCode) {
        // TODO: Map customer codes to friendly names
        // For now, just return the code in a more readable format
        const names = {
            'hts': 'HTS Prod',
            'cds': 'CDS Global',
            'fdbus': 'FDBUS',
            'unknown': 'Unknown Customer'
        };
        return names[customerCode.toLowerCase()] || customerCode.toUpperCase();
    }

    /**
     * Get status CSS class
     */
    getStatusClass(status) {
        const classes = {
            'draft': 'status-draft',
            'submitted': 'status-submitted',
            'pending': 'status-submitted',
            'approved': 'status-approved',
            'completed': 'status-completed',
            'cancelled': 'status-cancelled'
        };
        return classes[status] || 'status-draft';
    }

    /**
     * Get status label
     */
    getStatusLabel(status) {
        const labels = {
            'draft': 'Draft',
            'submitted': 'Pending Approval',
            'pending': 'Pending Approval',
            'approved': 'Approved',
            'completed': 'Completed',
            'cancelled': 'Cancelled'
        };
        return labels[status] || status;
    }

    /**
     * Format date for display
     */
    formatDate(dateString) {
        if (!dateString) return 'N/A';
        
        try {
            const date = new Date(dateString);
            return date.toLocaleDateString('en-US', {
                year: 'numeric',
                month: 'short',
                day: 'numeric',
                hour: '2-digit',
                minute: '2-digit'
            });
        } catch (error) {
            return dateString;
        }
    }

    /**
     * Escape HTML to prevent XSS
     */
    escapeHtml(text) {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }
}

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
    module.exports = { ApprovalsPage };
}
