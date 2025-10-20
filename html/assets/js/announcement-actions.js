/**
 * Announcement Actions Module
 * Provides action button rendering and status management for announcements
 * Mirrors the change management workflow for consistency
 */

class AnnouncementActions {
    constructor(announcementId, currentStatus, announcementData = null) {
        this.announcementId = announcementId;
        this.currentStatus = currentStatus;
        this.announcementData = announcementData;
        this.baseUrl = window.location.origin;
        this.isProcessing = false;
    }

    /**
     * Render action buttons based on announcement status
     * @returns {string} HTML string for action buttons
     */
    renderActionButtons() {
        const actions = this.getAvailableActions();
        if (actions.length === 0) {
            return this.renderStatusInfo();
        }

        return `
            <div class="announcement-actions" role="group" aria-label="Announcement actions">
                ${actions.map(action => this.renderButton(action)).join('')}
            </div>
        `;
    }

    /**
     * Get available actions based on current status
     * @returns {Array<string>} Array of action names
     */
    getAvailableActions() {
        const actions = {
            'draft': ['edit'],
            'submitted': ['edit', 'approve', 'cancel'],
            'approved': ['complete', 'cancel'],
            'completed': [],
            'cancelled': []
        };
        return actions[this.currentStatus] || [];
    }

    /**
     * Render a single action button
     * @param {string} action - Action name (edit, approve, cancel, complete)
     * @returns {string} HTML string for button
     */
    renderButton(action) {
        const buttonConfig = {
            edit: {
                label: '✏️ Edit',
                class: 'action-btn edit',
                handler: 'editAnnouncement',
                ariaLabel: 'Edit this announcement'
            },
            approve: {
                label: '✅ Approve',
                class: 'action-btn approve',
                handler: 'approveAnnouncement',
                ariaLabel: 'Approve this announcement'
            },
            cancel: {
                label: '💣 Cancel',
                class: 'action-btn cancel',
                handler: 'cancelAnnouncement',
                ariaLabel: 'Cancel this announcement'
            },
            complete: {
                label: '✓ Complete',
                class: 'action-btn complete',
                handler: 'completeAnnouncement',
                ariaLabel: 'Mark this announcement as complete'
            }
        };

        const config = buttonConfig[action];
        if (!config) return '';

        const disabled = this.isProcessing ? 'disabled' : '';

        return `
            <button 
                class="${config.class} announcement-action-btn" 
                aria-label="${config.ariaLabel}"
                ${disabled}
                data-action="${action}"
                data-handler="${config.handler}">
                ${config.label}
            </button>
        `;
    }

    /**
     * Render status information when no actions are available
     * @returns {string} HTML string for status info
     */
    renderStatusInfo() {
        const statusMessages = {
            'completed': {
                icon: '✓',
                message: 'This announcement has been completed',
                class: 'status-completed'
            },
            'cancelled': {
                icon: '✗',
                message: 'This announcement has been cancelled',
                class: 'status-cancelled'
            },
            'draft': {
                icon: '📝',
                message: 'This announcement is in draft status',
                class: 'status-draft'
            }
        };

        const info = statusMessages[this.currentStatus];
        if (!info) return '';

        return `
            <div class="announcement-status-info ${info.class}" role="status">
                <span class="status-icon" aria-hidden="true">${info.icon}</span>
                <span>${info.message}</span>
            </div>
        `;
    }

    /**
     * Validate status transition
     * @param {string} newStatus - Target status
     * @returns {boolean} True if transition is valid
     */
    validateStatusTransition(newStatus) {
        const validTransitions = {
            'draft': ['submitted', 'cancelled'],
            'submitted': ['approved', 'cancelled'],
            'approved': ['completed', 'cancelled'],
            'completed': [],
            'cancelled': []
        };

        const allowed = validTransitions[this.currentStatus] || [];
        return allowed.includes(newStatus);
    }

