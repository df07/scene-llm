class SceneLLMChat {
    constructor() {
        this.sessionId = null;
        this.eventSource = null;
        this.isConnected = false;
        this.isProcessing = false;

        this.initializeElements();
        this.attachEventListeners();
        this.startNewSession();
    }

    initializeElements() {
        this.messagesContainer = document.getElementById('messagesContainer');
        this.messageInput = document.getElementById('messageInput');
        this.sendButton = document.getElementById('sendButton');
        this.chatForm = document.getElementById('chatForm');
        this.connectionStatus = document.getElementById('connectionStatus');
        this.statusIndicator = document.getElementById('statusIndicator');
        this.statusText = document.getElementById('statusText');
        this.clearChatButton = document.getElementById('clearChat');
        this.scenePreview = document.getElementById('scenePreview');
    }

    attachEventListeners() {
        this.chatForm.addEventListener('submit', (e) => this.handleSubmit(e));
        this.clearChatButton.addEventListener('click', () => this.clearChat());

        // Auto-resize input and enable send on Enter
        this.messageInput.addEventListener('keydown', (e) => {
            if (e.key === 'Enter' && !e.shiftKey) {
                e.preventDefault();
                this.chatForm.dispatchEvent(new Event('submit'));
            }
        });
    }

    async startNewSession() {
        // Start with a fresh session
        this.sessionId = null;
        this.connectSSE();
    }

    connectSSE() {
        if (this.eventSource) {
            this.eventSource.close();
        }

        if (!this.sessionId) {
            // We'll get session ID from first message
            this.updateConnectionStatus('waiting', 'Ready to chat');
            return;
        }

        const url = `/api/chat/stream?session_id=${this.sessionId}`;
        this.eventSource = new EventSource(url);

        this.eventSource.onopen = () => {
            this.isConnected = true;
            this.updateConnectionStatus('connected', 'Connected');
        };

        this.eventSource.onmessage = (event) => {
            try {
                const data = JSON.parse(event.data);
                this.handleSSEEvent(data);
            } catch (error) {
                console.error('Failed to parse SSE event:', error);
            }
        };

        this.eventSource.onerror = () => {
            this.isConnected = false;
            this.updateConnectionStatus('disconnected', 'Connection lost');

            // Attempt to reconnect after 3 seconds
            setTimeout(() => {
                if (this.sessionId) {
                    this.connectSSE();
                }
            }, 3000);
        };
    }

    updateConnectionStatus(status, text) {
        this.statusIndicator.className = `status-indicator ${status}`;
        this.statusText.textContent = text;
    }

    async handleSubmit(e) {
        e.preventDefault();

        const message = this.messageInput.value.trim();
        if (!message || this.isProcessing) return;

        this.isProcessing = true;
        this.updateSendButton(false);

        // Clear input immediately
        this.messageInput.value = '';

        try {
            // Send message to server
            const response = await fetch('/api/chat', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    session_id: this.sessionId,
                    message: message
                })
            });

            if (!response.ok) {
                throw new Error(`HTTP ${response.status}`);
            }

            const data = await response.json();

            if (data.status === 'error') {
                throw new Error(data.error);
            }

            // Set session ID if this is the first message
            if (!this.sessionId && data.session_id) {
                this.sessionId = data.session_id;
                this.connectSSE();
            }

            // Add user message to conversation
            // Note: We add it immediately for better UX, but the server
            // maintains the canonical conversation order
            this.addMessage('user', message);

            // Show thinking indicator
            this.addThinkingMessage();

        } catch (error) {
            console.error('Failed to send message:', error);
            this.addMessage('system', `Error: ${error.message}`);
        } finally {
            this.isProcessing = false;
            this.updateSendButton(true);
        }
    }

    updateSendButton(enabled) {
        this.sendButton.disabled = !enabled;
        this.sendButton.textContent = enabled ? 'Send' : 'Sending...';
    }

    handleSSEEvent(event) {
        console.log('SSE Event:', event);

        switch (event.type) {
            case 'thinking':
                this.updateThinkingMessage(event.data);
                break;
            case 'llm_response':
                this.removeThinkingMessage();
                this.addMessage('assistant', event.data);
                break;
            case 'scene_update':
                this.handleSceneUpdate(event.data);
                break;
            case 'function_calls':
                this.handleFunctionCalls(event.data);
                break;
            case 'error':
                this.removeThinkingMessage();
                this.addMessage('system', `Error: ${event.data}`);
                break;
            case 'complete':
                this.removeThinkingMessage();
                break;
            case 'ping':
                // Keep-alive, ignore
                break;
            default:
                console.log('Unknown SSE event type:', event.type);
        }
    }

    addMessage(role, content) {
        const messageDiv = document.createElement('div');
        messageDiv.className = `message ${role}`;

        const contentDiv = document.createElement('div');
        contentDiv.className = 'message-content';

        if (typeof content === 'string') {
            // Simple text content
            contentDiv.innerHTML = this.formatMessageContent(content);
        } else if (content instanceof HTMLElement) {
            // DOM element content
            contentDiv.appendChild(content);
        } else {
            // Rich content (for future use)
            contentDiv.textContent = JSON.stringify(content);
        }

        messageDiv.appendChild(contentDiv);

        // Remove welcome message if this is the first real message
        const welcomeMessage = this.messagesContainer.querySelector('.welcome-message');
        if (welcomeMessage && role !== 'system') {
            welcomeMessage.remove();
        }

        this.messagesContainer.appendChild(messageDiv);
        this.scrollToBottom();
    }

    addThinkingMessage() {
        this.removeThinkingMessage(); // Remove any existing thinking message

        const messageDiv = document.createElement('div');
        messageDiv.className = 'message assistant thinking';
        messageDiv.id = 'thinking-message';

        const contentDiv = document.createElement('div');
        contentDiv.className = 'message-content';
        contentDiv.textContent = '🤖 Thinking...';

        messageDiv.appendChild(contentDiv);
        this.messagesContainer.appendChild(messageDiv);
        this.scrollToBottom();
    }

    updateThinkingMessage(text) {
        const thinkingMessage = document.getElementById('thinking-message');
        if (thinkingMessage) {
            const content = thinkingMessage.querySelector('.message-content');
            content.textContent = text;
        }
    }

    removeThinkingMessage() {
        const thinkingMessage = document.getElementById('thinking-message');
        if (thinkingMessage) {
            thinkingMessage.remove();
        }
    }

    formatMessageContent(content) {
        // Basic text formatting
        return content
            .replace(/\n/g, '<br>')
            .replace(/`([^`]+)`/g, '<code>$1</code>');
    }


    handleSceneUpdate(data) {
        if (data.image_base64) {
            this.displaySceneImage(data.image_base64);
        }
    }

    handleFunctionCalls(toolCallEvent) {
        // Handle the new ToolCallEvent format
        // toolCallEvent has: Operation, Success, Error, Duration, Timestamp

        // Create tool call message element
        const toolCallDiv = this.createToolCallElement(toolCallEvent);

        // Add as a agent message
        this.addMessage('assistant', toolCallDiv);
    }

    createToolCallElement(toolCallEvent) {
        const container = document.createElement('div');
        container.className = `tool-call-container ${toolCallEvent.success ? 'success' : 'error'}`;

        // Create summary line with expand/collapse button
        const summaryDiv = document.createElement('div');
        summaryDiv.className = `tool-call-summary ${toolCallEvent.success ? 'success' : 'error'}`;

        const summaryText = this.getToolCallSummary(toolCallEvent);
        const expandButton = document.createElement('button');
        expandButton.className = 'tool-call-expand';
        expandButton.textContent = '[+]';
        expandButton.setAttribute('aria-label', 'Show details');

        summaryDiv.innerHTML = `🔧 ${summaryText} `;
        summaryDiv.appendChild(expandButton);

        // Create details section (hidden by default)
        const detailsDiv = document.createElement('div');
        detailsDiv.className = 'tool-call-details';
        detailsDiv.style.display = 'none';
        detailsDiv.innerHTML = this.getToolCallDetails(toolCallEvent);

        // Toggle functionality
        let expanded = false;
        expandButton.addEventListener('click', () => {
            expanded = !expanded;
            detailsDiv.style.display = expanded ? 'block' : 'none';
            expandButton.textContent = expanded ? '[-]' : '[+]';
            expandButton.setAttribute('aria-label', expanded ? 'Hide details' : 'Show details');
        });

        container.appendChild(summaryDiv);
        container.appendChild(detailsDiv);

        return container;
    }

    getToolCallSummary(toolCallEvent) {
        const op = toolCallEvent.operation;
        const success = toolCallEvent.success;

        if (!success) {
            return `${this.getToolDisplayName(op.tool_name)} failed: ${toolCallEvent.error}`;
        }

        switch (op.tool_name) {
            case 'create_shape':
                return `Created shape: ${op.shape.id}`;
            case 'update_shape':
                // Check if ID was changed
                if (op.before && op.after && op.before.id !== op.after.id) {
                    return `Updated shape: ${op.before.id} → ${op.after.id}`;
                } else {
                    return `Updated shape: ${op.id}`;
                }
            case 'remove_shape':
                return `Removed shape: ${op.id}`;
            default:
                return `${this.getToolDisplayName(op.tool_name)} (${op.id})`;
        }
    }

    getToolDisplayName(toolName) {
        const displayNames = {
            'create_shape': 'Create Shape',
            'update_shape': 'Update Shape',
            'remove_shape': 'Remove Shape'
        };
        return displayNames[toolName] || toolName;
    }

    getToolCallDetails(toolCallEvent) {
        const op = toolCallEvent.operation;

        let details = `
            <div class="tool-call-meta">
                <strong>Function:</strong> ${op.tool_name}<br>
                <strong>Target:</strong> ${this.getOperationTarget(op) || 'N/A'}<br>
                <strong>Status:</strong> ${toolCallEvent.success ? '✓ Success' : '❌ Failed'}<br>
                <strong>Duration:</strong> ${toolCallEvent.duration}ms<br>
            </div>
        `;

        if (!toolCallEvent.success) {
            details += `<div class="tool-call-error"><strong>Error:</strong> ${toolCallEvent.error}</div>`;
        }

        // Add operation-specific details
        switch (op.tool_name) {
            case 'create_shape':
                details += this.getCreateShapeDetails(op);
                break;
            case 'update_shape':
                details += this.getUpdateShapeDetails(op);
                break;
            case 'remove_shape':
                details += this.getRemoveShapeDetails(op);
                break;
        }

        return details;
    }

    getOperationTarget(op) {
        switch (op.tool_name) {
            case 'create_shape':
                return op.shape ? op.shape.id : '';
            case 'update_shape':
            case 'remove_shape':
                return op.id;
            default:
                return '';
        }
    }

    getCreateShapeDetails(op) {
        const shape = op.shape;
        return `
            <div class="tool-call-shape-details">
                <strong>Created Shape:</strong>
                <div class="shape-properties">
                    <div>Type: ${shape.type}</div>
                    <div>Properties: <pre>${JSON.stringify(shape.properties, null, 2)}</pre></div>
                </div>
            </div>
        `;
    }

    getUpdateShapeDetails(op) {
        let details = `
            <div class="tool-call-shape-details">
                <strong>Updates:</strong> <pre>${JSON.stringify(op.updates, null, 2)}</pre>
        `;

        if (op.before && op.after) {
            details += `
                <strong>Before:</strong> <pre>${JSON.stringify(op.before, null, 2)}</pre>
                <strong>After:</strong> <pre>${JSON.stringify(op.after, null, 2)}</pre>
            `;
        }

        details += '</div>';
        return details;
    }

    getRemoveShapeDetails(op) {
        if (op.removed_shape) {
            return `
                <div class="tool-call-shape-details">
                    <strong>Removed Shape:</strong>
                    <div class="shape-properties">
                        <div>Type: ${op.removed_shape.type}</div>
                        <div>Properties: <pre>${JSON.stringify(op.removed_shape.properties, null, 2)}</pre></div>
                    </div>
                </div>
            `;
        }
        return '';
    }

    displaySceneImage(imageBase64) {
        // Remove "No scene yet" placeholder
        const placeholder = this.scenePreview.querySelector('.no-scene-placeholder');
        if (placeholder) placeholder.remove();

        // Remove loading indicator
        const loading = this.scenePreview.querySelector('.scene-loading');
        if (loading) loading.remove();

        // Remove existing image
        const existingImage = this.scenePreview.querySelector('.scene-image');
        if (existingImage) existingImage.remove();

        // Add new image
        const img = document.createElement('img');
        img.className = 'scene-image';
        img.src = `data:image/png;base64,${imageBase64}`;
        img.alt = 'Generated 3D scene';

        // Add the image to the scene preview
        this.scenePreview.appendChild(img);
    }

    scrollToBottom() {
        this.messagesContainer.scrollTop = this.messagesContainer.scrollHeight;
    }

    clearChat() {
        if (confirm('Are you sure you want to clear the chat? This will start a new session.')) {
            // Close current connection
            if (this.eventSource) {
                this.eventSource.close();
            }

            // Clear UI
            this.messagesContainer.innerHTML = `
                <div class="welcome-message">
                    <div class="message assistant">
                        <div class="message-content">
                            <p>👋 Hello! I'm your 3D scene assistant. I can help you create and modify 3D scenes using natural language.</p>
                            <p>Try saying something like:</p>
                            <ul>
                                <li>"Create a blue sphere"</li>
                                <li>"Add a red cube next to it"</li>
                                <li>"Make the sphere bigger"</li>
                            </ul>
                        </div>
                    </div>
                </div>
            `;

            this.scenePreview.innerHTML = `
                <div class="no-scene-placeholder">
                    <p>🎭 No scene yet</p>
                    <p>Start by asking me to create something!</p>
                </div>
            `;

            // Start new session
            this.startNewSession();
        }
    }
}

// Initialize chat when page loads
document.addEventListener('DOMContentLoaded', () => {
    new SceneLLMChat();
});