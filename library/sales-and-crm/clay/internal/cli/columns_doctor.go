// Copyright 2026 Ade Amos and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type doctorFinding struct {
	Severity string `json:"severity"`
	FieldID  string `json:"fieldId"`
	Column   string `json:"column"`
	Issue    string `json:"issue"`
	Detail   string `json:"detail,omitempty"`
}

type doctorReport struct {
	TableID  string          `json:"tableId"`
	Name     string          `json:"tableName"`
	Findings []doctorFinding `json:"findings"`
	Errors   int             `json:"errorCount"`
	Warnings int             `json:"warningCount"`
}

func newNovelColumnsDoctorCmd(flags *rootFlags) *cobra.Command {
	var flagWorkspace string
	cmd := &cobra.Command{
		Use:   "doctor <tableId>",
		Short: "Find formulas pointing at deleted columns and columns nothing consumes.",
		Long: "Use this command before a large enrichment run so broken formulas do not waste credits.\n" +
			"Do NOT use it to inspect run failures; use 'errors' for that.",
		Example: "  clay-pp-cli columns doctor t_abc123 --workspace 1234567 --agent",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:happy-args":       "<tableId>=t_example;--workspace=1;--dry-run",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "columns doctor")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<tableId> is required"))
			}
			ws, err := resolveWorkspace(flagWorkspace)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			tbl, err := fetchTable(ctx, c, ws, args[0])
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			byID := indexByID(tbl.Fields)
			consumed := map[string]bool{}
			rep := doctorReport{TableID: tbl.ID, Name: tbl.Name, Findings: make([]doctorFinding, 0)}

			for _, f := range tbl.Fields {
				ts := f.settings()
				for _, ref := range formulaRefs(ts.formulaBody()) {
					if _, ok := byID[ref]; ok {
						consumed[ref] = true
						continue
					}
					rep.Findings = append(rep.Findings, doctorFinding{
						Severity: "error", FieldID: f.ID, Column: f.Name,
						Issue:  "dangling formula reference",
						Detail: fmt.Sprintf("references %s which does not exist in this table", ref),
					})
				}
				if f.Type == "action" && ts.ActionKey == "" {
					rep.Findings = append(rep.Findings, doctorFinding{
						Severity: "error", FieldID: f.ID, Column: f.Name,
						Issue: "enrichment column has no actionKey",
					})
				}
			}
			for _, f := range tbl.Fields {
				if isSystemField(f.ID) || consumed[f.ID] {
					continue
				}
				if f.Type == "text" || f.Type == "number" {
					rep.Findings = append(rep.Findings, doctorFinding{
						Severity: "warning", FieldID: f.ID, Column: f.Name,
						Issue: "column is not referenced by any formula",
					})
				}
			}
			for _, f := range rep.Findings {
				if f.Severity == "error" {
					rep.Errors++
				} else {
					rep.Warnings++
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				if err := printJSONFiltered(cmd.OutOrStdout(), rep, flags); err != nil {
					return err
				}
			} else if len(rep.Findings) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "%s: no issues found across %d columns\n", tbl.Name, len(tbl.Fields))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "%s (%s)\n", rep.Name, rep.TableID)
				for _, f := range rep.Findings {
					fmt.Fprintf(cmd.OutOrStdout(), "  %-7s %-28s %s %s\n", f.Severity, f.Column, f.Issue, f.Detail)
				}
			}
			if rep.Errors > 0 {
				return notFoundErr(fmt.Errorf("%d broken column reference(s) found", rep.Errors))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagWorkspace, "workspace", "", "Clay workspace id (or set CLAY_WORKSPACE_ID)")
	return cmd
}
