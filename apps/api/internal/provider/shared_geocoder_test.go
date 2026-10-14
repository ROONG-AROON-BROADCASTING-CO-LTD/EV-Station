package provider

import (
	"context"
	"sync/atomic"
	"testing"
)

type heldGeocoder struct {
	calls   atomic.Int32
	started chan struct{}
	release chan struct{}
}

func (g *heldGeocoder) Reverse(ctx context.Context, _, _ float64) (ReverseGeocodingResult, error) {
	g.calls.Add(1)
	close(g.started)
	select {
	case <-ctx.Done():
		return ReverseGeocodingResult{}, ctx.Err()
	case <-g.release:
	}
	return ReverseGeocodingResult{Province: "เชียงใหม่"}, nil
}
func TestSharedReverseCancellationDoesNotDiscardOtherCaller(t *testing.T) {
	underlying := &heldGeocoder{started: make(chan struct{}), release: make(chan struct{})}
	g := NewSharedReverseGeocoder(underlying)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := g.Reverse(ctx, 18, 99); done <- err }()
	<-underlying.started
	cancel()
	if err := <-done; err != context.Canceled {
		t.Fatalf("cancel error: %v", err)
	}
	// Still-running lookup remains registered even after the first caller exits.
	g.mu.Lock()
	lookup := g.inflight["18.0000000|99.0000000"]
	g.mu.Unlock()
	if lookup == nil {
		t.Fatal("shared lookup discarded")
	}
	close(underlying.release)
	<-lookup.done
	if lookup.err != nil || lookup.location.Province != "เชียงใหม่" || underlying.calls.Load() != 1 {
		t.Fatalf("shared lookup failed: %+v", lookup)
	}
}
