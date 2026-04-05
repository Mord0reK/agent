# Design: Go Agent — Faza 1 (Host + VM)

## Cel

Lekki agent w Go zbierający metryki hosta i wysyłający je do VictoriaMetrics. Zastępuje ciężkie rozwiązania (Grafana Alloy, node_exporter + cadvisor) minimalnym zużyciem CPU.

## Architektura

```
vm-slim-agent/
├── collectors/
│   ├── collector.go    # interface Collector, struct Metric
│   ├── cpu.go          # CPU usage % (gopsutil)
│   ├── memory.go       # RAM total/used/free (gopsutil)
│   ├── disk.go         # Disk per mount (gopsutil)
│   └── network.go      # Network bytes in/out (gopsutil)
├── output/
│   └── vm.go           # VM output: plaintext → remote_write
├── config.go           # env vars: VM_URL, SCRAPE_INTERVAL, HOSTNAME
└── main.go             # loop: collect → batch → send
```

## Flow

1. `main.go` startuje ticker co `SCRAPE_INTERVAL`
2. Na każdy tick: iteruje po wszystkich `Collector`ach, zbiera `[]Metric`
3. Batchuje wszystkie metryki, wysyła przez `output/vm.go`
4. VM output: najpierw plaintext (`/api/v1/import/prometheus`), z retry + exponential backoff
5. Docelowo: przełączenie na remote_write protobuf (ten sam plik, nowa funkcja)

## Collectors

Każdy collector implementuje interface:

```go
type Collector interface {
    Name() string
    Collect() ([]Metric, error)
}
```

Struct Metric:

```go
type Metric struct {
    Name      string
    Labels    map[string]string
    Value     float64
    Timestamp time.Time
}
```

### Metryki hosta

| Metryka | Źródło | Typ |
|---------|--------|-----|
| CPU usage % | gopsutil/cpu | gauge |
| RAM total/used/free | gopsutil/mem | gauge |
| Disk usage per mount | gopsutil/disk | gauge |
| Network bytes in/out | gopsutil/net | counter |

Wszystkie metryki dostają label `hostname`.

## Output → VictoriaMetrics

### Faza 1a: Plaintext

- Endpoint: `POST /api/v1/import/prometheus`
- Format: Prometheus exposition format (`metric_name{labels} value timestamp`)
- Retry: exponential backoff, max 3 próby, potem log warning

### Faza 1b: Remote Write (docelowo)

- Endpoint: `POST /api/v1/write`
- Format: Prometheus remote write protobuf
- Biblioteka: `prometheus/client_model`
- Ten sam retry logic

## Config

Zmiennych środowiskowych:

| Env | Domyślnie | Opis |
|-----|-----------|------|
| `VM_URL` | (required) | URL VictoriaMetrics |
| `SCRAPE_INTERVAL` | `15s` | Częstotliwość zbierania metryk |
| `HOSTNAME` | `os.Hostname()` | Nazwa hosta w labelach |

## Build

- `CGO_ENABLED=0` — statyczny binary (gopsutil CPU działa bez CGO na Linux)
- `GOOS=linux GOARCH=amd64`
