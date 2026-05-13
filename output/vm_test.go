package output

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
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

func TestVMOutputSendRetriesWithSameBody(t *testing.T) {
	var bodies [][]byte
	var calls int
	errCh := make(chan error, 1)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/import/prometheus" {
			select {
			case errCh <- fmt.Errorf("unexpected path: %s", r.URL.Path):
			default:
			}
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		b, err := io.ReadAll(r.Body)
		if err != nil {
			select {
			case errCh <- fmt.Errorf("read body: %w", err):
			default:
			}
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		bodies = append(bodies, b)
		calls++

		if calls == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	v := NewVMOutput(srv.URL + "/")
	v.client = srv.Client()
	v.maxRetries = 2
	v.sleep = func(time.Duration) {}

	m := collectors.Metric{
		Name:      "test_metric",
		Labels:    map[string]string{"host": "test"},
		Value:     1.0,
		Timestamp: time.Unix(1700000000, 0),
	}
	expectedBody := formatMetricPlaintext(m) + "\n"

	if err := v.Send([]collectors.Metric{m}); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
	select {
	case err := <-errCh:
		t.Fatalf("server error: %v", err)
	default:
	}
	if calls != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
	if len(bodies) != 2 {
		t.Fatalf("expected 2 bodies, got %d", len(bodies))
	}
	if string(bodies[0]) != expectedBody {
		t.Fatalf("unexpected first body: %q", string(bodies[0]))
	}
	if string(bodies[1]) != expectedBody {
		t.Fatalf("unexpected second body: %q", string(bodies[1]))
	}
}
