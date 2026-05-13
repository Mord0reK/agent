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
