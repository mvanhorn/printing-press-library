// keyword-magic: SEMrush Keyword Magic Tool with Personal Keyword Difficulty.
// Uses the UI-mode transport (cookies + JSON-RPC to /kmtgw/v2/webapi) because
// PKD is not exposed by the public API. Requires 'auth login --chrome' first.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

const (
	kmtgwURL    = "https://www.semrush.com/kmtgw/v2/webapi"
	kmtgwOrigin = "https://www.semrush.com"
)

// SEMrush KMT internal mode codes (decoded from HAR capture).
var kmtModes = map[string]int{
	"broad":   0,
	"related": 3,
	"phrase":  4,
	// "exact" was not captured in our HAR; can be added when discovered.
}

type kmtRPCRequest struct {
	ID      int    `json:"id"`
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params"`
}

type kmtRPCResponse struct {
	ID      int             `json:"id"`
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *kmtRPCError    `json:"error,omitempty"`
}

type kmtRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type kmtParams struct {
	Mode          int       `json:"mode"`
	Currency      string    `json:"currency"`
	Database      string    `json:"database"`
	Filter        kmtFilter `json:"filter"`
	Groups        []any     `json:"groups"`
	Order         kmtOrder  `json:"order"`
	GroupsOrder   kmtOrder  `json:"groups_order"`
	Phrase        string    `json:"phrase"`
	QuestionsOnly bool      `json:"questions_only"`
	Domain        string    `json:"domain,omitempty"`
	Page          kmtPage   `json:"page"`
	// GroupsPage is set on ideas.GetGroups requests (the KMT "Groups" tab —
	// auto-clustered topic themes). Omitted from ideas.GetKeywords payloads.
	GroupsPage *kmtPage `json:"groups_page,omitempty"`
}

type kmtFilter struct {
	Phrase             []any                 `json:"phrase"`
	CompetitionLevel   []any                 `json:"competition_level"`
	CPC                []any                 `json:"cpc"`
	Difficulty         []any                 `json:"difficulty"`
	Results            []any                 `json:"results"`
	SERPFeatures       []kmtSERPFeatureGroup `json:"serp_features"`
	Volume             []any                 `json:"volume"`
	WordsCount         []any                 `json:"words_count"`
	PhraseIncludeLogic int                   `json:"phrase_include_logic"`
	DomainHidePosition bool                  `json:"domain_hide_position"`
	DomainDifficulty   []any                 `json:"domain_difficulty"`
}

type kmtSERPFeatureGroup struct {
	Inverted bool  `json:"inverted"`
	Value    []any `json:"value"`
}

type kmtOrder struct {
	Direction int    `json:"direction"`
	Field     string `json:"field"`
}

type kmtPage struct {
	Number int `json:"number"`
	Size   int `json:"size"`
}

