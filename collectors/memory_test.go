package collectors

import (
	"testing"
)

func TestMemoryCollectorName(t *testing.T) {
	c := NewMemoryCollector("testhost")
	if c.Name() != "memory" {
		t.Errorf("expected name 'memory', got %s", c.Name())
	}
}

func TestMemoryCollectorCollect(t *testing.T) {
	c := NewMemoryCollector("testhost")
	metrics, err := c.Collect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(metrics) < 3 {
		t.Fatalf("expected at least 3 metrics (MemTotal, MemAvailable, MemFree), got %d", len(metrics))
	}

	expectedNames := []string{"node_memory_MemTotal_bytes", "node_memory_MemAvailable_bytes", "node_memory_MemFree_bytes"}
	for _, expected := range expectedNames {
		found := false
		for _, m := range metrics {
			if m.Name == expected {
				found = true
				if m.Labels["hostname"] != "testhost" {
					t.Errorf("expected hostname 'testhost', got %s", m.Labels["hostname"])
				}
				if m.Value < 0 {
					t.Errorf("expected non-negative value for %s, got %f", expected, m.Value)
				}
				break
			}
		}
		if !found {
			t.Errorf("expected metric %s not found", expected)
		}
	}
}
