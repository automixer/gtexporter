package oclldp

import (
	"errors"
	"fmt"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/ygot"
	"regexp"
	"strings"

	// Local packages
	"github.com/automixer/gtexporter/pkg/datamodels/ysoclldp"
	"github.com/automixer/gtexporter/pkg/plugins"
)

const yStructInitialSize = 128

// pathMetadata represents metadata extracted from a path.
// It contains information about the interface name, neighbor ID, and leaf name.
type pathMetadata struct {
	ifName   string
	nbrId    string
	leafName string
}

// ocLldpParser represents a parser for OpenConfig LLDP (Link Layer Discovery Protocol) data.
// It implements the plugins.Parser interface and includes a ygot structure for storing LLDP data,
// an EnumMapper for mapping string enum values to their corresponding integer values,
// and a regular expression used for sanitizing description strings.
type ocLldpParser struct {
	plugins.ParserMon
	yStruct *ysoclldp.Root
	eMapper *ysoclldp.EnumMapper
	rxSD    *regexp.Regexp
}

// newParser creates a new ocLldpParser and initializes its fields based on the given configuration.
// It returns the newly created parser or an error if there was an issue during initialization.
func newParser(cfg plugins.Config) (plugins.Parser, error) {
	p := &ocLldpParser{}
	if err := p.ParserMon.Configure(cfg); err != nil {
		return nil, err
	}
	p.yStruct = &ysoclldp.Root{}
	p.yStruct.PopulateDefaults()
	p.yStruct.Lldp.Interface = make(map[string]*ysoclldp.Lldp_Interface, yStructInitialSize)
	p.eMapper = ysoclldp.NewEnumMapper()
	var err error
	p.rxSD, err = regexp.Compile(cfg.DescSanitize)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// CheckOut returns the yGot structure.
func (p *ocLldpParser) CheckOut() ygot.GoStruct {
	if p.yStruct == nil {
		panic(fmt.Sprint("yGot structure not initialized"))
	}
	return p.yStruct
}

// ClearCache resets the yGot structure, populates default values, and initializes the Lldp.Interface map.
func (p *ocLldpParser) ClearCache() {
	p.yStruct = &ysoclldp.Root{}
	p.yStruct.PopulateDefaults()
	p.yStruct.Lldp.Interface = make(map[string]*ysoclldp.Lldp_Interface, yStructInitialSize)
}

// sanitizeDescription removes all non-alphanumeric characters from the given string and returns the result.
func (p *ocLldpParser) sanitizeDescription(s string) string {
	matches := p.rxSD.FindAllString(s, -1)
	return strings.Join(matches, "")
}

// getPathMeta returns the metadata of the given path by parsing it and extracting the necessary information.
// The metadata includes the interface name, neighbor ID, and the name of the leaf node.
// If any of the metadata is missing or the path is invalid, an error is returned.
func (p *ocLldpParser) getPathMeta(pfx, path *gnmi.Path) (*pathMetadata, error) {
	var fullPath []string
	out := &pathMetadata{}

	// Build the full path as a slice of strings
	if pfx != nil {
		sPfx, err := ygot.PathToStrings(pfx)
		if err != nil {
			return nil, err
		}
		if len(sPfx) > 0 {
			fullPath = append(fullPath, sPfx...)
		}
	}
	if path != nil {
		sPath, err := ygot.PathToStrings(path)
		if err != nil {
			return nil, err
		}
		fullPath = append(fullPath, sPath...)
	}
	if len(fullPath) < 2 {
		return nil, errors.New("path too short")
	}

	// Scan fullPath and extract metadata
	for _, elem := range fullPath {
		switch {
		case strings.HasPrefix(elem, "interface[name=") && strings.Count(elem, "=") == 1:
			_, after, found := strings.Cut(elem, "=")
			if found {
				out.ifName = after[:len(after)-1]
			}
		case strings.HasPrefix(elem, "neighbor[id=") && strings.Count(elem, "=") == 1:
			_, after, found := strings.Cut(elem, "=")
			if found {
				out.nbrId = after[:len(after)-1]
			}
		}
	}
	out.leafName = fullPath[len(fullPath)-1]

	// Final check
	if out.ifName == "" || out.nbrId == "" || out.leafName == "" {
		return nil, errors.New("invalid path metadata")
	}
	return out, nil
}

// ParseNotification analyzes a GNMI notification and calls the appropriate decoding method.
func (p *ocLldpParser) ParseNotification(nf *gnmi.Notification) {
	if p.yStruct == nil {
		panic(fmt.Sprint("yGot structure not initialized"))
	}

	// Process GNMI delete messages
	for _, gDelete := range nf.Delete {
		p.removeDbEntry(nf.Prefix, gDelete)
	}

	// Process GNMI update messages
	for i, update := range nf.Update {
		updHandler := p.updHandlerLookup(nf.Prefix, update.Path)
		if updHandler == nil {
			continue
		}
		p.UpdateDuplicates(uint64(update.GetDuplicates()))
		updHandler(nf, i)
	}
}

// removeDbEntry removes the yGot GoStruct entry specified by the given prefix and path.
func (p *ocLldpParser) removeDbEntry(pfx, path *gnmi.Path) {
	pathMeta, err := p.getPathMeta(pfx, path)
	if err != nil {
		p.InvalidPath()
		return
	}

	if _, ok := p.yStruct.GetLldp().Interface[pathMeta.ifName]; ok {
		p.yStruct.GetLldp().Interface[pathMeta.ifName].DeleteNeighbor(pathMeta.nbrId)
	} else {
		p.DeleteNotFound()
	}

	if len(p.yStruct.GetLldp().Interface) == 0 {
		p.yStruct.GetLldp().DeleteInterface(pathMeta.ifName)
	}
}

// updHandlerLookup returns the appropriate decoding handler based on the given prefix and path.
func (p *ocLldpParser) updHandlerLookup(pfx, path *gnmi.Path) func(*gnmi.Notification, int) {
	sPfx, _ := ygot.PathToSchemaPath(pfx)
	sPath, _ := ygot.PathToSchemaPath(path)
	var fullPath string
	if len(sPfx) > 1 {
		fullPath += sPfx
	}
	fullPath += sPath
	leafIndex := strings.LastIndex(fullPath, "/")
	if leafIndex == -1 {
		p.InvalidPath()
		return nil
	}

	// Find the proper handler
	switch fullPath[:leafIndex] {
	case lldpNbState:
		return p.lldpIfNbState
	default:
		p.ContainerNotFound()
	}
	return nil
}

// lldpIfNbState updates the yGot structure with the information from the GNMI update message for the
// LLDP neighbor state.
func (p *ocLldpParser) lldpIfNbState(nf *gnmi.Notification, updNum int) {
	pathMeta, err := p.getPathMeta(nf.Prefix, nf.Update[updNum].Path)
	if err != nil {
		p.InvalidPath()
		return
	}
	// Create the interface if missing
	if p.yStruct != nil && p.yStruct.GetLldp() != nil {
		if _, ok := p.yStruct.GetLldp().Interface[pathMeta.ifName]; !ok {
			newIf, err := p.yStruct.GetLldp().NewInterface(pathMeta.ifName)
			if err != nil {
				return
			}
			newIf.PopulateDefaults()
		}
	} else {
		return
	}
	// Create the neighbor if missing
	if _, ok := p.yStruct.GetLldp().Interface[pathMeta.ifName].Neighbor[pathMeta.nbrId]; !ok {
		newNbr, err := p.yStruct.GetLldp().Interface[pathMeta.ifName].NewNeighbor(pathMeta.nbrId)
		if err != nil {
			return
		}
		newNbr.PopulateDefaults()
	}
	// Load the gnmi update into yGot struct
	source := nf.Update[updNum].Val
	target := p.yStruct.GetLldp().Interface[pathMeta.ifName].Neighbor[pathMeta.nbrId]
	switch pathMeta.leafName {
	case "age":
		target.Age = ygot.Uint64(source.GetUintVal())
	case "chassis-id":
		target.ChassisId = ygot.String(source.GetStringVal())
	case "chassis-id-type":
		target.ChassisIdType = ysoclldp.E_OpenconfigLldp_ChassisIdType(
			p.eMapper.GetEnumFromString(source.GetStringVal(), target.ChassisIdType))
	case "id":
		target.Id = ygot.String(source.GetStringVal())
	case "last-update":
		target.LastUpdate = ygot.Int64(source.GetIntVal())
	case "management-address":
		target.ManagementAddress = ygot.String(source.GetStringVal())
	case "management-address-type":
		target.ManagementAddressType = ygot.String(source.GetStringVal())
	case "port-description":
		target.PortDescription = ygot.String(p.sanitizeDescription(source.GetStringVal()))
	case "port-id":
		target.PortId = ygot.String(source.GetStringVal())
	case "port-id-type":
		target.PortIdType = ysoclldp.E_OpenconfigLldp_PortIdType(
			p.eMapper.GetEnumFromString(source.GetStringVal(), target.PortIdType))
	case "system-description":
		target.SystemDescription = ygot.String(source.GetStringVal())
	case "system-name":
		target.SystemName = ygot.String(source.GetStringVal())
	case "ttl":
		target.Ttl = ygot.Uint16(uint16(source.GetUintVal()))
	default:
		p.LeafNotFound()
	}
}