func newKeywordMagicCmd(_ *rootFlags) *cobra.Command {
	var (
		domain          string
		database        string
		currency        string
		mode            string
		limit           int
		page            int
		questionsOnly   bool
		orderField      string
		minVolume       int
		maxKD           int
		excludeBrand    bool
		excludeList     string
	)
	cmd := &cobra.Command{
		Use:     "keyword-magic <seed>",
		Aliases: []string{"kmt"},
		Short:   "Keyword Magic Tool with Personal Keyword Difficulty (UI mode — requires 'auth login --chrome')",
		Long: "Runs the SEMrush Keyword Magic Tool against your logged-in session. " +
			"When --domain is set, returns Personal Keyword Difficulty (PKD) scored " +
			"against that domain — PKD is not exposed by the public API and is the " +
			"primary reason to use this command instead of 'keyword related'.\n\n" +
			"Modes: broad (all variants), phrase (containing seed), related " +
			"(semantically related), questions (question-form). Default: broad.",
		Example: strings.Trim(`
  # Broad match for "seo audit", scored against your client
  semrush-pp-cli keyword-magic "seo audit" --domain client.com --database us

  # Related keywords (semantic variants), top 50
  semrush-pp-cli keyword-magic "tiles" --domain nationaltiles.com.au --database au --mode related --limit 50

  # Questions only, with sweet-spot filters
  semrush-pp-cli keyword-magic "content marketing" --domain client.com --mode questions --min-volume 100 --max-pkd 30
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			seed := args[0]

			modeCode, ok := kmtModes[strings.ToLower(mode)]
			if !ok {
				return fmt.Errorf("--mode must be one of: broad, phrase, related, questions (got %q)", mode)
			}
			if strings.ToLower(mode) == "questions" {
				questionsOnly = true
				modeCode = kmtModes["phrase"] // questions are mode=4 + questions_only=true
			}
			if limit <= 0 || limit > 100 {
				limit = 100
			}
			if page <= 0 {
				page = 1
			}
			if orderField == "" {
				if modeCode == 3 {
					orderField = "relation_level"
				} else {
					orderField = "volume"
				}
			}

			params := kmtParams{
				Mode:     modeCode,
				Currency: strings.ToUpper(currency),
				Database: database,
				Filter: kmtFilter{
					Phrase:           []any{},
					CompetitionLevel: []any{},
					CPC:              []any{},
					Difficulty:       []any{},
					Results:          []any{},
					SERPFeatures: []kmtSERPFeatureGroup{
						{Inverted: false, Value: []any{}},
					},
					Volume:           []any{},
					WordsCount:       []any{},
					DomainDifficulty: []any{},
				},
				Groups:        []any{},
				Order:         kmtOrder{Direction: 1, Field: orderField},
				GroupsOrder:   kmtOrder{Direction: 1, Field: "count"},
				Phrase:        seed,
				QuestionsOnly: questionsOnly,
				Domain:        domain,
				Page:          kmtPage{Number: page, Size: limit},
			}

			result, err := callKMTGateway(cmd.Context(), "ideas.GetKeywords", params, 30*time.Second)
			if err != nil {
				return err
			}

			// Extract the keyword row list — kmtgw returns either a top-level
			// JSON array or an object wrapping a "list"/"keywords"/"items" array.
			rows, putBack := extractKMTRowsAndBackref(result)
			before := len(rows)

			// Value filters (volume/KD)
			rows = applyResearchFilters(rows, minVolume, maxKD, 0)
			afterValue := len(rows)

			// Derive human-readable intent label per row + apply the
			// not-ranking sentinel rule for Position/Est Traffic.
			for _, r := range rows {
				r["_type"] = IntentCodeToString(r["intents"])
				deriveDisplayFields(r)
			}

			// Brand exclusion — self-brand from --domain plus explicit --exclude.
			bf := newBrandFilter(domain, excludeBrand, excludeList)
			var brandDropped int
			rows, brandDropped = applyBrandFilter(rows, bf)
			afterBrand := len(rows)

			// Report what happened on stderr (doesn't pollute JSON output).
			if before != afterBrand {
				fmt.Fprintf(cmd.ErrOrStderr(), "keyword-magic: %d → %d (value-filtered: -%d, brand-filtered: -%d)\n",
					before, afterBrand, before-afterValue, brandDropped)
				if len(bf.terms) > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "  excluded terms: %s\n", strings.Join(bf.terms, ", "))
				}
			}

			// Write back to the original shape (preserves wrapper keys if any).
			final := putBack(rows)
			out, _ := json.MarshalIndent(final, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}
	cmd.Flags().StringVar(&domain, "domain", "", "Target domain for Personal Keyword Difficulty (PKD) scoring")
	cmd.Flags().StringVar(&database, "database", "us", "Country database code (us, au, uk, ca, etc.)")
	cmd.Flags().StringVar(&currency, "currency", "USD", "Currency code for CPC values (USD, AUD, GBP, EUR, etc.)")
	cmd.Flags().StringVar(&mode, "mode", "broad", "Match mode: broad, phrase, related, questions")
	cmd.Flags().IntVar(&limit, "limit", 100, "Max keywords to return (1-100 per page)")
	cmd.Flags().IntVar(&page, "page", 1, "Page number (use with --limit for pagination)")
	cmd.Flags().StringVar(&orderField, "order", "", "Sort field (volume, cpc, difficulty, relation_level)")
	cmd.Flags().IntVar(&minVolume, "min-volume", 0, "Filter: minimum search volume")
	cmd.Flags().IntVar(&maxKD, "max-kd", 0, "Filter: maximum Keyword Difficulty (KD%) — 0 to disable")
	cmd.Flags().BoolVar(&excludeBrand, "exclude-self-brand", true, "Drop keywords containing your --domain's brand (e.g. 'nationaltiles' or 'national tiles' for nationaltiles.com.au). Disable with --exclude-self-brand=false.")
	cmd.Flags().StringVar(&excludeList, "exclude", "", "Comma-separated competitor brand terms to exclude (e.g. 'beaumont,bunnings,anaconda'). Case-insensitive whole-word match against the phrase.")
	return cmd
}

// newKMTGroupsCmd exposes KMT's auto-clustered topic groups for a seed. Same
// transport as keyword-magic (cookies + JSON-RPC) but calls `ideas.GetGroups`
// instead of `ideas.GetKeywords`. Useful for populating Topic / Category
// columns in keyword research deliverables without manual clustering.
func newKMTGroupsCmd(_ *rootFlags) *cobra.Command {
	var (
		domain   string
		database string
		currency string
		limit    int
	)
	cmd := &cobra.Command{
		Use:     "kmt-groups <seed>",
		Aliases: []string{"keyword-groups", "topic-clusters"},
		Short:   "KMT auto-clustered topic groups for a seed (the 'Groups' tab in KMT UI)",
		Long: "Returns SEMrush's auto-generated topic clusters for a seed keyword — " +
			"e.g. for 'tiles': 'bathroom tiles', 'kitchen tiles', 'outdoor tiles', " +
			"'tile installation'. Each group has a keyword count and total volume.\n\n" +
			"Use the output to seed Topic / Category columns in deliverables, or to " +
			"identify pillar/cluster taxonomy for a content strategy.\n\n" +
			"Requires UI mode auth (run 'auth login --chrome' first).",
		Example: strings.Trim(`
  # Topic clusters for a seed (no domain scoring)
  semrush-pp-cli kmt-groups "pipes" --database au

  # With domain context (filters/orders groups by domain relevance)
  semrush-pp-cli kmt-groups "tiles" --domain nationaltiles.com.au --database au --limit 30
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			seed := args[0]
			if limit <= 0 || limit > 100 {
				limit = 50
			}
			gp := kmtPage{Number: 1, Size: limit}
			params := kmtParams{
				Mode:     0,
				Currency: strings.ToUpper(currency),
				Database: database,
				Filter: kmtFilter{
					Phrase:           []any{},
					CompetitionLevel: []any{},
					CPC:              []any{},
					Difficulty:       []any{},
					Results:          []any{},
					SERPFeatures:     []kmtSERPFeatureGroup{{Inverted: false, Value: []any{}}},
					Volume:           []any{},
					WordsCount:       []any{},
					DomainDifficulty: []any{},
				},
				Groups:      []any{},
				Order:       kmtOrder{Direction: 1, Field: "volume"},
				GroupsOrder: kmtOrder{Direction: 1, Field: "count"},
				Phrase:      seed,
				Domain:      domain,
				Page:        kmtPage{Number: 1, Size: 100}, // required by API even though unused for groups
				GroupsPage:  &gp,
			}
			result, err := callKMTGateway(cmd.Context(), "ideas.GetGroups", params, 30*time.Second)
			if err != nil {
				return err
			}
			out, _ := json.MarshalIndent(result, "", "  ")
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}
	cmd.Flags().StringVar(&domain, "domain", "", "Optional: domain for relevance-aware group ordering")
	cmd.Flags().StringVar(&database, "database", "us", "Country database (us, au, uk, etc.)")
	cmd.Flags().StringVar(&currency, "currency", "USD", "Currency code")
	cmd.Flags().IntVar(&limit, "limit", 50, "Max groups to return (1-100)")
	return cmd
}

// callKMTGateway POSTs a JSON-RPC 2.0 request to /kmtgw/v2/webapi with the
// imported Chrome session cookies. Returns the unmarshaled result.
func callKMTGateway(ctx context.Context, method string, params any, timeout time.Duration) (any, error) {
	jar, err := loadSemrushCookieJar()
	if err != nil {
		return nil, err
	}
	req := kmtRPCRequest{ID: 1, JSONRPC: "2.0", Method: method, Params: params}
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", kmtgwURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json, text/plain, */*")
	httpReq.Header.Set("Origin", kmtgwOrigin)
	// A Referer that looks like the user came from the KMT UI keeps the
	// session-validation logic happy on some endpoints.
	refURL := url.URL{Scheme: "https", Host: "www.semrush.com", Path: "/analytics/keywordmagic/"}
	httpReq.Header.Set("Referer", refURL.String())
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36")
	jar.applyCookiesToRequest(httpReq)

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("POST kmtgw: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		hint := ""
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			hint = " — cookies may be expired; re-run 'semrush-pp-cli auth login --chrome'"
		}
		return nil, fmt.Errorf("kmtgw HTTP %d%s: %s", resp.StatusCode, hint, truncateForErr(respBody, 300))
	}

	var rpc kmtRPCResponse
	if err := json.Unmarshal(respBody, &rpc); err != nil {
		return nil, fmt.Errorf("decoding kmtgw response: %w (body: %s)", err, truncateForErr(respBody, 300))
	}
	if rpc.Error != nil {
		return nil, fmt.Errorf("kmtgw RPC error %d: %s", rpc.Error.Code, rpc.Error.Message)
	}
	var result any
	if err := json.Unmarshal(rpc.Result, &result); err != nil {
		// Return raw bytes as string if not decodable
		return string(rpc.Result), nil
	}
	return result, nil
}

