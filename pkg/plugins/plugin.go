package plugins

import (
	"fmt"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/ygot"
	"github.com/prometheus/client_golang/prometheus"
	"sync"
	"time"

	// Local packages
	"github.com/automixer/gtexporter/pkg/exporter"
)

// FormatterPaths represents the paths to be subscribed by the client on behalf of the formatter needs.
type FormatterPaths struct {
	XPaths     []string
	Datamodels []string
}

// Formatter is an interface that defines the methods required from a formatter object.
// A formatter object is responsible for building metrics pulling values from the chosen yGot GoStruct.
type Formatter interface {
	Describe() []exporter.GMetric
	Collect() []exporter.GMetric
	GetPaths() FormatterPaths
	ScrapeEvent(ys ygot.GoStruct) func()
}

// Parser represents an interface that defines the methods required from a parser object.
// A parser object is responsible for loading the received GNMI data into the chosen yGot GoStruct.
type Parser interface {
	Describe() []exporter.GMetric
	Collect() []exporter.GMetric
	CheckOut() ygot.GoStruct
	ParseNotification(nf *gnmi.Notification)
	ClearCache()
}

type Config struct {
	DevName        string
	PlugName       string
	CustomLabel    string
	UseGoDefaults  bool
	CacheData      bool
	ScrapeInterval time.Duration
}

// Plugin represents a plugin that collects metrics using a formatter and parser.
type Plugin struct {
	config         Config
	mutex          sync.Mutex
	buf            *uBuffer
	onSync         bool
	formatter      Formatter
	parser         Parser
	formatterInfos FormatterPaths
}

func New(cfg Config) (*Plugin, error) {
	plug := &Plugin{config: cfg}
	plug.buf = newBuf(cfg.ScrapeInterval)

	// Load plugin formatter
	if _, ok := formatters[cfg.PlugName]; !ok {
		return nil, fmt.Errorf("formatter %s not registered", cfg.PlugName)
	}
	formatter, err := formatters[cfg.PlugName](cfg)
	if err != nil {
		return nil, err
	}
	plug.formatter = formatter
	plug.formatterInfos = plug.formatter.GetPaths()

	// Load plugin parser
	if _, ok := parsers[cfg.PlugName]; !ok {
		return nil, fmt.Errorf("parser %s not registered", cfg.PlugName)
	}
	parser, err := parsers[cfg.PlugName](cfg)
	if err != nil {
		return nil, err
	}
	plug.parser = parser

	// Prepare descriptors for registration
	desc := formatter.Describe()                                                        // User metrics from formatter
	desc = append(desc, newFormatterMetric(prometheus.GaugeValue, plug.config.DevName)) // Formatter self monitoring
	desc = append(desc, parser.Describe()...)                                           // Parser self monitoring

	// Register plugin to exporter
	if err := exporter.Registry(plug, desc); err != nil {
		return nil, err
	}
	return plug, nil
}

// GetPlugName retrieves the name of the plugin from its configuration.
func (p *Plugin) GetPlugName() string {
	return p.config.PlugName
}

// GetPathsToSubscribe retrieves the list of XPaths that the plugin should subscribe to for data collection.
func (p *Plugin) GetPathsToSubscribe() []string {
	return p.formatterInfos.XPaths
}

// GetDataModels returns the list of data models associated with the plugin's formatter xPaths.
func (p *Plugin) GetDataModels() []string {
	return p.formatterInfos.Datamodels
}

// GetMetrics implements the exporter GMetricSource interface
// It is called by the exporter, and it sends the output of the formatter object.
func (p *Plugin) GetMetrics(ch chan<- exporter.GMetric) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// If passthrough mode, send nf buffer to parser
	if !p.config.CacheData {
		buf := p.buf.checkout()
		for _, nf := range buf {
			p.parser.ParseNotification(nf)
		}
	}

	// Check out the yGot GoStruct and send it to the formatter
	ys := p.parser.CheckOut()
	endScrape := p.formatter.ScrapeEvent(ys)
	defer endScrape()

	// Gather metrics from formatter
	mCounter := 0
	for _, m := range p.formatter.Collect() {
		mCounter++
		ch <- m
	}

	// Send formatter self monitoring data
	fMon := newFormatterMetric(prometheus.GaugeValue, p.config.DevName)
	fMon.Metric = "collected_series"
	fMon.Value = float64(mCounter)
	fMon.PlugName = p.config.PlugName
	ch <- fMon

	// Gather self monitoring from parser
	for _, m := range p.parser.Collect() {
		ch <- m
	}

	// If passthrough mode, clear parser yGot GoStruct
	if !p.config.CacheData {
		p.parser.ClearCache()
	}
}

// OnSync sets the synchronization status of the plugin.
// If the previous synchronization status is true and the new status is false,
// it clears the cache in the parser and the uBuffer
func (p *Plugin) OnSync(status bool) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.onSync && !status {
		p.parser.ClearCache()
		p.buf.clearBuffer()
	}
	p.onSync = status
}

// Notification sends the received GNMI notification to the parser, if cache mode is enabled.
// If Passthrough mode is engaged, notifications are temporarily stored into a buffer.
// The buffer content is then sent to the parser when a scrape event occurs.
func (p *Plugin) Notification(nf *gnmi.Notification) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.config.CacheData {
		// Cache mode
		p.parser.ParseNotification(nf)
	} else {
		// Passthrough mode
		p.buf.add(nf)
	}
}
