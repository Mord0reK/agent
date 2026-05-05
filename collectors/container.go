package collectors

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type ContainerCollector struct {
	hostname     string
	cgroups      []containerCgroup
	lastDiscover time.Time
}

type containerCgroup struct {
	containerID    string
	containerName  string
	image          string
	ports          string
	running        bool
	cpuPath        string
	memPath        string
	ioPath         string
	netDevPath     string
	hostNetwork    bool
	composeProject string
	composeService string
	lastCPUUsage   uint64
	lastTime       time.Time
}

type containerConfig struct {
	Name            string                   `json:"Name"`
	Config          containerInnerConfig     `json:"Config"`
	State           containerState           `json:"State"`
	NetworkSettings containerNetworkSettings `json:"NetworkSettings"`
	HostConfig      containerHostConfig      `json:"HostConfig"`
}

type containerHostConfig struct {
	NetworkMode string `json:"NetworkMode"`
}

type containerInnerConfig struct {
	Image  string            `json:"Image"`
	Labels map[string]string `json:"Labels"`
}

type containerState struct {
	Running bool `json:"Running"`
}

type containerNetworkSettings struct {
	Ports    map[string][]containerPortBinding `json:"Ports"`
	Networks map[string]networkInfo            `json:"Networks"`
}

type networkInfo struct {
	Gateway    string `json:"Gateway"`
	IPAddress  string `json:"IPAddress"`
	MacAddress string `json:"MacAddress"`
}

type containerPortBinding struct {
	HostIP   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}

func NewContainerCollector(hostname string) *ContainerCollector {
	return &ContainerCollector{hostname: hostname}
}

func (c *ContainerCollector) Name() string {
	return "container"
}

func (c *ContainerCollector) Collect() ([]Metric, error) {
	if time.Since(c.lastDiscover) > 30*time.Second {
		c.discoverContainers()
	}

	if len(c.cgroups) == 0 {
		return nil, nil
	}

	now := time.Now()
	metrics := make([]Metric, 0, len(c.cgroups)*12)

	for i := range c.cgroups {
		cg := &c.cgroups[i]

		infoLabels := map[string]string{
			"hostname":     c.hostname,
			"instance":     c.hostname,
			"container":    cg.containerName,
			"container_id": shortContainerID(cg.containerID),
			"name":         cg.containerName,
			"image":        cg.image,
			"ports":        cg.ports,
			"state":        stateLabel(cg.running),
		}

		// Add docker compose labels if present
		if cg.composeProject != "" {
			infoLabels["compose_project"] = cg.composeProject
		}
		if cg.composeService != "" {
			infoLabels["compose_service"] = cg.composeService
		}
		metrics = append(metrics, Metric{
			Name:      "container_info",
			Labels:    infoLabels,
			Value:     1,
			Timestamp: now,
		})

		cpuMetrics, err := c.collectCPU(cg, now)
		if err == nil {
			metrics = append(metrics, cpuMetrics...)
		}

		memMetrics, err := c.collectMemory(cg, now)
		if err == nil {
			metrics = append(metrics, memMetrics...)
		}

		ioMetrics, err := c.collectIO(cg, now)
		if err == nil {
			metrics = append(metrics, ioMetrics...)
		}

		netMetrics, err := c.collectNetwork(cg, now)
		if err == nil {
			metrics = append(metrics, netMetrics...)
		}
	}

	return metrics, nil
}

func stateLabel(running bool) string {
	if running {
		return "running"
	}
	return "stopped"
}

func (c *ContainerCollector) discoverContainers() {
	c.cgroups = nil

	baseDir := "/sys/fs/cgroup/system.slice"
	pattern := filepath.Join(baseDir, "docker-*.scope")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return
	}

	for _, match := range matches {
		base := filepath.Base(match)

		if strings.HasSuffix(base, ".service") || strings.HasSuffix(base, ".socket") {
			continue
		}

		id := strings.TrimPrefix(base, "docker-")
		id = strings.TrimSuffix(id, ".scope")

		if len(id) != 64 {
			continue
		}

		cfg := c.readConfig(id)

		c.cgroups = append(c.cgroups, containerCgroup{
			containerID:    id,
			containerName:  cfg.name,
			image:          cfg.image,
			ports:          cfg.ports,
			running:        cfg.running,
			hostNetwork:    cfg.hostNetwork,
			composeProject: cfg.composeProject,
			composeService: cfg.composeService,
			cpuPath:        filepath.Join(match, "cpu.stat"),
			memPath:        filepath.Join(match, "memory.current"),
			ioPath:         filepath.Join(match, "io.stat"),
			netDevPath:     fmt.Sprintf("/proc/%d/net/dev", firstPID(match)),
		})
	}

	c.lastDiscover = time.Now()
}

