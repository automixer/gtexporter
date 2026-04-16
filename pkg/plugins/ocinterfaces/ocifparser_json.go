package ocinterfaces

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/automixer/gtexporter/pkg/datamodels/ysocif"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/ygot"
)

// parseJsonUpdate decodes a JSON container-level update and routes it to the appropriate handler.
// This handles JSON/JSON_IETF encodings used by devices like Cisco IOS XE, where each gNMI
// update carries a whole container as a JSON blob rather than individual leaf values.
func (p *ocIfParser) parseJsonUpdate(nf *gnmi.Notification, updNum int, jsonBytes []byte) {
	// Compute schema path for container-level routing
	sPfx, _ := ygot.PathToSchemaPath(nf.Prefix)
	sPath, _ := ygot.PathToSchemaPath(nf.Update[updNum].Path)
	var fullPath string
	if len(sPfx) > 1 {
		fullPath += sPfx
	}
	fullPath += sPath

	// Extract interface/subinterface metadata from path keys
	pathMeta, err := p.getPathMeta(nf.Prefix, nf.Update[updNum].Path)
	if err != nil {
		p.InvalidPath()
		return
	}

	// Decode the JSON blob; UseNumber preserves large integers as strings
	var data map[string]interface{}
	dec := json.NewDecoder(bytes.NewReader(jsonBytes))
	dec.UseNumber()
	if err := dec.Decode(&data); err != nil {
		p.InvalidPath()
		return
	}

	switch fullPath {
	case ifState:
		p.ifStateJson(pathMeta, data)
	case subIfState:
		p.subIfStateJson(pathMeta, data)
	case ifAggState:
		p.ifAggStateJson(pathMeta, data)
	default:
		p.ContainerNotFound()
	}
}

// ifStateJson fills /interface/state GoStruct fields from a JSON map.
// Cisco IOS XE sends the state fields and the nested counters together in one update.
func (p *ocIfParser) ifStateJson(meta *pathMetadata, data map[string]interface{}) {
	if !p.rxName.MatchString(meta.ifName) {
		return
	}

	if _, ok := p.yStruct.Interface[meta.ifName]; !ok {
		newIf, err := p.yStruct.NewInterface(meta.ifName)
		if err != nil {
			return
		}
		newIf.PopulateDefaults()
	}
	target := p.yStruct.Interface[meta.ifName]

	for key, val := range data {
		// Keys containing a colon are YANG namespace-qualified extension fields
		// (e.g. "openconfig-platform-port:hardware-port") — skip them silently.
		if strings.ContainsRune(key, ':') {
			continue
		}
		switch key {
		case "admin-status":
			if s, ok := val.(string); ok {
				target.AdminStatus = ysocif.E_Interface_AdminStatus(
					p.eMapper.GetEnumFromString(s, target.AdminStatus))
			}
		case "description":
			if s, ok := val.(string); ok {
				target.Description = ygot.String(p.sanitizeDescription(s))
			}
		case "enabled":
			if b, ok := val.(bool); ok {
				target.Enabled = ygot.Bool(b)
			}
		case "ifindex":
			if u := jsonUint64(val); u != nil {
				target.Ifindex = ygot.Uint32(uint32(*u))
			}
		case "last-change":
			if u := jsonUint64(val); u != nil {
				target.LastChange = ygot.Uint64(*u)
			}
		case "name":
			if s, ok := val.(string); ok {
				target.Name = ygot.String(s)
			}
		case "oper-status":
			if s, ok := val.(string); ok {
				target.OperStatus = ysocif.E_Interface_OperStatus(
					p.eMapper.GetEnumFromString(s, target.OperStatus))
			}
		case "type":
			if s, ok := val.(string); ok {
				target.Type = ysocif.E_IETFInterfaces_InterfaceType(
					p.eMapper.GetEnumFromString(s, target.Type))
			}
		case "counters":
			if cMap, ok := val.(map[string]interface{}); ok {
				p.fillIfCountersJson(target.Counters, cMap)
			}
		}
	}
}

