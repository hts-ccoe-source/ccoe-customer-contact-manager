/**
 * Timeline Component - Modification history display
 * Renders modification history arrays with visual timeline and icons
 */

class Timeline {
    constructor(options = {}) {
        this.options = {
            modifications: options.modifications || [],
            showUserInfo: options.showUserInfo !== false,
            showTimestamps: options.showTimestamps !== false,
            reverseOrder: options.reverseOrder !== false, // Most recent first by default
            maxItems: options.maxItems || null,
            className: options.className || ''
        };
    }

    /**
     * Get icon for modification type
     */
    getModificationIcon(type) {
        const icons = {
            'created': '📝',
            'updated': '✏️',
            'submitted': '📋',
            'approved': '✅',
            'deleted': '🗑️',
            'meeting_scheduled': '📅',
            'meeting_cancelled': '❌'
        };
        return icons[type] || '📄';
    }

    /**
     * Get color for modification type
     */
    getModificationColor(type) {
        const colors = {
            'created': '#28a745',
            'updated': '#17a2b8',
            'submitted': '#ffc107',
            'approved': '#28a745',
            'deleted': '#dc3545',
            'meeting_scheduled': '#667eea',
            'meeting_cancelled': '#dc3545'
        };
        return colors[type] || '#6c757d';
    }

    /**
     * Format modification type for display
     */
    formatModificationType(type) {
        const labels = {
            'created': 'Created',
            'updated': 'Updated',
            'submitted': 'Submitted',
            'approved': 'Approved',
            'deleted': 'Deleted',
            'meeting_scheduled': 'Meeting Scheduled',
            'meeting_cancelled': 'Meeting Cancelled'
        };
        return labels[type] || type;
    }

    /**
     * Format timestamp for display
     */
    formatTimestamp(timestamp) {
        const date = new Date(timestamp);
        const now = new Date();
        const diffMs = now - date;
        const diffMins = Math.floor(diffMs / 60000);
        const diffHours = Math.floor(diffMs / 3600000);
        const diffDays = Math.floor(diffMs / 86400000);

        // Relative time for recent modifications
        if (diffMins < 1) {
            return 'Just now';
        } else if (diffMins < 60) {
            return `${diffMins} minute${diffMins > 1 ? 's' : ''} ago`;
        } else if (diffHours < 24) {
            return `${diffHours} hour${diffHours > 1 ? 's' : ''} ago`;
        } else if (diffDays < 7) {
            return `${diffDays} day${diffDays > 1 ? 's' : ''} ago`;
        }

        // Absolute time for older modifications
        return date.toLocaleDateString() + ' ' + date.toLocaleTimeString([], { 
            hour: '2-digit', 
            minute: '2-digit' 
        });
    }

    /**
     * Format user ID for display
     */
    formatUserId(userId) {
        if (!userId) return 'Unknown';
        
        // Extract email from user ID if it looks like an email
        if (userId.includes('@')) {
            return userId;
        }
        
        // Extract name from ARN if it's an IAM role ARN
        if (userId.startsWith('arn:aws:iam::')) {
            const parts = userId.split('/');
            return parts[parts.length - 1] || userId;
        }
        
        // Return as-is for other formats
        return userId;
    }

    /**
     * Render a single timeline entry
     */
    renderEntry(modification) {
        const entry = document.createElement('div');
        entry.className = 'timeline-entry';
        entry.setAttribute('data-type', modification.modification_type);

        const icon = document.createElement('div');
        icon.className = 'timeline-icon';
        icon.style.backgroundColor = this.getModificationColor(modification.modification_type);
        icon.textContent = this.getModificationIcon(modification.modification_type);

        const content = document.createElement('div');
        content.className = 'timeline-content';

        const header = document.createElement('div');
        header.className = 'timeline-header';

        const type = document.createElement('span');
        type.className = 'timeline-type';
        type.textContent = this.formatModificationType(modification.modification_type);
        header.appendChild(type);

        if (this.options.showTimestamps) {
            const timestamp = document.createElement('span');
            timestamp.className = 'timeline-timestamp';
            timestamp.textContent = this.formatTimestamp(modification.timestamp);
            timestamp.title = new Date(modification.timestamp).toLocaleString();
            header.appendChild(timestamp);
        }

        content.appendChild(header);

        if (this.options.showUserInfo && modification.user_id) {
            const user = document.createElement('div');
            user.className = 'timeline-user';
            user.textContent = `by ${this.formatUserId(modification.user_id)}`;
            content.appendChild(user);
        }

        // Add meeting metadata if present
        if (modification.meeting_metadata) {
            const meetingInfo = this.renderMeetingMetadata(modification.meeting_metadata);
            content.appendChild(meetingInfo);
        }

        entry.appendChild(icon);
        entry.appendChild(content);

        return entry;
    }

