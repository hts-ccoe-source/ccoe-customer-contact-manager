/**
 * Loading States Module - Loading indicators and error display
 * Provides loading spinners, progress indicators, and error messages
 */

class LoadingManager {
    constructor(options = {}) {
        this.options = {
            container: options.container || document.body,
            spinnerSize: options.spinnerSize || 'medium', // small, medium, large
            showOverlay: options.showOverlay !== false,
            overlayOpacity: options.overlayOpacity || 0.5
        };

        this.loadingElement = null;
        this.isLoading = false;
    }

    /**
     * Show loading indicator
     */
    showLoading(message = 'Loading...') {
        if (this.isLoading) return;

        this.loadingElement = this.createLoadingElement(message);
        
        if (typeof this.options.container === 'string') {
            const container = document.querySelector(this.options.container);
            if (container) {
                container.appendChild(this.loadingElement);
            }
        } else {
            this.options.container.appendChild(this.loadingElement);
        }

        this.isLoading = true;
    }

    /**
     * Hide loading indicator
     */
    hideLoading() {
        if (!this.isLoading || !this.loadingElement) return;

        if (this.loadingElement.parentNode) {
            this.loadingElement.parentNode.removeChild(this.loadingElement);
        }

        this.loadingElement = null;
        this.isLoading = false;
    }

    /**
     * Update loading message
     */
    updateMessage(message) {
        if (!this.loadingElement) return;

        const messageElement = this.loadingElement.querySelector('.loading-message');
        if (messageElement) {
            messageElement.textContent = message;
        }
    }

    /**
     * Create loading element
     */
    createLoadingElement(message) {
        const container = document.createElement('div');
        container.className = `loading-container ${this.options.showOverlay ? 'with-overlay' : ''}`;

        const spinner = document.createElement('div');
        spinner.className = `loading-spinner spinner-${this.options.spinnerSize}`;

        const messageElement = document.createElement('div');
        messageElement.className = 'loading-message';
        messageElement.textContent = message;

        container.appendChild(spinner);
        container.appendChild(messageElement);

        return container;
    }
}

/**
 * Show progress indicator
 */
function showProgress(container, current, total, message = '') {
    const containerId = typeof container === 'string' ? container : container.id;
    let progressElement = document.getElementById(`progress-${containerId}`);

    if (!progressElement) {
        progressElement = createProgressElement(containerId);
        
        const targetContainer = typeof container === 'string' 
            ? document.getElementById(container) || document.querySelector(container)
            : container;
            
        if (targetContainer) {
            targetContainer.appendChild(progressElement);
        }
    }

    // Update progress
    const percentage = Math.round((current / total) * 100);
    const progressBar = progressElement.querySelector('.progress-bar-fill');
    const progressText = progressElement.querySelector('.progress-text');

    if (progressBar) {
        progressBar.style.width = `${percentage}%`;
    }

    if (progressText) {
        progressText.textContent = message || `${current} / ${total} (${percentage}%)`;
    }

    return progressElement;
}

/**
 * Hide progress indicator
 */
function hideProgress(container) {
    const containerId = typeof container === 'string' ? container : container.id;
    const progressElement = document.getElementById(`progress-${containerId}`);

    if (progressElement && progressElement.parentNode) {
        progressElement.parentNode.removeChild(progressElement);
    }
}

/**
 * Create progress element
 */
function createProgressElement(containerId) {
    const container = document.createElement('div');
    container.id = `progress-${containerId}`;
    container.className = 'progress-container';

    const progressBar = document.createElement('div');
    progressBar.className = 'progress-bar';

    const progressFill = document.createElement('div');
    progressFill.className = 'progress-bar-fill';

    const progressText = document.createElement('div');
    progressText.className = 'progress-text';
    progressText.textContent = '0%';

    progressBar.appendChild(progressFill);
    container.appendChild(progressBar);
    container.appendChild(progressText);

    return container;
}

/**
 * Show error message
 */
function showError(container, message, options = {}) {
    return showMessage(container, message, 'error', options);
}

/**
 * Show success message
 */
function showSuccess(container, message, options = {}) {
    return showMessage(container, message, 'success', options);
}

/**
 * Show warning message
 */
function showWarning(container, message, options = {}) {
    return showMessage(container, message, 'warning', options);
}

/**
 * Show info message
 */
function showInfo(container, message, options = {}) {
    return showMessage(container, message, 'info', options);
}

/**
 * Show message with type
 */
function showMessage(container, message, type = 'info', options = {}) {
    const {
        duration = 5000,
        dismissible = true,
        icon = true,
        onDismiss = null
    } = options;

    const messageElement = createMessageElement(message, type, dismissible, icon);

    const targetContainer = typeof container === 'string'
        ? document.getElementById(container) || document.querySelector(container)
        : container;

    if (targetContainer) {
        targetContainer.appendChild(messageElement);

        // Auto-dismiss after duration
        if (duration > 0) {
            setTimeout(() => {
                dismissMessage(messageElement, onDismiss);
            }, duration);
        }

        // Add dismiss handler
        if (dismissible) {
            const dismissBtn = messageElement.querySelector('.message-dismiss');
            if (dismissBtn) {
                dismissBtn.addEventListener('click', () => {
                    dismissMessage(messageElement, onDismiss);
                });
            }
        }
    }

    return messageElement;
}

/**
 * Create message element
 */
