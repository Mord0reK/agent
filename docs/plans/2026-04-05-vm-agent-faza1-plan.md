# VM Agent — Faza 1 Implementation Plan

**Goal:** Lekki Go agent zbierający metryki hosta (CPU, RAM, disk, network) i wysyłający je do VictoriaMetrics przez plaintext API.

**Architecture:** TDD-driven implementation. Każdy collector i output mają osobne pliki. Zaczynamy od interface'ów, potem testy, potem implementacja.

**Tech Stack:** Go 1.24, gopsutil/v3, standard library (net/http, time, os, fmt)

---

### Task 1: Interface Collector + Struct Metric

**Files:**
- Create: `collectors/collector.go`
- Test: `collectors/collector_test.go`

**Step 1: Write the test**

```go
// collectors/collector_test.go
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
```

**Step 2: Run test to verify it fails**

```bash
go test ./collectors/ -v
```
Expected: FAIL — Metric not defined

**Step 3: Write implementation**

```go
// collectors/collector.go
package collectors

import "time"

type Metric struct {
	Name      string
	Labels    map[string]string
	Value     float64
	Timestamp time.Time
}

type Collector interface {
	Name() string
	Collect() ([]Metric, error)
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./collectors/ -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add collectors/collector.go collectors/collector_test.go
git commit -m "feat: add Collector interface and Metric struct"
```

---

### Task 2: Config z env vars

**Files:**
- Create: `config.go`
- Test: `config_test.go`

**Step 1: Write the test**

```go
// config_test.go
package main

import (
	"os"
	"testing"
	"time"
)

func TestLoadConfigDefaults(t *testing.T) {
	os.Setenv("VM_URL", "http://localhost:8428")
	os.Unsetenv("SCRAPE_INTERVAL")
	os.Unsetenv("HOSTNAME")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.VMURL != "http://localhost:8428" {
		t.Errorf("expected VM_URL http://localhost:8428, got %s", cfg.VMURL)
	}
	if cfg.ScrapeInterval != 15*time.Second {
		t.Errorf("expected interval 15s, got %v", cfg.ScrapeInterval)
	}
}

func TestLoadConfigCustomInterval(t *testing.T) {
	os.Setenv("VM_URL", "http://vm:8428")
	os.Setenv("SCRAPE_INTERVAL", "30s")
	os.Setenv("HOSTNAME", "myhost")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ScrapeInterval != 30*time.Second {
		t.Errorf("expected interval 30s, got %v", cfg.ScrapeInterval)
	}
	if cfg.Hostname != "myhost" {
		t.Errorf("expected hostname myhost, got %s", cfg.Hostname)
	}
}

func TestLoadConfigMissingVMURL(t *testing.T) {
	os.Unsetenv("VM_URL")

	_, err := LoadConfig()
	if err == nil {
		t.Fatal("expected error for missing VM_URL")
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test -run TestLoadConfig -v
```
Expected: FAIL — LoadConfig not defined

**Step 3: Write implementation**

```go
// config.go
package main

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	VMURL          string
	ScrapeInterval time.Duration
	Hostname       string
}

func LoadConfig() (*Config, error) {
	vmURL := os.Getenv("VM_URL")
	if vmURL == "" {
		return nil, fmt.Errorf("VM_URL environment variable is required")
	}

	interval := 15 * time.Second
	if s := os.Getenv("SCRAPE_INTERVAL"); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil {
			return nil, fmt.Errorf("invalid SCRAPE_INTERVAL: %w", err)
		}
		interval = d
	}

	hostname := os.Getenv("HOSTNAME")
	if hostname == "" {
		h, err := os.Hostname()
		if err != nil {
			hostname = "unknown"
		} else {
			hostname = h
		}
	}

	return &Config{
		VMURL:          vmURL,
		ScrapeInterval: interval,
		Hostname:       hostname,
	}, nil
}
```

**Step 4: Run test to verify it passes**

```bash
go test -run TestLoadConfig -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add config.go config_test.go
git commit -m "feat: add config with env var loading"
```

