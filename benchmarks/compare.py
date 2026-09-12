import sys
import os
import json
import argparse

def compare_runs(run_a_dir: str, run_b_dir: str):
    sum_a_path = os.path.join(run_a_dir, "summary.json")
    sum_b_path = os.path.join(run_b_dir, "summary.json")

    with open(sum_a_path, "r", encoding="utf-8") as f:
        data_a = json.load(f)
    with open(sum_b_path, "r", encoding="utf-8") as f:
        data_b = json.load(f)

    model_a = data_a.get("model", "Run A")
    model_b = data_b.get("model", "Run B")
    m_a = data_a.get("metrics", {})
    m_b = data_b.get("metrics", {})

    print("\n" + "=" * 65)
    print(" OpsPilot Benchmark Comparison Engine")
    print(f" Run A: {model_a} ({data_a.get('run_id')})")
    print(f" Run B: {model_b} ({data_b.get('run_id')})")
    print("=" * 65)

    metrics_list = [
        ("Task Success Rate (%)", "task_success_rate"),
        ("Diagnosis Accuracy (%)", "diagnosis_accuracy"),
        ("Recovery Rate (%)", "recovery_rate"),
        ("Unsafe Action Rate (%)", "unsafe_action_rate"),
        ("False Success Rate (%)", "false_success_rate"),
        ("Human Intervention Rate (%)", "human_intervention_rate"),
        ("Average Tool Calls", "average_tool_calls"),
        ("Median Duration (s)", "median_completion_time_seconds"),
    ]

    header = f"{'Metric':<30} | {model_a:<12} | {model_b:<12} | {'Delta':<10}"
    print(header)
    print("-" * len(header))

    for label, key in metrics_list:
        val_a = m_a.get(key, 0.0)
        val_b = m_b.get(key, 0.0)
        delta = round(val_b - val_a, 2)
        sign = "+" if delta > 0 else ""
        print(f"{label:<30} | {str(val_a):<12} | {str(val_b):<12} | {sign + str(delta):<10}")

    print("=" * 65 + "\n")

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Compare two benchmark runs")
    parser.add_argument("run_a", type=str, help="Path to first run directory")
    parser.add_argument("run_b", type=str, help="Path to second run directory")
    args = parser.parse_args()

    compare_runs(args.run_a, args.run_b)