// fillIfCountersJson fills /interface/state/counters GoStruct fields from a JSON map.
func (p *ocIfParser) fillIfCountersJson(target *ysocif.Interface_Counters, data map[string]interface{}) {
	for key, val := range data {
		u := jsonUint64(val)
		if u == nil {
			continue
		}
		switch key {
		case "carrier-transitions":
			target.CarrierTransitions = ygot.Uint64(*u)
		case "in-broadcast-pkts":
			target.InBroadcastPkts = ygot.Uint64(*u)
		case "in-discards":
			target.InDiscards = ygot.Uint64(*u)
		case "in-errors":
			target.InErrors = ygot.Uint64(*u)
		case "in-fcs-errors":
			target.InFcsErrors = ygot.Uint64(*u)
		case "in-multicast-pkts":
			target.InMulticastPkts = ygot.Uint64(*u)
		case "in-octets":
			target.InOctets = ygot.Uint64(*u)
		case "in-pkts":
			target.InPkts = ygot.Uint64(*u)
		case "in-unicast-pkts":
			target.InUnicastPkts = ygot.Uint64(*u)
		case "in-unknown-protos":
			target.InUnknownProtos = ygot.Uint64(*u)
		case "last-clear":
			target.LastClear = ygot.Uint64(*u)
		case "out-broadcast-pkts":
			target.OutBroadcastPkts = ygot.Uint64(*u)
		case "out-discards":
			target.OutDiscards = ygot.Uint64(*u)
		case "out-errors":
			target.OutErrors = ygot.Uint64(*u)
		case "out-multicast-pkts":
			target.OutMulticastPkts = ygot.Uint64(*u)
		case "out-octets":
			target.OutOctets = ygot.Uint64(*u)
		case "out-pkts":
			target.OutPkts = ygot.Uint64(*u)
		case "out-unicast-pkts":
			target.OutUnicastPkts = ygot.Uint64(*u)
		case "resets":
			target.Resets = ygot.Uint64(*u)
		}
	}
}

// subIfStateJson fills /interface/subinterfaces/subinterface/state GoStruct fields from a JSON map.
func (p *ocIfParser) subIfStateJson(meta *pathMetadata, data map[string]interface{}) {
	if !p.rxName.MatchString(meta.ifName) || !p.rxIndex.MatchString(fmt.Sprint(meta.ifIndex)) {
		return
	}

	if _, ok := p.yStruct.Interface[meta.ifName]; !ok {
		newIf, err := p.yStruct.NewInterface(meta.ifName)
		if err != nil {
			return
		}
		newIf.PopulateDefaults()
	}

	if _, ok := p.yStruct.Interface[meta.ifName].Subinterface[meta.ifIndex]; !ok {
		newSubIf, err := p.yStruct.Interface[meta.ifName].NewSubinterface(meta.ifIndex)
		if err != nil {
			return
		}
		newSubIf.PopulateDefaults()
	}
	target := p.yStruct.Interface[meta.ifName].Subinterface[meta.ifIndex]

	for key, val := range data {
		if strings.ContainsRune(key, ':') {
			continue
		}
		switch key {
		case "admin-status":
			if s, ok := val.(string); ok {
				target.AdminStatus = ysocif.E_Interface_AdminStatus(
					p.eMapper.GetEnumFromString(s, target.AdminStatus))
			}
		case "description":
			if s, ok := val.(string); ok {
				target.Description = ygot.String(p.sanitizeDescription(s))
			}
		case "enabled":
			if b, ok := val.(bool); ok {
				target.Enabled = ygot.Bool(b)
			}
		case "ifindex":
			if u := jsonUint64(val); u != nil {
				target.Ifindex = ygot.Uint32(uint32(*u))
			}
		case "index":
			if u := jsonUint64(val); u != nil {
				target.Index = ygot.Uint32(uint32(*u))
			}
		case "last-change":
			if u := jsonUint64(val); u != nil {
				target.LastChange = ygot.Uint64(*u)
			}
		case "name":
			if s, ok := val.(string); ok {
				target.Name = ygot.String(s)
			}
		case "oper-status":
			if s, ok := val.(string); ok {
				target.OperStatus = ysocif.E_Interface_OperStatus(
					p.eMapper.GetEnumFromString(s, target.OperStatus))
			}
		case "counters":
			if cMap, ok := val.(map[string]interface{}); ok {
				p.fillSubIfCountersJson(target.Counters, cMap)
			}
		}
	}
}

