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

type DockerCollector struct {
	hostname    string
	container   string
	containerID string
	logPath     string
	offset      int64
	resolved    bool
}

type dockerJSONLine struct {
	Log    string `json:"log"`
	Time   string `json:"time"`
	Stream string `json:"stream"`
}

func NewDockerCollector(hostname, container string) *DockerCollector {
	return &DockerCollector{hostname: hostname, container: container}
}

func (c *DockerCollector) Name() string { return "docker" }

func (c *DockerCollector) resolve() error {
	if c.resolved {
		return nil
	}

	matches, err := filepath.Glob("/var/lib/docker/containers/*/config.v2.json")
	if err != nil {
		return err
	}

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
		if name != c.container {
			continue
		}
		id := filepath.Base(filepath.Dir(path))
		c.containerID = id
		c.logPath = filepath.Join("/var/lib/docker/containers", id, fmt.Sprintf("%s-json.log", id))
		if st, err := os.Stat(c.logPath); err == nil {
			c.offset = st.Size()
		}
		c.resolved = true
		return nil
	}

	return fmt.Errorf("container %q not found", c.container)
}

func (c *DockerCollector) Collect() ([]Entry, error) {
	if err := c.resolve(); err != nil {
		return nil, err
	}

	f, err := os.Open(c.logPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if st.Size() < c.offset {
		c.offset = 0
	}

	if _, err := f.Seek(c.offset, io.SeekStart); err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var out []Entry
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
				"container": c.container,
				"source":    "docker",
			},
			Fields: map[string]string{
				"container_id": c.containerID,
				"stream":       line.Stream,
			},
		})
	}

	if err := scanner.Err(); err != nil {
		return out, err
	}

	pos, err := f.Seek(0, io.SeekCurrent)
	if err == nil {
		c.offset = pos
	}

	return out, nil
}
