package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type authBucket struct {
	count   int
	expires time.Time
}
type authLimiter struct {
	mu      sync.Mutex
	buckets map[[32]byte]authBucket
	now     func() time.Time
}

func newAuthLimiter() *authLimiter {
	return &authLimiter{buckets: make(map[[32]byte]authBucket), now: time.Now}
}

// Bounded process-local counters protect this single-instance deployment.
// Keys are hashes; email addresses and request bodies are never retained.
func (l *authLimiter) allow(key string, limit int, window time.Duration) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	for k, b := range l.buckets {
		if !now.Before(b.expires) {
			delete(l.buckets, k)
		}
	}
	hash := sha256.Sum256([]byte(key))
	b, exists := l.buckets[hash]
	if !exists {
		if len(l.buckets) >= 10000 {
			return false
		}
		b.expires = now.Add(window)
	}
	if b.count >= limit {
		return false
	}
	b.count++
	l.buckets[hash] = b
	return true
}

func (l *authLimiter) middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// A global ceiling also bounds requests with randomized addresses. Do not
		// trust forwarded client IP headers from a publicly reachable request.
		if !l.allow("all", 120, time.Minute) {
			authThrottled(c)
			return
		}
		body, err := io.ReadAll(io.LimitReader(c.Request.Body, 16385))
		if err != nil || len(body) > 16384 {
			c.AbortWithStatus(http.StatusRequestEntityTooLarge)
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
		var input struct {
			Email string `json:"email"`
		}
		if json.Unmarshal(body, &input) == nil && strings.TrimSpace(input.Email) != "" {
			email := strings.ToLower(strings.TrimSpace(input.Email))
			if !l.allow(c.FullPath()+":"+email, 10, 15*time.Minute) {
				authThrottled(c)
				return
			}
		}
		c.Next()
	}
}

func authThrottled(c *gin.Context) {
	c.Header("Retry-After", "900")
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": gin.H{"code": "AUTH_RATE_LIMITED", "message": "Too many authentication attempts. Please try again later."}})
}
