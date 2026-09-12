package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
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
	"opspilot/control-plane/internal/statemachine"
)

type Handler struct {
	store        database.Store
	authService  *auth.Service
	orchestrator *orchestrator.Orchestrator
	hub          *events.Hub
	machine      *statemachine.Machine
}

func NewRouter(
	store database.Store,
	authService *auth.Service,
	orchestrator *orchestrator.Orchestrator,
	hub *events.Hub,
	machine *statemachine.Machine,
) http.Handler {
	h := &Handler{
		store:        store,
		authService:  authService,
		orchestrator: orchestrator,
		hub:          hub,
		machine:      machine,
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
		Title          string   `json:"title"`
		Prompt         string   `json:"prompt"`
		TargetAgentIDs []string `json:"target_agent_ids"`
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

	task := &models.Task{
		ID:             uuid.New().String(),
		Title:          title,
		Prompt:         req.Prompt,
		Status:         models.TaskStatusCreated,
		TargetAgentIDs: req.TargetAgentIDs,
		PlanVersion:    1,
		RiskLevel:      models.RiskLow,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	if err := h.store.SaveTask(r.Context(), task); err != nil {
		jsonError(w, http.StatusInternalServerError, "failed to create task")
		return
	}

	_ = h.store.SaveAuditEvent(r.Context(), &models.AuditEvent{
		TaskID:    task.ID,
		EventType: "TASK_CREATED",
		Action:    "create",
		Details:   map[string]any{"prompt": task.Prompt, "targets": task.TargetAgentIDs},
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
	runbooks, err := h.store.ListRunbooks(r.Context())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, runbooks)
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
		Steps:           task.AIPlan.Steps,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
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
