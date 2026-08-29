// JOCKY Dashboard - Real-time Updates
console.log("JOCKY Dashboard loaded.");

// Global state
let selectedAgent = null;
let pollInterval = null;
let agentLogs = {};

// ==================== AGENT MANAGEMENT ====================

async function loadAgents() {
    try {
        const resp = await fetch('/api/v1/agent/list');
        const agents = await resp.json();
        
        const container = document.getElementById('agents-list');
        if (!container) return;
        
        container.innerHTML = '';
        
        agents.forEach(agent => {
            const statusClass = agent.status === 'online' ? 'status-online' : 'status-offline';
            const lastSeen = agent.last_seen ? new Date(agent.last_seen).toLocaleString() : 'Never';
            
            const agentEl = document.createElement('div');
            agentEl.className = 'agent-card';
            agentEl.innerHTML = `
                <div class="agent-header">
                    <h3>${agent.hostname}</h3>
                    <span class="status ${statusClass}">${agent.status}</span>
                </div>
                <div class="agent-info">
                    <p><strong>ID:</strong> ${agent.agent_id}</p>
                    <p><strong>OS:</strong> ${agent.os} (${agent.arch})</p>
                    <p><strong>IP:</strong> ${agent.ip}</p>
                    <p><strong>Last Seen:</strong> ${lastSeen}</p>
                </div>
                <div class="agent-actions">
                    <button onclick="selectAgent('${agent.agent_id}')" class="btn btn-primary">View Details</button>
                    <button onclick="sendCommand('${agent.agent_id}')" class="btn btn-success">Send Command</button>
                </div>
            `;
            container.appendChild(agentEl);
        });
    } catch (error) {
        console.error('Error loading agents:', error);
    }
}

async function selectAgent(agentId) {
    selectedAgent = agentId;
    
    try {
        const resp = await fetch(`/api/v1/agent/status/${agentId}`);
        const agent = await resp.json();
        
        const detailsEl = document.getElementById('agent-details');
        if (detailsEl) {
            detailsEl.innerHTML = `
                <h2>Agent: ${agent.hostname}</h2>
                <div class="details-grid">
                    <div><strong>Agent ID:</strong> ${agent.agent_id}</div>
                    <div><strong>Status:</strong> <span class="status status-${agent.status}">${agent.status}</span></div>
                    <div><strong>OS:</strong> ${agent.os} ${agent.arch}</div>
                    <div><strong>IP:</strong> ${agent.ip}</div>
                    <div><strong>Last Seen:</strong> ${agent.last_seen || 'Never'}</div>
                </div>
            `;
        }
        
        // Load results and logs for this agent
        loadAgentResults(agentId);
        loadAgentLogs(agentId);
        
        // Start polling for updates
        startPolling(agentId);
    } catch (error) {
        console.error('Error selecting agent:', error);
    }
}

// ==================== RESULTS MANAGEMENT ====================

async function loadAgentResults(agentId) {
    try {
        const resp = await fetch(`/api/v1/result/list?agent_id=${agentId}`);
        const results = await resp.json();
        
        const container = document.getElementById('results-container');
        if (!container) return;
        
        if (results.length === 0) {
            container.innerHTML = '<p class="no-data">No results yet</p>';
            return;
        }
        
        container.innerHTML = '';
        results.forEach(result => {
            const resultEl = document.createElement('div');
            resultEl.className = 'result-card';
            
            // Truncate long output
            const output = result.data_decrypted || result.data_encrypted;
            const truncated = output.length > 200 ? output.substring(0, 200) + '...' : output;
            
            resultEl.innerHTML = `
                <div class="result-header">
                    <h4>Script: ${result.script_id}</h4>
                    <span class="time">${new Date(result.submitted_at).toLocaleString()}</span>
                </div>
                <div class="result-output">
                    <pre>${escapeHtml(truncated)}</pre>
                </div>
                <button onclick="viewFullResult('${result.result_id}')" class="btn btn-small">View Full</button>
            `;
            container.appendChild(resultEl);
        });
    } catch (error) {
        console.error('Error loading results:', error);
    }
}

// ==================== LOGS MANAGEMENT ====================

async function loadAgentLogs(agentId) {
    try {
        const resp = await fetch(`/api/v1/logs/stream/${agentId}`);
        const data = await resp.json();
        
        const container = document.getElementById('logs-container');
        if (!container) return;
        
        agentLogs[agentId] = data.logs;
        
        if (data.logs.length === 0) {
            container.innerHTML = '<p class="no-data">No logs yet</p>';
            return;
        }
        
        container.innerHTML = '';
        data.logs.forEach(log => {
            const logEl = document.createElement('div');
            logEl.className = `log-entry log-${log.level}`;
            logEl.innerHTML = `
                <span class="log-time">${log.timestamp}</span>
                <span class="log-level">[${log.level.toUpperCase()}]</span>
                <span class="log-message">${escapeHtml(log.message)}</span>
            `;
            container.appendChild(logEl);
        });
    } catch (error) {
        console.error('Error loading logs:', error);
    }
}

// ==================== COMMAND EXECUTION ====================

async function sendCommand(agentId) {
    const command = prompt('Enter command to execute:');
    if (!command) return;
    
    try {
        // First create a script
        const scriptResp = await fetch('/api/v1/script/', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                name: `cmd-${Date.now()}`,
                code: command
            })
        });
        
        const script = await scriptResp.json();
        const scriptId = script.script_id;
        
        // Then deploy it to the agent
        const deployResp = await fetch('/api/v1/script/deploy', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                script_id: scriptId,
                agent_ids: [agentId]
            })
        });
        
        if (deployResp.ok) {
            alert('Command queued for execution!');
            // Reload results after a short delay
            setTimeout(() => loadAgentResults(agentId), 2000);
        }
    } catch (error) {
        console.error('Error sending command:', error);
        alert('Failed to send command');
    }
}

async function viewFullResult(resultId) {
    try {
        const resp = await fetch(`/api/v1/result/view?result_id=${resultId}`);
        const result = await resp.json();
        
        const output = result.data_decrypted || result.data_encrypted || 'No output';
        alert(`Result:\n\n${output}`);
    } catch (error) {
        console.error('Error viewing result:', error);
        alert('Failed to load result');
    }
}

// ==================== REAL-TIME POLLING ====================

function startPolling(agentId) {
    // Clear existing poll
    if (pollInterval) clearInterval(pollInterval);
    
    // Poll every 5 seconds
    pollInterval = setInterval(() => {
        loadAgentLogs(agentId);
        loadAgentResults(agentId);
    }, 5000);
}

function stopPolling() {
    if (pollInterval) {
        clearInterval(pollInterval);
        pollInterval = null;
    }
}

// ==================== UTILITY FUNCTIONS ====================

function escapeHtml(text) {
    const map = {
        '&': '&amp;',
        '<': '&lt;',
        '>': '&gt;',
        '"': '&quot;',
        "'": '&#039;'
    };
    return text.replace(/[&<>"']/g, m => map[m]);
}

// ==================== INITIALIZATION ====================

document.addEventListener('DOMContentLoaded', () => {
    loadAgents();
    
    // Auto-refresh agents list every 10 seconds
    setInterval(loadAgents, 10000);
});

// Cleanup on page unload
window.addEventListener('beforeunload', stopPolling);