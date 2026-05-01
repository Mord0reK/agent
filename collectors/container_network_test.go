package collectors

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestContainerCollectorNetworkSkipsHostNetwork(t *testing.T) {
	c := &ContainerCollector{hostname: "testhost"}
	cg := &containerCgroup{
		containerID:   "a",
		containerName: "test",
		image:         "img",
		ports:         "none",
		hostNetwork:   true,
		netDevPath:    "/proc/1/net/dev",
	}
	metrics, err := c.collectNetwork(cg, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(metrics) != 0 {
		t.Errorf("expected no metrics for host-network container, got %d", len(metrics))
	}
}

func TestContainerCollectorNetworkSkipsEmptyPath(t *testing.T) {
	c := &ContainerCollector{hostname: "testhost"}
	cg := &containerCgroup{
		containerID:   "a",
		containerName: "test",
		image:         "img",
		ports:         "none",
		hostNetwork:   false,
		netDevPath:    "",
	}
	metrics, err := c.collectNetwork(cg, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(metrics) != 0 {
		t.Errorf("expected no metrics for empty netDevPath, got %d", len(metrics))
	}
}

func TestContainerCollectorNetworkParsesNetDev(t *testing.T) {
	dir := t.TempDir()
	netDevFile := filepath.Join(dir, "net.dev")
	netDevContent := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo:       0       0    0    0    0     0          0         0        0       0    0    0    0     0       0          0
  eth0: 1234567       10    0    0    0     0          0         0  7654321       20    0    0    0     0       0          0`
	if err := os.WriteFile(netDevFile, []byte(netDevContent), 0644); err != nil {
		t.Fatal(err)
	}

	c := &ContainerCollector{hostname: "testhost"}
	cg := &containerCgroup{
		containerID:   "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
		containerName: "test",
		image:         "img",
		hostNetwork:   false,
		netDevPath:    netDevFile,
	}

	metrics, err := c.collectNetwork(cg, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(metrics) != 2 {
		t.Fatalf("expected 2 metrics (rx + tx for eth0), got %d", len(metrics))
	}

	if metrics[0].Name != "container_network_receive_bytes_total" {
		t.Errorf("expected first metric to be receive_bytes_total, got %s", metrics[0].Name)
	}
	if metrics[0].Value != 1234567 {
		t.Errorf("expected rx bytes 1234567, got %f", metrics[0].Value)
	}

	if metrics[1].Name != "container_network_transmit_bytes_total" {
		t.Errorf("expected second metric to be transmit_bytes_total, got %s", metrics[1].Name)
	}
	if metrics[1].Value != 7654321 {
		t.Errorf("expected tx bytes 7654321, got %f", metrics[1].Value)
	}

	if metrics[0].Labels["device"] != "eth0" {
		t.Errorf("expected device label 'eth0', got %s", metrics[0].Labels["device"])
	}
	if metrics[0].Labels["hostname"] != "testhost" {
		t.Errorf("expected hostname label 'testhost', got %s", metrics[0].Labels["hostname"])
	}
}

func TestContainerCollectorNetworkSkipsLoopback(t *testing.T) {
	dir := t.TempDir()
	netDevFile := filepath.Join(dir, "net.dev")
	netDevContent := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo:  100000       10    0    0    0     0          0         0   100000       10    0    0    0     0       0          0`
	if err := os.WriteFile(netDevFile, []byte(netDevContent), 0644); err != nil {
		t.Fatal(err)
	}

	c := &ContainerCollector{hostname: "testhost"}
	cg := &containerCgroup{
		containerID:   "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
		containerName: "test",
		image:         "img",
		hostNetwork:   false,
		netDevPath:    netDevFile,
	}

	metrics, err := c.collectNetwork(cg, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(metrics) != 0 {
		t.Errorf("expected no metrics (only lo interface), got %d", len(metrics))
	}
}

func TestContainerCollectorNetworkMultipleInterfaces(t *testing.T) {
	dir := t.TempDir()
	netDevFile := filepath.Join(dir, "net.dev")
	netDevContent := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo:       0       0    0    0    0     0          0         0        0       0    0    0    0     0       0          0
  eth0: 1000000       10    0    0    0     0          0         0   500000       20    0    0    0     0       0          0
  eth1: 2000000       30    0    0    0     0          0         0  1000000       40    0    0    0     0       0          0`
	if err := os.WriteFile(netDevFile, []byte(netDevContent), 0644); err != nil {
		t.Fatal(err)
	}

	c := &ContainerCollector{hostname: "testhost"}
	cg := &containerCgroup{
		containerID:   "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
		containerName: "test",
		image:         "img",
		hostNetwork:   false,
		netDevPath:    netDevFile,
	}

	metrics, err := c.collectNetwork(cg, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(metrics) != 4 {
		t.Fatalf("expected 4 metrics (rx + tx for eth0 and eth1), got %d", len(metrics))
	}

	rxEth0 := findMetric(metrics, "container_network_receive_bytes_total", "eth0")
	if rxEth0 == nil {
		t.Fatal("expected rx metric for eth0")
	}
	if rxEth0.Value != 1000000 {
		t.Errorf("expected eth0 rx 1000000, got %f", rxEth0.Value)
	}

	txEth1 := findMetric(metrics, "container_network_transmit_bytes_total", "eth1")
	if txEth1 == nil {
		t.Fatal("expected tx metric for eth1")
	}
	if txEth1.Value != 1000000 {
		t.Errorf("expected eth1 tx 1000000, got %f", txEth1.Value)
	}
}

func TestContainerCollectorNetworkInvalidFormat(t *testing.T) {
	dir := t.TempDir()
	netDevFile := filepath.Join(dir, "net.dev")
	netDevContent := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
  eth0: invalid_number       10    0    0    0     0          0         0   500000       20    0    0    0     0       0          0`
	if err := os.WriteFile(netDevFile, []byte(netDevContent), 0644); err != nil {
		t.Fatal(err)
	}

	c := &ContainerCollector{hostname: "testhost"}
	cg := &containerCgroup{
		containerID:   "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
		containerName: "test",
		image:         "img",
		hostNetwork:   false,
		netDevPath:    netDevFile,
	}

	_, err := c.collectNetwork(cg, time.Now())
	if err == nil {
		t.Error("expected error for invalid format, got nil")
	}
}

func TestContainerCollectorNetworkShortContainerID(t *testing.T) {
	dir := t.TempDir()
	netDevFile := filepath.Join(dir, "net.dev")
	netDevContent := `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
  eth0: 1000000       10    0    0    0     0          0         0   500000       20    0    0    0     0       0          0`
	if err := os.WriteFile(netDevFile, []byte(netDevContent), 0644); err != nil {
		t.Fatal(err)
	}

	c := &ContainerCollector{hostname: "testhost"}
	cg := &containerCgroup{
		containerID:   "abc",  // Short ID
		containerName: "test",
		image:         "img",
		hostNetwork:   false,
		netDevPath:    netDevFile,
	}

	metrics, err := c.collectNetwork(cg, time.Now())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(metrics))
	}
	if metrics[0].Labels["container_id"] != "abc" {
		t.Errorf("expected container_id 'abc', got %s", metrics[0].Labels["container_id"])
	}
}

func findMetric(metrics []Metric, name, device string) *Metric {
	for i, m := range metrics {
		if m.Name == name && m.Labels["device"] == device {
			return &metrics[i]
		}
	}
	return nil
}
