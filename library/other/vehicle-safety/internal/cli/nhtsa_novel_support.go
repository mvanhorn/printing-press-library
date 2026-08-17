// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"
)

const nhtsaBaseURL = "https://api.nhtsa.gov"

var nhtsaHTTPClient = &http.Client{}
var nhtsaRateMu sync.Mutex
var nhtsaLastRequest time.Time

type vehicleQuery struct {
	Year  int    `json:"year"`
	Make  string `json:"make"`
	Model string `json:"model"`
}

type nhtsaResponse struct {
	Count   int              `json:"Count"`
	Message string           `json:"Message,omitempty"`
	Results []map[string]any `json:"results"`
}

type vehicleDossier struct {
	Vehicle           vehicleQuery      `json:"vehicle"`
	RetrievedAt       string            `json:"retrieved_at"`
	Recalls           []map[string]any  `json:"recalls"`
	Complaints        []map[string]any  `json:"complaints"`
	RatingVariants    []map[string]any  `json:"rating_variants"`
	Ratings           []map[string]any  `json:"ratings"`
	SourceAttribution map[string]string `json:"source_attribution"`
	Caveats           []string          `json:"caveats"`
}

func validateVehicle(year int, make, model string) (vehicleQuery, error) {
	make, model = strings.TrimSpace(make), strings.TrimSpace(model)
	currentYear := time.Now().Year() + 2
	if year < 1949 || year > currentYear {
		return vehicleQuery{}, fmt.Errorf("--year must be between 1949 and %d", currentYear)
	}
	if make == "" || model == "" {
		return vehicleQuery{}, errors.New("--make and --model are required")
	}
	return vehicleQuery{Year: year, Make: make, Model: model}, nil
}

func parseVehicleArg(raw string) (vehicleQuery, error) {
	parts := strings.Fields(raw)
	if len(parts) < 3 {
		return vehicleQuery{}, fmt.Errorf("vehicle %q must be quoted as 'YEAR MAKE MODEL'", raw)
	}
	year, err := strconv.Atoi(parts[0])
	if err != nil {
		return vehicleQuery{}, fmt.Errorf("vehicle %q has invalid year: %w", raw, err)
	}
	return validateVehicle(year, parts[1], strings.Join(parts[2:], " "))
}

