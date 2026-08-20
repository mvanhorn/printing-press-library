package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-ads/internal/adsanalytics"
	"github.com/mvanhorn/printing-press-library/library/commerce/amazon-ads/internal/store"
	"github.com/spf13/cobra"
)

func newBidOptimizerCmd(flags *rootFlags) *cobra.Command {
	var reportPath string
	var targetACOS float64
	var reportKind string
	var allowPartial bool

	cmd := &cobra.Command{
		Use:   "bid-optimizer",
		Short: "Recommend keyword bid changes from keyword performance reports",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if reportPath == "" {
				return usageErr(fmt.Errorf("--report is required"))
			}
			rows, schemaReport, err := loadKeywordRowsForCommand(cmd, reportPath, reportLoadOptions{ReportKind: reportKind, AllowPartial: allowPartial, Command: "bid-optimizer"})
			if err != nil {
				return err
			}
			recs := adsanalytics.BidOptimizer(rows, targetACOS)
			out := map[string]any{
				"report":          reportPath,
				"target_acos":     targetACOS,
				"recommendations": recs,
				"count":           len(recs),
			}
			if schemaReport.Kind != "" {
				out["report_kind"] = schemaReport.Kind
				out["detected_candidates"] = schemaReport.Validation.Candidates
			}
			return printCommandJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&reportPath, "report", "", "Path to a keyword performance CSV or JSON export")
	cmd.Flags().Float64Var(&targetACOS, "target-acos", 25, "Target ACOS percentage")
	cmd.Flags().StringVar(&reportKind, "report-kind", "", "Explicit report schema kind (see reports recipe bid-optimizer)")
	cmd.Flags().BoolVar(&allowPartial, "allow-partial", false, "Allow missing schema columns with a warning")
	return cmd
}

func loadKeywordRowsForCommand(cmd *cobra.Command, reportPath string, opts reportLoadOptions) ([]adsanalytics.KeywordPerformance, adsanalytics.NormalizedSchemaReport, error) {
	if reportPath == "" {
		return nil, adsanalytics.NormalizedSchemaReport{}, usageErr(fmt.Errorf("--report is required"))
	}
	if opts.ReportKind == "" && !opts.AllowPartial {
		rows, err := adsanalytics.LoadKeywordPerformanceReport(reportPath)
		return rows, adsanalytics.NormalizedSchemaReport{}, err
	}
	rows, report, err := loadSchemaKeywordReport(cmd, reportPath, opts)
	return rows, report, err
}

func newKeywordDecayCmd(flags *rootFlags) *cobra.Command {
	var baselinePath string
	var currentPath string
	var threshold float64
	var minSpend float64

	cmd := &cobra.Command{
		Use:   "keyword-decay",
		Short: "Find keywords whose ACOS or conversion rate degraded between two snapshots",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if baselinePath == "" {
				return usageErr(fmt.Errorf("--baseline is required"))
			}
			if currentPath == "" {
				return usageErr(fmt.Errorf("--current is required"))
			}
			baseline, err := adsanalytics.LoadKeywordPerformanceReport(baselinePath)
			if err != nil {
				return err
			}
			current, err := adsanalytics.LoadKeywordPerformanceReport(currentPath)
			if err != nil {
				return err
			}
			findings := adsanalytics.KeywordDecay(baseline, current, threshold, minSpend)
			out := map[string]any{
				"baseline":              baselinePath,
				"current":               currentPath,
				"degradation_threshold": threshold,
				"min_spend":             minSpend,
				"findings":              findings,
				"count":                 len(findings),
			}
			return printCommandJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&baselinePath, "baseline", "", "Path to the older keyword performance CSV/JSON snapshot")
	cmd.Flags().StringVar(&currentPath, "current", "", "Path to the newer keyword performance CSV/JSON snapshot")
	cmd.Flags().Float64Var(&threshold, "degradation-threshold", 30, "Minimum percent ACOS increase or CVR drop")
	cmd.Flags().Float64Var(&minSpend, "min-spend", 0, "Minimum current spend before reporting decay")
	return cmd
}

func newKeywordLifecycleCmd(flags *rootFlags) *cobra.Command {
	var reportPath string
	var targetACOS float64

	cmd := &cobra.Command{
		Use:   "keyword-lifecycle",
		Short: "Classify keywords into discovery, graduation, maturity, decline, or neglected stages",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if reportPath == "" {
				return usageErr(fmt.Errorf("--report is required"))
			}
			rows, err := adsanalytics.LoadKeywordPerformanceReport(reportPath)
			if err != nil {
				return err
			}
			stages := adsanalytics.KeywordLifecycle(rows, targetACOS)
			out := map[string]any{
				"report":      reportPath,
				"target_acos": targetACOS,
				"keywords":    stages,
				"count":       len(stages),
			}
			return printCommandJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&reportPath, "report", "", "Path to a keyword performance CSV or JSON export")
	cmd.Flags().Float64Var(&targetACOS, "target-acos", 25, "Target ACOS percentage")
	return cmd
}

