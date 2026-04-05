package collectors

import (
	"strings"
	"testing"
)

func TestCPUCollectorName(t *testing.T) {
	c := NewCPUCollector("testhost")
	if c.Name() != "cpu" {
		t.Errorf("expected name 'cpu', got %s", c.Name())
	}
}

func TestCPUCollectorCollect(t *testing.T) {
	c := NewCPUCollector("testhost")
	metrics, err := c.Collect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(metrics) == 0 {
		t.Fatal("expected at least one metric")
	}

	for _, m := range metrics {
		if !strings.HasPrefix(m.Name, "node_cpu_") {
			t.Errorf("expected metric name prefix node_cpu_, got %s", m.Name)
		}
		if m.Labels["hostname"] != "testhost" {
			t.Errorf("expected hostname label 'testhost', got %s", m.Labels["hostname"])
		}
	}
}
