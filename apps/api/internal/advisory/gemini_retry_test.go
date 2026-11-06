package advisory

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rbc/ev-station/apps/api/internal/domain"
)

func writeRetryAssessment(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"candidates": []any{map[string]any{"content": map[string]any{"parts": []any{map[string]any{"text": `{"summary":"Evidence checked.","decision":"not_recommended","recommendation":"Review the site.","strengths":[],"risks":[],"requiredChecks":[],"disclaimer":"Human review is required."}`}}}}}})
}

func TestGenerateRetriesTransientFailureWithSameRequest(t *testing.T) {
	var requests atomic.Int32
	var original atomic.Value
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if requests.Add(1) == 1 {
			original.Store(string(body))
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		if string(body) != original.Load().(string) || r.Header.Get("x-goog-api-key") != "test-key" {
			t.Error("retry must replay the same server-side request")
		}
		writeRetryAssessment(w)
	}))
	defer server.Close()
	service := NewGeminiService(GeminiConfig{APIKey: "test-key", Model: "test-model", BaseURL: server.URL, Timeout: 5 * time.Second}, server.Client())
	result, err := service.Generate(context.Background(), domain.AnalysisRun{}, "en")
	if err != nil || result.Model != "test-model" || requests.Load() != 2 {
		t.Fatalf("transient 503 should recover automatically: requests=%d result=%+v error=%v", requests.Load(), result, err)
	}
}

func TestGenerateRetryAttemptsAreBounded(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	service := NewGeminiService(GeminiConfig{APIKey: "test-key", BaseURL: server.URL, Timeout: 5 * time.Second}, server.Client())
	_, err := service.Generate(context.Background(), domain.AnalysisRun{}, "en")
	if !errors.Is(err, ErrUnavailable) || requests.Load() != 3 {
		t.Fatalf("expected exactly three attempts before fallback: requests=%d error=%v", requests.Load(), err)
	}
}

func TestGenerateDoesNotRetryPermanentOrInvalidResponses(t *testing.T) {
	for _, status := range []int{400, 401, 402, 403, 404, 200} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(status)
				_, _ = io.WriteString(w, `{"invalid":"output"}`)
			}))
			defer server.Close()
			service := NewGeminiService(GeminiConfig{APIKey: "test-key", BaseURL: server.URL, Timeout: time.Second}, server.Client())
			_, err := service.Generate(context.Background(), domain.AnalysisRun{}, "en")
			if err == nil || requests.Load() != 1 {
				t.Fatalf("permanent/model-output errors must not retry: requests=%d error=%v", requests.Load(), err)
			}
		})
	}
}

func TestGenerateHonorsRetryAfterAndDeadline(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.Header().Set("Retry-After", "60")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer server.Close()
	service := NewGeminiService(GeminiConfig{APIKey: "test-key", BaseURL: server.URL, Timeout: 80 * time.Millisecond}, server.Client())
	start := time.Now()
	_, err := service.Generate(context.Background(), domain.AnalysisRun{}, "en")
	if !errors.Is(err, ErrUnavailable) || requests.Load() != 1 || time.Since(start) > 400*time.Millisecond {
		t.Fatalf("must not retry before Retry-After or exceed total budget: requests=%d elapsed=%s error=%v", requests.Load(), time.Since(start), err)
	}
}

func TestGenerateWaitsForRetryAfterBeforeRecovering(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if requests.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		writeRetryAssessment(w)
	}))
	defer server.Close()
	service := NewGeminiService(GeminiConfig{APIKey: "test-key", BaseURL: server.URL, Timeout: 3 * time.Second}, server.Client())
	start := time.Now()
	_, err := service.Generate(context.Background(), domain.AnalysisRun{}, "en")
	if err != nil || requests.Load() != 2 || time.Since(start) < time.Second {
		t.Fatalf("Retry-After must delay the next request: requests=%d elapsed=%s error=%v", requests.Load(), time.Since(start), err)
	}
}

func TestGenerateCanCancelDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
		time.AfterFunc(40*time.Millisecond, cancel)
	}))
	defer server.Close()
	service := NewGeminiService(GeminiConfig{APIKey: "test-key", BaseURL: server.URL, Timeout: 5 * time.Second}, server.Client())
	start := time.Now()
	_, err := service.Generate(ctx, domain.AnalysisRun{}, "en")
	if !errors.Is(err, context.Canceled) || requests.Load() != 1 || time.Since(start) > 400*time.Millisecond {
		t.Fatalf("backoff must be cancellable: requests=%d elapsed=%s error=%v", requests.Load(), time.Since(start), err)
	}
}

func TestNarrativeRetryAfterHTTPDate(t *testing.T) {
	now := time.Date(2026, 11, 5, 0, 0, 0, 0, time.UTC)
	if got := narrativeRetryAfter(now.Add(3*time.Second).Format(http.TimeFormat), now); got != 3*time.Second {
		t.Fatalf("HTTP-date Retry-After was not respected: %s", got)
	}
	for _, value := range []string{"-1", "invalid", now.Add(-time.Second).Format(http.TimeFormat)} {
		if got := narrativeRetryAfter(value, now); got != 0 {
			t.Fatalf("invalid/past Retry-After should not delay: %q -> %s", value, got)
		}
	}
}

func TestGenerateEnforcesTotalTimeoutWithCustomHTTPClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(150 * time.Millisecond):
			writeRetryAssessment(w)
		}
	}))
	defer server.Close()
	service := NewGeminiService(GeminiConfig{APIKey: "test-key", BaseURL: server.URL, Timeout: 25 * time.Millisecond}, server.Client())
	start := time.Now()
	_, err := service.Generate(context.Background(), domain.AnalysisRun{}, "en")
	if !errors.Is(err, ErrUnavailable) || time.Since(start) > 100*time.Millisecond {
		t.Fatalf("configured timeout must cover all attempts even with a supplied client: elapsed=%s error=%v", time.Since(start), err)
	}
}

func TestGenerateCancelledContextDoesNotRetry(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	service := NewGeminiService(GeminiConfig{APIKey: "test-key", BaseURL: "http://127.0.0.1:1", Timeout: time.Second}, nil)
	_, err := service.Generate(ctx, domain.AnalysisRun{}, "en")
	if !errors.Is(err, ErrUnavailable) || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("expected a prompt cancellation failure, got %v", err)
	}
}
