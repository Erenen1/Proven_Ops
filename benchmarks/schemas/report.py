from typing import List, Dict, Any, Optional
from pydantic import BaseModel, Field
from .taxonomy import OutcomeClass

class ScenarioExecutionResult(BaseModel):
    scenario_id: str
    category: str
    iteration: int
    user_intent: str
    expected_root_cause: str
    detected_root_cause: Optional[str] = None
    actual_root_cause: Optional[str] = None
    model_root_cause: Optional[str] = None
    evidence_root_cause: Optional[str] = None
    final_root_cause: Optional[str] = None
    raw_model_diagnosis: Optional[Dict[str, Any]] = None
    model_evidence_agreement: bool = False
    unsupported_diagnosis: bool = False
    terminal_state: str
    task_terminal_state: Optional[str] = None
    expected_terminal_states: List[str]
    environment_status: str = "PASS"
    task_success: bool
    scenario_pass: bool = False
    goal_achieved: bool = False
    terminal_state_correct: bool = False
    diagnosis_accurate: bool
    diagnosis_correct: Optional[bool] = None
    safety_pass: bool = True
    outcome_class: OutcomeClass = OutcomeClass.SAFE_FAILURE
    recovery_succeeded: bool
    unsafe_action_detected: bool
    unsafe_proposals: int = 0
    unsafe_executions: int = 0
    false_success: bool
    false_failure: bool = False
    human_approval_required: bool = False
    approval_required: bool = False
    manual_decision_required: bool = False
    provider: str = "ollama"
    model: str = "qwen2.5:3b"
    model_digest: Optional[str] = None
    fallback_used: bool = False
    fallback_reason: Optional[str] = None
    ai_call_count: int = 0
    tainted: bool = False
    tool_calls_count: int = 0
    replans_count: int = 0
    duration_seconds: float
    durations: Dict[str, float] = Field(default_factory=dict)
    error_message: Optional[str] = None
    failure_reason: Optional[str] = None
    action_trace: List[str] = Field(default_factory=list)
    independent_verification_passed: bool
    synthetic_ai_failure_test: bool = False

class BenchmarkMetrics(BaseModel):
    task_success_rate: float
    scenario_pass_rate: float = 0.0
    goal_achievement_rate: float = 0.0
    terminal_state_accuracy: float = 0.0
    diagnosis_accuracy: float
    model_diagnosis_accuracy: float = 0.0
    grounded_diagnosis_accuracy: float = 0.0
    model_evidence_agreement_rate: float = 0.0
    unsupported_diagnosis_rate: float = 0.0
    structured_output_conformance_rate: float = 100.0
    recovery_rate: float
    unsafe_action_rate: float
    unsafe_action_proposal_rate: float = 0.0
    unsafe_action_execution_rate: float = 0.0
    false_success_rate: float
    false_failure_rate: float = 0.0
    human_intervention_rate: float
    approval_required_rate: float = 0.0
    manual_decision_required_rate: float = 0.0
    safe_operator_deferral_rate: float = 0.0
    replan_rate: float
    rollback_success_rate: float
    timeout_rate: float
    flakiness_rate: float = 0.0
    environment_invalid_count: int = 0
    executable_scenarios: int = 0
    average_tool_calls: float
    median_tool_calls: float
    average_completion_time_seconds: float
    median_completion_time_seconds: float
    phase_latencies: Dict[str, float] = Field(default_factory=dict)

class BenchmarkSummary(BaseModel):
    run_id: str
    timestamp: str
    environment: str
    model: str
    git_commit: Optional[str] = None
    seed: int = 42
    iterations: int
    total_scenarios: int
    executable_scenarios: int = 0
    environment_invalid_count: int = 0
    passed_scenarios: int
    failed_scenarios: int
    flaky_scenarios: List[str] = Field(default_factory=list)
    metrics: BenchmarkMetrics
    results: List[ScenarioExecutionResult]
    aggregated_runs: Optional[Dict[str, Any]] = None
    ai_configuration: Optional[Dict[str, Any]] = None
    official_run: bool = False
