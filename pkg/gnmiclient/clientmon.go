package gnmiclient

import (
	"github.com/prometheus/client_golang/prometheus"
	"reflect"
	"sync"

	// Local packages
	"github.com/automixer/gtexporter/pkg/exporter"
)

// cmCounters represents the counters of a client instance.
// It includes the following fields:
// - Notifications: counter for the number of GNMI notifications received
// - Updates: counter for the number of GNMI updates received
// - Deletes: counter for the number of GNMI deletes received
// - DialErrors: counter for the number of dial errors encountered
// - CheckCapsErrors: counter for the number of capabilities check errors encountered
// - SubscribeErrors: counter for the number of subscribe errors encountered
// - Disconnections: counter for the number of disconnections
// - SrRoutingErrors: counter for the number of Subscribe Response messages routing errors
type cmCounters struct {
	Notifications   uint64 `label:"gnmi_notifications"`
	Updates         uint64 `label:"gnmi_updates"`
	Deletes         uint64 `label:"gnmi_deletes"`
	DialErrors      uint64 `label:"dial_errors"`
	CheckCapsErrors uint64 `label:"capabilities_errors"`
	SubscribeErrors uint64 `label:"subscribe_errors"`
	Disconnections  uint64 `label:"disconnections"`
	SrRoutingErrors uint64 `label:"sr_routing_errors"`
}

// cmGauges represents the gauges of a client instance.
// It includes the following fields:
// - NfBufUsagePC: gauge for the percentage of fullness of notification buffer.
type cmGauges struct {
	NfBufUsagePC uint64 `label:"notification_buf_usage_pc"`
}

type clientMon struct {
	devName  string
	counters cmCounters
	gauges   cmGauges
	mutex    sync.Mutex
}

// configure sets the device name and prepares metrics for registration.
func (m *clientMon) configure(devName string) error {
	m.devName = devName
	// Prepare metrics for registration
	mList := []exporter.GMetric{
		m.newMetric(prometheus.CounterValue),
		m.newMetric(prometheus.GaugeValue),
	}
	return exporter.Registry(m, mList)
}

// GetMetrics implements the exporter GMetricSource interface
// It is called by the exporter, and it sends the current reading of counters and gauges
func (m *clientMon) GetMetrics(ch chan<- exporter.GMetric) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	// Counters
	rType := reflect.TypeOf(m.counters)
	rValue := reflect.ValueOf(m.counters)
	for i := 0; i < rType.NumField(); i++ {
		metric := m.newMetric(prometheus.CounterValue)
		metric.Metric = rType.Field(i).Tag.Get("label")
		metric.Value = float64(rValue.Field(i).Uint())
		ch <- metric
	}
	// Gauges
	rType = reflect.TypeOf(m.gauges)
	rValue = reflect.ValueOf(m.gauges)
	for i := 0; i < rType.NumField(); i++ {
		metric := m.newMetric(prometheus.GaugeValue)
		metric.Metric = rType.Field(i).Tag.Get("label")
		metric.Value = float64(rValue.Field(i).Uint())
		ch <- metric
	}
	// Reset the nf buffer usage gauge
	m.gauges.NfBufUsagePC = 0
}

func (m *clientMon) incNfCounters(upd, del uint64) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.counters.Notifications++
	m.counters.Updates += upd
	m.counters.Deletes += del
}

func (m *clientMon) incDialErrors() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.counters.DialErrors++
}

func (m *clientMon) incCheckCapsErrors() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.counters.CheckCapsErrors++
}

func (m *clientMon) incSubscribeErrors() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.counters.SubscribeErrors++
}

func (m *clientMon) incDisconnections() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.counters.Disconnections++
}

func (m *clientMon) incSrRoutingErrors() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.counters.SrRoutingErrors++
}

func (m *clientMon) srBufSize(size int) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	currValue := uint64(100 / srBufferSize * size)
	// Only record peak usage
	if currValue > m.gauges.NfBufUsagePC {
		m.gauges.NfBufUsagePC = currValue
	}
}
