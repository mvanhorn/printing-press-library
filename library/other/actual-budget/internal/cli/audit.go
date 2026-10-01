// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

func newNovelAuditCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:         "audit",
		Short:       "Audit payees, rules and schedules in the local budget mirror for duplicates, dead rules and missed bills",
		Example:     "  actual-budget-pp-cli audit payees --min-similarity 0.85 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:typed-exit-codes": "0,2"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newNovelAuditPayeesCmd(flags))
	cmd.AddCommand(newNovelAuditRulesCmd(flags))
	cmd.AddCommand(newNovelAuditSchedulesCmd(flags))
	return cmd
}

// printOrderedTable renders a human table with a fixed column order (the
// generic auto-table sorts and truncates columns, hiding findings/status).
func printOrderedTable(w io.Writer, headers []string, rows [][]string) error {
	tw := newTabWriter(w)
	fmt.Fprintln(tw, strings.Join(headers, "\t"))
	for _, r := range rows {
		fmt.Fprintln(tw, strings.Join(r, "\t"))
	}
	return tw.Flush()
}
