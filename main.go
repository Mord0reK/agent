package main

import (
	"log"
	"time"

	"vm-slim-agent/collectors"
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
	}
}
