package collectors

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestContainerCollectorCollectIOParsesCgroupV2IOStat(t *testing.T) {
	tmp := t.TempDir()
	ioPath := filepath.Join(tmp, "io.stat")

	// cgroup v2 io.stat format: "<major>:<minor> rbytes=<n> wbytes=<n> ..."
	if err := os.WriteFile(ioPath, []byte("8:0 rbytes=100 wbytes=200 rios=1 wios=2\n8:16 rbytes=50 wbytes=60 rios=3 wios=4\n"), 0o600); err != nil {
		t.Fatalf("write io.stat: %v", err)
	}

	c := NewContainerCollector("test-host")
	cg := &containerCgroup{
		containerID:   strings.Repeat("a", 64),
		containerName: "test-container",
		ioPath:        ioPath,
	}

	now := time.Unix(1700000000, 0)
	metrics, err := c.collectIO(cg, now)
	if err != nil {
		t.Fatalf("collectIO returned error: %v", err)
	}
	if len(metrics) != 2 {
		t.Fatalf("expected 2 metrics, got %d", len(metrics))
	}

	got := map[string]float64{}
	for _, m := range metrics {
		got[m.Name] = m.Value
	}

	if got["container_fs_read_bytes_total"] != 150 {
		t.Fatalf("unexpected read bytes: %v", got["container_fs_read_bytes_total"])
	}
	if got["container_fs_write_bytes_total"] != 260 {
		t.Fatalf("unexpected write bytes: %v", got["container_fs_write_bytes_total"])
	}
}

func TestNetDevPathEmptyWhenNoPIDs(t *testing.T) {
	tmp := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmp, "cgroup.procs"), []byte(""), 0o600); err != nil {
		t.Fatalf("write cgroup.procs: %v", err)
	}
	if got := netDevPath(tmp); got != "" {
		t.Fatalf("expected empty netDevPath, got %q", got)
	}
}
