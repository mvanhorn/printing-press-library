package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/sumble/internal/cliutil"

	"github.com/spf13/cobra"
)

type reconcileResult struct {
	Input   string `json:"input"`
	Matched bool   `json:"matched"`
	OrgID   int    `json:"org_id,omitempty"`
	Slug    string `json:"slug,omitempty"`
	Name    string `json:"name,omitempty"`
	Domain  string `json:"domain,omitempty"`
}

func newReconcileCmd(flags *rootFlags) *cobra.Command {
	var nameCol, urlCol, locCol string

	cmd := &cobra.Command{
		Use:   "reconcile <csv>",
		Short: "Resolve a CSV of companies to Sumble IDs via the cheap match endpoint",
		Long: strings.Trim(`
Read a CSV of companies and resolve them to Sumble organizations using
organizations/match (1 credit per matched org; unmatched are free). Resolved IDs
are cached locally. The report separates matched orgs (ready to enrich) from
unmatched rows, so you spend enrich credits only on accounts that resolved.

The CSV must have a header row. By default the 'name', 'url', and 'location'
columns are used; override with --name-col / --url-col / --location-col.
`, "\n"),
		Example: strings.Trim(`
  sumble-pp-cli reconcile accounts.csv
  sumble-pp-cli reconcile accounts.csv --name-col company --json
`, "\n"),
		// No mcp:read-only — reconcile calls organizations/match, which spends
		// credits (1 per matched org). It does not mutate external state, but it
		// is not free, so it should not advertise as a safe read-only tool.
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			path := args[0]

			// Short-circuit under the verifier before any file IO or network
			// call, so verify/dry-run probes don't require the CSV to exist.
			if cliutil.IsVerifyEnv() {
				if flags.asJSON {
					return flags.printJSON(cmd, map[string]any{"verify_noop": true})
				}
				fmt.Fprintln(cmd.OutOrStdout(), "verify mode: no match call made")
				return nil
			}

			inputs, err := readReconcileCSV(path, nameCol, urlCol, locCol)
			if err != nil {
				return usageErr(err)
			}
			if len(inputs) == 0 {
				return usageErr(fmt.Errorf("no rows found in %s", path))
			}

			c, cerr := flags.newClient()
			if cerr != nil {
				return cerr
			}
			db, derr := openCreditStore()
			if derr != nil {
				return configErr(derr)
			}
			defer db.Close()

			raw, _, perr := c.Post(cmd.Context(), "/organizations/match", map[string]any{"organizations": inputs})
			if perr != nil {
				return classifyAPIError(perr, flags)
			}
			env := parseEnvelope(raw)
			recordEnvelope(db.DB(), "organizations.match", env, fmt.Sprintf("reconcile %d rows", len(inputs)))

			results := parseMatchResults(raw)
			matched := 0
			for _, r := range results {
				if r.Matched {
					matched++
				}
			}

			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{
					"total":     len(results),
					"matched":   matched,
					"unmatched": len(results) - matched,
					"results":   results,
				})
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Matched %d of %d companies. Matched orgs are ready for a billed enrich; unmatched rows cost nothing.\n", matched, len(results))
			for _, r := range results {
				if r.Matched {
					fmt.Fprintf(w, "  matched  %-30s -> id=%d slug=%s\n", truncate(r.Input, 30), r.OrgID, r.Slug)
				} else {
					fmt.Fprintf(w, "  no-match %-30s\n", truncate(r.Input, 30))
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&nameCol, "name-col", "name", "CSV column holding the company name")
	cmd.Flags().StringVar(&urlCol, "url-col", "url", "CSV column holding the company URL")
	cmd.Flags().StringVar(&locCol, "location-col", "location", "CSV column holding the company location")
	return cmd
}

func readReconcileCSV(path, nameCol, urlCol, locCol string) ([]map[string]any, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening CSV: %w", err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("reading CSV header: %w", err)
	}
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.TrimSpace(strings.ToLower(h))] = i
	}
	ni, hasName := idx[strings.ToLower(nameCol)]
	ui, hasURL := idx[strings.ToLower(urlCol)]
	li, hasLoc := idx[strings.ToLower(locCol)]
	if !hasName && !hasURL {
		return nil, fmt.Errorf("CSV must contain a %q or %q column", nameCol, urlCol)
	}

	var out []map[string]any
	// PATCH(csv-distinguish-eof-from-parse-error): csv.Reader returns
	// io.EOF when the input is exhausted, but malformed input (e.g. an
	// unterminated quoted field) returns *csv.ParseError. The original
	// loop treated both the same, so a parse error on row N silently
	// discarded rows N+1..M with no signal to the caller.
	for row := 0; ; row++ {
		rec, rerr := r.Read()
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return nil, fmt.Errorf("CSV parse error at row %d: %w", row+2, rerr)
		}
		org := map[string]any{}
		if hasName && ni < len(rec) && strings.TrimSpace(rec[ni]) != "" {
			org["name"] = strings.TrimSpace(rec[ni])
		}
		if hasURL && ui < len(rec) && strings.TrimSpace(rec[ui]) != "" {
			org["url"] = strings.TrimSpace(rec[ui])
		}
		if hasLoc && li < len(rec) && strings.TrimSpace(rec[li]) != "" {
			org["location"] = strings.TrimSpace(rec[li])
		}
		if len(org) > 0 {
			out = append(out, org)
		}
	}
	return out, nil
}

func parseMatchResults(raw json.RawMessage) []reconcileResult {
	var resp struct {
		Results []struct {
			Input any `json:"input"`
			Match *struct {
				ID     int    `json:"id"`
				Slug   string `json:"slug"`
				Name   string `json:"name"`
				Domain string `json:"domain"`
			} `json:"match"`
		} `json:"results"`
	}
	_ = json.Unmarshal(raw, &resp)
	out := make([]reconcileResult, 0, len(resp.Results))
	for _, r := range resp.Results {
		rr := reconcileResult{Input: stringifyInput(r.Input)}
		if r.Match != nil {
			rr.Matched = true
			rr.OrgID = r.Match.ID
			rr.Slug = r.Match.Slug
			rr.Name = r.Match.Name
			rr.Domain = r.Match.Domain
		}
		out = append(out, rr)
	}
	return out
}

func stringifyInput(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case map[string]any:
		for _, k := range []string{"name", "url", "location"} {
			if s, ok := t[k].(string); ok && s != "" {
				return s
			}
		}
	}
	b, _ := json.Marshal(v)
	return string(b)
}
