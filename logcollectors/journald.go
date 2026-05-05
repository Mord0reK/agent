package logcollectors

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type JournaldCollector struct {
	hostname string
	unit     string
	stateDir string
}

func NewJournaldCollector(hostname, unit, stateDir string) *JournaldCollector {
	return &JournaldCollector{hostname: hostname, unit: unit, stateDir: stateDir}
}

func (c *JournaldCollector) Name() string { return "journald" }

func (c *JournaldCollector) Collect() ([]Entry, error) {
	if c.stateDir == "" {
		c.stateDir = "/tmp/vm-slim-agent"
	}
	if err := os.MkdirAll(c.stateDir, 0o755); err != nil {
		return nil, fmt.Errorf("create journal state dir: %w", err)
	}

	// Check if journalctl is available
	if _, err := exec.LookPath("journalctl"); err != nil {
		return nil, fmt.Errorf("journalctl not available: %w", err)
	}

	// Check if journal storage is accessible
	if _, err := os.Stat("/var/log/journal"); os.IsNotExist(err) {
		if _, err := os.Stat("/run/log/journal"); os.IsNotExist(err) {
			return nil, fmt.Errorf("journal storage not accessible (neither /var/log/journal nor /run/log/journal found)")
		}
	}

	cursorFile := filepath.Join(c.stateDir, sanitizeFileName(c.unit)+".cursor")
	args := []string{"--no-pager", "--output=json", "--unit=" + c.unit, "--cursor-file=" + cursorFile}
	if _, err := os.Stat(cursorFile); os.IsNotExist(err) {
		args = append(args, "--since=now")
	}

	cmd := exec.Command("journalctl", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// If output contains data, try to parse it (journalctl might return partial data with error)
		if len(out) == 0 {
			return nil, fmt.Errorf("journalctl failed for unit %q: %v (stderr: %s)", c.unit, err, string(out))
		}
	}

	var entries []Entry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var e struct {
			Message string `json:"MESSAGE"`
			Time    string `json:"__REALTIME_TIMESTAMP"`
			Cursor  string `json:"__CURSOR"`
		}
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		ts := time.Now()
		if e.Time != "" {
			if us, err := parseJournalTime(e.Time); err == nil {
				ts = time.Unix(0, us*1000)
			}
		}
		if e.Message == "" {
			continue
		}
		entries = append(entries, Entry{
			Timestamp: ts,
			Message:   e.Message,
			Labels: map[string]string{
				"instance": c.hostname,
				"hostname": c.hostname,
				"unit":     c.unit,
				"source":   "journald",
			},
			Fields: map[string]string{
				"unit":   c.unit,
				"cursor": e.Cursor,
			},
		})
	}

	return entries, nil
}

func sanitizeFileName(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

func parseJournalTime(v string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(v), 10, 64)
}
