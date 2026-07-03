package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/vexarnetwork/vexpay/internal/invoice"
)

// Endpoint is a merchant's webhook destination and signing secret.
type Endpoint struct {
	URL    string
	Secret string
}

// Resolver maps a merchant to its webhook endpoint. Returning ok=false means the
// merchant has no webhook configured and the event is dropped.
type Resolver interface {
	Resolve(merchantID string) (Endpoint, bool)
}

// ResolverFunc adapts a function to Resolver.
type ResolverFunc func(merchantID string) (Endpoint, bool)

// Resolve implements Resolver.
func (f ResolverFunc) Resolve(merchantID string) (Endpoint, bool) { return f(merchantID) }

// Dispatcher delivers signed webhook events with retries. It implements
// invoice.Emitter.
type Dispatcher struct {
	client   *http.Client
	resolver Resolver

	maxAttempts int
	backoff     func(attempt int) time.Duration
	now         func() time.Time

	wg sync.WaitGroup
}

// Options configures a Dispatcher.
type Options struct {
	Resolver    Resolver
	Client      *http.Client
	MaxAttempts int
	// Backoff returns the delay before the given (1-based) retry attempt.
	Backoff func(attempt int) time.Duration
	Now     func() time.Time
}

// NewDispatcher constructs a Dispatcher.
func NewDispatcher(opts Options) *Dispatcher {
	client := opts.Client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	maxAttempts := opts.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	backoff := opts.Backoff
	if backoff == nil {
		backoff = func(attempt int) time.Duration {
			d := time.Duration(1<<uint(attempt-1)) * time.Second
			if d > 30*time.Second {
				d = 30 * time.Second
			}
			return d
		}
	}
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	return &Dispatcher{
		client:      client,
		resolver:    opts.Resolver,
		maxAttempts: maxAttempts,
		backoff:     backoff,
		now:         now,
	}
}

// Emit builds a signed event for the change and delivers it asynchronously.
func (d *Dispatcher) Emit(_ context.Context, change invoice.StatusChange) {
	ep, ok := d.resolver.Resolve(change.Invoice.MerchantID)
	if !ok || ep.URL == "" {
		return
	}
	event := NewEvent(change, d.now())
	body, err := json.Marshal(event)
	if err != nil {
		log.Printf("webhook: marshal event %s failed: %v", event.ID, err)
		return
	}

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		// Deliver on a detached context so a finished HTTP request doesn't
		// cancel an in-flight webhook.
		d.deliverWithRetry(context.Background(), ep, event.ID, body)
	}()
}

func (d *Dispatcher) deliverWithRetry(ctx context.Context, ep Endpoint, eventID string, body []byte) {
	for attempt := 1; attempt <= d.maxAttempts; attempt++ {
		if attempt > 1 {
			select {
			case <-time.After(d.backoff(attempt - 1)):
			case <-ctx.Done():
				return
			}
		}
		if err := d.deliverOnce(ctx, ep, body); err != nil {
			log.Printf("webhook: delivery %s attempt %d/%d failed: %v", eventID, attempt, d.maxAttempts, err)
			continue
		}
		return
	}
	log.Printf("webhook: delivery %s exhausted after %d attempts", eventID, d.maxAttempts)
}

func (d *Dispatcher) deliverOnce(ctx context.Context, ep Endpoint, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "VexPay-Webhooks/1")
	req.Header.Set(SignatureHeader, Sign(ep.Secret, d.now(), body))

	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("endpoint returned %d", resp.StatusCode)
	}
	return nil
}

// Close waits for in-flight deliveries to finish.
func (d *Dispatcher) Close() {
	d.wg.Wait()
}
