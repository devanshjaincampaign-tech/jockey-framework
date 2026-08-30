# JOCKY Framework - Complete Integration Setup

## System Architecture

```
┌─────────────────────────────────────────────────────────┐
│                  JOCKY Framework                        │
├─────────────────────────────────────────────────────────┤
│                                                         │
│  ┌──────────────────────┐        ┌──────────────────┐   │
│  │   Manager Service    │        │  Dashboard UI    │   │
│  │   (Flask + SQLite)   │◄──────►│  (Real-time)     │   │
│  │   Port: 5000         │        │  Localhost:5000  │   │
│  └──────────────────────┘        └──────────────────┘   │
│           ▲                                             │
│           │ HTTP/JSON                                   │
│           │                                             │
│  ┌────────┴───────────────────────────────────────────┐ │
│  │                                                    │ │
│  ├─ /api/v1/config/bootstrap    (New agent init)      │ │
│  ├─ /api/v1/agent/heartbeat     (Status updates)      │ │
│  ├─ /api/v1/result/submit       (Command results)     │ │
│  ├─ /api/v1/logs/submit         (Agent logs)          │ │
│  ├─ /api/v1/script/deploy       (Send commands)       │ │
│  └─ /                            (Dashboard)          │ │
│                                                         │
│  Database: instance/jocky.db (SQLite)                   │
└─────────────────────────────────────────────────────────┘

                        Network (HTTP)
                              │
                              ▼

┌──────────────────────────────────────┐
│      JOCKY Agent                     │
│      (Go Binary)                     │
├──────────────────────────────────────┤
│ • Bootstrap from manager             │
│ • Execute commands locally           │
│ • Encrypt/decrypt results            │
│ • Send logs to manager               │
│ • Persistent state management        │
└──────────────────────────────────────┘
```

## Quick Start - Full System Integration

### 1. Install Manager Dependencies

```bash
cd manager/
pip install -r requirements.txt
```

### 2. Start the Manager

```bash
cd manager/
python app.py
```

Expected output:
```
✅ Database tables created/verified.
 * Running on http://0.0.0.0:5000
 * Debug mode: on
```

### 3. In a New Terminal - Start the Agent

```bash
cd agent/
./agent.exe
```

Expected output:
```
[*] JOCKY Agent agent-xxxxxxxx starting (version 1.0.0)
[+] Agent registered: agent-xxxxxxxx
[+] Beaconing to: http://localhost:5000
```

### 4. Open Dashboard

Navigate to: **http://localhost:5000**

You should see:
- **Agents List**: Shows your connected agent
- **Agent Details**: Hostname, OS, status, last seen
- **Commands**: Send commands to the agent
- **Results**: View command execution results
- **Logs**: Real-time agent activity logs

## System Flow

### Initial Bootstrap (First Run)

```
Agent                          Manager
  │                               │
  ├─ POST /api/v1/config/bootstrap
  │  (bootstrap_id, hostname, os) ─────►
  │                               │
  │                        [Generate ID & Secret]
  │                               │
  │  ◄───── Valid: true           │
  │         agent_id: xxx         │
  │         agent_secret: yyy     │
  │         config: {...}         │
  │
  └─ Save local state (.jocky/agent.state)
```

### Regular Heartbeat (Every 30 seconds)

```
Agent                          Manager
  │                               │
  ├─ POST /api/v1/agent/heartbeat
  │  (agent_id, headers with secret)
  │                           ────►
  │                               │
  │                   [Check for pending tasks]
  │                               │
  │  ◄─ deployment: {script}      │
  │     or empty if no tasks      │
  │                               │
  ├─ Execute command locally
  ├─ Encrypt result
  │
  ├─ POST /api/v1/result/submit
  │  (agent_id, script_id, data_enc)
  │                           ────►
  │                               │
  │  ◄─ result_id: zzz            │
  │
  ├─ Async POST /api/v1/logs/submit
  │  (agent_id, logs array)   ────►
```

## Dashboard Features

### Real-Time Updates

- **Agent List**: Refreshes every 10 seconds
- **Agent Details**: Status, hostname, OS, IP, last seen
- **Command Results**: New results appear within 5 seconds
- **Agent Logs**: Real-time log stream with color-coding

### Log Levels

- **DEBUG** (gray): Detailed troubleshooting
- **INFO** (blue): Standard operations
- **WARN** (yellow): Warnings
- **ERROR** (red): Error conditions

### Sending Commands

1. Click "View Details" on an agent
2. Click "Send Command"
3. Enter command (e.g., `whoami`, `ipconfig`)
4. Command executes and results appear in "Recent Results"

## Integration Points

### 1. Bootstrap Endpoint ✅
- **URL**: `POST /api/v1/config/bootstrap`
- **Status**: Implemented in `manager/api/config_routes.py`
- **Returns**: Agent ID, Secret, Configuration

