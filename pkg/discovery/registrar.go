package discovery

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Registrar handles service registration and heartbeat renewal with the discovery backend.
type Registrar struct {
	catalog  Catalog
	self     Service
	interval time.Duration
	ttl      time.Duration
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewRegistrar creates a Registrar that will advertise the given service
// and renew its TTL by calling Health on the given interval.
func NewRegistrar(catalog Catalog, self Service, interval, ttl time.Duration) *Registrar {
	if interval <= 0 {
		interval = 10 * time.Second
	}
	if ttl <= 0 {
		ttl = 30 * time.Second
	}
	return &Registrar{
		catalog:  catalog,
		self:     self,
		interval: interval,
		ttl:      ttl,
		stopCh:   make(chan struct{}),
	}
}

// Start registers the service and begins background heartbeat renewal.
// It returns an error if registration fails; if nil the caller must call Stop.
func (r *Registrar) Start(ctx context.Context) error {
	if err := r.catalog.Register(ctx, r.self); err != nil {
		return fmt.Errorf("discovery: registrar: register %s: %w", r.self.ID, err)
	}
	r.wg.Add(1)
	go r.heartbeat(ctx)
	return nil
}

// Stop deregisters the service and waits for the heartbeat goroutine to exit.
func (r *Registrar) Stop(ctx context.Context) {
	close(r.stopCh)
	_ = r.catalog.Deregister(ctx, r.self.ID)
	r.wg.Wait()
}

// heartbeat periodically refreshes the service TTL until Stop is called.
func (r *Registrar) heartbeat(ctx context.Context) {
	defer r.wg.Done()
	ticker := time.NewTicker(r.interval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ctx.Done():
			_ = r.catalog.Deregister(context.Background(), r.self.ID)
			return
		case <-ticker.C:
			if err := r.catalog.Health(ctx, r.self.ID); err != nil {
				log.Printf("[discovery] heartbeat failed for %s: %v", r.self.ID, err)
			}
		}
	}
}
