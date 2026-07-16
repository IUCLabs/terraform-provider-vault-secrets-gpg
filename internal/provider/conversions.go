package provider

import "encoding/json"

// toInt64 coerces the numeric shapes Vault returns (json.Number, float64, int)
// into an int64.
func toInt64(v interface{}) (int64, bool) {
	switch n := v.(type) {
	case json.Number:
		if i, err := n.Int64(); err == nil {
			return i, true
		}
		if f, err := n.Float64(); err == nil {
			return int64(f), true
		}
		return 0, false
	case float64:
		return int64(n), true
	case int64:
		return n, true
	case int:
		return int64(n), true
	default:
		return 0, false
	}
}

func toBool(v interface{}) (bool, bool) {
	b, ok := v.(bool)
	return b, ok
}

func toString(v interface{}) (string, bool) {
	s, ok := v.(string)
	return s, ok
}