### 2. Encryption ✅
- **Algorithm**: AES-256-GCM
- **Key Derivation**: PBKDF2-SHA256 (100,000 iterations)
- **Salt**: "jocky-agent-kdf-v1"
- **Status**: Matches between agent and manager

### 3. Logs Endpoint ✅
- **URL**: `POST /api/v1/logs/submit`
- **Status**: Implemented in `manager/api/logs_routes.py`
- **View**: GET `/api/v1/logs/stream/<agent_id>`

### 4. Dashboard ✅
- **URL**: `http://localhost:5000/`
- **Status**: Fully functional with real-time updates
- **Features**: Agent management, command execution, result viewing, log streaming

## Configuration

### Manager Configuration (config.py)

```python
SQLALCHEMY_DATABASE_URI = 'sqlite:///instance/jocky.db'
C2_AUTH = 'jocky-c2-secret-key'  # Shared secret
```

### Agent Configuration (in bootstrap response)

Sent by manager during bootstrap:
```json
{
  "listener_url": "http://localhost:5000",
  "front_domain": "localhost",
  "c2_auth": "jocky-c2-secret-key",
  "heartbeat_interval": 30000000000,
  "tls_verify": false,
  "log_level": "debug",
  "timeout": 15000000000
}
```

## Troubleshooting

### Agent Won't Connect

1. **Check Manager is Running**
   ```bash
   curl http://localhost:5000/
   ```

2. **Check Bootstrap Endpoint**
   ```bash
   curl -X POST http://localhost:5000/api/v1/config/bootstrap \
     -H "Content-Type: application/json" \
     -d '{"bootstrap_id": "test123", "hostname": "test-pc", "username": "admin", "os": "windows", "arch": "amd64", "version": "1.0.0"}'
   ```

3. **Check Agent Logs**
   ```bash
   cat %TEMP%\.jocky\agent.log
   ```

### Manager Database Issues

Clear and reinitialize:
```bash
cd manager/
rm instance/jocky.db
python app.py
```

### Encryption Mismatch

If you see "Invalid encrypted payload" errors:
- Verify PBKDF2 implementation matches (100,000 iterations)
- Check salt is "jocky-agent-kdf-v1"
- Ensure AES-256-GCM is used

## Testing the Integration

### Test 1: Agent Registration

```bash
# Terminal 1: Manager
cd manager && python app.py

# Terminal 2: Agent
cd agent && ./agent.exe

# Expected: Agent shows in dashboard within 30 seconds
```

### Test 2: Send Command

```bash
# In dashboard:
# 1. Click agent
# 2. Click "Send Command"
# 3. Enter: whoami
# 4. Result appears in "Recent Results"
```

### Test 3: Verify Encryption

```bash
# In manager logs, verify:
# - Results encrypted with AES-256-GCM
# - Decryption succeeds
# - Plaintext shown in dashboard
```

### Test 4: Real-Time Updates

```bash
# Send multiple commands quickly
# Verify results appear within 5 seconds
# Verify logs update in real-time
```

## Files Modified for Full Integration

### Manager
- ✅ `manager/api/config_routes.py` (NEW) - Bootstrap endpoint
- ✅ `manager/api/logs_routes.py` (NEW) - Log submission endpoint
- ✅ `manager/api/result_routes.py` (UPDATED) - PBKDF2 encryption
- ✅ `manager/api/agent_routes.py` (UPDATED) - Removed duplicate bootstrap
- ✅ `manager/app.py` (UPDATED) - Registered new blueprints
- ✅ `manager/dashboard/templates/index.html` (UPDATED) - Real-time dashboard
- ✅ `manager/dashboard/static/dashboard.js` (UPDATED) - Poll logic

### Agent
- ✅ `agent/main.go` (UPDATED) - Config points to localhost:5000
- ✅ `agent/go.mod` (UPDATED) - PBKDF2 dependency
- ✅ `agent/agent.exe` (REBUILT) - Production binary

## Next Steps

### For Development
1. Test command execution with various inputs
2. Monitor encryption/decryption performance
3. Validate log streaming under load
4. Test agent reconnection after network issues

### For Production
1. Replace localhost with production domain
2. Enable TLS certificate validation
3. Implement proper authentication on bootstrap
4. Set up log persistence (database instead of memory)
5. Configure automated agent deployment

## Performance Metrics

Expected from local testing:
- **Bootstrap**: <100ms
- **Heartbeat**: 50-200ms
- **Command Execution**: <1s for simple commands
- **Result Transmission**: 100-500ms
- **Dashboard Update**: <5s latency
- **Memory (Agent)**: 5-10 MB
- **CPU (Agent)**: <1% idle

## Support

For issues:
1. Check manager logs: `python app.py` output
2. Check agent logs: `%TEMP%/.jocky/agent.log`
3. Verify endpoints with curl
4. Review Python traceback for server errors
5. Review Go errors for agent issues

---

**Status**: ✅ Fully Integrated  
**Last Updated**: 2026-08-29  
**Version**: 1.0.0
