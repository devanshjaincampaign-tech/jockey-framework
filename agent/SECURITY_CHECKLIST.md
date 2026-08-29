# JOCKY Agent - Security & Operations Checklist

## Pre-Deployment Security Review

### Code & Compilation
- [ ] Review source code for hardcoded secrets (grep for API keys, passwords)
- [ ] Verify compilation uses latest Go version (1.21+)
- [ ] Enable code signing for binary distribution
- [ ] Generate checksums (SHA-256) for integrity verification
- [ ] Store signed checksums securely for distribution

### TLS & Certificates
- [ ] Verify TLS certificates are valid and not self-signed in production
- [ ] Confirm certificate includes correct domain names
- [ ] Check certificate expiration date (set renewal 30 days before)
- [ ] Validate certificate chain completeness
- [ ] Enable TLS_VERIFY in production configuration

### Authentication & Secrets
- [ ] Generate strong C2_AUTH shared secret (32+ bytes)
- [ ] Store C2_AUTH in secrets management system (Vault, AWS Secrets Manager)
- [ ] Generate unique Agent_Secret per agent (never reuse)
- [ ] Implement secret rotation policy (monthly minimum)
- [ ] Document secret management procedures

### Bootstrap Endpoint
- [ ] Implement authentication on /api/v1/config/bootstrap endpoint
- [ ] Add rate limiting (e.g., 10 requests/minute per IP)
- [ ] Validate all bootstrap request parameters
- [ ] Log all bootstrap attempts with source IP
- [ ] Implement bootstrap ID allowlist validation

### Network Security
- [ ] Verify firewall rules allow agent outbound connections
- [ ] Implement network segmentation for C2 server
- [ ] Use VPN/proxy for agent communications (optional)
- [ ] Configure DNS filtering to prevent beacon detection
- [ ] Monitor outbound traffic patterns

### Deployment Security
- [ ] Use secure channel for agent binary distribution
- [ ] Implement code signing verification on target systems
- [ ] Use automated deployment (Group Policy, configuration management)
- [ ] Avoid manual/USB-based deployment where possible
- [ ] Audit deployment logs for unauthorized changes

## Operational Security Checklist

### Ongoing Monitoring
- [ ] Monitor agent heartbeat success rate (target >99%)
- [ ] Alert on failed bootstrap attempts
- [ ] Track command execution latency
- [ ] Monitor agent CPU and memory usage
- [ ] Implement anomaly detection for unusual patterns

### Credential Management
- [ ] Rotate C2_AUTH every 30 days
- [ ] Rotate agent secrets quarterly minimum
- [ ] Implement automated secret rotation (preferred)
- [ ] Audit secret access logs
- [ ] Remove secrets from logs and monitoring systems

### Log Management
- [ ] Configure log rotation (max 100 MB per log)
- [ ] Encrypt logs at rest
- [ ] Implement secure log transmission to server
- [ ] Retain logs for minimum 90 days
- [ ] Regularly review logs for security events

### Access Control
- [ ] Restrict agent.exe file permissions (read-only for operators)
- [ ] Restrict .jocky state directory permissions (0700)
- [ ] Limit C2 server API access to trusted IPs
- [ ] Implement multi-factor authentication for C2 server
- [ ] Use service accounts with minimal privileges

### Incident Response
- [ ] Develop incident response plan for agent compromise
- [ ] Document steps to revoke compromised agent secrets
- [ ] Prepare communication templates for notifications
- [ ] Establish timeline for decommissioning compromised agents
- [ ] Conduct regular incident response drills

## Network Security Configuration

### Firewall Rules

**Outbound (Agent -> C2 Server):**
```
Protocol: TCP/443 (HTTPS)
Destination: c2.example.com
Direction: Outbound
Schedule: Always (except during maintenance)
Logging: Enable
```

**Inbound (C2 Server <- Agent):**
```
Protocol: TCP/443
Source: Agent subnets
Destination: C2 server
Action: Accept
Logging: Enable
```

### DNS Security

**DNS Filtering Recommendations:**
- Block well-known C2 beacon domains (if not using customized infrastructure)
- Implement DNS sinkholing for unauthorized C2 communications
- Monitor DNS queries for anomalies
- Use DNSSEC where available

### DLP Considerations

**Data Loss Prevention Rules:**
- Monitor for exfiltration of sensitive data via agent results
- Alert on large result payloads (>100 MB)
- Implement egress filtering for known sensitive data patterns
- Audit command execution results for PII/confidential data

## Compliance Checklist

### Regulatory Compliance

- [ ] HIPAA: Encrypt PHI in transit and at rest
- [ ] PCI DSS: Secure agent communications for payment systems
- [ ] SOC 2: Document and audit security controls
- [ ] GDPR: Implement data retention and deletion policies
- [ ] CCPA: Ensure agent data handling complies with regulations

### Audit & Logging

- [ ] Maintain comprehensive audit logs of all operations
- [ ] Log all authentication attempts (success and failure)
- [ ] Document command execution with timestamp and operator
- [ ] Track all configuration changes
- [ ] Preserve audit logs for minimum 1 year

### Documentation

