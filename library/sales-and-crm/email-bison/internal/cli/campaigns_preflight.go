// Copyright 2026 riccardovandra and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored novel (transcendence) command: campaigns preflight.

package cli

import (
	"database/sql"
	"fmt"

	"github.com/spf13/cobra"
)

type preflightResult struct {
	CampaignID       string   `json:"campaign_id"`
	HasSchedule      bool     `json:"has_schedule"`
	HasSequenceSteps bool     `json:"has_sequence_steps"`
	HasSenders       bool     `json:"has_senders"`
	HasLeads         bool     `json:"has_leads"`
	MissingMergeTags []string `json:"missing_merge_tags"`
	Ready            bool     `json:"ready"`
	Issues           []string `json:"issues"`
}

func newNovelCampaignsPreflightCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "preflight <campaign-id>",
		Short: "Before resuming, check that a campaign has a schedule, a sequence, senders, leads, and valid merge tags.",
		Long: `Validates a campaign against the local store before launch: confirms it has a
schedule, at least one sequence step, at least one attached sender, attached leads,
and that every {VARIABLE} merge tag in the sequence exists as a custom variable.
Sync first with 'email-bison-pp-cli sync'.`,
		Example:     "  email-bison-pp-cli campaigns preflight 6 --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 1 {
				return cmd.Help()
			}
			campaignID := args[0]

			db, err := openNovelStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()

			exists := func(table string) (bool, error) {
				var n int
				row := db.DB().QueryRowContext(cmd.Context(),
					fmt.Sprintf(`SELECT COUNT(*) FROM %q WHERE CAST(campaigns_id AS TEXT) = ?`, table), campaignID)
				if err := row.Scan(&n); err != nil {
					return false, err
				}
				return n > 0, nil
			}

			res := preflightResult{CampaignID: campaignID, MissingMergeTags: []string{}, Issues: []string{}}
			if res.HasSchedule, err = exists("schedule"); err != nil {
				return fmt.Errorf("checking schedule: %w", err)
			}
			if res.HasSequenceSteps, err = exists("sequence_steps"); err != nil {
				return fmt.Errorf("checking sequence steps: %w", err)
			}
			if res.HasSenders, err = exists("attach_sender_emails"); err != nil {
				return fmt.Errorf("checking senders: %w", err)
			}
			if res.HasLeads, err = exists("campaigns_leads"); err != nil {
				return fmt.Errorf("checking leads: %w", err)
			}

			// Collect merge tags from this campaign's sequence step bodies/subjects.
			tagRows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT COALESCE(json_extract(data,'$.email_subject'), '') || ' ' ||
				        COALESCE(json_extract(data,'$.email_body'), '')
				   FROM sequence_steps WHERE CAST(campaigns_id AS TEXT) = ?`, campaignID)
			if err != nil {
				return fmt.Errorf("reading sequence steps: %w", err)
			}
			defer tagRows.Close()
			usedTags := map[string]bool{}
			for tagRows.Next() {
				var text sql.NullString
				if err := tagRows.Scan(&text); err != nil {
					return fmt.Errorf("scanning sequence step: %w", err)
				}
				for _, t := range extractMergeTags(text.String) {
					usedTags[t] = true
				}
			}
			if err := tagRows.Err(); err != nil {
				return fmt.Errorf("iterating sequence steps: %w", err)
			}

			// Known custom-variable names (compared case-insensitively).
			known := map[string]bool{}
			cvRows, err := db.DB().QueryContext(cmd.Context(),
				`SELECT COALESCE(json_extract(data,'$.name'), '') FROM resources WHERE resource_type = 'custom-variables'`)
			if err != nil {
				return fmt.Errorf("reading custom variables: %w", err)
			}
			defer cvRows.Close()
			for cvRows.Next() {
				var name sql.NullString
				if err := cvRows.Scan(&name); err != nil {
					return fmt.Errorf("scanning custom variable: %w", err)
				}
				if name.String != "" {
					known[upperASCII(name.String)] = true
				}
			}
			if err := cvRows.Err(); err != nil {
				return fmt.Errorf("iterating custom variables: %w", err)
			}
			for tag := range usedTags {
				if !known[upperASCII(tag)] {
					res.MissingMergeTags = append(res.MissingMergeTags, tag)
				}
			}

			if !res.HasSchedule {
				res.Issues = append(res.Issues, "no schedule set")
			}
			if !res.HasSequenceSteps {
				res.Issues = append(res.Issues, "no sequence steps")
			}
			if !res.HasSenders {
				res.Issues = append(res.Issues, "no sender emails attached")
			}
			if !res.HasLeads {
				res.Issues = append(res.Issues, "no leads attached")
			}
			for _, t := range res.MissingMergeTags {
				res.Issues = append(res.Issues, "merge tag {"+t+"} has no matching custom variable")
			}
			res.Ready = len(res.Issues) == 0

			out := cmd.OutOrStdout()
			if flags.asJSON {
				return printJSONFiltered(out, res, flags)
			}
			fmt.Fprintf(out, "Campaign %s preflight: %s\n", campaignID, readyLabel(res.Ready))
			fmt.Fprintf(out, "  schedule:       %s\n", checkLabel(res.HasSchedule))
			fmt.Fprintf(out, "  sequence steps: %s\n", checkLabel(res.HasSequenceSteps))
			fmt.Fprintf(out, "  senders:        %s\n", checkLabel(res.HasSenders))
			fmt.Fprintf(out, "  leads:          %s\n", checkLabel(res.HasLeads))
			if len(res.MissingMergeTags) > 0 {
				fmt.Fprintf(out, "  missing merge tags: %v\n", res.MissingMergeTags)
			}
			if !res.Ready {
				fmt.Fprintln(out, "Issues:")
				for _, i := range res.Issues {
					fmt.Fprintf(out, "  - %s\n", i)
				}
			}
			return nil
		},
	}
	return cmd
}

func upperASCII(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'a' && b[i] <= 'z' {
			b[i] -= 32
		}
	}
	return string(b)
}

func checkLabel(ok bool) string {
	if ok {
		return "ok"
	}
	return "MISSING"
}

func readyLabel(ready bool) string {
	if ready {
		return "READY"
	}
	return "NOT READY"
}
