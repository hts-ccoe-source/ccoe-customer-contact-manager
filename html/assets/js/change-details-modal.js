/**
 * Enhanced Change Details Modal Component
 * Displays comprehensive change information including modification history,
 * meeting metadata, and approval status
 */

class ChangeDetailsModal {
    constructor(changeData) {
        this.changeData = changeData;
        this.modalElement = null;
    }

    /**
     * Render and display the modal
     */
    show() {
        // Create modal if it doesn't exist
        if (!this.modalElement) {
            this.modalElement = this.createModal();
            document.body.appendChild(this.modalElement);
        }

        // Populate content
        this.render();

        // Show modal with animation
        this.modalElement.style.display = 'flex';
        this.modalElement.style.position = 'fixed';
        this.modalElement.style.top = '0';
        this.modalElement.style.left = '0';
        this.modalElement.style.width = '100%';
        this.modalElement.style.height = '100%';
        this.modalElement.style.alignItems = 'center';
        this.modalElement.style.justifyContent = 'center';
        setTimeout(() => {
            this.modalElement.classList.add('show');
        }, 10);

        // Setup event listeners
        this.setupEventListeners();

        // Trap focus within modal
        this.trapFocus();
    }

    /**
     * Hide and cleanup the modal
     */
    hide() {
        if (!this.modalElement) return;

        this.modalElement.classList.remove('show');
        setTimeout(() => {
            this.modalElement.style.display = 'none';
            // Remove from DOM to cleanup
            if (this.modalElement.parentNode) {
                this.modalElement.parentNode.removeChild(this.modalElement);
            }
            this.modalElement = null;
        }, 300);
    }

    /**
     * Create the modal structure
     */
    createModal() {
        const modal = document.createElement('div');
        modal.className = 'change-details-modal';
        modal.innerHTML = `
            <div class="change-details-modal-overlay"></div>
            <div class="change-details-modal-content">
                <div class="change-details-modal-header">
                    <div class="change-details-modal-title-section">
                        <h3 class="change-details-modal-title"></h3>
                        <div class="change-details-modal-subtitle"></div>
                    </div>
                    <button class="change-details-modal-close" aria-label="Close modal">
                        <span aria-hidden="true">&times;</span>
                    </button>
                </div>
                <div class="change-details-modal-body">
                    <!-- Content will be populated by render() -->
                </div>
            </div>
        `;
        return modal;
    }

    /**
     * Render modal content
     */
    render() {
        if (!this.modalElement) return;

        const change = this.changeData;

        // Update title
        const titleEl = this.modalElement.querySelector('.change-details-modal-title');
        titleEl.textContent = change.changeTitle || change.title || 'Untitled Change';

        // Update subtitle with change ID and status
        const subtitleEl = this.modalElement.querySelector('.change-details-modal-subtitle');
        const changeId = change.changeId || change.change_id || 'N/A';
        const status = change.status || 'unknown';
        const statusBadge = this.renderStatusBadge(status);
        subtitleEl.innerHTML = `
            <span class="change-details-change-id">${this.escapeHtml(changeId)}</span>
            ${statusBadge}
        `;

        // Render body sections
        const bodyEl = this.modalElement.querySelector('.change-details-modal-body');
        bodyEl.innerHTML = `
            ${this.renderDetailsSection()}
            ${this.renderMeetingSection()}
            ${this.renderApprovalSection()}
            ${this.renderTimelineSection()}
        `;
    }

    /**
     * Render the details section
     */
    renderDetailsSection() {
        const change = this.changeData;
        
        return `
            <div class="change-details-section">
                <h4 class="change-details-section-title">
                    <span class="change-details-section-icon">📋</span>
                    Basic Information
                </h4>
                <div class="change-details-grid">
                    ${this.renderDetailItem('Version', change.version || '1')}
                    ${this.renderDetailItem('Created By', this.getUserDisplay(change.createdBy || change.created_by))}
                    ${this.renderDetailItem('Created At', this.formatTimestamp(change.createdAt || change.created_at || change.submittedAt))}
                </div>
                ${this.renderAffectedCustomers()}
                ${this.renderChangeReason()}
                ${this.renderDescription()}
                ${this.renderImplementationPlan()}
                ${this.renderTestPlan()}
                ${this.renderCustomerImpact()}
                ${this.renderRollbackPlan()}
                ${this.renderSchedule()}
            </div>
        `;
    }

    /**
     * Render change reason
     */
    renderChangeReason() {
        const reason = this.changeData.changeReason || this.changeData.change_reason;
        if (!reason) return '';

        return `
            <div class="change-details-field">
                <div class="change-details-label">Change Reason</div>
                <div class="change-details-text">${this.escapeHtml(reason)}</div>
            </div>
        `;
    }

