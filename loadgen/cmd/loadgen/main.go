// Command loadgen sends steady, open-loop HTTP traffic to the slo-demo
// service. See loadgen/README.md.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/salemax/slo-demo/loadgen/internal/load"
)

func main() {
	if err := run(os.Args[1:], os.Stderr); err != nil {
		log.Fatal(err)
	}
}

func run(args []string, stderr io.Writer) error {
	cfg, err := parseConfig(args, os.LookupEnv, stderr)
	if err != nil {
		return err
	}
	cfg.Logger = log.New(stderr, "", log.LstdFlags)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-ctx.Done()
		// Restore default signal handling: a second Ctrl-C while waiting
		// for in-flight requests kills the process immediately.
		stop()
	}()
	defer stop()

	_, err = load.Run(ctx, cfg)
	return err
}

// parseConfig reads flags, falling back to LOADGEN_* environment variables,
// then to the defaults. A flag given on the command line wins over the env.
func parseConfig(args []string, lookup func(string) (string, bool), stderr io.Writer) (load.Config, error) {
	env := func(name, def string) string {
		if v, ok := lookup(name); ok && v != "" {
			return v
		}
		return def
	}

	fs := flag.NewFlagSet("loadgen", flag.ContinueOnError)
	fs.SetOutput(stderr)
	target := fs.String("target", env("LOADGEN_TARGET", "http://localhost:8080"),
		"base URL of the service (env LOADGEN_TARGET)")
	rate := fs.String("rate", env("LOADGEN_RATE", "20"),
		"offered load in requests per second (env LOADGEN_RATE)")
	routes := fs.String("routes", env("LOADGEN_ROUTES", "/api/fast=80,/api/slow=20"),
		"route mix as path=weight,... (env LOADGEN_ROUTES)")
	timeout := fs.String("timeout", env("LOADGEN_TIMEOUT", "5s"),
		"per-request client timeout (env LOADGEN_TIMEOUT)")
	duration := fs.String("duration", env("LOADGEN_DURATION", "0"),
		"how long to run; 0 = until SIGINT/SIGTERM (env LOADGEN_DURATION)")
	maxInFlight := fs.String("max-in-flight", env("LOADGEN_MAX_IN_FLIGHT", "1000"),
		"cap on concurrent requests; requests over it are dropped and counted (env LOADGEN_MAX_IN_FLIGHT)")
	report := fs.String("report", env("LOADGEN_REPORT", "10s"),
		"progress log interval; 0 = off (env LOADGEN_REPORT)")
	if err := fs.Parse(args); err != nil {
		return load.Config{}, err
	}
	if fs.NArg() > 0 {
		return load.Config{}, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}

	var cfg load.Config
	var err error
	cfg.Target = *target
	if cfg.Rate, err = strconv.ParseFloat(*rate, 64); err != nil {
		return cfg, fmt.Errorf("rate: %w", err)
	}
	if cfg.Routes, err = load.ParseRoutes(*routes); err != nil {
		return cfg, fmt.Errorf("routes: %w", err)
	}
	if cfg.Timeout, err = time.ParseDuration(*timeout); err != nil {
		return cfg, fmt.Errorf("timeout: %w", err)
	}
	if cfg.Duration, err = time.ParseDuration(*duration); err != nil {
		return cfg, fmt.Errorf("duration: %w", err)
	}
	if cfg.MaxInFlight, err = strconv.Atoi(*maxInFlight); err != nil {
		return cfg, fmt.Errorf("max-in-flight: %w", err)
	}
	if cfg.Report, err = time.ParseDuration(*report); err != nil {
		return cfg, fmt.Errorf("report: %w", err)
	}
	return cfg, nil
}
