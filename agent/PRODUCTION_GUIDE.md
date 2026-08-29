# JOCKY Agent - Production Deployment Guide

## Overview

This document provides comprehensive guidance for deploying the JOCKY Agent in production environments. The agent has been hardened with security, reliability, and operational best practices.

## Key Production Features

### Security Hardening
- **Cryptographically Secure Random Generation**: Uses `crypto/rand` instead of time-based randomization
- **Proper Key Derivation**: PBKDF2 with SHA256 (100,000 iterations) instead of simple SHA256
- **AES-256-GCM Encryption**: Authenticated encryption for all result payloads
- **Secure HTTP Transport**: Configurable TLS verification, proper certificate validation
- **No Hardcoded Credentials**: All configuration comes from bootstrap server
- **Auth Header Injection**: X-C2-Auth, X-Agent-Secret, X-Agent-ID in all requests

### Reliability & Resilience
- **Exponential Backoff Retry Logic**: Automatic retries with jitter for failed operations
- **Configurable Timeouts**: Per-operation timeout with graceful error handling
- **Local State Persistence**: Agent configuration cached locally with secure file permissions (0600)
- **Async Log Buffering**: Non-blocking log submission with 100-entry buffer
- **Command Execution Timeout**: Script execution with timeout protection
- **Connection Pooling**: Reusable HTTP transport with connection limits

### Operational Excellence
- **Remote Configuration Server**: Bootstrap configuration from server, not embedded in binary
- **Version Tracking**: Agent version included in all requests
- **Comprehensive Logging**: File-based logs + remote log submission with timestamps
- **Graceful Degradation**: Continues operating even if logging fails
- **Agent Identity Validation**: Bootstrap ID + Agent ID + Agent Secret authentication

## Deployment Architecture

```
┌─────────────┐
│   Jockey    │
│   Manager   │
│   Server    │
└──────┬──────┘
       │ Bootstrap Request
       │ ├─ BootstrapID
       │ ├─ Hostname
       │ ├─ Username
       │ └─ OS/Arch/Version
       │
       │ Bootstrap Response
       │ ├─ AgentID (unique)
       │ ├─ AgentSecret (32-byte, derived key)
       │ └─ AgentConfig (listener, timeout, etc)
       │
┌──────▼──────────┐
│  JOCKY Agent    │
├─────────────────┤
│ Local Storage:  │
│ - agent.state   │
│ - agent.log     │
└─────────────────┘
```

## Deployment Steps

### 1. Prepare Bootstrap Server

Ensure your C2 server exposes the bootstrap endpoint:
```
POST /api/v1/config/bootstrap
```

Expected request body:
```json
{
  "bootstrap_id": "hex-encoded-random-16-bytes",
  "hostname": "DESKTOP-ABC123",
  "username": "admin",
  "os": "windows",
  "arch": "amd64",
  "version": "1.0.0"
}
```

Expected response:
```json
{
  "valid": true,
  "agent_id": "agent-unique-identifier",
  "agent_secret": "base64-encoded-secret-or-plaintext",
  "config": {
    "listener_url": "https://c2.example.com",
    "front_domain": "c2.example.com",
    "c2_auth": "shared-secret-key",
    "heartbeat_interval": 30000000000,
    "tls_verify": true,
    "log_level": "info",
    "timeout": 15000000000
  }
}
```

### 2. Build the Agent

```bash
cd agent/
go mod tidy
go build -o agent.exe .
```

Output: `agent.exe` (approximately 10.4 MB)

### 3. Deploy Agent Binary

Deploy `agent.exe` to target system:
- Use Group Policy (Windows)
- Use configuration management tools (Ansible, Puppet, Chef)
- Package with installation scripts
- Deploy via USB/removable media for air-gapped networks

### 4. Run the Agent

**Basic execution:**
```bash
agent.exe
```

**With custom bootstrap server:**
Set the `ConfigBootstrapURL` environment variable or recompile with custom configuration.

