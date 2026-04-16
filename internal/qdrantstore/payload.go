package qdrantstore

import (
	"fmt"

	"github.com/qdrant/go-client/qdrant"
)

// payloadToMap converts Qdrant payload values to plain Go values for JSON APIs.
func payloadToMap(in map[string]*qdrant.Value) map[string]any {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		if v == nil {
			continue
		}
		out[k] = valueToAny(v)
	}
	return out
}

func valueToAny(v *qdrant.Value) any {
	switch k := v.GetKind().(type) {
	case nil:
		return nil
	case *qdrant.Value_NullValue:
		return nil
	case *qdrant.Value_DoubleValue:
		return k.DoubleValue
	case *qdrant.Value_IntegerValue:
		return k.IntegerValue
	case *qdrant.Value_StringValue:
		return k.StringValue
	case *qdrant.Value_BoolValue:
		return k.BoolValue
	case *qdrant.Value_StructValue:
		if k.StructValue == nil {
			return nil
		}
		m := make(map[string]any)
		for fk, fv := range k.StructValue.GetFields() {
			m[fk] = valueToAny(fv)
		}
		return m
	case *qdrant.Value_ListValue:
		if k.ListValue == nil {
			return []any{}
		}
		items := k.ListValue.GetValues()
		out := make([]any, 0, len(items))
		for _, it := range items {
			out = append(out, valueToAny(it))
		}
		return out
	default:
		return fmt.Sprintf("%v", k)
	}
}
