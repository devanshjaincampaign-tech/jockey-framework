from flask import Blueprint, request, jsonify, send_file
from datetime import datetime
from models import db, Result, Deploy, Agent
import uuid
import io
import json
import os
import base64
import hashlib
from cryptography.hazmat.primitives.ciphers.aead import AESGCM
from cryptography.hazmat.primitives import hashes
from cryptography.hazmat.primitives.kdf.pbkdf2 import PBKDF2HMAC
from cryptography.hazmat.backends import default_backend

result_bp = Blueprint("result", __name__, url_prefix='/api/v1/result')

# PBKDF2 key derivation (matches agent implementation)
def derive_agent_key(secret: str) -> bytes:
    salt = b"jocky-agent-kdf-v1"
    kdf = PBKDF2HMAC(
        algorithm=hashes.SHA256(),
        length=32,
        salt=salt,
        iterations=100000,
        backend=default_backend()
    )
    return kdf.derive(secret.encode('utf-8'))


def encrypt_payload(secret: str, plaintext: str) -> str:
    if not plaintext:
        return ""
    key = derive_agent_key(secret)
    aesgcm = AESGCM(key)
    nonce = os.urandom(12)
    ciphertext = aesgcm.encrypt(nonce, plaintext.encode('utf-8'), None)
    return base64.b64encode(nonce + ciphertext).decode('utf-8')


def decrypt_payload(secret: str, payload: str) -> str:
    if not payload:
        return ""
    key = derive_agent_key(secret)
    raw = base64.b64decode(payload)
    nonce = raw[:12]
    ciphertext = raw[12:]
    aesgcm = AESGCM(key)
    return aesgcm.decrypt(nonce, ciphertext, None).decode('utf-8', errors='replace')


@result_bp.route("/submit", methods=["POST"])
def submit():
    data = request.get_json()
    if not isinstance(data, dict):
        return jsonify({'error': 'Invalid JSON'}), 400

    agent_id = data.get("agent_id")
    script_id = data.get("script_id")
    encrypted_data = data.get("data_enc")
    agent_secret = request.headers.get("X-Agent-Secret") or request.headers.get("X-Agent-Token")

    if not all([agent_id, script_id, encrypted_data]):
        return jsonify({"error": "Missing fields"}), 400

    agent = Agent.query.get(agent_id)
    if not agent:
        return jsonify({"error": "Agent not found"}), 404
    if agent_secret != agent.token:
        return jsonify({"error": "Unauthorized"}), 401

    try:
        decrypted_data = decrypt_payload(agent.token, encrypted_data)
    except Exception:
        return jsonify({"error": "Invalid encrypted payload"}), 400

    result = Result(
        result_id=str(uuid.uuid4()),
        agent_id=agent_id,
        script_id=script_id,
        data_encrypted=encrypted_data,
        data_decrypted=decrypted_data,
        submitted_at=datetime.utcnow()
    )
    db.session.add(result)
    db.session.commit()

    deploy = Deploy.query.filter_by(
        agent_id=agent_id,
        script_id=script_id
    ).order_by(Deploy.executed_at.desc()).first()
    if deploy:
        deploy.status = "completed"
        deploy.result_id = result.result_id
        deploy.executed_at = datetime.utcnow()
        db.session.commit()

    return jsonify({"result_id": result.result_id}), 200


@result_bp.route("/view", methods=["GET"])
def view():
    result_id = request.args.get("result_id")
    if not result_id:
        return jsonify({"error": "result_id required"}), 400

    result = Result.query.get(result_id)
    if not result:
        return jsonify({"error": "Not found"}), 404

    return jsonify({
        "result_id": result.result_id,
        "agent_id": result.agent_id,
        "script_id": result.script_id,
        "data_encrypted": result.data_encrypted,
        "data_decrypted": result.data_decrypted or decrypt_payload(Agent.query.get(result.agent_id).token, result.data_encrypted),
        "submitted_at": result.submitted_at.isoformat()
    }), 200


@result_bp.route("/export", methods=["GET"])
def export():
    result_id = request.args.get("result_id")
    fmt = request.args.get("fmt", "json")

    if not result_id:
        return jsonify({"error": "result_id required"}), 400

    result = Result.query.get(result_id)
    if not result:
        return jsonify({"error": "Not found"}), 404

    agent = Agent.query.get(result.agent_id)
    data = {
        "result_id": result.result_id,
        "agent_id": result.agent_id,
        "script_id": result.script_id,
        "encrypted_data": result.data_encrypted,
        "decrypted_data": result.data_decrypted or (decrypt_payload(agent.token, result.data_encrypted) if agent else ""),
        "submitted_at": result.submitted_at.isoformat()
    }

    if fmt == "json":
        return jsonify(data), 200
    else:
        json_str = json.dumps(data, indent=2)
        return send_file(
            io.BytesIO(json_str.encode()),
            mimetype="application/json",
            as_attachment=True,
            download_name=f"result_{result_id}.json"
        )


# ==================== NEW ENDPOINT ====================
@result_bp.route("/list", methods=["GET"])
def list_results():
    """
    List all results (optionally filter by agent_id).
    Query param: ?agent_id=...
    """
    agent_id = request.args.get("agent_id")
    query = Result.query
    if agent_id:
        query = query.filter_by(agent_id=agent_id)
    results = query.order_by(Result.submitted_at.desc()).all()

    serialized = []
    for r in results:
        agent = Agent.query.get(r.agent_id)
        decrypted = r.data_decrypted
        if not decrypted and agent:
            try:
                decrypted = decrypt_payload(agent.token, r.data_encrypted)
            except Exception:
                decrypted = ""
        serialized.append({
            "result_id": r.result_id,
            "agent_id": r.agent_id,
            "script_id": r.script_id,
            "submitted_at": r.submitted_at.isoformat(),
            "data_encrypted": r.data_encrypted[:50] + "..." if r.data_encrypted and len(r.data_encrypted) > 50 else r.data_encrypted,
            "data_decrypted": decrypted or ""
        })
    return jsonify(serialized), 200