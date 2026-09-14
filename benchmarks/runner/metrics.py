import math
from typing import List
from benchmarks.schemas.report import ScenarioExecutionResult, BenchmarkMetrics

def calculate_metrics(results: List[ScenarioExecutionResult]) -> BenchmarkMetrics:
    if not results:
        return BenchmarkMetrics(
            task_success_rate=0.0,
            diagnosis_accuracy=0.0,
            structured_output_conformance_rate=100.0,
            recovery_rate=0.0,
            unsafe_action_rate=0.0,
            unsafe_action_proposal_rate=0.0,
            unsafe_action_execution_rate=0.0,
            false_success_rate=0.0,
            false_failure_rate=0.0,
            human_intervention_rate=0.0,
            approval_required_rate=0.0,
            manual_decision_required_rate=0.0,
            replan_rate=0.0,
            rollback_success_rate=0.0,
            timeout_rate=0.0,
            flakiness_rate=0.0,
            environment_invalid_count=0,
            executable_scenarios=0,
            average_tool_calls=0.0,
            median_tool_calls=0.0,
            average_completion_time_seconds=0.0,
            median_completion_time_seconds=0.0,
            phase_latencies={}
        )

    total_all = len(results)
    env_invalid_results = [r for r in results if r.environment_status == "ENVIRONMENT_INVALID"]
    executable_results = [r for r in results if r.environment_status != "ENVIRONMENT_INVALID"]
    executable_count = len(executable_results)
    env_invalid_count = len(env_invalid_results)

    denom = executable_count if executable_count > 0 else total_all

    success_count = sum(1 for r in executable_results if r.task_success)
    diag_count = sum(1 for r in executable_results if r.diagnosis_accurate or r.diagnosis_correct)
    recovery_count = sum(1 for r in executable_results if r.recovery_succeeded)
    unsafe_proposal_count = sum(1 for r in executable_results if r.unsafe_proposals > 0 or (r.unsafe_action_detected and r.terminal_state == "FAILED"))
    unsafe_execution_count = sum(1 for r in executable_results if r.unsafe_executions > 0 or (r.unsafe_action_detected and r.terminal_state == "COMPLETED"))
    false_success_count = sum(1 for r in executable_results if r.false_success)
    false_failure_count = sum(1 for r in executable_results if r.false_failure)
    human_count = sum(1 for r in executable_results if r.human_approval_required or r.approval_required)
    manual_decision_count = sum(1 for r in executable_results if r.manual_decision_required)
    replan_count = sum(1 for r in executable_results if r.replans_count > 0)
    timeout_count = sum(1 for r in executable_results if r.terminal_state == "TIMEOUT")

    rollback_attempts = sum(1 for r in executable_results if r.terminal_state in ["ROLLED_BACK", "ROLLBACK_FAILED"])
    rollback_success = sum(1 for r in executable_results if r.terminal_state == "ROLLED_BACK")
    rollback_rate = (rollback_success / rollback_attempts) if rollback_attempts > 0 else 1.0

    # Structured output conformance: count scenarios without schema validation / 422 errors
    conformance_failures = sum(1 for r in executable_results if r.error_message and ("422" in r.error_message or "schema" in r.error_message.lower()))
    conformance_rate = round(((denom - conformance_failures) / denom) * 100.0, 2)

    tool_calls = [r.tool_calls_count for r in executable_results]
    durations = [r.duration_seconds for r in executable_results]

    avg_tools = sum(tool_calls) / denom if denom > 0 else 0.0
    avg_duration = sum(durations) / denom if denom > 0 else 0.0

    def median(vals: List[float]) -> float:
        if not vals:
            return 0.0
        s = sorted(vals)
        mid = len(s) // 2
        if len(s) % 2 == 0:
            return (s[mid - 1] + s[mid]) / 2.0
        return float(s[mid])

    # Flakiness detection across iterations
    scenario_groups = {}
    for r in executable_results:
        scenario_groups.setdefault(r.scenario_id, []).append(r.task_success)
    flaky_count = sum(1 for sc_id, successes in scenario_groups.items() if len(successes) > 1 and len(set(successes)) > 1)
    unique_scenarios = len(scenario_groups)
    flakiness_rate = round((flaky_count / unique_scenarios * 100.0), 2) if unique_scenarios > 0 else 0.0

    return BenchmarkMetrics(
        task_success_rate=round(success_count / denom * 100.0, 2),
        diagnosis_accuracy=round(diag_count / denom * 100.0, 2),
        structured_output_conformance_rate=conformance_rate,
        recovery_rate=round(recovery_count / denom * 100.0, 2),
        unsafe_action_rate=round(unsafe_execution_count / denom * 100.0, 2),
        unsafe_action_proposal_rate=round(unsafe_proposal_count / denom * 100.0, 2),
        unsafe_action_execution_rate=round(unsafe_execution_count / denom * 100.0, 2),
        false_success_rate=round(false_success_count / denom * 100.0, 2),
        false_failure_rate=round(false_failure_count / denom * 100.0, 2),
        human_intervention_rate=round(human_count / denom * 100.0, 2),
        approval_required_rate=round(human_count / denom * 100.0, 2),
        manual_decision_required_rate=round(manual_decision_count / denom * 100.0, 2),
        replan_rate=round(replan_count / denom * 100.0, 2),
        rollback_success_rate=round(rollback_rate * 100.0, 2),
        timeout_rate=round(timeout_count / denom * 100.0, 2),
        flakiness_rate=flakiness_rate,
        environment_invalid_count=env_invalid_count,
        executable_scenarios=executable_count,
        average_tool_calls=round(avg_tools, 2),
        median_tool_calls=round(median(tool_calls), 2),
        average_completion_time_seconds=round(avg_duration, 2),
        median_completion_time_seconds=round(median(durations), 2),
        phase_latencies={
            "median_total_duration": round(median(durations), 2),
            "average_total_duration": round(avg_duration, 2)
        }
    )
