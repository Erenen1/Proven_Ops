import re
from typing import Dict, Any, List, Optional, Tuple
from ..models.schemas import RootCause, DiagnosisEvidence, StepPlan, RiskLevel

EVIDENCE_PATTERNS: List[Tuple[RootCause, re.Pattern, str]] = [
    (
        RootCause.UNSUPPORTED_RESOURCE,
        re.compile(r"(could not be found|unit [^\s]+ not found|ghost-service|missing-unit|no such service)", re.IGNORECASE),
        "Requested unit or system resource was not found on host"
    ),
    (
        RootCause.PORT_CONFLICT,
        re.compile(r"(address already in use|port conflict|already listening on|occupied by|bind: address already in use)", re.IGNORECASE),
        "Network port is already bound by another active process"
    ),
    (
        RootCause.INVALID_CONFIG,
        re.compile(r"(syntax error|directive [^\s]+ is not allowed|configuration test failed|test failed|emerg.*directive|unknown directive)", re.IGNORECASE),
        "Service configuration file contains invalid syntax or unknown directives"
    ),
    (
        RootCause.PERMISSION_DENIED,
        re.compile(r"(permission denied|operation not permitted|read-only file system|access denied|eacces)", re.IGNORECASE),
        "Operation failed due to filesystem permissions or capability restrictions"
    ),
    (
        RootCause.DNS_FAILURE,
        re.compile(r"(name or service not known|nxdomain|could not resolve host|temporary failure in name resolution)", re.IGNORECASE),
        "Domain name resolution failed via configured DNS resolvers"
    ),
    (
        RootCause.CONNECTION_REFUSED,
        re.compile(r"(connection refused|econnrefused|connect: connection refused)", re.IGNORECASE),
        "Target endpoint refused TCP connection (service not listening)"
    ),
    (
        RootCause.HTTP_APPLICATION_FAILURE,
        re.compile(r"(500 internal server error|http/1\.[01] 500|502 bad gateway|503 service unavailable|status code 500)", re.IGNORECASE),
        "Target HTTP application server returned a 5xx server error response"
    ),
    (
        RootCause.DISK_PRESSURE,
        re.compile(r"(no space left on device|disk pressure|disk full|out of disk space)", re.IGNORECASE),
        "Filesystem has exhausted available disk space or inodes"
    ),
    (
        RootCause.AGENT_DISCONNECTED,
        re.compile(r"(agent.*disconnected|heartbeat missed|connection to agent lost|agent transport closed)", re.IGNORECASE),
        "Server Agent mTLS gRPC connection dropped or missed heartbeats"
    ),
    (
        RootCause.POLICY_DENIED,
        re.compile(r"(policy denied|security policy violation|forbidden action|blocked by policy|not permitted by rbac)", re.IGNORECASE),
        "Operation violated ProvenOps security guardrails or RBAC policy"
    ),
    (
        RootCause.CONTAINER_CRASH,
        re.compile(r"(container.*exited|oomkilled|container.*crashed|dead container)", re.IGNORECASE),
        "Container process terminated unexpectedly or was terminated by OOM killer"
    ),
    (
        RootCause.SERVICE_CRASH,
        re.compile(r"(service.*crashed|core dumped|segfault|sigsegv|process killed by signal)", re.IGNORECASE),
        "Systemd unit process crashed or terminated with fatal signal"
    ),
    (
        RootCause.SERVICE_STOPPED,
        re.compile(r"(inactive \(dead\)|service is stopped|failed to start|unit is not active)", re.IGNORECASE),
        "Target systemd service is currently inactive or failed to start"
    ),
    (
        RootCause.PACKAGE_MISSING,
        re.compile(r"(package [^\s]+ is not installed|dpkg: error|unable to locate package|no package found)", re.IGNORECASE),
        "Required Linux package is missing from host package cache"
    ),
    (
        RootCause.TIMEOUT,
        re.compile(r"(timed out|timeout exceeded|deadline exceeded|context deadline exceeded)", re.IGNORECASE),
        "Operation exceeded execution deadline or verification probe timed out"
    ),
    (
        RootCause.IDEMPOTENT_SATISFIED,
        re.compile(r"(already installed|already active|idempotent satisfied|nothing to do)", re.IGNORECASE),
        "System state already matches target condition"
    ),
]

