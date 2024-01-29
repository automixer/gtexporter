package oclldp

import (
	"github.com/prometheus/client_golang/prometheus"

	// Local packages
	"github.com/automixer/gtexporter/pkg/exporter"
)

// ocLldpIfNbrMetric represents the Openconfig LLDP Interface Neighbors Metric.
//
// Fields:
// - Metric: Name of the metric.
// - CustomLabel: Custom label associated with the metric.
// - IfName: Local interface name.
// - SystemName: Neighbour system name.
// - PortId: Neighbour port ID.
// - PortIdType: Neighbour port ID type.
// - PortDescription: Neighbour port description.
type ocLldpIfNbrMetric struct {
	exporter.MetricCommons
	Metric          string `label:"metric"`
	CustomLabel     string `label:"custom_label"`
	IfName          string `label:"local_if_name"`
	SystemName      string `label:"nbr_system_name"`
	PortId          string `label:"nbr_port_id"`
	PortIdType      string `label:"nbr_port_id_type"`
	PortDescription string `label:"nbr_port_description"`
}

// newLldpIfNbrMetric creates a new ocLldpIfNbrMetric with the given metric type.
func (f *ocLldpFormatter) newLldpIfNbrMetric(mType prometheus.ValueType) ocLldpIfNbrMetric {
	metric := ocLldpIfNbrMetric{}
	// Common fields
	metric.Name = "oc_lldp_if_nbr"
	metric.Help = "Openconfig LLDP Interface Neighbors Metric"
	metric.Device = f.config.DevName
	metric.Type = mType
	metric.CustomLabel = f.config.CustomLabel
	return metric
}
