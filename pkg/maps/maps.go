package maps

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
				continue
			}
			Merge(dvType, svMap)
		default:
			dst[k] = sv
		}
	}
}
