/**
 * Modal Component - Reusable modal functionality
 * Provides a flexible modal system with keyboard navigation and accessibility support
 */

class Modal {
    constructor(options = {}) {
        this.options = {
            title: options.title || 'Modal',
            content: options.content || '',
            size: options.size || 'medium', // small, medium, large, full
            closeOnOverlay: options.closeOnOverlay !== false,
            closeOnEscape: options.closeOnEscape !== false,
            showCloseButton: options.showCloseButton !== false,
            onOpen: options.onOpen || null,
            onClose: options.onClose || null,
            className: options.className || ''
        };

        this.modalElement = null;
        this.isOpen = false;
        this.previousFocus = null;
    }

    /**
     * Render the modal structure
     */
    render() {
        // Create modal overlay
        const overlay = document.createElement('div');
        overlay.className = `modal-overlay ${this.options.className}`;
        overlay.setAttribute('role', 'dialog');
        overlay.setAttribute('aria-modal', 'true');
        overlay.setAttribute('aria-labelledby', 'modal-title');

        // Create modal container
        const container = document.createElement('div');
        container.className = `modal-container modal-${this.options.size}`;

        // Create modal header
        const header = document.createElement('div');
        header.className = 'modal-header';

        const title = document.createElement('h2');
        title.id = 'modal-title';
        title.className = 'modal-title';
        title.textContent = this.options.title;
        header.appendChild(title);

        // Add close button if enabled
        if (this.options.showCloseButton) {
            const closeBtn = document.createElement('button');
            closeBtn.className = 'modal-close-btn';
            closeBtn.setAttribute('aria-label', 'Close modal');
            closeBtn.innerHTML = '&times;';
            closeBtn.addEventListener('click', () => this.hide());
            header.appendChild(closeBtn);
        }

        // Create modal body
        const body = document.createElement('div');
        body.className = 'modal-body';
        
        if (typeof this.options.content === 'string') {
            body.innerHTML = this.options.content;
        } else if (this.options.content instanceof HTMLElement) {
            body.appendChild(this.options.content);
        }

        // Assemble modal
        container.appendChild(header);
        container.appendChild(body);
        overlay.appendChild(container);

        this.modalElement = overlay;

        // Add event listeners
        this.setupEventListeners();

        return overlay;
    }

    /**
     * Setup event listeners for modal interactions
     */
    setupEventListeners() {
        // Close on overlay click
        if (this.options.closeOnOverlay) {
            this.modalElement.addEventListener('click', (e) => {
                if (e.target === this.modalElement) {
                    this.hide();
                }
            });
        }

        // Close on escape key
        if (this.options.closeOnEscape) {
            this.escapeHandler = (e) => {
                if (e.key === 'Escape' && this.isOpen) {
                    this.hide();
                }
            };
            document.addEventListener('keydown', this.escapeHandler);
        }

        // Tab trapping for accessibility
        this.tabHandler = (e) => {
            if (e.key === 'Tab' && this.isOpen) {
                this.trapFocus(e);
            }
        };
        document.addEventListener('keydown', this.tabHandler);
    }

    /**
     * Trap focus within modal for accessibility
     */
    trapFocus(e) {
        const focusableElements = this.modalElement.querySelectorAll(
            'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
        );
        const firstElement = focusableElements[0];
        const lastElement = focusableElements[focusableElements.length - 1];

        if (e.shiftKey && document.activeElement === firstElement) {
            e.preventDefault();
            lastElement.focus();
        } else if (!e.shiftKey && document.activeElement === lastElement) {
            e.preventDefault();
            firstElement.focus();
        }
    }

    /**
     * Show the modal
     */
    show() {
        if (this.isOpen) return;

        // Store currently focused element
        this.previousFocus = document.activeElement;

        // Render if not already rendered
        if (!this.modalElement) {
            this.render();
        }

        // Add to DOM
        document.body.appendChild(this.modalElement);

        // Prevent body scroll
        document.body.style.overflow = 'hidden';

        // Trigger animation
        requestAnimationFrame(() => {
            this.modalElement.classList.add('modal-open');
        });

        this.isOpen = true;

        // Focus first focusable element
        const firstFocusable = this.modalElement.querySelector(
            'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])'
        );
        if (firstFocusable) {
            firstFocusable.focus();
        }

        // Call onOpen callback
        if (this.options.onOpen) {
            this.options.onOpen(this);
        }
    }

