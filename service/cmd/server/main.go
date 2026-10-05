// Command server runs the slo-demo HTTP service: the public API and, on a
// separate listener, the fault-injection admin API.
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

	"github.com/salemax/slo-demo/service/internal/server"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	adminAddr := os.Getenv("ADMIN_ADDR")
	if adminAddr == "" {
		// Loopback-only by default: the admin API has no authentication of
		// its own (see service/README.md), so not being reachable from
		// outside the host is the real protection for this local demo.
		adminAddr = "127.0.0.1:8081"
	}

	publicHandler, adminHandler := server.NewHandlers()

	publicSrv := &http.Server{
		Addr:    ":" + port,
		Handler: publicHandler,
		// Guard against slow or malicious clients holding connections open;
		// values are generous defaults for a demo service, not tuned to an SLO.
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	adminSrv := &http.Server{
		Addr:              adminAddr,
		Handler:           adminHandler,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Each server's goroutine calls stop() itself once ListenAndServe
	// returns, whether that's because it failed to start or because
	// Shutdown closed its listener. Either way the other server must stop
	// too, and routing both outcomes through the same ctx keeps there from
	// being two separate shutdown paths to get right.
	publicErr := make(chan error, 1)
	adminErr := make(chan error, 1)
	go func() {
		log.Printf("public listening on %s", publicSrv.Addr)
		publicErr <- runUntilClosed(publicSrv)
		stop()
	}()
	go func() {
		log.Printf("admin listening on %s", adminSrv.Addr)
		adminErr <- runUntilClosed(adminSrv)
		stop()
	}()

	<-ctx.Done()
	log.Print("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = publicSrv.Shutdown(shutdownCtx)
	_ = adminSrv.Shutdown(shutdownCtx)

	// Wait for both ListenAndServe goroutines to actually return before
	// exiting, and surface whichever of them failed (if any).
	if err := <-publicErr; err != nil {
		<-adminErr
		return err
	}
	return <-adminErr
}

// runUntilClosed runs srv until it is shut down, returning nil for a
// graceful close (http.ErrServerClosed) and any other error as-is.
func runUntilClosed(srv *http.Server) error {
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
