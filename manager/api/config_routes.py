"""Configuration bootstrap endpoint - separate from agent_routes to avoid URL prefix collision"""
from flask import Blueprint, request, jsonify
import secrets
from models import db, Agent
from datetime import datetime

config_bp = Blueprint('config', __name__, url_prefix='/api/v1/config')


@config_bp.route('/bootstrap', methods=['POST'])
def bootstrap():
    """
    Bootstrap endpoint - provides configuration to new/existing agents.
    Expected JSON: {
        "bootstrap_id": "hex-encoded-random-16-bytes",
        "hostname": "DESKTOP-ABC123",
        "username": "admin",
        "os": "windows",
        "arch": "amd64",
        "version": "1.0.0"
    }
    """
    data = request.get_json()
    if not data:
        return jsonify({'error': 'Invalid JSON', 'valid': False}), 400

    bootstrap_id = data.get('bootstrap_id')
    hostname = data.get('hostname', 'unknown')
    username = data.get('username', 'unknown')
    os_type = data.get('os', 'unknown')
    arch = data.get('arch', 'unknown')
    agent_version = data.get('version', '1.0.0')

    if not bootstrap_id:
        return jsonify({'error': 'bootstrap_id required', 'valid': False}), 400

    # Check if agent already registered with this bootstrap
    agent = Agent.query.filter_by(hostname=hostname, os=os_type).first()
    
    if not agent:
        # Generate new agent ID and secret
        agent_id = f"agent-{secrets.token_hex(8)}"
        agent_secret = secrets.token_hex(32)  # 64-char hex = 32 bytes
        
        agent = Agent(
            agent_id=agent_id,
            hostname=hostname,
            os=os_type,
            ip='0.0.0.0',  # Will be updated on heartbeat
            arch=arch,
            token=agent_secret,
            status='pending',
            last_seen=datetime.utcnow()
        )
        db.session.add(agent)
        db.session.commit()
    else:
        agent_id = agent.agent_id
        agent_secret = agent.token

    # Return bootstrap configuration
    return jsonify({
        'valid': True,
        'agent_id': agent_id,
        'agent_secret': agent_secret,
        'config': {
            'listener_url': 'http://localhost:5000',
            'front_domain': 'localhost',
            'c2_auth': 'jocky-c2-secret-key',
            'heartbeat_interval': 30000000000,  # 30 seconds in nanoseconds
            'tls_verify': False,  # False for local testing
            'log_level': 'debug',  # debug/info/warn/error
            'timeout': 15000000000  # 15 seconds in nanoseconds
        }
    }), 200
