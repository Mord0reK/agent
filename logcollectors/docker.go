package logcollectors

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type containerState struct {
	id     string
	name   string
	offset int64
}

type DockerCollector struct {
	hostname      string
	pattern       string
	containers    map[string]*containerState
	resolved      bool
	lastResolved  time.Time
}

type dockerJSONLine struct {
	Log    string `json:"log"`
	Time   string `json:"time"`
	Stream string `json:"stream"`
}

func NewDockerCollector(hostname, pattern string) *DockerCollector {
	return &DockerCollector{
		hostname:   hostname,
		pattern:    pattern,
		containers: make(map[string]*containerState),
	}
}

func (c *DockerCollector) Name() string { return "docker" }

// isWildcard checks if pattern contains glob characters
func isWildcard(pattern string) bool {
	return strings.Contains(pattern, "*") || strings.Contains(pattern, "?")
}

func (c *DockerCollector) resolve() error {
	if c.resolved && time.Since(c.lastResolved) < 60*time.Second {
		return nil
	}

	matches, err := filepath.Glob("/var/lib/docker/containers/*/config.v2.json")
	if err != nil {
		return err
	}

	foundAny := false
	useWildcard := isWildcard(c.pattern)

	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var cfg struct {
			Name string `json:"Name"`
		}
		if err := json.Unmarshal(data, &cfg); err != nil {
			continue
		}
		name := strings.TrimPrefix(cfg.Name, "/")

		// Check if name matches pattern
		var matches bool
		if useWildcard {
			matches, _ = filepath.Match(c.pattern, name)
		} else {
			matches = (name == c.pattern)
		}

		if !matches {
			continue
		}

		id := filepath.Base(filepath.Dir(path))
		logPath := filepath.Join("/var/lib/docker/containers", id, fmt.Sprintf("%s-json.log", id))

		offset := int64(0)
		if st, err := os.Stat(logPath); err == nil {
			offset = st.Size()
		}

		c.containers[name] = &containerState{
			id:     id,
			name:   name,
			offset: offset,
		}
		foundAny = true
	}

	if !foundAny {
		if useWildcard {
			return fmt.Errorf("no containers matching pattern %q", c.pattern)
		}
		return fmt.Errorf("container %q not found", c.pattern)
	}

	c.resolved = true
	c.lastResolved = time.Now()
	return nil
}

func (c *DockerCollector) Collect() ([]Entry, error) {
	if err := c.resolve(); err != nil {
		return nil, err
	}

	var out []Entry

	for _, cs := range c.containers {
		logPath := filepath.Join("/var/lib/docker/containers", cs.id, fmt.Sprintf("%s-json.log", cs.id))

		f, err := os.OpenFile(logPath, os.O_RDONLY, 0)
		if err != nil {
			continue
		}

		st, err := f.Stat()
		if err != nil {
			f.Close()
			continue
		}

		fileSize := st.Size()

		// If file was truncated (log rotation), start from beginning
		if fileSize < cs.offset {
			cs.offset = 0
		}

		// Nothing new since last collect
		if fileSize <= cs.offset {
			f.Close()
			continue
		}

		// Read all new data from offset to end of file
		// Safety limit: max 50MB per cycle
		remaining := fileSize - cs.offset
		if remaining > 50*1024*1024 {
			remaining = 50 * 1024 * 1024
		}

		// Read full new data into memory
		data := make([]byte, remaining)
		n, err := f.ReadAt(data, cs.offset)
		if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
			f.Close()
			continue
		}
		data = data[:n] // trim to actual bytes read
		f.Close()

		if len(data) == 0 {
			continue
		}

		// Drop trailing incomplete line (data may end without newline)
		// — it will be picked up in the next cycle after more data is appended
		dataLen := len(data)
		if dataLen > 0 && data[dataLen-1] != '\n' {
			lastNewline := bytes.LastIndexByte(data, '\n')
			if lastNewline < 0 {
				// No complete line at all, keep old offset
				continue
			}
			// Truncate to last complete line, leave the rest for next cycle
			dataLen = lastNewline + 1
			data = data[:dataLen]
		}

		if dataLen == 0 {
			continue
		}

		// Process complete lines
		scanner := NewLineScanner(data)
		for scanner.Scan() {
			var line dockerJSONLine
			if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
				continue
			}
			msg := strings.TrimRight(line.Log, "\n")
			if msg == "" {
				continue
			}

			ts, err := time.Parse(time.RFC3339Nano, line.Time)
			if err != nil {
				ts = time.Now()
			}

			out = append(out, Entry{
				Timestamp: ts,
				Message:   msg,
				Labels: map[string]string{
					"instance":  c.hostname,
					"hostname":  c.hostname,
					"container": cs.name,
					"source":    "docker",
					"level":     extractLevel(msg),
				},
				Fields: map[string]string{
					"stream": line.Stream,
				},
			})
		}

		if err := scanner.Err(); err != nil {
			// If there was an error (e.g., line too long), we still advance
			// past the data we read to avoid infinite retry loops.
			cs.offset += int64(dataLen)
			continue
		}

		// Successfully processed all data — advance offset
		cs.offset += int64(dataLen)
	}

	return out, nil
}

// lineScanner is a simple line-based scanner that handles long lines gracefully.
// Unlike bufio.Scanner, it doesn't stop on ErrTooLong — it skips the long line and continues.
type lineScanner struct {
	data   []byte
	pos    int
	line   []byte
	err    error
}

func NewLineScanner(data []byte) *lineScanner {
	return &lineScanner{data: data}
}

func (s *lineScanner) Scan() bool {
	if s.err != nil || s.pos >= len(s.data) {
		return false
	}

	// Find next newline
	end := bytes.IndexByte(s.data[s.pos:], '\n')
	if end < 0 {
		// Last line without newline — still process it
		s.line = s.data[s.pos:]
		s.pos = len(s.data)
		return len(s.line) > 0
	}

	s.line = s.data[s.pos : s.pos+end]
	s.pos += end + 1 // skip the \n
	return true
}

func (s *lineScanner) Bytes() []byte {
	return s.line
}

func (s *lineScanner) Err() error {
	return s.err
}

// extractLevel attempts to determine the log level from a message string.
// If the message is JSON, it looks for known level keys.
// Otherwise, it falls back to keyword matching on the lowercase message.
func extractLevel(msg string) string {
	if strings.HasPrefix(msg, "{") {
		var fields map[string]interface{}
		if err := json.Unmarshal([]byte(msg), &fields); err == nil {
			for _, key := range []string{"level", "severity", "lvl", "loglevel"} {
				if v, ok := fields[key]; ok {
					if s, ok := v.(string); ok {
						return strings.ToLower(s)
					}
				}
			}
		}
	}

	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "error") || strings.Contains(lower, "err="):
		return "error"
	case strings.Contains(lower, "warn"):
		return "warn"
	case strings.Contains(lower, "debug"):
		return "debug"
	case strings.Contains(lower, "info"):
		return "info"
	default:
		return "unknown"
	}
}
