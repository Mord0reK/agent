package logcollectors

import (
	"bufio"
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
	hostname   string
	pattern    string
	containers map[string]*containerState
	resolved   bool
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
	if c.resolved {
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
	return nil
}

func (c *DockerCollector) Collect() ([]Entry, error) {
	if err := c.resolve(); err != nil {
		return nil, err
	}

	var out []Entry

	for _, cs := range c.containers {
		logPath := filepath.Join("/var/lib/docker/containers", cs.id, fmt.Sprintf("%s-json.log", cs.id))

		f, err := os.Open(logPath)
		if err != nil {
			continue
		}

		st, err := f.Stat()
		if err != nil {
			f.Close()
			continue
		}

		if st.Size() < cs.offset {
			cs.offset = 0
		}

		if _, err := f.Seek(cs.offset, io.SeekStart); err != nil {
			f.Close()
			continue
		}

		scanner := bufio.NewScanner(f)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

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
				},
				Fields: map[string]string{
					"container_id": cs.id,
					"stream":       line.Stream,
				},
			})
		}

		if err := scanner.Err(); err != nil {
			f.Close()
			continue
		}

		pos, err := f.Seek(0, io.SeekCurrent)
		if err == nil {
			cs.offset = pos
		}

		f.Close()
	}

	return out, nil
}
