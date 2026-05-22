// Hand-authored: novel feature `person funnel`.
//
// Compose /search-person across pages with /enrich-person on every hit and
// upsert each enriched person into outreach.people via the user-authored
// mapping. Optionally writes results to CSV.

package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/prospeo/internal/client"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/prospeo/internal/supa"
)

// FunnelResult is the summary returned by `person funnel`.
type FunnelResult struct {
	TotalSearched         int    `json:"total_searched"`
	TotalEnriched         int    `json:"total_enriched"`
	TotalCreditsEstimated int    `json:"total_credits_estimated"`
	TotalUpserted         int    `json:"total_upserted"`
	OutputPath            string `json:"output_path,omitempty"`
}

func newPersonFunnelCmd(flags *rootFlags) *cobra.Command {
	var filtersPath, output string
	var max, pageStart int
	var verifiedOnly bool

	cmd := &cobra.Command{
		Use:   "funnel",
		Short: "Compose /search-person + /enrich-person across pages, upsert to outreach.people, optional CSV export.",
		Long: `Reads a JSON file of search-person filters, walks pages starting at
--page-start, and calls /enrich-person on every hit until --max is reached.
Each enriched person is upserted into outreach.people using the mapping at
~/.config/prospeo-pp-cli/outreach-mapping.md (run 'prospeo-pp-cli outreach map'
once to create it). If Supabase or the mapping is not configured, the upsert
step is skipped silently and results are still returned.`,
		Example:     "  prospeo-pp-cli person funnel --filters filters.json --max 50 --output leads.csv",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if filtersPath == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would run person funnel with filters=%s max=%d page-start=%d\n", filtersPath, max, pageStart)
				return nil
			}
			filtersRaw, err := os.ReadFile(filtersPath)
			if err != nil {
				return fmt.Errorf("reading filters file: %w", err)
			}
			var filters map[string]any
			if err := json.Unmarshal(filtersRaw, &filters); err != nil {
				return fmt.Errorf("parsing filters JSON: %w", err)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			var sc *supa.Client
			if supa.IsConfigured() {
				cfg, lerr := supa.LoadConfig()
				if lerr == nil {
					sc = supa.New(cfg)
				}
			}
			var mapping *OutreachMapping
			if sc != nil {
				m, merr := LoadOutreachMapping()
				if merr == nil {
					mapping = m
				} else if !errors.Is(merr, ErrMappingNotFound) {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not load outreach mapping: %v\n", merr)
				}
			}
			result, err := runPersonFunnel(cmd.Context(), c, sc, mapping, filters, max, pageStart, verifiedOnly, output)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&filtersPath, "filters", "", "Path to JSON file containing the search-person filters object.")
	cmd.Flags().IntVar(&max, "max", 25, "Maximum number of enriched results to collect.")
	cmd.Flags().IntVar(&pageStart, "page-start", 1, "Page to start walking search-person from.")
	cmd.Flags().BoolVar(&verifiedOnly, "verified-only", false, "Pass only_verified_email=true to /enrich-person.")
	cmd.Flags().StringVar(&output, "output", "", "Optional CSV output path; if set, enriched rows are written here.")
	return cmd
}