func newBidHistoryCmd(flags *rootFlags) *cobra.Command {
	var reportPath string
	var keyword string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "bid-history",
		Short: "Show bid, CPC, conversion, and ACOS history points for a keyword",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if keyword == "" {
				return usageErr(fmt.Errorf("--keyword is required"))
			}
			var rows []adsanalytics.KeywordPerformance
			source := reportPath
			if reportPath != "" {
				loaded, err := adsanalytics.LoadKeywordPerformanceReport(reportPath)
				if err != nil {
					return err
				}
				rows = loaded
			} else {
				if dbPath == "" {
					dbPath = defaultDBPath("amazon-ads-pp-cli")
				}
				db, err := store.OpenWithContext(cmd.Context(), dbPath)
				if err != nil {
					return fmt.Errorf("opening local database: %w", err)
				}
				defer db.Close()
				rawRows, err := db.KeywordHistory(cmd.Context(), keyword)
				if err != nil {
					return err
				}
				for _, raw := range rawRows {
					var row adsanalytics.KeywordPerformance
					if err := json.Unmarshal(raw, &row); err != nil {
						return fmt.Errorf("parsing stored keyword history: %w", err)
					}
					rows = append(rows, row)
				}
				source = dbPath
			}
			points := adsanalytics.BidHistory(rows, keyword)
			out := map[string]any{
				"source":  source,
				"keyword": keyword,
				"history": points,
				"count":   len(points),
			}
			return printCommandJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&reportPath, "report", "", "Path to a keyword performance CSV or JSON export")
	cmd.Flags().StringVar(&keyword, "keyword", "", "Keyword to extract history for")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite store path for local keyword snapshots")
	return cmd
}

func newKeywordSnapshotsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "keyword-snapshots",
		Short: "Import and list keyword performance snapshots in the local store",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newKeywordSnapshotsImportCmd(flags))
	cmd.AddCommand(newKeywordSnapshotsListCmd(flags))
	return cmd
}

func newKeywordSnapshotsImportCmd(flags *rootFlags) *cobra.Command {
	var reportPath string
	var name string
	var snapshotAtRaw string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import a keyword performance report as a local historical snapshot",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if reportPath == "" {
				return usageErr(fmt.Errorf("--report is required"))
			}
			rows, err := adsanalytics.LoadKeywordPerformanceReport(reportPath)
			if err != nil {
				return err
			}
			snapshotAt, err := parseSnapshotTime(snapshotAtRaw)
			if err != nil {
				return usageErr(err)
			}
			rawRows := make([]json.RawMessage, 0, len(rows))
			for _, row := range rows {
				data, err := json.Marshal(row)
				if err != nil {
					return fmt.Errorf("encoding keyword row: %w", err)
				}
				rawRows = append(rawRows, data)
			}
			if dbPath == "" {
				dbPath = defaultDBPath("amazon-ads-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			meta, err := db.ImportKeywordSnapshot(cmd.Context(), "", name, reportPath, snapshotAt, rawRows)
			if err != nil {
				return err
			}
			out := map[string]any{
				"snapshot": meta,
				"db":       dbPath,
			}
			return printCommandJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&reportPath, "report", "", "Path to a keyword performance CSV or JSON export")
	cmd.Flags().StringVar(&name, "name", "", "Optional snapshot label")
	cmd.Flags().StringVar(&snapshotAtRaw, "snapshot-at", "", "Snapshot timestamp (RFC3339 or YYYY-MM-DD); default now")
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite store path")
	return cmd
}

func newKeywordSnapshotsListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List imported keyword performance snapshots",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dbPath == "" {
				dbPath = defaultDBPath("amazon-ads-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()
			snapshots, err := db.ListKeywordSnapshots(cmd.Context())
			if err != nil {
				return err
			}
			out := map[string]any{
				"db":        dbPath,
				"snapshots": snapshots,
				"count":     len(snapshots),
			}
			return printCommandJSON(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite store path")
	return cmd
}

func parseSnapshotTime(raw string) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Now().UTC(), nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02"} {
		if ts, err := time.Parse(layout, raw); err == nil {
			return ts.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("--snapshot-at must be RFC3339 or YYYY-MM-DD")
}
