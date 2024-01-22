package ocinterfaces

import (
	"github.com/prometheus/client_golang/prometheus"

	// Local packages
	"github.com/automixer/gtexporter/pkg/exporter"
)

// ocIfMetric represents a metric emitted by the Openconfig Interfaces package.
type ocIfMetric struct {
	exporter.MetricCommons
	Metric      string `label:"metric"`
	CustomLabel string `label:"custom_label"`
	IfName      string `label:"name"`
	IfRealName  string `label:"real_name"`
	IfIndex     string `label:"index"`
	IfType      string `label:"if_type"`
	SnmpIndex   string `label:"if_index"`
	Description string `label:"description"`
	AdminStatus string `label:"admin_status"`
	OperStatus  string `label:"oper_status"`
	LagType     string `label:"lag_type"`
}

// newIfMetric creates a new ocIfMetric with the given metric type.
func (f *ocIfFormatter) newIfMetric(mType prometheus.ValueType) ocIfMetric {
	metric := ocIfMetric{}
	// Common fields
	metric.Source = exporter.SrcPlugin
	metric.Name = f.config.PlugName
	metric.Help = "Openconfig Interfaces Metric"
	metric.Device = f.config.DevName
	metric.Type = mType
	metric.CustomLabel = f.config.CustomLabel
	return metric
}
