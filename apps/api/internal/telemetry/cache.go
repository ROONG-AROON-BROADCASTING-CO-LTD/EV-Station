package telemetry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/rbc/ev-station/apps/api/internal/cache"
	"time"
)

type observedCache struct {
	cache.Cache
	provider string
	recorder Recorder
}

func WrapCache(inner cache.Cache, provider string, recorder Recorder) cache.Cache {
	if inner == nil {
		inner = cache.Noop{}
	}
	if recorder == nil {
		recorder = Noop()
	}
	return observedCache{Cache: inner, provider: provider, recorder: recorder}
}
func (c observedCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	started := time.Now().UTC()
	value, hit, err := c.Cache.Get(ctx, key)
	finished := time.Now().UTC()
	status := "success"
	if err != nil {
		status = "error"
	}
	missReason := ""
	if err != nil {
		missReason = "backend_error"
	} else if !hit {
		missReason = "not_found"
	}
	c.recorder.Record(Event{AnalysisID: AnalysisID(ctx), Provider: c.provider, Operation: "cache_get", Category: Category(ctx), StartedAt: started, FinishedAt: finished, DurationMS: finished.Sub(started).Milliseconds(), Status: status, ErrorType: ErrorType(err), CacheHit: &hit, CacheKeyFingerprint: cacheKeyFingerprint(key), CacheMissReason: missReason})
	return value, hit, err
}

func (c observedCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	started := time.Now().UTC()
	err := c.Cache.Set(ctx, key, value, ttl)
	finished := time.Now().UTC()
	status := "success"
	if err != nil {
		status = "error"
	}
	c.recorder.Record(Event{AnalysisID: AnalysisID(ctx), Provider: c.provider, Operation: "cache_set", Category: Category(ctx), StartedAt: started, FinishedAt: finished, DurationMS: finished.Sub(started).Milliseconds(), Status: status, ErrorType: ErrorType(err), CacheKeyFingerprint: cacheKeyFingerprint(key), CacheTTLSeconds: int64(ttl.Seconds())})
	return err
}

func cacheKeyFingerprint(key string) string {
	hash := sha256.Sum256([]byte(key))
	return hex.EncodeToString(hash[:8])
}
