package main

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	VMURL          string
	ScrapeInterval time.Duration
	Hostname       string
}

func LoadConfig() (*Config, error) {
	vmURL := os.Getenv("VM_URL")
	if vmURL == "" {
		return nil, fmt.Errorf("VM_URL environment variable is required")
	}

	interval := 5 * time.Second
	if s := os.Getenv("SCRAPE_INTERVAL"); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil {
			return nil, fmt.Errorf("invalid SCRAPE_INTERVAL: %w", err)
		}
		interval = d
	}

	hostname := os.Getenv("HOSTNAME")
	if hostname == "" {
		h, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		} else {
			hostname = h
		}
	}

	return &Config{
		VMURL:          vmURL,
		ScrapeInterval: interval,
		Hostname:       hostname,
	}, nil
}
