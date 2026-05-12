# VM Slim Agent — Agent Context

## Project overview

Lightweight Go agent collecting host and Docker container metrics, sending them to VictoriaMetrics via plaintext import API. Replacement for Grafana Alloy / node_exporter + cadvisor.

## Tech stack

- **Go 1.24.4**, module `vm-slim-agent`
- **gopsutil/v3** — host metrics (CPU, memory, disk, network)
- **cgroups v2** — container metrics (no Docker API)
- **VictoriaMetrics** `/api/v1/import/prometheus` — plaintext output (metrics)
- **VictoriaLogs** `/insert/loki/api/v1/push` — Loki-compatible push API (logs, optional)
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
├── logcollectors/
│   ├── collector.go    # Log Entry struct, Collector interface
│   ├── journald.go     # journald log collector (via journalctl --output=json)
│   └── docker.go       # Docker JSON-file log collector (reads -json.log files)
├── output/
│   ├── vm.go           # VM plaintext output with exponential backoff retry
│   └── vlogs.go        # VictoriaLogs Loki-compatible push with retry
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

### Log collectors

Optional subsystem for collecting logs and sending them to VictoriaLogs.

#### Log Collector interface

```go
type Collector interface {
    Name() string
    Collect() ([]Entry, error)
}

type Entry struct {
    Timestamp time.Time
    Message   string
    Labels    map[string]string
    Fields    map[string]string
}
```

#### Sources

Configured via YAML file (`LOGS_CONFIG_FILE`, see Config below):

```yaml
journald:
  - unit: ssh.service
  - unit: nginx.service
docker:
  - container: app-web
  - container: app-api
  - container: redis-*     # glob patterns supported
```

- **journald**: invokes `journalctl --output=json --unit=<unit>` with cursor tracking. Falls back to `time.Now()` if timestamp parsing fails.
- **Docker**: reads `/var/lib/docker/containers/<id>/<id>-json.log` files. Resolves container IDs from `config.v2.json` by name. Supports exact name match and glob patterns (`*`, `?`).

#### Offset tracking (Docker logs)

Docker collector tracks byte offset per container to read only new log data each cycle:

1. On each `Collect()` call, stats the log file to get current size
2. If file was truncated (log rotation), resets offset to 0
3. Reads all new data (offset → fileSize) into memory, capped at 50MB/cycle
4. Splits data into complete lines (drops trailing incomplete line for next cycle)
5. Uses a custom `lineScanner` (not `bufio.Scanner`) to avoid `ErrTooLong` infinite retry loops
6. Advances offset by bytes of successfully processed lines
7. On scanner error: still advances past processed data to prevent getting stuck

#### Output: VLogsOutput

Sends log entries to VictoriaLogs via `/insert/loki/api/v1/push` (Loki-compatible push API). Groups entries by label set into Loki streams. Implements retry with exponential backoff (3 attempts), matching the pattern used by `VMOutput` for metrics.

### Config

| Env | Default | Required |
|-----|---------|----------|
| `VM_URL` | — | Yes |
| `SCRAPE_INTERVAL` | `5s` | No |
| `HOSTNAME` | `os.Hostname()` | No |
| `LOGS_CONFIG_FILE` | — | No (enables logs) |
| `LOGS_BACKEND_URL` | — | Yes (if LOGS_CONFIG_FILE set) |
| `LOGS_STATE_DIR` | `/tmp/vm-slim-agent` | No |

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
