package main

import (
	"io"
	"testing"
	"time"
)

func lookupFrom(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
}

func TestParseConfigDefaults(t *testing.T) {
	cfg, err := parseConfig(nil, lookupFrom(nil), io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Target != "http://localhost:8080" || cfg.Rate != 20 || cfg.Timeout != 5*time.Second ||
		cfg.Duration != 0 || cfg.MaxInFlight != 1000 || cfg.Report != 10*time.Second {
		t.Fatalf("unexpected defaults: %+v", cfg)
	}
	if len(cfg.Routes) != 2 || cfg.Routes[0].String() != "/api/fast=80" || cfg.Routes[1].String() != "/api/slow=20" {
		t.Fatalf("unexpected default routes: %v", cfg.Routes)
	}
}

func TestParseConfigFlagBeatsEnv(t *testing.T) {
	env := lookupFrom(map[string]string{"LOADGEN_RATE": "7", "LOADGEN_DURATION": "30s"})
	cfg, err := parseConfig([]string{"-rate", "9"}, env, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Rate != 9 {
		t.Errorf("rate = %g, want flag value 9", cfg.Rate)
	}
	if cfg.Duration != 30*time.Second {
		t.Errorf("duration = %s, want env value 30s", cfg.Duration)
	}
}

func TestParseConfigErrors(t *testing.T) {
	for _, args := range [][]string{
		{"-rate", "x"},
		{"-routes", "/healthz=1"},
		{"-timeout", "5"},
		{"-max-in-flight", "lots"},
		{"extra"},
	} {
		if _, err := parseConfig(args, lookupFrom(nil), io.Discard); err == nil {
			t.Errorf("parseConfig(%v): want error", args)
		}
	}
}
