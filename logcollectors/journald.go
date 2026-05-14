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

	cursorFile := filepath.Join(c.stateDir, sanitizeFileName(c.unit)+".cursor")
	args := []string{"--no-pager", "--output=json", "--unit=" + c.unit}
	
	// Use -D flag to read from host journal if available (in container, host journal is usually mounted elsewhere)
	if _, err := os.Stat("/host/journal"); err == nil {
		args = append(args, "-D", "/host/journal")
	}
	
	args = append(args, "--cursor-file="+cursorFile)
	
	if _, err := os.Stat(cursorFile); os.IsNotExist(err) {
		// No cursor file yet — read the last hour to bootstrap.
		// Using --since=now would result in zero entries if no SSH activity
		// at this exact moment, preventing journalctl from creating the cursor file.
		args = append(args, "--since=-1h")
	}

	cmd := exec.Command("journalctl", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Exit status 1 means "no entries found" - not a real error
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, nil
		}
		// If output is empty (other error), it's a real error
		if len(out) == 0 {
			return nil, fmt.Errorf("journalctl failed for unit %q: %v", c.unit, err)
		}
		// Otherwise try to parse what we got
	}

	var entries []Entry
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var e struct {
			Message  string `json:"MESSAGE"`
			Time     string `json:"__REALTIME_TIMESTAMP"`
			Cursor   string `json:"__CURSOR"`
			Priority string `json:"PRIORITY"`
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

		level := priorityToLevel(e.Priority)

		entries = append(entries, Entry{
			Timestamp: ts,
			Message:   e.Message,
			Labels: map[string]string{
				"instance": c.hostname,
				"hostname": c.hostname,
				"unit":     c.unit,
				"source":   "journald",
				"level":    level,
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

// priorityToLevel maps journald PRIORITY values to log levels.
// syslog priorities: 0=emerg,1=alert,2=crit → error, 3=err,4=warning → warn,
// 5=notice,6=info → info, 7=debug → debug.
func priorityToLevel(p string) string {
	if p == "" {
		return "unknown"
	}
	n, err := strconv.Atoi(p)
	if err != nil {
		return "unknown"
	}
	switch {
	case n <= 2:
		return "error"
	case n <= 4:
		return "warn"
	case n <= 6:
		return "info"
	case n == 7:
		return "debug"
	default:
		return "unknown"
	}
}