---

### Task 3: CPU Collector

**Files:**
- Create: `collectors/cpu.go`
- Test: `collectors/cpu_test.go`

**Step 1: Install dependency**

```bash
go get github.com/shirou/gopsutil/v3@latest
```

**Step 2: Write the test**

```go
// collectors/cpu_test.go
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
```

**Step 3: Run test to verify it fails**

```bash
go test ./collectors/ -run TestCPU -v
```
Expected: FAIL — NewCPUCollector not defined

**Step 4: Write implementation**

```go
// collectors/cpu.go
package collectors

import (
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
)

type CPUCollector struct {
	hostname  string
	lastStats []cpu.TimesStat
	lastTime  time.Time
}

func NewCPUCollector(hostname string) *CPUCollector {
	return &CPUCollector{hostname: hostname}
}

func (c *CPUCollector) Name() string {
	return "cpu"
}

func (c *CPUCollector) Collect() ([]Metric, error) {
	stats, err := cpu.Times(false)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	metrics := make([]Metric, 0, 3)

	if len(c.lastStats) > 0 && !c.lastTime.IsZero() {
		elapsed := now.Sub(c.lastTime).Seconds()
		if elapsed > 0 {
			prev := c.lastStats[0]
			curr := stats[0]

			totalDelta := (curr.User - prev.User) + (curr.System - prev.System) +
				(curr.Idle - prev.Idle) + (curr.Iowait - prev.Iowait)

			if totalDelta > 0 {
				usage := ((totalDelta - (curr.Idle - prev.Idle)) / totalDelta) * 100.0
				metrics = append(metrics, Metric{
					Name: "node_cpu_usage_percent",
					Labels: map[string]string{
						"hostname": c.hostname,
					},
					Value:     usage,
					Timestamp: now,
				})
			}
		}
	}

	c.lastStats = stats
	c.lastTime = now

	if len(stats) > 0 {
		metrics = append(metrics, Metric{
			Name: "node_cpu_seconds_total",
			Labels: map[string]string{
				"hostname": c.hostname,
			},
			Value:     stats[0].User + stats[0].System + stats[0].Idle + stats[0].Iowait,
			Timestamp: now,
		})
	}

	return metrics, nil
}
```

**Step 5: Run test to verify it passes**

```bash
go test ./collectors/ -run TestCPU -v
```
Expected: PASS

**Step 6: Commit**

```bash
git add collectors/cpu.go collectors/cpu_test.go
git commit -m "feat: add CPU collector with delta-based usage calculation"
```

---

### Task 4: Memory Collector

**Files:**
- Create: `collectors/memory.go`
- Test: `collectors/memory_test.go`

**Step 1: Write the test**

```go
// collectors/memory_test.go
package collectors

import (
	"strings"
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
		t.Fatalf("expected at least 3 metrics (total, used, free), got %d", len(metrics))
	}

	expectedNames := []string{"node_memory_total_bytes", "node_memory_used_bytes", "node_memory_free_bytes"}
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
```

**Step 2: Run test to verify it fails**

```bash
go test ./collectors/ -run TestMemory -v
```
Expected: FAIL — NewMemoryCollector not defined

**Step 3: Write implementation**

```go
// collectors/memory.go
package collectors

import (
	"time"

	"github.com/shirou/gopsutil/v3/mem"
)

type MemoryCollector struct {
	hostname string
}

func NewMemoryCollector(hostname string) *MemoryCollector {
	return &MemoryCollector{hostname: hostname}
}

func (c *MemoryCollector) Name() string {
	return "memory"
}

func (c *MemoryCollector) Collect() ([]Metric, error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	labels := map[string]string{"hostname": c.hostname}

	return []Metric{
		{
			Name:      "node_memory_total_bytes",
			Labels:    labels,
			Value:     float64(v.Total),
			Timestamp: now,
		},
		{
			Name:      "node_memory_used_bytes",
			Labels:    labels,
			Value:     float64(v.Used),
			Timestamp: now,
		},
		{
			Name:      "node_memory_free_bytes",
			Labels:    labels,
			Value:     float64(v.Free),
			Timestamp: now,
		},
	}, nil
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./collectors/ -run TestMemory -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add collectors/memory.go collectors/memory_test.go
git commit -m "feat: add memory collector"
```

