package qdrantstore

import (
	"testing"

	"github.com/qdrant/go-client/qdrant"
)

func TestValueToAny_primitives(t *testing.T) {
	cases := []struct {
		name string
		v    *qdrant.Value
		want any
	}{
		{"string", qdrant.NewValueString("x"), "x"},
		{"bool", qdrant.NewValueBool(true), true},
		{"int", qdrant.NewValueInt(42), int64(42)},
		{"double", qdrant.NewValueDouble(1.5), 1.5},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := valueToAny(tc.v)
			if got != tc.want {
				t.Fatalf("got %#v want %#v", got, tc.want)
			}
		})
	}
}

func TestValueToAny_list(t *testing.T) {
	v := qdrant.NewValueFromList(
		qdrant.NewValueString("a"),
		qdrant.NewValueInt(2),
	)
	got := valueToAny(v)
	sl, ok := got.([]any)
	if !ok || len(sl) != 2 || sl[0] != "a" || sl[1] != int64(2) {
		t.Fatalf("unexpected list %#v", got)
	}
}

func TestPayloadToMap(t *testing.T) {
	m := payloadToMap(map[string]*qdrant.Value{
		"text": qdrant.NewValueString("hello"),
		"n":    qdrant.NewValueInt(7),
	})
	if m["text"] != "hello" || m["n"] != int64(7) {
		t.Fatalf("unexpected map %#v", m)
	}
}
