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

class PlanResponse(BaseModel):
    goal: str
    reasoning: str
    steps: List[StepPlan]
    overall_verification: Optional[List[VerificationRequirement]] = Field(default_factory=list)

class ReplanRequest(BaseModel):
    task_id: str
    intent: str
    host_context: HostContext
    failed_step_id: str
    failed_action: str
    exit_code: int
    untrusted_stdout: str
    untrusted_stderr: str
    prior_successful_steps: List[StepPlan] = Field(default_factory=list)

class DiagnosisRequest(BaseModel):
    task_id: str
    symptom: str
    host_context: HostContext
    untrusted_logs: str

class DiagnosisResponse(BaseModel):
    identified_problem: str
    root_cause: str
    confidence: float = Field(..., ge=0.0, le=1.0)
    remediation_steps: List[StepPlan] = Field(default_factory=list)
