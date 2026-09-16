package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"

	"opspilot/control-plane/internal/auth"
	"opspilot/control-plane/internal/database"
	"opspilot/control-plane/internal/events"
	"opspilot/control-plane/internal/models"
	"opspilot/control-plane/internal/orchestrator"
	"opspilot/control-plane/internal/policy"
	"opspilot/control-plane/internal/runbooks"
	"opspilot/control-plane/internal/statemachine"
)

type Handler struct {
	store         database.Store
	authService   *auth.Service
	orchestrator  *orchestrator.Orchestrator
	hub           *events.Hub
	machine       *statemachine.Machine
	runbookEngine *runbooks.Engine
}

func NewRouter(
	store database.Store,
	authService *auth.Service,
	orchestrator *orchestrator.Orchestrator,
	hub *events.Hub,
	machine *statemachine.Machine,
) http.Handler {
	pe := policy.NewEngine()
	h := &Handler{
		store:         store,
		authService:   authService,
		orchestrator:  orchestrator,
		hub:           hub,
		machine:       machine,
		runbookEngine: runbooks.NewEngine(pe),
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
	}))

	r.Get("/health", h.handleHealth)

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/auth/login", h.handleLogin)

		// Fleet & Agents
		r.Get("/agents", h.handleListAgents)
		r.Get("/agents/{id}", h.handleGetAgent)
		r.Get("/agents/{id}/telemetry", h.handleGetAgentTelemetry)

		// Tasks
		r.Get("/tasks", h.handleListTasks)
		r.Post("/tasks", h.handleCreateTask)
		r.Get("/tasks/{id}", h.handleGetTask)
		r.Post("/tasks/{id}/approve", h.handleApproveTask)
		r.Post("/tasks/{id}/reject", h.handleRejectTask)
		r.Get("/tasks/{id}/events", h.handleTaskSSE)

		// Global Events SSE
		r.Get("/events", h.handleGlobalSSE)

		// Runbooks
		r.Get("/runbooks", h.handleListRunbooks)
		r.Post("/runbooks", h.handleCreateRunbook)
		r.Get("/runbooks/{id}", h.handleGetRunbook)
		r.Post("/runbooks/{id}/execute", h.handleExecuteRunbook)
		r.Post("/runbooks/{id}/dry-run", h.handleDryRunRunbook)

		// Audit
		r.Get("/audit", h.handleListAudit)
	})

	return r
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]string{
		"status":    "healthy",
		"service":   "opspilot-control-plane",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	// Default demo credentials
	if req.Username == "admin" && (req.Password == "admin" || req.Password == "opspilot123") {
		token, err := h.authService.GenerateToken("usr-admin-1", "admin", []string{"admin"})
		if err != nil {
			jsonError(w, http.StatusInternalServerError, "failed to generate token")
			return
		}
		jsonResponse(w, http.StatusOK, map[string]any{
			"token":    token,
			"username": "admin",
			"roles":    []string{"admin"},
		})
		return
	}

	jsonError(w, http.StatusUnauthorized, "invalid credentials")
}

func (h *Handler) handleListAgents(w http.ResponseWriter, r *http.Request) {
	agents, err := h.store.ListAgents(r.Context())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, agents)
}

func (h *Handler) handleGetAgent(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	agent, err := h.store.GetAgent(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusNotFound, "agent not found")
		return
	}
	jsonResponse(w, http.StatusOK, agent)
}

func (h *Handler) handleGetAgentTelemetry(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	agent, err := h.store.GetAgent(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusNotFound, "agent not found")
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"agent_id": agent.ID,
		"hostname": agent.Hostname,
		"status":   agent.Status,
		"metrics":  agent.LastMetrics,
	})
}

func (h *Handler) handleListTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.store.ListTasks(r.Context())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, tasks)
}

