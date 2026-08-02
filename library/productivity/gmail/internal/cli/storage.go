// Copyright 2026 Rahul Bansal and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel feature: storage hogs. Gmail exposes only a single total-quota number;
// this ranks mailbox bytes by sender/year/attachments from the local mirror and
// pairs every group with the exact cleanup command to reclaim it.
// generate --force preserves implemented bodies; untouched TODO scaffolds may refresh.
// pp:data-source local
package cli

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type storageRow struct {
	Group      string `json:"group"`
	Count      int    `json:"count"`
	TotalBytes int64  `json:"total_bytes"`
	Human      string `json:"human_size"`
	// CleanupQuery is the Gmail query matching this group. CleanupArgv is the
	// cleanup invocation as an argv slice rather than a shell string: the
	// group name comes from a sender-controlled From header, and a shell
	// string would let `$(...)` in an address execute when a user or agent
	// runs the suggestion.
	CleanupQuery string   `json:"cleanup_query"`
	CleanupArgv  []string `json:"cleanup_argv,omitempty"`
}

// plausibleAddr gates which group names may be echoed into a suggested
// command at all.
var plausibleAddr = regexp.MustCompile(`^[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+$`)

func newNovelStorageCmd(flags *rootFlags) *cobra.Command {
	var groupBy string
	var limit int
	var minMB int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "storage",
		Short: "Rank mailbox storage by sender or year, with ready-made cleanup commands",
		Long: `Aggregates message sizes from the local mirror and prints, for every
group, the Gmail query and the CLI command that would clean it up. Nothing
is deleted; you review and run the suggestions yourself.`,
		Example: strings.Trim(`
  gmail-pp-cli storage --group-by sender
  gmail-pp-cli storage --group-by year --agent
  gmail-pp-cli storage --group-by sender --min-mb 50`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "aggregate local mirror storage usage")
			}
			var groupExpr, labelFmt string
			switch groupBy {
			case "sender":
				groupExpr = `COALESCE(json_extract(data,'$.from_email'),'(unknown)')`
				labelFmt = "from:%s"
			case "year":
				groupExpr = `COALESCE(substr(json_extract(data,'$.internal_ts'),1,4),'(unknown)')`
				labelFmt = "after:%s/01/01"
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("invalid --group-by %q: must be sender or year", groupBy))
			}
			db, err := openGmailStore(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if !hintIfUnsynced(cmd, db, "messages") {
				hintIfStale(cmd, db, "messages", flags.maxAge)
			}
			rows, err := db.DB().QueryContext(cmd.Context(), fmt.Sprintf(`
				SELECT %s AS grp, COUNT(*), COALESCE(SUM(size_estimate),0) AS bytes
				FROM messages
				GROUP BY grp
				HAVING bytes >= ?
				ORDER BY bytes DESC
				LIMIT ?`, groupExpr), int64(minMB)*1024*1024, limit)
			if err != nil {
				return fmt.Errorf("querying storage: %w", err)
			}
			defer rows.Close()
			var out []storageRow
			for rows.Next() {
				var r storageRow
				if err := rows.Scan(&r.Group, &r.Count, &r.TotalBytes); err != nil {
					return fmt.Errorf("scanning storage row: %w", err)
				}
				r.Human = humanBytes(r.TotalBytes)
				switch groupBy {
				case "sender":
					// Only well-formed addresses become a suggestion; anything
					// else is reported without an actionable command.
					if plausibleAddr.MatchString(r.Group) {
						r.CleanupQuery = fmt.Sprintf(labelFmt, r.Group)
					}
				case "year":
					if y, err := strconv.Atoi(r.Group); err == nil && len(r.Group) == 4 {
						// before: is exclusive, so bound with Jan 1 of the
						// next year to include Dec 31 of this one.
						r.CleanupQuery = fmt.Sprintf(labelFmt, r.Group) + fmt.Sprintf(" before:%d/01/01", y+1)
					}
				}
				if r.CleanupQuery != "" {
					r.CleanupArgv = []string{"gmail-pp-cli", "label", "--query", r.CleanupQuery, "--remove", "INBOX"}
				}
				out = append(out, r)
			}
			if err := rows.Err(); err != nil {
				return err
			}
			if wantsJSONOutput(cmd, flags) {
				if out == nil {
					out = []storageRow{}
				}
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no groups over the threshold; is the local mirror populated? (gmail-pp-cli pull)")
				return nil
			}
			tw := tabwriter.NewWriter(cmd.OutOrStdout(), 2, 4, 2, ' ', 0)
			fmt.Fprintln(tw, "SIZE\tCOUNT\tGROUP\tCLEANUP QUERY")
			for _, r := range out {
				fmt.Fprintf(tw, "%s\t%d\t%s\t%s\n", r.Human, r.Count, truncateCell(r.Group, 40), r.CleanupQuery)
			}
			tw.Flush()
			fmt.Fprintln(cmd.OutOrStdout(), "\nreview a group live:  gmail-pp-cli find \"<cleanup query> larger:1M\"")
			return nil
		},
	}
	cmd.Flags().StringVar(&groupBy, "group-by", "sender", "Aggregation: sender or year")
	cmd.Flags().IntVar(&limit, "limit", 20, "Number of groups to show")
	cmd.Flags().IntVar(&minMB, "min-mb", 0, "Hide groups smaller than this many megabytes")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: resolved data directory data.db)")
	return cmd
}
