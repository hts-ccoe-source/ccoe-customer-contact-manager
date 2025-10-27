/**
 * Surveys Page - Typeform survey integration
 * Handles inline and popup survey embeds with hidden field population
 */

class SurveysPage {
    constructor() {
        this.surveys = [];
        this.filteredSurveys = [];
        this.currentSurvey = null;
        this.embedWidget = null;
        this.init();
    }

    async init() {
        // Check for survey ID parameter in URL (for email links)
        const urlParams = new URLSearchParams(window.location.search);
        const surveyId = urlParams.get('surveyId');
        const customerCode = urlParams.get('customerCode');
        const objectId = urlParams.get('objectId');

        if (surveyId) {
            // Popup mode for email links
            await this.loadSurveyMetadata(surveyId, customerCode, objectId);
            this.embedSurveyPopup(surveyId, customerCode, objectId);
        } else {
            // List view mode for portal browsing
            await this.loadSurveyList();
            this.populateCustomerFilter();
        }
    }

    /**
     * Load survey metadata from S3
     */
    async loadSurveyMetadata(surveyId, customerCode, objectId) {
        try {
            // Fetch survey form metadata from S3
            // Path: surveys/forms/{customer_code}/{object_id}/{timestamp}-{survey_id}.json
            const path = `/surveys/forms/${customerCode}/${objectId}`;
            const surveys = await s3Client.fetchObjects(path);

            // Find the survey with matching ID
            this.currentSurvey = surveys.find(s => s.id === surveyId);

            if (!this.currentSurvey) {
                console.warn(`Survey ${surveyId} not found in metadata`);
                // Continue anyway - we can still embed with just the ID
                this.currentSurvey = {
                    id: surveyId,
                    title: 'Feedback Survey',
                    customer_code: customerCode,
                    object_id: objectId
                };
            }
        } catch (error) {
            console.error('Error loading survey metadata:', error);
            // Continue with minimal metadata
            this.currentSurvey = {
                id: surveyId,
                title: 'Feedback Survey',
                customer_code: customerCode,
                object_id: objectId
            };
        }
    }

    /**
     * Load list of available surveys from S3
     */
    async loadSurveyList() {
        const container = document.getElementById('surveysList');
        
        try {
            // Fetch all survey forms from S3
            // Path: surveys/forms/{customer_code}/{object_id}/{timestamp}-{survey_id}.json
            const path = '/surveys/forms';
            const surveys = await s3Client.fetchObjects(path);

            this.surveys = Array.isArray(surveys) ? surveys : [];
            this.filteredSurveys = [...this.surveys];

            this.renderSurveyList();
        } catch (error) {
            console.error('Error loading survey list:', error);
            this.renderEmptyState('Error loading surveys. Please try again.');
        }
    }

    /**
     * Render survey list
     */
    renderSurveyList() {
        const container = document.getElementById('surveysList');

        if (this.filteredSurveys.length === 0) {
            this.renderEmptyState();
            return;
        }

        const html = this.filteredSurveys.map(survey => {
            const createdDate = survey.created_at ? portal.formatDate(survey.created_at) : 'Unknown';
            const customerCode = survey.customer_code || 'Unknown';
            const objectId = survey.object_id || 'Unknown';

            return `
                <div class="survey-card" onclick="surveysPage.openSurvey('${survey.id}', '${customerCode}', '${objectId}')">
                    <div class="survey-card-title">${this.escapeHtml(survey.title || 'Untitled Survey')}</div>
                    <div class="survey-card-meta">
                        Customer: ${customerCode}<br>
                        Object: ${objectId}<br>
                        Created: ${createdDate}
                    </div>
                    <div class="survey-card-actions" onclick="event.stopPropagation()">
                        <button class="survey-btn primary" onclick="surveysPage.openSurvey('${survey.id}', '${customerCode}', '${objectId}')">
                            Take Survey
                        </button>
                    </div>
                </div>
            `;
        }).join('');

        container.innerHTML = `<div class="survey-list">${html}</div>`;
    }

    /**
     * Render empty state
     */
    renderEmptyState(message = 'No surveys available') {
        const container = document.getElementById('surveysList');
        container.innerHTML = `
            <div class="empty-state">
                <div class="empty-state-icon">📋</div>
                <h3>${message}</h3>
                <p>Surveys will appear here when changes and announcements are completed</p>
            </div>
        `;
    }

    /**
     * Populate customer filter dropdown
     */
    populateCustomerFilter() {
        const select = document.getElementById('customerFilter');
        if (!select) return;

        // Get unique customer codes from surveys
        const customerCodes = [...new Set(this.surveys.map(s => s.customer_code).filter(Boolean))];
        customerCodes.sort();

        // Add customer options
        const options = customerCodes.map(code => 
            `<option value="${code}">${code}</option>`
        ).join('');

        select.innerHTML = `<option value="all">All Customers</option>${options}`;
    }

    /**
     * Filter surveys by customer
     */
    filterSurveys() {
        const customerFilter = document.getElementById('customerFilter').value;

        if (customerFilter === 'all') {
            this.filteredSurveys = [...this.surveys];
        } else {
            this.filteredSurveys = this.surveys.filter(s => s.customer_code === customerFilter);
        }

        this.renderSurveyList();
    }