    /**
     * Render description
     */
    renderDescription() {
        const description = this.changeData.description;
        if (!description) return '';

        return `
            <div class="change-details-field">
                <div class="change-details-label">Description</div>
                <div class="change-details-text">${this.escapeHtml(description)}</div>
            </div>
        `;
    }

    /**
     * Render implementation plan
     */
    renderImplementationPlan() {
        const plan = this.changeData.implementationPlan || this.changeData.implementation_plan;
        if (!plan) return '';

        return `
            <div class="change-details-field">
                <div class="change-details-label">Implementation Plan</div>
                <div class="change-details-text">${this.escapeHtml(plan)}</div>
            </div>
        `;
    }

    /**
     * Render test plan
     */
    renderTestPlan() {
        const plan = this.changeData.testPlan || this.changeData.test_plan;
        if (!plan) return '';

        return `
            <div class="change-details-field">
                <div class="change-details-label">Test Plan</div>
                <div class="change-details-text">${this.escapeHtml(plan)}</div>
            </div>
        `;
    }

    /**
     * Render customer impact
     */
    renderCustomerImpact() {
        const impact = this.changeData.customerImpact || this.changeData.customer_impact;
        if (!impact) return '';

        return `
            <div class="change-details-field">
                <div class="change-details-label">Customer Impact</div>
                <div class="change-details-text">${this.escapeHtml(impact)}</div>
            </div>
        `;
    }

    /**
     * Render rollback plan
     */
    renderRollbackPlan() {
        const plan = this.changeData.rollbackPlan || this.changeData.rollback_plan;
        if (!plan) return '';

        return `
            <div class="change-details-field">
                <div class="change-details-label">Rollback Plan</div>
                <div class="change-details-text">${this.escapeHtml(plan)}</div>
            </div>
        `;
    }

    /**
     * Render schedule information
     */
    renderSchedule() {
        const schedule = this.changeData.schedule;
        if (!schedule) return '';

        const startTime = schedule.startTime || schedule.start_time;
        const endTime = schedule.endTime || schedule.end_time;
        const timezone = schedule.timezone || 'UTC';

        if (!startTime) return '';

        return `
            <div class="change-details-field">
                <div class="change-details-label">Schedule</div>
                <div class="change-details-schedule">
                    <div class="change-details-schedule-time">
                        <strong>Start:</strong> ${this.formatTimestamp(startTime)}
                    </div>
                    ${endTime ? `
                        <div class="change-details-schedule-time">
                            <strong>End:</strong> ${this.formatTimestamp(endTime)}
                        </div>
                    ` : ''}
                    <div class="change-details-schedule-timezone">
                        <strong>Timezone:</strong> ${this.escapeHtml(timezone)}
                    </div>
                </div>
            </div>
        `;
    }

    /**
     * Render affected customers
     */
    renderAffectedCustomers() {
        const customers = this.changeData.customers || this.changeData.affectedCustomers || this.changeData.affected_customers;
        if (!customers || !Array.isArray(customers) || customers.length === 0) return '';

        const customerNames = this.changeData.customerNames || customers;
        const customerTags = customers.map((customer, index) => {
            const customerName = customerNames[index] || this.getCustomerName(customer);
            return `<span class="change-details-customer-tag">${this.escapeHtml(customerName)}</span>`;
        }).join('');

        return `
            <div class="change-details-field">
                <div class="change-details-label">Affected Customers</div>
                <div class="change-details-customer-tags">
                    ${customerTags}
                </div>
            </div>
        `;
    }

