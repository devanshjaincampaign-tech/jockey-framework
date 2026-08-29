# Sample Bootstrap Configuration Response
# This is an example of the response the agent expects from the bootstrap endpoint

## Minimal Configuration
{
  "valid": true,
  "agent_id": "agent-a1b2c3d4e5f6g7h8",
  "agent_secret": "generated-secret-by-server-32-chars-or-base64",
  "config": {
    "listener_url": "https://c2.example.com:443",
    "front_domain": "c2.example.com",
    "c2_auth": "shared-secret-authentication-key",
    "heartbeat_interval": 30000000000,
    "tls_verify": true,
    "log_level": "info",
    "timeout": 15000000000
  }
}

## Production Configuration (Recommended)
{
  "valid": true,
  "agent_id": "agent-prod-20260829-001",
  "agent_secret": "base64-encoded-random-32-bytes",
  "config": {
    "listener_url": "https://c2-prod.internal.corp:8443",
    "front_domain": "c2.internal.corp",
    "c2_auth": "prod-shared-key-change-monthly",
    "heartbeat_interval": 60000000000,
    "tls_verify": true,
    "log_level": "info",
    "timeout": 20000000000
  }
}

## High-Performance Configuration
{
  "valid": true,
  "agent_id": "agent-perf-20260829-001",
  "agent_secret": "optimized-secret-for-throughput",
  "config": {
    "listener_url": "https://c2-load-balanced.example.com:443",
    "front_domain": "c2-lb.example.com",
    "c2_auth": "perf-key-optimized",
    "heartbeat_interval": 15000000000,
    "tls_verify": true,
    "log_level": "warn",
    "timeout": 10000000000
  }
}

## Debug Configuration (Development Only)
{
  "valid": true,
  "agent_id": "agent-dev-debug",
  "agent_secret": "debug-secret-not-for-production",
  "config": {
    "listener_url": "https://localhost:8443",
    "front_domain": "localhost",
    "c2_auth": "dev-secret",
    "heartbeat_interval": 5000000000,
    "tls_verify": false,
    "log_level": "debug",
    "timeout": 30000000000
  }
}

# Configuration Parameters Reference

## listener_url (string, required)
The base URL of the Jockey Manager server.
- Must be HTTPS in production
- Example: https://c2.example.com:443

## front_domain (string, required)
The domain name for TLS SNI (Server Name Indication).
- Must match the TLS certificate
- Can differ from listener_url hostname for proxied setups
- Example: c2.example.com

## c2_auth (string, required)
Shared authentication secret sent in X-C2-Auth header.
- Should be a strong, random string
- Recommend rotating monthly
- Example: sha256(timestamp + secret)

## heartbeat_interval (int64, required)
Interval between heartbeats in nanoseconds (Go time.Duration format).
- Minimum: 5 seconds (5000000000)
- Recommended: 30-60 seconds (30000000000 - 60000000000)
- Maximum: 1 hour (3600000000000)
- Converts: seconds * 1,000,000,000 = nanoseconds

## tls_verify (bool, required)
Enable/disable TLS certificate verification.
- true = Verify certificates (required for production)
- false = Skip verification (development only, security risk)

## log_level (string, required)
Logging verbosity level.
- "debug" = Maximum verbosity (development)
- "info" = Standard operational logging (recommended)
- "warn" = Warnings and errors only
- "error" = Errors only (minimal logging)

## timeout (int64, required)
HTTP request timeout in nanoseconds (Go time.Duration format).
- Minimum: 5 seconds (5000000000)
- Recommended: 15-20 seconds (15000000000 - 20000000000)
- Maximum: 5 minutes (300000000000)
- Applies to all HTTP operations (heartbeat, result submission, log flush)
- Converts: seconds * 1,000,000,000 = nanoseconds

# Nanosecond Conversion Reference
5 seconds   = 5,000,000,000
10 seconds  = 10,000,000,000
15 seconds  = 15,000,000,000
30 seconds  = 30,000,000,000
60 seconds  = 60,000,000,000
5 minutes   = 300,000,000,000

# Required Bootstrap Endpoints

All bootstrap requests must be handled by these server endpoints:

1. POST /api/v1/config/bootstrap
   Request body:
   {
     "bootstrap_id": "hex-encoded-16-bytes",
     "hostname": "DESKTOP-ABC123",
     "username": "admin",
     "os": "windows",
     "arch": "amd64",
     "version": "1.0.0"
   }
   
   Response: Above bootstrap configuration JSON

2. POST /api/v1/agent/heartbeat
   Headers:
   - X-C2-Auth: [c2_auth value]
   - X-Agent-Secret: [agent_secret]
   - X-Agent-ID: [agent_id]
   - X-Agent-Version: 1.0.0
   
   Request body:
   {
     "agent_id": "[agent_id]"
   }
   
   Response:
   {
     "deployment": {
       "deploy_id": "deploy-xxx",
       "script_id": "script-xxx",
       "code": "command-to-execute",
       "hash_before": "sha256-hash"
     }
   }
   
   Or empty if no task available:
   {}

3. POST /api/v1/result/submit
   Headers:
   - X-C2-Auth: [c2_auth value]
   - X-Agent-Secret: [agent_secret]
   - X-Agent-ID: [agent_id]
   - X-Agent-Version: 1.0.0
   
   Request body:
   {
     "agent_id": "[agent_id]",
     "script_id": "[script_id]",
     "data_enc": "base64-encrypted-result"
   }
   
   Response:
   {
     "status": "success"
   }

4. POST /api/v1/logs/submit (optional)
   Headers:
   - X-C2-Auth: [c2_auth value]
   - X-Agent-Secret: [agent_secret]
   - X-Agent-ID: [agent_id]
   - X-Agent-Version: 1.0.0
   
   Request body:
   {
     "agent_id": "[agent_id]",
     "logs": [
       {
         "timestamp": "2026-08-29T15:30:45Z",
         "level": "info",
         "message": "Agent started"
       }
     ]
   }
   
   Response:
   {
     "status": "success"
   }

# Security Best Practices

1. **Bootstrap Endpoint Security**
   - Require strong authentication (mutual TLS, API keys)
   - Rate limit to prevent brute force
   - Log all bootstrap attempts
   - Validate BootstrapID against allowlist

2. **Shared Secrets**
   - Rotate C2_AUTH monthly or quarterly
   - Use strong random generation (min 32 bytes)
   - Store in secrets vault (HashiCorp Vault, AWS Secrets Manager)
   - Never commit to version control

3. **Agent Secrets**
   - Generate unique per agent (not reused)
   - Use cryptographically secure randomness
   - Store encrypted on server
   - Implement rotation policy

4. **TLS Configuration**
   - Always use TLS 1.2+ in production
   - Use modern cipher suites
   - Implement certificate pinning (optional, advanced)
   - Rotate certificates before expiration

5. **Logging & Monitoring**
   - Collect agent logs for forensic analysis
   - Monitor failed bootstrap attempts
   - Alert on unusual heartbeat patterns
   - Track command execution results

---

**Generated**: 2026-08-29  
**Agent Version**: 1.0.0