// fillSubIfCountersJson fills /interface/subinterfaces/subinterface/state/counters fields from a JSON map.
func (p *ocIfParser) fillSubIfCountersJson(target *ysocif.Interface_Subinterface_Counters, data map[string]interface{}) {
	for key, val := range data {
		u := jsonUint64(val)
		if u == nil {
			continue
		}
		switch key {
		case "carrier-transitions":
			target.CarrierTransitions = ygot.Uint64(*u)
		case "in-broadcast-pkts":
			target.InBroadcastPkts = ygot.Uint64(*u)
		case "in-discards":
			target.InDiscards = ygot.Uint64(*u)
		case "in-errors":
			target.InErrors = ygot.Uint64(*u)
		case "in-fcs-errors":
			target.InFcsErrors = ygot.Uint64(*u)
		case "in-multicast-pkts":
			target.InMulticastPkts = ygot.Uint64(*u)
		case "in-octets":
			target.InOctets = ygot.Uint64(*u)
		case "in-pkts":
			target.InPkts = ygot.Uint64(*u)
		case "in-unicast-pkts":
			target.InUnicastPkts = ygot.Uint64(*u)
		case "in-unknown-protos":
			target.InUnknownProtos = ygot.Uint64(*u)
		case "last-clear":
			target.LastClear = ygot.Uint64(*u)
		case "out-broadcast-pkts":
			target.OutBroadcastPkts = ygot.Uint64(*u)
		case "out-discards":
			target.OutDiscards = ygot.Uint64(*u)
		case "out-errors":
			target.OutErrors = ygot.Uint64(*u)
		case "out-multicast-pkts":
			target.OutMulticastPkts = ygot.Uint64(*u)
		case "out-octets":
			target.OutOctets = ygot.Uint64(*u)
		case "out-pkts":
			target.OutPkts = ygot.Uint64(*u)
		case "out-unicast-pkts":
			target.OutUnicastPkts = ygot.Uint64(*u)
		}
	}
}

// ifAggStateJson fills /interface/aggregation/state GoStruct fields from a JSON map.
func (p *ocIfParser) ifAggStateJson(meta *pathMetadata, data map[string]interface{}) {
	if !p.rxName.MatchString(meta.ifName) {
		return
	}

	if _, ok := p.yStruct.Interface[meta.ifName]; !ok {
		newIf, err := p.yStruct.NewInterface(meta.ifName)
		if err != nil {
			return
		}
		newIf.PopulateDefaults()
	}
	target := p.yStruct.Interface[meta.ifName].Aggregation

	for key, val := range data {
		switch key {
		case "lag-speed":
			if u := jsonUint64(val); u != nil {
				target.LagSpeed = ygot.Uint32(uint32(*u))
			}
		case "lag-type":
			if s, ok := val.(string); ok {
				target.LagType = ysocif.E_OpenconfigIfAggregate_AggregationType(
					p.eMapper.GetEnumFromString(s, target.LagType))
			}
		case "member":
			// Member may be a single string or a JSON array of strings (multi-member LAG)
			switch v := val.(type) {
			case string:
				target.Member = append(target.Member, v)
			case []interface{}:
				for _, m := range v {
					if s, ok := m.(string); ok {
						target.Member = append(target.Member, s)
					}
				}
			}
		case "min-links":
			if u := jsonUint64(val); u != nil {
				target.MinLinks = ygot.Uint16(uint16(*u))
			}
		}
	}
}

// jsonUint64 converts a JSON-decoded value to *uint64.
// With json.Decoder.UseNumber(), JSON integers decode as json.Number.
// Cisco IOS XE also encodes large uint64 values as quoted JSON strings to
// avoid JavaScript 64-bit precision loss.
func jsonUint64(v interface{}) *uint64 {
	var s string
	switch val := v.(type) {
	case json.Number:
		s = val.String()
	case string:
		s = val
	default:
		return nil
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return nil
	}
	return &n
}
