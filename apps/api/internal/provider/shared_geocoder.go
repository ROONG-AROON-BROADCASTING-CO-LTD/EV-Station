package provider

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// SharedReverseGeocoder coalesces simultaneous administrative lookups for the
// same pin. The underlying geocoder retains its cache and public rate limiter.
type SharedReverseGeocoder struct {
	geocoder ReverseGeocoder
	mu       sync.Mutex
	inflight map[string]*reverseLookup
}

type reverseLookup struct {
	done     chan struct{}
	location ReverseGeocodingResult
	err      error
}

func NewSharedReverseGeocoder(geocoder ReverseGeocoder) *SharedReverseGeocoder {
	return &SharedReverseGeocoder{geocoder: geocoder, inflight: make(map[string]*reverseLookup)}
}

func (g *SharedReverseGeocoder) Reverse(ctx context.Context, lat, lon float64) (ReverseGeocodingResult, error) {
	key := fmt.Sprintf("%.7f|%.7f", lat, lon)
	g.mu.Lock()
	lookup, exists := g.inflight[key]
	if !exists {
		lookup = &reverseLookup{done: make(chan struct{})}
		g.inflight[key] = lookup
		go func() {
			// One caller cancelling must not cancel another caller's lookup.
			requestCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			lookup.location, lookup.err = g.geocoder.Reverse(requestCtx, lat, lon)
			g.mu.Lock()
			delete(g.inflight, key)
			close(lookup.done)
			g.mu.Unlock()
		}()
	}
	g.mu.Unlock()
	select {
	case <-ctx.Done():
		return ReverseGeocodingResult{}, ctx.Err()
	case <-lookup.done:
		return lookup.location, lookup.err
	}
}
