package collectors

import (
	"time"

	"github.com/shirou/gopsutil/v3/mem"
)

type MemoryCollector struct {
	hostname string
}

func NewMemoryCollector(hostname string) *MemoryCollector {
	return &MemoryCollector{hostname: hostname}
}

func (c *MemoryCollector) Name() string {
	return "memory"
}

func (c *MemoryCollector) Collect() ([]Metric, error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	labels := map[string]string{"hostname": c.hostname, "instance": c.hostname}

	metrics := []Metric{
		{Name: "node_memory_MemTotal_bytes", Labels: labels, Value: float64(v.Total), Timestamp: now},
		{Name: "node_memory_MemFree_bytes", Labels: labels, Value: float64(v.Free), Timestamp: now},
		{Name: "node_memory_MemAvailable_bytes", Labels: labels, Value: float64(v.Available), Timestamp: now},
		{Name: "node_memory_Buffers_bytes", Labels: labels, Value: float64(v.Buffers), Timestamp: now},
		{Name: "node_memory_Cached_bytes", Labels: labels, Value: float64(v.Cached), Timestamp: now},
		{Name: "node_memory_Active_bytes", Labels: labels, Value: float64(v.Active), Timestamp: now},
		{Name: "node_memory_Inactive_bytes", Labels: labels, Value: float64(v.Inactive), Timestamp: now},
		{Name: "node_memory_SwapTotal_bytes", Labels: labels, Value: float64(v.SwapTotal), Timestamp: now},
		{Name: "node_memory_SwapFree_bytes", Labels: labels, Value: float64(v.SwapFree), Timestamp: now},
	}

	return metrics, nil
}
