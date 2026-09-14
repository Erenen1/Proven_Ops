import json
import os
import re
import time
import uuid
import httpx
from typing import Dict, Any, Tuple, Optional
from .base import LLMProvider
from ..models.schemas import ProvenanceMetadata

class OllamaProvider(LLMProvider):
    def __init__(self, base_url: str, model_name: str, timeout: float = 60.0):
        self.base_url = base_url.rstrip("/")
        self.model_name = model_name
        self.timeout = timeout
        self._model_digest: Optional[str] = None

    async def get_model_digest(self) -> Optional[str]:
        if self._model_digest:
            return self._model_digest
        try:
            async with httpx.AsyncClient(timeout=5.0) as client:
                res = await client.get(f"{self.base_url}/api/tags")
                if res.status_code == 200:
                    data = res.json()
                    for m in data.get("models", []):
                        if m.get("name") == self.model_name or m.get("model") == self.model_name:
                            self._model_digest = m.get("digest")
                            return self._model_digest
        except Exception:
            pass
        return None

    async def generate_json(self, system_prompt: str, user_prompt: str) -> Dict[str, Any]:
        data, _ = await self.generate_json_with_provenance(system_prompt, user_prompt)
        return data

    async def generate_json_with_provenance(
        self,
        system_prompt: str,
        user_prompt: str,
        purpose: str = "PLAN",
        task_id: Optional[str] = None,
        scenario_id: Optional[str] = None
    ) -> Tuple[Dict[str, Any], ProvenanceMetadata]:
        invocation_id = str(uuid.uuid4())
        start_time = time.time()
        url = f"{self.base_url}/api/chat"
        payload = {
            "model": self.model_name,
            "messages": [
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_prompt}
            ],
            "format": "json",
            "stream": False,
            "options": {
                "temperature": 0.1
            }
        }

        digest = await self.get_model_digest()
        allow_fallback = os.getenv("ENABLE_HEURISTIC_FALLBACK", "false").lower() in ("true", "1")
        try:
            async with httpx.AsyncClient(timeout=self.timeout) as client:
                res = await client.post(url, json=payload)
                if res.status_code == 200:
                    data = res.json()
                    content = data.get("message", {}).get("content", "")
                    parsed = self._clean_and_parse_json(content)
                    latency_ms = int((time.time() - start_time) * 1000)
                    prov = ProvenanceMetadata(
                        invocation_id=invocation_id,
                        task_id=task_id,
                        scenario_id=scenario_id,
                        purpose=purpose,
                        provider="ollama",
                        model=self.model_name,
                        model_digest=digest,
                        fallback_used=False,
                        latency_ms=latency_ms,
                        schema_valid=True
                    )
                    return parsed, prov
                elif not allow_fallback:
                    raise RuntimeError(f"Ollama returned HTTP {res.status_code}: {res.text}")
        except Exception as e:
            if not allow_fallback:
                raise RuntimeError(f"Ollama planning failure: {e}") from e
            fallback_err = str(e)

        # Fallback used
        latency_ms = int((time.time() - start_time) * 1000)
        fallback_data = self._heuristic_fallback(user_prompt)
        prov = ProvenanceMetadata(
            invocation_id=invocation_id,
            task_id=task_id,
            scenario_id=scenario_id,
            purpose=purpose,
            provider="heuristic_fallback",
            model=self.model_name,
            model_digest=digest,
            fallback_used=True,
            fallback_reason=fallback_err if 'fallback_err' in locals() else "HTTP non-200 or connection error",
            latency_ms=latency_ms,
            schema_valid=True
        )
        return fallback_data, prov

    def _clean_and_parse_json(self, text: str) -> Dict[str, Any]:
        text = text.strip()
        # Strip markdown ```json ... ``` codeblocks if present
        match = re.search(r"```(?:json)?\s*(\{.*?\})\s*```", text, re.DOTALL)
        if match:
            text = match.group(1)
        elif "{" in text and "}" in text:
            start = text.find("{")
            end = text.rfind("}") + 1
            text = text[start:end]
        return json.loads(text)

    def _heuristic_fallback(self, prompt: str) -> Dict[str, Any]:
        prompt_lower = prompt.lower()
        if "nginx" in prompt_lower and ("8080" in prompt_lower or "port" in prompt_lower):
            return {
                "goal": "Install and configure Nginx on custom port 8080 and verify",
                "reasoning": "Standard web server deployment flow: inspect environment, install package, apply custom port configuration, restart systemd service, and verify open port and HTTP endpoint.",
                "steps": [
                    {
                        "id": "step-1",
                        "action": "get_os_info",
                        "arguments": {},
                        "reason": "Verify Linux distribution compatibility",
                        "suggested_risk": "READ_ONLY",
                        "verification_strategy": None
                    },
                    {
                        "id": "step-2",
                        "action": "check_package",
                        "arguments": {"name": "nginx"},
                        "reason": "Check whether Nginx is already installed",
                        "suggested_risk": "READ_ONLY",
                        "verification_strategy": None
                    },
                    {
                        "id": "step-3",
                        "action": "install_package",
                        "arguments": {"name": "nginx"},
                        "reason": "Install Nginx web server package via apt",
                        "suggested_risk": "MEDIUM",
                        "verification_strategy": {
                            "check_type": "package_installed",
                            "target": "nginx",
                            "expected": "installed",
                            "timeout_sec": 30
                        }
                    },
                    {
                        "id": "step-4",
                        "action": "write_config_file",
                        "arguments": {
                            "path": "/etc/nginx/sites-available/default",
                            "content": "server {\n    listen 8080 default_server;\n    listen [::]:8080 default_server;\n    root /var/www/html;\n    index index.html index.nginx-debian.html;\n    server_name _;\n    location / {\n        try_files $uri $uri/ =404;\n    }\n}\n"
                        },
                        "reason": "Update Nginx default site configuration to listen on port 8080",
                        "suggested_risk": "MEDIUM",
                        "verification_strategy": None
                    },
                    {
                        "id": "step-5",
                        "action": "restart_service",
                        "arguments": {"name": "nginx"},
                        "reason": "Restart Nginx to apply the new port 8080 configuration",
                        "suggested_risk": "MEDIUM",
                        "verification_strategy": {
                            "check_type": "systemd_active",
                            "target": "nginx",
                            "expected": "active",
                            "timeout_sec": 15
                        }
                    },
                    {
                        "id": "step-6",
                        "action": "check_port",
                        "arguments": {"port": 8080, "protocol": "tcp"},
                        "reason": "Verify that port 8080 is actively listening",
                        "suggested_risk": "READ_ONLY",
                        "verification_strategy": {
                            "check_type": "tcp_port_open",
                            "target": "8080",
                            "expected": "open",
                            "timeout_sec": 10
                        }
                    },
                    {
                        "id": "step-7",
                        "action": "http_probe",
                        "arguments": {"url": "http://127.0.0.1:8080", "expected_status": 200},
                        "reason": "Verify HTTP 200 response on port 8080",
                        "suggested_risk": "READ_ONLY",
                        "verification_strategy": {
                            "check_type": "http_probe",
                            "target": "http://127.0.0.1:8080",
                            "expected": "200",
                            "timeout_sec": 10
                        }
                    }
                ],
                "overall_verification": [
                    {"check_type": "systemd_active", "target": "nginx", "expected": "active", "timeout_sec": 10},
                    {"check_type": "tcp_port_open", "target": "8080", "expected": "open", "timeout_sec": 10},
                    {"check_type": "http_probe", "target": "http://127.0.0.1:8080", "expected": "200", "timeout_sec": 10}
                ]
            }
        elif "docker" in prompt_lower and "install" in prompt_lower:
            return {
                "goal": "Install Docker engine and verify service",
                "reasoning": "Install docker.io package, enable and start docker service, then verify with docker info.",
                "steps": [
                    {
                        "id": "step-1",
                        "action": "check_package",
                        "arguments": {"name": "docker.io"},
                        "reason": "Check docker package status",
                        "suggested_risk": "READ_ONLY",
                        "verification_strategy": None
                    },
                    {
                        "id": "step-2",
                        "action": "install_package",
                        "arguments": {"name": "docker.io"},
                        "reason": "Install docker engine package",
                        "suggested_risk": "MEDIUM",
                        "verification_strategy": {"check_type": "package_installed", "target": "docker.io", "expected": "installed", "timeout_sec": 60}
                    },
                    {
                        "id": "step-3",
                        "action": "start_service",
                        "arguments": {"name": "docker"},
                        "reason": "Ensure Docker service is running",
                        "suggested_risk": "MEDIUM",
                        "verification_strategy": {"check_type": "systemd_active", "target": "docker", "expected": "active", "timeout_sec": 15}
                    }
                ],
                "overall_verification": [
                    {"check_type": "systemd_active", "target": "docker", "expected": "active", "timeout_sec": 10}
                ]
            }
        
        # Generic fallback plan for system inspection
        return {
            "goal": "Discover and inspect system state",
            "reasoning": "Gather OS, service, and network status for evaluation.",
            "steps": [
                {
                    "id": "step-1",
                    "action": "get_os_info",
                    "arguments": {},
                    "reason": "Inspect OS and platform details",
                    "suggested_risk": "READ_ONLY",
                    "verification_strategy": None
                },
                {
                    "id": "step-2",
                    "action": "get_system_info",
                    "arguments": {},
                    "reason": "Inspect CPU, memory, and load average",
                    "suggested_risk": "READ_ONLY",
                    "verification_strategy": None
                }
            ],
            "overall_verification": []
        }
