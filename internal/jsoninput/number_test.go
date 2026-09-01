package jsoninput

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestIntParsesExactJSONIntegers(t *testing.T) {
	for input, want := range map[json.Number]int64{
		"1":   1,
		"1.0": 1,
		"1e3": 1000,
	} {
		got, err := Int(input, 64)
		if err != nil || got != want {
			t.Errorf("Int(%q) = %d, %v; want %d", input, got, err, want)
		}
	}
}

func TestIntRejectsFractionalAndOutOfRangeJSONNumbers(t *testing.T) {
	for _, input := range []json.Number{"1.9", "9223372036854775808"} {
		if _, err := Int(input, 64); err == nil {
			t.Errorf("Int(%q) unexpectedly succeeded", input)
		}
	}
}

func TestIntRejectsAmplifyingExponentBeforeBigIntegerExpansion(t *testing.T) {
	_, err := Int(json.Number("1e10000"), 64)
	if err == nil || !strings.Contains(err.Error(), "numeric input is too large") {
		t.Fatalf("amplifying exponent must hit the lexical bound, got %v", err)
	}
}

func TestFloat64RejectsLossyJSONInteger(t *testing.T) {
	if _, err := Float64(json.Number("9007199254740993")); !errors.Is(err, ErrIntegerPrecisionLoss) {
		t.Fatalf("lossy integer error = %v, want ErrIntegerPrecisionLoss", err)
	}
	got, err := Float64(json.Number("1.25"))
	if err != nil || got != 1.25 {
		t.Fatalf("Float64(1.25) = %v, %v", got, err)
	}
}
