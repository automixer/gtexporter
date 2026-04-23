package ocinterfaces

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/automixer/gtexporter/pkg/datamodels/ysocif"
	"github.com/automixer/gtexporter/pkg/plugins"
	"github.com/openconfig/gnmi/proto/gnmi"
)

// parseJsonUpdate decodes a JSON container-level update and routes it to the appropriate handler.
// This handles JSON/JSON_IETF encodings used by devices like Cisco IOS XE, where each gNMI
// update carries a whole container as a JSON blob rather than individual leaf values.
func (p *ocIfParser) parseJsonUpdate(nf *gnmi.Notification, updNum int, jsonBytes []byte) {
	// Compute schema path for container-level routing
	fullPath := plugins.BuildSchemaPath(nf.GetPrefix(), nf.GetUpdate()[updNum].GetPath())

	// Extract interface/subinterface metadata from path keys
	pathMeta, err := p.getPathMeta(nf.GetPrefix(), nf.GetUpdate()[updNum].GetPath())
	if err != nil {
		p.InvalidPath()
		return
	}

	// Decode the JSON blob; UseNumber preserves large integers as strings
	var data map[string]any
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
func (p *ocIfParser) ifStateJson(meta *pathMetadata, data map[string]any) {
	if !p.rxName.MatchString(meta.ifName) {
		return
	}

	if !p.ensureInterface(meta.ifName) {
		return
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
		case "cpu":
			if b, ok := val.(bool); ok {
				target.Cpu = new(b)
			}
		case "description":
			if s, ok := val.(string); ok {
				target.Description = new(p.sanitizeDescription(s))
			}
		case "enabled":
			if b, ok := val.(bool); ok {
				target.Enabled = new(b)
			}
		case "ifindex":
			if u := jsonUint64(val); u != nil {
				target.Ifindex = new(uint32(*u))
			}
		case "last-change":
			if u := jsonUint64(val); u != nil {
				target.LastChange = new(*u)
			}
		case "logical":
			if b, ok := val.(bool); ok {
				target.Logical = new(b)
			}
		case "loopback-mode":
			if s, ok := val.(string); ok {
				target.LoopbackMode = ysocif.E_OpenconfigInterfaces_LoopbackModeType(
					p.eMapper.GetEnumFromString(s, target.LoopbackMode))
			}
		case "management":
			if b, ok := val.(bool); ok {
				target.Management = new(b)
			}
		case "mtu":
			if u := jsonUint64(val); u != nil {
				target.Mtu = new(uint16(*u))
			}
		case "name":
			if s, ok := val.(string); ok {
				target.Name = new(s)
			}
		case "oper-status":
			if s, ok := val.(string); ok {
				target.OperStatus = ysocif.E_Interface_OperStatus(
					p.eMapper.GetEnumFromString(s, target.OperStatus))
			}
		case "tpid":
			// tpid isn't handled but present to avoid false ContainerNotFound() counting
		case "type":
			if s, ok := val.(string); ok {
				target.Type = ysocif.E_IETFInterfaces_InterfaceType(
					p.eMapper.GetEnumFromString(s, target.Type))
			}
		case "counters":
			if cMap, ok := val.(map[string]any); ok {
				p.fillIfCountersJson(target.Counters, cMap)
			}
		}
	}
}

// fillIfCountersJson fills /interface/state/counters GoStruct fields from a JSON map.
func (p *ocIfParser) fillIfCountersJson(target *ysocif.Interface_Counters, data map[string]any) {
	for key, val := range data {
		u := jsonUint64(val)
		if u == nil {
			continue
		}
		switch key {
		case "carrier-transitions":
			target.CarrierTransitions = new(*u)
		case "in-broadcast-pkts":
			target.InBroadcastPkts = new(*u)
		case "in-discards":
			target.InDiscards = new(*u)
		case "in-errors":
			target.InErrors = new(*u)
		case "in-fcs-errors":
			target.InFcsErrors = new(*u)
		case "in-multicast-pkts":
			target.InMulticastPkts = new(*u)
		case "in-octets":
			target.InOctets = new(*u)
		case "in-pkts":
			target.InPkts = new(*u)
		case "in-unicast-pkts":
			target.InUnicastPkts = new(*u)
		case "in-unknown-protos":
			target.InUnknownProtos = new(*u)
		case "last-clear":
			target.LastClear = new(*u)
		case "out-broadcast-pkts":
			target.OutBroadcastPkts = new(*u)
		case "out-discards":
			target.OutDiscards = new(*u)
		case "out-errors":
			target.OutErrors = new(*u)
		case "out-multicast-pkts":
			target.OutMulticastPkts = new(*u)
		case "out-octets":
			target.OutOctets = new(*u)
		case "out-pkts":
			target.OutPkts = new(*u)
		case "out-unicast-pkts":
			target.OutUnicastPkts = new(*u)
		case "resets":
			target.Resets = new(*u)
		}
	}
}

