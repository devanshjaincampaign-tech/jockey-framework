from flask import Blueprint, request, jsonify
from datetime import datetime
from models import db, Agent, Deploy, Script

def require_agent_secret(agent: Agent):
    provided = request.headers.get("X-Agent-Secret") or request.headers.get("X-Agent-Token")
    return provided == agent.token

agent_bp = Blueprint('agent', __name__, url_prefix='/api/v1/agent')


@agent_bp.route('/register', methods=['POST'])
def register_agent():
    """
    Register a new agent or update an existing one.
    Expected JSON: { "agent_id": "...", "hostname": "...", "os": "...", "ip": "...", "arch": "..." }
    """
    data = request.get_json()
    if not data:
        return jsonify({'error': 'Invalid JSON'}), 400

    agent_id = data.get('agent_id')
    hostname = data.get('hostname', 'unknown')
    os_type = data.get('os', 'unknown')
    ip = data.get('ip', '0.0.0.0')
    arch = data.get('arch', 'unknown')

    if not agent_id:
        return jsonify({'error': 'agent_id required'}), 400

    agent = Agent.query.get(agent_id)
    if not agent:
        agent = Agent(
            agent_id=agent_id,
            hostname=hostname,
            os=os_type,
            ip=ip,
            arch=arch
        )
        db.session.add(agent)
    else:
        agent.hostname = hostname or agent.hostname
        agent.os = os_type or agent.os
        agent.ip = ip or agent.ip
        agent.arch = arch or agent.arch
        if not agent.token:
            agent.token = Agent._generate_token()

    db.session.commit()
    return jsonify({'status': 'registered', 'agent_id': agent_id, 'agent_secret': agent.token}), 200


@agent_bp.route('/heartbeat', methods=['POST'])
def heartbeat():
    """
    Agent heartbeat – updates last_seen and returns pending deployments.
    Expected JSON: { "agent_id": "..." }
    """
    data = request.get_json()
    if not data:
        return jsonify({'error': 'Invalid JSON'}), 400

    agent_id = data.get('agent_id')
    if not agent_id:
        return jsonify({'error': 'agent_id required'}), 400

    agent = Agent.query.get(agent_id)
    if not agent:
        return jsonify({'error': 'Agent not found'}), 404

    if not require_agent_secret(agent):
        return jsonify({'error': 'Unauthorized'}), 401

    agent.last_seen = datetime.utcnow()
    agent.status = 'online'
    db.session.commit()

    pending = Deploy.query.filter_by(
        agent_id=agent_id,
        status='pending'
    ).first()

    response = {'status': 'ok'}

    if pending:
        script = Script.query.get(pending.script_id)
        if script:
            response['deployment'] = {
                'deploy_id': pending.deploy_id,
                'script_id': script.script_id,
                'code': script.code,
                'hash_before': script.hash_before
            }
            pending.status = 'in_progress'
            db.session.commit()

    return jsonify(response), 200


@agent_bp.route('/status/<agent_id>', methods=['GET'])
def get_agent_status(agent_id):
    """Get status of a specific agent."""
    agent = Agent.query.get(agent_id)
    if not agent:
        return jsonify({'error': 'Agent not found'}), 404

    return jsonify({
        'agent_id': agent.agent_id,
        'hostname': agent.hostname,
        'os': agent.os,
        'ip': agent.ip,
        'arch': agent.arch,
        'status': agent.status,
        'last_seen': agent.last_seen.isoformat() if agent.last_seen else None
    }), 200


# ==================== NEW ENDPOINT ====================
@agent_bp.route('/list', methods=['GET'])
def list_agents():
    """List all registered agents."""
    agents = Agent.query.all()
    return jsonify([{
        'agent_id': a.agent_id,
        'hostname': a.hostname,
        'os': a.os,
        'ip': a.ip,
        'arch': a.arch,
        'status': a.status,
        'last_seen': a.last_seen.isoformat() if a.last_seen else None
    } for a in agents]), 200