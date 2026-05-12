package main

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	VMURL          string
	ScrapeInterval time.Duration
	Hostname       string
	Logs           *LogsConfig
	LogsBackendURL string
	LogsStateDir   string
}

type LogsConfig struct {
	Journald []JournaldSourceConfig `yaml:"journald,omitempty"`
	Docker   []DockerSourceConfig   `yaml:"docker,omitempty"`
}

type JournaldSourceConfig struct {
	Unit string `yaml:"unit"`
}

type DockerSourceConfig struct {
	Container string `yaml:"container"`
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

	var logsCfg *LogsConfig
	var logsBackendURL string
	logsStateDir := os.Getenv("LOGS_STATE_DIR")
	if logsStateDir == "" {
		logsStateDir = "/tmp/vm-slim-agent"
	}

	if path := os.Getenv("LOGS_CONFIG_FILE"); path != "" {
		logsBackendURL = os.Getenv("LOGS_BACKEND_URL")
		if logsBackendURL == "" {
			return nil, fmt.Errorf("LOGS_BACKEND_URL environment variable is required when LOGS_CONFIG_FILE is set")
		}

		cfg, err := LoadLogsConfig(path)
		if err != nil {
			return nil, err
		}
		logsCfg = &LogsConfig{
			Journald: cfg.Journald,
			Docker:   cfg.Docker,
		}
	}

	return &Config{
		VMURL:          vmURL,
		ScrapeInterval: interval,
		Hostname:       hostname,
		Logs:           logsCfg,
		LogsBackendURL: logsBackendURL,
		LogsStateDir:   logsStateDir,
	}, nil
}

func LoadLogsConfig(path string) (*LogsConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read logs config: %w", err)
	}

	var cfg LogsConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse logs config: %w", err)
	}

	// At least one section must be defined
	if len(cfg.Journald) == 0 && len(cfg.Docker) == 0 {
		return nil, fmt.Errorf("at least one log source (journald or docker) is required")
	}

	// Validate journald sources
	for i, src := range cfg.Journald {
		if src.Unit == "" {
			return nil, fmt.Errorf("journald[%d]: unit is required", i)
		}
	}

	// Validate docker sources
	for i, src := range cfg.Docker {
		if src.Container == "" {
			return nil, fmt.Errorf("docker[%d]: container is required", i)
		}
	}

	return &cfg, nil
}