    /**
     * Render meeting information section
     */
    renderMeetingSection() {
        const modifications = this.changeData.modifications || [];
        const meetingMod = modifications.find(mod => 
            (mod.modificationType || mod.modification_type) === 'meeting_scheduled' &&
            (mod.meetingMetadata || mod.meeting_metadata)
        );

        if (!meetingMod) return '';

        const meeting = meetingMod.meetingMetadata || meetingMod.meeting_metadata;
        const joinUrl = meeting.joinUrl || meeting.join_url || meeting.onlineMeetingUrl || meeting.onlineMeeting?.joinUrl;
        const startTime = meeting.startTime || meeting.start_time || meeting.start?.dateTime;
        const endTime = meeting.endTime || meeting.end_time || meeting.end?.dateTime;
        const subject = meeting.subject || this.changeData.title;

        // Check if meeting is more than a day in the past
        const showJoinButton = this.isMeetingJoinable(startTime);

        return `
            <div class="change-details-section">
                <h4 class="change-details-section-title">
                    <span class="change-details-section-icon">📅</span>
                    Meeting Information
                </h4>
                <div class="change-details-meeting">
                    ${subject ? `<div class="change-details-meeting-subject">${this.escapeHtml(subject)}</div>` : ''}
                    ${startTime ? `
                        <div class="change-details-meeting-time">
                            <strong>Start:</strong> ${this.formatTimestamp(startTime)}
                        </div>
                    ` : ''}
                    ${endTime ? `
                        <div class="change-details-meeting-time">
                            <strong>End:</strong> ${this.formatTimestamp(endTime)}
                        </div>
                    ` : ''}
                    ${joinUrl && showJoinButton ? `
                        <div class="change-details-meeting-join">
                            <a href="${this.escapeHtml(joinUrl)}" target="_blank" rel="noopener noreferrer" class="change-details-meeting-link">
                                🔗 Join Meeting
                            </a>
                        </div>
                    ` : ''}
                </div>
            </div>
        `;
    }

    /**
     * Render approval status section
     */
    renderApprovalSection() {
        const modifications = this.changeData.modifications || [];
        const approvalMod = modifications.find(mod => 
            (mod.modificationType || mod.modification_type) === 'approved'
        );

        if (!approvalMod) return '';

        const approver = this.getUserDisplay(approvalMod.userId || approvalMod.user_id);
        const approvedAt = this.formatTimestamp(approvalMod.timestamp);
        const comments = approvalMod.comments || approvalMod.comment;

        return `
            <div class="change-details-section">
                <h4 class="change-details-section-title">
                    <span class="change-details-section-icon">✅</span>
                    Approval Status
                </h4>
                <div class="change-details-approval">
                    <div class="change-details-approval-info">
                        <div class="change-details-approval-item">
                            <strong>Approved by:</strong> ${this.escapeHtml(approver)}
                        </div>
                        <div class="change-details-approval-item">
                            <strong>Approved at:</strong> ${approvedAt}
                        </div>
                    </div>
                    ${comments ? `
                        <div class="change-details-approval-comments">
                            <strong>Comments:</strong>
                            <div class="change-details-text">${this.escapeHtml(comments)}</div>
                        </div>
                    ` : ''}
                </div>
            </div>
        `;
    }

    /**
     * Render modification history timeline
     */
    renderTimelineSection() {
        const modifications = this.changeData.modifications || [];
        if (!modifications || modifications.length === 0) return '';

        const timelineItems = modifications
            .sort((a, b) => new Date(a.timestamp) - new Date(b.timestamp))
            .map(mod => this.renderTimelineItem(mod))
            .join('');

        return `
            <div class="change-details-section">
                <h4 class="change-details-section-title">
                    <span class="change-details-section-icon">📊</span>
                    Modification History
                </h4>
                <div class="change-details-timeline">
                    ${timelineItems}
                </div>
            </div>
        `;
    }

    /**
     * Render a single timeline item
     */
    renderTimelineItem(modification) {
        const type = modification.modificationType || modification.modification_type || 'unknown';
        const timestamp = this.formatTimestamp(modification.timestamp);
        const user = this.getUserDisplay(modification.userId || modification.user_id);
        const icon = this.getModificationIcon(type);
        const label = this.getModificationLabel(type);

        return `
            <div class="change-details-timeline-item">
                <div class="change-details-timeline-marker">
                    <span class="change-details-timeline-icon">${icon}</span>
                </div>
                <div class="change-details-timeline-content">
                    <div class="change-details-timeline-header">
                        <span class="change-details-timeline-label">${label}</span>
                        <span class="change-details-timeline-user">by ${this.escapeHtml(user)}</span>
                    </div>
                    <div class="change-details-timeline-time">${timestamp}</div>
                </div>
            </div>
        `;
    }

    /**
     * Render a detail item in the grid
     */
    renderDetailItem(label, value) {
        if (!value) return '';
        return `
            <div class="change-details-detail-item">
                <div class="change-details-detail-label">${this.escapeHtml(label)}</div>
                <div class="change-details-detail-value">${this.escapeHtml(value)}</div>
            </div>
        `;
    }

    /**
     * Render status badge
     */
    renderStatusBadge(status) {
        const statusConfig = {
            draft: { icon: '📝', label: 'Draft', class: 'draft' },
            submitted: { icon: '📋', label: 'Submitted', class: 'submitted' },
            approved: { icon: '✅', label: 'Approved', class: 'approved' },
            completed: { icon: '🎉', label: 'Completed', class: 'completed' },
            cancelled: { icon: '❌', label: 'Cancelled', class: 'cancelled' }
        };

        const config = statusConfig[status] || { icon: '❓', label: status, class: 'unknown' };

        return `
            <span class="change-details-status-badge change-details-status-${config.class}">
                ${config.icon} ${config.label}
            </span>
        `;
    }

