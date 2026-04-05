package collectors

import (
	"time"

	"github.com/shirou/gopsutil/v3/net"
)

type NetworkCollector struct {
	hostname string
}

func NewNetworkCollector(hostname string) *NetworkCollector {
	return &NetworkCollector{hostname: hostname}
}

func (c *NetworkCollector) Name() string {
	return "network"
}

func (c *NetworkCollector) Collect() ([]Metric, error) {
	counters, err := net.IOCounters(true)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	metrics := make([]Metric, 0, len(counters)*2)

	for _, counter := range counters {
		if counter.Name == "lo" {
			continue
		}

		labels := map[string]string{
			"hostname": c.hostname,
			"instance": c.hostname,
			"device":   counter.Name,
		}

		metrics = append(metrics,
			Metric{
				Name:      "node_network_receive_bytes_total",
				Labels:    labels,
				Value:     float64(counter.BytesRecv),
				Timestamp: now,
			},
			Metric{
				Name:      "node_network_transmit_bytes_total",
				Labels:    labels,
				Value:     float64(counter.BytesSent),
				Timestamp: now,
			},
		)
	}

	return metrics, nil
}
