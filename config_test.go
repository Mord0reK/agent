package main

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfigDefaults(t *testing.T) {
	os.Setenv("VM_URL", "http://localhost:8428")
	os.Unsetenv("SCRAPE_INTERVAL")
	os.Unsetenv("HOSTNAME")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.VMURL != "http://localhost:8428" {
		t.Errorf("expected VM_URL http://localhost:8428, got %s", cfg.VMURL)
	}
	if cfg.ScrapeInterval != 5*time.Second {
		t.Errorf("expected interval 5s, got %v", cfg.ScrapeInterval)
	}
}

func TestLoadConfigCustomInterval(t *testing.T) {
	os.Setenv("VM_URL", "http://vm:8428")
	os.Setenv("SCRAPE_INTERVAL", "30s")
	os.Setenv("HOSTNAME", "myhost")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ScrapeInterval != 30*time.Second {
		t.Errorf("expected interval 30s, got %v", cfg.ScrapeInterval)
	}
	if cfg.Hostname != "myhost" {
		t.Errorf("expected hostname myhost, got %s", cfg.Hostname)
	}
}

func TestLoadConfigMissingVMURL(t *testing.T) {
	os.Unsetenv("VM_URL")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for missing VM_URL")
	}
}
