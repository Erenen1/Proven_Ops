package telemetry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTraceContext_TraceParent(t *testing.T) {
	tc := NewTraceContext()
	if len(tc.TraceID) != 32 {
		t.Fatalf("expected 32 char trace_id, got %d", len(tc.TraceID))
	}
	if len(tc.SpanID) != 16 {
		t.Fatalf("expected 16 char span_id, got %d", len(tc.SpanID))
	}

	header := tc.TraceParent()
	parsed, err := ParseTraceParent(header)
	if err != nil {
		t.Fatalf("failed to parse generated traceparent: %v", err)
	}

	if parsed.TraceID != tc.TraceID {
		t.Errorf("expected trace ID %s, got %s", tc.TraceID, parsed.TraceID)
	}
	if parsed.SpanID != tc.SpanID {
		t.Errorf("expected span ID %s, got %s", tc.SpanID, parsed.SpanID)
	}
}

func TestTraceContext_NewChildSpan(t *testing.T) {
	parent := NewTraceContext()
	child := parent.NewChildSpan()

	if child.TraceID != parent.TraceID {
		t.Errorf("child should inherit parent TraceID, got %s vs %s", child.TraceID, parent.TraceID)
	}
	if child.SpanID == parent.SpanID {
		t.Errorf("child must have a distinct SpanID, got same %s", child.SpanID)
	}
}

func TestHTTPContextPropagation(t *testing.T) {
	tc := NewTraceContext()
	ctx := WithTraceContext(context.Background(), tc)

	req := httptest.NewRequest("GET", "/api/v1/tasks", nil)
	InjectHTTP(ctx, req.Header)

	if req.Header.Get("traceparent") != tc.TraceParent() {
		t.Errorf("expected header %s, got %s", tc.TraceParent(), req.Header.Get("traceparent"))
	}
	if req.Header.Get("X-Trace-ID") != tc.TraceID {
		t.Errorf("expected X-Trace-ID %s, got %s", tc.TraceID, req.Header.Get("X-Trace-ID"))
	}

	extracted := ExtractHTTP(req)
	if extracted.TraceID != tc.TraceID {
		t.Errorf("extracted TraceID %s does not match original %s", extracted.TraceID, tc.TraceID)
	}
}

func TestHTTPMiddleware(t *testing.T) {
	var capturedTraceID string

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tc := FromContext(r.Context())
		capturedTraceID = tc.TraceID
		w.WriteHeader(http.StatusOK)
	})

	wrapped := HTTPMiddleware(handler)

	// Case 1: Without incoming traceparent (generates new)
	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if capturedTraceID == "" {
		t.Errorf("expected middleware to generate trace ID")
	}
	if rec.Header().Get("X-Trace-ID") != capturedTraceID {
		t.Errorf("expected response header X-Trace-ID to match %s", capturedTraceID)
	}

	// Case 2: With incoming valid traceparent (preserves TraceID)
	existingTC := NewTraceContext()
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("traceparent", existingTC.TraceParent())
	rec2 := httptest.NewRecorder()
	wrapped.ServeHTTP(rec2, req2)

	if capturedTraceID != existingTC.TraceID {
		t.Errorf("expected middleware to preserve trace ID %s, got %s", existingTC.TraceID, capturedTraceID)
	}
}