func firstPID(cgroupPath string) int {
	procsFile := filepath.Join(cgroupPath, "cgroup.procs")
	data, err := os.ReadFile(procsFile)
	if err != nil {
		return 0
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if pid, err := strconv.Atoi(strings.TrimSpace(line)); err == nil {
			return pid
		}
	}
	return 0
}

type parsedConfig struct {
	name           string
	image          string
	ports          string
	running        bool
	hostNetwork    bool
	composeProject string
	composeService string
}

func (c *ContainerCollector) readConfig(id string) parsedConfig {
	path := fmt.Sprintf("/var/lib/docker/containers/%s/config.v2.json", id)
	data, err := os.ReadFile(path)
	if err != nil {
		return parsedConfig{
			name:  id[:12],
			image: "unknown",
			ports: "none",
		}
	}

	var cfg containerConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return parsedConfig{
			name:  id[:12],
			image: "unknown",
			ports: "none",
		}
	}

	name := strings.TrimPrefix(cfg.Name, "/")
	if name == "" {
		name = id[:12]
	}

	image := cfg.Config.Image
	if image == "" {
		image = "unknown"
	}

	ports := c.formatPorts(cfg.NetworkSettings.Ports)

	// Detect host network mode by checking if "host" network exists in Networks
	// (HostConfig.NetworkMode is often null in config.v2.json)
	hostNetwork := false
	if cfg.NetworkSettings.Networks != nil {
		if _, ok := cfg.NetworkSettings.Networks["host"]; ok {
			hostNetwork = true
		}
	}

	// Extract docker compose labels
	composeProject := ""
	composeService := ""
	if cfg.Config.Labels != nil {
		composeProject = cfg.Config.Labels["com.docker.compose.project"]
		composeService = cfg.Config.Labels["com.docker.compose.service"]
	}

	return parsedConfig{
		name:           name,
		image:          image,
		ports:          ports,
		running:        cfg.State.Running,
		hostNetwork:    hostNetwork,
		composeProject: composeProject,
		composeService: composeService,
	}
}

func (c *ContainerCollector) formatPorts(ports map[string][]containerPortBinding) string {
	if len(ports) == 0 {
		return "none"
	}

	seen := make(map[string]bool)
	var parts []string
	for containerPort, bindings := range ports {
		cp := strings.TrimSuffix(containerPort, "/tcp")
		cp = strings.TrimSuffix(cp, "/udp")
		for _, b := range bindings {
			if b.HostPort != "" && b.HostPort != "0" {
				key := b.HostPort + ":" + cp
				if !seen[key] {
					seen[key] = true
					parts = append(parts, key)
				}
			}
		}
	}

	if len(parts) == 0 {
		return "none"
	}

	return strings.Join(parts, ",")
}

func shortContainerID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

func (c *ContainerCollector) collectCPU(cg *containerCgroup, now time.Time) ([]Metric, error) {
	data, err := os.ReadFile(cg.cpuPath)
	if err != nil {
		return nil, err
	}

	stats := make(map[string]uint64)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) == 2 {
			val, err := strconv.ParseUint(parts[1], 10, 64)
			if err == nil {
				stats[parts[0]] = val
			}
		}
	}

	labels := map[string]string{
		"hostname":     c.hostname,
		"instance":     c.hostname,
		"container":    cg.containerName,
		"container_id": shortContainerID(cg.containerID),
		"name":         cg.containerName,
	}
	if cg.composeProject != "" {
		labels["compose_project"] = cg.composeProject
	}
	if cg.composeService != "" {
		labels["compose_service"] = cg.composeService
	}

	metrics := make([]Metric, 0, 3)

	if cg.lastCPUUsage > 0 && !cg.lastTime.IsZero() {
		elapsed := now.Sub(cg.lastTime).Seconds()
		if elapsed > 0 {
			currentUsage := stats["usage_usec"]
			delta := float64(currentUsage-cg.lastCPUUsage) / 1e6
			cpuPercent := (delta / elapsed) * 100.0

			metrics = append(metrics, Metric{
				Name:      "container_cpu_usage_percent",
				Labels:    labels,
				Value:     cpuPercent,
				Timestamp: now,
			})
		}
	}

	cg.lastCPUUsage = stats["usage_usec"]
	cg.lastTime = now

	if v, ok := stats["usage_usec"]; ok {
		metrics = append(metrics, Metric{
			Name:      "container_cpu_usage_seconds_total",
			Labels:    labels,
			Value:     float64(v) / 1e6,
			Timestamp: now,
		})
	}
	if v, ok := stats["user_usec"]; ok {
		metrics = append(metrics, Metric{
			Name:      "container_cpu_user_seconds_total",
			Labels:    labels,
			Value:     float64(v) / 1e6,
			Timestamp: now,
		})
	}
	if v, ok := stats["system_usec"]; ok {
		metrics = append(metrics, Metric{
			Name:      "container_cpu_system_seconds_total",
			Labels:    labels,
			Value:     float64(v) / 1e6,
			Timestamp: now,
		})
	}

	return metrics, nil
}

