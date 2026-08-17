// Copyright 2026 Dhilip Subramanian and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	openAQKeyEnv  = "AIR_QUALITY_OPENAQ_API_KEY"
	openAQBaseEnv = "AIR_QUALITY_OPENAQ_BASE_URL"
)

type openAQClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func newOpenAQClient(timeout time.Duration) openAQClient {
	baseURL := strings.TrimRight(strings.TrimSpace(env(openAQBaseEnv)), "/")
	if baseURL == "" {
		baseURL = "https://api.openaq.org"
	}
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return openAQClient{
		baseURL: baseURL,
		apiKey:  strings.TrimSpace(env(openAQKeyEnv)),
		http:    &http.Client{Timeout: timeout},
	}
}

func (c openAQClient) configured() bool {
	return c.apiKey != ""
}

func (c openAQClient) locations(ctx context.Context, lat, lon float64, radius, limit int) ([]LocationSummary, error) {
	if radius <= 0 || radius > 25000 {
		radius = 25000
	}
	if limit <= 0 {
		limit = 5
	}
	values := url.Values{}
	values.Set("coordinates", fmt.Sprintf("%.4f,%.4f", lat, lon))
	values.Set("radius", strconv.Itoa(radius))
	values.Set("limit", strconv.Itoa(limit))
	root, err := c.get(ctx, "/v3/locations", values)
	if err != nil {
		return nil, err
	}
	return summarizeLocations(extractResults(root)), nil
}

func (c openAQClient) location(ctx context.Context, id string) (LocationSummary, error) {
	root, err := c.get(ctx, "/v3/locations/"+url.PathEscape(id), nil)
	if err != nil {
		return LocationSummary{}, err
	}
	results := extractResults(root)
	if len(results) == 0 {
		return LocationSummary{}, nil
	}
	return summarizeLocation(results[0]), nil
}

func (c openAQClient) latestByLocation(ctx context.Context, id string, sensors []SensorSummary) ([]Measurement, string, map[string]any, error) {
	root, err := c.get(ctx, "/v3/locations/"+url.PathEscape(id)+"/latest", nil)
	if err != nil {
		return nil, "", nil, err
	}
	measurements := summarizeMeasurementsWithSensors(extractResults(root), sensors)
	return measurements, latestTimestamp(measurements), root, nil
}

func (c openAQClient) measurementsBySensor(ctx context.Context, sensorID string, days int) ([]Measurement, string, error) {
	if days <= 0 {
		days = 7
	}
	if days > 31 {
		days = 31
	}
	const limit = 100
	const maxPages = 10
	all := make([]any, 0, limit)
	for page := 1; page <= maxPages; page++ {
		values := url.Values{}
		values.Set("limit", strconv.Itoa(limit))
		values.Set("page", strconv.Itoa(page))
		values.Set("datetime_from", time.Now().UTC().AddDate(0, 0, -days).Format(time.RFC3339))
		root, err := c.get(ctx, "/v3/sensors/"+url.PathEscape(sensorID)+"/measurements", values)
		if err != nil {
			return nil, "", err
		}
		results := extractResults(root)
		all = append(all, results...)
		if len(results) < limit || paginationComplete(root, len(all)) {
			break
		}
		if page == maxPages {
			return nil, "", fmt.Errorf("openaq history exceeded %d pages; reduce --days for a bounded result", maxPages)
		}
	}
	measurements := summarizeMeasurements(all)
	return measurements, latestTimestamp(measurements), nil
}

