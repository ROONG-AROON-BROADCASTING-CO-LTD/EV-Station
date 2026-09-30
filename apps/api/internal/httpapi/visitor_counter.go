package httpapi

import (
	"context"
	"net/http"
	"os"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// visitorCounter records every home-page visit. Redis keeps the total across
// API restarts; the mutex-backed value is a fallback for local development.
type visitorCounter struct {
	mu          sync.Mutex
	current     int64
	redisClient *redis.Client
	start       int64
}

func newVisitorCounter(start int64) *visitorCounter {
	counter := &visitorCounter{current: start, start: start}
	if rawURL := os.Getenv("REDIS_URL"); rawURL != "" {
		if options, err := redis.ParseURL(rawURL); err == nil {
			counter.redisClient = redis.NewClient(options)
		}
	}
	return counter
}

const visitorCountScript = `
if redis.call('EXISTS', KEYS[1]) == 0 then redis.call('SET', KEYS[1], ARGV[1]) end
return redis.call('INCR', KEYS[1])
`

func (v *visitorCounter) recordVisit(ctx context.Context) int64 {
	if v.redisClient != nil {
		ordinal, err := v.redisClient.Eval(ctx, visitorCountScript, []string{"rbc:page-visits"}, v.start).Int64()
		if err == nil {
			return ordinal
		}
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	v.current++
	return v.current
}

func (h *Handler) RegisterVisitorOrdinal(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"ordinal": h.visitors.recordVisit(c.Request.Context())}})
}
