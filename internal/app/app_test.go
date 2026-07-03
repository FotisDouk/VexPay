package app

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/vexarnetwork/vexpay/internal/config"
	"github.com/vexarnetwork/vexpay/internal/webhook"
)

// TestSandboxEndToEnd drives the whole pipeline over HTTP: create an invoice,
// simulate an on-chain payment, and assert the invoice settles and a verifiable
// signed webhook is delivered.
func TestSandboxEndToEnd(t *testing.T) {
	const sandboxKey = "vpk_test_e2ekey"
	const webhookSecret = "whsec_e2e"

	// Webhook receiver that verifies the signature and records the event.
	var mu sync.Mutex
	var received []webhook.Event
	hookSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if err := webhook.Verify(webhookSecret, r.Header.Get(webhook.SignatureHeader), body, time.Minute); err != nil {
			t.Errorf("webhook signature invalid: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var ev webhook.Event
		if err := json.Unmarshal(body, &ev); err != nil {
			t.Errorf("decode webhook: %v", err)
		}
		mu.Lock()
		received = append(received, ev)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer hookSrv.Close()

	cfg := config.Default()
	cfg.DatabaseURL = "memory:"
	cfg.EnableSandbox = true
	cfg.SandboxConfirmations = 1
	cfg.SandboxAPIKey = sandboxKey
	cfg.WebhookURL = hookSrv.URL
	cfg.WebhookSecret = webhookSecret

	application, err := Build(cfg)
	if err != nil {
		t.Fatalf("build app: %v", err)
	}
	defer application.Close()

	srv := httptest.NewServer(application.Handler)
	defer srv.Close()
	client := srv.Client()

	// Unauthenticated request is rejected.
	resp := doJSON(t, client, http.MethodPost, srv.URL+"/v1/invoices", "", `{}`)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauth create status = %d, want 401", resp.StatusCode)
	}

	// Create an invoice.
	createBody := `{"chain":"mock","asset":"tBTC","amount":"0.001","metadata":{"order":"1234"}}`
	resp = doJSON(t, client, http.MethodPost, srv.URL+"/v1/invoices", sandboxKey, createBody)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status = %d, want 201", resp.StatusCode)
	}
	var created invoiceView
	decode(t, resp, &created)
	if created.Status != "pending" {
		t.Fatalf("created status = %s, want pending", created.Status)
	}
	if created.ReceiveAddress == "" || created.PaymentURI == "" {
		t.Fatalf("expected receive address and payment URI, got %+v", created)
	}

	// Fetch it back.
	resp = doJSON(t, client, http.MethodGet, srv.URL+"/v1/invoices/"+created.ID, sandboxKey, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("get status = %d, want 200", resp.StatusCode)
	}

	// Simulate payment.
	resp = doJSON(t, client, http.MethodPost, srv.URL+"/v1/sandbox/pay/"+created.ID, sandboxKey, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("pay status = %d, want 200", resp.StatusCode)
	}
	var paid invoiceView
	decode(t, resp, &paid)
	if paid.Status != "paid" {
		t.Fatalf("after pay status = %s, want paid", paid.Status)
	}

	// Drain webhooks and assert one verified invoice.paid event arrived.
	application.Dispatcher.Close()
	mu.Lock()
	defer mu.Unlock()
	var gotPaid bool
	for _, ev := range received {
		if ev.Type == "invoice.paid" && ev.Data.ID == created.ID {
			gotPaid = true
		}
	}
	if !gotPaid {
		t.Fatalf("expected a verified invoice.paid webhook, got %d events", len(received))
	}
}

func TestInvoiceQRReturnsPNG(t *testing.T) {
	const sandboxKey = "vpk_test_qr"
	cfg := config.Default()
	cfg.DatabaseURL = "memory:"
	cfg.SandboxAPIKey = sandboxKey

	application, err := Build(cfg)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	defer application.Close()

	srv := httptest.NewServer(application.Handler)
	defer srv.Close()
	client := srv.Client()

	resp := doJSON(t, client, http.MethodPost, srv.URL+"/v1/invoices", sandboxKey,
		`{"chain":"mock","asset":"tBTC","amount":"0.001"}`)
	var created invoiceView
	decode(t, resp, &created)

	qrResp := doJSON(t, client, http.MethodGet, srv.URL+"/v1/invoices/"+created.ID+"/qr?size=128", sandboxKey, "")
	defer qrResp.Body.Close()
	if qrResp.StatusCode != http.StatusOK {
		t.Fatalf("qr status = %d, want 200", qrResp.StatusCode)
	}
	if ct := qrResp.Header.Get("Content-Type"); ct != "image/png" {
		t.Fatalf("qr content-type = %q, want image/png", ct)
	}
	png, _ := io.ReadAll(qrResp.Body)
	// PNG magic number.
	if len(png) < 8 || string(png[1:4]) != "PNG" {
		t.Fatalf("response is not a PNG (len=%d)", len(png))
	}
}

type invoiceView struct {
	ID             string `json:"id"`
	Status         string `json:"status"`
	ReceiveAddress string `json:"receive_address"`
	PaymentURI     string `json:"payment_uri"`
}

func doJSON(t *testing.T, c *http.Client, method, url, key, body string) *http.Response {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = bytes.NewReader([]byte(body))
	}
	req, err := http.NewRequest(method, url, r)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if key != "" {
		req.Header.Set("Authorization", "Bearer "+key)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	return resp
}

func decode(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}
