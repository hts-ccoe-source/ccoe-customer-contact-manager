/**
 * Change Details Modal Component
 * Displays comprehensive change information including description,
 * meeting metadata, rollback plan, and action buttons
 */

class ChangeDetailsModal {
    constructor(changeData) {
        this.changeData = changeData;
        this.modalElement = null;
        this.changeActions = null;
    }

    /**
     * Render and display the modal
     */
    show() {
        try {
            console.log('🔍 ChangeDetailsModal.show() called');
            
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

        // Cleanup change actions instance
        if (this.changeActions) {
            this.changeActions.unregisterGlobal();
            this.changeActions = null;
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

        const change = this.changeData;

        // Update title
        const titleEl = this.modalElement.querySelector('.change-details-modal-title');
        const workflowIcon = this.getWorkflowIcon(change.workflow);
        titleEl.innerHTML = `<span class="workflow-icon">${workflowIcon}</span> ${this.escapeHtml(change.changeTitle || change.title || 'Untitled Change')}`;

        // Update subtitle with change ID and status
        const subtitleEl = this.modalElement.querySelector('.change-details-modal-subtitle');
        const changeId = change.changeId || change.id || 'N/A';
        const status = change.status || 'unknown';
        const statusBadge = this.renderStatusBadge(status);
        const workflowLabel = this.getWorkflowLabel(change.workflow);
        subtitleEl.innerHTML = `
            <span class="change-details-change-id">${this.escapeHtml(changeId)}</span>
            <span class="workflow-label">${this.escapeHtml(workflowLabel)}</span>
            ${statusBadge}
        `;

        // Render body sections
        const bodyEl = this.modalElement.querySelector('.change-details-modal-body');
        
        try {
            const statusBar = this.renderStatusProgressBar();
            const details = this.renderDetailsSection();
            const description = this.renderDescriptionSection();
            const rollback = this.renderRollbackSection();
            const meeting = this.renderMeetingSection();
            const timeline = this.renderTimelineSection();
            
            bodyEl.innerHTML = `
                ${statusBar}
                ${details}
                ${description}
                ${rollback}
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
            const change = this.changeData;
            const currentStatus = change.status || 'draft';
            
            // Define status progression
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
        const change = this.changeData;
        
        return `
            <div class="change-details-section">
                <h4 class="change-details-section-title">
                    <span class="change-details-section-icon">📋</span>
                    Basic Information
                </h4>
                <div class="change-details-grid">
                    ${this.renderDetailItem('Created By', this.getUserDisplay(change.createdBy || change.created_by))}
                    ${this.renderDetailItem('Created At', this.formatTimestamp(change.createdAt || change.created_at))}
                    ${change.submittedAt || change.submitted_at ? this.renderDetailItem('Submitted At', this.formatTimestamp(change.submittedAt || change.submitted_at)) : ''}
                    ${this.renderApprovalInfo()}
                    ${change.workflow ? this.renderDetailItem('Workflow Type', this.getWorkflowLabel(change.workflow)) : ''}
                    ${change.impactLevel ? this.renderDetailItem('Impact Level', change.impactLevel) : ''}
                </div>
            </div>
        `;
    }

    /**
     * Render the description section
     */
    renderDescriptionSection() {
        const change = this.changeData;
        const description = change.changeDescription || change.description || '';
        
        if (!description) return '';

        return `
            <div class="change-details-section">
                <h4 class="change-details-section-title">
                    <span class="change-details-section-icon">📄</span>
                    Description
                </h4>
                <div class="change-details-content">
                    ${this.formatContent(description)}
                </div>
            </div>
        `;
    }

    /**
     * Render the rollback plan section
     */
    renderRollbackSection() {
        const change = this.changeData;
        const rollbackPlan = change.rollbackPlan || change.rollback_plan || '';
        
        if (!rollbackPlan) return '';

        return `
            <div class="change-details-section">
                <h4 class="change-details-section-title">
                    <span class="change-details-section-icon">🔄</span>
                    Rollback Plan
                </h4>
                <div class="change-details-content">
                    ${this.formatContent(rollbackPlan)}
                </div>
            </div>
        `;
    }

    /**
     * Render the meeting section
     */
    renderMeetingSection() {
        const change = this.changeData;
        // Backend uses meeting_metadata with join_url (snake_case)
        const meetingMetadata = change.meeting_metadata;
        
        // Check if meeting is required
        const meetingRequired = change.meetingRequired === 'yes' || change.meeting_required === 'yes';
        
        if (!meetingRequired) return '';

        // Build meeting information
        let meetingInfo = '';
        const isCompleted = change.status === 'completed';
        const isCancelled = change.status === 'cancelled';
        
        if (meetingMetadata) {
            // Use meeting_metadata if available (for approved/scheduled meetings)
            // Don't show Join link if status is completed or cancelled
            meetingInfo = `
                ${meetingMetadata.join_url && !isCompleted && !isCancelled ? `
                    <div class="change-details-item">
                        <div class="change-details-label">Join URL</div>
                        <div class="change-details-value">
                            <a href="${this.escapeHtml(meetingMetadata.join_url)}" 
                               target="_blank" 
                               class="action-btn join-meeting"
                               style="display: inline-block; text-decoration: none;">
                                🎥 Join Meeting
                            </a>
                        </div>
                    </div>
                ` : ''}
                ${meetingMetadata.start_time ? this.renderDetailItem('Start Time', this.formatTimestamp(meetingMetadata.start_time)) : ''}
                ${meetingMetadata.end_time ? this.renderDetailItem('End Time', this.formatTimestamp(meetingMetadata.end_time)) : ''}
                ${meetingMetadata.duration ? this.renderDetailItem('Duration', `${meetingMetadata.duration} minutes`) : ''}
            `;
        } else {
            // Use top-level fields for draft/pending changes
            meetingInfo = `
                ${!meetingMetadata ? `
                    <div class="change-details-item" style="grid-column: 1 / -1;">
                        <div class="change-details-value" style="color: #856404; background: #fff3cd; padding: 10px; border-radius: 4px;">
                            ℹ️ Meeting will be scheduled when this change is approved
                        </div>
                    </div>
                ` : ''}
                ${change.meetingTitle || change.meeting_title ? this.renderDetailItem('Meeting Title', change.meetingTitle || change.meeting_title) : ''}
                ${change.meetingDate || change.meeting_date ? this.renderDetailItem('Scheduled Date/Time', this.formatTimestamp(change.meetingDate || change.meeting_date)) : ''}
                ${change.meetingDuration || change.meeting_duration ? this.renderDetailItem('Duration', `${change.meetingDuration || change.meeting_duration} minutes`) : ''}
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
     * Render action buttons in footer
     */
    renderActionButtons() {
        const footerEl = this.modalElement.querySelector('.change-details-modal-footer');
        if (!footerEl) return;

        const changeId = this.changeData.changeId || this.changeData.id;
        const status = this.changeData.status;

        // Create ChangeActions instance (assuming it exists similar to AnnouncementActions)
        if (typeof ChangeActions !== 'undefined') {
            this.changeActions = new ChangeActions(
                changeId,
                status,
                this.changeData
            );

            // Register global instance
            this.changeActions.registerGlobal();

            // Get action buttons HTML
            const actionButtons = this.changeActions.renderActionButtons();

            footerEl.innerHTML = `
                <div class="modal-footer-actions">
                    ${actionButtons}
                </div>
            `;
            
            // Add event listeners to action buttons
            const actionBtns = footerEl.querySelectorAll('.change-action-btn');
            actionBtns.forEach(btn => {
                const handler = btn.getAttribute('data-handler');
                if (handler && typeof this.changeActions[handler] === 'function') {
                    btn.addEventListener('click', () => this.changeActions[handler]());
                }
            });
        } else {
            // Fallback: render basic action buttons
            footerEl.innerHTML = `
                <div class="modal-footer-actions">
                    <button class="btn-secondary" onclick="changeDetailsModal.hide()">Close</button>
                </div>
            `;
        }
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

    renderApprovalInfo() {
        const change = this.changeData;
        
        // Only show for approved or completed changes
        if (change.status !== 'approved' && change.status !== 'completed') {
            return '';
        }

        // Try to get approval info from modifications array first
        const modifications = change.modifications || [];
        const approvalMod = modifications.find(mod => 
            mod.modification_type === 'approved' || mod.modification_type === 'approve'
        );

        let approvedBy = 'Unknown';
        let approvedAt = 'N/A';

        if (approvalMod) {
            // Use data from modifications array
            approvedBy = approvalMod.user_id || approvalMod.user || 'Unknown';
            approvedAt = this.formatTimestamp(approvalMod.timestamp);
        } else {
            // Fallback to top-level fields
            approvedBy = this.getUserDisplay(change.approvedBy || change.approved_by);
            approvedAt = this.formatTimestamp(change.approvedAt || change.approved_at);
        }

        return `
            ${this.renderDetailItem('Approved By', approvedBy)}
            ${this.renderDetailItem('Approved At', approvedAt)}
        `;
    }

    renderDetailItem(label, value) {
        if (!value) return '';
        return `
            <div class="change-details-item">
                <div class="change-details-label">${this.escapeHtml(label)}</div>
                <div class="change-details-value">${this.escapeHtml(value)}</div>
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

    getWorkflowLabel(workflow) {
        if (!workflow) return 'Standard';
        
        const labels = {
            'standard': 'Standard',
            'emergency': 'Emergency',
            'expedited': 'Expedited'
        };
        
        return labels[workflow.toLowerCase()] || workflow;
    }

    getWorkflowIcon(workflow) {
        if (!workflow) return '📋';
        
        const icons = {
            'standard': '📋',
            'emergency': '🚨',
            'expedited': '⚡'
        };
        
        return icons[workflow.toLowerCase()] || '📋';
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

    escapeHtml(text) {
        if (!text) return '';
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    }

    /**
     * Advance change to the next status
     */
    async advanceStatus(newStatus) {
        const changeId = this.changeData.changeId || this.changeData.id;
        
        try {
            // Use the ChangeActions to handle the status change
            if (this.changeActions) {
                // Map status to action method name
                const actionMap = {
                    'submitted': 'submitChange',
                    'approved': 'approveChange',
                    'completed': 'completeChange'
                };
                
                const actionMethod = actionMap[newStatus];
                if (actionMethod && typeof this.changeActions[actionMethod] === 'function') {
                    await this.changeActions[actionMethod]();
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
    module.exports = ChangeDetailsModal;
}

// Global instance for easy access
let changeDetailsModal = null;
