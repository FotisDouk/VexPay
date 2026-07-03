// Package watcher periodically syncs open invoices against their chains so
// payments confirm automatically without any merchant action.
package watcher

import (
	"context"
	"log"
	"time"

	"github.com/vexarnetwork/vexpay/internal/invoice"
)

// Watcher drives invoice.Service.ProcessOpen on a fixed interval.
type Watcher struct {
	svc      *invoice.Service
	interval time.Duration
}

// New creates a Watcher. A non-positive interval defaults to 15s.
func New(svc *invoice.Service, interval time.Duration) *Watcher {
	if interval <= 0 {
		interval = 15 * time.Second
	}
	return &Watcher{svc: svc, interval: interval}
}

// Run polls until the context is cancelled.
func (w *Watcher) Run(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	log.Printf("watcher started (interval=%s)", w.interval)
	for {
		select {
		case <-ctx.Done():
			log.Println("watcher stopped")
			return
		case <-ticker.C:
			if err := w.svc.ProcessOpen(ctx); err != nil {
				log.Printf("watcher: process open failed: %v", err)
			}
		}
	}
}
