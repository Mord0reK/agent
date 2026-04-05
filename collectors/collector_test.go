package collectors

import (
	"testing"
	"time"
)

func TestMetricStruct(t *testing.T) {
	m := Metric{
		Name:      "test_metric",
		Labels:    map[string]string{"host": "test"},
		Value:     42.0,
		Timestamp: time.Now(),
	}

	if m.Name != "test_metric" {
		t.Errorf("expected name test_metric, got %s", m.Name)
	}
	if m.Labels["host"] != "test" {
		t.Errorf("expected label host=test, got %s", m.Labels["host"])
	}
	if m.Value != 42.0 {
		t.Errorf("expected value 42.0, got %f", m.Value)
	}
}
