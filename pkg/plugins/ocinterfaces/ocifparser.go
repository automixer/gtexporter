package ocinterfaces

import (
	"errors"
	"fmt"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/ygot"
	"regexp"
	"strconv"
	"strings"

	// Local packages
	"github.com/automixer/gtexporter/pkg/datamodels/ysocif"
	"github.com/automixer/gtexporter/pkg/plugins"
)

const yStructInitialSize = 128

// pathMetadata represents metadata extracted from a GNMI path.
// It includes information about the interface name, interface index,
// whether it is a subinterface, and the leaf name.
type pathMetadata struct {
	ifName   string
	ifIndex  uint32
	isSubInt bool
	leafName string
}

type ocIfParser struct {
	plugins.ParserMon
	yStruct        *ysocif.Root
	eMapper        *ysocif.EnumMapper
	rxSD           *regexp.Regexp // Description sanitize
	rxName         *regexp.Regexp // Interface name filter
	rxIndex        *regexp.Regexp // subInterface index filter
	disableDeletes bool
}

func newParser(cfg plugins.Config) (plugins.Parser, error) {
	p := &ocIfParser{}
	p.disableDeletes, _ = strconv.ParseBool(cfg.Options["disable_gnmi_delete"])

	// Load parser self-monitoring
	if err := p.ParserMon.Configure(cfg); err != nil {
		return nil, err
	}

	// Initialise the GoStruct and enum mapper
	p.yStruct = &ysocif.Root{
		Interface: make(map[string]*ysocif.Interface, yStructInitialSize),
	}
	p.eMapper = ysocif.NewEnumMapper()

	// Descriptions sanitization
	var err error
	p.rxSD, err = regexp.Compile(cfg.DescSanitize)
	if err != nil {
		return nil, err
	}

	// Interface name filter
	if cfg.Options["name_filter"] != "" {
		p.rxName, err = regexp.Compile(cfg.Options["name_filter"])
		if err != nil {
			return nil, err
		}
	} else {
		p.rxName = regexp.MustCompile(".*")
	}

	// SubInterface index filter
	if cfg.Options["index_filter"] != "" {
		p.rxIndex, err = regexp.Compile(cfg.Options["index_filter"])
		if err != nil {
			return nil, err
		}
	} else {
		p.rxIndex = regexp.MustCompile(".*")
	}

	return p, nil
}

// CheckOut returns the current yGot structure.
// It implements the plugin's parser interface
func (p *ocIfParser) CheckOut() ygot.GoStruct {
	if p.yStruct == nil {
		panic(fmt.Sprint("yGot structure not initialized"))
	}
	return p.yStruct
}

// ParseNotification implements the plugin's parser interface
// It is called by the plugin each time a GNMI notification is received.
func (p *ocIfParser) ParseNotification(nf *gnmi.Notification) {
	if p.yStruct == nil {
		panic(fmt.Sprint("yGot structure not initialized"))
	}

	// Process GNMI delete messages
	if !p.disableDeletes {
		for _, gDelete := range nf.Delete {
			p.removeDbEntry(nf.Prefix, gDelete)
		}
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

// ClearCache resets the cache of the ocIfParser by creating a new instance of the ocif.Root struct.
func (p *ocIfParser) ClearCache() {
	p.yStruct = &ysocif.Root{
		Interface: make(map[string]*ysocif.Interface, yStructInitialSize),
	}
}

// removeDbEntry processes the GNMI delete messages
func (p *ocIfParser) removeDbEntry(pfx, path *gnmi.Path) {
	pathMeta, err := p.getPathMeta(pfx, path)
	if err != nil {
		p.InvalidPath()
		return
	}
	if !pathMeta.isSubInt {
		// Delete interface
		if _, ok := p.yStruct.Interface[pathMeta.ifName]; ok {
			p.yStruct.DeleteInterface(pathMeta.ifName)
		} else {
			p.DeleteNotFound()
		}
	} else {
		// Delete subinterface
		if _, ok := p.yStruct.Interface[pathMeta.ifName]; ok {
			if p.yStruct.Interface[pathMeta.ifName].Subinterface != nil {
				if _, ok := p.yStruct.Interface[pathMeta.ifName].Subinterface[pathMeta.ifIndex]; ok {
					p.yStruct.Interface[pathMeta.ifName].DeleteSubinterface(pathMeta.ifIndex)
				} else {
					p.DeleteNotFound()
				}
			} else {
				p.DeleteNotFound()
			}
		} else {
			p.DeleteNotFound()
		}
	}
}

// updHandlerLookup scans the provided prefix and path to find the proper handler for a given GNMI notification.
// It returns that handler to the caller.
func (p *ocIfParser) updHandlerLookup(pfx, path *gnmi.Path) func(*gnmi.Notification, int) {
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
	case ifState:
		return p.ifState
	case ifState + "/counters":
		return p.ifStateCounters
	case subIfState:
		return p.subIfState
	case subIfState + "/counters":
		return p.subIfStateCounters
	case ifAggState:
		return p.ifAggState
	default:
		p.ContainerNotFound()
	}
	return nil
}

// getPathMeta returns the path metadata from the given prefix and path.
// It builds the full path as a slice of strings and then scans and extracts the metadata.
func (p *ocIfParser) getPathMeta(pfx, path *gnmi.Path) (*pathMetadata, error) {
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
		case strings.HasPrefix(elem, "subinterface[index=") && strings.Count(elem, "=") == 1:
			_, after, found := strings.Cut(elem, "=")
			if found {
				index, err := strconv.Atoi(after[:len(after)-1])
				if err != nil {
					return nil, err
				}
				out.isSubInt = true
				out.ifIndex = uint32(index)
			}
		}
	}
	out.leafName = fullPath[len(fullPath)-1]

	// Final check
	if out.ifName == "" || out.leafName == "" {
		return nil, errors.New("missing interface name or leaf name")
	}

	return out, nil
}

