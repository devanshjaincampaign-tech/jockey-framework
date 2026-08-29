from flask import Blueprint, request, jsonify
from datetime import datetime
from models import db, Agent
import uuid
import json

logs_bp = Blueprint('logs', __name__, url_prefix='/api/v1/logs')

# In-memory log storage (in production, use database)
agent_logs = {}


@logs_bp.route('/submit', methods=['POST'])
def submit_logs():
    """
    Receive and store agent logs.
    Expected JSON: {
        "agent_id": "agent-xxx",
        "logs": [
            {
                "timestamp": "2026-08-29T15:30:45Z",
                "level": "info",
                "message": "Agent started"
            }
        ]
    }
    """
    data = request.get_json()
    if not data:
        return jsonify({'error': 'Invalid JSON'}), 400

    agent_id = data.get('agent_id')
    logs = data.get('logs', [])
    agent_secret = request.headers.get('X-Agent-Secret') or request.headers.get('X-Agent-Token')

    if not agent_id:
        return jsonify({'error': 'agent_id required'}), 400

    # Verify agent and secret
    agent = Agent.query.get(agent_id)
    if not agent:
        return jsonify({'error': 'Agent not found'}), 404
    if agent_secret != agent.token:
        return jsonify({'error': 'Unauthorized'}), 401

    # Store logs
    if agent_id not in agent_logs:
        agent_logs[agent_id] = []

    for log_entry in logs:
        if isinstance(log_entry, dict):
            agent_logs[agent_id].append({
                'timestamp': log_entry.get('timestamp', datetime.utcnow().isoformat()),
                'level': log_entry.get('level', 'info'),
                'message': log_entry.get('message', ''),
                'received_at': datetime.utcnow().isoformat()
            })

    # Keep only last 1000 logs per agent (rotating buffer)
    if len(agent_logs[agent_id]) > 1000:
        agent_logs[agent_id] = agent_logs[agent_id][-1000:]

    return jsonify({'status': 'success', 'count': len(logs)}), 200


@logs_bp.route('/view/<agent_id>', methods=['GET'])
def view_logs(agent_id):
    """
    Retrieve logs for an agent.
    Query params: ?limit=100&level=info
    """
    limit = request.args.get('limit', 100, type=int)
    level = request.args.get('level', None)

    agent = Agent.query.get(agent_id)
    if not agent:
        return jsonify({'error': 'Agent not found'}), 404

    logs = agent_logs.get(agent_id, [])

    # Filter by level if specified
    if level:
        logs = [l for l in logs if l.get('level') == level]

    # Return last N logs
    return jsonify({
        'agent_id': agent_id,
        'total': len(logs),
        'logs': logs[-limit:] if limit else logs
    }), 200


@logs_bp.route('/stream/<agent_id>', methods=['GET'])
def stream_logs(agent_id):
    """
    Get logs for real-time dashboard updates.
    Returns latest logs since last query.
    """
    agent = Agent.query.get(agent_id)
    if not agent:
        return jsonify({'error': 'Agent not found'}), 404

    logs = agent_logs.get(agent_id, [])
    
    return jsonify({
        'agent_id': agent_id,
        'timestamp': datetime.utcnow().isoformat(),
        'logs': logs[-20:],  # Last 20 logs for real-time stream
        'count': len(logs)
    }), 200


@logs_bp.route('/clear/<agent_id>', methods=['DELETE'])
def clear_logs(agent_id):
    """Clear logs for an agent."""
    agent = Agent.query.get(agent_id)
    if not agent:
        return jsonify({'error': 'Agent not found'}), 404

    if agent_id in agent_logs:
        del agent_logs[agent_id]

    return jsonify({'status': 'cleared'}), 200