def extract_evidence_and_root_cause(symptom: str, logs: str) -> Tuple[RootCause, float, List[DiagnosisEvidence]]:
    """
    Deterministic evidence extraction matching system logs and symptoms against SRE root cause taxonomy.
    """
    combined = f"{symptom}\n{logs}"
    evidences: List[DiagnosisEvidence] = []
    detected_cause = RootCause.UNKNOWN
    confidence = 0.5

    for cause, pattern, desc in EVIDENCE_PATTERNS:
        match = pattern.search(combined)
        if match:
            detected_cause = cause
            confidence = 0.95
            snippet = match.group(0)
            evidences.append(DiagnosisEvidence(
                source="evidence_log_analysis",
                detail=f"{desc} (matched: '{snippet}')"
            ))
            break

    if detected_cause == RootCause.UNKNOWN and symptom:
        evidences.append(DiagnosisEvidence(
            source="symptom_inspection",
            detail=f"Reported symptom: {symptom}"
        ))

    return detected_cause, confidence, evidences

def generate_grounded_remediation(cause: RootCause, symptom: str) -> List[Dict[str, Any]]:
    """
    Produce safe, grounded remediation and diagnostic steps based on identified root cause.
    """
    steps: List[Dict[str, Any]] = []

    if cause == RootCause.PORT_CONFLICT:
        steps.append({
            "id": "step-1",
            "action": "execute_command",
            "arguments": {"command": "ss -tulpn"},
            "reason": "Identify active sockets and processes occupying conflicting port",
            "suggested_risk": "READ_ONLY",
            "verification_strategy": None
        })
    elif cause == RootCause.INVALID_CONFIG:
        steps.append({
            "id": "step-1",
            "action": "execute_command",
            "arguments": {"command": "nginx -t"},
            "reason": "Test configuration syntax to locate erroneous line or directive",
            "suggested_risk": "READ_ONLY",
            "verification_strategy": None
        })
    elif cause == RootCause.UNSUPPORTED_RESOURCE:
        steps.append({
            "id": "step-1",
            "action": "execute_command",
            "arguments": {"command": "systemctl list-unit-files --type=service"},
            "reason": "Verify installed systemd service unit files to detect missing dependencies",
            "suggested_risk": "READ_ONLY",
            "verification_strategy": None
        })
    elif cause == RootCause.PERMISSION_DENIED:
        steps.append({
            "id": "step-1",
            "action": "execute_command",
            "arguments": {"command": "ls -ld /etc/nginx /var/www/html"},
            "reason": "Inspect file and directory ownership and permissions",
            "suggested_risk": "READ_ONLY",
            "verification_strategy": None
        })
    elif cause == RootCause.SERVICE_STOPPED or cause == RootCause.SERVICE_CRASH:
        steps.append({
            "id": "step-1",
            "action": "execute_command",
            "arguments": {"command": "journalctl -xe --no-pager -n 50"},
            "reason": "Examine recent journalctl logs for termination or crash signals",
            "suggested_risk": "READ_ONLY",
            "verification_strategy": None
        })
    elif cause == RootCause.DISK_PRESSURE:
        steps.append({
            "id": "step-1",
            "action": "execute_command",
            "arguments": {"command": "df -h && du -sh /var/log/* 2>/dev/null | sort -rh | head -n 10"},
            "reason": "Locate disk usage hotspots and oversized log files",
            "suggested_risk": "READ_ONLY",
            "verification_strategy": None
        })
    else:
        steps.append({
            "id": "step-1",
            "action": "get_system_info",
            "arguments": {},
            "reason": "Collect system telemetry to assess host health",
            "suggested_risk": "READ_ONLY",
            "verification_strategy": None
        })

    return steps