func (p *ocIfParser) sanitizeDescription(s string) string {
	matches := p.rxSD.FindAllString(s, -1)
	return strings.Join(matches, "")
}

// ifStateCounters parses the content of the /interface/state/counters YANG container
func (p *ocIfParser) ifStateCounters(nf *gnmi.Notification, updNum int) {
	pathMeta, err := p.getPathMeta(nf.Prefix, nf.Update[updNum].Path)
	if err != nil {
		p.InvalidPath()
		return
	}

	// Name filtering
	if !p.rxName.MatchString(pathMeta.ifName) {
		return
	}

	// Create the interface if missing
	if _, ok := p.yStruct.Interface[pathMeta.ifName]; !ok {
		newIf, err := p.yStruct.NewInterface(pathMeta.ifName)
		if err != nil {
			return
		}
		newIf.PopulateDefaults()
	}

	source := nf.Update[updNum].Val
	target := p.yStruct.Interface[pathMeta.ifName].Counters
	switch pathMeta.leafName {
	case "carrier-transitions":
		target.CarrierTransitions = ygot.Uint64(source.GetUintVal())
	case "in-broadcast-pkts":
		target.InBroadcastPkts = ygot.Uint64(source.GetUintVal())
	case "in-discards":
		target.InDiscards = ygot.Uint64(source.GetUintVal())
	case "in-errors":
		target.InErrors = ygot.Uint64(source.GetUintVal())
	case "in-fcs-errors":
		target.InFcsErrors = ygot.Uint64(source.GetUintVal())
	case "in-multicast-pkts":
		target.InMulticastPkts = ygot.Uint64(source.GetUintVal())
	case "in-octets":
		target.InOctets = ygot.Uint64(source.GetUintVal())
	case "in-pkts":
		target.InPkts = ygot.Uint64(source.GetUintVal())
	case "in-unicast-pkts":
		target.InUnicastPkts = ygot.Uint64(source.GetUintVal())
	case "in-unknown-protos":
		target.InUnknownProtos = ygot.Uint64(source.GetUintVal())
	case "last-clear":
		target.LastClear = ygot.Uint64(source.GetUintVal())
	case "out-broadcast-pkts":
		target.OutBroadcastPkts = ygot.Uint64(source.GetUintVal())
	case "out-discards":
		target.OutDiscards = ygot.Uint64(source.GetUintVal())
	case "out-errors":
		target.OutErrors = ygot.Uint64(source.GetUintVal())
	case "out-multicast-pkts":
		target.OutMulticastPkts = ygot.Uint64(source.GetUintVal())
	case "out-octets":
		target.OutOctets = ygot.Uint64(source.GetUintVal())
	case "out-pkts":
		target.OutPkts = ygot.Uint64(source.GetUintVal())
	case "out-unicast-pkts":
		target.OutUnicastPkts = ygot.Uint64(source.GetUintVal())
	case "resets":
		target.Resets = ygot.Uint64(source.GetUintVal())
	default:
		p.LeafNotFound()
	}
}

