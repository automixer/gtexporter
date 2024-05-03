package plugins

import (
	"github.com/openconfig/gnmi/proto/gnmi"
	"sort"
	"time"
)

// Constants
const (
	bufInitialCap         = 2048
	scrapeDelayMultiplier = 2
)

// uBuffer represents a buffer for storing gNMI notifications.
type uBuffer struct {
	buf       []*gnmi.Notification
	scrapeInt time.Duration
	deadline  time.Time
	noScrape  bool
}

func newBuf(scrapeInt time.Duration) *uBuffer {
	buf := uBuffer{
		buf: make([]*gnmi.Notification, 0, bufInitialCap),
	}
	buf.scrapeInt = scrapeInt
	buf.deadline = time.Now().Add(scrapeInt * scrapeDelayMultiplier)
	return &buf
}

// add appends the given notification to the buffer if the uBuffer is not in noScrape state.
// If the buffer doesn't get regularly checked out within the deadline, add discards the provided notification.
func (b *uBuffer) add(nf *gnmi.Notification) {
	if b.noScrape {
		return
	}
	if time.Now().After(b.deadline) {
		b.noScrape = true
		b.clearBuffer()
		return
	}
	b.buf = append(b.buf, nf)
}

// checkout returns the buffered notifications.
func (b *uBuffer) checkout() []*gnmi.Notification {
	out := b.buf
	b.clearBuffer()
	// Sort updates by timestamp (ascending)
	sort.Slice(out, func(i, j int) bool { return out[i].Timestamp < out[j].Timestamp })
	b.noScrape = false
	b.deadline = time.Now().Add(b.scrapeInt * scrapeDelayMultiplier)
	return out
}

// clearBuffer empties the buffer by creating a new empty slice with the initial capacity.
func (b *uBuffer) clearBuffer() {
	b.buf = make([]*gnmi.Notification, 0, bufInitialCap)
}