**Persistence:**
- Register as Windows Service (for Windows):
  ```powershell
  New-Service -Name "JockeyAgent" -BinaryPathName "C:\Path\To\agent.exe" -DisplayName "Jockey Agent" -StartupType Automatic
  ```

- Scheduled Task (alternative):
  ```powershell
  Register-ScheduledTask -TaskName "JockeyAgent" -Action (New-ScheduledTaskAction -Execute "C:\Path\To\agent.exe")
  ```

- Linux/macOS: Use systemd or launchd plist

## Configuration Details

### AgentConfig Structure

The agent downloads this configuration from the server during bootstrap:

```go
type AgentConfig struct {
    ListenerURL    string        // C2 server base URL
    FrontDomain    string        // TLS SNI hostname
    C2Auth         string        // Shared authentication secret
    HeartbeatInt   time.Duration // Heartbeat interval (default 30s)
    TLSVerify      bool          // Validate TLS certificates
    LogLevel       string        // Log verbosity
    Timeout        time.Duration // HTTP request timeout
}
```

### Timeouts & Intervals

- **Heartbeat Interval**: Default 30 seconds (configurable via server)
- **HTTP Request Timeout**: Default 15 seconds
- **Command Execution Timeout**: Inherits HTTP timeout
- **Backoff Calculation**: `base * 2^attempt + jitter` (capped at 5 minutes)

### Retry Logic

- **Max Retries**: 5 attempts
- **Initial Backoff**: 1 second
- **Exponential Base**: 2x multiplier per attempt
- **Jitter**: ±25% random variance
- **Max Backoff**: 5 minutes

Example retry schedule:
1. Attempt 1 → immediate
2. Attempt 2 → ~1-1.5 sec
3. Attempt 3 → ~2-3 sec
4. Attempt 4 → ~4-6 sec
5. Attempt 5 → ~8-12 sec

## Local State Management

The agent maintains local state in `%TEMP%/.jocky/`:

```
%TEMP%\.jocky\
├── agent.state      (0600) - Serialized AgentID, AgentSecret, Config
└── agent.log        (0600) - Local activity log
```

**File Permissions:**
- Linux/macOS: 0700 (rwx------)
- Windows: Restricted to current user

**State Persistence:**
- Automatically saved after successful bootstrap
- Reused on subsequent runs (speeds up startup)
- Automatically regenerated if corrupted

## Logging

### Log Format

```
[YYYY-MM-DD HH:MM:SS] [LEVEL]: Message
```

### Log Levels

- `debug` - Detailed troubleshooting info
- `info` - Standard operational events
- `warn` - Warning conditions
- `error` - Error conditions

### Local Logging

All logs written to `%TEMP%/.jocky/agent.log` with append mode.

### Remote Logging

Logs batched and submitted to:
```
POST /api/v1/logs/submit
```

Batch size: 100 entries max
Submission interval: 10 seconds

Log submission payload:
```json
{
  "agent_id": "agent-xxx",
  "logs": [
    {
      "timestamp": "2026-08-29T15:30:45Z",
      "level": "info",
      "message": "Bootstrap successful..."
    }
  ]
}
```

## Encryption Details

### Key Derivation

```
Key = PBKDF2(
  password: AgentSecret,
  salt: "jocky-agent-kdf-v1",
  iterations: 100,000,
  keyLength: 32 bytes,
  hash: SHA-256
)
```

### Payload Encryption

- **Algorithm**: AES-256 in GCM mode (Galois/Counter Mode)
- **Nonce**: 12-byte random (cryptographically secure)
- **Encoding**: Base64 encoding of (nonce || ciphertext)

### Decryption on Server

1. Base64 decode payload
2. Extract first 12 bytes as nonce
3. AES-256-GCM decrypt remaining bytes
4. Verify authentication tag

## Command Execution

### Supported Command Formats

1. **Plain Shell Command:**
   ```
   ipconfig /all
   ```

2. **JOCKY exec() Function:**
   ```
   exec("whoami")
   ```

