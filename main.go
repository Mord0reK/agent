package main

import (
	"log/slog"
	"os"
	"time"

	"vm-slim-agent/collectors"
	"vm-slim-agent/logcollectors"
	"vm-slim-agent/output"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.MessageKey {
				a.Key = "_msg"
			}
			return a
		},
	})))

	cfg, err := LoadConfig()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	slog.Info("Starting VM Agent", "interval", cfg.ScrapeInterval, "vm_url", cfg.VMURL)

	cs := []collectors.Collector{
		collectors.NewCPUCollector(cfg.Hostname),
		collectors.NewMemoryCollector(cfg.Hostname),
		collectors.NewDiskCollector(cfg.Hostname),
		collectors.NewNetworkCollector(cfg.Hostname),
		collectors.NewContainerCollector(cfg.Hostname),
	}

	vmOut := output.NewVMOutput(cfg.VMURL)
	
	// Setup logs collection (optional)
	var logCollectors []logcollectors.Collector
	var logsOut *output.VLogsOutput

	if cfg.Logs != nil {
		if cfg.LogsBackendURL == "" {
			slog.Error("LOGS_BACKEND_URL environment variable is required when LOGS_CONFIG_FILE is set")
			os.Exit(1)
		}

		logsOut = output.NewVLogsOutput(cfg.LogsBackendURL)

		for _, src := range cfg.Logs.Journald {
			logCollectors = append(logCollectors, logcollectors.NewJournaldCollector(cfg.Hostname, src.Unit, cfg.LogsStateDir))
		}
		for _, src := range cfg.Logs.Docker {
			logCollectors = append(logCollectors, logcollectors.NewDockerCollector(cfg.Hostname, src.Container))
		}

		if len(logCollectors) > 0 {
			slog.Info("Logs enabled", "sources", len(logCollectors), "backend", cfg.LogsBackendURL, "state_dir", cfg.LogsStateDir)
		}
	}

	ticker := time.NewTicker(cfg.ScrapeInterval)
	defer ticker.Stop()

	for range ticker.C {
		var allMetrics []collectors.Metric

		for _, c := range cs {
			metrics, err := c.Collect()
			if err != nil {
				slog.Error("Error collecting metrics", "collector", c.Name(), "error", err)
				continue
			}
			allMetrics = append(allMetrics, metrics...)
		}

		if len(allMetrics) == 0 {
			slog.Debug("No metrics collected")
			continue
		}

		if err := vmOut.Send(allMetrics); err != nil {
			slog.Error("Error sending metrics", "error", err)
		} else {
			slog.Info("Sent metrics", "count", len(allMetrics))
		}

		if logsOut != nil && len(logCollectors) > 0 {
			var allLogs []logcollectors.Entry
			for _, c := range logCollectors {
				entries, err := c.Collect()
				if err != nil {
					slog.Error("Error collecting logs", "collector", c.Name(), "error", err)
					continue
				}
				allLogs = append(allLogs, entries...)
			}

			if len(allLogs) > 0 {
				if err := logsOut.Send(allLogs); err != nil {
					slog.Error("Error sending logs", "error", err)
				} else {
					slog.Info("Sent logs", "count", len(allLogs))
				}
			}
		}
	}
}
