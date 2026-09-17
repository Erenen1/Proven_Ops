import os
import subprocess
import time
import httpx
from typing import Dict, Any, Tuple, List

class BaselineChecker:
    def __init__(self, control_plane_url: str = None, ai_service_url: str = None):
        self.cp_url = control_plane_url or os.environ.get("CONTROL_PLANE_URL", "http://localhost:8080")
        self.ai_url = ai_service_url or os.environ.get("AI_SERVICE_URL", "http://localhost:8000")

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

        # 5. Check Docker Availability
        docker_ok, docker_msg = self.check_docker()
        results["docker_ready"] = docker_ok

        return results

    def check_docker(self) -> Tuple[bool, str]:
        if os.name != 'nt':
            cmd = ["bash", "-c", "docker --version && docker info >/dev/null 2>&1"]
        else:
            cmd = ["wsl", "-d", "Ubuntu", "-u", "root", "--", "bash", "-c", "docker --version && docker info >/dev/null 2>&1"]
        try:
            res = subprocess.run(cmd, capture_output=True, text=True, timeout=6)
            if res.returncode == 0:
                return True, "Docker daemon running and usable"
            return False, res.stderr.strip() or "Docker daemon not reachable"
        except Exception as e:
            return False, str(e)

    def validate_requirements(self, requirements: list) -> Tuple[bool, str]:
        if not requirements:
            return True, "No special requirements"

        for req in requirements:
            req_lower = req.lower().strip()
            if req_lower in ["docker", "docker_daemon_running"]:
                ok, msg = self.check_docker()
                if not ok:
                    return False, f"Prerequisite unmet: {msg}"
            elif req_lower == "wsl_ready":
                if os.name == 'nt':
                    try:
                        res = subprocess.run(["wsl", "-d", "Ubuntu", "-u", "root", "--", "uname", "-r"], capture_output=True, text=True, timeout=5)
                        if res.returncode != 0:
                            return False, "Prerequisite unmet: WSL2 Ubuntu not accessible"
                    except Exception as e:
                        return False, f"Prerequisite unmet: {str(e)}"
        return True, "All prerequisites satisfied"
