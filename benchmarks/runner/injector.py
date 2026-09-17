import os
import subprocess
from typing import Tuple

class FaultInjector:
    @staticmethod
    def run_wsl_script(script_path: str, args: str = "") -> Tuple[int, str, str]:
        if not os.path.exists(script_path):
            return 0, "script not found (skipped)", ""

        if os.name != 'nt':
            cmd = ["bash", os.path.abspath(script_path)]
        else:
            # Convert windows path to wsl path
            norm = os.path.abspath(script_path).replace("\\", "/")
            if ":" in norm:
                drive, rest = norm.split(":", 1)
                wsl_path = f"/mnt/{drive.lower()}{rest}"
            else:
                wsl_path = norm

            cmd = ["wsl", "-d", "Ubuntu", "-u", "root", "--", "bash", wsl_path]
        if args:
            cmd.extend(args.split())

        try:
            res = subprocess.run(cmd, capture_output=True, text=True, timeout=30)
            return res.returncode, res.stdout, res.stderr
        except subprocess.TimeoutExpired:
            return 124, "", "script execution timed out after 30s"
        except Exception as e:
            return 1, "", str(e)

    @staticmethod
    def check_prerequisites(requirements: list[str]) -> Tuple[bool, str]:
        if not requirements:
            return True, ""
        for req in requirements:
            if req in ("docker", "docker_daemon_running"):
                if os.name != 'nt':
                    cmd = ["docker", "info"]
                else:
                    cmd = ["wsl", "-d", "Ubuntu", "-u", "root", "--", "docker", "info"]
                try:
                    res = subprocess.run(cmd, capture_output=True, text=True, timeout=10)
                    if res.returncode != 0:
                        return False, "Prerequisite unmet: Docker daemon not reachable"
                except Exception as e:
                    return False, f"Prerequisite unmet: Docker check failed ({e})"
        return True, ""