// ifState parses the content of the /interface/state YANG container
func (p *ocIfParser) ifState(nf *gnmi.Notification, updNum int) {
	pathMeta, err := p.getPathMeta(nf.Prefix, nf.Update[updNum].Path)
	if err != nil {
		p.InvalidPath()
		return
	}

	// Name filtering
	if !p.rxName.MatchString(pathMeta.ifName) {
		return
	}

	// Create the interface if missing
	if _, ok := p.yStruct.Interface[pathMeta.ifName]; !ok {
		newIf, err := p.yStruct.NewInterface(pathMeta.ifName)
		if err != nil {
			return
		}
		newIf.PopulateDefaults()
	}

	source := nf.Update[updNum].Val
	target := p.yStruct.Interface[pathMeta.ifName]
	switch pathMeta.leafName {
	case "admin-status":
		target.AdminStatus = ysocif.E_Interface_AdminStatus(
			p.eMapper.GetEnumFromString(source.GetStringVal(), target.AdminStatus))
	case "cpu":
		target.Cpu = ygot.Bool(source.GetBoolVal())
	case "description":
		target.Description = ygot.String(p.sanitizeDescription(source.GetStringVal()))
	case "enabled":
		target.Enabled = ygot.Bool(source.GetBoolVal())
	case "ifindex":
		target.Ifindex = ygot.Uint32(uint32(source.GetUintVal()))
	case "last-change":
		target.LastChange = ygot.Uint64(source.GetUintVal())
	case "logical":
		target.Logical = ygot.Bool(source.GetBoolVal())
	case "loopback-mode":
		target.LoopbackMode = ysocif.E_OpenconfigInterfaces_LoopbackModeType(
			p.eMapper.GetEnumFromString(source.GetStringVal(), target.LoopbackMode))
	case "management":
		target.Management = ygot.Bool(source.GetBoolVal())
	case "mtu":
		target.Mtu = ygot.Uint16(uint16(source.GetUintVal()))
	case "name":
		target.Name = ygot.String(source.GetStringVal())
	case "oper-status":
		target.OperStatus = ysocif.E_Interface_OperStatus(
			p.eMapper.GetEnumFromString(source.GetStringVal(), target.OperStatus))
	case "tpid":
		// tpid isn't handled but present to avoid false LeafNotFound() counting
	case "type":
		target.Type = ysocif.E_IETFInterfaces_InterfaceType(
			p.eMapper.GetEnumFromString(source.GetStringVal(), target.Type))
	default:
		p.LeafNotFound()
	}
}

// ifAggState parses the content of the /interface/aggregation/state YANG container
func (p *ocIfParser) ifAggState(nf *gnmi.Notification, updNum int) {
	pathMeta, err := p.getPathMeta(nf.Prefix, nf.Update[updNum].Path)
	if err != nil {
		p.InvalidPath()
		return
	}

	// Name filtering
	if !p.rxName.MatchString(pathMeta.ifName) {
		return
	}

	// Create the interface if missing
	if _, ok := p.yStruct.Interface[pathMeta.ifName]; !ok {
		newIf, err := p.yStruct.NewInterface(pathMeta.ifName)
		if err != nil {
			return
		}
		newIf.PopulateDefaults()
	}

	source := nf.Update[updNum].Val
	target := p.yStruct.Interface[pathMeta.ifName].Aggregation
	switch pathMeta.leafName {
	case "lag-speed":
		target.LagSpeed = ygot.Uint32(uint32(source.GetUintVal()))
	case "lag-type":
		target.LagType = ysocif.E_OpenconfigIfAggregate_AggregationType(
			p.eMapper.GetEnumFromString(source.GetStringVal(), target.LagType))
	case "member":
		memberList := source.GetLeaflistVal()
		for _, member := range memberList.Element {
			target.Member = append(target.Member, member.GetStringVal())
		}
	case "min-links":
		target.MinLinks = ygot.Uint16(uint16(source.GetUintVal()))
	default:
		p.LeafNotFound()
	}
}

