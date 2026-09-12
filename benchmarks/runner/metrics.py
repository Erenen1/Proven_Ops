import math
from typing import List
from benchmarks.schemas.report import ScenarioExecutionResult, BenchmarkMetrics

def calculate_metrics(results: List[ScenarioExecutionResult]) -> BenchmarkMetrics:
    if not results:
        return BenchmarkMetrics(
            task_success_rate=0.0,
            diagnosis_accuracy=0.0,
            recovery_rate=0.0,
            unsafe_action_rate=0.0,
            false_success_rate=0.0,
            human_intervention_rate=0.0,
            replan_rate=0.0,
            rollback_success_rate=0.0,
            timeout_rate=0.0,
            average_tool_calls=0.0,
            median_tool_calls=0.0,
            average_completion_time_seconds=0.0,
            median_completion_time_seconds=0.0
        )

    total = len(results)
    success_count = sum(1 for r in results if r.task_success)
    diag_count = sum(1 for r in results if r.diagnosis_accurate)
    recovery_count = sum(1 for r in results if r.recovery_succeeded)
    unsafe_count = sum(1 for r in results if r.unsafe_action_detected)
    false_success_count = sum(1 for r in results if r.false_success)
    human_count = sum(1 for r in results if r.human_approval_required)
    replan_count = sum(1 for r in results if r.replans_count > 0)
    timeout_count = sum(1 for r in results if r.terminal_state == "TIMEOUT")

    rollback_attempts = sum(1 for r in results if r.terminal_state in ["ROLLED_BACK", "ROLLBACK_FAILED"])
    rollback_success = sum(1 for r in results if r.terminal_state == "ROLLED_BACK")
    rollback_rate = (rollback_success / rollback_attempts) if rollback_attempts > 0 else 1.0

    tool_calls = [r.tool_calls_count for r in results]
    durations = [r.duration_seconds for r in results]

    avg_tools = sum(tool_calls) / total
    avg_duration = sum(durations) / total

    def median(vals: List[float]) -> float:
        if not vals:
            return 0.0
        s = sorted(vals)
        mid = len(s) // 2
        if len(s) % 2 == 0:
            return (s[mid - 1] + s[mid]) / 2.0
        return float(s[mid])

    return BenchmarkMetrics(
        task_success_rate=round(success_count / total * 100.0, 2),
        diagnosis_accuracy=round(diag_count / total * 100.0, 2),
        recovery_rate=round(recovery_count / total * 100.0, 2),
        unsafe_action_rate=round(unsafe_count / total * 100.0, 2),
        false_success_rate=round(false_success_count / total * 100.0, 2),
        human_intervention_rate=round(human_count / total * 100.0, 2),
        replan_rate=round(replan_count / total * 100.0, 2),
        rollback_success_rate=round(rollback_rate * 100.0, 2),
        timeout_rate=round(timeout_count / total * 100.0, 2),
        average_tool_calls=round(avg_tools, 2),
        median_tool_calls=round(median(tool_calls), 2),
        average_completion_time_seconds=round(avg_duration, 2),
        median_completion_time_seconds=round(median(durations), 2)
    )
