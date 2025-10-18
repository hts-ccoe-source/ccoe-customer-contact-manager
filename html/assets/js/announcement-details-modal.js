/**
 * Announcement Details Modal Component
 * Displays comprehensive announcement information including content,
 * attachments, meeting metadata, and action buttons
 */

class AnnouncementDetailsModal {
    constructor(announcementData) {
        this.announcementData = announcementData;
        this.modalElement = null;
        this.announcementActions = null;
    }

    /**
     * Render and display the modal
     */
    show() {
        try {
            console.log('🔍 AnnouncementDetailsModal.show() called');
            
            // Create modal if it doesn't exist
            if (!this.modalElement) {
                console.log('📝 Creating modal element');
                this.modalElement = this.createModal();
                document.body.appendChild(this.modalElement);
                console.log('✅ Modal element created and appended');
            }

            // Populate content
            console.log('📝 Rendering modal content');
            this.render();
            console.log('✅ Modal content rendered');

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
            console.log('📝 Setting up event listeners');
            this.setupEventListeners();

            // Trap focus within modal
            console.log('📝 Trapping focus');
            this.trapFocus();
            
            console.log('✅ Modal should now be visible');
        } catch (error) {
            console.error('❌ Error in show():', error);
            alert('Error showing modal: ' + error.message);
        }
    }

    /**
     * Hide and cleanup the modal
     */
    hide() {
        if (!this.modalElement) return;

        // Cleanup announcement actions instance
        if (this.announcementActions) {
            this.announcementActions.unregisterGlobal();
            this.announcementActions = null;
        }

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
        modal.className = 'change-details-modal announcement-details-modal';
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
                <div class="change-details-modal-footer">
                    <!-- Action buttons will be populated by render() -->
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

        const announcement = this.announcementData;

        // Update title
        const titleEl = this.modalElement.querySelector('.change-details-modal-title');
        const typeIcon = this.getAnnouncementTypeIcon(announcement.announcement_type || announcement.object_type);
        titleEl.innerHTML = `<span class="announcement-type-icon">${typeIcon}</span> ${this.escapeHtml(announcement.title || 'Untitled Announcement')}`;

        // Update subtitle with announcement ID and status
        const subtitleEl = this.modalElement.querySelector('.change-details-modal-subtitle');
        const announcementId = announcement.announcement_id || announcement.id || 'N/A';
        const status = announcement.status || 'unknown';
        const statusBadge = this.renderStatusBadge(status);
        const typeLabel = this.getAnnouncementTypeLabel(announcement.announcement_type || announcement.object_type);
        subtitleEl.innerHTML = `
            <span class="change-details-change-id">${this.escapeHtml(announcementId)}</span>
            <span class="announcement-type-label">${this.escapeHtml(typeLabel)}</span>
            ${statusBadge}
        `;

        // Render body sections
        const bodyEl = this.modalElement.querySelector('.change-details-modal-body');
        
        try {
            const statusBar = this.renderStatusProgressBar();
            const details = this.renderDetailsSection();
            const content = this.renderContentSection();
            const attachments = this.renderAttachmentsSection();
            const meeting = this.renderMeetingSection();
            const timeline = this.renderTimelineSection();
            
            bodyEl.innerHTML = `
                ${statusBar}
                ${details}
                ${content}
                ${attachments}
                ${meeting}
                ${timeline}
            `;
            
            // Add click handlers to next step
            const nextStepEl = bodyEl.querySelector('.status-progress-step[data-next-status]');
            if (nextStepEl) {
                const nextStatus = nextStepEl.getAttribute('data-next-status');
                nextStepEl.addEventListener('click', () => this.advanceStatus(nextStatus));
            }
        } catch (error) {
            console.error('Error rendering modal body:', error);
            bodyEl.innerHTML = `
                <div class="error-message" style="padding: 20px; color: #dc3545;">
                    <h4>Error rendering modal</h4>
                    <p>${error.message}</p>
                </div>
            `;
        }

        // Render action buttons in footer
        this.renderActionButtons();
    }

