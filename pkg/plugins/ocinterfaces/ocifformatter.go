package ocinterfaces

import (
	"fmt"
	log "github.com/golang/glog"
	"github.com/openconfig/ygot/ygot"
	"github.com/prometheus/client_golang/prometheus"

	// Local packages
	"github.com/automixer/gtexporter/pkg/datamodels/ocif"
	"github.com/automixer/gtexporter/pkg/exporter"
	"github.com/automixer/gtexporter/pkg/plugins"
)

const (
	plugName  = "oc_interfaces"
	dataModel = "openconfig-interfaces"
	// Paths to subscribe
	ifState    = "/interfaces/interface/state"
	ifAggState = "/interfaces/interface/aggregation/state"
	subIfState = "/interfaces/interface/subinterfaces/subinterface/state"
)

// init register the parser and the formatter to the plugin registration system
func init() {
	err := plugins.Register(plugName, newFormatter, newParser)
	if err != nil {
		log.Error(err)
	}
}

type ocIfFormatter struct {
	config   plugins.Config
	root     *ocif.Root
	lagTable map[string]string // Key: ifName, Value: LAG name
	lagSet   map[string]bool   // Key: lagName
}

func newFormatter(cfg plugins.Config) (plugins.Formatter, error) {
	f := &ocIfFormatter{}
	f.config = cfg
	return f, nil
}

// GetPaths returns the XPaths and datamodels for the ocIfFormatter package.
// It implements the plugin's formatter interface
func (f *ocIfFormatter) GetPaths() plugins.FormatterPaths {
	return plugins.FormatterPaths{
		XPaths:     []string{ifState, ifAggState, subIfState},
		Datamodels: []string{dataModel},
	}
}

// ScrapeEvent implements the plugin's formatter interface.
// It is called by the plugin when a scrape events occurs.
func (f *ocIfFormatter) ScrapeEvent(ys ygot.GoStruct) func() {
	f.root = ocif.GoStructToOcIf(ys)

	// Build LAG tables
	f.lagTable = make(map[string]string, 128)
	f.lagSet = make(map[string]bool, 128)
	for name, iface := range f.root.Interface {
		lag := iface.GetAggregation()
		for _, lagMember := range lag.GetMember() {
			f.lagTable[lagMember] = name
			f.lagSet[name] = true
		}
	}
	return func() {
		f.lagTable = nil
		f.lagSet = nil
		f.root = nil
	}
}

// Describe implements the plugin's formatter interface.
// It returns a slice of GMetric to describe the metrics itself.
func (f *ocIfFormatter) Describe() []exporter.GMetric {
	return []exporter.GMetric{
		f.newIfMetric(prometheus.CounterValue),
		f.newIfMetric(prometheus.GaugeValue),
	}
}

// Collect implements the plugin's formatter interface.
// It calls all the methods to generate Metrics from the yGot GoStruct content.
func (f *ocIfFormatter) Collect() []exporter.GMetric {
	out := f.ifCounters()
	out = append(out, f.ifGauges()...)
	out = append(out, f.subIfCounters()...)
	out = append(out, f.subIfGauges()...)
	return out
}

// ifCounters scans the yGot GoStruct and returns a slice of interface/counters metrics
func (f *ocIfFormatter) ifCounters() []exporter.GMetric {
	out := make([]exporter.GMetric, 0, len(f.root.Interface))
	for name, iface := range f.root.Interface {
		var lagType, realName string
		alias := name

		// Check if the interface is a LAG
		if f.lagSet[name] {
			lagType = iface.GetAggregation().GetLagType().ShortString()
		}

		// Check if the interface is a LAG member
		if _, ok := f.lagTable[name]; ok {
			realName = name
			alias = f.lagTable[name]
		}

		// Set counters pull mode
		pullMode := ocif.Normal
		if f.config.UseGoDefaults {
			pullMode = ocif.UseGoDefault
		}
		if f.lagSet[name] {
			pullMode = ocif.ForceToZero
		}

		// Get counters
		ifCnt := ocif.GetCountersFromStruct(*iface.GetCounters(), pullMode)
		for counterName, counterValue := range ifCnt {
			metric := f.newIfMetric(prometheus.CounterValue)
			// Labels
			metric.IfName = alias
			metric.IfRealName = realName
			metric.SnmpIndex = fmt.Sprint(iface.GetIfindex())
			metric.Description = iface.GetDescription()
			metric.AdminStatus = iface.GetAdminStatus().ShortString()
			metric.OperStatus = iface.GetOperStatus().ShortString()
			metric.IfType = iface.GetType().ShortString()
			metric.LagType = lagType
			// Values
			metric.Metric = counterName
			metric.Value = counterValue
			out = append(out, metric)
		}
	}
	return out
}

