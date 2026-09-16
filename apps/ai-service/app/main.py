import logging
import os
from fastapi import FastAPI, HTTPException, Request
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
from .services.diagnosis_grounding import extract_evidence_and_root_cause
from .models.schemas import RootCause

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

@app.middleware("http")
async def tracing_middleware(request: Request, call_next):
    traceparent = request.headers.get("traceparent")
    trace_id = None
    span_id = None
    if traceparent and traceparent.startswith("00-"):
        parts = traceparent.split("-")
        if len(parts) >= 4:
            trace_id = parts[1]
            span_id = parts[2]
    elif request.headers.get("x-trace-id"):
        trace_id = request.headers.get("x-trace-id")

    request.state.trace_id = trace_id
    request.state.span_id = span_id

    response = await call_next(request)
    if traceparent:
        response.headers["traceparent"] = traceparent
    if trace_id:
        response.headers["X-Trace-ID"] = trace_id
    return response

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
async def create_plan(req: PlanRequest, request: Request):
    logger.info(f"Generating plan for task={req.task_id}, intent='{req.intent}'")
    user_prompt = build_planning_prompt(req)
    trace_id = getattr(request.state, "trace_id", None)
    span_id = getattr(request.state, "span_id", None)
    try:
        raw_json, prov = await provider.generate_json_with_provenance(
            SYSTEM_PROMPT, user_prompt, purpose="PLAN", task_id=req.task_id, trace_id=trace_id, span_id=span_id
        )
        plan = PlanResponse.model_validate(raw_json)
        plan.provenance = prov
        return plan
    except Exception as e:
        logger.error(f"Planning failed: {str(e)}", exc_info=True)
        raise HTTPException(status_code=500, detail=f"Planning generation error: {str(e)}")

@app.post("/api/v1/replan", response_model=PlanResponse)
async def replan_task(req: ReplanRequest, request: Request):
    logger.info(f"Replanning task={req.task_id}, failed_step={req.failed_step_id}")
    user_prompt = build_replanning_prompt(req)
    trace_id = getattr(request.state, "trace_id", None)
    span_id = getattr(request.state, "span_id", None)
    try:
        raw_json, prov = await provider.generate_json_with_provenance(
            SYSTEM_PROMPT, user_prompt, purpose="REPLAN", task_id=req.task_id, trace_id=trace_id, span_id=span_id
        )
        plan = PlanResponse.model_validate(raw_json)
        plan.provenance = prov
        return plan
    except Exception as e:
        logger.error(f"Replanning failed: {str(e)}", exc_info=True)
        raise HTTPException(status_code=500, detail=f"Replanning generation error: {str(e)}")

@app.post("/api/v1/diagnose", response_model=DiagnosisResponse)
async def diagnose_issue(req: DiagnosisRequest, request: Request):
    logger.info(f"Diagnosing issue for task={req.task_id}, symptom='{req.symptom}'")
    user_prompt = build_diagnosis_prompt(req)
    trace_id = getattr(request.state, "trace_id", None)
    span_id = getattr(request.state, "span_id", None)
    try:
        raw_json, prov = await provider.generate_json_with_provenance(
            SYSTEM_PROMPT, user_prompt, purpose="DIAGNOSIS", task_id=req.task_id, trace_id=trace_id, span_id=span_id
        )
        diagnosis = DiagnosisResponse.model_validate(raw_json)
        diagnosis.provenance = prov

        # Grounding reconciliation: Ensure deterministic evidence corrects any UNKNOWN or unsupported diagnoses
        grounded_cause, grounded_conf, grounded_evidences = extract_evidence_and_root_cause(req.symptom, req.untrusted_logs)
        if grounded_cause != RootCause.UNKNOWN:
            if diagnosis.root_cause in ["UNKNOWN", "NONE"] or diagnosis.confidence < 0.7:
                diagnosis.root_cause = grounded_cause.value
                diagnosis.confidence = max(diagnosis.confidence, grounded_conf)
            # Append high-confidence deterministic evidence if not already present
            existing_details = {e.detail for e in (diagnosis.evidence or [])}
            for ge in grounded_evidences:
                if ge.detail not in existing_details:
                    if diagnosis.evidence is None:
                        diagnosis.evidence = []
                    diagnosis.evidence.append(ge)

        return diagnosis
    except Exception as e:
        logger.error(f"Diagnosis failed: {str(e)}", exc_info=True)
        raise HTTPException(status_code=500, detail=f"Diagnosis generation error: {str(e)}")
