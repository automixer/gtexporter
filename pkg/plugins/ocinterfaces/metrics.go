package ocinterfaces

import (
	"github.com/prometheus/client_golang/prometheus"

	// Local packages
	"github.com/automixer/gtexporter/pkg/exporter"
)

// ifKind is a custom type representing different kinds of interfaces.
type ifKind int

// String returns the string representation of the ifKind enum.
func (k ifKind) String() string {
	switch k {
	case kindIface:
		return "iface"
	case kindIfaceLag:
		return "iface_lag"
	case kindIfaceLagMember:
		return "iface_lag_member"
	case kindSubIface:
		return "sub_iface"
	case kindSubIfaceLag:
		return "sub_iface_lag"
	case kindSubIfaceLagMember:
		return "sub_iface_lag_member"
	default:
		return "unknown"
	}
}

const (
	_ ifKind = iota
	kindIface
	kindIfaceLag
	kindIfaceLagMember
	kindSubIface
	kindSubIfaceLag
	kindSubIfaceLagMember
)

// ocIfMetric represents a metric emitted by the Openconfig Interfaces package.
type ocIfMetric struct {
	exporter.MetricCommons
	Kind        string `label:"kind"`
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
	metric.Name = "oc_if"
	metric.Help = "Openconfig Interfaces Metric"
	metric.Device = f.config.DevName
	metric.Type = mType
	metric.CustomLabel = f.config.CustomLabel
	return metric
}
