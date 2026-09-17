package telemetry

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"

	"google.golang.org/grpc/metadata"
)

type contextKey struct{}

var traceContextKey = contextKey{}

// TraceContext represents W3C TraceContext (traceparent)
type TraceContext struct {
	TraceID    string `json:"trace_id"`
	SpanID     string `json:"span_id"`
	TraceFlags string `json:"trace_flags"`
}

// NewTraceContext generates a new distributed trace context
func NewTraceContext() *TraceContext {
	traceIDBytes := make([]byte, 16)
	spanIDBytes := make([]byte, 8)
	_, _ = rand.Read(traceIDBytes)
	_, _ = rand.Read(spanIDBytes)

	return &TraceContext{
		TraceID:    hex.EncodeToString(traceIDBytes),
		SpanID:     hex.EncodeToString(spanIDBytes),
		TraceFlags: "01", // Sampled
	}
}

// NewChildSpan creates a child span preserving the TraceID
func (tc *TraceContext) NewChildSpan() *TraceContext {
	spanIDBytes := make([]byte, 8)
	_, _ = rand.Read(spanIDBytes)
	return &TraceContext{
		TraceID:    tc.TraceID,
		SpanID:     hex.EncodeToString(spanIDBytes),
		TraceFlags: tc.TraceFlags,
	}
}

// TraceParent serializes to W3C traceparent header format: 00-{trace_id}-{span_id}-{flags}
func (tc *TraceContext) TraceParent() string {
	flags := tc.TraceFlags
	if flags == "" {
		flags = "01"
	}
	return fmt.Sprintf("00-%s-%s-%s", tc.TraceID, tc.SpanID, flags)
}

// ParseTraceParent parses a W3C traceparent header string
func ParseTraceParent(header string) (*TraceContext, error) {
	header = strings.TrimSpace(header)
	parts := strings.Split(header, "-")
	if len(parts) != 4 || parts[0] != "00" {
		return nil, fmt.Errorf("invalid traceparent format: %s", header)
	}
	if len(parts[1]) != 32 || len(parts[2]) != 16 || len(parts[3]) != 2 {
		return nil, fmt.Errorf("invalid traceparent lengths: %s", header)
	}
	return &TraceContext{
		TraceID:    parts[1],
		SpanID:     parts[2],
		TraceFlags: parts[3],
	}, nil
}

// WithTraceContext returns a context containing the TraceContext
func WithTraceContext(ctx context.Context, tc *TraceContext) context.Context {
	return context.WithValue(ctx, traceContextKey, tc)
}

// FromContext extracts the TraceContext from context, or creates a new one
func FromContext(ctx context.Context) *TraceContext {
	if ctx == nil {
		return NewTraceContext()
	}
	if tc, ok := ctx.Value(traceContextKey).(*TraceContext); ok && tc != nil {
		return tc
	}
	return NewTraceContext()
}

// InjectHTTP writes the W3C traceparent and X-Trace-ID headers into an HTTP request
func InjectHTTP(ctx context.Context, header http.Header) {
	tc := FromContext(ctx)
	header.Set("traceparent", tc.TraceParent())
	header.Set("X-Trace-ID", tc.TraceID)
}

// ExtractHTTP extracts trace context from HTTP request headers or initializes a new one
func ExtractHTTP(r *http.Request) *TraceContext {
	if header := r.Header.Get("traceparent"); header != "" {
		if tc, err := ParseTraceParent(header); err == nil {
			return tc
		}
	}
	if traceID := r.Header.Get("X-Trace-ID"); traceID != "" {
		spanBytes := make([]byte, 8)
		_, _ = rand.Read(spanBytes)
		return &TraceContext{
			TraceID:    traceID,
			SpanID:     hex.EncodeToString(spanBytes),
			TraceFlags: "01",
		}
	}
	return NewTraceContext()
}

// InjectGRPC injects trace context into outgoing gRPC metadata
func InjectGRPC(ctx context.Context) context.Context {
	tc := FromContext(ctx)
	return metadata.AppendToOutgoingContext(ctx,
		"traceparent", tc.TraceParent(),
		"x-trace-id", tc.TraceID,
	)
}

// ExtractGRPC extracts trace context from incoming gRPC metadata
func ExtractGRPC(ctx context.Context) *TraceContext {
	md, ok := metadata.FromIncomingContext(ctx)
	if ok {
		if vals := md.Get("traceparent"); len(vals) > 0 {
			if tc, err := ParseTraceParent(vals[0]); err == nil {
				return tc
			}
		}
		if vals := md.Get("x-trace-id"); len(vals) > 0 {
			spanBytes := make([]byte, 8)
			_, _ = rand.Read(spanBytes)
			return &TraceContext{
				TraceID:    vals[0],
				SpanID:     hex.EncodeToString(spanBytes),
				TraceFlags: "01",
			}
		}
	}
	return NewTraceContext()
}

// HTTPMiddleware automatically extracts or generates trace context for incoming HTTP requests
func HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tc := ExtractHTTP(r)
		ctx := WithTraceContext(r.Context(), tc)

		// Set response headers for client tracing visibility
		w.Header().Set("traceparent", tc.TraceParent())
		w.Header().Set("X-Trace-ID", tc.TraceID)

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
