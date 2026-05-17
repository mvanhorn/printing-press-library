package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-salestech/internal/salestech"
)

func newReportsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reports",
		Short: "Cross-entity sales reports computed locally over the synced store",
		Long:  "Subcommands for the questions pipeline-review meetings ask: close rate, days-to-sell, leaderboard, pipeline snapshot, dismissed reasons, SKU frequency, follow-up call list.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newReportsRepLeaderboardCmd(flags))
	cmd.AddCommand(newReportsCloseRateCmd(flags))
	cmd.AddCommand(newReportsDaysToSellCmd(flags))
	cmd.AddCommand(newReportsDismissedReasonsCmd(flags))
	cmd.AddCommand(newReportsPipelineCmd(flags))
	cmd.AddCommand(newReportsSkuFrequencyCmd(flags))
	cmd.AddCommand(newReportsFollowUpsCmd(flags))
	return cmd
}

func newReportsRepLeaderboardCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		since  string
		limit  int
	)
	cmd := &cobra.Command{
		Use:   "rep-leaderboard",
		Short: "Per-rep close rate + avg days-to-sell + total sold $ for the chosen window",
		Long: "Groups every estimate created since --since by soldById and reports\n" +
			"close rate, avg days-to-sell, and total sold $. Estimates without a\n" +
			"soldBy bucket under id 0 (\"unassigned\"). Run 'sync' + 'sync-status-\n" +
			"changes' first; without status_changes, days-to-sell is 0.",
		Example: strings.Trim(`
  servicetitan-salestech-pp-cli reports rep-leaderboard --since 90d --json
  servicetitan-salestech-pp-cli reports rep-leaderboard --since 2026-01-01 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			t, err := parseSinceDate(since)
			if err != nil {
				return err
			}
			db, err := openSalestechStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := salestech.RepLeaderboard(db, t)
			if err != nil {
				return err
			}
			rows = capRows(rows, limit)
			if rows == nil {
				rows = []salestech.RepLeaderboardRow{}
			}
			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{
					i64(r.SoldByID), iN(r.EstimatesCreated), iN(r.Sold), iN(r.Dismissed), iN(r.Open),
					f3(r.CloseRate), f2(r.AvgDaysToSell), f2(r.TotalSold),
				})
			}
			return stOutput(cmd, flags, rows,
				[]string{"REP", "CREATED", "SOLD", "DISMISSED", "OPEN", "CLOSE RATE", "AVG DAYS", "TOTAL SOLD $"},
				table)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&since, "since", "90d", "Window: YYYY-MM-DD or duration (24h, 7d, 90d, 1y); estimates created before this are excluded")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows (0 = all)")
	return cmd
}

func newReportsCloseRateCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath  string
		groupBy string
		since   string
		limit   int
	)
	cmd := &cobra.Command{
		Use:   "close-rate",
		Short: "sold/(sold+dismissed) pivoted on businessUnit, rep, or month",
		Long: "Computes close rate as sold/(sold+dismissed) grouped by the chosen\n" +
			"dimension across estimates created since --since. Use --group-by\n" +
			"month for trend analysis; use --group-by businessUnit for org-level\n" +
			"performance.",
		Example: strings.Trim(`
  servicetitan-salestech-pp-cli reports close-rate --group-by businessUnit --since 90d --json
  servicetitan-salestech-pp-cli reports close-rate --group-by month --since 1y --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			t, err := parseSinceDate(since)
			if err != nil {
				return err
			}
			db, err := openSalestechStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			dim := salestech.GroupByDim(groupBy)
			rows, err := salestech.CloseRate(db, dim, t)
			if err != nil {
				return err
			}
			rows = capRows(rows, limit)
			if rows == nil {
				rows = []salestech.CloseRateRow{}
			}
			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{r.Group, iN(r.Sold), iN(r.Dismissed), iN(r.Open), f3(r.CloseRate), f2(r.TotalSold)})
			}
			return stOutput(cmd, flags, rows,
				[]string{"GROUP", "SOLD", "DISMISSED", "OPEN", "CLOSE RATE", "TOTAL SOLD $"},
				table)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&groupBy, "group-by", "businessUnit", "Pivot dimension: businessUnit | rep | month")
	cmd.Flags().StringVar(&since, "since", "90d", "Window: YYYY-MM-DD or duration (90d, 1y, ...)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows (0 = all)")
	return cmd
}

func newReportsDaysToSellCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath      string
		groupBy     string
		since       string
		percentiles bool
		limit       int
	)
	cmd := &cobra.Command{
		Use:   "days-to-sell",
		Short: "Distribution of days from createdOn to first Sold transition (per rep or business unit)",
		Long: "Computes the percentile distribution of (Sold timestamp − createdOn)\n" +
			"in days, grouped by rep (default) or businessUnit. Estimates without\n" +
			"a Sold status_change are skipped. Run 'sync' + 'sync-status-changes'\n" +
			"first.",
		Example: strings.Trim(`
  servicetitan-salestech-pp-cli reports days-to-sell --since 90d --json
  servicetitan-salestech-pp-cli reports days-to-sell --group-by businessUnit --since 1y --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			_ = percentiles // reserved for a future --no-percentiles flag
			t, err := parseSinceDate(since)
			if err != nil {
				return err
			}
			db, err := openSalestechStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := salestech.DaysToSell(db, salestech.GroupByDim(groupBy), t)
			if err != nil {
				return err
			}
			rows = capRows(rows, limit)
			if rows == nil {
				rows = []salestech.DaysToSellRow{}
			}
			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{r.Group, iN(r.N), f2(r.Min), f2(r.P50), f2(r.P90), f2(r.Max), f2(r.Avg)})
			}
			return stOutput(cmd, flags, rows,
				[]string{"GROUP", "N", "MIN", "P50", "P90", "MAX", "AVG"},
				table)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&groupBy, "group-by", "rep", "Pivot dimension: rep | businessUnit")
	cmd.Flags().StringVar(&since, "since", "90d", "Window: YYYY-MM-DD or duration")
	cmd.Flags().BoolVar(&percentiles, "percentiles", true, "Include p50/p90 percentiles (currently always on; reserved for future toggle)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows (0 = all)")
	return cmd
}

func newReportsDismissedReasonsCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		since  string
		top    int
	)
	cmd := &cobra.Command{
		Use:   "dismissed-reasons",
		Short: "Top-N exact-match group-by on dismissal reason strings from the status-change feed",
		Long: "Counts the literal reason strings from status_changes where the\n" +
			"to-status was Dismissed. Mechanical exact-match group-by — no NLP, so\n" +
			"variations of the same human meaning bucket separately. Empty/missing\n" +
			"reasons surface under '<no reason recorded>' so the user sees that\n" +
			"state rather than silently losing rows. Run 'sync-status-changes' first.",
		Example: strings.Trim(`
  servicetitan-salestech-pp-cli reports dismissed-reasons --since 90d --top 20 --json
  servicetitan-salestech-pp-cli reports dismissed-reasons --since 1y --top 50
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			t, err := parseSinceDate(since)
			if err != nil {
				return err
			}
			db, err := openSalestechStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := salestech.DismissedReasons(db, t, top)
			if err != nil {
				return err
			}
			if rows == nil {
				rows = []salestech.DismissedReasonRow{}
			}
			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{iN(r.Count), r.Reason})
			}
			return stOutput(cmd, flags, rows,
				[]string{"COUNT", "REASON"},
				table)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&since, "since", "90d", "Window: YYYY-MM-DD or duration")
	cmd.Flags().IntVar(&top, "top", 20, "Maximum buckets to return")
	return cmd
}

func newReportsPipelineCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		asOf   string
	)
	cmd := &cobra.Command{
		Use:   "pipeline",
		Short: "Total $ in each status as-of a chosen date (reconstructed from status_changes)",
		Long: "Replays the status_changes feed forward chronologically up to --as-of\n" +
			"to reconstruct what each estimate's status was on that date, then sums\n" +
			"into Open/Sold/Dismissed buckets. The current ST API only returns\n" +
			"current state; this is the as-of view it can't give. If --as-of is\n" +
			"older than the oldest status_change in the store, a `warning` field\n" +
			"flags the gap rather than silently misreporting.",
		Example: strings.Trim(`
  servicetitan-salestech-pp-cli reports pipeline --as-of 2026-05-10 --json
  servicetitan-salestech-pp-cli reports pipeline --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			var asOfTime time.Time
			if asOf == "" {
				asOfTime = time.Now().UTC()
			} else {
				t, err := time.Parse("2006-01-02", asOf)
				if err != nil {
					return fmt.Errorf("as-of must be YYYY-MM-DD: %w", err)
				}
				// Use end-of-day so a same-day request includes all of that day.
				asOfTime = t.UTC().Add(24*time.Hour - time.Nanosecond)
			}
			db, err := openSalestechStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, warning, err := salestech.PipelineSnapshot(db, asOfTime)
			if err != nil {
				return err
			}
			out := map[string]any{
				"as_of":   asOfTime.Format(time.RFC3339),
				"buckets": rows,
			}
			if warning != "" {
				out["warning"] = warning
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "as-of: %s\n", asOfTime.Format("2006-01-02"))
			if warning != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "warning: %s\n", warning)
			}
			fmt.Fprintln(cmd.OutOrStdout())
			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{r.Status, iN(r.Count), f2(r.Total)})
			}
			return stOutput(cmd, flags, rows,
				[]string{"STATUS", "COUNT", "TOTAL $"},
				table)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&asOf, "as-of", "", "As-of date YYYY-MM-DD (defaults to today)")
	return cmd
}

func newReportsSkuFrequencyCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath   string
		onStatus string
		since    string
		top      int
	)
	cmd := &cobra.Command{
		Use:   "sku-frequency",
		Short: "Top SKUs by appearance on estimates with the chosen status",
		Long: "Joins estimate_items with estimates filtered by --on (status) and\n" +
			"--since (window) and groups by sku id. Returns appearances, distinct\n" +
			"estimate count, total qty, and total $. The ST API only returns items\n" +
			"per single estimate — this is the cross-estimate view it can't give.\n" +
			"Run 'sync' + 'sync-items' first.",
		Example: strings.Trim(`
  servicetitan-salestech-pp-cli reports sku-frequency --on sold --since 90d --top 50 --json
  servicetitan-salestech-pp-cli reports sku-frequency --on dismissed --since 1y --top 25
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			t, err := parseSinceDate(since)
			if err != nil {
				return err
			}
			db, err := openSalestechStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := salestech.SkuFrequency(db, onStatus, t, top)
			if err != nil {
				return err
			}
			if rows == nil {
				rows = []salestech.SkuFreqRow{}
			}
			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{i64(r.SkuID), iN(r.Appearances), iN(r.EstimateCount), f2(r.TotalQty), f2(r.TotalDollars), r.SkuType, r.SkuDisplay})
			}
			return stOutput(cmd, flags, rows,
				[]string{"SKU ID", "APPEARS", "ON N EST", "TOTAL QTY", "TOTAL $", "TYPE", "DISPLAY NAME"},
				table)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&onStatus, "on", "Sold", "Estimate status to count appearances on (Sold|Dismissed|Open or empty for all)")
	cmd.Flags().StringVar(&since, "since", "90d", "Window for estimate createdOn")
	cmd.Flags().IntVar(&top, "top", 50, "Maximum SKUs to return")
	return cmd
}

func newReportsFollowUpsCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath string
		rep    string
		since  string
		limit  int
		tenant string
	)
	cmd := &cobra.Command{
		Use:   "follow-ups",
		Short: "Per-rep open estimates from the last N hours — today's call list with deeplinks",
		Long: "Returns Open estimates created in the last --since window, grouped by\n" +
			"rep (soldById) — the daily 'who needs a callback' work queue. Each row\n" +
			"carries customerId, jobNumber, total $, and a ServiceTitan web deep-\n" +
			"link. To enrich with customer phone numbers, pipe customer_id into the\n" +
			"sibling `servicetitan-crm-pp-cli customers get <id>` (see the README\n" +
			"recipes).",
		Example: strings.Trim(`
  servicetitan-salestech-pp-cli reports follow-ups --rep all --since 48h --json
  servicetitan-salestech-pp-cli reports follow-ups --rep 1234 --since 7d --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			d, err := parseAgeDuration(since)
			if err != nil {
				return err
			}
			repID, err := parseRepFilter(rep)
			if err != nil {
				return err
			}
			db, err := openSalestechStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			tID := resolveTenant(tenant)
			rows, err := salestech.FollowUps(db, repID, d, tID)
			if err != nil {
				return err
			}
			rows = capRows(rows, limit)
			if rows == nil {
				rows = []salestech.FollowUpCallRow{}
			}
			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{i64(r.SoldByID), iN(r.AgeHours), i64(r.EstimateID), r.JobNumber, i64(r.CustomerID), f2(r.Total), r.Name, r.Deeplink})
			}
			return stOutput(cmd, flags, rows,
				[]string{"REP", "AGE HRS", "EST ID", "JOB", "CUST ID", "TOTAL", "NAME", "LINK"},
				table)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().StringVar(&rep, "rep", "all", "Rep filter: 'all' / empty / 0 for every rep, or a numeric soldById to filter to one rep")
	cmd.Flags().StringVar(&since, "since", "48h", "Window relative to now (e.g. 48h, 7d)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows (0 = all)")
	cmd.Flags().StringVar(&tenant, "tenant", "", "Tenant id for deeplinks (defaults to ST_TENANT_ID)")
	return cmd
}

// parseRepFilter accepts "all" / "" / "0" → 0 (every rep), or a positive
// integer → that rep id. Returns an error for non-integer non-keyword values.
func parseRepFilter(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" || strings.EqualFold(s, "all") || s == "0" {
		return 0, nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("--rep must be 'all', 0, or a positive integer soldById (got %q)", s)
	}
	return n, nil
}
