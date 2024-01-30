package exporter

import (
	"context"
	"errors"
	log "github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
	"sync"
)

// Registry is a variable of type func(src GMetricSource, metrics []GMetric) error.
// It is used to register metric sources with the promExporter.
var Registry func(src GMetricSource, metrics []GMetric) error

// GMetricSource is an interface for objects that provide metrics.
type GMetricSource interface {
	GetMetrics(ch chan<- GMetric)
}

type Config struct {
	ListenAddress string
	ListenPort    string
	ListenPath    string
	InstanceName  string
	MetricPrefix  string
}

type promExporter struct {
	config     Config
	httpServer *http.Server
	mutex      sync.Mutex

	descriptors   map[string]*prometheus.Desc // Key: metric FQName
	metricSources map[GMetricSource]bool      // Key: metric source
}

// New creates a new promExporter instance with the provided configuration.
func New(cfg Config) (*promExporter, error) {
	pExp := &promExporter{config: cfg}
	Registry = pExp.registerSource
	pExp.descriptors = make(map[string]*prometheus.Desc)
	// Note: SelfMon sources are collected after Metric sources
	pExp.metricSources = make(map[GMetricSource]bool)
	return pExp, nil
}

// Start method starts the Prometheus exporter by performing the following steps:
// It must be called after metric sources registration and is non-blocking
func (p *promExporter) Start() error {
	lAddr := p.config.ListenAddress + ":" + p.config.ListenPort
	if err := prometheus.Register(p); err != nil {
		return err
	}
	http.Handle(p.config.ListenPath, promhttp.Handler())
	p.httpServer = &http.Server{Addr: lAddr}
	go func() { log.Info(p.httpServer.ListenAndServe()) }()
	return nil
}

// Close stops the Prometheus exporter and unregisters all metric sources.
func (p *promExporter) Close() {
	if p.httpServer != nil {
		p.unRegisterAllSources()
		err := p.httpServer.Shutdown(context.Background())
		if err != nil {
			log.Error(err)
		}
	}
}

// Describe implements the Prometheus collector interface
// The method iterates over the descriptors map of the promExporter and sends each descriptor to the channel.
// The purpose of this method is to allow Prometheus to collect the metadata about the metrics.
// This method is automatically called by Prometheus when the exporter is registered to Prom.
func (p *promExporter) Describe(ch chan<- *prometheus.Desc) {
	for _, desc := range p.descriptors {
		ch <- desc
	}
}

// Collect implements the Prometheus collector interface
// It starts a goroutine to handle the gathering of metrics from the sources concurrently.
// The goroutine receives metrics from a channel, validates them, and prepares them for sending to Prometheus.
// This method is called by Prometheus when a scrape event occurs.
func (p *promExporter) Collect(ch chan<- prometheus.Metric) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Start gatherer goroutine
	mChan := make(chan GMetric)
	done := make(chan struct{})
	go func() {
		for gMetric := range mChan {
			if gMetric == nil {
				log.Error("Received nil from a metric source")
				continue
			}
			// Get metric commons and fetch the Prom descriptor
			commons := gMetric.getCommons()
			if err := commons.validate(); err != nil {
				log.Error(err)
				continue
			}
			desc, ok := p.descriptors[buildFQName(p.config.MetricPrefix, commons)]
			if !ok {
				log.Error("metric descriptor not found")
				continue
			}
			// Prepare labels
			lv := []string{p.config.InstanceName, commons.Device}
			lv = append(lv, getLabelValues(gMetric)...)
			// Send metric to Prom
			pMetric, err := prometheus.NewConstMetric(desc, commons.Type, commons.Value, lv...)
			if err != nil {
				log.Error("cannot send a malformed metric to prometheus")
				continue
			}
			ch <- pMetric
		}
		close(done)
	}()

	var wg sync.WaitGroup
	// Gather data from metric sources
	for mSource := range p.metricSources {
		wg.Add(1)
		go func(s GMetricSource) {
			s.GetMetrics(mChan)
			wg.Done()
		}(mSource)
	}
	wg.Wait()

	// End collection
	close(mChan)
	<-done
}

// registerSource registers a metric source and its corresponding metrics with the promExporter.
// This method is assigned to the global Registry variable
func (p *promExporter) registerSource(src GMetricSource, metrics []GMetric) error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Sanity check
	if len(metrics) == 0 {
		return errors.New("no metrics provided")
	}
	if src == nil {
		return errors.New("cannot register a nil source")
	}

	// Metric source registration
	p.metricSources[src] = true

	// Prometheus descriptor creation
	for _, m := range metrics {
		if m == nil {
			return errors.New("cannot register a nil metric")
		}
		commons := m.getCommons()
		if err := commons.validate(); err != nil {
			return err
		}
		fqName := buildFQName(p.config.MetricPrefix, commons)
		if _, ok := p.descriptors[fqName]; ok {
			// This is the case where different sources register the same metric. (e.g.: Self monitoring)
			continue
		}
		labelKeys := []string{"instance_name", "device"}
		labelKeys = append(labelKeys, getLabelKeys(m)...)
		p.descriptors[fqName] = prometheus.NewDesc(fqName, commons.Help, labelKeys, nil)
	}
	return nil
}

// unRegisterAllSources method removes all metric sources from the promExporter.
func (p *promExporter) unRegisterAllSources() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.metricSources = nil
}