// subIfStateJson fills /interface/subinterfaces/subinterface/state GoStruct fields from a JSON map.
func (p *ocIfParser) subIfStateJson(meta *pathMetadata, data map[string]any) {
	if !p.rxName.MatchString(meta.ifName) || !p.rxIndex.MatchString(fmt.Sprint(meta.ifIndex)) {
		return
	}

	if !p.ensureInterface(meta.ifName) || !p.ensureSubinterface(meta.ifName, meta.ifIndex) {
		return
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
		case "cpu":
			if b, ok := val.(bool); ok {
				target.Cpu = new(b)
			}
		case "description":
			if s, ok := val.(string); ok {
				target.Description = new(p.sanitizeDescription(s))
			}
		case "enabled":
			if b, ok := val.(bool); ok {
				target.Enabled = new(b)
			}
		case "ifindex":
			if u := jsonUint64(val); u != nil {
				target.Ifindex = new(uint32(*u))
			}
		case "index":
			if u := jsonUint64(val); u != nil {
				target.Index = new(uint32(*u))
			}
		case "last-change":
			if u := jsonUint64(val); u != nil {
				target.LastChange = new(*u)
			}
		case "logical":
			if b, ok := val.(bool); ok {
				target.Logical = new(b)
			}
		case "management":
			if b, ok := val.(bool); ok {
				target.Management = new(b)
			}
		case "name":
			if s, ok := val.(string); ok {
				target.Name = new(s)
			}
		case "oper-status":
			if s, ok := val.(string); ok {
				target.OperStatus = ysocif.E_Interface_OperStatus(
					p.eMapper.GetEnumFromString(s, target.OperStatus))
			}
		case "counters":
			if cMap, ok := val.(map[string]any); ok {
				p.fillSubIfCountersJson(target.Counters, cMap)
			}
		}
	}
}

// fillSubIfCountersJson fills /interface/subinterfaces/subinterface/state/counters fields from a JSON map.
func (p *ocIfParser) fillSubIfCountersJson(target *ysocif.Interface_Subinterface_Counters, data map[string]any) {
	for key, val := range data {
		u := jsonUint64(val)
		if u == nil {
			continue
		}
		switch key {
		case "carrier-transitions":
			target.CarrierTransitions = new(*u)
		case "in-broadcast-pkts":
			target.InBroadcastPkts = new(*u)
		case "in-discards":
			target.InDiscards = new(*u)
		case "in-errors":
			target.InErrors = new(*u)
		case "in-fcs-errors":
			target.InFcsErrors = new(*u)
		case "in-multicast-pkts":
			target.InMulticastPkts = new(*u)
		case "in-octets":
			target.InOctets = new(*u)
		case "in-pkts":
			target.InPkts = new(*u)
		case "in-unicast-pkts":
			target.InUnicastPkts = new(*u)
		case "in-unknown-protos":
			target.InUnknownProtos = new(*u)
		case "last-clear":
			target.LastClear = new(*u)
		case "out-broadcast-pkts":
			target.OutBroadcastPkts = new(*u)
		case "out-discards":
			target.OutDiscards = new(*u)
		case "out-errors":
			target.OutErrors = new(*u)
		case "out-multicast-pkts":
			target.OutMulticastPkts = new(*u)
		case "out-octets":
			target.OutOctets = new(*u)
		case "out-pkts":
			target.OutPkts = new(*u)
		case "out-unicast-pkts":
			target.OutUnicastPkts = new(*u)
		}
	}
}

// ifAggStateJson fills /interface/aggregation/state GoStruct fields from a JSON map.
func (p *ocIfParser) ifAggStateJson(meta *pathMetadata, data map[string]any) {
	if !p.rxName.MatchString(meta.ifName) {
		return
	}

	if !p.ensureInterface(meta.ifName) {
		return
	}
	target := p.yStruct.Interface[meta.ifName].Aggregation

	for key, val := range data {
		switch key {
		case "lag-speed":
			if u := jsonUint64(val); u != nil {
				target.LagSpeed = new(uint32(*u))
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
			case []any:
				for _, m := range v {
					if s, ok := m.(string); ok {
						target.Member = append(target.Member, s)
					}
				}
			}
		case "min-links":
			if u := jsonUint64(val); u != nil {
				target.MinLinks = new(uint16(*u))
			}
		}
	}
}

// jsonUint64 converts a JSON-decoded value to *uint64.
// With json.Decoder.UseNumber(), JSON integers decode as json.Number.
// Cisco IOS XE also encodes large uint64 values as quoted JSON strings to
// avoid JavaScript 64-bit precision loss.
func jsonUint64(v any) *uint64 {
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
