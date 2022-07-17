package maps

import "fmt"

// Merge merges two maps
func Merge[T comparable](dst, src map[T]interface{}) {
	for k, sv := range src {
		dv, ok := dst[k]
		if !ok {
			dst[k] = sv
			continue
		}

		switch dvType := dv.(type) {
		case map[T]interface{}:
			svMap, ok := sv.(map[T]interface{})
			if !ok {
				dst[k] = sv
				continue
			}
			Merge(dvType, svMap)
		case map[interface{}]interface{}:
			svMap, ok := sv.(map[interface{}]interface{})
			if !ok {
				dst[k] = sv
				continue
			}

			dvStrMap := castToStringKeyMap(dvType)
			svStrMap := castToStringKeyMap(svMap)

			Merge(dvStrMap, svStrMap)

			dst[k] = dvStrMap
		default:
			dst[k] = sv
		}
	}
}

func castToStringKeyMap(src map[interface{}]interface{}) map[string]interface{} {
	dst := map[string]interface{}{}
	for k, v := range src {
		dst[fmt.Sprintf("%v", k)] = v
	}
	return dst
}
