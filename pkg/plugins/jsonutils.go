package plugins

import (
	"encoding/json"
	"strconv"
)

// JsonUint64 converts a JSON-decoded value to *uint64.
// With json.Decoder.UseNumber(), JSON integers decode as json.Number.
// Some devices also encode large uint64 values as quoted JSON strings to
// avoid JavaScript 64-bit precision loss.
func JsonUint64(v any) *uint64 {
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

// JsonInt64 converts a JSON-decoded value to *int64.
// Handles both json.Number and quoted string encodings.
func JsonInt64(v any) *int64 {
	var s string
	switch val := v.(type) {
	case json.Number:
		s = val.String()
	case string:
		s = val
	default:
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}
	return &n
}
