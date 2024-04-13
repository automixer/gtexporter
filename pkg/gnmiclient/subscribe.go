package gnmiclient

import (
	"context"
	log "github.com/golang/glog"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/ygot"
)

// subscribe creates a subscription client and sends SubscribeRequests to the server.
// It returns the subscription client and any error encountered during the process.
func (c *GnmiClient) subscribe(ctx context.Context, stub gnmi.GNMIClient) (gnmi.GNMI_SubscribeClient, error) {
	// Create client
	gNMISubClt, err := stub.Subscribe(ctx)
	if err != nil {
		return nil, err
	}
	if c.config.OverSampling == 0 {
		c.config.OverSampling = oversampling
	}
	if c.config.OverSampling < 1 || c.config.OverSampling > 10 {
		log.Warningf("%s: Oversampling must fall between 1 and 10", c.config.DevName)
		c.config.OverSampling = oversampling
	}

	// Prepare the subscription list
	subLists := c.newSubList()

	// Subscribe
	for _, sl := range subLists {
		// Prepare the SubscribeRequest struct
		req := &gnmi.SubscribeRequest{
			Request:   &gnmi.SubscribeRequest_Subscribe{Subscribe: sl},
			Extension: nil,
		}
		// Send it to the device
		err = gNMISubClt.Send(req)
		if err != nil {
			return nil, err
		}
	}

	return gNMISubClt, nil
}

// newSubList creates a list with a single subscriptions for all the configured plugins.
// This is the default way for subscribing telemetries.
func (c *GnmiClient) newSubList() []*gnmi.SubscriptionList {
	var subs []*gnmi.Subscription
	var subLists []*gnmi.SubscriptionList

	for _, plug := range c.plugins {
		for _, path := range c.xPathList[plug.GetPlugName()] {
			// Huawei requires prepending the datamodel name to paths
			if c.config.Vendor == "huawei" {
				path = plug.GetDataModel() + ":" + path[1:]
			}

			// One subscription for each plugin's path
			p, err := ygot.StringToPath(path, ygot.StructuredPath, ygot.StringSlicePath)
			if err != nil {
				log.Error(err)
				continue
			}
			newSub := &gnmi.Subscription{
				Path:              p,
				Mode:              c.config.GnmiSubscriptionMode,
				SampleInterval:    uint64(c.config.ScrapeInterval.Nanoseconds() / c.config.OverSampling),
				SuppressRedundant: false,
				HeartbeatInterval: 0,
			}
			subs = append(subs, newSub)
		}
	}

	// One subscription list per device
	subLists = append(subLists, &gnmi.SubscriptionList{
		Prefix:           nil,
		Subscription:     subs,
		Qos:              nil,
		Mode:             gnmi.SubscriptionList_STREAM,
		AllowAggregation: false,
		UseModels:        nil,
		Encoding:         c.encoding,
		UpdatesOnly:      c.config.GnmiUpdatesOnly,
	})

	return subLists
}
