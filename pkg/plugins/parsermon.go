package plugins

import (
	"github.com/prometheus/client_golang/prometheus"
	"reflect"
	"sync"

	// Local packages
	"github.com/automixer/gtexporter/pkg/exporter"
)

// pmCounters represents a set of performance monitoring counters for a generic parser instance.
// The available counters in pmCounters are:
//   - Duplicates: Tracks the number of coalesced duplicates of the GNMI updates received.
//   - DeleteNotFound: Tracks the number of times a path of a delete message was not found.
//   - ContainerNotFound: Tracks the number of times the update's YANG container was not found.
//   - LeafNotFound: Tracks the number of times the update's YANG leaf was not found.
//   - InvalidPath: Tracks the number of times an invalid GNMI path was encountered.
type pmCounters struct {
	Duplicates        uint64 `label:"gnmi_update_duplicates"`
	DeleteNotFound    uint64 `label:"delete_path_not_found"`
	ContainerNotFound uint64 `label:"yang_container_not_found"`
	LeafNotFound      uint64 `label:"yang_leaf_not_found"`
	InvalidPath       uint64 `label:"invalid_gnmi_path"`
}

type ParserMon struct {
	Cfg      Config
	counters pmCounters
	mutex    sync.Mutex
}

// Configure takes a Config struct and assigns it to the Cfg field of the ParserMon struct.
func (p *ParserMon) Configure(cfg Config) error {
	p.Cfg = cfg
	return nil
}

// Describe implements the plugin's parser interface.
// It returns an GMetric to describe the metric itself.
func (p *ParserMon) Describe() []exporter.GMetric {
	return []exporter.GMetric{newParserMetric(prometheus.CounterValue, p.Cfg.DevName)}
}

// Collect implements the plugin's parser interface.
// It uses reflection to iterate over the pmCounters struct and creates a new GMetric for each counter.
func (p *ParserMon) Collect() []exporter.GMetric {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Counters
	rType := reflect.TypeOf(p.counters)
	rValue := reflect.ValueOf(p.counters)
	out := make([]exporter.GMetric, 0, rType.NumField())
	for i := 0; i < rType.NumField(); i++ {
		metric := newParserMetric(prometheus.CounterValue, p.Cfg.DevName)
		metric.PlugName = p.Cfg.PlugName
		metric.Metric = rType.Field(i).Tag.Get("label")
		metric.Value = float64(rValue.Field(i).Uint())
		out = append(out, metric)
	}
	return out
}

// UpdateDuplicates takes an uint64 parameter representing the value of the duplicates field of the update.
func (p *ParserMon) UpdateDuplicates(dups uint64) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.counters.Duplicates += dups
}

func (p *ParserMon) DeleteNotFound() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.counters.DeleteNotFound++
}

func (p *ParserMon) ContainerNotFound() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.counters.ContainerNotFound++
}

func (p *ParserMon) LeafNotFound() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.counters.LeafNotFound++
}

func (p *ParserMon) InvalidPath() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.counters.InvalidPath++
}
