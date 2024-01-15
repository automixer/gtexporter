package exporter

import (
	"errors"
	"github.com/prometheus/client_golang/prometheus"
	"reflect"
)

// MetricSourceEnum is an enumeration type that represents all the valid source of metrics.
type MetricSourceEnum int

func (s MetricSourceEnum) String() string {
	switch s {
	case SrcGnmiClient:
		return "gclient"
	case SrcFormatter:
		return "formatter"
	case SrcParser:
		return "parser"
	case SrcPlugin:
		return "plugin"
	default:
		return ""
	}
}

const (
	SrcUnknown MetricSourceEnum = iota
	SrcGnmiClient
	SrcFormatter
	SrcParser
	SrcPlugin
)

// GMetric is an interface that represents a generic metric.
// User defined metrics must implement this interface
type GMetric interface {
	getCommons() MetricCommons
	validate() error
}

// MetricCommons represents a common set of keys of a metric used in the application.
// Metric sources must embed this structure into their user defined metrics
type MetricCommons struct {
	Source MetricSourceEnum // Source name of the sender module (gnmi client, plug name, parser, etc.)
	Name   string           // Name of the metric
	Help   string           // Help string for Prom metric description
	Device string           // Device name (gnmi client)
	Type   prometheus.ValueType
	Value  float64
}

// getCommons returns a copy of the MetricCommons object on which it is invoked.
func (m MetricCommons) getCommons() MetricCommons {
	return m
}

// validate checks if the MetricCommons content is valid.
// It returns an error if any of the required fields is missing.
func (m MetricCommons) validate() error {
	if m.Source == SrcUnknown {
		return errors.New("source is required")
	}
	if m.Name == "" {
		return errors.New("name is required")
	}
	if m.Device == "" {
		return errors.New("device is required")
	}
	return nil
}

// getLabelKeys retrieves the keys of the labeled fields in the provided GMetric object.
// Fields key names from user defined metrics are extracted by this method using reflection and the "label" tag.
func getLabelKeys(m GMetric) []string {
	rType := reflect.TypeOf(m)
	labelKeys := make([]string, 0, rType.NumField())
	for i := 0; i < rType.NumField(); i++ {
		if lk, ok := rType.Field(i).Tag.Lookup("label"); ok {
			labelKeys = append(labelKeys, lk)
		}
	}
	if len(labelKeys) == 0 {
		return nil
	}
	return labelKeys
}

// getLabelValues retrieves the string values of the labeled fields in the provided GMetric object.
// Fields key values from user defined metrics are extracted by this method using reflection and the "label" tag.
func getLabelValues(m GMetric) []string {
	rType := reflect.TypeOf(m)
	rValue := reflect.ValueOf(m)
	labelValues := make([]string, 0, rValue.NumField())
	for i := 0; i < rValue.NumField(); i++ {
		if _, ok := rType.Field(i).Tag.Lookup("label"); ok {
			lv := rValue.Field(i).String()
			labelValues = append(labelValues, lv)
		}
	}
	if len(labelValues) == 0 {
		return nil
	}
	return labelValues
}

// buildFQName builds a fully qualified metric name using the provided prefix and MetricCommons object.
// The fqName is constructed by concatenating the prefix, source, and name fields of the MetricCommons object.
// If the type of the MetricCommons object is CounterValue, "_counters" is appended to the fqName.
// If the type of the MetricCommons object is GaugeValue, "_gauges" is appended to the fqName.
// If the type of the MetricCommons object is UntypedValue, nothing is appended to the fqName.
// The resulting fqName is returned.
func buildFQName(pfx string, mh MetricCommons) string {
	fqName := prometheus.BuildFQName(pfx, mh.Source.String(), mh.Name)
	switch mh.getCommons().Type {
	case prometheus.CounterValue:
		fqName += "_counters"
	case prometheus.GaugeValue:
		fqName += "_gauges"
	case prometheus.UntypedValue:
	}
	return fqName
}
