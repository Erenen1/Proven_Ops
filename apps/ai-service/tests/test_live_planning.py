import asyncio
import os
import sys

# Add apps/ai-service to sys.path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), ".."))

from app.config import settings
from app.models.schemas import PlanRequest, HostContext, PlanResponse
from app.prompts.planner import build_planning_prompt
from app.prompts.system_prompt import SYSTEM_PROMPT
from app.providers.ollama_provider import OllamaProvider

async def main():
    print(f"[TEST] Calling Ollama at {settings.OLLAMA_BASE_URL} with model {settings.DEFAULT_LLM_MODEL}...")
    provider = OllamaProvider(
        base_url=settings.OLLAMA_BASE_URL,
        model_name=settings.DEFAULT_LLM_MODEL,
        timeout=120.0
    )

    req = PlanRequest(
        task_id="test-task-nginx-live",
        intent="Install nginx on this server and expose it on port 8080.",
        host_context=HostContext(
            hostname="ubuntu-node-01",
            os="linux",
            distribution="ubuntu",
            version="24.04",
            architecture="amd64",
            capabilities=["systemd", "apt", "network"],
            open_ports=[22],
            active_services=["ssh"]
        ),
        supported_tools=[
            "get_os_info", "check_package", "install_package",
            "write_config_file", "read_file", "start_service",
            "restart_service", "get_service_status", "check_port", "http_probe"
        ]
    )

    prompt = build_planning_prompt(req)
    raw_json = await provider.generate_json(SYSTEM_PROMPT, prompt)
    print("\n[TEST] Raw JSON returned from Qwen:")
    import json
    print(json.dumps(raw_json, indent=2))

    plan = PlanResponse.model_validate(raw_json)
    print(f"\n[TEST] Plan successfully validated by Pydantic!")
    print(f"Goal: {plan.goal}")
    print(f"Number of steps: {len(plan.steps)}")
    for i, s in enumerate(plan.steps, 1):
        print(f"  {i}. action={s.action}, args={s.arguments}, reason={s.reason}")
    print(f"Overall Verification checks: {len(plan.overall_verification)}")
    for v in plan.overall_verification:
        print(f"  - Check: {v.check_type} target={v.target}")

if __name__ == "__main__":
    asyncio.run(main())
