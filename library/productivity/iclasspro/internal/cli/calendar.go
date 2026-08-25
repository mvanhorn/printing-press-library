// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/iclasspro/internal/icp"

	"github.com/spf13/cobra"
)

type calendarResult struct {
	Account  string `json:"account"`
	Format   string `json:"format"`
	Events   int    `json:"events"`
	Entities int    `json:"entities"`
	Skipped  int    `json:"entities_without_dates"`
	Out      string `json:"out,omitempty"`
	Note     string `json:"note,omitempty"`
}

func newNovelCalendarCmd(flags *rootFlags) *cobra.Command {
	var (
		format string
		out    string
		dbPath string
		kinds  string
	)

	cmd := &cobra.Command{
		Use:   "calendar [account]",
		Short: "Export synced camps and classes to an RFC 5545 calendar file, one event per session.",
		Long: strings.Trim(`
Render the local mirror as an iCalendar feed.

iClassPro publishes no feed in any format, so gyms and the agencies that build
their websites re-derive this by hand. Each dated session becomes one VEVENT
carrying the program, age band, availability, instructors, and the portal
registration deep link.

Entities with no dated session are counted and reported rather than silently
dropped.`, "\n"),
		Example: "  iclasspro-pp-cli calendar scaq --format ics --out fall-camps.ics",
		// Deliberately NOT mcp:read-only: --out writes an arbitrary caller-supplied
		// path to disk. Without --out the command only prints, but a static
		// annotation cannot express that, and the unsafe direction is the one that
		// matters — an MCP host must prompt before this tool can overwrite a file.
		Annotations: map[string]string{"pp:happy-args": "<account>=scaq"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "calendar")
			}
			account, err := icpRequireAccount(args)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			if f := strings.ToLower(strings.TrimSpace(format)); f != "" && f != "ics" && f != "ical" {
				return usageErr(fmt.Errorf("--format accepts ics (got %q)", format))
			}
			wantKinds := icpParseKinds(kinds)
			if wantKinds == nil {
				return usageErr(fmt.Errorf("--kinds accepts class, camp, or both"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			path := icpDBPath(dbPath)
			res := calendarResult{Account: account, Format: "ics"}
			s, ok, err := icpOpenStoreForRead(ctx, path)
			if err != nil {
				return err
			}
			if !ok {
				res.Note = fmt.Sprintf("no local mirror yet; run 'iclasspro-pp-cli sync %s'", account)
				return icpNoLocalData(cmd, flags, res, account)
			}
			defer func() { _ = s.Close() }()
			icpStaleHint(ctx, cmd.ErrOrStderr(), s, flags, account)

			raw, err := icpLatestEntities(ctx, s, account)
			if err != nil {
				return err
			}
			ents := icpFilterKinds(raw, wantKinds)
			if len(ents) == 0 {
				res.Note = fmt.Sprintf("no synced entities for %s", account)
				return icpNoLocalData(cmd, flags, res, account)
			}
			res.Entities = len(ents)

			ics, skipped := icp.RenderICS(ents, icpNow())
			res.Skipped = skipped
			res.Events = strings.Count(ics, "BEGIN:VEVENT")
			if skipped > 0 {
				res.Note = fmt.Sprintf("%d of %d entities have no dated session and produced no events", skipped, len(ents))
			}

			if strings.TrimSpace(out) != "" {
				if err := icpEnsureParentDir(out); err != nil {
					return err
				}
				if err := os.WriteFile(out, []byte(ics), 0o600); err != nil {
					return fmt.Errorf("writing %s: %w", out, err)
				}
				res.Out = out
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), res, flags)
				}
				if res.Note != "" {
					fmt.Fprintln(cmd.ErrOrStderr(), "note:", res.Note)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Wrote %d events from %d entities to %s\n", res.Events, res.Entities, out)
				return nil
			}

			// Without --out the calendar itself is the payload, so it goes to
			// stdout verbatim. Machine formats still get the structured envelope
			// so an agent is never handed a non-JSON body it did not expect.
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				type withBody struct {
					calendarResult
					Calendar string `json:"calendar"`
				}
				return printJSONFiltered(cmd.OutOrStdout(), withBody{calendarResult: res, Calendar: ics}, flags)
			}
			if res.Note != "" {
				fmt.Fprintln(cmd.ErrOrStderr(), "note:", res.Note)
			}
			_, err = cmd.OutOrStdout().Write([]byte(ics))
			return err
		},
	}

	cmd.Flags().StringVar(&format, "format", "ics", "Output format (ics)")
	cmd.Flags().StringVar(&out, "out", "", "Write the calendar to this file instead of stdout")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path")
	cmd.Flags().StringVar(&kinds, "kinds", "both", "Which entities to export: class, camp, or both")
	return cmd
}
