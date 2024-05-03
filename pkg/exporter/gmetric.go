package exporter

import (
	"errors"
	"github.com/prometheus/client_golang/prometheus"
	"reflect"
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
	Name   string // Name of the metric
	Help   string // Help string for Prom metric description
	Device string // Device name (gnmi client)
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

// buildFQName builds a fully qualified metric name using the provided prefix and MetricCommons.
// It appends "_counters" or "_gauges" to the metric name based on its Type.
// Parameters:
// - pfx: the prefix for the metric name
// - mc: the MetricCommons object containing the metric name and type
// Returns the fully qualified metric name as a string.
func buildFQName(pfx string, mc MetricCommons) string {
	fqName := prometheus.BuildFQName(pfx, "", mc.Name)
	switch mc.getCommons().Type {
	case prometheus.CounterValue:
		fqName += "_total"
	case prometheus.GaugeValue:
		fqName += "_gauges"
	case prometheus.UntypedValue:
	}
	return fqName
}