func runPersonFunnel(ctx context.Context, c *client.Client, sc *supa.Client, mapping *OutreachMapping, filters map[string]any, max, pageStart int, verifiedOnly bool, output string) (*FunnelResult, error) {
	if max <= 0 {
		max = 25
	}
	if pageStart <= 0 {
		pageStart = 1
	}
	result := &FunnelResult{OutputPath: output}

	// Walk search-person pages.
	var hits []map[string]any
	for page := pageStart; len(hits) < max; page++ {
		body := map[string]any{"filters": filters, "page": page}
		raw, _, err := c.PostQueryWithParams(ctx, "/search-person", nil, body)
		if err != nil {
			return result, fmt.Errorf("search-person page %d: %w", page, err)
		}
		batch := extractPersonBatch(raw)
		if len(batch) == 0 {
			break
		}
		for _, h := range batch {
			if len(hits) >= max {
				break
			}
			hits = append(hits, h)
		}
		if len(batch) < 25 { // Prospeo standard page size
			break
		}
	}
	result.TotalSearched = len(hits)

	// Enrich each hit.
	var enriched []map[string]any
	for _, h := range hits {
		personID, _ := h["id"].(string)
		if personID == "" {
			personID, _ = h["person_id"].(string)
		}
		if personID == "" {
			continue
		}
		body := map[string]any{"person_id": personID}
		if verifiedOnly {
			body["only_verified_email"] = true
		}
		raw, _, err := c.PostWithParams(ctx, "/enrich-person", nil, body)
		if err != nil {
			continue
		}
		creditsSpent, _ := extractCreditsInfo(raw)
		result.TotalCreditsEstimated += creditsSpent

		var person map[string]any
		_ = json.Unmarshal(raw, &person)
		enriched = append(enriched, person)

		// Upsert to outreach.people if mapping + supa available.
		if sc != nil && mapping != nil {
			row, key, mapErr := ApplyPersonMapping(mapping, person)
			if mapErr == nil && len(row) > 0 && key != "" {
				if _, uerr := sc.Upsert(ctx, mapping.People.Table, []map[string]any{row}); uerr == nil {
					result.TotalUpserted++
				}
			}
		}
	}
	result.TotalEnriched = len(enriched)

	if output != "" {
		if err := writeFunnelCSV(output, enriched); err != nil {
			return result, fmt.Errorf("writing CSV: %w", err)
		}
	}
	return result, nil
}

// extractPersonBatch normalizes the various wrapper shapes /search-person
// may return into a flat slice of hit objects.
func extractPersonBatch(raw json.RawMessage) []map[string]any {
	var arr []map[string]any
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr
	}
	var env map[string]json.RawMessage
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil
	}
	for _, key := range []string{"data", "results", "items", "people"} {
		if v, ok := env[key]; ok {
			var inner []map[string]any
			if err := json.Unmarshal(v, &inner); err == nil {
				return inner
			}
			// Try nested {data: {results: [...]}}
			var nested map[string]json.RawMessage
			if err := json.Unmarshal(v, &nested); err == nil {
				for _, k := range []string{"results", "items", "people", "data"} {
					if vv, ok := nested[k]; ok {
						var x []map[string]any
						if err := json.Unmarshal(vv, &x); err == nil {
							return x
						}
					}
				}
			}
		}
	}
	return nil
}

func extractCreditsInfo(raw json.RawMessage) (int, bool) {
	var env map[string]json.RawMessage
	if err := json.Unmarshal(raw, &env); err != nil {
		return 1, false
	}
	free := false
	credits := 1
	if v, ok := env["free_enrichment"]; ok {
		_ = json.Unmarshal(v, &free)
	}
	if v, ok := env["credits_spent"]; ok {
		_ = json.Unmarshal(v, &credits)
	}
	return credits, free
}

// writeFunnelCSV flattens enriched person objects into a CSV. Each row's
// keys form the union of headers; nested values are JSON-encoded.
func writeFunnelCSV(path string, rows []map[string]any) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if len(rows) == 0 {
		return nil
	}
	keySet := map[string]struct{}{}
	for _, r := range rows {
		for k := range r {
			keySet[k] = struct{}{}
		}
	}
	headers := make([]string, 0, len(keySet))
	for k := range keySet {
		headers = append(headers, k)
	}
	w := csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write(headers); err != nil {
		return err
	}
	for _, r := range rows {
		out := make([]string, len(headers))
		for i, h := range headers {
			v, ok := r[h]
			if !ok || v == nil {
				continue
			}
			switch tv := v.(type) {
			case string:
				out[i] = tv
			case float64:
				out[i] = fmt.Sprintf("%v", tv)
			case bool:
				out[i] = fmt.Sprintf("%t", tv)
			default:
				b, _ := json.Marshal(v)
				out[i] = string(b)
			}
		}
		if err := w.Write(out); err != nil {
			return err
		}
	}
	return nil
}
