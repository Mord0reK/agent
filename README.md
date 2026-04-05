# VM Slim Agent

A lightweight Go agent for collecting host and Docker container metrics and sending them to VictoriaMetrics. Designed as a minimal replacement for heavy solutions like Grafana Alloy, node_exporter + cadvisor.

## Why

Standard monitoring stacks (node_exporter + cadvisor + Grafana Alloy) consume significant CPU and memory. This agent provides the same metric compatibility with minimal resource usage by:

- Reading host metrics directly from **gopsutil** (no /proc scraping)
- Reading container metrics from **cgroups v2** (no Docker API polling)
- Sending metrics via VictoriaMetrics **plaintext import API** (no Prometheus remote_write protobuf overhead)
- Building as a **static binary** (`CGO_ENABLED=0`) running from a `scratch` Docker image

## Metrics

### Host metrics (node_exporter compatible)

| Metric | Description |
|--------|-------------|
| `node_cpu_seconds_total{mode,cpu}` | CPU time per mode (user, system, idle, iowait, irq, softirq, steal, nice) |
| `node_cpu_usage_percent{cpu}` | CPU usage % per core |
| `node_memory_MemTotal_bytes` | Total RAM |
| `node_memory_MemAvailable_bytes` | Available RAM |
| `node_memory_MemFree_bytes` | Free RAM |
| `node_memory_Buffers_bytes` | Buffer memory |
| `node_memory_Cached_bytes` | Cached memory |
| `node_memory_Active_bytes` | Active memory |
| `node_memory_Inactive_bytes` | Inactive memory |
| `node_memory_SwapTotal_bytes` | Total swap |
| `node_memory_SwapFree_bytes` | Free swap |
| `node_filesystem_size_bytes{mountpoint,device,fstype}` | Filesystem size |
| `node_filesystem_avail_bytes{mountpoint,device,fstype}` | Filesystem available space |
| `node_filesystem_free_bytes{mountpoint,device,fstype}` | Filesystem free space |
| `node_network_receive_bytes_total{device}` | Network bytes received |
| `node_network_transmit_bytes_total{device}` | Network bytes transmitted |

### Container metrics (per Docker container)

| Metric | Description |
|--------|-------------|
| `container_info{container,image,ports,state}` | Container metadata (value=1, used as info metric) |
| `container_cpu_usage_seconds_total{container}` | CPU seconds per container |
| `container_cpu_usage_percent{container}` | CPU usage % per container |
| `container_memory_usage_bytes{container}` | Memory usage per container |
| `container_fs_read_bytes_total{container}` | Filesystem read bytes |
| `container_fs_write_bytes_total{container}` | Filesystem write bytes |

All metrics include `hostname` and `instance` labels for multi-server setups.

## Quick start

### Docker Compose (recommended)

```yaml
services:
  vm-slim-agent:
    build: .
    container_name: vm-slim-agent
    restart: unless-stopped
    network_mode: host
    pid: host
    environment:
      - VM_URL=http://localhost:8428
      - HOSTNAME=myserver
      - SCRAPE_INTERVAL=5s
    volumes:
      - /sys/fs/cgroup:/sys/fs/cgroup:ro
      - /var/lib/docker/containers:/var/lib/docker/containers:ro
```

```bash
docker compose up -d
```

### Manual

```bash
VM_URL=http://localhost:8428 SCRAPE_INTERVAL=5s go run .
```

### Build static binary

```bash
CGO_ENABLED=0 GOOS=linux go build -o vm-agent .
```

### Build Docker image

```bash
docker build -t vm-slim-agent .
docker run -d --name vm-slim-agent --network host --pid host \
  -e VM_URL=http://localhost:8428 \
  -e HOSTNAME=myserver \
  -e SCRAPE_INTERVAL=5s \
  -v /sys/fs/cgroup:/sys/fs/cgroup:ro \
  -v /var/lib/docker/containers:/var/lib/docker/containers:ro \
  vm-slim-agent
```

## Configuration

| Environment variable | Default | Description |
|---------------------|---------|-------------|
| `VM_URL` | (required) | VictoriaMetrics URL |
| `SCRAPE_INTERVAL` | `5s` | Metric collection interval |
| `HOSTNAME` | `os.Hostname()` | Hostname label for all metrics |

## Architecture

```
vm-slim-agent/
├── collectors/
│   ├── collector.go    # Collector interface + Metric struct
│   ├── cpu.go          # CPU usage per core (gopsutil)
│   ├── memory.go       # RAM metrics (gopsutil)
│   ├── disk.go         # Filesystem metrics (gopsutil)
│   ├── network.go      # Network I/O (gopsutil)
│   └── container.go    # Container metrics via cgroups v2
├── output/
│   └── vm.go           # VictoriaMetrics plaintext output
├── config.go           # Environment variable config
└── main.go             # Main collection loop
```

## Grafana compatibility

All host metrics use **node_exporter** naming conventions, so standard Grafana dashboards (e.g. "Node Exporter Full") work without modification — just set the `instance` filter to your hostname.

Container metrics use **cadvisor**-style naming (`container_*`) with a `container_info` info metric that includes image name, published ports, and running state — suitable for a container inventory table panel.
