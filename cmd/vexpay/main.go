// Command vexpay is the self-hosted, non-custodial crypto payment gateway.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vexarnetwork/vexpay/internal/app"
	"github.com/vexarnetwork/vexpay/internal/config"
	"github.com/vexarnetwork/vexpay/internal/version"
)

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)

	if len(os.Args) > 1 && (os.Args[1] == "version" || os.Args[1] == "-v" || os.Args[1] == "--version") {
		log.SetFlags(0)
		log.Println("vexpay " + version.String())
		return
	}

	if err := run(); err != nil {
		log.Fatalf("fatal: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	application, err := app.Build(cfg)
	if err != nil {
		return err
	}
	defer application.Close()

	if application.SeededSandboxKey != "" {
		log.Printf("generated sandbox API key (test only): %s", application.SeededSandboxKey)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Background watcher: confirms payments automatically.
	go application.Watcher.Run(ctx)

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           application.Handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("vexpay %s listening on %s (env=%s)", version.Version, cfg.Addr, cfg.Env)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		log.Println("shutdown signal received, draining connections...")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
