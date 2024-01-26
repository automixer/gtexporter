package dmoclldp

import (
	"reflect"
	"strings"

	"github.com/openconfig/ygot/ygot"
)

// Generate OpenConfig lldp GoStruct code
//go:generate generator -output_file=gen.go -compress_paths=true -path=yang -exclude_modules=ietf-interfaces,openconfig-interfaces -package_name=dmoclldp -fakeroot_name=root -prefer_operational_state=true -ignore_shadow_schema_paths=true -shorten_enum_leaf_names=true -generate_fakeroot=true -include_schema=false -generate_getters=true -generate_leaf_getters=true -generate_delete=true -generate_populate_defaults=true openconfig-lldp.yang

// EnumMapper is a struct that maps enum names and their values.
type EnumMapper struct {
	eMap map[string]map[string]int64 // Outer key: Enum name - Inner key: enum element name - Value: enum element value
}

func NewEnumMapper() *EnumMapper {
	em := &EnumMapper{make(map[string]map[string]int64)}

	for enumType, enum := range ΛEnum {
		em.eMap[enumType] = make(map[string]int64)
		for enumValue, enumName := range enum {
			em.eMap[enumType][enumName.Name] = enumValue
		}
	}
	return em
}

// GetEnumFromString retrieves the enum value corresponding to the given string representation.
// If the string representation does not match any enum value, it returns 0.
func (m EnumMapper) GetEnumFromString(s string, e ygot.GoEnum) int64 {
	// Sometimes enum names are prefixed with yang source
	_, after, found := strings.Cut(s, ":")
	if found {
		s = after
	}

	// Get the enum name
	rType := reflect.TypeOf(e).Name()
	if _, ok := m.eMap[rType]; !ok {
		// 0 means unset (ygot)
		return 0
	}
	if _, ok := m.eMap[rType][s]; !ok {
		return 0
	}
	return m.eMap[rType][s]
}

// GoStructToOcLldp converts a GoStruct interface to a pointer of a Root struct.
func GoStructToOcLldp(ys ygot.GoStruct) *Root {
	if root, ok := ys.(*Root); ok {
		return root
	}
	panic("not an ygot interfaces GoStruct")
}

type CntMode int

const (
	Normal CntMode = iota
	UseGoDefault
)

// GetCountersFromStruct extract a map of counters from a yang container of counters
func GetCountersFromStruct(s any, mode CntMode) map[string]float64 {
	if s == nil {
		return nil
	}
	sType := reflect.TypeOf(s)
	sValue := reflect.ValueOf(s)
	numField := sType.NumField()
	out := make(map[string]float64, numField) // Map key: Counter name - Map value: Counter value
	for i := 0; i < numField; i++ {
		fieldName := sType.Field(i).Tag.Get("path")
		if fieldName == "last-clear" {
			// last clear is not a counter
			continue
		}
		valPtr := sValue.Field(i).Interface().(*uint64)
		switch mode {
		case UseGoDefault:
			if valPtr != nil {
				out[fieldName] = float64(*valPtr)
			} else {
				out[fieldName] = 0.0
			}
		default:
			if valPtr != nil {
				out[fieldName] = float64(*valPtr)
			}
		}
	}
	return out
}

// ShortString returns a short string representation of the E_OpenconfigLldpTypes_LLDP_SYSTEM_CAPABILITY enum value.
func (e E_OpenconfigLldpTypes_LLDP_SYSTEM_CAPABILITY) ShortString() string {
	if e == OpenconfigLldpTypes_LLDP_SYSTEM_CAPABILITY_UNSET {
		return ""
	}
	return ygot.EnumLogString(e, int64(e), "E_OpenconfigLldpTypes_LLDP_SYSTEM_CAPABILITY")
}

// ShortString returns a short string representation of the E_OpenconfigLldpTypes_LLDP_TLV enum value.
func (e E_OpenconfigLldpTypes_LLDP_TLV) ShortString() string {
	if e == OpenconfigLldpTypes_LLDP_TLV_UNSET {
		return ""
	}
	return ygot.EnumLogString(e, int64(e), "E_OpenconfigLldpTypes_LLDP_TLV")
}

// ShortString returns a short string representation of the E_OpenconfigLldp_ChassisIdType enum value.
func (e E_OpenconfigLldp_ChassisIdType) ShortString() string {
	if e == OpenconfigLldp_ChassisIdType_UNSET {
		return ""
	}
	return ygot.EnumLogString(e, int64(e), "E_OpenconfigLldp_ChassisIdType")
}

// ShortString returns a short string representation of the E_OpenconfigLldp_PortIdType enum value.
func (e E_OpenconfigLldp_PortIdType) ShortString() string {
	if e == OpenconfigLldp_PortIdType_UNSET {
		return ""
	}
	return ygot.EnumLogString(e, int64(e), "E_OpenconfigLldp_PortIdType")
}
