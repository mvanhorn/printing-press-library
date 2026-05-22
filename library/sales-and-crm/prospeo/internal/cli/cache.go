// Hand-authored: novel feature `cache predict`.
//
// Read a CSV of leads, look each row up in the local Supabase cache, and
// predict which rows are likely free Prospeo lifetime-duplicate hits before
// the user spends any credits.

package cli

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/prospeo/internal/supa"
)

// PredictionRow is one CSV row's cache prediction.
type PredictionRow struct {
	RowIndex             int    `json:"row_index"`
	Key                  string `json:"key"`
	KeyKind              string `json:"key_kind"`
	Cached               bool   `json:"cached"`
	PersonID             string `json:"person_id,omitempty"`
	FreeEnrichmentLikely bool   `json:"free_enrichment_likely"`
}

// PredictSummary aggregates a predict run.
type PredictSummary struct {
	TotalRows            int `json:"total_rows"`
	CachedRows           int `json:"cached_rows"`
	UncachedRows         int `json:"uncached_rows"`
	ProjectedCostCredits int `json:"projected_cost_credits"`
	ProjectedFreeHits    int `json:"projected_free_hits"`
}

// PredictResult is the full predict response.
type PredictResult struct {
	Input   string          `json:"input"`
	Summary PredictSummary  `json:"summary"`
	Rows    []PredictionRow `json:"rows"`
}

func newCacheCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cache",
		Short: "Cache-aware helpers for offline lookup and credit-cost prediction.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCachePredictCmd(flags))
	return cmd
}

func newCachePredictCmd(flags *rootFlags) *cobra.Command {
	var input string
	var mobile bool
	cmd := &cobra.Command{
		Use:   "predict",
		Short: "Predict which rows in a leads CSV are already cached (and free) before spending credits.",
		Long: `Reads a CSV of leads (headers: first_name, last_name, linkedin_url|linkedin,
email, company|domain|company_website) and looks each row up in the local
Supabase cache. Reports which rows are likely free Prospeo lifetime-
duplicate hits and projects the credit cost for the remainder.`,
		Example: "  prospeo-pp-cli cache predict --input leads.csv\n  prospeo-pp-cli cache predict --input leads.csv --mobile",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if input == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would read %s and query supabase outreach.people for cache hits\n", input)
				return nil
			}
			rows, err := readPredictCSV(input)
			if err != nil {
				return fmt.Errorf("reading CSV: %w", err)
			}
			if !supa.IsConfigured() {
				return configErr(fmt.Errorf("SUPABASE_URL and SUPABASE_SERVICE_KEY must be set to use 'cache predict'"))
			}
			cfg, err := supa.LoadConfig()
			if err != nil {
				return configErr(fmt.Errorf("supabase config: %w", err))
			}
			sc := supa.New(cfg)
			result, err := runPredict(cmd.Context(), sc, input, rows, mobile)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&input, "input", "", "Path to leads CSV.")
	cmd.Flags().BoolVar(&mobile, "mobile", false, "Estimate cost assuming --enrich-mobile is set (10 credits per uncached row).")
	return cmd
}

// csvRow is the canonicalized form of a single CSV row.
type csvRow struct {
	firstName   string
	lastName    string
	linkedinURL string
	email       string
	domain      string
}

func readPredictCSV(path string) ([]csvRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}
	idx := map[string]int{}
	for i, h := range header {
		key := strings.ToLower(strings.TrimSpace(h))
		idx[key] = i
	}
	pick := func(row []string, names ...string) string {
		for _, n := range names {
			if i, ok := idx[n]; ok && i < len(row) {
				v := strings.TrimSpace(row[i])
				if v != "" {
					return v
				}
			}
		}
		return ""
	}
	var rows []csvRow
	for {
		rec, err := r.Read()
		if err != nil {
			break
		}
		rows = append(rows, csvRow{
			firstName:   pick(rec, "first_name", "firstname", "first"),
			lastName:    pick(rec, "last_name", "lastname", "last"),
			linkedinURL: pick(rec, "linkedin_url", "linkedin", "linkedinurl"),
			email:       pick(rec, "email"),
			domain:      pick(rec, "domain", "company_website", "company", "website"),
		})
	}
	return rows, nil
}

func runPredict(ctx context.Context, sc *supa.Client, input string, rows []csvRow, mobile bool) (*PredictResult, error) {
	out := &PredictResult{Input: input, Rows: make([]PredictionRow, 0, len(rows))}
	for i, r := range rows {
		key, kind := canonicalKey(r)
		pred := PredictionRow{RowIndex: i, Key: key, KeyKind: kind}
		if key != "" {
			id, ok := lookupPersonByKey(ctx, sc, r, kind)
			if ok {
				pred.Cached = true
				pred.PersonID = id
				pred.FreeEnrichmentLikely = true
			}
		}
		out.Rows = append(out.Rows, pred)
	}
	creditsPerUncached := 1
	if mobile {
		creditsPerUncached = 10
	}
	for _, p := range out.Rows {
		out.Summary.TotalRows++
		if p.Cached {
			out.Summary.CachedRows++
			out.Summary.ProjectedFreeHits++
		} else {
			out.Summary.UncachedRows++
		}
	}
	out.Summary.ProjectedCostCredits = out.Summary.UncachedRows * creditsPerUncached
	return out, nil
}

func canonicalKey(r csvRow) (string, string) {
	switch {
	case r.linkedinURL != "":
		return r.linkedinURL, "linkedin_url"
	case r.email != "":
		return strings.ToLower(r.email), "email"
	case r.firstName != "" && r.lastName != "" && r.domain != "":
		return strings.ToLower(r.firstName + "+" + r.lastName + "+" + r.domain), "name_company"
	}
	return "", "none"
}

// lookupPersonByKey hits Supabase by the most identifying key available.
func lookupPersonByKey(ctx context.Context, sc *supa.Client, r csvRow, kind string) (string, bool) {
	params := url.Values{}
	params.Set("select", "id")
	params.Set("limit", strconv.Itoa(1))
	switch kind {
	case "linkedin_url":
		params.Set("linkedin_url", "eq."+r.linkedinURL)
	case "email":
		params.Set("email", "eq."+strings.ToLower(r.email))
	case "name_company":
		full := strings.TrimSpace(r.firstName + " " + r.lastName)
		params.Set("full_name", "ilike."+full)
	default:
		return "", false
	}
	raw, err := sc.Select(ctx, "people", params)
	if err != nil {
		return "", false
	}
	var hits []struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &hits); err != nil {
		return "", false
	}
	if len(hits) == 0 {
		return "", false
	}
	return hits[0].ID, true
}
