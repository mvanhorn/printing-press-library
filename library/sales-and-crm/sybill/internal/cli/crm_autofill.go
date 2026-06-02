// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel feature: surface Sybill's crmAutofill suggestions as a diff.

package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/sybill/internal/store"
	"github.com/spf13/cobra"
)

type autofillRow struct {
	DealID    string `json:"dealId"`
	DealName  string `json:"dealName,omitempty"`
	Field     string `json:"field"`
	Suggested string `json:"suggested"`
	Current   string `json:"current"`
}

// dealFieldForAutofillKey maps a crmAutofill key onto the deal field that holds
// the current value, when the key corresponds to a field we already track.
var dealFieldForAutofillKey = map[string]string{
	"stage":       "stage",
	"amount":      "amount",
	"name":        "name",
	"pipeline":    "pipeline",
	"closedate":   "closeDate",
	"close_date":  "closeDate",
	"accountname": "accountName",
	"account":     "accountName",
}

func newNovelCrmAutofillCmd(flags *rootFlags) *cobra.Command {
	var deal string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "crm-autofill",
		Short: "Show the AI-suggested CRM field updates Sybill generated, as a reviewable field-by-field diff.",
		Long: `Surface the crmAutofill suggestions Sybill attaches to deal detail: the CRM
field values its AI recommends writing. Each row shows the suggested value and,
where the field maps onto a value we already track, the current value — so you
can review pending changes before pushing them to your CRM.

crmAutofill lives in deal detail, not the deal list. With --deal, the detail is
fetched live if it is not already in the store. Without --deal, this reads
whatever deal detail has been synced locally.`,
		Example: strings.Trim(`
  # All pending CRM suggestions across synced deals
  sybill-pp-cli crm-autofill

  # One deal (fetched live if needed)
  sybill-pp-cli crm-autofill --deal 550e8400-e29b-41d4-a716-446655440000

  # Just the diff columns, as JSON
  sybill-pp-cli crm-autofill --agent --select dealName,field,suggested,current
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			out := cmd.OutOrStdout()
			if dbPath == "" {
				dbPath = defaultDBPath("sybill-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'sybill-pp-cli sync' first.", err)
			}
			defer db.Close()

			var deals []map[string]any
			if deal != "" {
				// Try the store first, then fall back to a live detail fetch.
				if obj, err := db.Get("deals", deal); err == nil && len(obj) > 0 {
					var d map[string]any
					if json.Unmarshal(obj, &d) == nil {
						deals = append(deals, d)
					}
				}
				if len(deals) == 0 || nestedObj(deals[0], "crmAutofill") == nil {
					if strings.EqualFold(flags.dataSource, "local") {
						// Offline-only: don't reach the network.
					} else if d, ferr := fetchDealDetail(cmd, flags, deal); ferr == nil && d != nil {
						deals = []map[string]any{d}
					} else if ferr != nil && len(deals) == 0 {
						return ferr
					}
				}
			} else {
				deals, err = loadRecords(db, "deals")
				if err != nil {
					return err
				}
			}

			var rows []autofillRow
			for _, d := range deals {
				af := nestedObj(d, "crmAutofill")
				if af == nil || len(af) == 0 {
					continue
				}
				keys := make([]string, 0, len(af))
				for k := range af {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					rows = append(rows, autofillRow{
						DealID:    dealID(d),
						DealName:  dealName(d),
						Field:     k,
						Suggested: scalarString(af[k]),
						Current:   currentForKey(d, k),
					})
				}
			}

			if novelMachineOutput(out, flags) {
				return printJSONFiltered(out, rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(out, "No crmAutofill suggestions found.")
				fmt.Fprintln(cmd.ErrOrStderr(), "crmAutofill is only present in deal detail. Try 'sybill-pp-cli crm-autofill --deal <id>' or sync deal details first.")
				return nil
			}
			fmt.Fprintf(out, "%-28s  %-20s  %-24s  %s\n", "DEAL", "FIELD", "SUGGESTED", "CURRENT")
			for _, r := range rows {
				dn := r.DealName
				if dn == "" {
					dn = r.DealID
				}
				fmt.Fprintf(out, "%-28s  %-20s  %-24s  %s\n",
					truncate(dn, 28), truncate(r.Field, 20), truncate(r.Suggested, 24), truncate(orNone(r.Current), 24))
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "\n%d suggestion(s).\n", len(rows))
			return nil
		},
	}
	cmd.Flags().StringVar(&deal, "deal", "", "Limit to one deal id (fetched live if not in the local store)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: standard cache location)")
	return cmd
}

// fetchDealDetail GETs a single deal's detail from the live API.
func fetchDealDetail(cmd *cobra.Command, flags *rootFlags, id string) (map[string]any, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	path := replacePathParam("/v1/deals/{dealId}", "dealId", id)
	data, err := c.Get(cmd.Context(), path, nil)
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	var d map[string]any
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("parsing deal detail: %w", err)
	}
	return d, nil
}

// currentForKey returns the deal's current value for a crmAutofill key when the
// key maps onto a field we track; otherwise empty.
func currentForKey(d map[string]any, key string) string {
	if field, ok := dealFieldForAutofillKey[strings.ToLower(key)]; ok {
		return str(d, field)
	}
	return ""
}

// scalarString renders a crmAutofill value (string/number/bool/array) as a
// compact string for display.
func scalarString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64, bool:
		return fmt.Sprintf("%v", t)
	case []any:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			parts = append(parts, fmt.Sprintf("%v", item))
		}
		return strings.Join(parts, ", ")
	case nil:
		return ""
	default:
		b, _ := json.Marshal(t)
		return string(b)
	}
}

func orNone(s string) string {
	if s == "" {
		return "(unset)"
	}
	return s
}
