// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newTBCalendarCmd(flags))
	})
}

type tbEventDoc struct {
	ID         string `json:"id"`
	CalendarID string `json:"calendar_id"`
	Title      string `json:"title"`
	Start      string `json:"start"`
	End        string `json:"end"`
	Location   string `json:"location"`
}

func newTBCalendarCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "calendar",
		Short:       "Events from the local Thunderbird calendar",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:parent-group": "true", "pp:typed-exit-codes": "0,2", "pp:data-source": "local"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newTBCalendarEventsCmd(flags))
	return cmd
}

func newTBCalendarEventsCmd(flags *rootFlags) *cobra.Command {
	var since, until string
	var limit int
	cmd := &cobra.Command{
		Use:   "events",
		Short: "List calendar events in a time window, earliest first",
		Long: `List events of the local calendar (calendar-data/local.sqlite) ordered by
start time. --since takes a past duration (30d) or a date; --until takes a
future duration (14d, counted from now) or a date. Network calendars that
Thunderbird does not cache locally are not included.`,
		Example: strings.Trim(`
  thunderbird-pp-cli calendar events
  thunderbird-pp-cli calendar events --since 30d --until 14d --json
  thunderbird-pp-cli calendar events --since 2025-01-01 --until 2025-03-31`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "calendar events")
			}
			now := time.Now()
			from, err := tbParseTimeBound(since, now, false)
			if err != nil {
				return usageErr(fmt.Errorf("--since: %w", err))
			}
			to, err := tbParseTimeBound(until, now, true)
			if err != nil {
				return usageErr(fmt.Errorf("--until: %w", err))
			}
			if len(strings.TrimSpace(until)) == len("2006-01-02") && !to.IsZero() {
				to = to.Add(24*time.Hour - time.Nanosecond)
			}
			db, err := tbStoreFor(cmd, flags, "events")
			if err != nil || db == nil {
				return err
			}
			docs, err := tbLoadDocs[tbEventDoc](db, "events")
			_ = db.Close()
			if err != nil {
				return err
			}
			rows := make([]tbEventDoc, 0, len(docs))
			for _, e := range docs {
				start, perr := time.Parse(time.RFC3339, e.Start)
				if (!from.IsZero() || !to.IsZero()) && perr != nil {
					continue
				}
				if !from.IsZero() && start.Before(from) || !to.IsZero() && start.After(to) {
					continue
				}
				rows = append(rows, e)
			}
			sort.Slice(rows, func(i, j int) bool {
				if rows[i].Start != rows[j].Start {
					return rows[i].Start < rows[j].Start
				}
				return rows[i].ID < rows[j].ID
			})
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			tw := newTabWriter(tbHumanOut(cmd))
			fmt.Fprintln(tw, "START\tEND\tTITLE\tLOCATION")
			for _, e := range rows {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", tbShortDate(e.Start), tbShortDate(e.End), e.Title, e.Location)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&since, "since", "", "Only events starting after a past duration (30d) or a date (2025-01-01)")
	cmd.Flags().StringVar(&until, "until", "", "Only events starting before a future duration (14d) or a date (2025-03-31)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Maximum events to show (0 = all)")
	return cmd
}
