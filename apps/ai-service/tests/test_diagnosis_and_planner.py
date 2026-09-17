import pytest
from app.models.schemas import (
    PlanRequest,
    PlanResponse,
    ReplanRequest,
    DiagnosisRequest,
    DiagnosisResponse,
    HostContext,
    RootCause
)
from app.services.diagnosis_grounding import (
    extract_evidence_and_root_cause,
    generate_grounded_remediation,
    EVIDENCE_PATTERNS
)
from app.providers.ollama_provider import OllamaProvider
from app.prompts.planner import (
    build_planning_prompt,
    build_replanning_prompt,
    build_diagnosis_prompt
)

def test_extract_evidence_canonical_causes():
    cases = [
        ("Nginx failed to bind", "nginx: [emerg] bind() to 0.0.0.0:8080 failed (98: Address already in use)", RootCause.PORT_CONFLICT),
        ("Invalid directive", "nginx: [emerg] unknown directive 'server_nam' in /etc/nginx/sites-enabled/default:5", RootCause.INVALID_CONFIG),
        ("Missing service unit", "Failed to restart ghost-service.service: Unit ghost-service.service could not be found.", RootCause.UNSUPPORTED_RESOURCE),
        ("Permission denied", "open() '/var/log/app.log' failed (13: Permission denied)", RootCause.PERMISSION_DENIED),
        ("DNS resolution issue", "curl: (6) Could not resolve host: api.internal.corp; Name or service not known", RootCause.DNS_FAILURE),
        ("Refused connection", "dial tcp 127.0.0.1:9090: connect: connection refused", RootCause.CONNECTION_REFUSED),
        ("HTTP 500 error", "HTTP/1.1 500 Internal Server Error returned by backend", RootCause.HTTP_APPLICATION_FAILURE),
        ("Disk full", "write /var/lib/data: no space left on device", RootCause.DISK_PRESSURE),
        ("Agent disconnected", "Agent heartbeat missed for 60s, transport closed", RootCause.AGENT_DISCONNECTED),
        ("Policy blocked", "Command blocked: security policy violation (destructive pattern)", RootCause.POLICY_DENIED),
        ("Container died", "Container worker-1 exited with code 137 (OOMKilled)", RootCause.CONTAINER_CRASH),
        ("Service terminated", "Process 1234 terminated with SIGSEGV (core dumped)", RootCause.SERVICE_CRASH),
        ("Service stopped", "Active: inactive (dead) since Wed 2026-09-16", RootCause.SERVICE_STOPPED),
        ("Package missing", "dpkg: error processing package nginx-custom (--install)", RootCause.PACKAGE_MISSING),
        ("Execution timeout", "verification check timed out after 15 seconds", RootCause.TIMEOUT),
        ("Idempotent satisfied", "package nginx is already installed and up to date", RootCause.IDEMPOTENT_SATISFIED),
    ]

    for symptom, logs, expected_cause in cases:
        cause, conf, evidences = extract_evidence_and_root_cause(symptom, logs)
        assert cause == expected_cause, f"Failed for {symptom}: expected {expected_cause}, got {cause}"
        assert conf >= 0.9
        assert len(evidences) >= 1

def test_generate_grounded_remediation_safety():
    for cause in RootCause:
        steps = generate_grounded_remediation(cause, "sample symptom")
        assert len(steps) >= 1
        for s in steps:
            assert s["suggested_risk"] == "READ_ONLY"
            if "arguments" in s and "command" in s["arguments"]:
                cmd = s["arguments"]["command"]
                assert "rm -rf" not in cmd
                assert "shutdown" not in cmd
                assert "reboot" not in cmd

def test_heuristic_fallback_diagnosis():
    provider = OllamaProvider(base_url="http://localhost:11434", model_name="qwen2.5:3b")
    prompt = 'Reported Symptom: "Port 8080 bind failure"\n<UNTRUSTED_OBSERVATION>\nbind: address already in use\n</UNTRUSTED_OBSERVATION>'
    data = provider._heuristic_fallback(prompt, purpose="DIAGNOSIS")
    diagnosis = DiagnosisResponse.model_validate(data)
    assert diagnosis.root_cause == RootCause.PORT_CONFLICT.value
    assert diagnosis.confidence >= 0.9
    assert len(diagnosis.remediation_steps) >= 1

def test_heuristic_fallback_replan():
    provider = OllamaProvider(base_url="http://localhost:11434", model_name="qwen2.5:3b")
    prompt = 'REPLANNING REQUEST\nfailed_action: restart_service\n<UNTRUSTED_OBSERVATION>\naddress already in use\n</UNTRUSTED_OBSERVATION>'
    data = provider._heuristic_fallback(prompt, purpose="REPLAN")
    plan = PlanResponse.model_validate(data)
    assert len(plan.steps) >= 1
    assert plan.steps[0].action == "execute_command"
    assert "ss -tulpn" in plan.steps[0].arguments.get("command", "")

def test_heuristic_fallback_plan():
    provider = OllamaProvider(base_url="http://localhost:11434", model_name="qwen2.5:3b")
    prompt = "Install nginx on port 8080"
    data = provider._heuristic_fallback(prompt, purpose="PLAN")
    plan = PlanResponse.model_validate(data)
    assert "8080" in plan.goal
    assert len(plan.steps) >= 3
