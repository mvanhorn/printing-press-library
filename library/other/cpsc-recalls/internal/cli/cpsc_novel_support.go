// Copyright 2026 avanderheyde and contributors. Licensed under Apache-2.0. See LICENSE.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/mvanhorn/printing-press-library/library/other/cpsc-recalls/internal/cliutil"
	"github.com/spf13/cobra"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

const cpscRecallURL = "https://www.saferproducts.gov/RestWebServices/Recall"

const cpscMaxAttempts = 3

type cpscPacer struct {
	mu       sync.Mutex
	interval time.Duration
	next     time.Time
}

func newCPSCPacer(flags *rootFlags) *cpscPacer {
	if flags == nil || flags.rateLimit <= 0 {
		return nil
	}
	return &cpscPacer{interval: time.Duration(float64(time.Second) / flags.rateLimit)}
}

func (p *cpscPacer) wait(ctx context.Context) error {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	now := time.Now()
	when := now
	if p.next.After(now) {
		when = p.next
	}
	p.next = when.Add(p.interval)
	p.mu.Unlock()
	if delay := time.Until(when); delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	return nil
}

type cpscFetcher struct {
	baseURL     string
	httpClient  *http.Client
	pacer       *cpscPacer
	maxAttempts int
	waitRetry   func(context.Context, time.Duration) error
}

func newCPSCFetcher(flags *rootFlags) *cpscFetcher {
	return &cpscFetcher{
		baseURL:     cpscRecallURL,
		httpClient:  http.DefaultClient,
		pacer:       newCPSCPacer(flags),
		maxAttempts: cpscMaxAttempts,
		waitRetry:   waitCPSC,
	}
}

