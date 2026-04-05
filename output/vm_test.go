package output

import (
	"testing"
	"time"

	"vm-slim-agent/collectors"
)

func TestFormatMetricPlaintext(t *testing.T) {
	m := collectors.Metric{
		Name:      "test_metric",
		Labels:    map[string]string{"host": "test", "env": "prod"},
		Value:     42.5,
		Timestamp: time.Unix(1700000000, 0),
	}

	result := formatMetricPlaintext(m)

	expected := `test_metric{env="prod",host="test"} 42.5 1700000000000`
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}

func TestFormatMetricNoLabels(t *testing.T) {
	m := collectors.Metric{
		Name:      "test_metric",
		Labels:    map[string]string{},
		Value:     1.0,
		Timestamp: time.Unix(1700000000, 0),
	}

	result := formatMetricPlaintext(m)

	expected := `test_metric 1 1700000000000`
	if result != expected {
		t.Errorf("expected %q, got %q", expected, result)
	}
}