    /**
     * Render the status progress bar
     */
    renderStatusProgressBar() {
        try {
            const announcement = this.announcementData;
            const currentStatus = announcement.status || 'draft';
            
            // Define status progression with both action and past tense labels
            const statuses = [
                { key: 'draft', actionLabel: 'Draft', completedLabel: 'Draft', icon: '📝' },
                { key: 'submitted', actionLabel: 'Submit', completedLabel: 'Submitted', icon: '📤' },
                { key: 'approved', actionLabel: 'Approve', completedLabel: 'Approved', icon: '✅' },
                { key: 'completed', actionLabel: 'Complete', completedLabel: 'Completed', icon: '🎯' }
            ];
            
            // Handle cancelled status separately
            if (currentStatus === 'cancelled') {
                return '<div class="status-progress-bar cancelled"><div class="status-progress-step active cancelled"><div class="status-progress-icon">❌</div><div class="status-progress-label">Cancelled</div></div></div>';
            }
            
            // Find current status index
            const currentIndex = statuses.findIndex(s => s.key === currentStatus);
            
            // Build steps HTML
            let stepsHtml = '';
            for (let i = 0; i < statuses.length; i++) {
                const status = statuses[i];
                const isActive = i === currentIndex;
                const isCompleted = i < currentIndex;
                const isNextStep = i === currentIndex + 1;
                
                // Use past tense for completed/active steps, action verb for future steps
                const label = (isCompleted || isActive) ? status.completedLabel : status.actionLabel;
                
                let stateClass = isActive ? 'active' : (isCompleted ? 'completed' : '');
                if (isNextStep) {
                    stateClass += ' next-step clickable';
                }
                // Add final-step class when on the completed status
                if (isActive && status.key === 'completed') {
                    stateClass += ' final-step';
                }
                
                const dataAttr = isNextStep ? ` data-next-status="${status.key}"` : '';
                const cursorStyle = isNextStep ? ' style="cursor: pointer;"' : '';
                
                stepsHtml += '<div class="status-progress-step ' + stateClass + '"' + dataAttr + cursorStyle + '>';
                stepsHtml += '<div class="status-progress-icon">' + status.icon + '</div>';
                stepsHtml += '<div class="status-progress-label">' + label + '</div>';
                stepsHtml += '</div>';
                
                // Add connector between steps
                if (i < statuses.length - 1) {
                    const connectorClass = isCompleted ? 'completed' : '';
                    stepsHtml += '<div class="status-progress-connector ' + connectorClass + '"></div>';
                }
            }
            
            return '<div class="status-progress-bar">' + stepsHtml + '</div>';
        } catch (error) {
            console.error('Error in renderStatusProgressBar:', error);
            return '<div class="status-progress-bar"><div style="padding: 10px; color: red;">Error rendering status bar</div></div>';
        }
    }

    /**
     * Render the details section
     */
    renderDetailsSection() {
        const announcement = this.announcementData;
        
        return `
            <div class="change-details-section">
                <h4 class="change-details-section-title">
                    <span class="change-details-section-icon">📋</span>
                    Basic Information
                </h4>
                <div class="change-details-grid">
                    ${this.renderDetailItem('Created By', this.getUserDisplay(announcement.created_by || announcement.createdBy))}
                    ${this.renderDetailItem('Created At', this.formatTimestamp(announcement.created_at || announcement.createdAt))}
                    ${announcement.submitted_at || announcement.submittedAt ? this.renderDetailItem('Submitted At', this.formatTimestamp(announcement.submitted_at || announcement.submittedAt)) : ''}
                    ${announcement.status === 'approved' || announcement.status === 'completed' ? this.renderDetailItem('Approved By', this.getUserDisplay(announcement.approvedBy || announcement.approved_by)) : ''}
                    ${announcement.status === 'approved' || announcement.status === 'completed' ? this.renderDetailItem('Approved At', this.formatTimestamp(announcement.approvedAt || announcement.approved_at)) : ''}
                </div>
                ${this.renderAffectedCustomers()}
                ${this.renderSummary()}
            </div>
        `;
    }

