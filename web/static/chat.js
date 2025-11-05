class SceneLLMChat {
    constructor() {
        this.sessionId = null;
        this.eventSource = null;
        this.isConnected = false;
        this.isProcessing = false;
        this.renderQuality = 'draft'; // default to fast/draft quality

        this.initializeTheme();
        this.initializeQuality();
        this.initializeElements();
        this.attachEventListeners();
        this.startNewSession();
    }

    initializeTheme() {
        // Load saved theme or default to light
        const savedTheme = localStorage.getItem('scene-llm-theme') || 'light';
        this.setTheme(savedTheme);
    }

    initializeQuality() {
        // Always intialize quality to draft
        this.setQuality('draft');
    }

    setTheme(theme) {
        // Update document theme
        document.documentElement.setAttribute('data-theme', theme);

        // Update theme toggle buttons
        document.querySelectorAll('.theme-toggle').forEach(toggle => {
            if (toggle.dataset.theme === theme) {
                toggle.classList.add('active');
            } else {
                toggle.classList.remove('active');
            }
        });

        // Save theme preference
        localStorage.setItem('scene-llm-theme', theme);
    }

    initializeElements() {
        this.messagesContainer = document.getElementById('messagesContainer');
        this.messageInput = document.getElementById('messageInput');
        this.sendButton = document.getElementById('sendButton');
        this.stopButton = document.getElementById('stopButton');
        this.chatForm = document.getElementById('chatForm');
        this.connectionStatus = document.getElementById('connectionStatus');
        this.statusIndicator = document.getElementById('statusIndicator');
        this.statusText = document.getElementById('statusText');
        this.clearChatButton = document.getElementById('clearChat');
        this.scenePreview = document.getElementById('scenePreview');
    }

    setQuality(quality) {
        this.renderQuality = quality;

        // Update quality toggle buttons
        document.querySelectorAll('.quality-switcher .quality-toggle').forEach(toggle => {
            if (toggle.dataset.quality === quality) {
                toggle.classList.add('active');
            } else {
                toggle.classList.remove('active');
            }
        });

        // If there's an existing scene, re-render it with the new quality
        this.triggerSceneRerender();
    }

    attachEventListeners() {
        this.chatForm.addEventListener('submit', (e) => this.handleSubmit(e));
        this.stopButton.addEventListener('click', () => this.handleStop());
        this.clearChatButton.addEventListener('click', () => this.clearChat());

        // Add theme toggle handlers
        document.querySelectorAll('.theme-toggle').forEach(toggle => {
            toggle.addEventListener('click', () => {
                this.setTheme(toggle.dataset.theme);
            });
        });

        // Add quality toggle handlers
        document.querySelectorAll('.quality-switcher .quality-toggle').forEach(toggle => {
            toggle.addEventListener('click', () => {
                this.setQuality(toggle.dataset.quality);
            });
        });

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
        this.updateInputState(false);

        // Clear input immediately
        this.messageInput.value = '';

        try {
            // Send message to server with current quality preference
            const requestBody = {
                session_id: this.sessionId,
                message: message,
                quality: this.renderQuality
            };

            console.log('Sending chat message:', requestBody);

            const response = await fetch('/api/chat', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify(requestBody)
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

            // Show processing indicator
            this.addProcessingMessage();
            // Keep isProcessing true - will be cleared by complete/error events

        } catch (error) {
            console.error('Failed to send message:', error);
            this.addMessage('system', `Error: ${error.message}`);
            this.isProcessing = false;
            this.updateInputState(true);
        }
    }

    updateInputState(enabled) {
        this.messageInput.disabled = !enabled;
        this.sendButton.disabled = !enabled;
        this.sendButton.textContent = enabled ? 'Send' : 'Processing...';
        // Show stop button when processing, hide when not
        this.stopButton.style.display = enabled ? 'none' : 'inline-block';
    }

    async handleStop() {
        if (!this.sessionId) return;

        try {
            const response = await fetch('/api/chat/interrupt', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    session_id: this.sessionId
                })
            });

            const data = await response.json();
            console.log('Interrupt response:', data);

            if (!response.ok) {
                console.error('Failed to interrupt:', data.error);
            }
        } catch (error) {
            console.error('Failed to send interrupt request:', error);
        }
    }

    handleSSEEvent(event) {
        console.log('SSE Event:', event);

        switch (event.type) {
            case 'processing':
                this.updateProcessingMessage(event.data);
                break;
            case 'llm_response':
                // Don't remove processing indicator - wait for 'complete' event
                // Check if this is a thinking token
                const isThought = event.data.thought === true;
                let text = event.data.text;

                // Strip "thought\n" prefix from thinking tokens
                if (isThought && text.toLowerCase().startsWith('thought\n')) {
                    text = text.substring(8); // Remove "thought\n"
                }

                this.addMessage('assistant', text, isThought);
                break;
            case 'render_start':
                this.handleRenderStart(event.data);
                break;
            case 'scene_update':
                this.handleSceneUpdate(event.data);
                break;
            case 'function_call_start':
                this.handleFunctionCallStart(event.data);
                break;
            case 'function_calls':
                this.handleFunctionCalls(event.data);
                break;
            case 'error':
                this.removeProcessingMessage();
                this.isProcessing = false;
                this.updateInputState(true);

                // Handle session not found - stop reconnection loop
                if (event.data === 'Session not found') {
                    this.sessionId = null;
                    this.isConnected = false;
                    if (this.eventSource) {
                        this.eventSource.close();
                        this.eventSource = null;
                    }
                    this.updateConnectionStatus('disconnected', 'Server restarted - please refresh');
                    this.addMessage('system', 'Server was restarted. Please refresh the page to continue.');
                } else {
                    this.addMessage('system', `Error: ${event.data}`);
                }
                break;
            case 'complete':
                this.removeProcessingMessage();
                this.isProcessing = false;
                this.updateInputState(true);
                break;
            case 'ping':
                // Keep-alive, ignore
                break;
            default:
                console.log('Unknown SSE event type:', event.type);
        }
    }

    addMessage(role, content, isThought = false) {
        const messageDiv = document.createElement('div');
        messageDiv.className = `message ${role}`;

        const contentDiv = document.createElement('div');
        contentDiv.className = 'message-content';

        // Apply thinking styling if this is a thought
        if (isThought) {
            contentDiv.className = 'message-content thinking';
        }

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

        // Insert before processing message if it exists, otherwise append normally
        const processingMessage = document.getElementById('processing-message');
        if (processingMessage) {
            this.messagesContainer.insertBefore(messageDiv, processingMessage);
        } else {
            this.messagesContainer.appendChild(messageDiv);
        }
        this.scrollToBottom();
    }

    addProcessingMessage() {
        this.removeProcessingMessage(); // Remove any existing processing message

        const messageDiv = document.createElement('div');
        messageDiv.className = 'message assistant processing-persistent';
        messageDiv.id = 'processing-message';

        const contentDiv = document.createElement('div');
        contentDiv.className = 'message-content';

        const spinner = document.createElement('span');
        spinner.className = 'processing-spinner';

        const text = document.createElement('span');
        text.className = 'processing-text';
        text.textContent = 'Processing...';

        contentDiv.appendChild(spinner);
        contentDiv.appendChild(text);
        messageDiv.appendChild(contentDiv);
        this.messagesContainer.appendChild(messageDiv);
        this.scrollToBottom();
    }

    updateProcessingMessage(text) {
        const processingMessage = document.getElementById('processing-message');
        if (processingMessage) {
            const textSpan = processingMessage.querySelector('.processing-text');
            if (textSpan) {
                textSpan.textContent = text;
            }
        }
    }

    removeProcessingMessage() {
        const processingMessage = document.getElementById('processing-message');
        if (processingMessage) {
            processingMessage.remove();
        }
    }

    formatMessageContent(content) {
        // Basic text formatting
        // First trim trailing newlines, then convert remaining newlines to <br>
        const formatted = content
            .replace(/\n+$/g, '') // Remove trailing newlines
            .replace(/\n/g, '<br>')
            .replace(/`([^`]+)`/g, '<code>$1</code>');

        return formatted;
    }


    showRenderingIndicator() {
        const existingImage = this.scenePreview.querySelector('.scene-image');
        if (existingImage) {
            existingImage.classList.add('rendering');
        }
        this.scenePreview.classList.add('rendering');
    }

    hideRenderingIndicator() {
        const existingImage = this.scenePreview.querySelector('.scene-image');
        if (existingImage) {
            existingImage.classList.remove('rendering');
        }
        this.scenePreview.classList.remove('rendering');
    }

    handleRenderStart(data) {
        console.log('Render started:', data);
        this.showRenderingIndicator();
    }

    handleSceneUpdate(data) {
        console.log('Scene update received:', { quality: data.quality, shape_count: data.shape_count });
        if (data.image_base64) {
            this.displaySceneImage(data.image_base64);
        }
    }

    async triggerSceneRerender() {
        // Only re-render if we have an active session
        if (!this.sessionId || !this.isConnected) {
            return;
        }

        this.showRenderingIndicator();

        try {
            // Request a scene re-render with the current quality setting
            const response = await fetch('/api/render', {
                method: 'POST',
                headers: {
                    'Content-Type': 'application/json',
                },
                body: JSON.stringify({
                    session_id: this.sessionId,
                    quality: this.renderQuality
                })
            });

            if (!response.ok) {
                console.error('Failed to trigger re-render:', response.status);
                this.hideRenderingIndicator();
            }
        } catch (error) {
            console.error('Failed to trigger scene re-render:', error);
            this.hideRenderingIndicator();
        }
    }

    handleFunctionCallStart(toolCallStartEvent) {
        // Create a placeholder tool call element with a loading indicator
        const container = document.createElement('div');
        container.className = 'tool-call-container in-progress';
        container.dataset.toolCallId = toolCallStartEvent.id;

        const summaryDiv = document.createElement('div');
        summaryDiv.className = 'tool-call-summary in-progress';

        const toolName = this.getToolDisplayName(toolCallStartEvent.request.tool_name);
        summaryDiv.innerHTML = `🔧 ${toolName} <span class="loading-spinner">⏳</span>`;

        container.appendChild(summaryDiv);

        // Get or create a tool calls group message
        this.addToToolCallsGroup(container);
    }

    getOrCreateToolCallsGroup() {
        // Check if the last message is a tool calls group
        let lastMessage = null;
        const processingMessage = document.getElementById('processing-message');

        if (processingMessage && processingMessage.previousElementSibling) {
            lastMessage = processingMessage.previousElementSibling;
        } else {
            // No processing message, get actual last child
            lastMessage = this.messagesContainer.lastElementChild;
        }

        // If last message is a tool calls group, reuse it
        if (lastMessage && lastMessage.classList.contains('tool-calls-group')) {
            return lastMessage.querySelector('.message-content');
        }

        // Otherwise create a new tool calls group message
        const messageDiv = document.createElement('div');
        messageDiv.className = 'message assistant tool-calls-group';

        const contentDiv = document.createElement('div');
        contentDiv.className = 'message-content';

        messageDiv.appendChild(contentDiv);

        // Insert before processing message if it exists
        if (processingMessage) {
            this.messagesContainer.insertBefore(messageDiv, processingMessage);
        } else {
            this.messagesContainer.appendChild(messageDiv);
        }

        return contentDiv;
    }

    addToToolCallsGroup(toolCallElement) {
        const groupContent = this.getOrCreateToolCallsGroup();
        groupContent.appendChild(toolCallElement);
        this.scrollToBottom();
    }

    handleFunctionCalls(toolCallEvent) {
        // Handle the ToolCallEvent format (completion event)
        // Find and replace the matching start event by ID
        const existingContainer = this.messagesContainer.querySelector(
            `[data-tool-call-id="${toolCallEvent.id}"]`
        );

        // Create the completed tool call element
        const toolCallDiv = this.createToolCallElement(toolCallEvent);

        if (existingContainer) {
            // Replace the in-progress container with the completed one
            existingContainer.replaceWith(toolCallDiv);
        } else {
            // If no matching start event (shouldn't happen, but be defensive)
            this.addToToolCallsGroup(toolCallDiv);
        }
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
        const op = toolCallEvent.request;
        const success = toolCallEvent.success;

        if (!success) {
            return `${this.getToolDisplayName(op.tool_name)} failed: ${toolCallEvent.error}`;
        }

        // Completely generic approach
        const target = this.getToolRequestTarget(op);
        const displayName = this.getToolDisplayName(op.tool_name);

        if (target) {
            return `${displayName}: ${target}`;
        } else {
            return displayName;
        }
    }

    getToolDisplayName(toolName) {
        // Auto-generate a nice display name from any tool name
        return toolName
            .split('_')
            .map(word => word.charAt(0).toUpperCase() + word.slice(1))
            .join(' ');
    }

    getToolCallDetails(toolCallEvent) {
        const op = toolCallEvent.request;

        let details = `
            <div class="tool-call-meta">
                <strong>Function:</strong> ${op.tool_name}<br>
                <strong>Target:</strong> ${this.getToolRequestTarget(op) || 'N/A'}<br>
                <strong>Status:</strong> ${toolCallEvent.success ? '✓ Success' : '❌ Failed'}<br>
                <strong>Duration:</strong> ${toolCallEvent.duration}ms<br>
            </div>
        `;

        if (!toolCallEvent.success) {
            details += `<div class="tool-call-error"><strong>Error:</strong> ${toolCallEvent.error}</div>`;
        }

        // Generic details that work for any tool operation
        details += this.getGenericToolDetails(op);

        return details;
    }

    getToolRequestTarget(op) {
        // Try common patterns to extract a target identifier

        // Direct ID field (update/remove operations)
        if (op.id) {
            return op.id;
        }

        // Shape creation
        if (op.shape && op.shape.id) {
            return op.shape.id;
        }

        // Light creation
        if (op.light && op.light.id) {
            return op.light.id;
        }

        // Lighting type for environment lighting
        if (op.lighting_type) {
            return op.lighting_type;
        }

        // Fallback: look for any property ending with "_id" or just "id"
        for (const [key, value] of Object.entries(op)) {
            if ((key.endsWith('_id') || key === 'id') && typeof value === 'string') {
                return value;
            }
        }

        return '';
    }


    getGenericToolDetails(op) {
        // Generic details that work for any tool operation
        let details = '<div class="tool-call-generic-details">';

        // Show all operation properties except tool_name (already shown above)
        const properties = { ...op };
        delete properties.tool_name;

        // Extract rendered_image if present
        const renderedImage = properties.rendered_image;
        delete properties.rendered_image;

        if (Object.keys(properties).length > 0) {
            details += `<strong>Tool Request Data:</strong> <pre>${this.formatCompactJSON(properties)}</pre>`;
        }

        // If there's a rendered image, display it as a thumbnail
        if (renderedImage) {
            details += `
                <div class="rendered-image-preview">
                    <strong>Rendered Image:</strong><br>
                    <img src="data:image/png;base64,${renderedImage}" alt="Rendered scene" style="max-width: 200px; border: 1px solid var(--border-color); border-radius: 4px; margin-top: 8px;">
                </div>
            `;
        }

        details += '</div>';
        return details;
    }

    formatCompactJSON(obj) {
        // Custom JSON formatter that keeps simple arrays and small objects on one line
        return JSON.stringify(obj, null, 2).replace(
            /\[\s*(-?\d+(?:\.\d+)?),\s*(-?\d+(?:\.\d+)?),\s*(-?\d+(?:\.\d+)?)\s*\]/g,
            '[$1, $2, $3]'
        ).replace(
            /\[\s*(-?\d+(?:\.\d+)?),\s*(-?\d+(?:\.\d+)?)\s*\]/g,
            '[$1, $2]'
        ).replace(
            /\[\s*(-?\d+(?:\.\d+)?),\s*(-?\d+(?:\.\d+)?),\s*(-?\d+(?:\.\d+)?),\s*(-?\d+(?:\.\d+)?)\s*\]/g,
            '[$1, $2, $3, $4]'
        ).replace(
            /\[\s*"([^"]+)",\s*"([^"]+)",\s*"([^"]+)"\s*\]/g,
            '["$1", "$2", "$3"]'
        );
    }

    displaySceneImage(imageBase64) {
        // Remove "No scene yet" placeholder
        const placeholder = this.scenePreview.querySelector('.no-scene-placeholder');
        if (placeholder) placeholder.remove();

        // Remove loading indicator
        const loading = this.scenePreview.querySelector('.scene-loading');
        if (loading) loading.remove();

        // Hide rendering indicator
        this.hideRenderingIndicator();

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