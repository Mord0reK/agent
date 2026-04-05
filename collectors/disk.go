package collectors

import (
	"time"

	"github.com/shirou/gopsutil/v3/disk"
)

type DiskCollector struct {
	hostname string
}

func NewDiskCollector(hostname string) *DiskCollector {
	return &DiskCollector{hostname: hostname}
}

func (c *DiskCollector) Name() string {
	return "disk"
}

func (c *DiskCollector) Collect() ([]Metric, error) {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	metrics := make([]Metric, 0, len(partitions)*3)

	for _, p := range partitions {
		usage, err := disk.Usage(p.Mountpoint)
		if err != nil {
			continue
		}

		labels := map[string]string{
			"hostname":   c.hostname,
			"instance":   c.hostname,
			"mountpoint": p.Mountpoint,
			"device":     p.Device,
			"fstype":     p.Fstype,
		}

		metrics = append(metrics,
			Metric{
				Name:      "node_filesystem_size_bytes",
				Labels:    labels,
				Value:     float64(usage.Total),
				Timestamp: now,
			},
			Metric{
				Name:      "node_filesystem_avail_bytes",
				Labels:    labels,
				Value:     float64(usage.Free),
				Timestamp: now,
			},
			Metric{
				Name:      "node_filesystem_free_bytes",
				Labels:    labels,
				Value:     float64(usage.Free),
				Timestamp: now,
			},
		)
	}

	return metrics, nil
}