    /**
     * Render the content section
     */
    renderContentSection() {
        const announcement = this.announcementData;
        const content = announcement.content || announcement.description || '';
        
        if (!content) return '';

        return `
            <div class="change-details-section">
                <h4 class="change-details-section-title">
                    <span class="change-details-section-icon">📄</span>
                    Content
                </h4>
                <div class="change-details-content">
                    ${this.formatContent(content)}
                </div>
            </div>
        `;
    }

    /**
     * Render the attachments section
     */
    renderAttachmentsSection() {
        const announcement = this.announcementData;
        const attachments = announcement.attachments || [];
        
        if (!attachments || attachments.length === 0) return '';

        return `
            <div class="change-details-section">
                <h4 class="change-details-section-title">
                    <span class="change-details-section-icon">📎</span>
                    Attachments
                </h4>
                <div class="attachments-list">
                    ${attachments.map(attachment => this.renderAttachment(attachment)).join('')}
                </div>
            </div>
        `;
    }

    /**
     * Render a single attachment
     */
    renderAttachment(attachment) {
        const name = attachment.name || 'Unnamed file';
        const size = attachment.size ? this.formatFileSize(attachment.size) : '';
        const url = attachment.url || attachment.s3_key || '#';
        
        return `
            <div class="attachment-item">
                <span class="attachment-icon">📄</span>
                <a href="${this.escapeHtml(url)}" target="_blank" class="attachment-link">
                    ${this.escapeHtml(name)}
                </a>
                ${size ? `<span class="attachment-size">(${size})</span>` : ''}
            </div>
        `;
    }

    /**
     * Render the meeting section
     */
    renderMeetingSection() {
        const announcement = this.announcementData;
        const meetingMetadata = announcement.meeting_metadata || announcement.meetingMetadata;
        
        // Check if meeting is included (either via include_meeting flag or meetingRequired field)
        const includeMeeting = announcement.include_meeting || announcement.meetingRequired === 'yes';
        
        if (!includeMeeting) return '';

        // Build meeting information from either meeting_metadata or top-level fields
        let meetingInfo = '';
        
        if (meetingMetadata) {
            // Use meeting_metadata if available (for approved/scheduled meetings)
            meetingInfo = `
                ${meetingMetadata.join_url ? `
                    <div class="change-details-item">
                        <div class="change-details-label">Join URL</div>
                        <div class="change-details-value">
                            <a href="${this.escapeHtml(meetingMetadata.join_url)}" target="_blank" class="meeting-link">
                                Click to Join Meeting
                            </a>
                        </div>
                    </div>
                ` : ''}
                ${meetingMetadata.start_time ? this.renderDetailItem('Start Time', this.formatTimestamp(meetingMetadata.start_time)) : ''}
                ${meetingMetadata.end_time ? this.renderDetailItem('End Time', this.formatTimestamp(meetingMetadata.end_time)) : ''}
                ${meetingMetadata.duration ? this.renderDetailItem('Duration', `${meetingMetadata.duration} minutes`) : ''}
            `;
        } else {
            // Use top-level fields for draft/pending announcements
            meetingInfo = `
                ${!meetingMetadata ? `
                    <div class="change-details-item" style="grid-column: 1 / -1;">
                        <div class="change-details-value" style="color: #856404; background: #fff3cd; padding: 10px; border-radius: 4px;">
                            ℹ️ Meeting will be scheduled when this announcement is approved
                        </div>
                    </div>
                ` : ''}
                ${announcement.meeting_title ? this.renderDetailItem('Meeting Title', announcement.meeting_title) : ''}
                ${announcement.meeting_date ? this.renderDetailItem('Scheduled Date/Time', this.formatTimestamp(announcement.meeting_date)) : ''}
                ${announcement.meeting_duration ? this.renderDetailItem('Duration', `${announcement.meeting_duration} minutes`) : ''}
                ${announcement.attendees ? this.renderDetailItem('Attendees', announcement.attendees) : ''}
                ${announcement.meeting_location ? this.renderDetailItem('Location', announcement.meeting_location) : ''}
            `;
        }

        return `
            <div class="change-details-section">
                <h4 class="change-details-section-title">
                    <span class="change-details-section-icon">📅</span>
                    Meeting Information
                </h4>
                <div class="change-details-grid">
                    ${meetingInfo}
                </div>
            </div>
        `;
    }

