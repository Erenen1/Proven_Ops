import os
import subprocess
import time
import httpx
from typing import Dict, Any

class BaselineChecker:
    def __init__(self, control_plane_url: str = None, ai_service_url: str = None):
        self.cp_url = control_plane_url or os.environ.get("CONTROL_PLANE_URL", "http://172.21.96.1:8080" if os.name != "nt" else "http://localhost:8080")
        self.ai_url = ai_service_url or os.environ.get("AI_SERVICE_URL", "http://172.21.96.1:8000" if os.name != "nt" else "http://localhost:8000")

    def check_all(self) -> Dict[str, Any]:
        results = {
            "control_plane": False,
            "ai_service": False,
            "agent_online": False,
            "wsl_ready": False,
        }

        # 1. Check host/WSL2 connectivity
        if os.name != 'nt':
            results["wsl_ready"] = True
        else:
            try:
                res = subprocess.run(["wsl", "-d", "Ubuntu", "-u", "root", "--", "uname", "-r"], capture_output=True, text=True, timeout=5)
                if res.returncode == 0:
                    results["wsl_ready"] = True
            except Exception:
                pass

        # 2. Check Control Plane HTTP
        try:
            with httpx.Client(timeout=3.0) as client:
                res = client.get(f"{self.cp_url}/health")
                if res.status_code == 200:
                    results["control_plane"] = True
        except Exception:
            pass

        # 3. Check Control Plane Agent Registration
        try:
            with httpx.Client(timeout=3.0) as client:
                res = client.get(f"{self.cp_url}/api/v1/agents")
                if res.status_code == 200:
                    agents = res.json()
                    for a in agents:
                        if a.get("status") == "online":
                            results["agent_online"] = True
                            break
        except Exception:
            pass

        # 4. Check AI Service & Ollama
        try:
            with httpx.Client(timeout=5.0) as client:
                res = client.get(f"{self.ai_url}/health")
                if res.status_code == 200:
                    results["ai_service"] = True
        except Exception:
            pass

        return results