// extractKMTRowsAndBackref returns the keyword row list along with a "back-
// reference" function that re-inserts filtered rows into the original shape.
// kmtgw's `ideas.GetKeywords` result can be either:
//   - a top-level JSON array of keyword objects, or
//   - an object with the array under a "list"/"keywords"/"items" key (sometimes
//     nested under "data")
// Both shapes are observed in practice; this helper hides the difference.
func extractKMTRowsAndBackref(result any) ([]map[string]any, func([]map[string]any) any) {
	// Case 1: top-level array
	if arr, ok := result.([]any); ok {
		rows := castRowSlice(arr)
		return rows, func(filtered []map[string]any) any {
			out := make([]any, len(filtered))
			for i, r := range filtered {
				out[i] = r
			}
			return out
		}
	}
	// Case 2: object with array under a known key
	if obj, ok := result.(map[string]any); ok {
		for _, key := range []string{"list", "keywords", "items"} {
			if v, ok := obj[key]; ok {
				if arr, ok := v.([]any); ok {
					rows := castRowSlice(arr)
					return rows, func(filtered []map[string]any) any {
						out := make([]any, len(filtered))
						for i, r := range filtered {
							out[i] = r
						}
						obj[key] = out
						return obj
					}
				}
			}
		}
		// Case 3: nested under "data"
		if d, ok := obj["data"].(map[string]any); ok {
			for _, key := range []string{"list", "keywords", "items"} {
				if v, ok := d[key]; ok {
					if arr, ok := v.([]any); ok {
						rows := castRowSlice(arr)
						return rows, func(filtered []map[string]any) any {
							out := make([]any, len(filtered))
							for i, r := range filtered {
								out[i] = r
							}
							d[key] = out
							return obj
						}
					}
				}
			}
		}
	}
	// Unknown shape — return empty and a no-op back-ref
	return nil, func(_ []map[string]any) any { return result }
}

