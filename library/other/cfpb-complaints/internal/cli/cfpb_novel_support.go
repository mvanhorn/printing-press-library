// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/cfpb-complaints/internal/cliutil"
	"github.com/spf13/cobra"
)

const cfpbSearchURL = "https://www.consumerfinance.gov/data-research/consumer-complaints/search/api/v1/"

var cfpbHTTPClient = &http.Client{}
var cfpbRateMu sync.Mutex
var cfpbLastRequest time.Time

type cfpbHit struct {
	ID     string         `json:"_id"`
	Source map[string]any `json:"_source"`
}
type cfpbResponse struct {
	Hits struct {
		Total struct {
			Value int `json:"value"`
		} `json:"total"`
		Hits []cfpbHit `json:"hits"`
	} `json:"hits"`
	Aggregations map[string]json.RawMessage `json:"aggregations"`
	Meta         map[string]any             `json:"_meta"`
}

type complaintCohort struct {
	Company, Product, State, Window string
	Size                            int
	NarrativeOnly                   bool
}

func parseWindow(raw string) (time.Duration, error) {
	d, err := cliutil.ParseDurationLoose(raw)
	if err != nil || d < 24*time.Hour || d%(24*time.Hour) != 0 {
		return 0, fmt.Errorf("invalid window %q: use a whole number of days such as 7d or 4w", raw)
	}
	return d, nil
}

func cohortParams(c complaintCohort, start, end time.Time) url.Values {
	v := url.Values{"date_received_min": {start.UTC().Format("2006-01-02")}, "date_received_max": {end.UTC().Format("2006-01-02")}, "sort": {"created_date_desc"}}
	if c.Size > 0 {
		v.Set("size", strconv.Itoa(c.Size))
	}
	if c.Company != "" {
		v.Set("company", c.Company)
	}
	if c.Product != "" {
		v.Set("product", c.Product)
	}
	if c.State != "" {
		v.Set("state", strings.ToUpper(c.State))
	}
	if c.NarrativeOnly {
		v.Set("has_narrative", "true")
	}
	return v
}

func fetchCFPB(ctx context.Context, flags *rootFlags, params url.Values) (cfpbResponse, error) {
	u, _ := url.Parse(cfpbSearchURL)
	u.RawQuery = params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return cfpbResponse{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "cfpb-complaints-pp-cli/1.0.0")
	if flags != nil && flags.rateLimit > 0 {
		cfpbRateMu.Lock()
		gap := time.Duration(float64(time.Second) / flags.rateLimit)
		if wait := gap - time.Since(cfpbLastRequest); wait > 0 {
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				cfpbRateMu.Unlock()
				return cfpbResponse{}, ctx.Err()
			case <-timer.C:
			}
		}
		cfpbLastRequest = time.Now()
		cfpbRateMu.Unlock()
	}
	var resp *http.Response
	for attempt := 0; attempt < 3; attempt++ {
		resp, err = cfpbHTTPClient.Do(req)
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
				return cfpbResponse{}, ctx.Err()
			case <-time.After(time.Duration(attempt+1) * 250 * time.Millisecond):
			}
		}
	}
	if err != nil {
		return cfpbResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return cfpbResponse{}, fmt.Errorf("CFPB returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out cfpbResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, err
	}
	return out, nil
}

func bucketMap(response cfpbResponse, name string) (map[string]int, bool) {
	raw, ok := response.Aggregations[name]
	if !ok {
		return map[string]int{}, false
	}
	var value any
	if json.Unmarshal(raw, &value) != nil {
		return map[string]int{}, false
	}
	node := value
	if object, ok := node.(map[string]any); ok {
		if nested, exists := object[name]; exists {
			node = nested
		}
	}
	object, ok := node.(map[string]any)
	if !ok {
		return map[string]int{}, false
	}
	items, ok := object["buckets"].([]any)
	if !ok {
		return map[string]int{}, false
	}
	out := map[string]int{}
	for _, item := range items {
		row, ok := item.(map[string]any)
		if !ok {
			continue
		}
		key := fmt.Sprint(row["key"])
		if label, exists := row["key_as_string"]; exists {
			key = fmt.Sprint(label)
		}
		count, err := strconv.Atoi(fmt.Sprint(row["doc_count"]))
		if err == nil {
			out[key] = count
		}
	}
	truncated := fmt.Sprint(object["sum_other_doc_count"]) != "0" && fmt.Sprint(object["sum_other_doc_count"]) != "<nil>"
	return out, truncated
}

