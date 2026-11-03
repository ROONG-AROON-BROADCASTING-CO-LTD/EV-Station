package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net"
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
	mu             sync.Mutex
	buckets        map[[32]byte]authBucket
	now            func() time.Time
	trustedProxies []*net.IPNet
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
			// Saturation must not turn one source's traffic into a global lockout.
			var oldest [32]byte
			var expiry time.Time
			for key, entry := range l.buckets {
				if expiry.IsZero() || entry.expires.Before(expiry) {
					oldest, expiry = key, entry.expires
				}
			}
			delete(l.buckets, oldest)
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
		source := l.requestSource(c.Request)
		if !l.allow("source:"+source, 120, time.Minute) {
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
		accountKey := ""
		if json.Unmarshal(body, &input) == nil && strings.TrimSpace(input.Email) != "" {
			email := strings.ToLower(strings.TrimSpace(input.Email))
			accountKey = c.FullPath() + ":" + source + ":" + email
			if !l.allow(accountKey, 10, 15*time.Minute) {
				authThrottled(c)
				return
			}
		}
		c.Next()
		// Successful logins must not accumulate a failed-attempt lockout. Keep
		// the source-wide request budget and OTP delivery limits in place.
		if c.FullPath() == "/api/v1/auth/login" && c.Writer.Status() == http.StatusOK && accountKey != "" {
			l.mu.Lock()
			delete(l.buckets, sha256.Sum256([]byte(accountKey)))
			l.mu.Unlock()
		}
	}
}

func (l *authLimiter) trustProxies(cidrs string) {
	for _, raw := range strings.Split(cidrs, ",") {
		if _, network, err := net.ParseCIDR(strings.TrimSpace(raw)); err == nil {
			l.trustedProxies = append(l.trustedProxies, network)
		}
	}
}

func (l *authLimiter) requestSource(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(host)
	// nginx overwrites this header. Trust it only from explicitly configured
	// proxy peers; never use caller-supplied X-Forwarded-For implicitly.
	for _, network := range l.trustedProxies {
		if network.Contains(peer) {
			if ip := net.ParseIP(r.Header.Get("X-RBC-Client-IP")); ip != nil {
				return ip.String()
			}
		}
	}
	if peer != nil {
		return peer.String()
	}
	return "unknown"
}

func authThrottled(c *gin.Context) {
	c.Header("Retry-After", "900")
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": gin.H{"code": "AUTH_RATE_LIMITED", "message": "Too many authentication attempts. Please try again later."}})
}