    /**
     * Edit announcement
     * Redirects to edit page for draft or submitted announcements
     * For approved announcements, shows warning that meeting will be cancelled
     */
    async editAnnouncement() {
        if (this.isProcessing) return;

        // For draft and submitted, just redirect to edit page
        if (this.currentStatus === 'draft' || this.currentStatus === 'submitted') {
            window.location.href = `edit-announcement.html?id=${this.announcementId}`;
            return;
        }

        // For approved, show warning (though this shouldn't happen based on getAvailableActions)
        if (this.currentStatus === 'approved') {
            window.location.href = `edit-announcement.html?id=${this.announcementId}`;
            return;
        }

        // For other statuses, show error
        this.showErrorMessage(`Cannot edit announcement with status: ${this.currentStatus}`);
    }

    /**
     * Approve announcement
     * Updates status to 'approved' and triggers backend processing
     */
    async approveAnnouncement() {
        if (this.isProcessing) return;

        try {
            this.isProcessing = true;
            this.updateButtonStates(true);

            console.log('Approving announcement:', this.announcementId);

            // Validate transition
            if (!this.validateStatusTransition('approved')) {
                throw new Error(`Cannot approve announcement with status: ${this.currentStatus}`);
            }

            // Get current user and timestamp
            const currentUser = window.portal?.currentUser || 'Unknown';
            const approvalTimestamp = new Date().toISOString();

            // Update status with approval metadata
            await this.updateAnnouncementStatus('approved', 'approved', {
                approvedBy: currentUser,
                approvedAt: approvalTimestamp
            });

            // Show success message
            this.showSuccessMessage('Announcement approved successfully! Emails will be sent and meetings scheduled if configured.');

            // Close modal if open
            if (typeof announcementDetailsModal !== 'undefined' && announcementDetailsModal) {
                announcementDetailsModal.hide();
            }

            // Clear cache and reload data without full page refresh
            setTimeout(async () => {
                if (typeof announcementsPage !== 'undefined' && announcementsPage) {
                    // On announcements page - clear cache and reload
                    if (window.s3Client) {
                        s3Client.clearCache();
                    }
                    await announcementsPage.loadAnnouncements();
                    // Switch to approved filter to show the newly approved announcement
                    announcementsPage.filterByStatus('approved');
                } else if (typeof approvalsPage !== 'undefined' && approvalsPage.refresh) {
                    approvalsPage.refresh();
                } else {
                    window.location.reload();
                }
            }, 1000);

        } catch (error) {
            console.error('Error approving announcement:', error);
            this.showErrorMessage(`Failed to approve announcement: ${error.message}`);
        } finally {
            this.isProcessing = false;
            this.updateButtonStates(false);
        }
    }

    /**
     * Cancel announcement
     * Updates status to 'cancelled' immediately and cancels any scheduled meetings asynchronously
     * NOTE: Backend will reload the announcement from S3 to get latest meeting metadata
     * This prevents race conditions where frontend might have stale data
     */
    async cancelAnnouncement() {
        if (this.isProcessing) return;

        try {
            this.isProcessing = true;
            this.updateButtonStates(true);

            console.log('Cancelling announcement:', this.announcementId);

            // Validate transition
            if (!this.validateStatusTransition('cancelled')) {
                throw new Error(`Cannot cancel announcement with status: ${this.currentStatus}`);
            }

            // Update status - backend will handle reloading from S3 to get latest meeting metadata
            await this.updateAnnouncementStatus('cancelled', 'cancelled');

            // Show success message immediately
            this.showSuccessMessage('Announcement cancelled successfully. Meeting cancellation in progress...');

            // Close modal if open
            if (typeof announcementDetailsModal !== 'undefined' && announcementDetailsModal) {
                announcementDetailsModal.hide();
            }

            // Immediately redirect to cancelled filter (don't wait for meeting cancellation)
            setTimeout(() => {
                if (typeof announcementsPage !== 'undefined' && announcementsPage) {
                    // On announcements page - reload and filter to cancelled
                    window.location.href = 'announcements.html?status=cancelled';
                } else if (typeof approvalsPage !== 'undefined' && approvalsPage.refresh) {
                    approvalsPage.refresh();
                } else {
                    window.location.reload();
                }
            }, 1000);

            // Start background polling to wait for meeting cancellation (non-blocking)
            // This will update the join button once the backend completes the cancellation
            this.startMeetingCancellationWatch();

        } catch (error) {
            console.error('Error cancelling announcement:', error);
            this.showErrorMessage(`Failed to cancel announcement: ${error.message}`);
        } finally {
            this.isProcessing = false;
            this.updateButtonStates(false);
        }
    }

