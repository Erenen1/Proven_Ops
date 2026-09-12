#!/usr/bin/env python3
import argparse
import sys
import os

# Ensure repo root is in python path
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "..")))

from benchmarks.runner.runner import BenchmarkRunner

def main():
    parser = argparse.ArgumentParser(description="OpsPilot Benchmark Lab CLI")
    parser.add_argument("--model", type=str, default="qwen2.5:3b", help="Target LLM model name")
    parser.add_argument("--iterations", type=int, default=1, help="Number of benchmark iterations")
    parser.add_argument("--seed", type=int, default=42, help="Randomization seed")
    parser.add_argument("--scenarios-dir", type=str, default="benchmarks/scenarios", help="Scenarios root directory")
    parser.add_argument("--results-dir", type=str, default="benchmarks/results", help="Results output directory")
    default_cp = os.environ.get("CONTROL_PLANE_URL", "http://172.21.96.1:8080" if os.name != "nt" else "http://localhost:8080")
    default_ai = os.environ.get("AI_SERVICE_URL", "http://172.21.96.1:8000" if os.name != "nt" else "http://localhost:8000")

    parser.add_argument("--control-plane", type=str, default=default_cp, help="Control Plane HTTP URL")
    parser.add_argument("--ai-service", type=str, default=default_ai, help="AI Service HTTP URL")

    args = parser.parse_args()

    runner = BenchmarkRunner(
        model=args.model,
        iterations=args.iterations,
        seed=args.seed,
        scenarios_dir=args.scenarios_dir,
        results_dir=args.results_dir,
        control_plane_url=args.control_plane,
        ai_service_url=args.ai_service
    )

    try:
        summary = runner.run()
        if summary.metrics.task_success_rate < 50.0:
            print(f"[NOTE] Benchmark completed with task success rate: {summary.metrics.task_success_rate}%")
        sys.exit(0)
    except Exception as e:
        print(f"\n[ERROR] Benchmark execution failed: {e}")
        sys.exit(1)

if __name__ == "__main__":
    main()
