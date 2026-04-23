package plugins

import (
	"errors"

	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/ygot/ygot"
)

// BuildPathElems builds a flat slice of gNMI path element strings from a prefix and path.
func BuildPathElems(pfx, path *gnmi.Path) ([]string, error) {
	var out []string
	if pfx != nil {
		elems, err := ygot.PathToStrings(pfx)
		if err != nil {
			return nil, err
		}
		out = append(out, elems...)
	}
	if path != nil {
		elems, err := ygot.PathToStrings(path)
		if err != nil {
			return nil, err
		}
		out = append(out, elems...)
	}
	if len(out) < 2 {
		return nil, errors.New("path too short")
	}
	return out, nil
}

// BuildSchemaPath joins the schema-form prefix and path into a single slash-delimited string.
func BuildSchemaPath(pfx, path *gnmi.Path) string {
	sPfx, _ := ygot.PathToSchemaPath(pfx)
	sPath, _ := ygot.PathToSchemaPath(path)
	if len(sPfx) > 1 {
		return sPfx + sPath
	}
	return sPath
}
