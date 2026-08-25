// Copyright 2026 qazmataz and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored absorbed command: fetch one job by id (or first keyword match).
// Preserved across `generate --force`.
// pp:data-source live

package cli

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <id>",
		Short: "Fetch one Amazon job by its numeric id (or the first keyword match)",
		Long: strings.Trim(`
Fetch the full record for a single Amazon job. The argument is normally the
numeric id_icims (as shown by 'find'), but any keyword also works and returns
the first match. Use --plain to strip HTML from the description and
qualifications.`, "\n"),
		Example: strings.Trim(`
  amazon-jobs-pp-cli get 10483922
  amazon-jobs-pp-cli get 10483922 --plain
  amazon-jobs-pp-cli get "software engineer" --json`, "\n"),
		// no-error-path-probe: amazon.jobs returns HTTP 200 + an empty jobs
		// array for any unmatched query, and this command accepts an id OR a
		// keyword, so a not-found result is indistinguishable from a valid
		// empty search — it is exit 0, not an error.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch one job")
				return nil
			}
			if err := guardDataSource(flags, "live"); err != nil {
				return err
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a job id (or keyword) is required"))
			}
			arg := strings.TrimSpace(strings.Join(args, " "))

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			values := buildSearchValues(arg, "", "", "", "relevant", 1, 0)
			_, raw, ferr := searchPage(ctx, c, values)
			if ferr != nil {
				return classifyAPIError(ferr, flags)
			}
			if len(raw) == 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "no job found for %q\n", arg)
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "null")
				}
				return nil
			}
			job, perr := parseJob(raw[0])
			if perr != nil {
				return fmt.Errorf("parsing job: %w", perr)
			}
			// Always convert HTML to readable text (raw <br/> and anchors are
			// never useful); the raw `postings search` endpoint stays faithful.
			job = cleanJob(job)
			job.UpdatedDiverged = updatedDiverged(job, time.Now())
			return emitLiveResult(cmd, flags, job, func(w io.Writer) {
				printJobDetail(w, job)
			})
		},
	}
	return cmd
}

// printJobDetail renders the full human view of one job.
func printJobDetail(w io.Writer, j Job) {
	id := j.IDIcims
	if id == "" {
		id = j.ID
	}
	fmt.Fprintf(w, "%s\n", j.Title)
	fmt.Fprintf(w, "id: %s\n", id)
	if j.Location != "" {
		fmt.Fprintf(w, "location: %s\n", j.Location)
	}
	if j.JobCategory != "" {
		fmt.Fprintf(w, "category: %s\n", j.JobCategory)
	}
	if j.Team.Label != "" {
		fmt.Fprintf(w, "team: %s\n", j.Team.Label)
	}
	if j.JobScheduleType != "" {
		fmt.Fprintf(w, "schedule: %s\n", j.JobScheduleType)
	}
	if j.PostedDate != "" {
		fmt.Fprintf(w, "posted: %s\n", j.PostedDate)
	}
	// updated_time is shown next to posted so the pair can be compared. It is
	// an edit/re-index clock, not a posting time -- see updatedDiverged.
	if j.UpdatedTime != "" {
		fmt.Fprintf(w, "updated: %s ago", j.UpdatedTime)
		if j.UpdatedDiverged {
			fmt.Fprintf(w, "  (edited — re-indexed or edited, not newly posted)")
		}
		fmt.Fprintln(w)
	}
	if url := j.applyURL(); url != "" {
		fmt.Fprintf(w, "url: %s\n", url)
	}
	// Fields are already HTML-cleaned by the caller (cleanJob).
	if j.Description != "" {
		fmt.Fprintf(w, "\nDESCRIPTION\n%s\n", j.Description)
	}
	if j.BasicQualifications != "" {
		fmt.Fprintf(w, "\nBASIC QUALIFICATIONS\n%s\n", j.BasicQualifications)
	}
	if j.PreferredQualifications != "" {
		fmt.Fprintf(w, "\nPREFERRED QUALIFICATIONS\n%s\n", j.PreferredQualifications)
	}
}