    /**
     * Hide the modal
     */
    hide() {
        if (!this.isOpen) return;

        // Remove animation class
        this.modalElement.classList.remove('modal-open');

        // Wait for animation to complete
        setTimeout(() => {
            if (this.modalElement && this.modalElement.parentNode) {
                this.modalElement.parentNode.removeChild(this.modalElement);
            }

            // Restore body scroll
            document.body.style.overflow = '';

            // Restore focus
            if (this.previousFocus) {
                this.previousFocus.focus();
            }

            this.isOpen = false;

            // Call onClose callback
            if (this.options.onClose) {
                this.options.onClose(this);
            }
        }, 300); // Match CSS transition duration
    }

    /**
     * Update modal content
     */
    updateContent(content) {
        if (!this.modalElement) return;

        const body = this.modalElement.querySelector('.modal-body');
        if (body) {
            if (typeof content === 'string') {
                body.innerHTML = content;
            } else if (content instanceof HTMLElement) {
                body.innerHTML = '';
                body.appendChild(content);
            }
        }
    }

    /**
     * Update modal title
     */
    updateTitle(title) {
        if (!this.modalElement) return;

        const titleElement = this.modalElement.querySelector('.modal-title');
        if (titleElement) {
            titleElement.textContent = title;
        }
    }

    /**
     * Destroy the modal and cleanup
     */
    destroy() {
        // Remove event listeners
        if (this.escapeHandler) {
            document.removeEventListener('keydown', this.escapeHandler);
        }
        if (this.tabHandler) {
            document.removeEventListener('keydown', this.tabHandler);
        }

        // Hide and remove from DOM
        if (this.isOpen) {
            this.hide();
        }

        this.modalElement = null;
    }
}

/**
 * Inject modal CSS styles into the page
 */
function injectModalStyles() {
    // Check if styles already injected
    if (document.getElementById('modal-styles')) return;

    const style = document.createElement('style');
    style.id = 'modal-styles';
    style.textContent = `
        /* Modal Overlay */
        .modal-overlay {
            position: fixed;
            top: 0;
            left: 0;
            right: 0;
            bottom: 0;
            background: rgba(0, 0, 0, 0.5);
            display: flex;
            align-items: center;
            justify-content: center;
            z-index: 9999;
            opacity: 0;
            transition: opacity 0.3s ease;
            padding: 20px;
        }

        .modal-overlay.modal-open {
            opacity: 1;
        }

        /* Modal Container */
        .modal-container {
            background: white;
            border-radius: 8px;
            box-shadow: 0 4px 20px rgba(0, 0, 0, 0.15);
            max-height: 90vh;
            display: flex;
            flex-direction: column;
            transform: scale(0.9);
            transition: transform 0.3s ease;
        }

        .modal-open .modal-container {
            transform: scale(1);
        }

        /* Modal Sizes */
        .modal-small {
            width: 100%;
            max-width: 400px;
        }

        .modal-medium {
            width: 100%;
            max-width: 600px;
        }

        .modal-large {
            width: 100%;
            max-width: 900px;
        }

        .modal-full {
            width: 95%;
            max-width: 1200px;
        }

        /* Modal Header */
        .modal-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 20px;
            border-bottom: 1px solid #e1e8ed;
        }

        .modal-title {
            margin: 0;
            font-size: 1.5rem;
            color: #2c3e50;
            font-weight: 600;
        }

        .modal-close-btn {
            background: none;
            border: none;
            font-size: 2rem;
            color: #6c757d;
            cursor: pointer;
            padding: 0;
            width: 32px;
            height: 32px;
            display: flex;
            align-items: center;
            justify-content: center;
            border-radius: 4px;
            transition: all 0.2s ease;
        }

        .modal-close-btn:hover {
            background: #f8f9fa;
            color: #495057;
        }

        .modal-close-btn:focus {
            outline: 2px solid #667eea;
            outline-offset: 2px;
        }

        /* Modal Body */
        .modal-body {
            padding: 20px;
            overflow-y: auto;
            flex: 1;
        }

        /* Responsive Design */
        @media (max-width: 768px) {
            .modal-overlay {
                padding: 10px;
            }

            .modal-container {
                max-height: 95vh;
            }

            .modal-header {
                padding: 15px;
            }

            .modal-title {
                font-size: 1.25rem;
            }

            .modal-body {
                padding: 15px;
            }

            .modal-small,
            .modal-medium,
            .modal-large,
            .modal-full {
                width: 100%;
                max-width: 100%;
            }
        }
    `;
    document.head.appendChild(style);
}

// Auto-inject styles when script loads
if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', injectModalStyles);
} else {
    injectModalStyles();
}

// Export for use in other modules
if (typeof module !== 'undefined' && module.exports) {
    module.exports = Modal;
}
