/**
 * Announcements Page Module
 * Manages the announcements page functionality including loading, filtering, and displaying announcements
 */

class AnnouncementsPage {
    constructor() {
        this.announcements = [];
        this.filteredAnnouncements = [];
        this.filters = {
            type: 'all',
            sort: 'newest',
            customer: 'all'
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
        
        // Set up event listeners
        this.setupEventListeners();
        
        // Load announcements
        await this.loadAnnouncements();
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
        console.log('🔍 Applying filters:', this.filters);
        
        let filtered = [...this.announcements];

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
        this.render();
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
        const announcementTitle = this.escapeHtml(announcement.title || 'Untitled Announcement');
        
        // Format date
        const postedDate = announcement.posted_date 
            ? new Date(announcement.posted_date).toLocaleDateString('en-US', {
                year: 'numeric',
                month: 'short',
                day: 'numeric'
            })
            : 'Unknown date';

        card.setAttribute('aria-label', `${typeName} announcement: ${announcementTitle}, posted ${postedDate}`);

        card.innerHTML = `
            <div class="announcement-header">
                <div class="announcement-icon ${type}" aria-hidden="true">${icon}</div>
                <div class="announcement-content">
                    <h3 class="announcement-title">${announcementTitle}</h3>
                    <div class="announcement-meta">
                        <span><span aria-hidden="true">📅</span> <span class="sr-only">Posted:</span> ${postedDate}</span>
                        <span class="announcement-type-badge ${type}" role="status" aria-label="Type: ${typeName}">${typeName}</span>
                        ${announcement.author ? `<span><span aria-hidden="true">👤</span> <span class="sr-only">Author:</span> ${this.escapeHtml(announcement.author)}</span>` : ''}
                    </div>
                </div>
            </div>
            <div class="announcement-summary">
                ${this.escapeHtml(announcement.summary || 'No summary available.')}
            </div>
            <div class="announcement-footer">
                <div class="announcement-tags" role="list" aria-label="Tags">
                    ${this.renderTags(announcement.tags)}
                </div>
                <button class="read-more-btn" aria-label="Read more about ${announcementTitle}">Read More</button>
            </div>
        `;

        // Add click handler for "Read More"
        const readMoreBtn = card.querySelector('.read-more-btn');
        readMoreBtn.addEventListener('click', (e) => {
            e.stopPropagation();
            this.showAnnouncementDetails(announcement);
        });

        // Make entire card clickable
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
            'cic': 'CIC / Cloud Enablement',
            'general': 'General'
        };
        return names[type] || 'General';
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
     * Show announcement details in modal
     */
    showAnnouncementDetails(announcement) {
        console.log('📖 Showing announcement details:', announcement.announcement_id);
        
        const type = this.getAnnouncementType(announcement.object_type);
        const icon = this.getTypeIcon(type);
        const typeName = this.getTypeName(type);
        
        // Format date
        const postedDate = announcement.posted_date 
            ? new Date(announcement.posted_date).toLocaleDateString('en-US', {
                year: 'numeric',
                month: 'long',
                day: 'numeric'
            })
            : 'Unknown date';

        // Build modal content
        let content = `
            <div style="padding: 20px;">
                <div style="display: flex; align-items: center; gap: 15px; margin-bottom: 20px;">
                    <div style="font-size: 3rem;">${icon}</div>
                    <div>
                        <h2 style="margin: 0 0 8px 0; color: #2c3e50;">${this.escapeHtml(announcement.title || 'Untitled Announcement')}</h2>
                        <div style="display: flex; gap: 15px; font-size: 0.9rem; color: #6c757d;">
                            <span>📅 ${postedDate}</span>
                            <span class="announcement-type-badge ${type}">${typeName}</span>
                            ${announcement.author ? `<span>👤 ${this.escapeHtml(announcement.author)}</span>` : ''}
                        </div>
                    </div>
                </div>
                
                <div style="margin-bottom: 20px; padding: 15px; background: #f8f9fa; border-radius: 8px;">
                    <strong>Summary:</strong>
                    <p style="margin: 8px 0 0 0;">${this.escapeHtml(announcement.summary || 'No summary available.')}</p>
                </div>
                
                ${announcement.content ? `
                    <div style="margin-bottom: 20px; line-height: 1.6;">
                        <strong>Details:</strong>
                        <div style="margin-top: 10px;">${this.formatContent(announcement.content)}</div>
                    </div>
                ` : ''}
                
                ${this.renderAttachments(announcement.attachments)}
                ${this.renderLinks(announcement.links)}
                ${this.renderFullTags(announcement.tags)}
            </div>
        `;

        // Create and show modal
        const modal = new Modal({
            title: 'Announcement Details',
            content: content,
            size: 'large'
        });
        
        modal.show();
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
        this.s3Client.clearCache('/api/announcements');
        
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
