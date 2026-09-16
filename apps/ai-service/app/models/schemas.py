from enum import Enum
from typing import Any, Dict, List, Optional
from pydantic import BaseModel, Field

class RiskLevel(str, Enum):
    READ_ONLY = "READ_ONLY"
    LOW = "LOW"
    MEDIUM = "MEDIUM"
    HIGH = "HIGH"
    FORBIDDEN = "FORBIDDEN"

class VerificationRequirement(BaseModel):
    check_type: str = Field(..., description="e.g. systemd_active, tcp_port_open, http_probe, package_installed")
    target: str = Field(..., description="e.g. nginx, 8080, http://localhost:8080")
    expected: Optional[str] = Field(None, description="Expected value or condition, e.g. 200, active")
    timeout_sec: int = Field(15, description="Verification timeout in seconds")

class StepPlan(BaseModel):
    id: str = Field(..., description="Unique step identifier, e.g. step-1")
    action: str = Field(..., description="Typed action name, e.g. install_package, restart_service")
    arguments: Dict[str, Any] = Field(default_factory=dict, description="Arguments for the typed tool")
    reason: str = Field(..., description="Brief explanation of why this step is necessary")
    suggested_risk: Optional[str] = Field(default="LOW", description="Model suggested risk level (overridden by Control Plane)")
    verification_strategy: Optional[VerificationRequirement] = Field(None, description="Deterministic verification check for this step")

class HostContext(BaseModel):
    hostname: str
    os: str = "linux"
    distribution: str = "ubuntu"
    version: str = "24.04"
    architecture: str = "amd64"
    capabilities: List[str] = Field(default_factory=list)
    open_ports: Optional[List[int]] = Field(default_factory=list)
    active_services: Optional[List[str]] = Field(default_factory=list)

class PlanRequest(BaseModel):
    task_id: str
    intent: str
    host_context: HostContext
    supported_tools: List[str] = Field(default_factory=list)
    untrusted_observations: Optional[List[str]] = Field(default_factory=list, description="Host command outputs and logs, strictly isolated")

class ProvenanceMetadata(BaseModel):
    invocation_id: str
    trace_id: Optional[str] = None
    span_id: Optional[str] = None
    task_id: Optional[str] = None
    scenario_id: Optional[str] = None
    purpose: str = "PLAN"
    provider: str = "ollama"
    model: str
    model_digest: Optional[str] = None
    fallback_used: bool = False
    fallback_reason: Optional[str] = None
    latency_ms: int = 0
    schema_valid: bool = True
    error_type: Optional[str] = None

class PlanResponse(BaseModel):
    goal: str
    reasoning: str
    steps: List[StepPlan]
    overall_verification: Optional[List[VerificationRequirement]] = Field(default_factory=list)
    provenance: Optional[ProvenanceMetadata] = None

class RootCause(str, Enum):
    PORT_CONFLICT = "PORT_CONFLICT"
    INVALID_CONFIG = "INVALID_CONFIG"
    PACKAGE_MISSING = "PACKAGE_MISSING"
    SERVICE_STOPPED = "SERVICE_STOPPED"
    SERVICE_CRASH = "SERVICE_CRASH"
    RESTART_LOOP = "RESTART_LOOP"
    PERMISSION_DENIED = "PERMISSION_DENIED"
    DNS_FAILURE = "DNS_FAILURE"
    CONNECTION_REFUSED = "CONNECTION_REFUSED"
    HTTP_APPLICATION_FAILURE = "HTTP_APPLICATION_FAILURE"
    DISK_PRESSURE = "DISK_PRESSURE"
    CONTAINER_CRASH = "CONTAINER_CRASH"
    CONTAINER_RESTART_LOOP = "CONTAINER_RESTART_LOOP"
    CONTAINER_UNHEALTHY = "CONTAINER_UNHEALTHY"
    IMAGE_NOT_FOUND = "IMAGE_NOT_FOUND"
    AGENT_DISCONNECTED = "AGENT_DISCONNECTED"
    TIMEOUT = "TIMEOUT"
    UNSUPPORTED_RESOURCE = "UNSUPPORTED_RESOURCE"
    POLICY_DENIED = "POLICY_DENIED"
    IDEMPOTENT_SATISFIED = "IDEMPOTENT_SATISFIED"
    NONE = "NONE"
    UNKNOWN = "UNKNOWN"

class ReplanRequest(BaseModel):
    task_id: str
    intent: str
    host_context: HostContext
    failed_step_id: str
    failed_action: str
    exit_code: Optional[int] = None
    untrusted_stdout: Optional[str] = ""
    untrusted_stderr: Optional[str] = ""
    prior_successful_steps: Optional[List[Dict[str, Any]]] = Field(default_factory=list)

class DiagnosisRequest(BaseModel):
    task_id: str
    symptom: str
    host_context: HostContext
    untrusted_logs: str

class DiagnosisEvidence(BaseModel):
    source: str
    detail: str

class DiagnosisResponse(BaseModel):
    identified_problem: str
    root_cause: str
    confidence: float = Field(..., ge=0.0, le=1.0)
    evidence: Optional[List[DiagnosisEvidence]] = Field(default_factory=list)
    remediation_steps: List[StepPlan] = Field(default_factory=list)
    provenance: Optional[ProvenanceMetadata] = None