func (h *Handler) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title          string                `json:"title"`
		Prompt         string                `json:"prompt"`
		TargetAgentIDs []string              `json:"target_agent_ids"`
		RolloutConfig  *models.RolloutConfig `json:"rollout_config,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}

	if req.Prompt == "" {
		jsonError(w, http.StatusBadRequest, "prompt is required")
		return
	}
	if len(req.TargetAgentIDs) == 0 {
		// Pick first available online agent if none provided
		agents, _ := h.store.ListAgents(r.Context())
		if len(agents) > 0 {
			req.TargetAgentIDs = []string{agents[0].ID}
		} else {
			jsonError(w, http.StatusBadRequest, "no available agents in fleet")
			return
		}
	}

	title := req.Title
	if title == "" {
		title = req.Prompt
		if len(title) > 60 {
			title = title[:57] + "..."
		}
	}

	idempotencyKey := strings.TrimSpace(r.Header.Get("Idempotency-Key"))
	if idempotencyKey != "" {
		existing, err := h.store.GetTaskByIdempotencyKey(r.Context(), idempotencyKey)
		if err == nil && existing != nil {
			_ = h.store.SaveAuditEvent(r.Context(), &models.AuditEvent{
				TaskID:    existing.ID,
				EventType: "IDEMPOTENT_NO_OP",
				Action:    "duplicate_request_prevented",
				Details:   map[string]any{"idempotency_key": idempotencyKey},
				CreatedAt: time.Now(),
			})
			jsonResponse(w, http.StatusOK, existing)
			return
		}
	}

	task := &models.Task{
		ID:             uuid.New().String(),
		Title:          title,
		Prompt:         req.Prompt,
		Status:         models.TaskStatusCreated,
		TargetAgentIDs: req.TargetAgentIDs,
		PlanVersion:    1,
		MaxReplans:     3,
		RiskLevel:      models.RiskLow,
		RolloutConfig:  req.RolloutConfig,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if task.RolloutConfig == nil && len(task.TargetAgentIDs) > 1 {
		task.RolloutConfig = &models.RolloutConfig{
			Strategy:    models.RolloutCanary,
			CanaryNodes: 1,
			MaxFailures: 1,
		}
	}
	if idempotencyKey != "" {
		task.IdempotencyKey = &idempotencyKey
	}

	if err := h.store.SaveTask(r.Context(), task); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to create task")
		return
	}

	_ = h.store.SaveAuditEvent(r.Context(), &models.AuditEvent{
		TaskID:    task.ID,
		EventType: "TASK_CREATED",
		Action:    "create",
		Details:   map[string]any{"prompt": task.Prompt, "targets": task.TargetAgentIDs, "idempotency_key": idempotencyKey},
		CreatedAt: time.Now(),
	})

	h.hub.Publish(task.ID, "TASK_CREATED", map[string]any{
		"task_id": task.ID,
		"title":   task.Title,
		"prompt":  task.Prompt,
	})

	// Start orchestrator pipeline in background
	go func() {
		_ = h.orchestrator.StartTask(context.Background(), task.ID)
	}()

	jsonResponse(w, http.StatusCreated, task)
}

func (h *Handler) handleGetTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	task, err := h.store.GetTask(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusNotFound, "task not found")
		return
	}
	history := h.machine.GetHistory(task.ID)
	verifications, _ := h.store.GetVerificationResults(r.Context(), task.ID)

	jsonResponse(w, http.StatusOK, map[string]any{
		"task":          task,
		"history":       history,
		"verifications": verifications,
	})
}

func (h *Handler) handleApproveTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		PlanVersion int    `json:"plan_version"`
		Notes       string `json:"notes"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.PlanVersion <= 0 {
		req.PlanVersion = 1
	}

	if err := h.orchestrator.ApproveTask(r.Context(), id, req.PlanVersion, "admin", req.Notes); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"status": "approved"})
}

