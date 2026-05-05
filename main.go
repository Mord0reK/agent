package main

import (
	"log"
	"os"
	"time"

	"vm-slim-agent/collectors"
	"vm-slim-agent/logcollectors"
	"vm-slim-agent/output"
)

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Starting VM Agent (interval=%s, vm_url=%s)", cfg.ScrapeInterval, cfg.VMURL)

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
		backendURL := os.Getenv("LOGS_BACKEND_URL")
		stateDir := os.Getenv("LOGS_STATE_DIR")
		if stateDir == "" {
			stateDir = "/tmp/vm-slim-agent"
		}
		
		logsOut = output.NewVLogsOutput(backendURL)
		
		for _, src := range cfg.Logs.Journald {
			logCollectors = append(logCollectors, logcollectors.NewJournaldCollector(cfg.Hostname, src.Unit, stateDir))
		}
		for _, src := range cfg.Logs.Docker {
			logCollectors = append(logCollectors, logcollectors.NewDockerCollector(cfg.Hostname, src.Container))
		}
		
		if len(logCollectors) > 0 {
			log.Printf("Logs enabled (%d sources, backend=%s, state_dir=%s)", len(logCollectors), backendURL, stateDir)
		}
	}

	ticker := time.NewTicker(cfg.ScrapeInterval)
	defer ticker.Stop()

	for range ticker.C {
		var allMetrics []collectors.Metric

		for _, c := range cs {
			metrics, err := c.Collect()
			if err != nil {
				log.Printf("Error collecting from %s: %v", c.Name(), err)
				continue
			}
			allMetrics = append(allMetrics, metrics...)
		}

		if len(allMetrics) == 0 {
			log.Println("No metrics collected")
			continue
		}

		if err := vmOut.Send(allMetrics); err != nil {
			log.Printf("Error sending metrics: %v", err)
		} else {
			log.Printf("Sent %d metrics", len(allMetrics))
		}

		if logsOut != nil && len(logCollectors) > 0 {
			var allLogs []logcollectors.Entry
			for _, c := range logCollectors {
				entries, err := c.Collect()
				if err != nil {
					log.Printf("Error collecting logs from %s: %v", c.Name(), err)
					continue
				}
				allLogs = append(allLogs, entries...)
			}

			if len(allLogs) > 0 {
				if err := logsOut.Send(allLogs); err != nil {
					log.Printf("Error sending logs: %v", err)
				} else {
					log.Printf("Sent %d logs", len(allLogs))
				}
			}
		}
	}
}