function createMessageElement(message, type, dismissible, showIcon) {
    const container = document.createElement('div');
    container.className = `message message-${type}`;

    if (showIcon) {
        const icon = document.createElement('span');
        icon.className = 'message-icon';
        icon.textContent = getMessageIcon(type);
        container.appendChild(icon);
    }

    const content = document.createElement('div');
    content.className = 'message-content';
    content.textContent = message;
    container.appendChild(content);

    if (dismissible) {
        const dismissBtn = document.createElement('button');
        dismissBtn.className = 'message-dismiss';
        dismissBtn.innerHTML = '&times;';
        dismissBtn.setAttribute('aria-label', 'Dismiss message');
        container.appendChild(dismissBtn);
    }

    return container;
}

/**
 * Get icon for message type
 */
function getMessageIcon(type) {
    const icons = {
        error: '❌',
        success: '✅',
        warning: '⚠️',
        info: 'ℹ️'
    };
    return icons[type] || 'ℹ️';
}

/**
 * Dismiss message
 */
function dismissMessage(messageElement, callback) {
    messageElement.classList.add('message-dismissing');

    setTimeout(() => {
        if (messageElement.parentNode) {
            messageElement.parentNode.removeChild(messageElement);
        }

        if (callback) {
            callback();
        }
    }, 300);
}

/**
 * Clear all messages in container
 */
function clearMessages(container) {
    const targetContainer = typeof container === 'string'
        ? document.getElementById(container) || document.querySelector(container)
        : container;

    if (targetContainer) {
        const messages = targetContainer.querySelectorAll('.message');
        messages.forEach(msg => {
            if (msg.parentNode) {
                msg.parentNode.removeChild(msg);
            }
        });
    }
}

/**
 * Inject loading styles into the page
 */
function injectLoadingStyles() {
    // Check if styles already injected
    if (document.getElementById('loading-styles')) return;

    const style = document.createElement('style');
    style.id = 'loading-styles';
    style.textContent = `
        /* Loading Container */
        .loading-container {
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;
            padding: 40px 20px;
            gap: 15px;
        }

        .loading-container.with-overlay {
            position: fixed;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background: rgba(255, 255, 255, 0.9);
            z-index: 9998;
        }

        /* Loading Spinner */
        .loading-spinner {
            border: 3px solid #f3f3f3;
            border-top: 3px solid #667eea;
            border-radius: 50%;
            animation: spin 1s linear infinite;
        }

        .spinner-small {
            width: 24px;
            height: 24px;
            border-width: 2px;
        }

        .spinner-medium {
            width: 40px;
            height: 40px;
        }

        .spinner-large {
            width: 60px;
            height: 60px;
            border-width: 4px;
        }

        @keyframes spin {
            0% { transform: rotate(0deg); }
            100% { transform: rotate(360deg); }
        }

        /* Loading Message */
        .loading-message {
            color: #495057;
            font-size: 0.95rem;
            text-align: center;
        }

        /* Progress Container */
        .progress-container {
            margin: 20px 0;
        }

        .progress-bar {
            width: 100%;
            height: 24px;
            background: #e9ecef;
            border-radius: 12px;
            overflow: hidden;
            margin-bottom: 8px;
        }

        .progress-bar-fill {
            height: 100%;
            background: linear-gradient(90deg, #667eea, #764ba2);
            transition: width 0.3s ease;
            display: flex;
            align-items: center;
            justify-content: center;
            color: white;
            font-size: 0.85rem;
            font-weight: 600;
        }

        .progress-text {
            text-align: center;
            color: #6c757d;
            font-size: 0.9rem;
        }

        /* Message Styles */
        .message {
            display: flex;
            align-items: center;
            gap: 12px;
            padding: 12px 16px;
            border-radius: 8px;
            margin-bottom: 10px;
            border-left: 4px solid;
            animation: slideIn 0.3s ease;
        }

        .message-dismissing {
            animation: slideOut 0.3s ease;
        }

        @keyframes slideIn {
            from {
                opacity: 0;
                transform: translateY(-10px);
            }
            to {
                opacity: 1;
                transform: translateY(0);
            }
        }

        @keyframes slideOut {
            from {
                opacity: 1;
                transform: translateY(0);
            }
            to {
                opacity: 0;
                transform: translateY(-10px);
            }
        }

        .message-error {
            background: #f8d7da;
            border-left-color: #dc3545;
            color: #721c24;
        }

        .message-success {
            background: #d4edda;
            border-left-color: #28a745;
            color: #155724;
        }

        .message-warning {
            background: #fff3cd;
            border-left-color: #ffc107;
            color: #856404;
        }

        .message-info {
            background: #d1ecf1;
            border-left-color: #17a2b8;
            color: #0c5460;
        }

        .message-icon {
            font-size: 1.2rem;
            flex-shrink: 0;
        }

        .message-content {
            flex: 1;
            font-size: 0.95rem;
        }

        .message-dismiss {
            background: none;
            border: none;
            font-size: 1.5rem;
            color: inherit;
            opacity: 0.6;
            cursor: pointer;
            padding: 0;
            width: 24px;
            height: 24px;
            display: flex;
            align-items: center;
            justify-content: center;
            border-radius: 4px;
            transition: all 0.2s ease;
        }

        .message-dismiss:hover {
            opacity: 1;
            background: rgba(0, 0, 0, 0.1);
        }

        /* Responsive Design */
        @media (max-width: 768px) {
            .loading-container {
                padding: 30px 15px;
            }

            .message {
                padding: 10px 12px;
                font-size: 0.9rem;
            }
        }
    `;
    document.head.appendChild(style);
}

// Auto-inject styles when script loads
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', injectLoadingStyles);
} else {
    injectLoadingStyles();
}

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
    module.exports = {
        LoadingManager,
        showProgress,
        hideProgress,
        showError,
        showSuccess,
        showWarning,
        showInfo,
        showMessage,
        clearMessages
    };
}
