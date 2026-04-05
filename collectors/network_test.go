package collectors

import (
	"testing"
)

func TestNetworkCollectorName(t *testing.T) {
	c := NewNetworkCollector("testhost")
	if c.Name() != "network" {
		t.Errorf("expected name 'network', got %s", c.Name())
	}
}

func TestNetworkCollectorCollect(t *testing.T) {
	c := NewNetworkCollector("testhost")
	metrics, err := c.Collect()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(metrics) == 0 {
		t.Fatal("expected at least one network metric")
	}

	for _, m := range metrics {
		if m.Labels["hostname"] != "testhost" {
			t.Errorf("expected hostname 'testhost', got %s", m.Labels["hostname"])
		}
		if m.Labels["device"] == "" {
			t.Error("expected device label")
		}
	}
}
