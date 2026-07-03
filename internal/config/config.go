// Package config loads VexPay runtime configuration from environment variables
// and an optional config file, applying sane defaults.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// Config holds all runtime settings for the gateway.
type Config struct {
	// Env is "development", "sandbox" or "production".
	Env string
	// HTTP listen address, e.g. ":8080".
	Addr string
	// PublicURL is the externally reachable base URL (used in checkout links,
	// payment URIs and webhook callbacks). No trailing slash.
	PublicURL string
	// DatabaseURL selects the store. "sqlite:vexpay.db" (default) or a
	// "postgres://..." DSN.
	DatabaseURL string
	// InvoiceExpiry is how long an invoice (and its locked rate) stays valid.
	InvoiceExpiry time.Duration
	// RequestTimeout bounds a single inbound HTTP request.
	RequestTimeout time.Duration
	// AdminSessionSecret signs admin dashboard sessions. Required in production.
	AdminSessionSecret string
}

// Default returns a Config populated with development-friendly defaults.
func Default() Config {
	return Config{
		Env:            "development",
		Addr:           ":8080",
		PublicURL:      "http://localhost:8080",
		DatabaseURL:    "sqlite:vexpay.db",
		InvoiceExpiry:  15 * time.Minute,
		RequestTimeout: 30 * time.Second,
	}
}

// Load builds a Config from defaults overlaid with VEXPAY_* environment
// variables, then validates it.
func Load() (Config, error) {
	c := Default()

	if v := env("ENV"); v != "" {
		c.Env = v
	}
	if v := env("ADDR"); v != "" {
		c.Addr = v
	}
	if v := env("PUBLIC_URL"); v != "" {
		c.PublicURL = strings.TrimRight(v, "/")
	}
	if v := env("DATABASE_URL"); v != "" {
		c.DatabaseURL = v
	}
	if v := env("ADMIN_SESSION_SECRET"); v != "" {
		c.AdminSessionSecret = v
	}
	if v := env("INVOICE_EXPIRY"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return c, fmt.Errorf("invalid VEXPAY_INVOICE_EXPIRY: %w", err)
		}
		c.InvoiceExpiry = d
	}
	if v := env("REQUEST_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return c, fmt.Errorf("invalid VEXPAY_REQUEST_TIMEOUT: %w", err)
		}
		c.RequestTimeout = d
	}

	if err := c.Validate(); err != nil {
		return c, err
	}
	return c, nil
}

// Validate checks that the configuration is internally consistent and safe for
// the selected environment.
func (c Config) Validate() error {
	if c.Addr == "" {
		return errors.New("addr must not be empty")
	}
	if c.PublicURL == "" {
		return errors.New("public_url must not be empty")
	}
	if c.InvoiceExpiry <= 0 {
		return errors.New("invoice_expiry must be positive")
	}
	if c.IsProduction() {
		if c.AdminSessionSecret == "" {
			return errors.New("admin_session_secret is required in production (set VEXPAY_ADMIN_SESSION_SECRET)")
		}
		if strings.HasPrefix(c.PublicURL, "http://") {
			return errors.New("public_url must use https in production")
		}
	}
	return nil
}

// IsProduction reports whether the gateway is running in production mode.
func (c Config) IsProduction() bool { return c.Env == "production" }

// env reads a VEXPAY_-prefixed environment variable.
func env(key string) string { return os.Getenv("VEXPAY_" + key) }
