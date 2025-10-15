/**
 * Approvals Page Module - Manage change approvals organized by customer
 * Provides functionality for viewing, filtering, and approving changes
 */

class ApprovalsPage {
    constructor() {
        this.changes = [];
        this.filteredChanges = [];
        this.customerGroups = {};
        this.filters = {
            status: 'pending',
            customer: 'all',
            dateRange: ''
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
    }

    /**
     * Load changes from S3 and filter by object_type "change"
     */
    async loadChanges() {
        const container = document.getElementById('approvalsList');
        if (!container) return;

        try {
            // Show loading state
            container.innerHTML = `
                <div class="loading">
                    <div class="spinner"></div>
                    Loading changes...
                </div>
            `;

            console.log('Fetching changes from S3...');
            
            // Fetch changes based on user context
            let changes;
            if (!this.userContext.isAdmin && this.userContext.customerCode) {
                // Customer user - fetch only their changes
                console.log(`Fetching changes for customer: ${this.userContext.customerCode}`);
                changes = await s3Client.fetchCustomerChanges(this.userContext.customerCode);
            } else {
                // Admin user - fetch all changes
                console.log('Fetching all changes (admin view)');
                changes = await s3Client.fetchAllChanges();
            }
            
            console.log(`Loaded ${changes.length} changes from S3`);

            // Filter by object_type "change"
            this.changes = s3Client.filterByObjectType(changes, 'change');
            
            console.log(`Filtered to ${this.changes.length} change objects`);

            // Show user context banner
            this.showUserContextBanner();

            // Populate customer filter dropdown
            this.populateCustomerFilter();

            // Apply filters and render
            this.applyFilters();

        } catch (error) {
            console.error('Error loading changes:', error);
            
            container.innerHTML = `
                <div class="empty-state">
                    <div class="empty-state-icon">⚠️</div>
                    <h3>Error Loading Changes</h3>
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
     * Apply filters to changes
     */
    applyFilters() {
        let filtered = [...this.changes];

        // Filter by status
        if (this.filters.status && this.filters.status !== 'all') {
            if (this.filters.status === 'pending') {
                // Pending means submitted but not approved
                filtered = filtered.filter(change => 
                    change.status === 'submitted' || change.status === 'pending'
                );
            } else {
                filtered = filterByStatus(filtered, this.filters.status);
            }
        }

        // Filter by customer
        if (this.filters.customer && this.filters.customer !== 'all') {
            filtered = filterByCustomer(filtered, this.filters.customer);
        }

        // Filter by date range
        if (this.filters.dateRange) {
            filtered = this.filterByDateRange(filtered, this.filters.dateRange);
        }

        this.filteredChanges = filtered;

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
     * Group filtered changes by customer
     */
    groupByCustomer() {
        this.customerGroups = {};

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

        // Sort changes within each customer group by submission date (newest first)
        Object.keys(this.customerGroups).forEach(customer => {
            this.customerGroups[customer].sort((a, b) => {
                const dateA = new Date(a.submittedAt || a.createdAt || 0);
                const dateB = new Date(b.submittedAt || b.createdAt || 0);
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

        // Check if there are any changes to display
        if (this.filteredChanges.length === 0) {
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
        let message = 'No changes found';
        let description = 'There are no changes matching your current filters.';

        if (this.filters.status === 'pending') {
            message = 'No Pending Approvals';
            description = 'All changes have been reviewed. Great job!';
        } else if (this.filters.status === 'approved') {
            message = 'No Approved Changes';
            description = 'There are no approved changes matching your filters.';
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
     * Render a customer section with collapsible changes
     */
    renderCustomerSection(customerCode, changes) {
        const isExpanded = this.expandedCustomers.has(customerCode);
        const customerName = this.getCustomerName(customerCode);
        const pendingCount = changes.filter(c => 
            c.status === 'submitted' || c.status === 'pending'
        ).length;

        return `
            <div class="customer-section" role="region" aria-label="${customerName} changes">
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
                     aria-label="Changes for ${customerName}">
                    ${changes.map(change => this.renderChangeCard(change)).join('')}
                </div>
            </div>
        `;
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

            // Update S3 object with new status
            await s3Client.updateChange(changeId, updatedChange);

            // Clear messages and show success
            clearMessages('statusContainer');
            showSuccess('statusContainer', 'Change approved successfully!');

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
     * Clear all filters
     */
    clearFilters() {
        this.filters = {
            status: 'pending',
            customer: 'all',
            dateRange: ''
        };

        // Reset filter controls
        const statusFilter = document.getElementById('statusFilter');
        const customerFilter = document.getElementById('customerFilter');
        const dateRangeFilter = document.getElementById('dateRangeFilter');

        if (statusFilter) statusFilter.value = 'pending';
        if (customerFilter) customerFilter.value = 'all';
        if (dateRangeFilter) dateRangeFilter.value = '';

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
