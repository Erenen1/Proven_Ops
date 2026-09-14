import logging
import os
from fastapi import FastAPI, HTTPException
from fastapi.middleware.cors import CORSMiddleware
from .config import settings
from .models.schemas import (
    PlanRequest,
    PlanResponse,
    ReplanRequest,
    DiagnosisRequest,
    DiagnosisResponse,
)
from .prompts.system_prompt import SYSTEM_PROMPT
from .prompts.planner import (
    build_planning_prompt,
    build_replanning_prompt,
    build_diagnosis_prompt,
)
from .providers.ollama_provider import OllamaProvider

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger("ai-service")

app = FastAPI(
    title="OpsPilot AI Service",
    version="1.0.0",
    description="Deterministic AI Planning & Reasoning microservice for OpsPilot"
)

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

provider = OllamaProvider(
    base_url=settings.OLLAMA_BASE_URL,
    model_name=settings.DEFAULT_LLM_MODEL,
    timeout=settings.REQUEST_TIMEOUT_SEC
)

@app.get("/health")
async def health_check():
    return {
        "status": "ok",
        "model": settings.DEFAULT_LLM_MODEL,
        "ollama_base_url": settings.OLLAMA_BASE_URL
    }

@app.get("/api/v1/config")
async def get_config():
    allow_fallback = os.getenv("ENABLE_HEURISTIC_FALLBACK", "false").lower() in ("true", "1")
    digest = await provider.get_model_digest()
    return {
        "status": "ok",
        "provider": "ollama",
        "model": provider.model_name,
        "model_digest": digest,
        "fallback_allowed": allow_fallback,
        "ollama_base_url": provider.base_url
    }

@app.post("/api/v1/plan", response_model=PlanResponse)
async def create_plan(req: PlanRequest):
    logger.info(f"Generating plan for task={req.task_id}, intent='{req.intent}'")
    user_prompt = build_planning_prompt(req)
    try:
        raw_json, prov = await provider.generate_json_with_provenance(
            SYSTEM_PROMPT, user_prompt, purpose="PLAN", task_id=req.task_id
        )
        plan = PlanResponse.model_validate(raw_json)
        plan.provenance = prov
        return plan
    except Exception as e:
        logger.error(f"Planning failed: {str(e)}", exc_info=True)
        raise HTTPException(status_code=500, detail=f"Planning generation error: {str(e)}")

@app.post("/api/v1/replan", response_model=PlanResponse)
async def replan_task(req: ReplanRequest):
    logger.info(f"Replanning task={req.task_id}, failed_step={req.failed_step_id}")
    user_prompt = build_replanning_prompt(req)
    try:
        raw_json, prov = await provider.generate_json_with_provenance(
            SYSTEM_PROMPT, user_prompt, purpose="REPLAN", task_id=req.task_id
        )
        plan = PlanResponse.model_validate(raw_json)
        plan.provenance = prov
        return plan
    except Exception as e:
        logger.error(f"Replanning failed: {str(e)}", exc_info=True)
        raise HTTPException(status_code=500, detail=f"Replanning generation error: {str(e)}")

@app.post("/api/v1/diagnose", response_model=DiagnosisResponse)
async def diagnose_issue(req: DiagnosisRequest):
    logger.info(f"Diagnosing issue for task={req.task_id}, symptom='{req.symptom}'")
    user_prompt = build_diagnosis_prompt(req)
    try:
        raw_json, prov = await provider.generate_json_with_provenance(
            SYSTEM_PROMPT, user_prompt, purpose="DIAGNOSIS", task_id=req.task_id
        )
        diagnosis = DiagnosisResponse.model_validate(raw_json)
        diagnosis.provenance = prov
        return diagnosis
    except Exception as e:
        logger.error(f"Diagnosis failed: {str(e)}", exc_info=True)
        raise HTTPException(status_code=500, detail=f"Diagnosis generation error: {str(e)}")
