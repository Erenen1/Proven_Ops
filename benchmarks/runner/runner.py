import os
import sys
import glob
import time
import random
import yaml
from datetime import datetime
from typing import List, Dict, Any, Tuple

from benchmarks.schemas.scenario import ScenarioMetadata
from benchmarks.schemas.report import ScenarioExecutionResult, BenchmarkSummary
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

        print("\n" + "=" * 70)
        print("  OpsPilot Benchmark Lab — Real Fault Injection Evaluation")
        print(f"  Target Model: {self.model} | Iterations: {self.iterations} | Seed: {self.seed}")
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
        print(f"\n[2/4] Discovered {len(scenario_pairs)} benchmark scenarios.\n")

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
                badge = "PASS" if res.task_success else "FAIL"
                print(f"       -> Status: {res.terminal_state} | Evaluator: {badge} | Duration: {res.duration_seconds}s\n", flush=True)

        # 4. Metrics & Report Generation
        print("[4/4] Calculating benchmark metrics and generating report...")
        metrics = calculate_metrics(results)
        passed_count = sum(1 for r in results if r.task_success)
        failed_count = len(results) - passed_count

        summary = BenchmarkSummary(
            run_id=run_id,
            timestamp=datetime.now().isoformat(),
            environment="Ubuntu 24.04 LTS under WSL2",
            model=self.model,
            iterations=self.iterations,
            total_scenarios=len(results),
            passed_scenarios=passed_count,
            failed_scenarios=failed_count,
            metrics=metrics,
            results=results
        )

        output_path = os.path.join(self.results_dir, run_id)
        BenchmarkReporter.save_run(output_path, summary)

        print("\n" + "=" * 70)
        print(f"  Benchmark Run Complete: {output_path}")
        print(f"  Task Success Rate: {metrics.task_success_rate}%")
        print(f"  Diagnosis Accuracy: {metrics.diagnosis_accuracy}%")
        print(f"  Unsafe Action Rate: {metrics.unsafe_action_rate}%")
        print(f"  False Success Rate: {metrics.false_success_rate}%")
        print("=" * 70 + "\n")

        return summary

    def execute_single_scenario(self, sc_dir: str, meta: ScenarioMetadata, iteration: int) -> ScenarioExecutionResult:
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
            elif meta.expected_root_cause and ("crash" in meta.expected_root_cause or "loop" in meta.expected_root_cause or "pressure" in meta.expected_root_cause):
                # Diagnostic evaluation task
                diag_res = self.client.diagnose_issue(meta.user_intent, f"Simulated observation for {meta.id}")
                detected_root_cause = diag_res.get("root_cause", "")
                terminal_state = "COMPLETED"
            else:
                # Full orchestrator task lifecycle
                task_id = self.client.create_task(meta.user_intent)
                terminal_state, task_data, approval_handled = self.client.poll_task(task_id, timeout_sec=meta.timeout_seconds)
                human_approval_required = approval_handled

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

            # D. INDEPENDENT EVALUATION (Evaluated before cleanup removes scenario state)
            independent_passed, verify_msg = self.evaluator.evaluate_host_condition(verify_sh)
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
        unsafe_detected = self.evaluator.check_unsafe_actions(action_trace, arguments_trace, meta.forbidden_actions)

        # Diagnosis accuracy
        diag_accurate = True
        if meta.expected_root_cause and detected_root_cause:
            diag_accurate = self.evaluator.match_root_cause(meta.expected_root_cause, detected_root_cause)

        # Task success
        state_matched = (terminal_state in meta.expected_terminal_state) if meta.expected_terminal_state else (terminal_state == "COMPLETED")
        task_success = state_matched and independent_passed and not unsafe_detected

        # False success detection: agent claimed COMPLETED but independent verification failed
        false_success = (terminal_state == "COMPLETED" and not independent_passed)

        recovery_succeeded = (terminal_state in ["COMPLETED", "ROLLED_BACK"])

        return ScenarioExecutionResult(
            scenario_id=meta.id,
            category=meta.category,
            iteration=iteration,
            user_intent=meta.user_intent,
            expected_root_cause=meta.expected_root_cause,
            detected_root_cause=detected_root_cause,
            terminal_state=terminal_state,
            expected_terminal_states=meta.expected_terminal_state,
            task_success=task_success,
            diagnosis_accurate=diag_accurate,
            recovery_succeeded=recovery_succeeded,
            unsafe_action_detected=unsafe_detected,
            false_success=false_success,
            human_approval_required=human_approval_required,
            tool_calls_count=tool_calls_count,
            replans_count=replans_count,
            duration_seconds=duration,
            error_message=error_message,
            action_trace=action_trace,
            independent_verification_passed=independent_passed,
            synthetic_ai_failure_test=meta.synthetic_ai_failure_test
        )
