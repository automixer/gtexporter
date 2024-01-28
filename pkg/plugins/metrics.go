package plugins

import (
	"github.com/prometheus/client_golang/prometheus"

	// Local packages
	"github.com/automixer/gtexporter/pkg/exporter"
)

// smMetric is a struct used for self monitoring tasks. It contains common fields inherited from
// exporter.MetricCommons, as well as additional fields specific to the
// SM exporter.
type smMetric struct {
	exporter.MetricCommons
	PlugName string `label:"plugin_name"`
	Metric   string `label:"metric"`
}

// newParserMetric creates a new empty smMetric object to be used by the parser.
func newParserMetric(mType prometheus.ValueType, devName string) smMetric {
	metric := smMetric{}
	// Common fields
	metric.Name = "plugin_parser"
	metric.Help = "Plugin parser statistics"
	metric.Device = devName
	metric.Type = mType
	return metric
}

// newFormatterMetric creates a new empty smMetric object to be used by the plugin.
func newFormatterMetric(mType prometheus.ValueType, devName string) smMetric {
	metric := smMetric{}
	// Common fields
	metric.Name = "plugin_formatter"
	metric.Help = "Plugin formatter statistics"
	metric.Device = devName
	metric.Type = mType
	return metric
}
