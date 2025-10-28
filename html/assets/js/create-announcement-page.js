/**
 * Create Announcement Page JavaScript
 * Handles announcement creation with type-specific ID generation, meeting scheduling, and file attachments
 */

// Global state
let currentAnnouncementId = null;
let currentAnnouncementType = null;
let uploadedFiles = [];
let uploadInProgress = false;

/**
 * Initialize page on load
 */
document.addEventListener('DOMContentLoaded', function () {
    initializeEventListeners();
    loadUserInfo();
});

/**
 * Initialize all event listeners
 */
function initializeEventListeners() {
    // Announcement type selection
    const typeRadios = document.querySelectorAll('input[name="announcementType"]');
    typeRadios.forEach(radio => {
        radio.addEventListener('change', handleTypeChange);
    });

    // Meeting toggle
    const meetingSelect = document.getElementById('meetingRequired');
    if (meetingSelect) {
        meetingSelect.addEventListener('change', handleMeetingToggle);
    }

    // File upload
    const fileUploadArea = document.getElementById('fileUploadArea');
    const fileInput = document.getElementById('fileInput');

    if (fileUploadArea && fileInput) {
        fileUploadArea.addEventListener('click', () => fileInput.click());
        fileInput.addEventListener('change', handleFileSelect);

        // Drag and drop
        fileUploadArea.addEventListener('dragover', handleDragOver);
        fileUploadArea.addEventListener('dragleave', handleDragLeave);
        fileUploadArea.addEventListener('drop', handleFileDrop);
    }

    // Form submission
    const form = document.getElementById('announcementForm');
    if (form) {
        form.addEventListener('submit', handleFormSubmit);
    }

    // Auto-populate meeting title when announcement title changes
    const announcementTitle = document.getElementById('announcementTitle');
    const meetingTitle = document.getElementById('meetingTitle');
    if (announcementTitle && meetingTitle) {
        announcementTitle.addEventListener('input', function () {
            if (!meetingTitle.value || meetingTitle.value === meetingTitle.placeholder) {
                meetingTitle.value = announcementTitle.value;
            }
        });
    }
}

/**
 * Handle announcement type change
 */
function handleTypeChange(event) {
    const type = event.target.value;
    currentAnnouncementType = type;

    // Generate announcement ID
    currentAnnouncementId = generateAnnouncementId(type);

    // Display announcement ID
    const idDisplay = document.getElementById('announcementIdDisplay');
    const idElement = document.getElementById('currentAnnouncementId');

    if (idDisplay && idElement) {
        idElement.textContent = currentAnnouncementId;
        idDisplay.style.display = 'block';
    }
}

/**
 * Generate announcement ID with type-specific prefix (GUID format like changes)
 * Format: PREFIX-xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx
 * Examples: CIC-a1b2c3d4-e5f6-4789-a012-b3c4d5e6f7a8
 * @param {string} type - Announcement type (cic, finops, innersource)
 * @returns {string} Generated announcement ID in GUID format
 */
function generateAnnouncementId(type) {
    const prefix = getTypePrefix(type);

    // Generate a proper GUID/UUID v4 (RFC 4122 compliant)
    // This ensures IDs are globally unique and follow the same format as changes
    const guid = 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, function (c) {
        const r = Math.random() * 16 | 0;
        const v = c === 'x' ? r : (r & 0x3 | 0x8);
        return v.toString(16);
    });

    return `${prefix}-${guid}`;
}

/**
 * Get prefix for announcement type
 * @param {string} type - Announcement type
 * @returns {string} Prefix
 */
function getTypePrefix(type) {
    const prefixes = {
        'cic': 'CIC',
        'finops': 'FIN',
        'innersource': 'INN'
    };
    return prefixes[type] || 'ANN';
}

/**
 * Handle meeting toggle
 */
