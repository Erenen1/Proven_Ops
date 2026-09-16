package grpcserver

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"opspilot/control-plane/internal/database"
	"opspilot/control-plane/internal/events"
	"opspilot/control-plane/internal/models"
	opspilotv1 "opspilot/proto/v1"
)

type Server struct {
	opspilotv1.UnimplementedAgentServiceServer
	store          database.Store
	hub            *events.Hub
	bootstrapToken string
	mu             sync.RWMutex
	activeStreams  map[string]opspilotv1.AgentService_ConnectStreamServer
	stepResultChans map[string]chan *opspilotv1.StepResult
}

func NewServer(store database.Store, hub *events.Hub, bootstrapToken string) *Server {
	return &Server{
		store:           store,
		hub:             hub,
		bootstrapToken:  bootstrapToken,
		activeStreams:   make(map[string]opspilotv1.AgentService_ConnectStreamServer),
		stepResultChans: make(map[string]chan *opspilotv1.StepResult),
	}
}

func (s *Server) Register(ctx context.Context, req *opspilotv1.RegisterRequest) (*opspilotv1.RegisterResponse, error) {
	if req.BootstrapToken != s.bootstrapToken {
		return &opspilotv1.RegisterResponse{
			Approved: false,
			Message:  "Invalid bootstrap enrollment token",
		}, status.Error(codes.Unauthenticated, "invalid bootstrap token")
	}

	agentID := fmt.Sprintf("agent-%s", req.Hostname)
	if req.Hostname == "" {
		agentID = fmt.Sprintf("agent-%s", uuid.New().String()[:8])
	}

	env := req.Environment
	if env == "" {
		env = "development"
	}

	now := time.Now()
	agent := &models.Agent{
		ID:           agentID,
		Hostname:     req.Hostname,
		IPAddress:    req.IpAddress,
		OS:           req.Os,
		Distribution: req.Distribution,
		Version:      req.Version,
		Architecture: req.Architecture,
		Status:       "online",
		Environment:  env,
		Capabilities: req.Capabilities,
		LastHeartbeat: &now,
		CreatedAt:    now,
	}

	if err := s.store.SaveAgent(ctx, agent); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to save agent: %v", err)
	}

	s.hub.Publish("", "AGENT_REGISTERED", map[string]any{
		"agent_id":     agentID,
		"hostname":     req.Hostname,
		"capabilities": req.Capabilities,
		"distribution": req.Distribution,
	})

	return &opspilotv1.RegisterResponse{
		AgentId:              agentID,
		Approved:             true,
		Message:              "Registration approved",
		HeartbeatIntervalSec: 5,
	}, nil
}

func (s *Server) SendHeartbeat(ctx context.Context, req *opspilotv1.HeartbeatRequest) (*opspilotv1.HeartbeatResponse, error) {
	metrics := &models.AgentMetrics{
		CPUUsagePercent:  req.CpuUsagePercent,
		MemoryUsageBytes: req.MemoryUsageBytes,
		MemoryTotalBytes: req.MemoryTotalBytes,
		DiskUsagePercent: req.DiskUsagePercent,
		LoadAvg1m:        req.LoadAvg_1M,
		ActiveTasks:      int(req.ActiveTasks),
		RecordedAt:       time.Now(),
	}

	_ = s.store.SaveHeartbeat(ctx, req.AgentId, metrics)

	if s.hub != nil {
		s.hub.Publish("", "NODE_TELEMETRY", map[string]any{
			"agent_id":           req.AgentId,
			"cpu_usage_percent":  req.CpuUsagePercent,
			"memory_usage_bytes": req.MemoryUsageBytes,
			"memory_total_bytes": req.MemoryTotalBytes,
			"disk_usage_percent": req.DiskUsagePercent,
			"load_avg_1m":        req.LoadAvg_1M,
			"active_tasks":       req.ActiveTasks,
			"timestamp_unix":     req.TimestampUnix,
		})
	}

	return &opspilotv1.HeartbeatResponse{
		Acknowledged:    true,
		HasPendingTasks: false,
	}, nil
}

func (s *Server) ConnectStream(stream opspilotv1.AgentService_ConnectStreamServer) error {
	var currentAgentID string

	defer func() {
		if currentAgentID != "" {
			s.mu.Lock()
			delete(s.activeStreams, currentAgentID)
			for stepID, ch := range s.stepResultChans {
				select {
				case ch <- &opspilotv1.StepResult{
					StepId:       stepID,
					Success:      false,
					ExitCode:     1,
					Stderr:       "connection closed: agent disconnected",
					ErrorMessage: "connection closed: agent disconnected",
				}:
				default:
				}
			}
			s.mu.Unlock()
			_ = s.store.UpdateAgentStatus(context.Background(), currentAgentID, "offline")
			s.hub.Publish("", "AGENT_DISCONNECTED", map[string]any{
				"agent_id": currentAgentID,
			})
		}
	}()

	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}

		switch payload := msg.Payload.(type) {
		case *opspilotv1.AgentStreamMessage_Handshake:
			currentAgentID = payload.Handshake.AgentId
			s.mu.Lock()
			s.activeStreams[currentAgentID] = stream
			s.mu.Unlock()
			_ = s.store.UpdateAgentStatus(context.Background(), currentAgentID, "online")

		case *opspilotv1.AgentStreamMessage_OutputChunk:
			s.hub.Publish(payload.OutputChunk.TaskId, "STEP_OUTPUT", map[string]any{
				"task_id":     payload.OutputChunk.TaskId,
				"step_id":     payload.OutputChunk.StepId,
				"stream_type": payload.OutputChunk.StreamType,
				"data":        payload.OutputChunk.Data,
			})

		case *opspilotv1.AgentStreamMessage_StepResult:
			s.mu.RLock()
			ch, ok := s.stepResultChans[payload.StepResult.StepId]
			s.mu.RUnlock()
			if ok {
				select {
				case ch <- payload.StepResult:
				default:
				}
			}
			s.hub.Publish(payload.StepResult.TaskId, "STEP_COMPLETED", map[string]any{
				"task_id":   payload.StepResult.TaskId,
				"step_id":   payload.StepResult.StepId,
				"action":    payload.StepResult.Action,
				"exit_code": payload.StepResult.ExitCode,
				"success":   payload.StepResult.Success,
			})

		case *opspilotv1.AgentStreamMessage_Pong:
			// Heartbeat pong received
		}
	}
}

// DispatchStep sends a step execution command to the connected agent stream and waits for the result
func (s *Server) DispatchStep(ctx context.Context, agentID string, cmd *opspilotv1.ExecuteStepCommand) (*opspilotv1.StepResult, error) {
	s.mu.RLock()
	stream, online := s.activeStreams[agentID]
	s.mu.RUnlock()

	if !online {
		return nil, fmt.Errorf("agent %s is not connected to active gRPC stream", agentID)
	}

	resultChan := make(chan *opspilotv1.StepResult, 1)
	s.mu.Lock()
	s.stepResultChans[cmd.StepId] = resultChan
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.stepResultChans, cmd.StepId)
		s.mu.Unlock()
	}()

	msg := &opspilotv1.ControlStreamMessage{
		MessageId: uuid.New().String(),
		Payload: &opspilotv1.ControlStreamMessage_ExecuteStep{
			ExecuteStep: cmd,
		},
	}

	if err := stream.Send(msg); err != nil {
		return nil, fmt.Errorf("failed to dispatch step to agent %s: %v", agentID, err)
	}

	timeout := time.Duration(cmd.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(timeout):
		return nil, fmt.Errorf("step execution timed out after %v", timeout)
	case res := <-resultChan:
		return res, nil
	}
}
