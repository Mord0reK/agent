package collectors

import "time"

type Metric struct {
	Name      string
	Labels    map[string]string
	Value     float64
	Timestamp time.Time
}

type Collector interface {
	Name() string
	Collect() ([]Metric, error)
}