function handleMeetingToggle(event) {
    const meetingDetails = document.getElementById('meetingDetails');
    const includeMeeting = event.target.value === 'yes';

    if (meetingDetails) {
        meetingDetails.style.display = includeMeeting ? 'block' : 'none';

        // Auto-populate meeting title if announcement title exists
        if (includeMeeting) {
            const announcementTitle = document.getElementById('announcementTitle');
            const meetingTitle = document.getElementById('meetingTitle');
            if (announcementTitle && meetingTitle && announcementTitle.value) {
                meetingTitle.value = announcementTitle.value;
            }
        }
    }
}

/**
 * Handle file selection
 */
function handleFileSelect(event) {
    const files = Array.from(event.target.files);
    addFiles(files);
}

/**
 * Handle drag over
 */
function handleDragOver(event) {
    event.preventDefault();
    event.currentTarget.classList.add('drag-over');
}

/**
 * Handle drag leave
 */
function handleDragLeave(event) {
    event.currentTarget.classList.remove('drag-over');
}

/**
 * Handle file drop
 */
function handleFileDrop(event) {
    event.preventDefault();
    event.currentTarget.classList.remove('drag-over');

    const files = Array.from(event.dataTransfer.files);
    addFiles(files);
}

/**
 * Add files to upload list
 * @param {File[]} files - Files to add
 */
function addFiles(files) {
    files.forEach(file => {
        // Check if file already exists
        const exists = uploadedFiles.some(f => f.name === file.name && f.size === file.size);
        if (!exists) {
            uploadedFiles.push(file);
        }
    });

    renderFileList();
}

/**
 * Render file list
 */
function renderFileList() {
    const fileList = document.getElementById('fileList');
    if (!fileList) return;

    if (uploadedFiles.length === 0) {
        fileList.innerHTML = '';
        return;
    }

    fileList.innerHTML = uploadedFiles.map((file, index) => `
        <div class="file-item">
            <div class="file-info">
                <span class="file-icon">📎</span>
                <div class="file-details">
                    <div class="file-name">${escapeHtml(file.name)}</div>
                    <div class="file-size">${formatFileSize(file.size)}</div>
                </div>
            </div>
            <button type="button" class="remove-file-btn" onclick="removeFile(${index})">Remove</button>
        </div>
    `).join('');
}

/**
 * Remove file from upload list
 * @param {number} index - File index
 */
function removeFile(index) {
    uploadedFiles.splice(index, 1);
    renderFileList();
}

/**
 * Format file size
 * @param {number} bytes - File size in bytes
 * @returns {string} Formatted size
 */
