package provider

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// jsonSubsetEqual reports whether every key present in want (recursively)
// equals the corresponding value in got. Extra keys in got are ignored, which
// is how server-normalized responses (filled defaults, added type
// discriminators) are compared against user configuration. Arrays must match
// element-wise in length and order.
func jsonSubsetEqual(want, got []byte) (bool, error) {
	var w, g any
	if err := json.Unmarshal(want, &w); err != nil {
		return false, fmt.Errorf("parsing configured JSON: %w", err)
	}
	if err := json.Unmarshal(got, &g); err != nil {
		return false, fmt.Errorf("parsing API JSON: %w", err)
	}
	return subsetEqual(w, g), nil
}

func subsetEqual(want, got any) bool {
	switch w := want.(type) {
	case map[string]any:
		g, ok := got.(map[string]any)
		if !ok {
			return false
		}
		for k, wv := range w {
			gv, ok := g[k]
			if !ok {
				return false
			}
			if !subsetEqual(wv, gv) {
				return false
			}
		}
		return true
	case []any:
		g, ok := got.([]any)
		if !ok || len(g) != len(w) {
			return false
		}
		for i := range w {
			if !subsetEqual(w[i], g[i]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(normalizeNumber(want), normalizeNumber(got))
	}
}

func normalizeNumber(v any) any {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	}
	return v
}

// canonicalJSON re-encodes JSON with sorted keys and no whitespace so state
// comparisons are byte-stable.
func canonicalJSON(raw []byte) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", err
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
