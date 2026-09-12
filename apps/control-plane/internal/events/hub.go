package events

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"opspilot/control-plane/internal/models"
)

type Hub struct {
	mu          sync.RWMutex
	subscribers map[string]map[chan *models.TaskEvent]bool
	globalSubs  map[chan *models.TaskEvent]bool
}

func NewHub() *Hub {
	return &Hub{
		subscribers: make(map[string]map[chan *models.TaskEvent]bool),
		globalSubs:  make(map[chan *models.TaskEvent]bool),
	}
}

func (h *Hub) Publish(taskID, eventType string, payload map[string]any) {
	event := &models.TaskEvent{
		ID:        time.Now().UnixNano(),
		TaskID:    taskID,
		EventType: eventType,
		Payload:   payload,
		CreatedAt: time.Now(),
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	// Notify task-specific subscribers
	if subs, ok := h.subscribers[taskID]; ok {
		for ch := range subs {
			select {
			case ch <- event:
			default:
			}
		}
	}

	// Notify global subscribers
	for ch := range h.globalSubs {
		select {
		case ch <- event:
		default:
		}
	}
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request, taskID string) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	eventChan := make(chan *models.TaskEvent, 50)

	h.mu.Lock()
	if taskID != "" {
		if _, exists := h.subscribers[taskID]; !exists {
			h.subscribers[taskID] = make(map[chan *models.TaskEvent]bool)
		}
		h.subscribers[taskID][eventChan] = true
	} else {
		h.globalSubs[eventChan] = true
	}
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		if taskID != "" {
			delete(h.subscribers[taskID], eventChan)
		} else {
			delete(h.globalSubs, eventChan)
		}
		close(eventChan)
		h.mu.Unlock()
	}()

	// Send initial ping
	fmt.Fprintf(w, "event: connected\ndata: {\"status\":\"connected\",\"task_id\":\"%s\"}\n\n", taskID)
	flusher.Flush()

	notify := r.Context().Done()
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-notify:
			return
		case <-ticker.C:
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		case event := <-eventChan:
			data, err := json.Marshal(event)
			if err == nil {
				fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.EventType, string(data))
				flusher.Flush()
			}
		}
	}
}
