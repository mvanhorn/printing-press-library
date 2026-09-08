// Copyright 2026 Max Michel and contributors. Licensed under Apache-2.0. See LICENSE.

package portfolio

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// NormalizeEndpoint converts the two public command names and their RPC
// spellings to a stable portfolio endpoint. Account is reserved for the
// verified currency/account helper receipt and is never normalized as Planner
// data.
func NormalizeEndpoint(value string) (string, error) {
	v := strings.ToLower(strings.TrimSpace(value))
	v = strings.ReplaceAll(v, "_", "")
	v = strings.ReplaceAll(v, "-", "")
	v = strings.ReplaceAll(v, ".", "")
	v = strings.ReplaceAll(v, "/", "")
	switch {
	case v == EndpointIdeas, strings.Contains(v, "generatekeywordideas"):
		return EndpointIdeas, nil
	case v == EndpointHistorical, strings.Contains(v, "generatekeywordhistoricalmetrics"):
		return EndpointHistorical, nil
	case v == EndpointAccount, strings.Contains(v, "listaccessiblecustomers"), strings.Contains(v, "customerssearch"), strings.Contains(v, "googleadssearch"):
		return EndpointAccount, nil
	default:
		return "", fmt.Errorf("%w: unsupported endpoint %q", ErrInvalidInput, value)
	}
}

// ParseYearMonth validates the ISO YYYY-MM form used by requests, SQLite and
// DuckDB. It does not silently accept a day or a vendor enum number.
func ParseYearMonth(value string) (Month, error) {
	v := strings.TrimSpace(value)
	if len(v) != 7 || v[4] != '-' {
		return Month{}, fmt.Errorf("%w: month %q must be YYYY-MM", ErrInvalidInput, value)
	}
	year, errYear := strconv.Atoi(v[:4])
	number, errMonth := strconv.Atoi(v[5:])
	if errYear != nil || errMonth != nil || year < 1 || number < 1 || number > 12 {
		return Month{}, fmt.Errorf("%w: invalid month %q", ErrInvalidInput, value)
	}
	return Month{Year: year, Number: number}, nil
}

func formatMonth(m Month) string {
	if m.Year < 1 || m.Number < 1 || m.Number > 12 {
		return ""
	}
	return fmt.Sprintf("%04d-%02d", m.Year, m.Number)
}

func compareMonth(a, b Month) int {
	if a.Year < b.Year || (a.Year == b.Year && a.Number < b.Number) {
		return -1
	}
	if a.Year > b.Year || (a.Year == b.Year && a.Number > b.Number) {
		return 1
	}
	return 0
}

func currentMonth(now time.Time) Month {
	now = currentTime(now)
	return Month{Year: now.Year(), Number: int(now.Month())}
}

func validateRange(start, end string, now time.Time) (Month, Month, error) {
	from, err := ParseYearMonth(start)
	if err != nil {
		return Month{}, Month{}, err
	}
	to, err := ParseYearMonth(end)
	if err != nil {
		return Month{}, Month{}, err
	}
	if compareMonth(from, to) > 0 {
		return Month{}, Month{}, fmt.Errorf("%w: requested start %q is after end %q", ErrInvalidInput, start, end)
	}
	if compareMonth(to, currentMonth(now)) >= 0 {
		return Month{}, Month{}, fmt.Errorf("%w: requested end %q is current or future; only closed months are allowed", ErrInvalidInput, end)
	}
	return from, to, nil
}

func monthsBetween(from, to Month) []Month {
	if compareMonth(from, to) > 0 {
		return []Month{}
	}
	result := make([]Month, 0, (to.Year-from.Year)*12+to.Number-from.Number+1)
	for year, number := from.Year, from.Number; year < to.Year || (year == to.Year && number <= to.Number); {
		result = append(result, Month{Year: year, Number: number})
		number++
		if number == 13 {
			number = 1
			year++
		}
	}
	return result
}

func geoSetID(geos []string) (string, []string, error) {
	copyGeos := append([]string(nil), geos...)
	for i := range copyGeos {
		copyGeos[i] = strings.TrimSpace(copyGeos[i])
		if copyGeos[i] == "" {
			return "", nil, fmt.Errorf("%w: geo target at index %d is empty", ErrInvalidInput, i)
		}
	}
	sort.Strings(copyGeos)
	encoded, err := json.Marshal(copyGeos)
	if err != nil {
		return "", nil, fmt.Errorf("encode geo target set: %w", err)
	}
	hash := sha256.Sum256(encoded)
	return hex.EncodeToString(hash[:]), copyGeos, nil
}

func marshalStrings(values []string) ([]byte, error) {
	if values == nil {
		values = []string{}
	}
	return json.Marshal(values)
}

func unmarshalStrings(data []byte) ([]string, error) {
	if len(data) == 0 {
		return []string{}, nil
	}
	var values []string
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, err
	}
	if values == nil {
		return []string{}, nil
	}
	return values, nil
}

func sortedUnique(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func flagsJSON(flags []string) ([]byte, error) {
	flags = sortedUnique(flags)
	if flags == nil {
		flags = []string{}
	}
	return json.Marshal(flags)
}

func parseFlags(data []byte) ([]string, error) {
	values, err := unmarshalStrings(data)
	if err != nil {
		return nil, err
	}
	return sortedUnique(values), nil
}
