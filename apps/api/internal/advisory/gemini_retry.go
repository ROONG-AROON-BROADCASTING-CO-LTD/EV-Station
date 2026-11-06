package advisory

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const narrativeMaxAttempts = 3

// Retries share one timeout budget. A transient provider outage must not turn a
// 20-second optional narrative into three full-length requests or an endless loop.
func (s *Service) generateNarrativeContent(parent context.Context, body []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(parent, s.config.Timeout)
	defer cancel()
	endpoint := strings.TrimRight(s.config.BaseURL, "/") + "/models/" + url.PathEscape(s.config.Model) + ":generateContent"
	for attempt := 0; attempt < narrativeMaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrUnavailable, err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-goog-api-key", s.config.APIKey)
		response, err := s.client.Do(req)
		var retry bool
		var retryAfter time.Duration
		var lastErr error
		if err != nil {
			lastErr = fmt.Errorf("%w: %w", ErrUnavailable, err)
			retry = transientNarrativeTransportError(err)
		} else {
			responseBody, readErr := io.ReadAll(io.LimitReader(response.Body, 1<<20))
			response.Body.Close()
			retryAfter = narrativeRetryAfter(response.Header.Get("Retry-After"), time.Now())
			if readErr != nil {
				lastErr = fmt.Errorf("%w: %w", ErrUnavailable, readErr)
				retry = transientNarrativeTransportError(readErr)
			} else if response.StatusCode >= 200 && response.StatusCode < 300 {
				return responseBody, nil
			} else {
				lastErr = fmt.Errorf("%w: Gemini returned HTTP %d", ErrUnavailable, response.StatusCode)
				retry = transientNarrativeStatus(response.StatusCode)
			}
		}
		if !retry || attempt+1 == narrativeMaxAttempts {
			return nil, lastErr
		}
		base := 500 * time.Millisecond * time.Duration(1<<attempt)
		delay := base + time.Duration(rand.Int64N(int64(base/2)))
		if retryAfter > delay {
			delay = retryAfter
		}
		// Never shorten Retry-After to squeeze another call inside the deadline.
		if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= delay {
			return nil, lastErr
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, fmt.Errorf("%w: %w", ErrUnavailable, ctx.Err())
		case <-timer.C:
		}
	}
	return nil, ErrUnavailable
}

func transientNarrativeStatus(status int) bool {
	switch status {
	case http.StatusRequestTimeout, http.StatusTooManyRequests, http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

func transientNarrativeTransportError(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var networkError net.Error
	return errors.As(err, &networkError) && (networkError.Timeout() || networkError.Temporary())
}

func narrativeRetryAfter(value string, now time.Time) time.Duration {
	if seconds, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64); err == nil && seconds > 0 {
		// A day already exceeds our bounded request budget; avoid duration overflow.
		if seconds > 86400 {
			return 24 * time.Hour
		}
		return time.Duration(seconds) * time.Second
	}
	if retryAt, err := http.ParseTime(value); err == nil && retryAt.After(now) {
		return retryAt.Sub(now)
	}
	return 0
}
