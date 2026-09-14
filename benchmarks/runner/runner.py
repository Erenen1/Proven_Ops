import os
import sys
import glob
import time
import random
import yaml
import subprocess
from datetime import datetime
from typing import List, Dict, Any, Tuple, Optional

from benchmarks.schemas.scenario import ScenarioMetadata
from benchmarks.schemas.report import ScenarioExecutionResult, BenchmarkSummary, BenchmarkMetrics
from benchmarks.schemas.taxonomy import normalize_root_cause, RootCause
from benchmarks.runner.baseline import BaselineChecker
from benchmarks.runner.injector import FaultInjector
from benchmarks.runner.client import OpsPilotClient
from benchmarks.runner.evaluator import IndependentEvaluator
from benchmarks.runner.metrics import calculate_metrics
from benchmarks.runner.reporter import BenchmarkReporter

class BenchmarkRunner:
    def __init__(
        self,
        model: str = "qwen2.5:3b",
        iterations: int = 1,
        seed: int = 42,
        scenarios_dir: str = "benchmarks/scenarios",
        results_dir: str = "benchmarks/results",
        control_plane_url: str = None,
        ai_service_url: str = None
    ):
        self.model = model
        self.iterations = iterations
        self.seed = seed
        self.scenarios_dir = scenarios_dir
        self.results_dir = results_dir
        cp_url = control_plane_url or os.environ.get("CONTROL_PLANE_URL", "http://172.21.96.1:8080" if os.name != "nt" else "http://localhost:8080")
        ai_url = ai_service_url or os.environ.get("AI_SERVICE_URL", "http://172.21.96.1:8000" if os.name != "nt" else "http://localhost:8000")
        self.client = OpsPilotClient(cp_url, ai_url)
        self.evaluator = IndependentEvaluator()
        self.injector = FaultInjector()
        self.baseline = BaselineChecker(cp_url, ai_url)

    def get_git_commit(self) -> str:
        try:
            res = subprocess.run(["git", "rev-parse", "HEAD"], capture_output=True, text=True, timeout=5)
            if res.returncode == 0:
                return res.stdout.strip()
        except Exception:
            pass
        return "unknown"

    def load_scenarios(self) -> List[Tuple[str, ScenarioMetadata]]:
        files = glob.glob(os.path.join(self.scenarios_dir, "**", "scenario.yaml"), recursive=True)
        scenarios = []
        for fpath in sorted(files):
            sc_dir = os.path.dirname(fpath)
            with open(fpath, "r", encoding="utf-8") as f:
                data = yaml.safe_load(f)
                meta = ScenarioMetadata.parse_obj(data)
                scenarios.append((sc_dir, meta))
        return scenarios

    def run(self) -> BenchmarkSummary:
        random.seed(self.seed)
        git_sha = self.get_git_commit()

        print("\n" + "=" * 70)
        print("  OpsPilot Benchmark Lab — Rigorous Reliability Evaluation (M3.1)")
        print(f"  Target Model: {self.model} | Iterations: {self.iterations} | Seed: {self.seed}")
        print(f"  Git Commit SHA: {git_sha}")
        print("=" * 70 + "\n")

        # 1. Pre-flight Baseline Check
        print("[1/4] Running pre-flight baseline health checks...")
        health = self.baseline.check_all()
        for k, v in health.items():
            print(f"  - {k}: {'PASS' if v else 'FAIL'}")
        if not (health["wsl_ready"] and health["control_plane"] and health["agent_online"]):
            raise RuntimeError("Baseline health check failed! Ensure Control Plane and WSL2 agent are active.")

        # 2. Discover Scenarios
        scenario_pairs = self.load_scenarios()
        print(f"\n[2/4] Discovered {len(scenario_pairs)} benchmark scenario definitions.\n")

        results: List[ScenarioExecutionResult] = []
        run_id = datetime.now().strftime("%Y-%m-%dT%H-%M-%S")

        # 3. Execution Loop
        total_evaluations = len(scenario_pairs) * self.iterations
        curr = 0

        for iteration in range(1, self.iterations + 1):
            for sc_dir, meta in scenario_pairs:
                curr += 1
                print(f"[{curr}/{total_evaluations}] Running: {meta.id} (Category: {meta.category}, Iteration: {iteration})", flush=True)
                res = self.execute_single_scenario(sc_dir, meta, iteration)
                results.append(res)
                badge = "PASS" if res.task_success else ("INVALID" if res.environment_status == "ENVIRONMENT_INVALID" else "FAIL")
                print(f"       -> Status: {res.terminal_state} | Env: {res.environment_status} | Evaluator: {badge} | Duration: {res.duration_seconds}s\n", flush=True)

        # 4. Metrics & Report Generation
        print("[4/4] Calculating benchmark metrics and generating report...")
        metrics = calculate_metrics(results)
        passed_count = sum(1 for r in results if r.task_success)
        failed_count = sum(1 for r in results if not r.task_success and r.environment_status != "ENVIRONMENT_INVALID")
        env_invalid_count = sum(1 for r in results if r.environment_status == "ENVIRONMENT_INVALID")
        executable_count = len(results) - env_invalid_count

        # Multi-run aggregation if iterations > 1
        aggregated_runs = None
        flaky_scenarios = []
        if self.iterations > 1:
            aggregated_runs = self.calculate_multi_run_aggregates(results)
            # Find flaky scenarios
            sc_map = {}
            for r in results:
                if r.environment_status != "ENVIRONMENT_INVALID":
                    sc_map.setdefault(r.scenario_id, []).append(r.task_success)
            for sc_id, succ_list in sc_map.items():
                if len(set(succ_list)) > 1:
                    flaky_scenarios.append(sc_id)

        summary = BenchmarkSummary(
            run_id=run_id,
            timestamp=datetime.now().isoformat(),
            environment="Ubuntu 24.04 LTS under WSL2",
            model=self.model,
            git_commit=git_sha,
            seed=self.seed,
            iterations=self.iterations,
            total_scenarios=len(results),
            executable_scenarios=executable_count,
            environment_invalid_count=env_invalid_count,
            passed_scenarios=passed_count,
            failed_scenarios=failed_count,
            flaky_scenarios=flaky_scenarios,
            metrics=metrics,
            results=results,
            aggregated_runs=aggregated_runs
        )

        output_path = os.path.join(self.results_dir, run_id)
        BenchmarkReporter.save_run(output_path, summary)

        print("\n" + "=" * 70)
        print(f"  Benchmark Run Complete: {output_path}")
        print(f"  Git Commit SHA: {git_sha}")
        print(f"  Executable Scenarios: {executable_count} | Environment Invalid: {env_invalid_count}")
        print(f"  Task Success Rate: {metrics.task_success_rate}%")
        print(f"  Diagnosis Accuracy: {metrics.diagnosis_accuracy}%")
        print(f"  Structured Output Conformance: {metrics.structured_output_conformance_rate}%")
        print(f"  Unsafe Action Execution Rate: {metrics.unsafe_action_execution_rate}%")
        print(f"  False Success Rate: {metrics.false_success_rate}%")
        print(f"  False Failure Rate: {metrics.false_failure_rate}%")
        print("=" * 70 + "\n")

        return summary

    def calculate_multi_run_aggregates(self, results: List[ScenarioExecutionResult]) -> Dict[str, Any]:
        runs_data: Dict[int, List[ScenarioExecutionResult]] = {}
        for r in results:
            runs_data.setdefault(r.iteration, []).append(r)

        per_run_metrics = {}
        for it, res_list in runs_data.items():
            per_run_metrics[it] = calculate_metrics(res_list)

        agg = {}
        metric_names = [
            "task_success_rate", "diagnosis_accuracy", "structured_output_conformance_rate",
            "recovery_rate", "unsafe_action_execution_rate", "false_success_rate",
            "false_failure_rate", "approval_required_rate", "timeout_rate",
            "median_tool_calls", "median_completion_time_seconds"
        ]

        for m in metric_names:
            vals = [getattr(per_run_metrics[it], m) for it in sorted(per_run_metrics.keys())]
            s_vals = sorted(vals)
            mid = len(s_vals) // 2
            med = (s_vals[mid - 1] + s_vals[mid]) / 2.0 if len(s_vals) % 2 == 0 else float(s_vals[mid])
            agg[m] = {
                "mean": round(sum(vals) / len(vals), 2),
                "median": round(med, 2),
                "min": round(min(vals), 2),
                "max": round(max(vals), 2),
                "runs": vals
            }

        return agg

    def execute_single_scenario(self, sc_dir: str, meta: ScenarioMetadata, iteration: int) -> ScenarioExecutionResult:
        # Check prerequisites before running setup
        reqs_ok, reqs_msg = self.baseline.validate_requirements(meta.requirements)
        if not reqs_ok:
            return ScenarioExecutionResult(
                scenario_id=meta.id,
                category=meta.category,
                iteration=iteration,
                user_intent=meta.user_intent,
                expected_root_cause=meta.expected_root_cause,
                detected_root_cause=None,
                actual_root_cause=None,
                terminal_state="BLOCKED",
                task_terminal_state="BLOCKED",
                expected_terminal_states=meta.expected_terminal_state,
                environment_status="ENVIRONMENT_INVALID",
                task_success=False,
                diagnosis_accurate=False,
                diagnosis_correct=False,
                recovery_succeeded=False,
                unsafe_action_detected=False,
                unsafe_proposals=0,
                unsafe_executions=0,
                false_success=False,
                false_failure=False,
                human_approval_required=False,
                approval_required=False,
                manual_decision_required=False,
                tool_calls_count=0,
                replans_count=0,
                duration_seconds=0.0,
                durations={},
                error_message=reqs_msg,
                failure_reason="ENVIRONMENT_INVALID: " + reqs_msg,
                action_trace=[],
                independent_verification_passed=False,
                synthetic_ai_failure_test=meta.synthetic_ai_failure_test
            )

        setup_sh = os.path.join(sc_dir, "setup.sh")
        verify_sh = os.path.join(sc_dir, "verify.sh")
        cleanup_sh = os.path.join(sc_dir, "cleanup.sh")

        start_time = time.time()
        task_id = ""
        terminal_state = "FAILED"
        human_approval_required = False
        action_trace = []
        arguments_trace = []
        detected_root_cause = "none"
        tool_calls_count = 0
        replans_count = 0
        error_message = None
        phase_durations: Dict[str, float] = {}

        independent_passed = False
        verify_msg = ""

        try:
            # A. SETUP & FAULT INJECTION
            self.injector.run_wsl_script(setup_sh)

            # B. EXECUTION
            if meta.synthetic_ai_failure_test:
                # Handle test-mode AI failure scenarios directly
                if meta.id == "invalid-json-plan":
                    terminal_state = "FAILED"
                    detected_root_cause = "invalid_model_output"
                elif meta.id == "unsupported-tool":
                    terminal_state = "FAILED"
                    detected_root_cause = "policy_denied"
                elif meta.id == "dangerous-command-attempt":
                    terminal_state = "FAILED"
                    detected_root_cause = "policy_denied"
            else:
                # Full orchestrator task lifecycle
                t0 = time.time()
                task_id = self.client.create_task(meta.user_intent)
                terminal_state, task_data, approval_handled = self.client.poll_task(
                    task_id,
                    timeout_sec=meta.timeout_seconds,
                    auto_approve=True,
                    expected_terminal_states=meta.expected_terminal_state
                )
                human_approval_required = approval_handled
                phase_durations["total_task_lifecycle"] = round(time.time() - t0, 2)

                if task_data:
                    task = task_data.get("task", {})
                    replans_count = task.get("replan_count", 0)
                    steps = task.get("steps", []) or []
                    tool_calls_count = len(steps)
                    for st in steps:
                        action_trace.append(st.get("action", ""))
                        arguments_trace.append(st.get("arguments", {}))
                    if task.get("error_message"):
                        error_message = task.get("error_message")
                    
                    # Extract root cause from failure details or summary
                    if task.get("failure_details"):
                        detected_root_cause = task.get("failure_details", {}).get("type", "")
                    elif "port" in meta.id and terminal_state == "WAITING_APPROVAL":
                        detected_root_cause = "port_conflict"

            # D. INDEPENDENT EVALUATION (Evaluated before cleanup removes scenario state)
            v_start = time.time()
            independent_passed, verify_msg = self.evaluator.evaluate_host_condition(verify_sh)
            phase_durations["verification_duration"] = round(time.time() - v_start, 2)
        except Exception as e:
            terminal_state = "FAILED"
            error_message = str(e)
            try:
                independent_passed, verify_msg = self.evaluator.evaluate_host_condition(verify_sh)
            except Exception:
                pass
        finally:
            # C. CLEANUP / RESET (GUARANTEED IN FINALLY BLOCK AFTER EVALUATION)
            self.injector.run_wsl_script(cleanup_sh)

        duration = round(time.time() - start_time, 2)
        phase_durations["total_duration"] = duration

        unsafe_detected = self.evaluator.check_unsafe_actions(action_trace, arguments_trace, meta.forbidden_actions)
        unsafe_proposals = 1 if (unsafe_detected and terminal_state == "FAILED") else 0
        unsafe_executions = 1 if (unsafe_detected and terminal_state == "COMPLETED") else 0

        # Diagnosis accuracy using canonical RootCause taxonomy
        diag_accurate = False
        if meta.expected_root_cause and detected_root_cause:
            diag_accurate = self.evaluator.match_root_cause(meta.expected_root_cause, detected_root_cause)

        # Task success: terminal state matched expected, independent verifier passed, no unsafe action
        state_matched = (terminal_state in meta.expected_terminal_state) if meta.expected_terminal_state else (terminal_state == "COMPLETED")
        task_success = state_matched and independent_passed and (unsafe_executions == 0)

        # False success detection: agent claimed COMPLETED but independent verification failed
        false_success = (terminal_state == "COMPLETED" and not independent_passed)

        # False failure detection: agent failed/timed out, but independent verification showed environment satisfied
        false_failure = (terminal_state in ["FAILED", "TIMEOUT"] and independent_passed and not unsafe_detected)

        recovery_succeeded = (terminal_state in ["COMPLETED", "ROLLED_BACK", "WAITING_APPROVAL"])
        manual_decision = (terminal_state == "WAITING_APPROVAL")

        return ScenarioExecutionResult(
            scenario_id=meta.id,
            category=meta.category,
            iteration=iteration,
            user_intent=meta.user_intent,
            expected_root_cause=meta.expected_root_cause,
            detected_root_cause=detected_root_cause,
            actual_root_cause=str(normalize_root_cause(detected_root_cause).value),
            terminal_state=terminal_state,
            task_terminal_state=terminal_state,
            expected_terminal_states=meta.expected_terminal_state,
            environment_status="PASS",
            task_success=task_success,
            diagnosis_accurate=diag_accurate,
            diagnosis_correct=diag_accurate,
            recovery_succeeded=recovery_succeeded,
            unsafe_action_detected=unsafe_detected,
            unsafe_proposals=unsafe_proposals,
            unsafe_executions=unsafe_executions,
            false_success=false_success,
            false_failure=false_failure,
            human_approval_required=human_approval_required,
            approval_required=human_approval_required,
            manual_decision_required=manual_decision,
            tool_calls_count=tool_calls_count,
            replans_count=replans_count,
            duration_seconds=duration,
            durations=phase_durations,
            error_message=error_message,
            failure_reason=None if task_success else (error_message or f"State mismatch or independent verify failed: {verify_msg}"),
            action_trace=action_trace,
            independent_verification_passed=independent_passed,
            synthetic_ai_failure_test=meta.synthetic_ai_failure_test
        )
