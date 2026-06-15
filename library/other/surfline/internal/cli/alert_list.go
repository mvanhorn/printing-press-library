// Copyright 2026 Shoffner and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored novel command: list stored alert rules.
//
// pp:data-source local

package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newNovelAlertListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List stored alert rules.",
		Example:     "  surfline-pp-cli alert list --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			resolved := dbPath
			if resolved == "" {
				resolved = defaultDBPath(surflineDBName)
			}
			if _, statErr := os.Stat(resolved); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no alerts yet; add one with: surfline-pp-cli alert add <name> --spot <id> ...\n")
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				}
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := openSurflineStore(ctx, dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			rules, err := listAlertRules(ctx, db, "")
			if err != nil {
				return fmt.Errorf("listing alerts: %w", err)
			}
			if rules == nil {
				rules = []alertRule{}
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return printJSONFiltered(cmd.OutOrStdout(), rules, flags)
			}
			if len(rules) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no alerts stored")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "NAME\tSPOT\tMIN_SURF\tMIN_PERIOD\tMAX_WIND\tOFFSHORE\tMIN_RATING")
			for _, r := range rules {
				fmt.Fprintf(tw, "%s\t%s\t%.0f\t%.0f\t%.0f\t%v\t%.0f\n",
					r.Name, r.SpotID, r.MinSurf, r.MinPeriod, r.MaxWind, r.OffshoreOnly, r.MinRating)
			}
			return tw.Flush()
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/surfline-pp-cli/data.db)")
	return cmd
}