func waitCPSC(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (f *cpscFetcher) fetch(ctx context.Context, params url.Values) ([]map[string]any, error) {
	if params == nil {
		params = url.Values{}
	}
	cloned := make(url.Values, len(params))
	for key, values := range params {
		cloned[key] = append([]string(nil), values...)
	}
	params = cloned
	params.Set("format", "json")
	var lastErr error
	for attempt := 0; attempt < f.maxAttempts; attempt++ {
		if err := f.pacer.wait(ctx); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, f.baseURL+"?"+params.Encode(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("User-Agent", "cpsc-recalls-pp-cli/1.0.0")
		resp, err := f.httpClient.Do(req)
		if err != nil {
			lastErr = err
		} else {
			body, readErr := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
			_ = resp.Body.Close()
			if readErr != nil {
				lastErr = readErr
			} else if resp.StatusCode == http.StatusTooManyRequests {
				lastErr = rateLimitErr(&cliutil.RateLimitError{URL: req.URL.String(), RetryAfter: cliutil.RetryAfter(resp), Body: string(body)})
			} else if resp.StatusCode != http.StatusOK {
				lastErr = apiErr(fmt.Errorf("CPSC returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body))))
				if resp.StatusCode < 500 {
					return nil, lastErr
				}
			} else {
				decoder := json.NewDecoder(strings.NewReader(string(body)))
				decoder.UseNumber()
				var out []map[string]any
				if err := decoder.Decode(&out); err != nil {
					return nil, apiErr(fmt.Errorf("decode CPSC response: %w", err))
				}
				providerError := ""
				for _, row := range out {
					if title := fmt.Sprint(row["Title"]); strings.HasPrefix(title, "Error retrieving Recalls:") {
						providerError = title
						break
					}
				}
				if providerError != "" {
					lastErr = apiErr(fmt.Errorf("CPSC provider error: %s", providerError))
				} else {
					return out, nil
				}
			}
		}
		if attempt < f.maxAttempts-1 {
			delay := time.Duration(attempt+1) * 500 * time.Millisecond
			if resp != nil && resp.StatusCode == http.StatusTooManyRequests {
				delay = cliutil.RetryAfter(resp)
			}
			if err := f.waitRetry(ctx, delay); err != nil {
				return nil, err
			}
		}
	}
	return nil, lastErr
}

func emitCPSCDryRun(cmd *cobra.Command, flags *rootFlags, operation string, plan map[string]any) error {
	plan["dry_run"] = true
	plan["operation"] = operation
	plan["network_requests_sent"] = 0
	return emitCPSC(cmd, flags, "dry-run", plan)
}
func cpscWindow(raw string) (time.Time, time.Time, error) {
	duration, err := cliutil.ParseDurationLoose(raw)
	if err != nil || duration < 24*time.Hour {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid --window %q; use at least 1d", raw)
	}
	end := time.Now().UTC()
	return end.Add(-duration), end, nil
}
func nestedNames(row map[string]any, field string) []string {
	raw, ok := row[field].([]any)
	if !ok {
		return []string{}
	}
	var out []string
	for _, item := range raw {
		if object, ok := item.(map[string]any); ok {
			for _, key := range []string{"Name", "Option", "URL"} {
				value := strings.TrimSpace(fmt.Sprint(object[key]))
				if value != "" && value != "<nil>" {
					out = append(out, value)
					break
				}
			}
		}
	}
	return out
}
func tokens(value string) map[string]bool {
	out := map[string]bool{}
	for _, token := range strings.FieldsFunc(strings.ToLower(value), func(r rune) bool { return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9') }) {
		if len(token) >= 3 {
			out[token] = true
		}
	}
	return out
}
func overlap(a, b string) ([]string, float64) {
	left, right := tokens(a), tokens(b)
	var shared []string
	for token := range left {
		if right[token] {
			shared = append(shared, token)
		}
	}
	sort.Strings(shared)
	denom := len(left)
	if len(right) < denom {
		denom = len(right)
	}
	if denom == 0 {
		return shared, 0
	}
	return shared, float64(len(shared)) / float64(denom)
}

func normalizedExact(left, right string) bool {
	return strings.EqualFold(strings.TrimSpace(left), strings.TrimSpace(right)) && strings.TrimSpace(left) != ""
}

func nestedFieldValues(row map[string]any, collection, field string) []string {
	raw, _ := row[collection].([]any)
	values := make([]string, 0, len(raw))
	for _, item := range raw {
		object, _ := item.(map[string]any)
		value := strings.TrimSpace(fmt.Sprint(object[field]))
		if value != "" && value != "<nil>" {
			values = append(values, value)
		}
	}
	return values
}

func exactInventoryEvidence(item map[string]string, recall map[string]any) map[string]any {
	candidateValues := map[string][]string{
		"name":  nestedFieldValues(recall, "Products", "Name"),
		"brand": nestedFieldValues(recall, "Manufacturers", "Name"),
		"model": nestedFieldValues(recall, "Products", "Model"),
		"upc":   nestedFieldValues(recall, "Products", "UPC"),
	}
	evidence := map[string]any{}
	for _, field := range []string{"name", "brand", "model", "upc"} {
		matches := []string{}
		for _, candidate := range candidateValues[field] {
			if normalizedExact(item[field], candidate) {
				matches = append(matches, candidate)
			}
		}
		evidence[field] = map[string]any{"input": item[field], "exact": len(matches) > 0, "matched_values": matches}
	}
	return evidence
}

func hasExactInventoryEvidence(evidence map[string]any) bool {
	for _, value := range evidence {
		field, _ := value.(map[string]any)
		if exact, _ := field["exact"].(bool); exact {
			return true
		}
	}
	return false
}

func canonicalMaterialRecall(row map[string]any) (json.RawMessage, error) {
	material := map[string]any{}
	for _, field := range []string{"RecallID", "RecallDate", "Title", "Description", "URL", "ConsumerContact", "Products", "Hazards", "Remedies", "RemedyOptions", "Injuries", "Incidents", "Manufacturers", "Retailers"} {
		if value, ok := row[field]; ok {
			material[field] = canonicalJSONValue(value)
		}
	}
	return json.Marshal(material)
}

func canonicalJSONValue(value any) any {
	switch typed := value.(type) {
	case []any:
		items := make([]any, len(typed))
		for i, item := range typed {
			items[i] = canonicalJSONValue(item)
		}
		sort.SliceStable(items, func(i, j int) bool {
			left, _ := json.Marshal(items[i])
			right, _ := json.Marshal(items[j])
			return string(left) < string(right)
		})
		return items
	case map[string]any:
		object := make(map[string]any, len(typed))
		for key, item := range typed {
			object[key] = canonicalJSONValue(item)
		}
		return object
	default:
		return value
	}
}
func emitCPSC(cmd *cobra.Command, flags *rootFlags, source string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return printOutputWithFlagsMeta(cmd.OutOrStdout(), raw, flags, map[string]any{"source": source, "provider": "US CPSC SaferProducts", "retrieved_at": time.Now().UTC().Format(time.RFC3339)})
}
func cpscCaveats() []string {
	return []string{"Text overlap produces candidate matches only; verify model, UPC, description, and the official CPSC source before acting.", "Recall counts are not incident or injury rates because sales and exposure denominators are not supplied.", "Always follow the remedy and contact instructions on the official recall record."}
}