    /**
     * Start background watch for meeting cancellation (non-blocking)
     * Polls in the background to detect when join button should be removed
     */
    startMeetingCancellationWatch() {
        // This runs in the background and doesn't block the UI
        this.waitForMeetingCancellation().then(() => {
            console.log('✅ Meeting cancellation confirmed in background');
            // If we're still on the page, grey out any remaining join buttons
            const joinButtons = document.querySelectorAll('.join-meeting');
            joinButtons.forEach(btn => {
                btn.style.opacity = '0.5';
                btn.style.pointerEvents = 'none';
                btn.setAttribute('aria-disabled', 'true');
            });
        }).catch(error => {
            console.warn('⚠️ Background meeting cancellation watch failed:', error);
        });
    }

    /**
     * Wait for backend to cancel the meeting by polling for meeting_metadata removal
     * Uses ETag-based polling to detect when backend has processed the cancellation
     */
    async waitForMeetingCancellation() {
        const maxAttempts = 60; // 60 seconds max wait
        const pollInterval = 1000; // 1 second between polls
        let attempts = 0;

        console.log('⏳ Waiting for meeting cancellation to complete...');

        while (attempts < maxAttempts) {
            attempts++;

            try {
                // Fetch latest announcement data with skipCache to force fresh data
                const s3Client = new S3Client();
                const announcements = await s3Client.fetchAnnouncements({ skipCache: true });
                const announcement = announcements.find(a => 
                    (a.announcement_id || a.id) === this.announcementId
                );

                if (announcement) {
                    // Check if meeting has been cancelled (meeting_metadata should be removed)
                    const meetingMetadata = announcement.meeting_metadata;
                    const hasJoinUrl = meetingMetadata?.join_url;

                    if (!hasJoinUrl) {
                        console.log('✅ Meeting cancellation confirmed (no join_url found)');
                        return true;
                    }

                    console.log(`⏳ Attempt ${attempts}/${maxAttempts}: Meeting still has join_url, waiting...`);
                } else {
                    console.warn('⚠️ Announcement not found, assuming cancellation complete');
                    return true;
                }

            } catch (error) {
                console.warn(`⚠️ Error polling for cancellation (attempt ${attempts}):`, error);
            }

            // Wait before next poll
            await new Promise(resolve => setTimeout(resolve, pollInterval));
        }

        console.warn('⚠️ Timeout waiting for meeting cancellation, proceeding anyway');
        return false;
    }

    /**
     * Complete announcement
     * Updates status to 'completed'
     */
    async completeAnnouncement() {
        if (this.isProcessing) return;

        try {
            this.isProcessing = true;
            this.updateButtonStates(true);

            console.log('Completing announcement:', this.announcementId);

            // Validate transition
            if (!this.validateStatusTransition('completed')) {
                throw new Error(`Cannot complete announcement with status: ${this.currentStatus}`);
            }

            // Update status
            await this.updateAnnouncementStatus('completed', 'completed');

            // Show success message
            this.showSuccessMessage('Announcement marked as complete.');

            // Close modal if open
            if (typeof announcementDetailsModal !== 'undefined' && announcementDetailsModal) {
                announcementDetailsModal.hide();
            }

            // Trigger page refresh after delay
            setTimeout(() => {
                if (typeof announcementsPage !== 'undefined' && announcementsPage) {
                    // On announcements page - reload and filter to completed
                    window.location.href = 'announcements.html?status=completed';
                } else if (typeof approvalsPage !== 'undefined' && approvalsPage.refresh) {
                    approvalsPage.refresh();
                } else {
                    window.location.reload();
                }
            }, 1000);

        } catch (error) {
            console.error('Error completing announcement:', error);
            this.showErrorMessage(`Failed to complete announcement: ${error.message}`);
        } finally {
            this.isProcessing = false;
            this.updateButtonStates(false);
        }
    }

