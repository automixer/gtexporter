package oclldp

import (
	"github.com/prometheus/client_golang/prometheus"

	// Local packages
	"github.com/automixer/gtexporter/pkg/exporter"
)

type ocLldpIfMetric struct {
	exporter.MetricCommons
	Metric          string `label:"metric"`
	CustomLabel     string `label:"custom_label"`
	IfName          string `label:"local_if_name"`
	SystemName      string `label:"nbr_system_name"`
	PortId          string `label:"nbr_port_id"`
	PortIdType      string `label:"nbr_port_id_type"`
	PortDescription string `label:"nbr_port_description"`
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