func (h *Handler) handleRejectTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req struct {
		PlanVersion int    `json:"plan_version"`
		Notes       string `json:"notes"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.PlanVersion <= 0 {
		req.PlanVersion = 1
	}

	if err := h.orchestrator.RejectTask(r.Context(), id, req.PlanVersion, "admin", req.Notes); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"status": "rejected"})
}

func (h *Handler) handleTaskSSE(w http.ResponseWriter, r *http.Request) {
	taskID := chi.URLParam(r, "id")
	h.hub.ServeHTTP(w, r, taskID)
}

func (h *Handler) handleGlobalSSE(w http.ResponseWriter, r *http.Request) {
	h.hub.ServeHTTP(w, r, "")
}

func (h *Handler) handleListRunbooks(w http.ResponseWriter, r *http.Request) {
	runbooksList, err := h.store.ListRunbooks(r.Context())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if len(runbooksList) == 0 {
		defaults := runbooks.GetDefaultRunbooks()
		for _, d := range defaults {
			_ = h.store.SaveRunbook(r.Context(), d)
		}
		runbooksList, _ = h.store.ListRunbooks(r.Context())
	}
	jsonResponse(w, http.StatusOK, runbooksList)
}

func (h *Handler) handleGetRunbook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rb, err := h.store.GetRunbook(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusNotFound, "runbook not found")
		return
	}
	jsonResponse(w, http.StatusOK, rb)
}

func (h *Handler) handleDryRunRunbook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rb, err := h.store.GetRunbook(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusNotFound, "runbook not found")
		return
	}

	var req models.RunbookExecutionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		jsonError(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	res, err := h.runbookEngine.DryRun(rb, req.Parameters)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, res)
}

func (h *Handler) handleExecuteRunbook(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	rb, err := h.store.GetRunbook(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusNotFound, "runbook not found")
		return
	}

	var req models.RunbookExecutionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && err.Error() != "EOF" {
		jsonError(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	if req.DryRun {
		res, err := h.runbookEngine.DryRun(rb, req.Parameters)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, res)
		return
	}

	plan, err := h.runbookEngine.CompilePlan(rb, req.Parameters)
	if err != nil {
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("invalid parameters: %v", err))
		return
	}

	dryRunRes, _ := h.runbookEngine.DryRun(rb, req.Parameters)
	if !dryRunRes.PolicyPass {
		jsonResponse(w, http.StatusForbidden, map[string]any{
			"error":   "Runbook execution blocked by security guardrails",
			"details": dryRunRes.Validations,
		})
		return
	}

	targetAgents := req.TargetAgentIDs
	if len(targetAgents) == 0 {
		agents, _ := h.store.ListAgents(r.Context())
		if len(agents) > 0 {
			targetAgents = []string{agents[0].ID}
		} else {
			jsonError(w, http.StatusBadRequest, "no available target agents in fleet")
			return
		}
	}

	task := &models.Task{
		ID:             uuid.New().String(),
		Title:          fmt.Sprintf("Runbook: %s", plan.Goal),
		Prompt:         fmt.Sprintf("Execute runbook %s (%s)", rb.Slug, rb.Title),
		Status:         models.TaskStatusCreated,
		TargetAgentIDs: targetAgents,
		PlanVersion:    1,
		MaxReplans:     3,
		RiskLevel:      dryRunRes.MaxRisk,
		AIPlan:         plan,
		RolloutConfig:  req.RolloutConfig,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if task.RolloutConfig == nil && len(task.TargetAgentIDs) > 1 {
		task.RolloutConfig = &models.RolloutConfig{
			Strategy:    models.RolloutCanary,
			CanaryNodes: 1,
			MaxFailures: 1,
		}
	}

	if err := h.store.SaveTask(r.Context(), task); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to create task for runbook")
		return
	}

	for idx, sp := range plan.Steps {
		step := &models.TaskStep{
			ID:                   uuid.New().String(),
			TaskID:               task.ID,
			StepOrder:            idx + 1,
			Action:               sp.Action,
			Arguments:            sp.Arguments,
			Status:               "PENDING",
			RiskLevel:            sp.SuggestedRisk,
			VerificationStrategy: sp.VerificationStrategy,
		}
		_ = h.store.SaveTaskStep(r.Context(), step)
	}

	_ = h.store.SaveAuditEvent(r.Context(), &models.AuditEvent{
		TaskID:    task.ID,
		EventType: "RUNBOOK_EXECUTED",
		Action:    "runbook_execute",
		Details: map[string]any{
			"runbook_id":   rb.ID,
			"runbook_slug": rb.Slug,
			"parameters":   req.Parameters,
			"targets":      targetAgents,
		},
		CreatedAt: time.Now(),
	})

	h.hub.Publish(task.ID, "TASK_CREATED", map[string]any{
		"task_id":      task.ID,
		"title":        task.Title,
		"runbook_slug": rb.Slug,
	})

	go func() {
		_ = h.orchestrator.StartTask(context.Background(), task.ID)
	}()

	jsonResponse(w, http.StatusCreated, task)
}

func (h *Handler) handleCreateRunbook(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TaskID      string `json:"task_id"`
		Slug        string `json:"slug"`
		Title       string `json:"title"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request")
		return
	}

	task, err := h.store.GetTask(r.Context(), req.TaskID)
	if err != nil {
		jsonError(w, http.StatusNotFound, "task not found")
		return
	}

	rb := &models.Runbook{
		ID:              uuid.New().String(),
		Slug:            req.Slug,
		Title:           req.Title,
		Description:     req.Description,
		CreatedFromTask: task.ID,
		LatestVersion:   1,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	if task.AIPlan != nil {
		rb.Steps = task.AIPlan.Steps
		rb.OverallVerification = task.AIPlan.OverallVerification
	}

	if err := h.store.SaveRunbook(r.Context(), rb); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusCreated, rb)
}


func (h *Handler) handleListAudit(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
		limit = l
	}
	events, err := h.store.ListAuditEvents(r.Context(), limit)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, events)
}

func jsonResponse(w http.ResponseWriter, code int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, code int, message string) {
	jsonResponse(w, code, map[string]string{"error": message})
}
