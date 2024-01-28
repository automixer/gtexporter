package oclldp

import (
	log "github.com/golang/glog"
	"github.com/openconfig/ygot/ygot"
	"github.com/prometheus/client_golang/prometheus"

	// Local packages
	"github.com/automixer/gtexporter/pkg/datamodels/dmoclldp"
	"github.com/automixer/gtexporter/pkg/exporter"
	"github.com/automixer/gtexporter/pkg/plugins"
)

const (
	plugName  = "oc_lldp"
	dataModel = "openconfig-lldp"
	// Paths to subscribe
	lldpNbState = "/lldp/interfaces/interface/neighbors/neighbor/state"
)

// init register the parser and the formatter to the plugin registration system
func init() {
	err := plugins.Register(plugName, newFormatter, newParser)
	if err != nil {
		log.Error(err)
	}
}

type ocLldpFormatter struct {
	config plugins.Config
	root   *dmoclldp.Root
}

func newFormatter(cfg plugins.Config) (plugins.Formatter, error) {
	f := &ocLldpFormatter{}
	f.config = cfg
	return f, nil
}

func (f *ocLldpFormatter) GetPaths() plugins.FormatterPaths {
	return plugins.FormatterPaths{
		XPaths:     []string{lldpNbState},
		Datamodels: []string{dataModel},
	}
}

func (f *ocLldpFormatter) Describe() []exporter.GMetric {
	return []exporter.GMetric{f.newLldpIfNbrMetric(prometheus.GaugeValue)}
}

func (f *ocLldpFormatter) Collect() []exporter.GMetric {
	out := make([]exporter.GMetric, 0)
	out = append(out, f.lldpIfNbrGauges()...)
	return out
}

func (f *ocLldpFormatter) ScrapeEvent(ys ygot.GoStruct) func() {
	f.root = dmoclldp.GoStructToOcLldp(ys)
	return func() {
		f.root = nil
	}
}

func (f *ocLldpFormatter) lldpIfNbrGauges() []exporter.GMetric {
	out := make([]exporter.GMetric, 0, len(f.root.GetLldp().Interface))
	gauges := make(map[string]float64, 3)

	for ifName, ifObject := range f.root.GetLldp().Interface {
		for _, nbrObject := range ifObject.Neighbor {
			// Read gauges values from GoStruct
			gauges["age"] = float64(nbrObject.GetAge())
			gauges["last_update"] = float64(nbrObject.GetLastUpdate())
			gauges["ttl"] = float64(nbrObject.GetTtl())
			// Create metrics
			for gaugeName, gaugeValue := range gauges {
				metric := f.newLldpIfNbrMetric(prometheus.GaugeValue)
				metric.Metric = gaugeName
				metric.Value = gaugeValue
				metric.IfName = ifName
				metric.SystemName = nbrObject.GetSystemName()
				metric.PortId = nbrObject.GetPortId()
				metric.PortIdType = nbrObject.GetPortIdType().ShortString()
				metric.PortDescription = nbrObject.GetPortDescription()
				out = append(out, metric)
			}
		}
	}
	return out
}
