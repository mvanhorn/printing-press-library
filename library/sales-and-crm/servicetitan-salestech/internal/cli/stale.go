package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-salestech/internal/salestech"
)

func newStaleEstimatesCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath    string
		olderThan string
		status    string
		limit     int
	)
	cmd := &cobra.Command{
		Use:   "stale",
		Short: "List Open estimates older than N days, ranked by age × total $",
		Long: "Surfaces stuck quotes — Open estimates whose createdOn is at or before\n" +
			"--older-than days ago, ranked by (ageDays × total $) descending so the\n" +
			"biggest-dollar quotes that have been stuck longest come first. Use to\n" +
			"build a stale-quote work queue for pipeline review. Run 'sync' first.",
		Example: strings.Trim(`
  servicetitan-salestech-pp-cli estimates stale
  servicetitan-salestech-pp-cli estimates stale --older-than 7 --json --limit 25
  servicetitan-salestech-pp-cli estimates stale --status Sold --older-than 30
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			// Accept either a bare integer (days, e.g. "3") or a duration suffix
			// (e.g. "3d", "48h"). Internally Stale takes integer days.
			days, err := parseOlderThanDays(olderThan)
			if err != nil {
				return err
			}
			db, err := openSalestechStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := salestech.Stale(db, status, days)
			if err != nil {
				return err
			}
			rows = capRows(rows, limit)
			if rows == nil {
				rows = []salestech.StaleRow{}
			}
			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{
					iN(r.AgeDays), i64(r.ID), r.JobNumber, r.Status, f2(r.Total), f2(r.Priority), i64(r.SoldByID), r.Name,
				})
			}
			return stOutput(cmd, flags, rows,
				[]string{"AGE", "ID", "JOB", "STATUS", "TOTAL", "PRIORITY", "SOLD BY", "NAME"},
				table)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/servicetitan-salestech-pp-cli/data.db)")
	cmd.Flags().StringVar(&olderThan, "older-than", "3d", "Minimum age — bare integer (days) or duration suffix (e.g. 3d, 48h, 1w); 0 returns every matching estimate")
	cmd.Flags().StringVar(&status, "status", "Open", "Status to filter on (Open|Sold|Dismissed; case-insensitive)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows to return (0 = all)")
	return cmd
}

// parseOlderThanDays accepts a bare integer (interpreted as days), a duration
// suffix (`3d`, `48h`, `1w`), or "0"/"" → 0 days.
func parseOlderThanDays(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "0" {
		return 0, nil
	}
	if n, err := strconv.Atoi(s); err == nil {
		return n, nil
	}
	d, err := parseAgeDuration(s)
	if err != nil {
		return 0, fmt.Errorf("--older-than must be an integer (days), a duration like 3d/48h/1w, or 0: %w", err)
	}
	return int(d.Hours() / 24), nil
}