3. **Registry Collection:**
   ```
   collect_registry("HKLM\\Software\\Microsoft\\Windows")
   ```

### Execution Behavior

- **Windows**: `cmd /c [command]`
- **Linux/macOS**: `sh -c [command]`
- **Output Limit**: 10 MB per command
- **Timeout**: Configured per agent (default 15s)

### Error Handling

- Command errors logged with error prefix: `error: [message]`
- Timeout returns: `error: execution timeout`
- Output captured from stdout (stderr merged on some systems)

## Security Considerations

### Network Security

1. **Always use HTTPS** for all communication
2. **Validate TLS certificates** in production (`tls_verify: true`)
3. **Use DNS filtering** to prevent C2 beacon detection
4. **Monitor outbound connections** from agent systems

### Credential Management

1. **Bootstrap server** must be secure and authenticated
2. **Shared C2 secret** should be strong and rotated
3. **Agent secrets** generated server-side, never hardcoded
4. **Log files** contain sensitive activity - restrict access

### Operational Security

1. **Disable debug logging** in production (`log_level: "info"`)
2. **Rotate bootstrap IDs** periodically
3. **Monitor heartbeat patterns** for anomalies
4. **Clean up log files** on decommissioned systems
5. **Use code signing** for agent binary distribution

## Monitoring & Troubleshooting

### Key Metrics

Monitor these agent indicators:
- Heartbeat success rate (should be >99%)
- Average heartbeat latency
- Task execution success rate
- Failed bootstrap attempts
- Command execution timeouts

### Common Issues

**Issue: Agent fails to bootstrap**
- Cause: Network unreachable or server down
- Resolution: Check network connectivity, verify bootstrap URL, review firewall rules
- Logs: Check `%TEMP%/.jocky/agent.log` for error details

**Issue: High latency on heartbeats**
- Cause: Network congestion or server overload
- Resolution: Increase heartbeat interval via server config, check network health
- Logs: Review `[DEBUG]` logs for timing information

**Issue: Commands timing out**
- Cause: Command takes longer than configured timeout
- Resolution: Increase timeout via server config, optimize commands
- Logs: Watch for `[WARN] Command execution timeout` messages

**Issue: Encryption/decryption errors**
- Cause: Mismatched agent secrets or corrupted state
- Resolution: Delete local state file (`agent.state`), force re-bootstrap
- Logs: Look for decryption-related errors

### Debug Mode

To enable detailed logging, modify bootstrap response:
```json
"config": {
  "log_level": "debug",
  ...
}
```

This produces verbose output in `%TEMP%/.jocky/agent.log`.

## Scalability & Performance

### Single Agent Performance

- **Memory Footprint**: ~5-10 MB at rest
- **CPU Usage**: <1% idle (negligible)
- **Network Bandwidth**: ~1-5 KB per heartbeat + result payloads
- **Disk I/O**: Minimal (only logging)

### Multiple Agent Deployment

Estimated server requirements:
- **1,000 agents**: 1-2 CPU cores, 2 GB RAM, 10 Mbps network
- **10,000 agents**: 4-8 CPU cores, 8 GB RAM, 50 Mbps network
- **100,000 agents**: 16+ CPU cores, 32+ GB RAM, 500+ Mbps network

Optimization tips:
- Stagger heartbeats using jitter
- Implement task queueing on server
- Use connection pooling
- Compress large result payloads
- Implement log rotation/cleanup

## Updates & Maintenance

### Agent Updates

To deploy a new agent version:

1. Compile new binary with same version constant
2. Deploy alongside existing agent
3. Gradually migrate via bootstrap server config changes
4. Old agents can coexist during transition period

### Security Patches

Critical security patches should be deployed as soon as possible:
1. Update source code
2. Rebuild binary
3. Use fast-track deployment mechanism
4. Monitor adoption rate

## License & Support

This agent is part of the JOCKY Framework. For issues or questions, refer to the main project documentation.

---

**Last Updated**: 2026-08-29  
**Agent Version**: 1.0.0  
**Go Version Required**: 1.21+
