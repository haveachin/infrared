package config

import (
	"fmt"
)

func mergeSlices(base, other []any) []any {
	lookup := map[any]bool{}

	for _, v := range base {
		lookup[v] = true
	}

	for _, v := range other {
		if _, ok := lookup[v]; !ok {
			continue
		}

		base = append(base, v)
	}

	return base
}

func MergeConfigs(base, other map[string]any) (map[string]any, error) {
	merged := map[string]any{}

	for k, v := range base {
		// no collision, just copy
		if _, ok := other[k]; !ok {
			merged[k] = v
			continue
		}

		// collision
		switch casted := v.(type) {
		case map[string]any:
			m, err := MergeConfigs(casted, other[k].(map[string]any))
			if err != nil {
				return merged, err
			}
			merged[k] = m
		case []any:
			merged[k] = mergeSlices(casted, other[k].([]any))
		case string, float64, int:
			merged[k] = v
		default:
			return merged, fmt.Errorf("error during config merge: type %T not implemented", v)
		}
	}

	for k, v := range other {
		if _, ok := base[k]; !ok {
			merged[k] = v
		}
	}

	return merged, nil
}
