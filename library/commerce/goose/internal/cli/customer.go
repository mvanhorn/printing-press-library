package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/goose/internal/auth"
	"github.com/mvanhorn/printing-press-library/library/commerce/goose/internal/config"

	"github.com/spf13/cobra"
)

// newCustomerCmd implements `customer <query>` — one-shot search → detail.
// Hits search-api.goose.pet for fuzzy match, then api.goose.pet for the
// full customer record (pets, vouchers, agreements, payments, notes,
// balance, recent bookings).
func newCustomerCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:         "customer [query]",
		Short:       "One-shot search→detail: take a name/phone/email, return the full customer record",
		Example:     "  goose-pp-cli customer 'Pat Smith'\n  goose-pp-cli customer smith --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			query := strings.Join(args, " ")
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			if err := auth.RefreshIfNeeded(cfg, 60*time.Second); err != nil {
				return err
			}
			facility := cfg.TemplateVars["facility"]
			if facility == "" {
				return fmt.Errorf("no facility configured; run `goose-pp-cli auth login` or set GOOSE_FACILITY")
			}
			authHeader := cfg.AuthHeader()

			// Search.
			hits, err := searchCustomers(authHeader, facility, query, limit)
			if err != nil {
				return fmt.Errorf("searching customers: %w", err)
			}
			if len(hits) == 0 {
				return fmt.Errorf("no customers matching %q", query)
			}

			// Resolve detail for the top hit (or all when --json + --limit > 1).
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			out := []map[string]any{}
			for _, h := range hits {
				id, _ := h["id"].(string)
				if id == "" {
					continue
				}
				params := map[string]string{
					"includes": strings.Join([]string{
						"locationPetProfiles",
						"tagRelations.tag",
						"agreements",
						"vouchers",
						"locationUserProfileMemberships",
						"associatedContacts",
					}, ","),
				}
				detail, derr := c.Get("/location-user-profiles/"+id, params)
				if derr != nil {
					return fmt.Errorf("fetching customer %s: %w", id, derr)
				}
				var m map[string]any
				if err := json.Unmarshal(detail, &m); err != nil {
					return fmt.Errorf("parsing customer %s: %w", id, err)
				}
				out = append(out, m)
			}

			if flags.asJSON || flags.compact || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			// Pretty-print top result.
			top := out[0]
			renderCustomerHuman(cmd.OutOrStdout(), top)
			if len(out) > 1 {
				fmt.Fprintf(cmd.OutOrStdout(), "\n(%d additional matches — re-run with --json --limit %d to see all)\n", len(out)-1, len(out))
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 1, "Max matches to resolve in detail")
	return cmd
}

func renderCustomerHuman(w io.Writer, c map[string]any) {
	fmt.Fprintf(w, "%s %s — %s — %s\n",
		strOrDash(c["firstName"]), strOrDash(c["lastName"]),
		strOrDash(c["email"]), strOrDash(c["phone"]))
	if pets, ok := c["locationPetProfiles"].([]any); ok && len(pets) > 0 {
		fmt.Fprintln(w, "Pets:")
		for _, p := range pets {
			pm, _ := p.(map[string]any)
			fmt.Fprintf(w, "  - %s\n", strOrDash(pm["displayName"]))
		}
	}
	if vchs, ok := c["vouchers"].([]any); ok && len(vchs) > 0 {
		fmt.Fprintf(w, "Vouchers: %d\n", len(vchs))
	}
	if ags, ok := c["agreements"].([]any); ok {
		fmt.Fprintf(w, "Agreements: %d\n", len(ags))
	}
}

func strOrDash(v any) string {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return "—"
}

// searchCustomers calls search-api.goose.pet, a different host than the main
// admin API. Returns a slice of result maps (id, displayName, etc.).
func searchCustomers(authHeader, facility, query string, size int) ([]map[string]any, error) {
	return searchAPI(authHeader, facility, "location-user-profile", query, size, "petProfiles")
}

// searchPets reuses the search-api but with a different resource path.
// (Same host, parameterised by resource segment.)
func searchPets(authHeader, facility, query string, size int) ([]map[string]any, error) {
	// search-api indexes pets via the include flag on the customer search.
	// The endpoint catalog confirmed: /location-user-profile/search?include=petProfiles
	// returns customer hits with pet details nested. We re-rank for pet match here.
	return searchAPI(authHeader, facility, "location-user-profile", query, size, "petProfiles")
}

func searchAPI(authHeader, facility, resource, query string, size int, include string) ([]map[string]any, error) {
	if size <= 0 {
		size = 15
	}
	u := &url.URL{
		Scheme: "https",
		Host:   "search-api.goose.pet",
		Path:   "/api/v1/admin/account/" + facility + "/" + resource + "/search",
	}
	q := u.Query()
	q.Set("query", query)
	q.Set("location", facility)
	if include != "" {
		q.Set("include", include)
	}
	q.Set("from", "0")
	q.Set("size", fmt.Sprintf("%d", size))
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	if authHeader != "" {
		req.Header.Set("Authorization", authHeader)
	}
	req.Header.Set("Origin", "https://app.goose.pet")
	req.Header.Set("Referer", "https://app.goose.pet/")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("search-api returned %d: %s", resp.StatusCode, snippetN(body, 200))
	}
	// search-api response shape (observed from live capture):
	// { "hits": { "hits": [ { "_source": {...}, ...} ] } }  (Elasticsearch passthrough)
	// or { "results": [...] } depending on the resource.
	var es struct {
		Hits struct {
			Hits []struct {
				Source map[string]any `json:"_source"`
				ID     string         `json:"_id"`
			} `json:"hits"`
		} `json:"hits"`
		Results []map[string]any `json:"results"`
	}
	if err := json.Unmarshal(body, &es); err != nil {
		// Some responses are a bare array.
		var arr []map[string]any
		if jerr := json.Unmarshal(body, &arr); jerr == nil {
			return arr, nil
		}
		return nil, fmt.Errorf("parsing search response: %w", err)
	}
	if len(es.Results) > 0 {
		return es.Results, nil
	}
	out := make([]map[string]any, 0, len(es.Hits.Hits))
	for _, h := range es.Hits.Hits {
		src := h.Source
		if src == nil {
			src = map[string]any{}
		}
		if _, hasID := src["id"]; !hasID && h.ID != "" {
			src["id"] = h.ID
		}
		out = append(out, src)
	}
	return out, nil
}

func snippetN(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n]) + "…"
	}
	return string(b)
}
