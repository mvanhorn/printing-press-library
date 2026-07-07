// Novel command: Lowe's autocomplete and store locator via standard HTTP.
//
// Lowe's API surface was assessed using the Printing Press printer protocol:
//   - probe-reachability classified autocomplete and store locator as standard_http (0.95 confidence)
//   - browser-sniff on a Firefox HAR captured 14 pythia-recs-svc endpoints (cookie auth, browser_clearance)
//   - Product search is SSR (server-rendered HTML with __PRELOADED_STATE__); no separate API endpoint
//   - The recommendation engine (pythia-recs-svc) requires browser session cookies; stubbed for future work
//
// Active endpoints (no auth, User-Agent header only):
//   - GET /LowesSearchServices/resources/autocomplete/v2_0 — search term suggestions with category facets
//   - GET /store/api/search — store locator by ZIP with hours, coordinates, features
//
// Home Depot was assessed via the same protocol:
//   - probe-reachability: browser_clearance_http on all endpoints (homepage, autocomplete, store finder, GraphQL)
//   - browser-sniff on Firefox HAR: zero requests to www.homedepot.com captured; only Sprinklr chat widget
//   - Home Depot renders everything server-side; there is no XHR/fetch API surface for product or store data
//   - Remains a stub with no viable path to activation without an HTML scraping adapter

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/spf13/cobra"
)

const (
	lowesBaseURL      = "https://www.lowes.com"
	lowesAutocomplete = "/LowesSearchServices/resources/autocomplete/v2_0"
	lowesStoreFinder  = "/store/api/search"
	lowesUserAgent    = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/148.0.0.0 Safari/537.36"
)

// lowesHTTP executes a GET request against Lowe's with the required User-Agent header.
func lowesHTTP(ctx context.Context, path string, params url.Values, timeout time.Duration) ([]byte, error) {
	u, err := url.Parse(lowesBaseURL + path)
	if err != nil {
		return nil, fmt.Errorf("parsing lowes URL: %w", err)
	}
	u.RawQuery = params.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating lowes request: %w", err)
	}
	req.Header.Set("User-Agent", lowesUserAgent)
	req.Header.Set("Accept", "application/json, text/plain, */*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")

	client := &http.Client{Timeout: timeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("lowes request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading lowes response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("lowes returned %d: %s", resp.StatusCode, truncateBody(body))
	}

	return body, nil
}

// --- Autocomplete (suggest) ---

// lowesAutocompleteResult is the top-level response from the autocomplete API.
type lowesAutocompleteResult struct {
	Terms []lowesSuggestion `json:"terms"`
}

type lowesSuggestion struct {
	Name       string          `json:"name"`
	URL        string          `json:"url"`
	Redirect   string          `json:"redirect,omitempty"`
	Source     int             `json:"source,omitempty"`
	Categories []lowesCategory `json:"categories,omitempty"`
}

type lowesCategory struct {
	Category string `json:"category"`
	URL      string `json:"url"`
}

