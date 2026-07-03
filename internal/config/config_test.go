package config

import (
	"testing"
	"time"
)

func TestValidateProductionRequirements(t *testing.T) {
	base := Default()
	base.Env = "production"
	base.PublicURL = "https://pay.example.com"

	t.Run("missing session secret fails", func(t *testing.T) {
		c := base
		if err := c.Validate(); err == nil {
			t.Fatal("expected error when admin session secret is empty in production")
		}
	})

	t.Run("http public url fails", func(t *testing.T) {
		c := base
		c.AdminSessionSecret = "secret"
		c.PublicURL = "http://pay.example.com"
		if err := c.Validate(); err == nil {
			t.Fatal("expected error for non-https public url in production")
		}
	})

	t.Run("valid production config passes", func(t *testing.T) {
		c := base
		c.AdminSessionSecret = "secret"
		if err := c.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestLoadOverridesFromEnv(t *testing.T) {
	t.Setenv("VEXPAY_ADDR", ":9090")
	t.Setenv("VEXPAY_PUBLIC_URL", "http://localhost:9090/")
	t.Setenv("VEXPAY_INVOICE_EXPIRY", "5m")

	c, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if c.Addr != ":9090" {
		t.Errorf("addr = %q, want :9090", c.Addr)
	}
	if c.PublicURL != "http://localhost:9090" {
		t.Errorf("public url = %q, want trailing slash trimmed", c.PublicURL)
	}
	if c.InvoiceExpiry != 5*time.Minute {
		t.Errorf("invoice expiry = %v, want 5m", c.InvoiceExpiry)
	}
}

func TestLoadRejectsBadDuration(t *testing.T) {
	t.Setenv("VEXPAY_INVOICE_EXPIRY", "not-a-duration")
	if _, err := Load(); err == nil {
		t.Fatal("expected error for invalid duration")
	}
}
