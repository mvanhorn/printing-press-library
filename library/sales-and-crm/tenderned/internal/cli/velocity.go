package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newVelocityCmd(flags *rootFlags) *cobra.Command {
	var dbPath, cpv, buyer string
	var weeks int

	cmd := &cobra.Command{
		Use:   "velocity",
		Short: "Weekly notice-count trend — see if a market is heating up or cooling (mirrors eu-tenders velocity)",
		Long: `Weekly bucket count of notices in the local snapshot, optionally filtered
by CPV stripe or buyer. Returns the last N weeks (default 12).`,
		Example: "  tenderned-pp-cli velocity --cpv 72000000-5 --weeks 12",
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
			cpvList := tnSplitCSV(cpv)
			now := time.Now().UTC()
			earliest := now.AddDate(0, 0, -weeks*7)
			buckets := map[string]int{}
			for _, n := range notices {
				if buyer != "" && !strings.Contains(strings.ToLower(n.OpdrachtgeverNaam), strings.ToLower(buyer)) {
					continue
				}
				if !tnHasCPV(n, cpvList) {
					continue
				}
				d := tnParseDate(n.PublicatieDatum)
				if d.IsZero() || d.Before(earliest) || d.After(now) {
					continue
				}
				y, w := d.ISOWeek()
				key := fmt.Sprintf("%04d-W%02d", y, w)
				buckets[key]++
			}
			keys := make([]string, 0, len(buckets))
			for k := range buckets {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			series := make([]map[string]any, len(keys))
			for i, k := range keys {
				series[i] = map[string]any{"week": k, "count": buckets[k]}
			}
			result := map[string]any{
				"velocity": series,
				"weeks":    weeks,
				"cpv":      cpvList,
				"buyer":    buyer,
			}
			if flags.asJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(result)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Week     | Notices")
			for _, row := range series {
				fmt.Fprintf(cmd.OutOrStdout(), "%-8s | %d\n", row["week"], row["count"])
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&cpv, "cpv", "", "Comma-separated CPV stripes")
	cmd.Flags().StringVar(&buyer, "buyer", "", "Substring match on buyer name")
	cmd.Flags().IntVar(&weeks, "weeks", 12, "Number of past weeks to include")
	return cmd
}