func newSuggestLowesSuggestCmd(flags *rootFlags) *cobra.Command {
	var maxTerms int

	cmd := &cobra.Command{
		Use:     "lowes-suggest <query>",
		Short:   "Autocomplete suggestions from Lowe's. Returns search term completions with optional category facets.",
		Example: "  reno-goat-pp-cli suggest lowes-suggest faucet",
		Annotations: map[string]string{
			"pp:endpoint":   "suggest.lowes-suggest",
			"pp:method":     "GET",
			"pp:path":       lowesAutocomplete,
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			params := url.Values{}
			params.Set("searchTerm", args[0])
			params.Set("maxTerms", fmt.Sprintf("%d", maxTerms))
			params.Set("visitorStatus", "Guest")
			params.Set("state", "null")

			data, err := lowesHTTP(cmd.Context(), lowesAutocomplete, params, flags.timeout)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Parse to normalize output.
			var result lowesAutocompleteResult
			if err := json.Unmarshal(data, &result); err != nil {
				return fmt.Errorf("parsing lowes autocomplete response: %w", err)
			}

			// Re-marshal the terms array as the output.
			out, err := json.MarshalIndent(result.Terms, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling lowes suggestions: %w", err)
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				filtered := out
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				return printOutput(cmd.OutOrStdout(), filtered, true)
			}

			// Human-readable table.
			var items []map[string]any
			if json.Unmarshal(out, &items) == nil && len(items) > 0 {
				return printAutoTable(cmd.OutOrStdout(), items)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&maxTerms, "max-terms", 8, "Maximum number of suggestions to return.")
	return cmd
}

// --- Store Locator ---

// lowesStoreResult is the top-level response from the store API.
type lowesStoreResult struct {
	Stores []lowesStoreWrapper `json:"stores"`
}

type lowesStoreWrapper struct {
	Store lowesStore `json:"store"`
}

type lowesStore struct {
	ID               string           `json:"id"`
	StoreName        string           `json:"store_name"`
	BisName          string           `json:"bis_name"`
	Address          string           `json:"address"`
	City             string           `json:"city"`
	State            string           `json:"state"`
	ZIP              string           `json:"zip"`
	Phone            string           `json:"phone"`
	Lat              string           `json:"lat"`
	Long             string           `json:"long"`
	TimeZone         string           `json:"timeZone"`
	StoreFeature     string           `json:"storeFeature"`
	OpenDate         string           `json:"openDate"`
	StoreDescription string           `json:"storeDescription"`
	StoreHours       []lowesStoreHour `json:"storeHours"`
}

type lowesStoreHour struct {
	Day lowesDay `json:"day"`
}

type lowesDay struct {
	Day   string `json:"day"`
	Open  string `json:"open"`
	Close string `json:"close"`
}

func newStoresLowesStoresCmd(flags *rootFlags) *cobra.Command {
	var maxResults int

	cmd := &cobra.Command{
		Use:     "lowes-stores <zip>",
		Short:   "Find Lowe's stores near a ZIP code. Returns address, hours, coordinates, phone, and features.",
		Example: "  reno-goat-pp-cli stores lowes-stores 66101",
		Annotations: map[string]string{
			"pp:endpoint":   "stores.lowes-stores",
			"pp:method":     "GET",
			"pp:path":       lowesStoreFinder,
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			params := url.Values{}
			params.Set("searchTerm", args[0])
			params.Set("maxResults", fmt.Sprintf("%d", maxResults))
			params.Set("responseGroup", "large")

			data, err := lowesHTTP(cmd.Context(), lowesStoreFinder, params, flags.timeout)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			// Parse and flatten the nested store wrapper.
			var result lowesStoreResult
			if err := json.Unmarshal(data, &result); err != nil {
				return fmt.Errorf("parsing lowes store response: %w", err)
			}

			// Flatten: extract the inner store objects for cleaner output.
			stores := make([]lowesStore, len(result.Stores))
			for i, w := range result.Stores {
				stores[i] = w.Store
			}

			out, err := json.MarshalIndent(stores, "", "  ")
			if err != nil {
				return fmt.Errorf("marshaling lowes stores: %w", err)
			}

			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				filtered := out
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				return printOutput(cmd.OutOrStdout(), filtered, true)
			}

			// Human table: show key fields.
			if len(stores) > 0 {
				headers := []string{"ID", "NAME", "ADDRESS", "CITY", "STATE", "ZIP", "PHONE", "FEATURES"}
				rows := make([][]string, len(stores))
				for i, s := range stores {
					rows[i] = []string{
						s.ID,
						s.StoreName,
						s.Address,
						s.City,
						s.State,
						s.ZIP,
						s.Phone,
						s.StoreFeature,
					}
				}
				return flags.printTable(cmd, headers, rows)
			}

			fmt.Fprintf(os.Stderr, "No Lowe's stores found near %s\n", args[0])
			return nil
		},
	}
	cmd.Flags().IntVar(&maxResults, "max-results", 5, "Maximum number of stores to return.")
	return cmd
}
