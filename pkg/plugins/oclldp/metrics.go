package oclldp

import (
	"github.com/automixer/gtexporter/pkg/exporter"
	"github.com/prometheus/client_golang/prometheus"
)

type ocLldpIfMetric struct {
	exporter.MetricCommons
	Metric      string `label:"metric"`
	CustomLabel string `label:"custom_label"`
	IfName      string `label:"name"`
	SystemName  string `label:"system_name"`
	PortId      string `label:"port_id"`
	PortIdType  string `label:"port_id_type"`
}

// newLldpIfMetric creates a new ocLldpIfMetric with the given metric type.
func (f *ocLldpFormatter) newLldpIfMetric(mType prometheus.ValueType) ocLldpIfMetric {
	metric := ocLldpIfMetric{}
	// Common fields
	metric.Source = exporter.SrcPlugin
	metric.Name = f.config.PlugName
	metric.Help = "Openconfig LLDP Metric"
	metric.Device = f.config.DevName
	metric.Type = mType
	metric.CustomLabel = f.config.CustomLabel
	return metric
}
