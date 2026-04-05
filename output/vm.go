package output

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"vm-slim-agent/collectors"
)

type VMOutput struct {
	vmURL      string
	client     *http.Client
	maxRetries int
}

func NewVMOutput(vmURL string) *VMOutput {
	return &VMOutput{
		vmURL:      vmURL,
		client:     &http.Client{Timeout: 30 * time.Second},
		maxRetries: 3,
	}
}

func formatMetricPlaintext(m collectors.Metric) string {
	var sb strings.Builder
	sb.WriteString(m.Name)

	if len(m.Labels) > 0 {
		keys := make([]string, 0, len(m.Labels))
		for k := range m.Labels {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		sb.WriteString("{")
		for i, k := range keys {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(fmt.Sprintf("%s=%q", k, m.Labels[k]))
		}
		sb.WriteString("}")
	}

	sb.WriteString(fmt.Sprintf(" %v %d", m.Value, m.Timestamp.UnixMilli()))
	return sb.String()
}

func (v *VMOutput) Send(metrics []collectors.Metric) error {
	if len(metrics) == 0 {
		return nil
	}

	var body bytes.Buffer
	for _, m := range metrics {
		body.WriteString(formatMetricPlaintext(m))
		body.WriteString("\n")
	}

	url := v.vmURL + "/api/v1/import/prometheus"

	var lastErr error
	for attempt := 0; attempt < v.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * time.Second
			log.Printf("retry %d/%d after %v", attempt, v.maxRetries, backoff)
			time.Sleep(backoff)
		}

		resp, err := v.client.Post(url, "text/plain", &body)
		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			return nil
		}

		lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	return fmt.Errorf("failed to send metrics after %d attempts: %w", v.maxRetries, lastErr)
}
