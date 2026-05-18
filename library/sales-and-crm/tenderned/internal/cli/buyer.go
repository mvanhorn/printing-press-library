package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newBuyerCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "buyer",
		Short: "Per-buyer queries against the local SQLite snapshot (mirrors eu-tenders buyer)",
	}
	cmd.AddCommand(newBuyerDossierCmd(flags))
	return cmd
}

func newBuyerDossierCmd(flags *rootFlags) *cobra.Command {
	var dbPath, since, until string
	var topN int

	cmd := &cobra.Command{
		Use:   "dossier [buyer-name-or-id]",
		Short: "Full procurement profile for one contracting authority over a window",
		Long: `Full procurement profile for one Dutch contracting authority computed from the
local SQLite snapshot: notice cadence by month, top CPVs, top procedures,
active-vs-awarded-vs-cancelled counts.

Requires a populated local store. Run 'tenderned-pp-cli sync' first.`,
		Example: "  tenderned-pp-cli buyer dossier \"Gemeente Rotterdam\" --since 2025-01-01",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			needle := strings.ToLower(strings.TrimSpace(args[0]))
			s, err := tnOpenStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			notices, err := tnLoadNotices(cmd.Context(), s)
			if err != nil {
				return err
			}

			filtered := []tnNotice{}
			for _, n := range notices {
				if !strings.Contains(strings.ToLower(n.OpdrachtgeverNaam), needle) {
					continue
				}
				// PATCH: parse publicatieDatum via tnParseDate and compare as
				// time.Time so sub-second precision and "Z" tz suffix values
				// inside the window aren't dropped by a raw string compare.
				pd := tnParseDate(n.PublicatieDatum)
				if since != "" {
					if sinceT := tnParseDate(since); !sinceT.IsZero() && !pd.IsZero() && pd.Before(sinceT) {
						continue
					}
				}
				if until != "" {
					if untilT := tnParseDate(until); !untilT.IsZero() && !pd.IsZero() && pd.After(untilT.Add(24*time.Hour-time.Nanosecond)) {
						continue
					}
				}
				filtered = append(filtered, n)
			}

			cpvCounts := map[string]int{}
			procCounts := map[string]int{}
			monthly := map[string]int{}
			active, awarded, cancelled := 0, 0, 0
			for _, n := range filtered {
				if n.IsGegund {
					awarded++
				} else if n.IsVroegtijdigBeeindigd {
					cancelled++
				} else {
					active++
				}
				for _, c := range n.CPVCodes {
					code2 := strings.SplitN(c.Code, "-", 2)[0]
					if len(code2) >= 2 {
						cpvCounts[code2[:2]+" "+c.Omschrijving]++
					}
				}
				if n.ProcedureCode.Omschrijving != "" {
					procCounts[n.ProcedureCode.Omschrijving]++
				}
				if len(n.PublicatieDatum) >= 7 {
					monthly[n.PublicatieDatum[:7]]++
				}
			}

			dossier := map[string]any{
				"buyer_query":     args[0],
				"total_notices":   len(filtered),
				"active":          active,
				"awarded":         awarded,
				"cancelled":       cancelled,
				"top_cpvs":        topNStringCount(cpvCounts, topN),
				"top_procedures":  topNStringCount(procCounts, topN),
				"monthly_cadence": sortedMonthlyMap(monthly),
				"window": map[string]string{
					"since": since,
					"until": until,
				},
			}
			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(dossier)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Dossier: %s\n", args[0])
			fmt.Fprintf(cmd.OutOrStdout(), "  total notices: %d  (active=%d, awarded=%d, cancelled=%d)\n", len(filtered), active, awarded, cancelled)
			fmt.Fprintln(cmd.OutOrStdout(), "  top CPVs:")
			for _, kv := range topNStringCount(cpvCounts, topN) {
				fmt.Fprintf(cmd.OutOrStdout(), "    %4d  %s\n", kv["count"], kv["key"])
			}
			fmt.Fprintln(cmd.OutOrStdout(), "  top procedures:")
			for _, kv := range topNStringCount(procCounts, topN) {
				fmt.Fprintf(cmd.OutOrStdout(), "    %4d  %s\n", kv["count"], kv["key"])
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default ~/.local/share/tenderned-pp-cli/data.db)")
	cmd.Flags().StringVar(&since, "since", "", "Earliest publication date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&until, "until", "", "Latest publication date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&topN, "top", 10, "Top-N CPVs and procedures to surface")
	return cmd
}

func topNStringCount(m map[string]int, n int) []map[string]any {
	type kv struct {
		Key   string
		Count int
	}
	list := make([]kv, 0, len(m))
	for k, c := range m {
		list = append(list, kv{k, c})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Count > list[j].Count })
	if n > 0 && len(list) > n {
		list = list[:n]
	}
	out := make([]map[string]any, len(list))
	for i, item := range list {
		out[i] = map[string]any{"key": item.Key, "count": item.Count}
	}
	return out
}

func sortedMonthlyMap(m map[string]int) []map[string]any {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]map[string]any, len(keys))
	for i, k := range keys {
		out[i] = map[string]any{"month": k, "count": m[k]}
	}
	return out
}
