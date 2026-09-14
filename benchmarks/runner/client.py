import json
import time
from typing import Dict, Any, Tuple, List
import httpx

class OpsPilotClient:
    def __init__(self, control_plane_url: str = "http://172.21.96.1:8080", ai_service_url: str = "http://localhost:8000"):
        self.cp_url = control_plane_url
        self.ai_url = ai_service_url

    def create_task(self, prompt: str, target_agent: str = "agent-eren") -> str:
        with httpx.Client(timeout=10.0) as client:
            payload = {
                "title": prompt[:50],
                "prompt": prompt,
                "target_agent_ids": [target_agent]
            }
            res = client.post(f"{self.cp_url}/api/v1/tasks", json=payload)
            if res.status_code != 201:
                raise RuntimeError(f"Task creation failed HTTP {res.status_code}: {res.text}")
            return res.json()["id"]

    def poll_task(
        self,
        task_id: str,
        timeout_sec: int = 180,
        auto_approve: bool = True,
        expected_terminal_states: List[str] = None
    ) -> Tuple[str, Dict[str, Any], bool]:
        start = time.time()
        approved_versions = set()
        approval_handled = False
        last_data: Dict[str, Any] = {}
        expected_states = expected_terminal_states or []

        with httpx.Client(timeout=10.0) as client:
            while time.time() - start < timeout_sec:
                res = client.get(f"{self.cp_url}/api/v1/tasks/{task_id}")
                if res.status_code == 200:
                    data = res.json()
                    last_data = data
                    task = data.get("task", {})
                    status = task.get("status", "")
                    plan_version = task.get("plan_version", 1)

                    if status == "WAITING_APPROVAL":
                        # If WAITING_APPROVAL is an accepted terminal outcome for this scenario (e.g. port conflict operator pause)
                        if "WAITING_APPROVAL" in expected_states:
                            # Let it settle for 3 seconds to ensure inspection completed, then return
                            time.sleep(3)
                            res2 = client.get(f"{self.cp_url}/api/v1/tasks/{task_id}")
                            if res2.status_code == 200:
                                last_data = res2.json()
                                status = last_data.get("task", {}).get("status", status)
                            return status, last_data, True

                        if auto_approve and plan_version not in approved_versions:
                            app_res = client.post(
                                f"{self.cp_url}/api/v1/tasks/{task_id}/approve",
                                json={"plan_version": plan_version, "notes": f"Benchmark Operator Auto-Approval v{plan_version}"}
                            )
                            if app_res.status_code == 200:
                                approved_versions.add(plan_version)
                                approval_handled = True

                    if status in ["COMPLETED", "FAILED", "ROLLED_BACK", "ROLLBACK_FAILED", "TIMEOUT", "CANCELLED"]:
                        return status, data, approval_handled

                time.sleep(2)

        return "TIMEOUT", last_data, approval_handled

    def diagnose_issue(self, symptom: str, logs: str) -> Dict[str, Any]:
        with httpx.Client(timeout=60.0) as client:
            payload = {
                "task_id": "diag-bench",
                "symptom": symptom,
                "host_context": {
                    "hostname": "ubuntu-node-01",
                    "os": "linux",
                    "distribution": "ubuntu",
                    "version": "24.04",
                    "architecture": "amd64",
                    "capabilities": ["systemd", "apt", "docker", "network"]
                },
                "untrusted_logs": logs
            }
            res = client.post(f"{self.ai_url}/api/v1/diagnose", json=payload)
            if res.status_code == 200:
                return res.json()
            return {"identified_problem": "error", "root_cause": "diagnosis_failed", "confidence": 0.0}

    def fetch_audit_events(self, task_id: str) -> List[Dict[str, Any]]:
        with httpx.Client(timeout=5.0) as client:
            res = client.get(f"{self.cp_url}/api/v1/audit?limit=100")
            if res.status_code == 200:
                events = res.json()
                return [e for e in events if e.get("task_id") == task_id]
            return []