// ifGauges scans the yGot GoStruct and returns a slice with the interface gauges metrics
func (f *ocIfFormatter) ifGauges() []exporter.GMetric {
	out := make([]exporter.GMetric, 0, len(f.root.Interface))
	for name, iface := range f.root.Interface {
		var lagType, realName string
		alias := name

		// Build gauges value map
		gauges := map[string]float64{
			"last_change":   float64(iface.GetLastChange()),
			"last_clear":    float64(iface.GetCounters().GetLastClear()),
			"mtu":           float64(iface.GetMtu()),
			"lag_speed":     float64(iface.GetAggregation().GetLagSpeed()),
			"lag_min_links": float64(iface.GetAggregation().GetMinLinks()),
		}

		// Check if the interface is a LAG
		if f.lagSet[name] {
			lagType = iface.GetAggregation().GetLagType().ShortString()
		}

		// Check if the interface is a LAG member
		if _, ok := f.lagTable[name]; ok {
			realName = name
			alias = f.lagTable[name]
		}

		// Build gauge metrics
		for gaugeName, gaugeValue := range gauges {
			metric := f.newIfMetric(prometheus.GaugeValue)
			// Labels
			metric.IfName = alias
			metric.IfRealName = realName
			metric.SnmpIndex = fmt.Sprint(iface.GetIfindex())
			metric.Description = iface.GetDescription()
			metric.AdminStatus = iface.GetAdminStatus().ShortString()
			metric.OperStatus = iface.GetOperStatus().ShortString()
			metric.IfType = iface.GetType().ShortString()
			metric.LagType = lagType
			// Values
			metric.Metric = gaugeName
			metric.Value = gaugeValue
			out = append(out, metric)
		}
	}
	return out
}

// subIfCounters scans the yGot GoStruct and returns a slice of subinterface/counters metrics
func (f *ocIfFormatter) subIfCounters() []exporter.GMetric {
	out := make([]exporter.GMetric, 0, len(f.root.Interface))
	for name, iface := range f.root.Interface {
		var lagType, realName string
		alias := name

		// Check if the interface is a LAG
		if f.lagSet[name] {
			lagType = iface.GetAggregation().GetLagType().ShortString()
		}

		// Check if the interface is a LAG member
		if _, ok := f.lagTable[name]; ok {
			realName = name
			alias = f.lagTable[name]
		}

		// Set counters pull mode
		pullMode := ocif.Normal
		if f.config.UseGoDefaults {
			pullMode = ocif.UseGoDefault
		}
		if f.lagSet[name] {
			pullMode = ocif.ForceToZero
		}

		// Walk subinterfaces
		for index, subIface := range f.root.Interface[name].Subinterface {
			// Get counters
			ifCnt := ocif.GetCountersFromStruct(*subIface.GetCounters(), pullMode)
			for counterName, counterValue := range ifCnt {
				metric := f.newIfMetric(prometheus.CounterValue)
				// Labels
				metric.IfName = alias
				metric.IfRealName = realName
				metric.IfIndex = fmt.Sprint(index)
				metric.SnmpIndex = fmt.Sprint(subIface.GetIfindex())
				metric.Description = subIface.GetDescription()
				metric.AdminStatus = subIface.GetAdminStatus().ShortString()
				metric.OperStatus = subIface.GetOperStatus().ShortString()
				metric.LagType = lagType
				// Values
				metric.Metric = counterName
				metric.Value = counterValue
				out = append(out, metric)
			}
		}
	}
	return out
}

// subIfGauges scans the yGot GoStruct and returns a slice with the subinterfaces gauges metrics
func (f *ocIfFormatter) subIfGauges() []exporter.GMetric {
	out := make([]exporter.GMetric, 0, len(f.root.Interface))
	for name, iface := range f.root.Interface {
		var lagType, realName string
		alias := name

		// Check if the interface is a LAG
		if f.lagSet[name] {
			lagType = iface.GetAggregation().GetLagType().ShortString()
		}

		// Check if the interface is a LAG member
		if _, ok := f.lagTable[name]; ok {
			realName = name
			alias = f.lagTable[name]
		}

		// Walk subinterfaces
		for index, subIface := range f.root.Interface[name].Subinterface {
			// Build gauges value map
			gauges := map[string]float64{
				"last_change":   float64(subIface.GetLastChange()),
				"last_clear":    float64(subIface.GetCounters().GetLastClear()),
				"lag_speed":     float64(iface.GetAggregation().GetLagSpeed()),
				"lag_min_links": float64(iface.GetAggregation().GetMinLinks()),
			}
			// Build gauge metrics
			for gaugeName, gaugeValue := range gauges {
				metric := f.newIfMetric(prometheus.GaugeValue)
				// Labels
				metric.IfName = alias
				metric.IfRealName = realName
				metric.IfIndex = fmt.Sprint(index)
				metric.SnmpIndex = fmt.Sprint(subIface.GetIfindex())
				metric.Description = subIface.GetDescription()
				metric.AdminStatus = subIface.GetAdminStatus().ShortString()
				metric.OperStatus = subIface.GetOperStatus().ShortString()
				metric.LagType = lagType
				// Values
				metric.Metric = gaugeName
				metric.Value = gaugeValue
				out = append(out, metric)
			}
		}
	}
	return out
}
