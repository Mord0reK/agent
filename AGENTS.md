# VM Slim Agent — Agent Context

## Project overview

Lightweight Go agent collecting host and Docker container metrics, sending them to VictoriaMetrics via plaintext import API. Replacement for Grafana Alloy / node_exporter + cadvisor.

## Tech stack

- **Go 1.24.4**, module `vm-slim-agent`
- **gopsutil/v3** — host metrics (CPU, memory, disk, network)
- **cgroups v2** — container metrics (no Docker API)
- **VictoriaMetrics** `/api/v1/import/prometheus` — plaintext output
- **Docker** — multi-stage build, `scratch` final image, `CGO_ENABLED=0`

## Project structure

```
vm-slim-agent/
├── collectors/
│   ├── collector.go    # Collector interface, Metric struct
│   ├── cpu.go          # Per-core CPU with mode labels (user/system/idle/...)
│   ├── memory.go       # node_exporter compatible (MemTotal, MemAvailable, ...)
│   ├── disk.go         # Filesystem metrics (node_filesystem_*_bytes)
│   ├── network.go      # Network I/O (label: device, not interface)
│   └── container.go    # Container metrics via cgroups v2 + config.v2.json
├── output/
│   └── vm.go           # VM plaintext output with exponential backoff retry
├── config.go           # Env vars: VM_URL (required), SCRAPE_INTERVAL (5s), HOSTNAME
├── main.go             # Ticker loop: collect → batch → send
├── docker-compose.yml  # Recommended deployment
└── Dockerfile          # golang:1.24-alpine → scratch
```

## Key conventions

### Metric naming

- **Host metrics**: `node_*` prefix, fully compatible with node_exporter Grafana dashboards
- **Container metrics**: `container_*` prefix, cadvisor-style naming
- **Labels**: always include `hostname` and `instance`
- **Network**: use label `device` (not `interface`)
- **Disk**: use `node_filesystem_*_bytes` (not `node_disk_*_bytes`)

### Collector interface

```go
type Collector interface {
    Name() string
    Collect() ([]Metric, error)
}

type Metric struct {
    Name      string
    Labels    map[string]string
    Value     float64
    Timestamp time.Time
}
```

### Container discovery

- Scans `/sys/fs/cgroup/system.slice/docker-*.scope` every 30s
- Reads `/var/lib/docker/containers/<id>/config.v2.json` for name, image, ports, state
- No Docker API calls — pure filesystem reads

### Config

| Env | Default | Required |
|-----|---------|----------|
| `VM_URL` | — | Yes |
| `SCRAPE_INTERVAL` | `5s` | No |
| `HOSTNAME` | `os.Hostname()` | No |

## Testing

```bash
go test ./... -v
```

TDD approach — each collector and output has its own test file. Run tests before committing.

## Build

```bash
# Static binary
CGO_ENABLED=0 GOOS=linux go build -o vm-agent .

# Docker
docker build -t vm-slim-agent .

# Docker compose
docker compose up -d
```

## Docker compose volumes (required)

- `/sys/fs/cgroup:/sys/fs/cgroup:ro` — container cgroup metrics
- `/var/lib/docker/containers:/var/lib/docker/containers:ro` — container name/image/ports
- `network_mode: host` — access to VictoriaMetrics on localhost
- `pid: host` — proper cgroup visibility
