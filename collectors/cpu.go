package collectors

import (
	"fmt"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
)

type CPUCollector struct {
	hostname  string
	lastStats []cpu.TimesStat
}

func NewCPUCollector(hostname string) *CPUCollector {
	return &CPUCollector{hostname: hostname}
}

func (c *CPUCollector) Name() string {
	return "cpu"
}

func (c *CPUCollector) Collect() ([]Metric, error) {
	stats, err := cpu.Times(true)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	metrics := make([]Metric, 0, len(stats)*10)

	if len(c.lastStats) > 0 && len(stats) == len(c.lastStats) {
		for i, curr := range stats {
			prev := c.lastStats[i]

			totalDelta := (curr.User - prev.User) + (curr.System - prev.System) +
				(curr.Idle - prev.Idle) + (curr.Iowait - prev.Iowait) +
				(curr.Irq - prev.Irq) + (curr.Softirq - prev.Softirq) +
				(curr.Steal - prev.Steal) + (curr.Nice - prev.Nice)

			if totalDelta > 0 {
				usage := ((totalDelta - (curr.Idle - prev.Idle)) / totalDelta) * 100.0
				cpuLabel := fmt.Sprintf("cpu%d", i)
				metrics = append(metrics, Metric{
					Name: "node_cpu_usage_percent",
					Labels: map[string]string{
						"hostname": c.hostname,
						"instance": c.hostname,
						"cpu":      cpuLabel,
					},
					Value:     usage,
					Timestamp: now,
				})
			}
		}
	}

	for i, s := range stats {
		cpuLabel := fmt.Sprintf("cpu%d", i)
		labels := map[string]string{
			"hostname": c.hostname,
			"instance": c.hostname,
			"cpu":      cpuLabel,
		}

		metrics = append(metrics,
			Metric{Name: "node_cpu_seconds_total", Labels: merge(labels, "mode", "user"), Value: s.User, Timestamp: now},
			Metric{Name: "node_cpu_seconds_total", Labels: merge(labels, "mode", "system"), Value: s.System, Timestamp: now},
			Metric{Name: "node_cpu_seconds_total", Labels: merge(labels, "mode", "idle"), Value: s.Idle, Timestamp: now},
			Metric{Name: "node_cpu_seconds_total", Labels: merge(labels, "mode", "iowait"), Value: s.Iowait, Timestamp: now},
			Metric{Name: "node_cpu_seconds_total", Labels: merge(labels, "mode", "irq"), Value: s.Irq, Timestamp: now},
			Metric{Name: "node_cpu_seconds_total", Labels: merge(labels, "mode", "softirq"), Value: s.Softirq, Timestamp: now},
			Metric{Name: "node_cpu_seconds_total", Labels: merge(labels, "mode", "steal"), Value: s.Steal, Timestamp: now},
			Metric{Name: "node_cpu_seconds_total", Labels: merge(labels, "mode", "nice"), Value: s.Nice, Timestamp: now},
		)
	}

	c.lastStats = stats

	return metrics, nil
}

func merge(base map[string]string, k, v string) map[string]string {
	m := make(map[string]string, len(base)+1)
	for key, val := range base {
		m[key] = val
	}
	m[k] = v
	return m
}
