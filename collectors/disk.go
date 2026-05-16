package collectors

import (
	"syscall"
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

	ioCounters, ioErr := disk.IOCounters()

	now := time.Now()
	metrics := make([]Metric, 0, len(partitions)*3+len(ioCounters)*4)

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

		// Use syscall.Statfs for accurate avail/free bytes.
		// Bavail = blocks available to non-root users (avail_bytes)
		// Bfree  = blocks free to root (free_bytes)
		var stat syscall.Statfs_t
		avail := float64(usage.Free)
		free := float64(usage.Free)
		if err := syscall.Statfs(p.Mountpoint, &stat); err == nil {
			avail = float64(stat.Bavail) * float64(stat.Bsize)
			free = float64(stat.Bfree) * float64(stat.Bsize)
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
				Value:     avail,
				Timestamp: now,
			},
			Metric{
				Name:      "node_filesystem_free_bytes",
				Labels:    labels,
				Value:     free,
				Timestamp: now,
			},
		)
	}

	if ioErr == nil {
		for dev, st := range ioCounters {
			if dev == "" {
				continue
			}

			labels := map[string]string{
				"hostname": c.hostname,
				"instance": c.hostname,
				"device":   dev,
			}

			metrics = append(metrics,
				Metric{
					Name:      "node_disk_read_bytes_total",
					Labels:    labels,
					Value:     float64(st.ReadBytes),
					Timestamp: now,
				},
				Metric{
					Name:      "node_disk_written_bytes_total",
					Labels:    labels,
					Value:     float64(st.WriteBytes),
					Timestamp: now,
				},
				Metric{
					Name:      "node_disk_reads_completed_total",
					Labels:    labels,
					Value:     float64(st.ReadCount),
					Timestamp: now,
				},
				Metric{
					Name:      "node_disk_writes_completed_total",
					Labels:    labels,
					Value:     float64(st.WriteCount),
					Timestamp: now,
				},
			)
		}
	}

	return metrics, nil
}