---

### Task 5: Disk Collector

**Files:**
- Create: `collectors/disk.go`
- Test: `collectors/disk_test.go`

**Step 1: Write the test**

```go
// collectors/disk_test.go
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
		if !strings.HasPrefix(m.Name, "node_disk_") {
			t.Errorf("expected metric name prefix node_disk_, got %s", m.Name)
		}
		if m.Labels["hostname"] != "testhost" {
			t.Errorf("expected hostname 'testhost', got %s", m.Labels["hostname"])
		}
		if m.Labels["mountpoint"] == "" {
			t.Error("expected mountpoint label")
		}
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./collectors/ -run TestDisk -v
```
Expected: FAIL — NewDiskCollector not defined

**Step 3: Write implementation**

```go
// collectors/disk.go
package collectors

import (
	"time"

	"github.com/shirou/gopsutil/v3/disk"
)

type DiskCollector struct {
	hostname string
}

func NewDiskCollector(hostname string) *DiskCollector {
	return &DiskCollector{hostname: hostname}
}

func (c *DiskCollector) Name() string {
	return "disk"
}

func (c *DiskCollector) Collect() ([]Metric, error) {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	metrics := make([]Metric, 0, len(partitions)*2)

	for _, p := range partitions {
		usage, err := disk.Usage(p.Mountpoint)
		if err != nil {
			continue
		}

		labels := map[string]string{
			"hostname":   c.hostname,
			"mountpoint": p.Mountpoint,
			"device":     p.Device,
			"fstype":     p.Fstype,
		}

		metrics = append(metrics,
			Metric{
				Name:      "node_disk_total_bytes",
				Labels:    labels,
				Value:     float64(usage.Total),
				Timestamp: now,
			},
			Metric{
				Name:      "node_disk_used_bytes",
				Labels:    labels,
				Value:     float64(usage.Used),
				Timestamp: now,
			},
			Metric{
				Name:      "node_disk_free_bytes",
				Labels:    labels,
				Value:     float64(usage.Free),
				Timestamp: now,
			},
		)
	}

	return metrics, nil
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./collectors/ -run TestDisk -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add collectors/disk.go collectors/disk_test.go
git commit -m "feat: add disk collector with per-mount metrics"
```

---

### Task 6: Network Collector

**Files:**
- Create: `collectors/network.go`
- Test: `collectors/network_test.go`

**Step 1: Write the test**

```go
// collectors/network_test.go
package collectors

import (
	"strings"
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
		if !strings.HasPrefix(m.Name, "node_network_") {
			t.Errorf("expected metric name prefix node_network_, got %s", m.Name)
		}
		if m.Labels["hostname"] != "testhost" {
			t.Errorf("expected hostname 'testhost', got %s", m.Labels["hostname"])
		}
		if m.Labels["interface"] == "" {
			t.Error("expected interface label")
		}
	}
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./collectors/ -run TestNetwork -v
```
Expected: FAIL — NewNetworkCollector not defined

**Step 3: Write implementation**

```go
// collectors/network.go
package collectors

import (
	"time"

	"github.com/shirou/gopsutil/v3/net"
)

type NetworkCollector struct {
	hostname string
}

func NewNetworkCollector(hostname string) *NetworkCollector {
	return &NetworkCollector{hostname: hostname}
}

func (c *NetworkCollector) Name() string {
	return "network"
}

func (c *NetworkCollector) Collect() ([]Metric, error) {
	counters, err := net.IOCounters(true)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	metrics := make([]Metric, 0, len(counters)*2)

	for _, counter := range counters {
		if counter.Name == "lo" {
			continue
		}

		labels := map[string]string{
			"hostname":  c.hostname,
			"interface": counter.Name,
		}

		metrics = append(metrics,
			Metric{
				Name:      "node_network_receive_bytes_total",
				Labels:    labels,
				Value:     float64(counter.BytesRecv),
				Timestamp: now,
			},
			Metric{
				Name:      "node_network_transmit_bytes_total",
				Labels:    labels,
				Value:     float64(counter.BytesSent),
				Timestamp: now,
			},
		)
	}

	return metrics, nil
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./collectors/ -run TestNetwork -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add collectors/network.go collectors/network_test.go
git commit -m "feat: add network collector (excluding loopback)"
```

