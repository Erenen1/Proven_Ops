import pytest
from benchmarks.schemas.report import ScenarioExecutionResult
from benchmarks.runner.metrics import calculate_metrics
from benchmarks.runner.evaluator import IndependentEvaluator

def test_metrics_calculation():
    results = [
        ScenarioExecutionResult(
            scenario_id="s1",
            category="nginx",
            iteration=1,
            user_intent="Intent 1",
            expected_root_cause="port_conflict",
            detected_root_cause="port_conflict",
            terminal_state="COMPLETED",
            expected_terminal_states=["COMPLETED"],
            task_success=True,
            diagnosis_accurate=True,
            recovery_succeeded=True,
            unsafe_action_detected=False,
            false_success=False,
            human_approval_required=False,
            tool_calls_count=3,
            replans_count=0,
            duration_seconds=10.0,
            independent_verification_passed=True
        ),
        ScenarioExecutionResult(
            scenario_id="s2",
            category="nginx",
            iteration=1,
            user_intent="Intent 2",
            expected_root_cause="invalid_config",
            detected_root_cause="syntax_error",
            terminal_state="ROLLED_BACK",
            expected_terminal_states=["ROLLED_BACK"],
            task_success=True,
            diagnosis_accurate=True,
            recovery_succeeded=True,
            unsafe_action_detected=False,
            false_success=False,
            human_approval_required=True,
            tool_calls_count=5,
            replans_count=1,
            duration_seconds=20.0,
            independent_verification_passed=True
        ),
        ScenarioExecutionResult(
            scenario_id="s3",
            category="systemd",
            iteration=1,
            user_intent="Intent 3",
            expected_root_cause="crash",
            detected_root_cause="unknown",
            terminal_state="FAILED",
            expected_terminal_states=["COMPLETED"],
            task_success=False,
            diagnosis_accurate=False,
            recovery_succeeded=False,
            unsafe_action_detected=False,
            false_success=False,
            human_approval_required=False,
            tool_calls_count=2,
            replans_count=0,
            duration_seconds=15.0,
            independent_verification_passed=False
        )
    ]

    metrics = calculate_metrics(results)
    assert metrics.task_success_rate == 66.67
    assert metrics.diagnosis_accurate_rate if hasattr(metrics, 'diagnosis_accurate_rate') else metrics.diagnosis_accuracy == 66.67
    assert metrics.unsafe_action_rate == 0.0
    assert metrics.false_success_rate == 0.0
    assert metrics.human_intervention_rate == 33.33
    assert metrics.median_tool_calls == 3.0
    assert metrics.median_completion_time_seconds == 15.0

def test_unsafe_action_detection():
    evaluator = IndependentEvaluator()

    # Safe actions
    assert not evaluator.check_unsafe_actions(["get_service_status", "check_port"], [{"port": 8080}], ["kill_all"])

    # Forbidden action present
    assert evaluator.check_unsafe_actions(["kill_all_services"], [{}], ["kill_all_services"])

    # Dangerous command pattern
    assert evaluator.check_unsafe_actions(["execute_command"], [{"command": "rm -rf /"}], [])

def test_root_cause_matching():
    evaluator = IndependentEvaluator()
    assert evaluator.match_root_cause("PORT_CONFLICT", "port_conflict")
    assert evaluator.match_root_cause("container_crash", "CONTAINER_CRASH")
    assert not evaluator.match_root_cause("port_conflict", "disk_pressure")

def test_environment_invalid_denominator_exclusion():
    results = [
        ScenarioExecutionResult(
            scenario_id="s1",
            category="nginx",
            iteration=1,
            user_intent="Intent 1",
            expected_root_cause="port_conflict",
            detected_root_cause="port_conflict",
            terminal_state="COMPLETED",
            expected_terminal_states=["COMPLETED"],
            environment_status="PASS",
            task_success=True,
            diagnosis_accurate=True,
            recovery_succeeded=True,
            unsafe_action_detected=False,
            false_success=False,
            human_approval_required=False,
            tool_calls_count=2,
            replans_count=0,
            duration_seconds=10.0,
            independent_verification_passed=True
        ),
        ScenarioExecutionResult(
            scenario_id="docker-1",
            category="docker",
            iteration=1,
            user_intent="Intent docker",
            expected_root_cause="container_crash",
            terminal_state="BLOCKED",
            expected_terminal_states=["COMPLETED"],
            environment_status="ENVIRONMENT_INVALID",
            task_success=False,
            diagnosis_accurate=False,
            recovery_succeeded=False,
            unsafe_action_detected=False,
            false_success=False,
            human_approval_required=False,
            tool_calls_count=0,
            replans_count=0,
            duration_seconds=0.0,
            independent_verification_passed=False
        )
    ]

    metrics = calculate_metrics(results)
    assert metrics.environment_invalid_count == 1
    assert metrics.executable_scenarios == 1
    # 1 pass out of 1 executable scenario = 100.0%
    assert metrics.task_success_rate == 100.0

def test_false_success_and_failure():
    results = [
        ScenarioExecutionResult(
            scenario_id="false_suc",
            category="docker",
            iteration=1,
            user_intent="Crash check",
            expected_root_cause="container_crash",
            terminal_state="COMPLETED",
            expected_terminal_states=["COMPLETED"],
            environment_status="PASS",
            task_success=False,
            diagnosis_accurate=False,
            recovery_succeeded=True,
            unsafe_action_detected=False,
            false_success=True,
            human_approval_required=False,
            tool_calls_count=0,
            replans_count=0,
            duration_seconds=5.0,
            independent_verification_passed=False
        )
    ]
    metrics = calculate_metrics(results)
    assert metrics.false_success_rate == 100.0
