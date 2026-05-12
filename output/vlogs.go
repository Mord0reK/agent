package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"vm-slim-agent/logcollectors"
)

const vlogsMaxRetries = 3

type VLogsOutput struct {
	baseURL    string
	client     *http.Client
	maxRetries int
}

func NewVLogsOutput(baseURL string) *VLogsOutput {
	return &VLogsOutput{
		baseURL:    strings.TrimRight(baseURL, "/"),
		client:     &http.Client{Timeout: 30 * time.Second},
		maxRetries: vlogsMaxRetries,
	}
}

type lokiPush struct {
	Streams []lokiStream `json:"streams"`
}

type lokiStream struct {
	Stream map[string]string `json:"stream"`
	Values [][]string        `json:"values"`
}

func (o *VLogsOutput) Send(entries []logcollectors.Entry) error {
	if len(entries) == 0 {
		return nil
	}

	grouped := map[string]*lokiStream{}
	for _, e := range entries {
		key := labelsKey(e.Labels)
		stream := grouped[key]
		if stream == nil {
			copyLabels := make(map[string]string, len(e.Labels))
			for k, v := range e.Labels {
				copyLabels[k] = v
			}
			stream = &lokiStream{Stream: copyLabels}
			grouped[key] = stream
		}
		stream.Values = append(stream.Values, []string{fmt.Sprintf("%d", e.Timestamp.UnixNano()), e.Message})
	}

	streams := make([]lokiStream, 0, len(grouped))
	for _, s := range grouped {
		streams = append(streams, *s)
	}

	body, err := json.Marshal(lokiPush{Streams: streams})
	if err != nil {
		return err
	}

	url := o.baseURL + "/insert/loki/api/v1/push"

	var lastErr error
	for attempt := 0; attempt < o.maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(attempt*attempt) * time.Second
			log.Printf("vlogs retry %d/%d after %v", attempt, o.maxRetries, backoff)
			time.Sleep(backoff)
		}

		req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := o.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			return nil
		}

		// Read error body for diagnostics
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()

		lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(errBody)))
	}

	return fmt.Errorf("failed to send logs after %d attempts: %w", o.maxRetries, lastErr)
}

func labelsKey(labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteString("|")
		}
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(labels[k])
	}
	return b.String()
}
