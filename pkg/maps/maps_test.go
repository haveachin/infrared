package maps_test

import (
	"reflect"
	"testing"

	"github.com/haveachin/infrared/pkg/maps"
)

func TestMerge(t *testing.T) {
	tt := []struct {
		name string
		dst  map[string]interface{}
		src  map[string]interface{}
		exp  map[string]interface{}
	}{
		{
			name: "WithShallowMap",
			dst: map[string]interface{}{
				"int":    1,
				"double": 1.5,
			},
			src: map[string]interface{}{
				"string": "string",
			},
			exp: map[string]interface{}{
				"int":    1,
				"double": 1.5,
				"string": "string",
			},
		},
		{
			name: "WithNestedMap",
			dst: map[string]interface{}{
				"map": map[string]interface{}{
					"mapStr": "mapStr",
				},
			},
			src: map[string]interface{}{
				"map": map[string]interface{}{
					"mapInt": 1,
				},
			},
			exp: map[string]interface{}{
				"map": map[string]interface{}{
					"mapStr": "mapStr",
					"mapInt": 1,
				},
			},
		},
		{
			name: "WithComplexNestedMap",
			dst: map[string]interface{}{
				"map": map[string]interface{}{
					"mapStr": "mapStr",
					"map": map[string]interface{}{
						"mapStr": "mapStr",
					},
				},
			},
			src: map[string]interface{}{
				"map": map[string]interface{}{
					"mapInt": 1,
					"map": map[string]interface{}{
						"mapInt": 1,
					},
				},
			},
			exp: map[string]interface{}{
				"map": map[string]interface{}{
					"mapStr": "mapStr",
					"mapInt": 1,
					"map": map[string]interface{}{
						"mapStr": "mapStr",
						"mapInt": 1,
					},
				},
			},
		},
		{
			name: "WithComplexNestedMapWithOverrides",
			dst: map[string]interface{}{
				"mapDup": "dst",
				"map": map[string]interface{}{
					"mapDup": "dst",
					"map": map[string]interface{}{
						"mapDup": "dst",
					},
				},
			},
			src: map[string]interface{}{
				"mapDup": "src",
				"map": map[string]interface{}{
					"mapDup": "src",
					"map": map[string]interface{}{
						"mapDup": "src",
					},
				},
			},
			exp: map[string]interface{}{
				"mapDup": "src",
				"map": map[string]interface{}{
					"mapDup": "src",
					"map": map[string]interface{}{
						"mapDup": "src",
					},
				},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			maps.Merge(tc.dst, tc.src)

			if !reflect.DeepEqual(tc.dst, tc.exp) {
				t.Fail()
			}
		})
	}
}