func nhtsaGet(ctx context.Context, flags *rootFlags, base, path string, query url.Values) (nhtsaResponse, error) {
	u, err := url.Parse(base + path)
	if err != nil {
		return nhtsaResponse{}, err
	}
	u.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nhtsaResponse{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "vehicle-safety-pp-cli/1.0.0")
	if flags != nil && flags.rateLimit > 0 {
		nhtsaRateMu.Lock()
		minimumGap := time.Duration(float64(time.Second) / flags.rateLimit)
		if wait := minimumGap - time.Since(nhtsaLastRequest); wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				nhtsaRateMu.Unlock()
				return nhtsaResponse{}, ctx.Err()
			case <-timer.C:
			}
		}
		nhtsaLastRequest = time.Now()
		nhtsaRateMu.Unlock()
	}
	var resp *http.Response
	for attempt := 0; attempt < 3; attempt++ {
		resp, err = nhtsaHTTPClient.Do(req)
		if err == nil && resp.StatusCode != http.StatusTooManyRequests && resp.StatusCode < 500 {
			break
		}
		if resp != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
		if attempt < 2 {
			select {
			case <-ctx.Done():
				return nhtsaResponse{}, ctx.Err()
			case <-time.After(time.Duration(attempt+1) * 250 * time.Millisecond):
			}
		}
	}
	if err != nil {
		return nhtsaResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nhtsaResponse{}, fmt.Errorf("NHTSA returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out nhtsaResponse
	decoder := json.NewDecoder(resp.Body)
	decoder.UseNumber()
	if err := decoder.Decode(&out); err != nil {
		return nhtsaResponse{}, fmt.Errorf("decode NHTSA response: %w", err)
	}
	return out, nil
}

func vehicleParams(v vehicleQuery) url.Values {
	return url.Values{"modelYear": {strconv.Itoa(v.Year)}, "make": {v.Make}, "model": {v.Model}}
}

func fetchDossier(ctx context.Context, flags *rootFlags, v vehicleQuery) (vehicleDossier, error) {
	recalls, err := nhtsaGet(ctx, flags, nhtsaBaseURL, "/recalls/recallsByVehicle", vehicleParams(v))
	if err != nil {
		return vehicleDossier{}, fmt.Errorf("fetch recalls: %w", err)
	}
	complaints, err := nhtsaGet(ctx, flags, nhtsaBaseURL, "/complaints/complaintsByVehicle", vehicleParams(v))
	if err != nil {
		return vehicleDossier{}, fmt.Errorf("fetch complaints: %w", err)
	}
	ratingPath := fmt.Sprintf("/SafetyRatings/modelyear/%d/make/%s/model/%s", v.Year, url.PathEscape(v.Make), url.PathEscape(v.Model))
	variants, err := nhtsaGet(ctx, flags, nhtsaBaseURL, ratingPath, nil)
	if err != nil {
		return vehicleDossier{}, fmt.Errorf("fetch rating variants: %w", err)
	}
	var ratings []map[string]any
	for _, variant := range variants.Results {
		id := stringValue(variant, "VehicleId", "VehicleID")
		if id == "" {
			continue
		}
		detail, detailErr := nhtsaGet(ctx, flags, nhtsaBaseURL, "/SafetyRatings/VehicleId/"+url.PathEscape(id), nil)
		if detailErr != nil {
			return vehicleDossier{}, fmt.Errorf("fetch rating %s: %w", id, detailErr)
		}
		ratings = append(ratings, detail.Results...)
	}
	return vehicleDossier{
		Vehicle: v, RetrievedAt: time.Now().UTC().Format(time.RFC3339),
		Recalls: recalls.Results, Complaints: complaints.Results,
		RatingVariants: variants.Results, Ratings: ratings,
		SourceAttribution: map[string]string{
			"recalls": "NHTSA Recalls API", "complaints": "NHTSA Complaints API", "ratings": "NHTSA 5-Star Safety Ratings API",
		},
		Caveats: []string{
			"Complaint counts are raw reports, not rates; exposure and fleet-size denominators are unavailable.",
			"Model-level campaigns do not establish whether a particular VIN is covered or repaired.",
			"A missing rating means NHTSA returned no tested variant; it is not a zero-star rating.",
		},
	}, nil
}

func stringValue(item map[string]any, keys ...string) string {
	for _, key := range keys {
		for actual, value := range item {
			if strings.EqualFold(actual, key) && value != nil {
				if number, ok := value.(json.Number); ok {
					return number.String()
				}
				return strings.TrimSpace(fmt.Sprint(value))
			}
		}
	}
	return ""
}

func boolValue(item map[string]any, key string) bool {
	for actual, value := range item {
		if !strings.EqualFold(actual, key) {
			continue
		}
		switch typed := value.(type) {
		case bool:
			return typed
		case string:
			parsed, _ := strconv.ParseBool(typed)
			return parsed
		}
	}
	return false
}

func componentCounts(items []map[string]any) []map[string]any {
	counts := map[string]int{}
	for _, item := range items {
		value := stringValue(item, "components", "Component")
		component := strings.TrimSpace(value)
		if component != "" {
			counts[component]++
		}
	}
	type pair struct {
		name  string
		count int
	}
	rows := make([]pair, 0, len(counts))
	for name, count := range counts {
		rows = append(rows, pair{name, count})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].count == rows[j].count {
			return rows[i].name < rows[j].name
		}
		return rows[i].count > rows[j].count
	})
	out := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]any{"component": row.name, "count": row.count})
	}
	return out
}

type communicationRecord struct {
	ID, DocumentID, Date, Type, Make, Model, Year, Components, System, Subsystem, Summary string
}

func readCommunicationFile(path string, v vehicleQuery) ([]communicationRecord, error) {
	// #nosec G304 -- path is an explicit operator-supplied CLI input.
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	var matches []communicationRecord
	for {
		row, readErr := r.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return nil, fmt.Errorf("read manufacturer communications TSV: %w", readErr)
		}
		if len(row) < 14 {
			continue
		}
		record := communicationRecord{ID: row[0], Date: row[4], DocumentID: row[3], Type: row[6], Make: row[7], Model: row[8], Year: row[9], Components: row[10], System: row[11], Subsystem: row[12], Summary: row[13]}
		if strings.EqualFold(strings.TrimSpace(record.Make), v.Make) && strings.EqualFold(strings.TrimSpace(record.Model), v.Model) && strings.TrimSpace(record.Year) == strconv.Itoa(v.Year) {
			matches = append(matches, record)
		}
	}
	return matches, nil
}

func emitLive(cmd *cobra.Command, flags *rootFlags, value any) error {
	return emitNHTSA(cmd, flags, "live", value)
}

func emitNHTSA(cmd *cobra.Command, flags *rootFlags, source string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return printOutputWithFlagsMeta(cmd.OutOrStdout(), raw, flags, map[string]any{"source": source, "provider": "NHTSA", "retrieved_at": time.Now().UTC().Format(time.RFC3339)})
}