// filterKMTKeywords applies client-side filters to the keyword list inside the
// response. Only operates on the standard SEMrush KMT response shape (object
// with a "list" or "keywords" array of keyword records); returns unchanged for
// unrecognized shapes.
func filterKMTKeywords(result any, minVolume, maxKD, maxPKD int) any {
	if minVolume == 0 && maxKD == 0 && maxPKD == 0 {
		return result
	}
	obj, ok := result.(map[string]any)
	if !ok {
		return result
	}
	// SEMrush KMT response wraps the keyword array under "data.keywords" or
	// just "keywords" depending on the method. Try both.
	var listKey string
	var arr []any
	for _, k := range []string{"list", "keywords", "items"} {
		if v, ok := obj[k]; ok {
			if a, ok := v.([]any); ok {
				listKey = k
				arr = a
				break
			}
		}
	}
	if listKey == "" {
		// Look one level deeper under "data"
		if d, ok := obj["data"].(map[string]any); ok {
			for _, k := range []string{"list", "keywords", "items"} {
				if v, ok := d[k]; ok {
					if a, ok := v.([]any); ok {
						listKey = "data." + k
						arr = a
						_ = d // mutate in-place below if found here
						break
					}
				}
			}
		}
	}
	if arr == nil {
		return result
	}

	filtered := make([]any, 0, len(arr))
	for _, item := range arr {
		row, ok := item.(map[string]any)
		if !ok {
			filtered = append(filtered, item)
			continue
		}
		if minVolume > 0 {
			if v := keywordIntField(row, "volume", "search_volume", "Volume"); v < minVolume {
				continue
			}
		}
		if maxKD > 0 {
			if v := keywordIntField(row, "difficulty", "kd", "keyword_difficulty"); v > maxKD {
				continue
			}
		}
		if maxPKD > 0 {
			if v := keywordIntField(row, "domain_difficulty", "personal_keyword_difficulty", "pkd"); v > maxPKD {
				continue
			}
		}
		filtered = append(filtered, row)
	}

	if strings.HasPrefix(listKey, "data.") {
		if d, ok := obj["data"].(map[string]any); ok {
			d[strings.TrimPrefix(listKey, "data.")] = filtered
		}
	} else {
		obj[listKey] = filtered
	}
	return obj
}

func keywordIntField(row map[string]any, names ...string) int {
	for _, n := range names {
		if v, ok := row[n]; ok {
			switch t := v.(type) {
			case float64:
				return int(t)
			case int:
				return t
			case string:
				if i, err := strconv.Atoi(t); err == nil {
					return i
				}
				if f, err := strconv.ParseFloat(t, 64); err == nil {
					return int(f)
				}
			}
		}
	}
	return 0
}

func truncateForErr(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