    /**
     * Open survey in inline mode
     */
    openSurvey(surveyId, customerCode, objectId) {
        // Load survey metadata
        this.currentSurvey = this.surveys.find(s => s.id === surveyId) || {
            id: surveyId,
            title: 'Feedback Survey',
            customer_code: customerCode,
            object_id: objectId
        };

        // Show embed container, hide list
        document.getElementById('surveyListContainer').style.display = 'none';
        document.getElementById('surveyEmbedContainer').style.display = 'block';

        // Update header
        document.getElementById('surveyTitle').textContent = this.currentSurvey.title || 'Feedback Survey';
        document.getElementById('surveyMeta').innerHTML = `
            Customer: ${customerCode} • Object: ${objectId}
        `;

        // Embed survey inline
        this.embedSurveyInline(surveyId, customerCode, objectId);
    }

    /**
     * Close survey and return to list
     */
    closeSurvey() {
        // Clean up embed
        if (this.embedWidget) {
            const container = document.getElementById('surveyEmbed');
            container.innerHTML = '';
            this.embedWidget = null;
        }

        // Show list, hide embed
        document.getElementById('surveyListContainer').style.display = 'block';
        document.getElementById('surveyEmbedContainer').style.display = 'none';
    }

    /**
     * Embed survey inline (for portal browsing)
     */
    embedSurveyInline(surveyId, customerCode, objectId) {
        if (!window.tf || !window.tf.createWidget) {
            console.error('Typeform Embed SDK not loaded');
            portal.showStatus('Error loading survey. Please refresh the page.', 'error');
            return;
        }

        const container = document.getElementById('surveyEmbed');
        container.innerHTML = ''; // Clear previous embed

        // Get hidden field values
        const hiddenFields = this.getHiddenFields(customerCode, objectId);

        // Create inline widget
        try {
            this.embedWidget = window.tf.createWidget(surveyId, {
                container: container,
                medium: 'portal-inline',
                hidden: hiddenFields,
                onSubmit: () => {
                    console.log('Survey submitted');
                    portal.showStatus('Thank you for your feedback!', 'success');
                    // Return to list after short delay
                    setTimeout(() => {
                        this.closeSurvey();
                    }, 2000);
                }
            });
        } catch (error) {
            console.error('Error embedding survey:', error);
            portal.showStatus('Error loading survey. Please try again.', 'error');
        }
    }

    /**
     * Embed survey popup (for email links with autoclose)
     */
    embedSurveyPopup(surveyId, customerCode, objectId) {
        if (!window.tf || !window.tf.createPopup) {
            console.error('Typeform Embed SDK not loaded');
            portal.showStatus('Error loading survey. Please refresh the page.', 'error');
            return;
        }

        // Get hidden field values
        const hiddenFields = this.getHiddenFields(customerCode, objectId);

        // Create popup with autoclose
        try {
            const popup = window.tf.createPopup(surveyId, {
                medium: 'email-link',
                autoClose: 2000, // Auto-close after 2 seconds
                hidden: hiddenFields,
                size: 80, // Popup size as percentage of screen (default is 100)
                width: 800, // Fixed width in pixels
                height: 600, // Fixed height in pixels
                onSubmit: () => {
                    console.log('Survey submitted via popup');
                    portal.showStatus('Thank you for your feedback!', 'success');
                },
                onClose: () => {
                    console.log('Survey popup closed');
                    // Redirect to dashboard or show list
                    window.location.href = 'surveys.html';
                }
            });

            // Auto-open popup on page load
            popup.open();
        } catch (error) {
            console.error('Error creating survey popup:', error);
            portal.showStatus('Error loading survey. Please try again.', 'error');
        }
    }

    /**
     * Get hidden fields for survey
     */
    getHiddenFields(customerCode, objectId) {
        // Extract metadata from current survey or URL params
        const urlParams = new URLSearchParams(window.location.search);
        
        return {
            user_login: portal.currentUser || 'unknown',
            customer_code: customerCode || urlParams.get('customerCode') || 'unknown',
            year: new Date().getFullYear().toString(),
            quarter: this.getCurrentQuarter(),
            event_type: this.getEventType(objectId),
            event_subtype: this.getEventSubtype(objectId),
            object_id: objectId || urlParams.get('objectId') || 'unknown'
        };
    }

    /**
     * Get current quarter (Q1, Q2, Q3, Q4)
     */
    getCurrentQuarter() {
        const month = new Date().getMonth() + 1; // 1-12
        if (month <= 3) return 'Q1';
        if (month <= 6) return 'Q2';
        if (month <= 9) return 'Q3';
        return 'Q4';
    }

    /**
     * Get event type from object ID
     */
    getEventType(objectId) {
        if (!objectId) return 'unknown';
        
        // Check if it's an announcement (starts with CIC-, FIN-, INN-)
        if (objectId.startsWith('CIC-') || objectId.startsWith('FIN-') || objectId.startsWith('INN-')) {
            return 'announcement';
        }
        
        // Otherwise it's a change
        return 'change';
    }

    /**
     * Get event subtype from object ID
     */
    getEventSubtype(objectId) {
        if (!objectId) return 'general';
        
        if (objectId.startsWith('CIC-')) return 'cic';
        if (objectId.startsWith('FIN-')) return 'finops';
        if (objectId.startsWith('INN-')) return 'innersource';
        
        return 'general';
    }

    /**
     * Refresh survey list
     */
    async refresh() {
        portal.showStatus('Refreshing surveys...', 'info');
        await this.loadSurveyList();
        this.populateCustomerFilter();
        portal.showStatus('Surveys refreshed', 'success');
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