func buckets(response cfpbResponse, name string) []map[string]any {
	values, truncated := bucketMap(response, name)
	out := make([]map[string]any, 0, len(values))
	for value, count := range values {
		out = append(out, map[string]any{"value": value, "count": count})
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["count"].(int) > out[j]["count"].(int) })
	if truncated {
		out = append(out, map[string]any{"value": "__OTHER_BUCKETS_OMITTED__", "count": nil})
	}
	return out
}

func cohortSummary(response cfpbResponse) map[string]any {
	return map[string]any{"complaint_count": response.Hits.Total.Value, "products": buckets(response, "product"), "issues": buckets(response, "issue"), "company_responses": buckets(response, "company_response"), "timeliness": buckets(response, "timely"), "narrative_availability": buckets(response, "has_narrative")}
}

func deltaBuckets(current, baseline cfpbResponse, name string) []map[string]any {
	c, currentTruncated := bucketMap(current, name)
	b, baselineTruncated := bucketMap(baseline, name)
	keys := map[string]bool{}
	for k := range c {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	out := make([]map[string]any, 0, len(keys))
	for k := range keys {
		cv, cok := c[k]
		bv, bok := b[k]
		row := map[string]any{"value": k, "count_delta": nil, "present_in_current_buckets": cok, "present_in_baseline_buckets": bok, "current_aggregation_truncated": currentTruncated, "baseline_aggregation_truncated": baselineTruncated}
		if cok {
			row["current_count"] = cv
		}
		if bok {
			row["baseline_count"] = bv
		}
		if cok && bok {
			row["count_delta"] = cv - bv
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool {
		di, iKnown := out[i]["count_delta"].(int)
		dj, jKnown := out[j]["count_delta"].(int)
		if iKnown != jKnown {
			return iKnown // Comparable deltas sort before unknown one-sided buckets.
		}
		if di == dj {
			return out[i]["value"].(string) < out[j]["value"].(string)
		}
		return di > dj
	})
	return out
}

func emitCFPB(cmd *cobra.Command, flags *rootFlags, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return printOutputWithFlagsMeta(cmd.OutOrStdout(), raw, flags, map[string]any{"source": "live", "provider": "CFPB Consumer Complaint Database"})
}

func validateCohort(company, product, state, window string, size int) (complaintCohort, time.Duration, error) {
	if strings.TrimSpace(company) == "" {
		return complaintCohort{}, 0, errors.New("--company is required")
	}
	if size < 0 || size > 100 {
		return complaintCohort{}, 0, errors.New("size must be between 0 and 100")
	}
	d, err := parseWindow(window)
	return complaintCohort{Company: strings.TrimSpace(company), Product: strings.TrimSpace(product), State: strings.TrimSpace(state), Window: window, Size: size}, d, err
}

func standardCaveats() []string {
	return []string{"Complaint counts are raw published records, not rates; CFPB does not provide company market-share or account denominators.", "Published narratives are consumer-opt-in and redacted; narrative absence does not mean complaint absence.", "Category changes are descriptive and do not establish causation or company quality."}
}

func currentCalendarRange(duration time.Duration) (time.Time, time.Time) {
	now := time.Now().UTC()
	end := time.Date(now.Year(), now.Month(), now.Day()+1, 0, 0, 0, 0, time.UTC)
	return end.Add(-duration), end
}

func rangeMetadata(start, end time.Time) map[string]string {
	return map[string]string{"date_received_min": start.Format("2006-01-02"), "date_received_max_exclusive": end.Format("2006-01-02")}
}
