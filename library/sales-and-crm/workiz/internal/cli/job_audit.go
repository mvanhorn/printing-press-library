// Copyright 2026 Eldar and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

// pp:data-source local

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

type auditIssue struct {
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	Issue      string `json:"issue"`
}

func newNovelJobAuditCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "audit",
		Short:       "Find jobs, leads, and clients missing phone, email, amount, or crew fields that would block a downstream billing push.",
		Example:     "  workiz-pp-cli job audit --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would scan jobs, leads, and customers in the local mirror for billing-blocking gaps")
				return nil
			}
			ctx := cmd.Context()
			var bail bool
			if dbPath, bail = checkNovelMirror(cmd, flags, dbPath, "job,lead", []auditIssue{}); bail {
				return nil
			}
			db, err := openNovelStore(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			jobs, err := loadJobs(ctx, db.DB())
			if err != nil {
				return fmt.Errorf("loading jobs: %w", err)
			}
			leads, err := loadLeads(ctx, db.DB())
			if err != nil {
				return fmt.Errorf("loading leads: %w", err)
			}
			customers, err := loadCustomers(ctx, db.DB())
			if err != nil {
				return fmt.Errorf("loading customers: %w", err)
			}

			issues := make([]auditIssue, 0)
			for _, j := range jobs {
				if j.Phone == "" && j.Email == "" {
					issues = append(issues, auditIssue{EntityType: "job", EntityID: j.UUID, Issue: "no phone or email on file"})
				}
				if strings.TrimSpace(j.Address) == "" {
					issues = append(issues, auditIssue{EntityType: "job", EntityID: j.UUID, Issue: "missing service address"})
				}
				if len(j.Team) == 0 {
					issues = append(issues, auditIssue{EntityType: "job", EntityID: j.UUID, Issue: "no crew assigned"})
				}
				if parseMoney(j.JobTotalPrice) == 0 {
					issues = append(issues, auditIssue{EntityType: "job", EntityID: j.UUID, Issue: "no job total price recorded"})
				}
			}
			for _, l := range leads {
				if l.Phone == "" && l.Email == "" {
					issues = append(issues, auditIssue{EntityType: "lead", EntityID: l.UUID, Issue: "no phone or email on file"})
				}
			}
			for _, c := range customers {
				if c.Email == "" {
					issues = append(issues, auditIssue{EntityType: "customer", EntityID: c.Id, Issue: "no email on file"})
				}
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), issues, flags)
			}
			if len(issues) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no billing-readiness issues found")
				return nil
			}
			for _, i := range issues {
				fmt.Fprintf(cmd.OutOrStdout(), "%s %s: %s\n", i.EntityType, i.EntityID, i.Issue)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/workiz-pp-cli/data.db)")
	return cmd
}
