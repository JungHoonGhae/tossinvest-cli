package output

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestProjectionPreservesNumbersArraysAndNulls(t *testing.T) {
	input := map[string]any{"snapshot": map[string]any{"id": int64(9007199254740993), "date": "2026-09-11"}, "positions": []any{map[string]any{"symbol": "AAPL", "quantity": 0.25, "extra": "omit"}, map[string]any{"symbol": "005930", "quantity": 0.0}}, "empty": []any{}, "nullable": nil}
	var buf bytes.Buffer
	err := WriteJSON(WithJSONOptions(&buf, JSONOptions{Fields: []string{"snapshot.id", "positions.symbol", "positions.quantity", "empty.name", "nullable.value"}, Compact: true}), input)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"empty":[],"nullable":null,"positions":[{"quantity":0.25,"symbol":"AAPL"},{"quantity":0,"symbol":"005930"}],"snapshot":{"id":9007199254740993}}` + "\n"
	if buf.String() != want {
		t.Fatalf("projection=%s", buf.String())
	}
	if !reflect.DeepEqual(input["positions"].([]any)[0].(map[string]any)["extra"], "omit") {
		t.Fatal("input mutated")
	}
	for _, fields := range [][]string{{"positions..symbol"}, {"positions[0]"}, {" *"}, {""}} {
		if _, err := Project(input, fields); err == nil {
			t.Fatalf("invalid fields accepted: %v", fields)
		}
	}
}

func TestCompactKeepsAllFieldsAndParentSelectionWins(t *testing.T) {
	var buf bytes.Buffer
	value := map[string]any{"thing": map[string]any{"large": json.Number("9007199254740993"), "other": "yes"}}
	if err := WriteJSON(WithJSONOptions(&buf, JSONOptions{Fields: []string{"thing.large", "thing"}, Compact: true}), value); err != nil {
		t.Fatal(err)
	}
	if strings.Count(buf.String(), "\n") != 1 || !strings.Contains(buf.String(), `"other":"yes"`) {
		t.Fatalf("compact lost fields: %s", buf.String())
	}
	buf.Reset()
	if err := WriteJSON(&buf, value); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "\n  ") {
		t.Fatal("default indent changed")
	}
}