func (c *ContainerCollector) collectMemory(cg *containerCgroup, now time.Time) ([]Metric, error) {
	data, err := os.ReadFile(cg.memPath)
	if err != nil {
		return nil, err
	}

	content := strings.TrimSpace(string(data))
	val, err := strconv.ParseUint(content, 10, 64)
	if err != nil {
		return nil, err
	}

	labels := map[string]string{
		"hostname":     c.hostname,
		"instance":     c.hostname,
		"container":    cg.containerName,
		"container_id": shortContainerID(cg.containerID),
		"name":         cg.containerName,
	}
	if cg.composeProject != "" {
		labels["compose_project"] = cg.composeProject
	}
	if cg.composeService != "" {
		labels["compose_service"] = cg.composeService
	}

	return []Metric{
		{
			Name:      "container_memory_usage_bytes",
			Labels:    labels,
			Value:     float64(val),
			Timestamp: now,
		},
	}, nil
}

func (c *ContainerCollector) collectIO(cg *containerCgroup, now time.Time) ([]Metric, error) {
	data, err := os.ReadFile(cg.ioPath)
	if err != nil {
		return nil, err
	}

	var rBytes, wBytes uint64
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		parts := strings.Fields(scanner.Text())
		if len(parts) >= 3 {
			switch parts[0] {
			case "rbytes":
				rBytes, _ = strconv.ParseUint(parts[1], 10, 64)
			case "wbytes":
				wBytes, _ = strconv.ParseUint(parts[1], 10, 64)
			}
		}
	}

	labels := map[string]string{
		"hostname":     c.hostname,
		"instance":     c.hostname,
		"container":    cg.containerName,
		"container_id": shortContainerID(cg.containerID),
		"name":         cg.containerName,
	}
	if cg.composeProject != "" {
		labels["compose_project"] = cg.composeProject
	}
	if cg.composeService != "" {
		labels["compose_service"] = cg.composeService
	}

	return []Metric{
		{
			Name:      "container_fs_read_bytes_total",
			Labels:    labels,
			Value:     float64(rBytes),
			Timestamp: now,
		},
		{
			Name:      "container_fs_write_bytes_total",
			Labels:    labels,
			Value:     float64(wBytes),
			Timestamp: now,
		},
	}, nil
}

func (c *ContainerCollector) collectNetwork(cg *containerCgroup, now time.Time) ([]Metric, error) {
	// Skip containers with host network mode - they share host's network namespace
	if cg.hostNetwork {
		return nil, nil
	}
	if cg.netDevPath == "" {
		return nil, nil
	}

	data, err := os.ReadFile(cg.netDevPath)
	if err != nil {
		return nil, fmt.Errorf("read network stats from %s: %w", cg.netDevPath, err)
	}

	labels := map[string]string{
		"hostname":     c.hostname,
		"instance":     c.hostname,
		"container":    cg.containerName,
		"container_id": shortContainerID(cg.containerID),
		"name":         cg.containerName,
	}
	if cg.composeProject != "" {
		labels["compose_project"] = cg.composeProject
	}
	if cg.composeService != "" {
		labels["compose_service"] = cg.composeService
	}

	var metrics []Metric
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.Contains(line, ":") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		iface := strings.TrimSpace(parts[0])
		if iface == "lo" {
			continue
		}

		fields := strings.Fields(strings.TrimSpace(parts[1]))
		if len(fields) < 9 {
			continue
		}

		rxBytes, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse rx bytes for interface %s: %w", iface, err)
		}
		txBytes, err := strconv.ParseUint(fields[8], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse tx bytes for interface %s: %w", iface, err)
		}

		ifaceLabels := make(map[string]string, len(labels)+1)
		for k, v := range labels {
			ifaceLabels[k] = v
		}
		ifaceLabels["device"] = iface

		metrics = append(metrics, Metric{
			Name:      "container_network_receive_bytes_total",
			Labels:    ifaceLabels,
			Value:     float64(rxBytes),
			Timestamp: now,
		}, Metric{
			Name:      "container_network_transmit_bytes_total",
			Labels:    ifaceLabels,
			Value:     float64(txBytes),
			Timestamp: now,
		})
	}

	return metrics, nil
}