    /**
     * Render the timeline section (modification history)
     */
    renderTimelineSection() {
        const modifications = this.announcementData.modifications || [];
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
     * Render action buttons in footer
     */
    renderActionButtons() {
        const footerEl = this.modalElement.querySelector('.change-details-modal-footer');
        if (!footerEl) return;

        const announcementId = this.announcementData.announcement_id || this.announcementData.id;
        const status = this.announcementData.status;

        // Create AnnouncementActions instance
        this.announcementActions = new AnnouncementActions(
            announcementId,
            status,
            this.announcementData
        );

        // Register global instance
        this.announcementActions.registerGlobal();

        // Get action buttons HTML
        const actionButtons = this.announcementActions.renderActionButtons();

        footerEl.innerHTML = `
            <div class="modal-footer-actions">
                ${actionButtons}
            </div>
        `;
        
        // Add event listeners to action buttons
        const actionBtns = footerEl.querySelectorAll('.announcement-action-btn');
        actionBtns.forEach(btn => {
            const handler = btn.getAttribute('data-handler');
            if (handler && typeof this.announcementActions[handler] === 'function') {
                btn.addEventListener('click', () => this.announcementActions[handler]());
            }
        });
    }

    /**
     * Setup event listeners
     */
    setupEventListeners() {
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
        const focusableElements = this.modalElement.querySelectorAll(
            'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
        );
        
        if (focusableElements.length === 0) return;

        const firstElement = focusableElements[0];
        const lastElement = focusableElements[focusableElements.length - 1];

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
        firstElement.focus();
    }

    // Helper methods

    renderDetailItem(label, value) {
        if (!value) return '';
        return `
            <div class="change-details-item">
                <div class="change-details-label">${this.escapeHtml(label)}</div>
                <div class="change-details-value">${this.escapeHtml(value)}</div>
            </div>
        `;
    }

    renderAffectedCustomers() {
        const announcement = this.announcementData;
        const customers = announcement.customers || [];
        
        if (!customers || customers.length === 0) return '';

        return `
            <div class="change-details-item full-width">
                <div class="change-details-label">Customers</div>
                <div class="change-details-value">
                    ${customers.map(c => `<span class="customer-badge">${this.escapeHtml(c)}</span>`).join(' ')}
                </div>
            </div>
        `;
    }

    renderSummary() {
        const announcement = this.announcementData;
        const summary = announcement.summary || '';
        
        if (!summary) return '';

        return `
            <div class="change-details-item full-width">
                <div class="change-details-label">Summary</div>
                <div class="change-details-value">${this.escapeHtml(summary)}</div>
            </div>
        `;
    }

    renderStatusBadge(status) {
        const statusClass = this.getStatusClass(status);
        const statusLabel = this.getStatusLabel(status);
        return `<span class="change-status ${statusClass}">${statusLabel}</span>`;
    }

    getStatusClass(status) {
        const statusMap = {
            'draft': 'status-draft',
            'submitted': 'status-pending',
            'approved': 'status-approved',
            'completed': 'status-completed',
            'cancelled': 'status-cancelled'
        };
        return statusMap[status] || 'status-unknown';
    }

    getStatusLabel(status) {
        const labelMap = {
            'draft': 'Draft',
            'submitted': 'Pending Approval',
            'approved': 'Approved',
            'completed': 'Completed',
            'cancelled': 'Cancelled'
        };
        return labelMap[status] || status;
    }

    getAnnouncementTypeLabel(type) {
        if (!type) return 'General';
        
        const cleanType = type.replace('announcement_', '');
        const labels = {
            'cic': 'CIC (Cloud Innovator Community)',
            'finops': 'FinOps',
            'innersource': 'Innersource Guild',
            'general': 'General'
        };
        
        return labels[cleanType.toLowerCase()] || cleanType;
    }

    getAnnouncementTypeIcon(type) {
        if (!type) return '📢';
        
        const cleanType = type.replace('announcement_', '');
        const icons = {
            'cic': '☁️',
            'finops': '💰',
            'innersource': '🔧',
            'general': '📢'
        };
        
        return icons[cleanType.toLowerCase()] || '📢';
    }

    getModificationIcon(type) {
        const icons = {
            'created': '➕',
            'updated': '✏️',
            'submitted': '📤',
            'approved': '✅',
            'cancelled': '❌',
            'completed': '✓',
            'meeting_scheduled': '📅',
            'meeting_cancelled': '🚫'
        };
        return icons[type] || '●';
    }

    getModificationLabel(type) {
        const labels = {
            'created': 'Created',
            'updated': 'Updated',
            'submitted': 'Submitted for Approval',
            'approved': 'Approved',
            'cancelled': 'Cancelled',
            'completed': 'Completed',
            'meeting_scheduled': 'Meeting Scheduled',
            'meeting_cancelled': 'Meeting Cancelled'
        };
        return labels[type] || type;
    }

    getUserDisplay(userId) {
        if (!userId) return 'Unknown';
        // Extract name from email if possible
        if (userId.includes('@')) {
            return userId.split('@')[0].replace(/[._]/g, ' ');
        }
        return userId;
    }

    formatTimestamp(timestamp) {
        if (!timestamp) return 'N/A';
        try {
            const date = new Date(timestamp);
            return date.toLocaleString('en-US', {
                year: 'numeric',
                month: 'short',
                day: 'numeric',
                hour: '2-digit',
                minute: '2-digit'
            });
        } catch (e) {
            return timestamp;
        }
    }

    formatContent(content) {
        // Simple markdown-like formatting
        return this.escapeHtml(content)
            .replace(/\n\n/g, '</p><p>')
            .replace(/\n/g, '<br>');
    }

    formatFileSize(bytes) {
        if (bytes === 0) return '0 Bytes';
        const k = 1024;
        const sizes = ['Bytes', 'KB', 'MB', 'GB'];
        const i = Math.floor(Math.log(bytes) / Math.log(k));
        return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
    }

    escapeHtml(text) {
        if (!text) return '';
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    /**
     * Advance announcement to the next status
     */
    async advanceStatus(newStatus) {
        const announcementId = this.announcementData.announcement_id || this.announcementData.id;
        
        try {
            // Use the AnnouncementActions to handle the status change
            if (this.announcementActions) {
                // Map status to action method name
                const actionMap = {
                    'submitted': 'submitAnnouncement',
                    'approved': 'approveAnnouncement',
                    'completed': 'completeAnnouncement'
                };
                
                const actionMethod = actionMap[newStatus];
                if (actionMethod && typeof this.announcementActions[actionMethod] === 'function') {
                    await this.announcementActions[actionMethod]();
                    // Modal will be refreshed by the action handler
                } else {
                    console.error('No action handler for status:', newStatus);
                }
            }
        } catch (error) {
            console.error('Error advancing status:', error);
            alert('Failed to advance status: ' + error.message);
        }
    }
}

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
    module.exports = AnnouncementDetailsModal;
}

// Global instance for easy access
let announcementDetailsModal = null;
