// Hand-authored World Bank envelope helpers. NOT generated — survives regen.
//
// Every World Bank v2 list response is a two-element JSON array:
//
//	[ {page,pages,per_page,total,...}, [ ...rows... ] ]
//
// The generator's response_path cannot index array element 1, so all novel
// commands parse the envelope through wbRows / wbFetchObservations here.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/world-bank/internal/client"
)

// wbMeta is the pagination object at index 0 of every World Bank response.
type wbMeta struct {
	Page    int `json:"page"`
	Pages   int `json:"pages"`
	PerPage any `json:"per_page"`
	Total   int `json:"total"`
}

// wbObservation is one indicator data point (index-1 rows of /country/.../indicator/...).
type wbObservation struct {
	Indicator       wbCodeValue `json:"indicator"`
	Country         wbCodeValue `json:"country"`
	CountryISO3Code string      `json:"countryiso3code"`
	Date            string      `json:"date"`
	Value           *float64    `json:"value"`
	Unit            string      `json:"unit"`
	ObsStatus       string      `json:"obs_status"`
	Decimal         int         `json:"decimal"`
}

type wbCodeValue struct {
	ID    string `json:"id"`
	Value string `json:"value"`
}

// wbParseEnvelope splits the [meta, [rows]] array. Element 0 may also be an
// error object {"message":[{"id","key","value"}]}; that case is surfaced.
func wbParseEnvelope(raw json.RawMessage) (wbMeta, []json.RawMessage, error) {
	var top []json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return wbMeta{}, nil, fmt.Errorf("unexpected World Bank response shape (not a 2-element array): %w", err)
	}
	if len(top) == 0 {
		return wbMeta{}, nil, fmt.Errorf("empty World Bank response")
	}
	// Error envelope: {"message":[...]}
	var maybeErr struct {
		Message []struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		} `json:"message"`
	}
	if json.Unmarshal(top[0], &maybeErr) == nil && len(maybeErr.Message) > 0 {
		return wbMeta{}, nil, fmt.Errorf("World Bank API error: %s — %s", maybeErr.Message[0].Key, maybeErr.Message[0].Value)
	}
	var meta wbMeta
	_ = json.Unmarshal(top[0], &meta)
	if len(top) < 2 {
		// Metadata only, no data array (valid: zero matches).
		return meta, nil, nil
	}
	var rows []json.RawMessage
	if err := json.Unmarshal(top[1], &rows); err != nil {
		// top[1] may be null when there is no data.
		return meta, nil, nil
	}
	return meta, rows, nil
}

// wbGetRows fetches one path and returns the parsed index-1 rows for a single page.
func wbGetRows(ctx context.Context, c *client.Client, path string, params map[string]string) (wbMeta, []json.RawMessage, error) {
	if params == nil {
		params = map[string]string{}
	}
	params["format"] = "json"
	raw, err := c.GetNoCache(ctx, path, params)
	if err != nil {
		return wbMeta{}, nil, err
	}
	return wbParseEnvelope(raw)
}

// wbGetAllRows pages through all results (capped) and returns every row.
func wbGetAllRows(ctx context.Context, c *client.Client, path string, params map[string]string, maxPages int) ([]json.RawMessage, error) {
	if params == nil {
		params = map[string]string{}
	}
	if params["per_page"] == "" {
		params["per_page"] = "1000"
	}
	var all []json.RawMessage
	page := 1
	for page <= maxPages {
		params["page"] = strconv.Itoa(page)
		meta, rows, err := wbGetRows(ctx, c, path, params)
		if err != nil {
			return all, err
		}
		all = append(all, rows...)
		if meta.Pages <= page || len(rows) == 0 {
			break
		}
		page++
	}
	return all, nil
}

// wbFetchObservations fetches indicator observations for ;-joined countries.
func wbFetchObservations(ctx context.Context, c *client.Client, countries, indicator string, extra map[string]string, maxPages int) ([]wbObservation, error) {
	if strings.TrimSpace(countries) == "" || strings.TrimSpace(indicator) == "" {
		return nil, fmt.Errorf("country and indicator are required")
	}
	path := fmt.Sprintf("/country/%s/indicator/%s", countries, indicator)
	rows, err := wbGetAllRows(ctx, c, path, extra, maxPages)
	if err != nil {
		return nil, err
	}
	obs := make([]wbObservation, 0, len(rows))
	for _, r := range rows {
		var o wbObservation
		if json.Unmarshal(r, &o) == nil {
			obs = append(obs, o)
		}
	}
	return obs, nil
}

// wbUnwrapEnvelope turns a raw [meta, [rows]] World Bank body into just the
// rows array for clean display. Non-envelope or error bodies are returned
// unchanged so the caller's existing handling still applies.
func wbUnwrapEnvelope(raw json.RawMessage) json.RawMessage {
	_, rows, err := wbParseEnvelope(raw)
	if err != nil {
		return raw
	}
	if rows == nil {
		return json.RawMessage("[]")
	}
	out, mErr := json.Marshal(rows)
	if mErr != nil {
		return raw
	}
	return out
}

// wbFloat renders a *float64 value for CSV/table output.
func wbFloat(v *float64) string {
	if v == nil {
		return ""
	}
	return strconv.FormatFloat(*v, 'f', -1, 64)
}