    /**
     * Render meeting metadata
     */
    renderMeetingMetadata(metadata) {
        const container = document.createElement('div');
        container.className = 'timeline-meeting-info';

        if (metadata.subject) {
            const subject = document.createElement('div');
            subject.className = 'meeting-subject';
            subject.textContent = metadata.subject;
            container.appendChild(subject);
        }

        if (metadata.join_url) {
            const link = document.createElement('a');
            link.href = metadata.join_url;
            link.target = '_blank';
            link.rel = 'noopener noreferrer';
            link.className = 'meeting-link';
            link.textContent = '🔗 Join Meeting';
            container.appendChild(link);
        }

        if (metadata.start_time) {
            const time = document.createElement('div');
            time.className = 'meeting-time';
            const startDate = new Date(metadata.start_time);
            time.textContent = `📅 ${startDate.toLocaleString()}`;
            container.appendChild(time);
        }

        return container;
    }

    /**
     * Render the complete timeline
     */
    render() {
        const container = document.createElement('div');
        container.className = `timeline ${this.options.className}`;

        let modifications = [...this.options.modifications];

        // Sort modifications
        if (this.options.reverseOrder) {
            modifications.sort((a, b) => new Date(b.timestamp) - new Date(a.timestamp));
        } else {
            modifications.sort((a, b) => new Date(a.timestamp) - new Date(b.timestamp));
        }

        // Limit items if specified
        if (this.options.maxItems) {
            modifications = modifications.slice(0, this.options.maxItems);
        }

        // Render entries
        if (modifications.length === 0) {
            const empty = document.createElement('div');
            empty.className = 'timeline-empty';
            empty.textContent = 'No modification history available';
            container.appendChild(empty);
        } else {
            modifications.forEach(mod => {
                container.appendChild(this.renderEntry(mod));
            });
        }

        return container;
    }

    /**
     * Update timeline with new modifications
     */
    update(modifications) {
        this.options.modifications = modifications;
    }
}

/**
 * Inject timeline CSS styles into the page
 */
function injectTimelineStyles() {
    // Check if styles already injected
    if (document.getElementById('timeline-styles')) return;

    const style = document.createElement('style');
    style.id = 'timeline-styles';
    style.textContent = `
        /* Timeline Container */
        .timeline {
            position: relative;
            padding: 20px 0;
        }

        .timeline::before {
            content: '';
            position: absolute;
            left: 15px;
            top: 0;
            bottom: 0;
            width: 2px;
            background: #e1e8ed;
        }

        /* Timeline Entry */
        .timeline-entry {
            position: relative;
            display: flex;
            gap: 15px;
            margin-bottom: 20px;
            padding-left: 10px;
        }

        .timeline-entry:last-child {
            margin-bottom: 0;
        }

        /* Timeline Icon */
        .timeline-icon {
            flex-shrink: 0;
            width: 32px;
            height: 32px;
            border-radius: 50%;
            display: flex;
            align-items: center;
            justify-content: center;
            font-size: 1rem;
            background: #667eea;
            color: white;
            z-index: 1;
            box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
        }

        /* Timeline Content */
        .timeline-content {
            flex: 1;
            background: #f8f9fa;
            border-radius: 8px;
            padding: 12px 15px;
            border: 1px solid #e1e8ed;
        }

        .timeline-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 5px;
        }

        .timeline-type {
            font-weight: 600;
            color: #2c3e50;
            font-size: 0.95rem;
        }

        .timeline-timestamp {
            font-size: 0.85rem;
            color: #6c757d;
        }

        .timeline-user {
            font-size: 0.85rem;
            color: #6c757d;
            font-style: italic;
        }

        /* Meeting Info */
        .timeline-meeting-info {
            margin-top: 10px;
            padding-top: 10px;
            border-top: 1px solid #dee2e6;
        }

        .meeting-subject {
            font-weight: 500;
            color: #495057;
            margin-bottom: 5px;
        }

        .meeting-link {
            display: inline-block;
            color: #667eea;
            text-decoration: none;
            font-size: 0.9rem;
            margin: 5px 0;
            transition: color 0.2s ease;
        }

        .meeting-link:hover {
            color: #5568d3;
            text-decoration: underline;
        }

        .meeting-time {
            font-size: 0.85rem;
            color: #6c757d;
            margin-top: 5px;
        }

        /* Empty State */
        .timeline-empty {
            text-align: center;
            padding: 40px 20px;
            color: #6c757d;
            font-style: italic;
        }

        /* Responsive Design */
        @media (max-width: 768px) {
            .timeline::before {
                left: 10px;
            }

            .timeline-entry {
                gap: 10px;
            }

            .timeline-icon {
                width: 28px;
                height: 28px;
                font-size: 0.9rem;
            }

            .timeline-content {
                padding: 10px 12px;
            }

            .timeline-header {
                flex-direction: column;
                align-items: flex-start;
                gap: 5px;
            }
        }
    `;
    document.head.appendChild(style);
}

// Auto-inject styles when script loads
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', injectTimelineStyles);
} else {
    injectTimelineStyles();
}

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
    module.exports = Timeline;
}
