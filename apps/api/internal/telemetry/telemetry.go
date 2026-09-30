package telemetry

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"time"
)

type Event struct {
	AnalysisID          string
	Provider            string
	Operation           string
	Endpoint            string
	Category            string
	StartedAt           time.Time
	FinishedAt          time.Time
	DurationMS          int64
	Status              string
	ErrorType           string
	CacheHit            *bool
	HTTPStatus          int
	RetryCount          int
	PollCount           int
	SelectedEndpoint    string
	EndpointHealth      string
	FallbackTriggered   bool
	FallbackReason      string
	CooldownRemainingMS int64
	CacheKeyFingerprint string
	CacheTTLSeconds     int64
	CacheMissReason     string
}

type Recorder interface{ Record(Event) }
type noopRecorder struct{}

func (noopRecorder) Record(Event) {}
func Noop() Recorder              { return noopRecorder{} }

type SlogRecorder struct{ logger *slog.Logger }

func NewSlogRecorder(logger *slog.Logger) Recorder { return &SlogRecorder{logger: logger} }
func (r *SlogRecorder) Record(e Event) {
	if r == nil || r.logger == nil {
		return
	}
	attrs := []any{
		"analysis_id", e.AnalysisID,
		"provider", e.Provider,
		"operation", e.Operation,
		"started_at", e.StartedAt,
		"finished_at", e.FinishedAt,
		"duration_ms", e.DurationMS,
		"status", e.Status,
		"error_type", e.ErrorType,
		"http_status", e.HTTPStatus,
		"retry_count", e.RetryCount,
		"poll_count", e.PollCount,
	}
	if e.Endpoint != "" {
		attrs = append(attrs, "endpoint", e.Endpoint)
	}
	if e.Category != "" {
		attrs = append(attrs, "category", e.Category)
	}
	if e.CacheHit != nil {
		attrs = append(attrs, "cache_hit", *e.CacheHit)
	}
	if e.SelectedEndpoint != "" {
		attrs = append(attrs, "selected_endpoint", e.SelectedEndpoint)
	}
	if e.EndpointHealth != "" {
		attrs = append(attrs, "endpoint_health_before_request", e.EndpointHealth)
	}
	if e.FallbackTriggered {
		attrs = append(attrs, "fallback_triggered", true)
	}
	if e.FallbackReason != "" {
		attrs = append(attrs, "fallback_reason", e.FallbackReason)
	}
	if e.CooldownRemainingMS > 0 {
		attrs = append(attrs, "cooldown_remaining_ms", e.CooldownRemainingMS)
	}
	if e.CacheKeyFingerprint != "" {
		attrs = append(attrs, "cache_key_fingerprint", e.CacheKeyFingerprint)
	}
	if e.CacheTTLSeconds > 0 {
		attrs = append(attrs, "cache_ttl_seconds", e.CacheTTLSeconds)
	}
	if e.CacheMissReason != "" {
		attrs = append(attrs, "cache_miss_reason", e.CacheMissReason)
	}
	r.logger.Info("analysis_provider_telemetry", attrs...)
}

type contextKey string

const (
	analysisIDKey contextKey = "analysis-telemetry-id"
	operationKey  contextKey = "analysis-telemetry-operation"
	categoryKey   contextKey = "analysis-telemetry-category"
	recorderKey   contextKey = "analysis-telemetry-recorder"
)

func WithAnalysisID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, analysisIDKey, id)
}

func WithOperation(ctx context.Context, operation string) context.Context {
	return context.WithValue(ctx, operationKey, operation)
}

func WithCategory(ctx context.Context, category string) context.Context {
	return context.WithValue(ctx, categoryKey, category)
}

func WithRecorder(ctx context.Context, recorder Recorder) context.Context {
	if recorder == nil {
		recorder = Noop()
	}
	return context.WithValue(ctx, recorderKey, recorder)
}

func AnalysisID(ctx context.Context) string {
	value, _ := ctx.Value(analysisIDKey).(string)
	return value
}

func Operation(ctx context.Context) string {
	value, _ := ctx.Value(operationKey).(string)
	return value
}

func Category(ctx context.Context) string {
	value, _ := ctx.Value(categoryKey).(string)
	return value
}

func RecorderFromContext(ctx context.Context) Recorder {
	recorder, _ := ctx.Value(recorderKey).(Recorder)
	if recorder == nil {
		return Noop()
	}
	return recorder
}

func Record(ctx context.Context, event Event) {
	if event.AnalysisID == "" {
		event.AnalysisID = AnalysisID(ctx)
	}
	RecorderFromContext(ctx).Record(event)
}

func Endpoint(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return parsed.Host
}
func ErrorType(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "cancelled"
	}
	var networkErr net.Error
	if errors.As(err, &networkErr) && networkErr.Timeout() {
		return "timeout"
	}
	if strings.Contains(strings.ToLower(err.Error()), "timeout") {
		return "timeout"
	}
	return "error"
}
