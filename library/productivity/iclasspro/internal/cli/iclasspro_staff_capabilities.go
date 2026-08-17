// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
// pp:novel-static-reference

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

type icpStaffCapability struct {
	Resource string `json:"resource"`
	Command  string `json:"command"`
	WireVerb string `json:"wire_verb"`
	Scope    string `json:"scope"`
}

func newIclassproAdminCapabilitiesCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "capabilities",
		Short:   "Describe the allow-listed staff reads without requiring a session",
		Example: "  iclasspro-pp-cli admin capabilities --json",
		Args:    cobra.NoArgs,
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:endpoint":   "staff.capabilities.local",
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			rows := []icpStaffCapability{
				{Resource: "dashboard", Command: "admin dashboard", WireVerb: "GET + POST query", Scope: "saved dashboard, widget catalog, widget data"},
				{Resource: "families", Command: "admin families", WireVerb: "GET + POST query", Scope: "filter defaults and family search"},
				{Resource: "students", Command: "admin students", WireVerb: "GET + POST query", Scope: "filter defaults and student search"},
				{Resource: "classes", Command: "admin class-search", WireVerb: "GET + POST query", Scope: "filter defaults and class search"},
				{Resource: "enrollments", Command: "admin enrollments", WireVerb: "GET + POST query", Scope: "filter defaults and enrollment search"},
				{Resource: "attendance", Command: "admin attendance", WireVerb: "GET", Scope: "one class roster and attendance state"},
				{Resource: "transactions", Command: "admin transactions", WireVerb: "GET + POST query", Scope: "filter defaults and gateway transaction search"},
				{Resource: "reports", Command: "admin reports", WireVerb: "GET", Scope: "report definitions only"},
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"read_only": true,
					"cached":    false,
					"resources": rows,
					"excluded": []string{
						"arbitrary requests", "record mutation", "payments", "attendance writes",
						"report generation", "email", "download", "export", "payroll", "time clock",
					},
				}, flags)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "RESOURCE\tCOMMAND\tWIRE\tSCOPE")
			for _, row := range rows {
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", row.Resource, row.Command, row.WireVerb, row.Scope)
			}
			return tw.Flush()
		},
	}
}