---

### Task 7: VM Output (Plaintext)

**Files:**
- Create: `output/vm.go`
- Test: `output/vm_test.go`

**Step 1: Write the test**

```go
// output/vm_test.go
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
```

**Step 2: Run test to verify it fails**

```bash
go test ./output/ -v
```
Expected: FAIL — formatMetricPlaintext not defined

**Step 3: Write implementation**

```go
// output/vm.go
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
	vmURL    string
	client   *http.Client
	maxRetries int
}

func NewVMOutput(vmURL string) *VMOutput {
	return &VMOutput{
		vmURL:    vmURL,
		client:   &http.Client{Timeout: 30 * time.Second},
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
```

**Step 4: Run test to verify it passes**

```bash
go test ./output/ -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add output/vm.go output/vm_test.go
git commit -m "feat: add VM plaintext output with retry logic"
```

---

### Task 8: Main Loop

**Files:**
- Modify: `main.go`

**Step 1: Write implementation**

```go
// main.go
package main

import (
	"log"
	"time"

	"vm-slim-agent/collectors"
	"vm-slim-agent/output"
)

func main() {
	cfg, err := LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Starting VM Agent (interval=%s, vm_url=%s)", cfg.ScrapeInterval, cfg.VMURL)

	cs := []collectors.Collector{
		collectors.NewCPUCollector(cfg.Hostname),
		collectors.NewMemoryCollector(cfg.Hostname),
		collectors.NewDiskCollector(cfg.Hostname),
		collectors.NewNetworkCollector(cfg.Hostname),
	}

	vmOut := output.NewVMOutput(cfg.VMURL)

	ticker := time.NewTicker(cfg.ScrapeInterval)
	defer ticker.Stop()

	for range ticker.C {
		var allMetrics []collectors.Metric

		for _, c := range cs {
			metrics, err := c.Collect()
			if err != nil {
				log.Printf("Error collecting from %s: %v", c.Name(), err)
				continue
			}
			allMetrics = append(allMetrics, metrics...)
		}

		if len(allMetrics) == 0 {
			log.Println("No metrics collected")
			continue
		}

		if err := vmOut.Send(allMetrics); err != nil {
			log.Printf("Error sending metrics: %v", err)
		} else {
			log.Printf("Sent %d metrics", len(allMetrics))
		}
	}
}
```

**Step 2: Build and verify**

```bash
go build -o vm-agent .
```
Expected: Binary builds successfully

**Step 3: Commit**

```bash
git add main.go
git commit -m "feat: add main loop with collector orchestration"
```

---

### Task 9: Integration Test (manual verification)

**Step 1: Run with a test VM**

```bash
VM_URL=http://localhost:8428 SCRAPE_INTERVAL=5s go run .
```

**Step 2: Verify in VictoriaMetrics**

Check VM logs or query:
```
http://localhost:8428/api/v1/query?query=node_cpu_usage_percent
```

**Step 3: Commit any fixes**

---

### Task 10: Dockerfile

**Files:**
- Create: `Dockerfile`

```dockerfile
FROM golang:1.24-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o vm-agent .

FROM scratch
COPY --from=builder /app/vm-agent /vm-agent
USER nobody
ENTRYPOINT ["/vm-agent"]
```

**Build and test:**

```bash
docker build -t vm-agent .
docker run --rm -e VM_URL=http://host.docker.internal:8428 vm-agent
```

**Commit:**

```bash
git add Dockerfile
git commit -m "add: Dockerfile for minimal scratch image"
```
