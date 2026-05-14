package oclldp

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/automixer/gtexporter/pkg/datamodels/ysoclldp"
	"github.com/automixer/gtexporter/pkg/plugins"
	"github.com/openconfig/gnmi/proto/gnmi"
)

// parseJsonUpdate decodes a JSON container-level update and routes it to the appropriate handler.
// This handles JSON/JSON_IETF encodings where each gNMI update carries a whole container as a
// JSON blob rather than individual leaf values.
func (p *ocLldpParser) parseJsonUpdate(nf *gnmi.Notification, updNum int, jsonBytes []byte) {
	fullPath := plugins.BuildSchemaPath(nf.GetPrefix(), nf.GetUpdate()[updNum].GetPath())

	pathMeta, err := p.getPathMeta(nf.GetPrefix(), nf.GetUpdate()[updNum].GetPath())
	if err != nil {
		p.InvalidPath()
		return
	}

	var data map[string]any
	dec := json.NewDecoder(bytes.NewReader(jsonBytes))
	dec.UseNumber()
	if err := dec.Decode(&data); err != nil {
		p.InvalidPath()
		return
	}

	switch fullPath {
	case lldpNbState:
		p.lldpIfNbStateJson(pathMeta, data)
	default:
		p.ContainerNotFound()
	}
}

// lldpIfNbStateJson fills /lldp/interfaces/interface/neighbors/neighbor/state GoStruct fields
// from a JSON map.
func (p *ocLldpParser) lldpIfNbStateJson(meta *pathMetadata, data map[string]any) {
	// Create the interface if missing
	if _, ok := p.yStruct.GetLldp().Interface[meta.ifName]; !ok {
		newIf, err := p.yStruct.GetLldp().NewInterface(meta.ifName)
		if err != nil {
			return
		}
		newIf.PopulateDefaults()
	}
	// Create the neighbor if missing
	if _, ok := p.yStruct.GetLldp().Interface[meta.ifName].Neighbor[meta.nbrId]; !ok {
		newNbr, err := p.yStruct.GetLldp().Interface[meta.ifName].NewNeighbor(meta.nbrId)
		if err != nil {
			return
		}
		newNbr.PopulateDefaults()
	}

	target := p.yStruct.GetLldp().Interface[meta.ifName].Neighbor[meta.nbrId]

	for key, val := range data {
		if strings.ContainsRune(key, ':') {
			continue
		}
		switch key {
		case "age":
			if u := plugins.JsonUint64(val); u != nil {
				target.Age = new(*u)
			}
		case "chassis-id":
			if s, ok := val.(string); ok {
				target.ChassisId = new(s)
			}
		case "chassis-id-type":
			if s, ok := val.(string); ok {
				target.ChassisIdType = ysoclldp.E_OpenconfigLldp_ChassisIdType(
					p.eMapper.GetEnumFromString(s, target.ChassisIdType))
			}
		case "id":
			if s, ok := val.(string); ok {
				target.Id = new(s)
			}
		case "last-update":
			if i := plugins.JsonInt64(val); i != nil {
				target.LastUpdate = new(*i)
			}
		case "management-address":
			if s, ok := val.(string); ok {
				target.ManagementAddress = new(s)
			}
		case "management-address-type":
			if s, ok := val.(string); ok {
				target.ManagementAddressType = new(s)
			}
		case "port-description":
			if s, ok := val.(string); ok {
				target.PortDescription = new(p.sanitizeDescription(s))
			}
		case "port-id":
			if s, ok := val.(string); ok {
				target.PortId = new(s)
			}
		case "port-id-type":
			if s, ok := val.(string); ok {
				target.PortIdType = ysoclldp.E_OpenconfigLldp_PortIdType(
					p.eMapper.GetEnumFromString(s, target.PortIdType))
			}
		case "system-description":
			if s, ok := val.(string); ok {
				target.SystemDescription = new(s)
			}
		case "system-name":
			if s, ok := val.(string); ok {
				target.SystemName = new(s)
			}
		case "ttl":
			if u := plugins.JsonUint64(val); u != nil {
				target.Ttl = new(uint16(*u))
			}
		}
	}
}
