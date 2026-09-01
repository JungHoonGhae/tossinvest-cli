package jsoninput

import (
	"encoding/json"
	"testing"
)

func TestDecodePreservesInterfaceNumbers(t *testing.T) {
	var got map[string]any
	if err := Decode([]byte(`{"value":9007199254740993}`), &got); err != nil {
		t.Fatal(err)
	}
	n, ok := got["value"].(json.Number)
	if !ok || n.String() != "9007199254740993" {
		t.Fatalf("number was rounded or changed: %#v", got["value"])
	}
}

func TestDecodeRejectsTrailingJSONValue(t *testing.T) {
	var got map[string]any
	if err := Decode([]byte(`{} {}`), &got); err == nil {
		t.Fatal("multiple JSON values must be rejected")
	}
}