function formatFileSize(bytes) {
    if (bytes === 0) return '0 Bytes';
    const k = 1024;
    const sizes = ['Bytes', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return Math.round(bytes / Math.pow(k, i) * 100) / 100 + ' ' + sizes[i];
}

/**
 * Escape HTML to prevent XSS
 * @param {string} text - Text to escape
 * @returns {string} Escaped text
 */
function escapeHtml(text) {
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

/**
 * Select all customers
 */
function selectAllCustomers() {
    const checkboxes = document.querySelectorAll('input[name="customers"]');
    checkboxes.forEach(checkbox => {
        checkbox.checked = true;
    });
}

/**
 * Clear all customers
 */
function clearAllCustomers() {
    const checkboxes = document.querySelectorAll('input[name="customers"]');
    checkboxes.forEach(checkbox => {
        checkbox.checked = false;
    });
}

/**
 * Clear form
 */
function clearForm() {
    document.getElementById('announcementForm').reset();
    uploadedFiles = [];
    renderFileList();
    currentAnnouncementId = null;
    currentAnnouncementType = null;
    document.getElementById('announcementIdDisplay').style.display = 'none';
    document.getElementById('meetingDetails').style.display = 'none';
}

/**
 * Save draft
 */
async function saveDraft() {
    if (uploadInProgress) {
        showStatus('Upload already in progress', 'warning');
        return;
    }

    try {
        uploadInProgress = true;
        showStatus('Saving draft...', 'info');

        const announcementData = collectFormData();
        announcementData.status = 'draft';
        announcementData.modifications = [{
            timestamp: new Date().toISOString(),
            user_id: getCurrentUserId(),
            modification_type: 'created'
        }];

        // Upload files first
        if (uploadedFiles.length > 0) {
            await uploadFiles(announcementData.announcement_id);
        }

        // Save to S3
        await saveToS3(announcementData);

        showStatus('Draft saved successfully!', 'success');

        // Redirect to announcements page with drafts filter after delay
        setTimeout(() => {
            window.location.href = 'announcements.html?status=draft';
        }, 1500);

    } catch (error) {
        console.error('Error saving draft:', error);
        showStatus('Error saving draft: ' + error.message, 'error');
    } finally {
        uploadInProgress = false;
    }
}

/**
 * Handle form submission
 */
async function handleFormSubmit(event) {
    event.preventDefault();

    if (uploadInProgress) {
        showStatus('Upload already in progress', 'warning');
        return;
    }

    // Validate form
    if (!validateForm()) {
        return;
    }

    try {
        uploadInProgress = true;
        showStatus('Submitting announcement for approval...', 'info');

        const announcementData = collectFormData();
        announcementData.status = 'submitted';
        announcementData.modifications = [
            {
                timestamp: new Date().toISOString(),
                user_id: getCurrentUserId(),
                modification_type: 'created'
            },
            {
                timestamp: new Date().toISOString(),
                user_id: getCurrentUserId(),
                modification_type: 'submitted'
            }
        ];

        // Upload files first
        if (uploadedFiles.length > 0) {
            await uploadFiles(announcementData.announcement_id);
        }

        // Save to S3
        await saveToS3(announcementData);

        showStatus('Announcement submitted successfully!', 'success');

        // Redirect after delay to the requesting approval filter
        setTimeout(() => {
            window.location.href = 'announcements.html?status=submitted';
        }, 2000);

    } catch (error) {
        console.error('Error submitting announcement:', error);
        showStatus('Error submitting announcement: ' + error.message, 'error');
    } finally {
        uploadInProgress = false;
    }
}

/**
 * Validate form
 * @returns {boolean} True if valid
 */
function validateForm() {
    // Check announcement type
    if (!currentAnnouncementType) {
        showStatus('Please select an announcement type', 'error');
        return false;
    }

    // Check title
    const title = document.getElementById('announcementTitle').value.trim();
    if (!title) {
        showStatus('Please enter an announcement title', 'error');
        return false;
    }

    // Check summary
    const summary = document.getElementById('announcementSummary').value.trim();
    if (!summary) {
        showStatus('Please enter an announcement summary', 'error');
        return false;
    }

    // Check content
    const content = document.getElementById('announcementContent').value.trim();
    if (!content) {
        showStatus('Please enter announcement content', 'error');
        return false;
    }

    // Check customers
    const customers = getSelectedCustomers();
    if (customers.length === 0) {
        showStatus('Please select at least one customer', 'error');
        return false;
    }

    // Check meeting details if meeting is required
    const meetingRequired = document.getElementById('meetingRequired').value === 'yes';
    if (meetingRequired) {
        const meetingDate = document.getElementById('meetingDate').value;
        if (!meetingDate) {
            showStatus('Please enter a meeting date', 'error');
            return false;
        }
    }

    return true;
}

/**
 * Collect form data
 * @returns {Object} Announcement data
 */
function collectFormData() {
    const meetingRequired = document.getElementById('meetingRequired').value === 'yes';

    const now = new Date().toISOString();

    const data = {
        object_type: `announcement_${currentAnnouncementType}`,
        announcement_id: currentAnnouncementId,
        announcement_type: currentAnnouncementType,
        title: document.getElementById('announcementTitle').value.trim(),
        summary: document.getElementById('announcementSummary').value.trim(),
        content: document.getElementById('announcementContent').value.trim(),
        customers: getSelectedCustomers(),
        include_meeting: meetingRequired,
        meeting_metadata: null,
        attachments: uploadedFiles.map(file => ({
            name: file.name,
            s3_key: `announcements/${currentAnnouncementId}/attachments/${file.name}`,
            size: file.size,
            uploaded_at: now
        })),
        created_by: getCurrentUserId(),
        created_at: now,
        posted_date: now,  // Add posted_date for announcements page
        author: getCurrentUserId()  // Add author field
    };

    // Add meeting details if required - use flat fields like changes
    if (meetingRequired) {
        data.meeting_title = document.getElementById('meetingTitle').value.trim();
        
        // Parse datetime-local value and add timezone information
        const meetingDateValue = document.getElementById('meetingDate').value;
        const meetingTimezone = document.getElementById('meetingTimezone').value;
        
        if (meetingDateValue) {
            // datetime-local returns format like "2025-10-30T12:00"
            // Parse it in the selected timezone and convert to ISO string
            try {
                // Create a date string with timezone
                const dateTimeString = `${meetingDateValue}:00`; // Add seconds
                const localDate = new Date(dateTimeString);
                
                // Get timezone offset for the selected timezone
                // For now, convert to ISO string (UTC) - backend will handle timezone conversion
                data.meeting_date = localDate.toISOString();
                data.meeting_timezone = meetingTimezone;
            } catch (error) {
                console.error('Error parsing meeting date:', error);
                data.meeting_date = meetingDateValue;
                data.meeting_timezone = meetingTimezone;
            }
        }
        
        data.meeting_duration = parseInt(document.getElementById('meetingDuration').value);
        data.attendees = document.getElementById('attendees').value.split(',').map(e => e.trim()).filter(e => e).join(',');
        data.meeting_location = document.getElementById('meetingLocation')?.value || '';
    }

    return data;
}

/**
 * Get selected customers
 * @returns {string[]} Array of customer codes
 */
function getSelectedCustomers() {
    const checkboxes = document.querySelectorAll('input[name="customers"]:checked');
    return Array.from(checkboxes).map(cb => cb.value);
}

/**
 * Upload files to S3
 * @param {string} announcementId - Announcement ID
 */
async function uploadFiles(announcementId) {
    // This would integrate with the actual S3 upload mechanism
    // For now, just simulate the upload
    console.log('Uploading files for announcement:', announcementId);

    for (const file of uploadedFiles) {
        const s3Key = `announcements/${announcementId}/attachments/${file.name}`;
        console.log('Uploading file:', file.name, 'to', s3Key);
        // Actual S3 upload would happen here
    }
}

/**
 * Save announcement to S3
 * @param {Object} announcementData - Announcement data
 */
async function saveToS3(announcementData) {
    try {
        // Use the same upload endpoint as changes
        const response = await fetch(`${window.location.origin}/upload`, {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            credentials: 'same-origin',
            body: JSON.stringify(announcementData)
        });

        if (!response.ok) {
            const errorText = await response.text();
            throw new Error(`Failed to save announcement: ${response.statusText} - ${errorText}`);
        }

        const result = await response.json();
        console.log('✅ Announcement saved successfully:', result);
        return result;
    } catch (error) {
        console.error('❌ Error saving announcement to S3:', error);
        throw error;
    }
}

/**
 * Get current user ID
 * @returns {string} User ID
 */
function getCurrentUserId() {
    // Get the actual user email from portal authentication
    return window.portal?.currentUser || 'Unknown';
}

/**
 * Load user info
 */
function loadUserInfo() {
    // This would load actual user info from authentication
    const userInfo = document.getElementById('userInfo');
    if (userInfo) {
        userInfo.textContent = 'User Name';
    }
}

/**
 * Show status message
 * @param {string} message - Message to show
 * @param {string} type - Message type (success, error, warning, info)
 */
function showStatus(message, type) {
    const statusContainer = document.getElementById('uploadStatus');
    const messageElement = document.getElementById('uploadMessage');

    if (statusContainer && messageElement) {
        messageElement.textContent = message;
        statusContainer.className = `upload-status ${type}`;
        statusContainer.style.display = 'block';

        // Auto-hide after 5 seconds for success messages
        if (type === 'success') {
            setTimeout(() => {
                statusContainer.style.display = 'none';
            }, 5000);
        }
    }
}