// subIfStateCounters parses the content of the /interface/subinterfaces/subinterface/state/counters YANG container
func (p *ocIfParser) subIfStateCounters(nf *gnmi.Notification, updNum int) {
	pathMeta, err := p.getPathMeta(nf.Prefix, nf.Update[updNum].Path)
	if err != nil {
		p.InvalidPath()
		return
	}

	// Name and index filtering
	if !p.rxName.MatchString(pathMeta.ifName) || !p.rxIndex.MatchString(fmt.Sprint(pathMeta.ifIndex)) {
		return
	}

	// Create the interface if missing
	if _, ok := p.yStruct.Interface[pathMeta.ifName]; !ok {
		newIf, err := p.yStruct.NewInterface(pathMeta.ifName)
		if err != nil {
			return
		}
		newIf.PopulateDefaults()
	}

	// Create the subinterface if missing
	if _, ok := p.yStruct.Interface[pathMeta.ifName].Subinterface[pathMeta.ifIndex]; !ok {
		newSubIf, err := p.yStruct.Interface[pathMeta.ifName].NewSubinterface(pathMeta.ifIndex)
		if err != nil {
			return
		}
		newSubIf.PopulateDefaults()
	}

	source := nf.Update[updNum].Val
	target := p.yStruct.Interface[pathMeta.ifName].Subinterface[pathMeta.ifIndex].Counters
	switch pathMeta.leafName {
	case "carrier-transitions":
		target.CarrierTransitions = ygot.Uint64(source.GetUintVal())
	case "in-broadcast-pkts":
		target.InBroadcastPkts = ygot.Uint64(source.GetUintVal())
	case "in-discards":
		target.InDiscards = ygot.Uint64(source.GetUintVal())
	case "in-errors":
		target.InErrors = ygot.Uint64(source.GetUintVal())
	case "in-fcs-errors":
		target.InFcsErrors = ygot.Uint64(source.GetUintVal())
	case "in-multicast-pkts":
		target.InMulticastPkts = ygot.Uint64(source.GetUintVal())
	case "in-octets":
		target.InOctets = ygot.Uint64(source.GetUintVal())
	case "in-pkts":
		target.InPkts = ygot.Uint64(source.GetUintVal())
	case "in-unicast-pkts":
		target.InUnicastPkts = ygot.Uint64(source.GetUintVal())
	case "in-unknown-protos":
		target.InUnknownProtos = ygot.Uint64(source.GetUintVal())
	case "last-clear":
		target.LastClear = ygot.Uint64(source.GetUintVal())
	case "out-broadcast-pkts":
		target.OutBroadcastPkts = ygot.Uint64(source.GetUintVal())
	case "out-discards":
		target.OutDiscards = ygot.Uint64(source.GetUintVal())
	case "out-errors":
		target.OutErrors = ygot.Uint64(source.GetUintVal())
	case "out-multicast-pkts":
		target.OutMulticastPkts = ygot.Uint64(source.GetUintVal())
	case "out-octets":
		target.OutOctets = ygot.Uint64(source.GetUintVal())
	case "out-pkts":
		target.OutPkts = ygot.Uint64(source.GetUintVal())
	case "out-unicast-pkts":
		target.OutUnicastPkts = ygot.Uint64(source.GetUintVal())
	default:
		p.LeafNotFound()
	}
}

// subIfState parses the content of the /interface/subinterfaces/subinterface/state YANG container
func (p *ocIfParser) subIfState(nf *gnmi.Notification, updNum int) {
	pathMeta, err := p.getPathMeta(nf.Prefix, nf.Update[updNum].Path)
	if err != nil {
		p.InvalidPath()
		return
	}

	// Name and index filtering
	if !p.rxName.MatchString(pathMeta.ifName) || !p.rxIndex.MatchString(fmt.Sprint(pathMeta.ifIndex)) {
		return
	}

	// Create the interface if missing
	if _, ok := p.yStruct.Interface[pathMeta.ifName]; !ok {
		newIf, err := p.yStruct.NewInterface(pathMeta.ifName)
		if err != nil {
			return
		}
		newIf.PopulateDefaults()
	}

	// Create the subinterface if missing
	if _, ok := p.yStruct.Interface[pathMeta.ifName].Subinterface[pathMeta.ifIndex]; !ok {
		newSubIf, err := p.yStruct.Interface[pathMeta.ifName].NewSubinterface(pathMeta.ifIndex)
		if err != nil {
			return
		}
		newSubIf.PopulateDefaults()
	}

	source := nf.Update[updNum].Val
	target := p.yStruct.Interface[pathMeta.ifName].Subinterface[pathMeta.ifIndex]
	switch pathMeta.leafName {
	case "admin-status":
		target.AdminStatus = ysocif.E_Interface_AdminStatus(
			p.eMapper.GetEnumFromString(source.GetStringVal(), target.AdminStatus))
	case "cpu":
		target.Cpu = ygot.Bool(source.GetBoolVal())
	case "description":
		target.Description = ygot.String(p.sanitizeDescription(source.GetStringVal()))
	case "enabled":
		target.Enabled = ygot.Bool(source.GetBoolVal())
	case "ifindex":
		target.Ifindex = ygot.Uint32(uint32(source.GetUintVal()))
	case "index":
		target.Index = ygot.Uint32(uint32(source.GetUintVal()))
	case "last-change":
		target.LastChange = ygot.Uint64(source.GetUintVal())
	case "logical":
		target.Logical = ygot.Bool(source.GetBoolVal())
	case "management":
		target.Management = ygot.Bool(source.GetBoolVal())
	case "name":
		target.Name = ygot.String(source.GetStringVal())
	case "oper-status":
		target.OperStatus = ysocif.E_Interface_OperStatus(
			p.eMapper.GetEnumFromString(source.GetStringVal(), target.OperStatus))
	default:
		p.LeafNotFound()
	}
}
