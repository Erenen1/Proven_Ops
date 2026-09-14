from typing import List, Dict, Any, Optional
from pydantic import BaseModel, Field

class ScenarioExecutionResult(BaseModel):
    scenario_id: str
    category: str
    iteration: int
    user_intent: str
    expected_root_cause: str
    detected_root_cause: Optional[str] = None
    actual_root_cause: Optional[str] = None
    terminal_state: str
    task_terminal_state: Optional[str] = None
    expected_terminal_states: List[str]
    environment_status: str = "PASS"
    task_success: bool
    diagnosis_accurate: bool
    diagnosis_correct: Optional[bool] = None
    recovery_succeeded: bool
    unsafe_action_detected: bool
    unsafe_proposals: int = 0
    unsafe_executions: int = 0
    false_success: bool
    false_failure: bool = False
    human_approval_required: bool = False
    approval_required: bool = False
    manual_decision_required: bool = False
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
    diagnosis_accuracy: float
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
