package ysocif

import (
	"reflect"
	"strings"

	"github.com/openconfig/ygot/ygot"
)

// Generate OpenConfig Interfaces GoStruct code
//go:generate generator -output_file=gen.go -compress_paths=true -path=yang -exclude_modules=ietf-interfaces -package_name=ysocif -fakeroot_name=root -prefer_operational_state=true -ignore_shadow_schema_paths=true -shorten_enum_leaf_names=true -generate_fakeroot=true -include_schema=false -generate_getters=true -generate_leaf_getters=true -generate_delete=true -generate_populate_defaults=true openconfig-interfaces.yang openconfig-if-aggregate.yang openconfig-if-ethernet

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

// GoStructToOcIf converts a GoStruct interface to a pointer of a Root struct.
func GoStructToOcIf(ys ygot.GoStruct) *Root {
	if root, ok := ys.(*Root); ok {
		return root
	}
	panic("not an ygot interfaces GoStruct")
}

type CntMode int

const (
	Normal CntMode = iota
	UseGoDefault
	ForceToZero
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
		case ForceToZero:
			if strings.HasPrefix(fieldName, "in-") || strings.HasPrefix(fieldName, "out-") {
				// Wipe counters only
				out[fieldName] = 0.0
			} else {
				// If not a counter, apply Normal policy
				if valPtr != nil {
					out[fieldName] = float64(*valPtr)
				}
			}
		default:
			if valPtr != nil {
				out[fieldName] = float64(*valPtr)
			}
		}
	}
	return out
}

// ShortString returns a short string representation of the E_Interface_AdminStatus enum value.
func (e E_Interface_AdminStatus) ShortString() string {
	if e == Interface_AdminStatus_UNSET {
		return ""
	}
	return ygot.EnumLogString(e, int64(e), "E_Interface_AdminStatus")
}

// ShortString returns a short string representation of the E_Interface_OperStatus enum value.
func (e E_Interface_OperStatus) ShortString() string {
	if int64(e) == 0 {
		return ""
	}
	return ygot.EnumLogString(e, int64(e), "E_Interface_OperStatus")
}

// ShortString returns a short string representation of the E_OpenconfigIfAggregate_AggregationType enum value.
func (e E_OpenconfigIfAggregate_AggregationType) ShortString() string {
	if int64(e) == 0 {
		return ""
	}
	return ygot.EnumLogString(e, int64(e), "E_OpenconfigIfAggregate_AggregationType")
}

// ShortString returns the string representation of E_IETFInterfaces_InterfaceType.
func (e E_IETFInterfaces_InterfaceType) ShortString() string {
	if int64(e) == 0 {
		return ""
	}
	return ygot.EnumLogString(e, int64(e), "E_IETFInterfaces_InterfaceType")
}
