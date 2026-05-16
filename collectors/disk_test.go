package collectors

import (
	"strings"
	"testing"
)

func TestDiskCollectorName(t *testing.T) {
	c := NewDiskCollector("testhost")
	if c.Name() != "disk" {
		t.Errorf("expected name 'disk', got %s", c.Name())
	}
}

func TestDiskCollectorCollect(t *testing.T) {
	c := NewDiskCollector("testhost")
	metrics, err := c.Collect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(metrics) == 0 {
		t.Fatal("expected at least one disk metric")
	}

	for _, m := range metrics {
		if m.Labels["hostname"] != "testhost" {
			t.Errorf("expected hostname 'testhost', got %s", m.Labels["hostname"])
		}
		if strings.HasPrefix(m.Name, "node_filesystem_") {
			if m.Labels["mountpoint"] == "" {
				t.Errorf("expected mountpoint label for %s", m.Name)
			}
		}
		if strings.HasPrefix(m.Name, "node_disk_") {
			if m.Labels["device"] == "" {
				t.Errorf("expected device label for %s", m.Name)
			}
		}
	}
}

func TestDiskCollectorAvailNotExceedsFree(t *testing.T) {
	c := NewDiskCollector("testhost")
	metrics, err := c.Collect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Group metrics by mountpoint
	type fsMetrics struct {
		avail float64
		free  float64
		size  float64
	}
	fsMap := make(map[string]*fsMetrics)

	for _, m := range metrics {
		if !strings.HasPrefix(m.Name, "node_filesystem_") {
			continue
		}
		mp := m.Labels["mountpoint"]
		if mp == "" {
			continue
		}
		if _, ok := fsMap[mp]; !ok {
			fsMap[mp] = &fsMetrics{}
		}
		switch m.Name {
		case "node_filesystem_avail_bytes":
			fsMap[mp].avail = m.Value
		case "node_filesystem_free_bytes":
			fsMap[mp].free = m.Value
		case "node_filesystem_size_bytes":
			fsMap[mp].size = m.Value
		}
	}

	for mp, fs := range fsMap {
		if fs.avail > fs.free {
			t.Errorf("mountpoint %s: avail_bytes (%.0f) > free_bytes (%.0f) — avail should be <= free",
				mp, fs.avail, fs.free)
		}
		if fs.free > fs.size {
			t.Errorf("mountpoint %s: free_bytes (%.0f) > size_bytes (%.0f) — free should be <= size",
				mp, fs.free, fs.size)
		}
	}
}