- [ ] Maintain asset inventory of deployed agents
- [ ] Document agent deployment locations
- [ ] Record OS versions and patch levels
- [ ] Document authorized operators
- [ ] Create operational runbooks

## Incident Response

### Agent Compromise

**Detection:**
- Unexpected heartbeat pattern changes
- Unusual command execution frequency
- Abnormal result payload sizes
- Failed authentication attempts

**Immediate Actions:**
1. Isolate affected system from network
2. Revoke compromised agent secret
3. Review command execution history
4. Collect forensic evidence
5. Notify relevant stakeholders

**Recovery:**
1. Re-image system if possible
2. Deploy new agent with fresh secret
3. Monitor closely for re-infection
4. Update security monitoring rules

### C2 Server Compromise

**Detection:**
- Unauthorized agent enrollment
- Unusual command patterns
- Unexpected configuration changes
- Authentication failures

**Immediate Actions:**
1. Take C2 server offline
2. Preserve forensic evidence
3. Invalidate all agent secrets
4. Notify all authorized operators
5. Begin incident investigation

**Recovery:**
1. Deploy clean C2 server from backup
2. Re-bootstrap all agents with new secrets
3. Update firewall rules
4. Implement additional monitoring
5. Review incident post-mortem

## Performance & Scalability

### Agent Performance Targets

| Metric | Target | Alert Threshold |
|--------|--------|-----------------|
| Heartbeat Success Rate | >99% | <98% |
| Heartbeat Latency (p95) | <1s | >2s |
| Command Execution Success | >95% | <90% |
| Log Submission Latency | <10s | >30s |
| CPU Usage (idle) | <1% | >5% |
| Memory Usage | <10 MB | >50 MB |

### Server Performance Targets

| Scale | CPU Cores | RAM | Network |
|-------|-----------|-----|---------|
| 1K agents | 2-4 | 4-8 GB | 10 Mbps |
| 10K agents | 8-16 | 16-32 GB | 50-100 Mbps |
| 100K agents | 32+ | 64+ GB | 500+ Mbps |

## Testing Checklist

### Pre-Production Testing

- [ ] Deploy agent to test environment
- [ ] Verify bootstrap configuration retrieval
- [ ] Test all command execution types (shell, registry, etc)
- [ ] Verify encryption/decryption of results
- [ ] Test network failover scenarios
- [ ] Verify log collection and remote submission
- [ ] Load test with multiple concurrent agents
- [ ] Test timeout and error handling
- [ ] Verify secure file permissions on state directory
- [ ] Test agent updates and version upgrades

### Security Testing

- [ ] Verify TLS certificate validation
- [ ] Test authentication header validation
- [ ] Attempt command injection attacks
- [ ] Test with modified/corrupted payloads
- [ ] Verify encrypted data integrity
- [ ] Test key derivation consistency
- [ ] Verify state file permissions (Linux/Windows)
- [ ] Test against network proxies/firewalls

### Operational Testing

- [ ] Test deployment automation scripts
- [ ] Verify log rotation and cleanup
- [ ] Test state directory recovery
- [ ] Test secret rotation procedures
- [ ] Verify incident response procedures
- [ ] Test configuration rollback
- [ ] Test agent decommissioning process

## Deployment Rollout

### Phased Deployment

**Phase 1: Pilot** (Week 1-2)
- Deploy to 5-10% of target systems
- Monitor closely for issues
- Collect feedback from operators
- Verify security controls

**Phase 2: Early Adopters** (Week 3-4)
- Deploy to 25-30% of target systems
- Expand monitoring coverage
- Begin performance baseline
- Train support team

**Phase 3: Standard Deployment** (Week 5-8)
- Deploy to remaining systems in waves
- Stagger deployments to avoid network impact
- Monitor for any compatibility issues
- Prepare for production operations

**Phase 4: Operations** (Ongoing)
- Monitor agent fleet health
- Perform regular backups of C2 server
- Rotate secrets on schedule
- Maintain incident response readiness

### Rollback Plan

- Maintain previous agent version binary
- Document quick rollback procedures
- Test rollback in lab environment
- Prepare for rapid deployment if needed
- Monitor during rollback window

## Maintenance Schedule

### Daily Tasks
- [ ] Monitor agent heartbeat dashboard
- [ ] Review error logs for exceptions
- [ ] Check C2 server disk space
- [ ] Verify backup completion

### Weekly Tasks
- [ ] Review security audit logs
- [ ] Analyze command execution patterns
- [ ] Verify network connectivity
- [ ] Update threat intelligence

### Monthly Tasks
- [ ] Rotate C2_AUTH shared secret
- [ ] Review and update security policies
- [ ] Conduct penetration testing (if applicable)
- [ ] Performance analysis and optimization
- [ ] Capacity planning review

### Quarterly Tasks
- [ ] Rotate agent secrets
- [ ] Full security audit
- [ ] Test disaster recovery procedures
- [ ] Update operational documentation
- [ ] Incident response drill

### Annual Tasks
- [ ] Comprehensive security assessment
- [ ] Update deployment procedures
- [ ] Review and refresh all policies
- [ ] Plan infrastructure upgrades
- [ ] Conduct risk assessment

---

**Document Version**: 1.0  
**Last Updated**: 2026-08-29  
**Next Review**: 2026-12-29
