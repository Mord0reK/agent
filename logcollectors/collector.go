package logcollectors

import "time"

type Entry struct {
	Timestamp time.Time
	Message   string
	Labels    map[string]string
	Fields    map[string]string
}

type Collector interface {
	Name() string
	Collect() ([]Entry, error)
}
