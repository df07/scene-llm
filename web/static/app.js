const form = document.getElementById('sceneForm');
const resultDiv = document.getElementById('result');
const resultTitle = document.getElementById('resultTitle');
const resultContent = document.getElementById('resultContent');
const loadingDiv = document.getElementById('loading');
const generateBtn = document.getElementById('generateBtn');

form.addEventListener('submit', async (e) => {
    e.preventDefault();

    const prompt = document.getElementById('prompt').value;

    // Show loading state
    loadingDiv.classList.remove('hidden');
    resultDiv.classList.add('hidden');
    generateBtn.disabled = true;
    generateBtn.textContent = 'Generating...';

    try {
        const response = await fetch('/api/generate-scene', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json',
            },
            body: JSON.stringify({ prompt: prompt })
        });

        const data = await response.json();
        displayResult(data);

    } catch (error) {
        displayError('Network error: ' + error.message);
    } finally {
        // Hide loading state
        loadingDiv.classList.add('hidden');
        generateBtn.disabled = false;
        generateBtn.textContent = 'Generate Scene';
    }
});

function displayResult(data) {
    resultDiv.classList.remove('hidden');

    if (data.status === 'success') {
        resultDiv.className = 'result status-success';
        resultTitle.textContent = '✅ Scene Generated Successfully!';

        let html = `
            <p><strong>Prompt:</strong> "${data.prompt}"</p>
        `;

        if (data.image_base64) {
            html += `
                <div class="rendered-image">
                    <h4>Rendered Scene:</h4>
                    <img src="data:image/png;base64,${data.image_base64}" alt="Generated scene" />
                </div>
            `;
        }

        if (data.scene) {
            html += `
                <h4>Scene Details:</h4>
                <p><strong>Shape:</strong> ${data.scene.shapes[0].type}</p>
                <p><strong>Position:</strong> [${data.scene.shapes[0].position.join(', ')}]</p>
                <p><strong>Size:</strong> ${data.scene.shapes[0].size}</p>
                <p><strong>Color:</strong> RGB[${data.scene.shapes[0].color.join(', ')}]</p>
            `;
        }

        html += `
            <details>
                <summary>Debug Info (LLM Conversation)</summary>
                <div class="debug-info">
                    <strong>Model:</strong> ${data.llm_response.model}<br>
                    <strong>Function Called:</strong> ${data.llm_response.function_called}<br>
                    ${data.llm_response.function_name ? `<strong>Function:</strong> ${data.llm_response.function_name}<br>` : ''}
                    ${data.llm_response.function_args ? `<strong>Arguments:</strong> ${JSON.stringify(data.llm_response.function_args, null, 2)}<br>` : ''}
                    ${data.llm_response.text_response ? `<strong>Text Response:</strong> ${data.llm_response.text_response}<br>` : ''}
                </div>
            </details>
        `;

        resultContent.innerHTML = html;

    } else {
        resultDiv.className = 'result status-error';
        resultTitle.textContent = '❌ Generation Failed';

        let html = `
            <p><strong>Prompt:</strong> "${data.prompt}"</p>
            <p><strong>Error:</strong> ${data.error || 'Unknown error'}</p>
        `;

        if (data.llm_response && data.llm_response.text_response) {
            html += `
                <p><strong>LLM Response:</strong> ${data.llm_response.text_response}</p>
            `;
        }

        html += `
            <details>
                <summary>Debug Info</summary>
                <div class="debug-info">${JSON.stringify(data, null, 2)}</div>
            </details>
        `;

        resultContent.innerHTML = html;
    }
}

function displayError(message) {
    resultDiv.classList.remove('hidden');
    resultDiv.className = 'result status-error';
    resultTitle.textContent = '❌ Error';
    resultContent.innerHTML = `<p>${message}</p>`;
}