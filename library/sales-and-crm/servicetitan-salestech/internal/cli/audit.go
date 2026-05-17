package cli

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/servicetitan-salestech/internal/salestech"
)

func newAuditCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit",
		Short: "Single-estimate forensic + cross-estimate timeline + follow-up log",
		Long:  "Subcommands for forensic lookup: full single-estimate envelope, recent-changes sweep, and local follow-up note logging.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newAuditEstimateCmd(flags))
	cmd.AddCommand(newAuditRecentChangesCmd(flags))
	cmd.AddCommand(newAuditFollowUpCmd(flags))
	cmd.AddCommand(newAuditFollowUpsListCmd(flags))
	return cmd
}

func newAuditEstimateCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:   "estimate <id>",
		Short: "Full forensic envelope for one estimate: header + items + status timeline + local follow-ups",
		Long: "Joins the parent estimate row with every line item, the full status\n" +
			"change timeline (oldest first), and any locally-recorded follow-up notes\n" +
			"into a single shaped JSON object. The 'what happened with quote X' call.\n" +
			"Run 'sync' + 'sync-items' + 'sync-status-changes' first for full detail.",
		Example: strings.Trim(`
  servicetitan-salestech-pp-cli audit estimate 78421 --json
  servicetitan-salestech-pp-cli audit estimate 78421 --json --select estimate.id,items.sku.displayName,status_changes
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil || id <= 0 {
				return fmt.Errorf("estimate id must be a positive integer, got %q", args[0])
			}
			db, err := openSalestechStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			out, err := salestech.Audit(db, id)
			if err != nil {
				return err
			}
			// Render compact human table OR JSON envelope.
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Estimate %d  %s\n", out.Estimate.ID, out.Estimate.Name)
			fmt.Fprintf(cmd.OutOrStdout(), "  job=%v  customer=%v  status=%s  total=$%.2f\n",
				ptrInt(out.Estimate.JobID), ptrInt(out.Estimate.CustomerID), out.Estimate.Status.String(), out.Estimate.Total())
			fmt.Fprintf(cmd.OutOrStdout(), "  created=%s  modified=%s  soldOn=%s\n",
				out.Estimate.CreatedOn, out.Estimate.ModifiedOn, ptrStr(out.Estimate.SoldOn))
			fmt.Fprintf(cmd.OutOrStdout(), "\nLine items (%d):\n", len(out.Items))
			for _, it := range out.Items {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s  qty=%.2f  rate=$%.2f  total=$%.2f\n",
					it.Sku.DisplayName, it.Qty, it.UnitRate, it.Total)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "\nStatus timeline (%d):\n", len(out.StatusChanges))
			for _, c := range out.StatusChanges {
				reason := c.Reason
				if reason != "" {
					reason = " — " + reason
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s  %s -> %s  by employee=%d%s\n",
					c.ChangedAt, c.From, c.To, c.ChangedByID, reason)
			}
			if len(out.FollowUps) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\nLocal follow-ups (%d):\n", len(out.FollowUps))
				for _, f := range out.FollowUps {
					rem := ""
					if f.RemindOn != "" {
						rem = "  remind=" + f.RemindOn
					}
					fmt.Fprintf(cmd.OutOrStdout(), "  %s%s  %s\n", f.CreatedAt, rem, f.Note)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/servicetitan-salestech-pp-cli/data.db)")
	return cmd
}

func newAuditRecentChangesCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath   string
		since    string
		toStatus string
		limit    int
	)
	cmd := &cobra.Command{
		Use:   "recent-changes",
		Short: "Every estimate whose status changed in the window — from → to, actor, total $",
		Long: "Sweeps the local status_changes feed for transitions whose changedAt\n" +
			"falls within --since before now, joined to the parent estimate header.\n" +
			"--to-status narrows to transitions whose `to` matches (case-insensitive)\n" +
			"— pipe `audit recent-changes --to-status Unsold` to surface revenue\n" +
			"reversals. Run 'sync-status-changes' first.",
		Example: strings.Trim(`
  servicetitan-salestech-pp-cli audit recent-changes --since 24h --json
  servicetitan-salestech-pp-cli audit recent-changes --since 7d --to-status Sold --json
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
			if d == 0 {
				d = 24 * time.Hour
			}
			db, err := openSalestechStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := salestech.RecentChanges(db, d, toStatus)
			if err != nil {
				return err
			}
			rows = capRows(rows, limit)
			if rows == nil {
				rows = []salestech.RecentChangeRow{}
			}
			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{
					r.ChangedAt, i64(r.EstimateID), r.JobNumber, r.From, r.To, i64(r.ChangedByID), f2(r.Total), r.Reason,
				})
			}
			return stOutput(cmd, flags, rows,
				[]string{"CHANGED AT", "ID", "JOB", "FROM", "TO", "BY", "TOTAL", "REASON"},
				table)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/servicetitan-salestech-pp-cli/data.db)")
	cmd.Flags().StringVar(&since, "since", "24h", "Window relative to now (e.g. 24h, 7d, 30d)")
	cmd.Flags().StringVar(&toStatus, "to-status", "", "Limit to transitions whose `to` matches (Open|Sold|Dismissed|Unsold; case-insensitive)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows (0 = all)")
	return cmd
}

func newAuditFollowUpCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "follow-up",
		Short: "Local follow-up notes attached to estimates",
		Long:  "Subcommands for the local follow-up log (the ServiceTitan API has no estimate-notes endpoint).",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newAuditFollowUpAddCmd(flags))
	return cmd
}

func newAuditFollowUpAddCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath    string
		note      string
		remindOn  string
		createdBy string
	)
	cmd := &cobra.Command{
		Use:   "add <estimate-id>",
		Short: "Add a local follow-up note (and optional reminder date) to one estimate",
		Long: "Writes a follow-up note into the local SQLite store under the estimate\n" +
			"id. The ST API has no estimate-notes endpoint, so this is local-only;\n" +
			"`audit follow-ups --due-by <date>` reads back. Pair with `reports\n" +
			"follow-ups` for the rep-grouped call list.",
		Example: strings.Trim(`
  servicetitan-salestech-pp-cli audit follow-up add 78421 --note "customer wants to talk Monday" --remind 2026-05-20
  servicetitan-salestech-pp-cli audit follow-up add 78421 --note "left voicemail" --created-by pierce
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil || id <= 0 {
				return fmt.Errorf("estimate id must be a positive integer, got %q", args[0])
			}
			db, err := openSalestechStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			fu, err := salestech.AddFollowUp(db, id, note, remindOn, createdBy)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), fu, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/servicetitan-salestech-pp-cli/data.db)")
	cmd.Flags().StringVar(&note, "note", "", "Required: the follow-up note text")
	cmd.Flags().StringVar(&remindOn, "remind", "", "Optional reminder date (YYYY-MM-DD); surfaced by `audit follow-ups --due-by`")
	cmd.Flags().StringVar(&createdBy, "created-by", "", "Optional: who logged the follow-up (e.g. username)")
	return cmd
}

func newAuditFollowUpsListCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath     string
		estimateID int64
		dueBy      string
		limit      int
	)
	cmd := &cobra.Command{
		Use:   "follow-ups",
		Short: "List locally-stored follow-up notes, optionally filtered by estimate id or reminder due date",
		Long: "Reads the local follow-up log. Without --due-by, returns every follow-up\n" +
			"in chronological order; with --due-by YYYY-MM-DD, returns only follow-ups\n" +
			"whose reminder is on or before that date — the 'who needs a callback this\n" +
			"week' query.",
		Example: strings.Trim(`
  servicetitan-salestech-pp-cli audit follow-ups
  servicetitan-salestech-pp-cli audit follow-ups --due-by 2026-05-20 --json
  servicetitan-salestech-pp-cli audit follow-ups --estimate 78421
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := openSalestechStore(cmd, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			rows, err := salestech.ListFollowUps(db, estimateID, dueBy)
			if err != nil {
				return err
			}
			rows = capRows(rows, limit)
			if rows == nil {
				rows = []salestech.FollowUp{}
			}
			table := make([][]string, 0, len(rows))
			for _, r := range rows {
				table = append(table, []string{r.RemindOn, r.CreatedAt, i64(r.EstimateID), r.CreatedBy, r.Note})
			}
			return stOutput(cmd, flags, rows,
				[]string{"REMIND", "CREATED AT", "ESTIMATE", "BY", "NOTE"},
				table)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/servicetitan-salestech-pp-cli/data.db)")
	cmd.Flags().Int64Var(&estimateID, "estimate", 0, "Filter to one estimate id (0 = all)")
	cmd.Flags().StringVar(&dueBy, "due-by", "", "Return follow-ups whose reminder is on or before this date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum rows (0 = all)")
	return cmd
}

func ptrInt(p *int64) any {
	if p == nil {
		return "-"
	}
	return *p
}

func ptrStr(p *string) string {
	if p == nil {
		return "-"
	}
	return *p
}
