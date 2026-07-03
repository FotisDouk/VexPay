// Package app assembles the VexPay gateway from its components: store, chain
// adapters, invoice service, webhooks, auth, HTTP handler and watcher. Both the
// binary and end-to-end tests build the app the same way.
package app

import (
	"net/http"

	"github.com/vexarnetwork/vexpay/internal/api"
	"github.com/vexarnetwork/vexpay/internal/auth"
	"github.com/vexarnetwork/vexpay/internal/chain"
	"github.com/vexarnetwork/vexpay/internal/chain/btc"
	"github.com/vexarnetwork/vexpay/internal/chain/mock"
	"github.com/vexarnetwork/vexpay/internal/config"
	"github.com/vexarnetwork/vexpay/internal/invoice"
	"github.com/vexarnetwork/vexpay/internal/store"
	"github.com/vexarnetwork/vexpay/internal/watcher"
	"github.com/vexarnetwork/vexpay/internal/webhook"
)

// DefaultMerchantID is used until multi-merchant support lands.
const DefaultMerchantID = "default"

// App is a fully-wired gateway.
type App struct {
	Config     config.Config
	Store      store.Store
	Registry   *chain.Registry
	Invoices   *invoice.Service
	Auth       *auth.Store
	Sandbox    *mock.Adapter
	Dispatcher *webhook.Dispatcher
	Watcher    *watcher.Watcher
	Handler    http.Handler

	// SeededSandboxKey is a freshly generated sandbox key, set only when none
	// was configured and the gateway is not in production. The binary logs it so
	// the operator can start testing immediately.
	SeededSandboxKey string
}

// Build wires the application from configuration.
func Build(cfg config.Config) (*App, error) {
	st, err := store.Open(cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}

	registry := chain.NewRegistry()
	var sandbox *mock.Adapter
	if cfg.EnableSandbox {
		sandbox = mock.New(cfg.SandboxConfirmations)
		if err := registry.Register(sandbox); err != nil {
			return nil, err
		}
	}
	if cfg.EnableBitcoin {
		mainnet := btc.New(btc.Mainnet, btc.NewMempoolBackend(cfg.BTCExplorerURL), 0)
		testnet := btc.New(btc.Testnet, btc.NewMempoolBackend(cfg.BTCTestnetExplorerURL), 0)
		if err := registry.Register(mainnet); err != nil {
			return nil, err
		}
		if err := registry.Register(testnet); err != nil {
			return nil, err
		}
	}

	resolver := webhook.ResolverFunc(func(string) (webhook.Endpoint, bool) {
		if cfg.WebhookURL == "" {
			return webhook.Endpoint{}, false
		}
		return webhook.Endpoint{URL: cfg.WebhookURL, Secret: cfg.WebhookSecret}, true
	})
	dispatcher := webhook.NewDispatcher(webhook.Options{Resolver: resolver})

	invoices := invoice.NewService(invoice.Options{
		Repo:     st.Invoices(),
		Registry: registry,
		Emitter:  dispatcher,
		Expiry:   cfg.InvoiceExpiry,
	})

	authStore := auth.NewStore()
	var seededSandbox string
	if cfg.APIKey != "" {
		authStore.Add(auth.NewKey(cfg.APIKey, DefaultMerchantID, false))
	}
	if sandbox != nil {
		switch {
		case cfg.SandboxAPIKey != "":
			authStore.Add(auth.NewKey(cfg.SandboxAPIKey, DefaultMerchantID, true))
		case !cfg.IsProduction():
			raw, key := auth.GenerateKey(DefaultMerchantID, true)
			authStore.Add(key)
			seededSandbox = raw
		}
	}

	handler := api.New(api.Deps{
		Config:   cfg,
		Store:    st,
		Invoices: invoices,
		Chains:   registry,
		Auth:     authStore,
		Sandbox:  sandbox,
	}).Handler()

	return &App{
		Config:           cfg,
		Store:            st,
		Registry:         registry,
		Invoices:         invoices,
		Auth:             authStore,
		Sandbox:          sandbox,
		Dispatcher:       dispatcher,
		Watcher:          watcher.New(invoices, cfg.WatchInterval),
		Handler:          handler,
		SeededSandboxKey: seededSandbox,
	}, nil
}

// Close releases resources: it drains in-flight webhooks and closes the store.
func (a *App) Close() error {
	a.Dispatcher.Close()
	return a.Store.Close()
}
