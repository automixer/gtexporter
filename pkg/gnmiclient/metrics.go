package gnmiclient

import (
	"github.com/prometheus/client_golang/prometheus"

	// Local packages
	"github.com/automixer/gtexporter/pkg/exporter"
)

// smMetric represents a metric used to monitor a single client instance.
type smMetric struct {
	exporter.MetricCommons
	Metric string `label:"metric"`
}

// newMetric creates a new smMetric object with the provided MetricType and initializes its headers.
func (m *clientMon) newMetric(mType prometheus.ValueType) smMetric {
	metric := smMetric{}
	// Headers
	metric.Source = exporter.SrcGnmiClient
	metric.Name = "statistics"
	metric.Help = "Gnmi client statistics"
	metric.Device = m.devName
	metric.Type = mType
	return metric
}
