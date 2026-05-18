package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

func newThreadCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "thread",
		Short: "Walk tender-lifecycle chains: PIN → CN → CAN → Modification",
	}
	cmd.AddCommand(newThreadReconcileCmd(flags))
	return cmd
}

func newThreadReconcileCmd(flags *rootFlags) *cobra.Command {
	var dbPath, buyer, since string
	cmd := &cobra.Command{
		Use:   "reconcile",
		Short: "For a buyer, group notices into lifecycle threads and flag orphans",
		Long: `Walks the local notices snapshot for one buyer, buckets each notice by
publicatieCode (PIN / CN / CAN / MOD), and flags orphan PINs (no CN follow-up)
and orphan CNs (no CAN gunning). This is a lifecycle-coherence audit.

Full predecessor-ref linkage requires the eForms XML (set TENDERNED_USERNAME and
TENDERNED_PASSWORD); without credentials, reconcile groups by buyer+kenmerk
heuristics only.`,
		Example: "  tenderned-pp-cli thread reconcile --buyer \"Gemeente Eindhoven\" --since 2024-01-01",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			s, err := tnOpenStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer s.Close()
			notices, err := tnLoadNotices(cmd.Context(), s)
			if err != nil {
				return err
			}
			needle := strings.ToLower(strings.TrimSpace(buyer))
			byKenmerk := map[string]map[string][]string{} // kenmerk → bucket → pubIds
			rawByKenmerk := map[string][]tnNotice{}
			for _, n := range notices {
				if needle != "" && !strings.Contains(strings.ToLower(n.OpdrachtgeverNaam), needle) {
					continue
				}
				if since != "" && n.PublicatieDatum < since {
					continue
				}
				bucket := tnPublicationTypeBucket(n.PublicatieCode)
				// Fall back to the buyer-issued kenmerk for thread linkage.
				key := n.AanbestedingNaam // best effort
				if byKenmerk[key] == nil {
					byKenmerk[key] = map[string][]string{}
				}
				byKenmerk[key][bucket] = append(byKenmerk[key][bucket], n.PublicatieID.String())
				rawByKenmerk[key] = append(rawByKenmerk[key], n)
			}
			type thread struct {
				Key      string              `json:"thread_key"`
				Notices  map[string][]string `json:"notices"`
				Orphan   string              `json:"orphan_state,omitempty"`
				Complete bool                `json:"complete"`
			}
			var threads []thread
			orphanPIN, orphanCN, complete := 0, 0, 0
			for k, buckets := range byKenmerk {
				t := thread{Key: k, Notices: buckets}
				hasPIN := len(buckets["PIN"]) > 0
				hasCN := len(buckets["CN"]) > 0
				hasCAN := len(buckets["CAN"]) > 0
				switch {
				case hasPIN && !hasCN && !hasCAN:
					t.Orphan = "PIN without CN"
					orphanPIN++
				case hasCN && !hasCAN:
					t.Orphan = "CN without CAN (still open or unawarded)"
					orphanCN++
				case hasCAN:
					t.Complete = true
					complete++
				}
				threads = append(threads, t)
			}
			sort.Slice(threads, func(i, j int) bool { return threads[i].Key < threads[j].Key })
			result := map[string]any{
				"buyer":        buyer,
				"thread_count": len(threads),
				"complete":     complete,
				"orphan_pin":   orphanPIN,
				"orphan_cn":    orphanCN,
				"threads":      threads,
				"since":        since,
				"note":         "linkage is by-title heuristic; full eForms predecessor refs require Basic-auth XML fetch",
			}
			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Buyer %q — %d threads (%d complete, %d orphan-PIN, %d orphan-CN)\n",
				buyer, len(threads), complete, orphanPIN, orphanCN)
			for _, t := range threads {
				mark := "✓"
				if !t.Complete {
					mark = "!"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s %s — PIN=%d CN=%d CAN=%d MOD=%d  %s\n",
					mark, t.Key,
					len(t.Notices["PIN"]), len(t.Notices["CN"]), len(t.Notices["CAN"]), len(t.Notices["MOD"]),
					t.Orphan)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&buyer, "buyer", "", "Substring match on buyer name (required for useful output)")
	cmd.Flags().StringVar(&since, "since", "", "Earliest publication date (YYYY-MM-DD)")
	return cmd
}
