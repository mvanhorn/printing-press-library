package traveloka

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

// ParseMoney accepts source integer minor units with explicit source currency and scale.
// Missing amount or nullOrEmpty is unknown; missing currency/scale is never guessed.
func ParseMoney(source any, scale any) (*Money, error) {
	if source == nil {
		return nil, nil
	}
	obj, ok := source.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("money must be a source object")
	}
	if empty, ok := obj["nullOrEmpty"].(bool); ok && empty {
		return nil, nil
	}
	amount, exists := obj["amount"]
	if !exists || amount == nil || isEmptyString(amount) {
		return nil, nil
	}
	raw, err := integerString(amount)
	if err != nil {
		return nil, fmt.Errorf("invalid money amount: %w", err)
	}
	decimals, err := moneyScale(scale)
	if err != nil {
		return nil, err
	}
	if embedded, ok := obj["numOfDecimalPoint"]; ok && embedded != nil && scale != nil {
		own, err := moneyScale(embedded)
		if err != nil {
			return nil, err
		}
		if own != nil && decimals != nil && *own != *decimals {
			return nil, fmt.Errorf("money decimal scale mismatch")
		}
	}
	currency, _ := obj["currency"].(string)
	m := &Money{Currency: currency, MinorUnits: raw, Decimals: decimals}
	if decimals != nil {
		m.Amount, err = FormatMinorUnits(raw, *decimals)
	}
	return m, err
}

func integerString(v any) (string, error) {
	var s string
	switch n := v.(type) {
	case string:
		s = n
	case json.Number:
		s = string(n)
	case int:
		s = strconv.Itoa(n)
	case int64:
		s = strconv.FormatInt(n, 10)
	case int32:
		s = strconv.FormatInt(int64(n), 10)
	case uint:
		s = strconv.FormatUint(uint64(n), 10)
	case uint64:
		s = strconv.FormatUint(n, 10)
	default:
		return "", fmt.Errorf("expected an exact integer string or json.Number")
	}
	if s == "" || strings.TrimSpace(s) != s {
		return "", fmt.Errorf("expected an exact integer")
	}
	for i, r := range s {
		if (r == '-' || r == '+') && i == 0 {
			continue
		}
		if r < '0' || r > '9' {
			return "", fmt.Errorf("expected integer minor units, without rounding")
		}
	}
	if _, ok := new(big.Int).SetString(s, 10); !ok {
		return "", fmt.Errorf("expected an exact integer")
	}
	return s, nil
}
func moneyScale(v any) (*int, error) {
	if v == nil || isEmptyString(v) {
		return nil, nil
	}
	s, err := integerString(v)
	if err != nil {
		return nil, fmt.Errorf("invalid money decimal scale")
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 || n > 18 {
		return nil, fmt.Errorf("money decimal scale must be an integer between 0 and 18")
	}
	return &n, nil
}

// FormatMinorUnits formats exact integer minor units without floating point or rounding.
func FormatMinorUnits(raw string, decimals int) (string, error) {
	if decimals < 0 || decimals > 18 {
		return "", fmt.Errorf("money decimal scale must be between 0 and 18")
	}
	if _, err := integerString(raw); err != nil {
		return "", err
	}
	n, _ := new(big.Int).SetString(raw, 10)
	negative := n.Sign() < 0
	digits := new(big.Int).Abs(n).String()
	if decimals > 0 {
		if len(digits) <= decimals {
			digits = strings.Repeat("0", decimals+1-len(digits)) + digits
		}
		digits = digits[:len(digits)-decimals] + "." + digits[len(digits)-decimals:]
	}
	if negative {
		digits = "-" + digits
	}
	return digits, nil
}

func isEmptyString(v any) bool { s, ok := v.(string); return ok && s == "" }
