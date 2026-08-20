// Hand-authored Lancet analytics command. Not generated.

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/thelancet/internal/lancet"
)

// parseYearWindow parses a "YYYY:YYYY" (or "YYYY-YYYY") window into start,end.
func parseYearWindow(s string) (int, int, error) {
	s = strings.TrimSpace(s)
	sep := ":"
	if !strings.Contains(s, ":") && strings.Contains(s, "-") {
		sep = "-"
	}
	parts := strings.SplitN(s, sep, 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("window %q must be YYYY:YYYY (e.g. 2018:2020)", s)
	}
	start, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	end, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err1 != nil || err2 != nil || start == 0 || end == 0 {
		return 0, 0, fmt.Errorf("window %q must be YYYY:YYYY (e.g. 2018:2020)", s)
	}
	if start > end {
		start, end = end, start
	}
	return start, end, nil
}

func newNovelDriftCmd(flags *rootFlags) *cobra.Command {
	var journal string
	var window1 string
	var window2 string
	var topN int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Compare a journal's topic distribution between two year windows",
		Long: "Show how a Lancet journal's topic mix shifts between two publication-year\n" +
			"windows (positive delta = rising in the later window). Reads the local\n" +
			"mirror; run 'thelancet-pp-cli refresh' first.",
		Example:     "  thelancet-pp-cli drift --journal lancet-oncology --window1 2018:2020 --window2 2023:2024\n  thelancet-pp-cli drift --window1 2015:2019 --window2 2020:2024 --top-n 15 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--window1=2018:2021;--window2=2022:2026"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if window1 == "" || window2 == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--window1 and --window2 are required (e.g. --window1 2018:2020 --window2 2023:2024)"))
			}
			w1s, w1e, err := parseYearWindow(window1)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			w2s, w2e, err := parseYearWindow(window2)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			if w2s <= w1e {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("window2 must start after window1 ends: window1 ends %d, window2 starts %d", w1e, w2s))
			}
			issn, err := resolveJournalISSN(journal)
			if err != nil {
				_ = cmd.Usage()
				return err
			}
			st, err := ensureLancetStore(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			defer st.Close()

			rows, err := lancet.TopicDrift(cmd.Context(), st.DB(), issn, w1s, w1e, w2s, w2e, topN)
			if err != nil {
				return fmt.Errorf("computing drift: %w", err)
			}
			if rows == nil {
				rows = []lancet.TopicShift{}
			}
			if done, err := emitLancet(cmd, flags, rows); done || err != nil {
				return err
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no topic data in those windows (try wider windows or run refresh)")
				return nil
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%-44s %6s %6s %9s\n", "TOPIC", "W1", "W2", "ΔSHARE")
			for _, r := range rows {
				fmt.Fprintf(cmd.OutOrStdout(), "%-44.44s %6d %6d %+8.1f%%\n", r.Topic, r.Window1Count, r.Window2Count, r.DeltaShare*100)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&journal, "journal", "", "Scope to a Lancet journal slug, or omit for all")
	cmd.Flags().StringVar(&window1, "window1", "", "Earlier year window, YYYY:YYYY (e.g. 2018:2020)")
	cmd.Flags().StringVar(&window2, "window2", "", "Later year window, YYYY:YYYY (e.g. 2023:2024)")
	cmd.Flags().IntVar(&topN, "top-n", 20, "Maximum topics to return (largest absolute share change)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default ~/.local/share/thelancet-pp-cli/data.db)")
	return cmd
}