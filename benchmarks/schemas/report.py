from typing import List, Dict, Any, Optional
from pydantic import BaseModel, Field

class ScenarioExecutionResult(BaseModel):
    scenario_id: str
    category: str
    iteration: int
    user_intent: str
    expected_root_cause: str
    detected_root_cause: Optional[str] = None
    terminal_state: str
    expected_terminal_states: List[str]
    task_success: bool
    diagnosis_accurate: bool
    recovery_succeeded: bool
    unsafe_action_detected: bool
    false_success: bool
    human_approval_required: bool
    tool_calls_count: int
    replans_count: int
    duration_seconds: float
    error_message: Optional[str] = None
    action_trace: List[str] = Field(default_factory=list)
    independent_verification_passed: bool
    synthetic_ai_failure_test: bool = False

class BenchmarkMetrics(BaseModel):
    task_success_rate: float
    diagnosis_accuracy: float
    recovery_rate: float
    unsafe_action_rate: float
    false_success_rate: float
    human_intervention_rate: float
    replan_rate: float
    rollback_success_rate: float
    timeout_rate: float
    average_tool_calls: float
    median_tool_calls: float
    average_completion_time_seconds: float
    median_completion_time_seconds: float

class BenchmarkSummary(BaseModel):
    run_id: str
    timestamp: str
    environment: str
    model: str
    iterations: int
    total_scenarios: int
    passed_scenarios: int
    failed_scenarios: int
    metrics: BenchmarkMetrics
    results: List[ScenarioExecutionResult]