    /**
     * Update announcement status via upload_lambda API
     * @param {string} newStatus - New status value
     * @param {string} modificationType - Type of modification for history
     * @param {Object} additionalData - Additional data to include (e.g., approvedBy, approvedAt)
     */
    async updateAnnouncementStatus(newStatus, modificationType, additionalData = {}) {
        // Get current user
        const currentUser = window.portal?.currentUser || 'Unknown';

        // Create modification entry
        const modification = {
            timestamp: new Date().toISOString(),
            user_id: currentUser,
            modification_type: modificationType
        };

        // Prepare update payload
        const updatePayload = {
            action: 'update_announcement',
            announcement_id: this.announcementId,
            status: newStatus,
            modification: modification,
            // Include additional fields like approvedBy, approvedAt, etc.
            ...additionalData
        };

        // If we have full announcement data, include customers list
        if (this.announcementData && this.announcementData.customers) {
            updatePayload.customers = this.announcementData.customers;
        }

        console.log('Sending update to upload_lambda:', updatePayload);

        // Call upload_lambda API
        let response;
        try {
            response = await fetch('/upload', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json'
                },
                body: JSON.stringify(updatePayload),
                credentials: 'same-origin'
            });
        } catch (fetchError) {
            // CORS errors or network failures often indicate session timeout
            // The browser blocks the redirect to SSO, causing a fetch failure
            console.warn('Fetch failed (likely session timeout):', fetchError.message);
            console.warn('Reloading page to re-authenticate...');
            window.location.reload();
            throw new Error('Session expired. Please log in again.');
        }

        // Check for authentication redirect (session timeout)
        if (response.redirected || response.status === 401 || response.status === 403) {
            console.warn('Session expired or authentication required, redirecting to login...');
            window.location.reload();
            throw new Error('Session expired. Please log in again.');
        }

        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(`API request failed: ${response.status} ${errorText}`);
        }

        const result = await response.json();
        console.log('Update response:', result);

        return result;
    }

    /**
     * Update button states (enable/disable)
     * @param {boolean} disabled - Whether buttons should be disabled
     */
    updateButtonStates(disabled) {
        const buttons = document.querySelectorAll(`[data-action]`);
        buttons.forEach(button => {
            button.disabled = disabled;
            if (disabled) {
                button.classList.add('processing');
            } else {
                button.classList.remove('processing');
            }
        });
    }

    /**
     * Show success message
     * @param {string} message - Success message to display
     */
    showSuccessMessage(message) {
        // Try to use global message system if available
        if (typeof showSuccess === 'function') {
            showSuccess('statusContainer', message);
        } else {
            alert(message);
        }
    }

    /**
     * Show error message
     * @param {string} message - Error message to display
     */
    showErrorMessage(message) {
        // Try to use global message system if available
        if (typeof showError === 'function') {
            showError('statusContainer', message);
        } else {
            alert(message);
        }
    }

    /**
     * Create a global instance for this announcement
     * This allows onclick handlers to reference the instance
     */
    registerGlobal() {
        window[`announcementActions_${this.announcementId}`] = this;
    }

    /**
     * Remove global instance
     */
    unregisterGlobal() {
        delete window[`announcementActions_${this.announcementId}`];
    }
}

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
    module.exports = AnnouncementActions;
}
