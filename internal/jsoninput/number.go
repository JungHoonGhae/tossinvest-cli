package jsoninput

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

const (
	maxJSONNumberChars    = 128
	maxJSONNumberExponent = 400
)

// ErrIntegerPrecisionLoss reports an exact JSON integer that cannot be
// represented by float64 without changing its value.
var ErrIntegerPrecisionLoss = errors.New("integer precision loss")

// Int converts a JSON number to a signed integer without rounding. Decimal and
// exponent spellings are accepted when their exact value is integral.
func Int(n json.Number, bitSize int) (int64, error) {
	r, err := boundedRat(n)
	if err != nil {
		return 0, err
	}
	if !r.IsInt() || !r.Num().IsInt64() {
		return 0, fmt.Errorf("%q is not an integer", n)
	}
	value := r.Num().Int64()
	if bitSize > 0 && bitSize < 64 {
		limit := int64(1) << (bitSize - 1)
		if value < -limit || value >= limit {
			return 0, fmt.Errorf("%q is outside the signed %d-bit integer range", n, bitSize)
		}
	}
	return value, nil
}

// Float64 converts a JSON number to a finite float64 while rejecting exact
// integer values outside float64's universally lossless integer range.
func Float64(n json.Number) (float64, error) {
	if err := validateNumericLexeme(n.String()); err != nil {
		return 0, err
	}
	value, err := strconv.ParseFloat(n.String(), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, fmt.Errorf("%q is not a finite float64", n)
	}
	r, err := boundedRat(n)
	if err != nil {
		return 0, err
	}
	if r.IsInt() {
		limit := big.NewInt(1 << 53)
		if new(big.Int).Abs(r.Num()).Cmp(limit) > 0 {
			return 0, ErrIntegerPrecisionLoss
		}
	}
	return value, nil
}

func boundedRat(n json.Number) (*big.Rat, error) {
	if err := validateNumericLexeme(n.String()); err != nil {
		return nil, err
	}
	r, ok := new(big.Rat).SetString(n.String())
	if !ok {
		return nil, fmt.Errorf("%q is not a JSON number", n)
	}
	return r, nil
}

func validateNumericLexeme(value string) error {
	if len(value) == 0 || len(value) > maxJSONNumberChars {
		return fmt.Errorf("numeric input is too large")
	}
	if index := strings.IndexAny(value, "eE"); index >= 0 {
		exponent, err := strconv.ParseInt(value[index+1:], 10, 32)
		if err != nil || exponent < -maxJSONNumberExponent || exponent > maxJSONNumberExponent {
			return fmt.Errorf("numeric input is too large")
		}
	}
	return nil
}
