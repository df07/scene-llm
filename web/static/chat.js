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
        this.sceneInfo = document.getElementById('sceneInfo');
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
            case 'scene_state':
                this.handleSceneState(event.data);
                break;
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

    handleSceneState(data) {
        if (data.scene && data.scene.shapes) {
            const shapeCount = data.scene.shapes.length;
            this.sceneInfo.textContent = shapeCount === 0 ? 'Empty scene' : `${shapeCount} shape${shapeCount !== 1 ? 's' : ''}`;

            if (shapeCount > 0) {
                this.updateScenePreview(data.scene);
            }
        }
    }

    handleSceneUpdate(data) {
        if (data.scene) {
            this.handleSceneState({scene: data.scene});
        }

        if (data.image_base64) {
            this.displaySceneImage(data.image_base64);
        }
    }

    handleFunctionCalls(functionCalls) {
        // Show what functions the LLM called
        const callsText = functionCalls.map(call =>
            `Created ${call.type} at [${call.position.join(', ')}] size ${call.size} color RGB[${call.color.join(', ')}]`
        ).join('\n');

        this.addMessage('system', `Function calls:\n${callsText}`);
    }

    updateScenePreview(scene) {
        const placeholder = this.scenePreview.querySelector('.no-scene-placeholder');
        if (placeholder) {
            placeholder.remove();
        }

        // Add or update scene details
        let detailsDiv = this.scenePreview.querySelector('.scene-details');
        if (!detailsDiv) {
            detailsDiv = document.createElement('div');
            detailsDiv.className = 'scene-details';
            this.scenePreview.appendChild(detailsDiv);
        }

        const shapesList = scene.shapes.map(shape =>
            `${shape.type} at [${shape.position.map(p => p.toFixed(1)).join(', ')}] size ${shape.size} color RGB[${shape.color.map(c => c.toFixed(1)).join(', ')}]`
        ).join('');

        detailsDiv.innerHTML = `
            <h4>Scene Objects:</h4>
            <ul>
                ${scene.shapes.map(shape => `<li>${shape.type} at [${shape.position.map(p => p.toFixed(1)).join(', ')}] size ${shape.size} color RGB[${shape.color.map(c => c.toFixed(1)).join(', ')}]</li>`).join('')}
            </ul>
        `;
    }

    displaySceneImage(imageBase64) {
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

        // Insert before scene details if they exist
        const details = this.scenePreview.querySelector('.scene-details');
        if (details) {
            this.scenePreview.insertBefore(img, details);
        } else {
            this.scenePreview.appendChild(img);
        }
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

            this.sceneInfo.textContent = 'Empty scene';

            // Start new session
            this.startNewSession();
        }
    }
}

// Initialize chat when page loads
document.addEventListener('DOMContentLoaded', () => {
    new SceneLLMChat();
});