    /**
     * Get modification icon
     */
    getModificationIcon(type) {
        const icons = {
            created: '✨',
            updated: '✏️',
            submitted: '📤',
            approved: '✅',
            deleted: '🗑️',
            cancelled: '❌',
            meeting_scheduled: '📅',
            meeting_cancelled: '🚫',
            completed: '🎉'
        };
        return icons[type] || '●';
    }

    /**
     * Get modification label
     */
    getModificationLabel(type) {
        const labels = {
            created: 'Created',
            updated: 'Updated',
            submitted: 'Submitted',
            approved: 'Approved',
            deleted: 'Deleted',
            cancelled: 'Cancelled',
            meeting_scheduled: 'Meeting Scheduled',
            meeting_cancelled: 'Meeting Cancelled',
            completed: 'Completed'
        };
        return labels[type] || type;
    }

    /**
     * Check if a meeting is still joinable (not more than a day in the past)
     */
    isMeetingJoinable(meetingTime) {
        if (!meetingTime) return false;

        try {
            const meetingDate = new Date(meetingTime);
            if (isNaN(meetingDate.getTime())) return false;

            const now = new Date();
            const oneDayAgo = new Date(now.getTime() - (24 * 60 * 60 * 1000));

            // Meeting is joinable if it's not more than a day in the past
            return meetingDate >= oneDayAgo;
        } catch (error) {
            console.error('Error checking meeting joinability:', error);
            return false;
        }
    }

    /**
     * Format timestamp for display
     */
    formatTimestamp(timestamp) {
        if (!timestamp) return 'N/A';

        try {
            const date = new Date(timestamp);
            if (isNaN(date.getTime())) return timestamp;

            // Format: Oct 15, 2025 10:30 AM EDT
            const options = {
                year: 'numeric',
                month: 'short',
                day: 'numeric',
                hour: '2-digit',
                minute: '2-digit',
                timeZoneName: 'short'
            };

            return date.toLocaleString('en-US', options);
        } catch (error) {
            console.error('Error formatting timestamp:', error);
            return timestamp;
        }
    }

    /**
     * Get user display name
     */
    getUserDisplay(userId) {
        if (!userId) return 'Unknown';

        // Try to get friendly name from portal if available
        if (window.portal && window.portal.getUserFriendlyName) {
            return window.portal.getUserFriendlyName(userId);
        }

        return userId;
    }

    /**
     * Get customer display name
     */
    getCustomerName(customerCode) {
        if (!customerCode) return 'Unknown';

        // Try to get friendly name from portal if available
        if (window.portal && window.portal.getCustomerFriendlyName) {
            return window.portal.getCustomerFriendlyName(customerCode);
        }

        return customerCode;
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

    /**
     * Setup event listeners
     */
    setupEventListeners() {
        if (!this.modalElement) return;

        // Close button
        const closeBtn = this.modalElement.querySelector('.change-details-modal-close');
        if (closeBtn) {
            closeBtn.addEventListener('click', () => this.hide());
        }

        // Overlay click
        const overlay = this.modalElement.querySelector('.change-details-modal-overlay');
        if (overlay) {
            overlay.addEventListener('click', () => this.hide());
        }

        // ESC key
        this.escapeHandler = (e) => {
            if (e.key === 'Escape') {
                this.hide();
            }
        };
        document.addEventListener('keydown', this.escapeHandler);
    }

    /**
     * Trap focus within modal for accessibility
     */
    trapFocus() {
        if (!this.modalElement) return;

        const focusableElements = this.modalElement.querySelectorAll(
            'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
        );

        if (focusableElements.length === 0) return;

        const firstElement = focusableElements[0];
        const lastElement = focusableElements[focusableElements.length - 1];

        // Focus first element
        firstElement.focus();

        // Trap focus
        this.tabHandler = (e) => {
            if (e.key !== 'Tab') return;

            if (e.shiftKey) {
                if (document.activeElement === firstElement) {
                    e.preventDefault();
                    lastElement.focus();
                }
            } else {
                if (document.activeElement === lastElement) {
                    e.preventDefault();
                    firstElement.focus();
                }
            }
        };

        this.modalElement.addEventListener('keydown', this.tabHandler);
    }
}

// Export for use in other scripts
if (typeof module !== 'undefined' && module.exports) {
    module.exports = ChangeDetailsModal;
}