func (c openAQClient) get(ctx context.Context, path string, values url.Values) (map[string]any, error) {
	if !c.configured() {
		return nil, fmt.Errorf("missing %s", openAQKeyEnv)
	}
	endpoint := c.baseURL + path
	if len(values) > 0 {
		endpoint += "?" + values.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("openaq %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	var root map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&root); err != nil {
		return nil, err
	}
	return root, nil
}

func openAQGuidance(command string, query map[string]any) GuidanceResult {
	return GuidanceResult{
		Source:     "OpenAQ API v3",
		Configured: false,
		Command:    command,
		Query:      query,
		Title:      "OpenAQ API key is required for live measurement requests",
		Setup: []string{
			"Create an OpenAQ Explorer account and API key.",
			"Set AIR_QUALITY_OPENAQ_API_KEY before running live OpenAQ commands.",
			"Use sources or doctor to confirm which source families are configured.",
		},
		Caveats: []string{
			"OpenAQ returns physical pollutant measurements and metadata, not AQI categories.",
			"OpenAQ free limits are documented as 60 requests per minute and 2,000 requests per hour.",
			"OpenAQ v1 and v2 were retired on January 31, 2025; this CLI uses v3 only.",
		},
	}
}

func extractResults(root map[string]any) []any {
	if root == nil {
		return nil
	}
	if results, ok := root["results"].([]any); ok {
		return results
	}
	if result, ok := root["result"]; ok {
		return []any{result}
	}
	return []any{root}
}

func summarizeLocations(values []any) []LocationSummary {
	locations := make([]LocationSummary, 0, len(values))
	for _, value := range values {
		locations = append(locations, summarizeLocation(value))
	}
	return locations
}

func summarizeLocation(value any) LocationSummary {
	m, _ := value.(map[string]any)
	if m == nil {
		return LocationSummary{Raw: value}
	}
	loc := LocationSummary{
		ID:          anyString(m["id"]),
		Name:        firstString(m["name"], m["location"], m["label"]),
		Locality:    firstString(m["locality"], m["city"]),
		Country:     countryString(m["country"], m["countryCode"], m["iso"]),
		Coordinates: coordinatesMap(m["coordinates"]),
		Providers:   stringList(m["providers"], "name"),
		Sensors:     sensorList(m["sensors"]),
		Raw:         m,
	}
	if loc.Name == "" {
		loc.Name = "OpenAQ location " + loc.ID
	}
	return loc
}

func summarizeMeasurements(values []any) []Measurement {
	return summarizeMeasurementsWithSensors(values, nil)
}

func summarizeMeasurementsWithSensors(values []any, sensors []SensorSummary) []Measurement {
	sensorIndex := map[string]SensorSummary{}
	for _, sensor := range sensors {
		if sensor.ID != "" {
			sensorIndex[sensor.ID] = sensor
		}
	}
	measurements := make([]Measurement, 0, len(values))
	for _, value := range values {
		measurements = append(measurements, summarizeMeasurement(value, sensorIndex))
	}
	return measurements
}

func summarizeMeasurement(value any, sensorIndex map[string]SensorSummary) Measurement {
	m, _ := value.(map[string]any)
	if m == nil {
		return Measurement{Raw: value}
	}
	parameter := firstString(m["parameter"], m["parameterName"], m["parameter_name"])
	unit := firstString(m["unit"], m["units"])
	if parameterMap, ok := m["parameter"].(map[string]any); ok {
		parameter = firstString(parameterMap["name"], parameterMap["displayName"], parameterMap["parameter"])
		unit = firstString(parameterMap["units"], parameterMap["unit"])
	}
	sensorID := firstString(m["sensor_id"], m["sensors_id"], m["sensorId"], m["sensorsId"], m["sensorID"])
	if sensorMap, ok := m["sensor"].(map[string]any); ok {
		sensorID = firstString(sensorMap["id"], sensorMap["sensor_id"], sensorMap["sensorsId"])
	}
	if sensor := sensorIndex[sensorID]; parameter == "" && sensor.Parameter != "" {
		parameter = sensor.Parameter
		unit = sensor.Unit
	}
	if parameter == "" {
		if parameterID := firstString(m["parameter_id"], m["parameterId"], m["parameters_id"], m["parametersId"]); parameterID != "" {
			parameter = "parameter:" + parameterID
		} else if sensorID != "" {
			parameter = "sensor:" + sensorID
		} else {
			parameter = "unknown"
		}
	}
	return Measurement{
		Parameter: parameter,
		Value:     anyString(firstNonNil(m["value"], m["coverage"], m["average"])),
		Unit:      unit,
		Timestamp: timestampString(firstNonNil(m["datetime"], m["date"], m["time"], m["period"])),
		SensorID:  sensorID,
		Raw:       m,
	}
}

func latestTimestamp(measurements []Measurement) string {
	for _, measurement := range measurements {
		if measurement.Timestamp != "" {
			return measurement.Timestamp
		}
	}
	return ""
}

func paginationComplete(root map[string]any, seen int) bool {
	meta, _ := root["meta"].(map[string]any)
	if meta == nil {
		return false
	}
	for _, key := range []string{"found", "total", "count"} {
		if value, ok := anyFloat(meta[key]); ok && value > 0 {
			return seen >= int(value)
		}
	}
	return false
}

func countryString(values ...any) string {
	for _, value := range values {
		if s := anyString(value); s != "" {
			return s
		}
		if m, ok := value.(map[string]any); ok {
			if s := firstString(m["code"], m["iso"], m["name"]); s != "" {
				return s
			}
		}
	}
	return ""
}

func coordinatesMap(value any) map[string]float64 {
	m, _ := value.(map[string]any)
	if m == nil {
		return nil
	}
	lat, latOK := anyFloat(firstNonNil(m["latitude"], m["lat"]))
	lon, lonOK := anyFloat(firstNonNil(m["longitude"], m["lon"], m["lng"]))
	if !latOK || !lonOK {
		return nil
	}
	return map[string]float64{"latitude": lat, "longitude": lon}
}

func sensorList(value any) []SensorSummary {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	sensors := make([]SensorSummary, 0, len(items))
	for _, item := range items {
		m, _ := item.(map[string]any)
		if m == nil {
			continue
		}
		parameter := firstString(m["parameter"], m["parameterName"])
		unit := firstString(m["unit"], m["units"])
		if parameterMap, ok := m["parameter"].(map[string]any); ok {
			parameter = firstString(parameterMap["name"], parameterMap["displayName"])
			unit = firstString(parameterMap["units"], parameterMap["unit"])
		}
		sensors = append(sensors, SensorSummary{
			ID:        anyString(m["id"]),
			Parameter: parameter,
			Unit:      unit,
		})
	}
	return sensors
}

func stringList(value any, field string) []string {
	items, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if s := anyString(item); s != "" {
			out = append(out, s)
			continue
		}
		if m, ok := item.(map[string]any); ok {
			if s := firstString(m[field], m["name"], m["id"]); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

func firstString(values ...any) string {
	for _, value := range values {
		if s := anyString(value); s != "" {
			return s
		}
	}
	return ""
}

func firstNonNil(values ...any) any {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func anyString(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case json.Number:
		return v.String()
	case float64:
		if v == float64(int64(v)) {
			return strconv.FormatInt(int64(v), 10)
		}
		return strconv.FormatFloat(v, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(v), 'f', -1, 32)
	case int:
		return strconv.Itoa(v)
	case int64:
		return strconv.FormatInt(v, 10)
	case bool:
		return strconv.FormatBool(v)
	case map[string]any:
		return firstString(v["name"], v["label"], v["id"], v["utc"], v["local"], v["datetime"])
	default:
		return fmt.Sprint(v)
	}
}

func anyFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

func timestampString(value any) string {
	if s := anyString(value); s != "" {
		return s
	}
	if m, ok := value.(map[string]any); ok {
		return firstString(m["utc"], m["local"], m["date"], m["datetime"], m["from"], m["to"])
	}
	return ""
}
