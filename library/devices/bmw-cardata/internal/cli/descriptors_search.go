// Copyright 2026 jvm and contributors. Licensed under Apache-2.0.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/devices/bmw-cardata/internal/store"

	"github.com/spf13/cobra"
)

// pp:novel-static-reference
// pp:data-source local

func newNovelDescriptorsSearchCmd(flags *rootFlags) *cobra.Command {
	var (
		flagDB    string
		flagLimit int
	)
	cmd := &cobra.Command{
		Use:         "search <pattern>",
		Short:       "Search the VSS telematic descriptor catalogue (e.g. batteryManagement, charging, navigation).",
		Long:        "Search BMW CarData's VSS telematic data catalogue by descriptor path, unit, or domain to find the exact keys for a container.",
		Example:     "  bmw-cardata-pp-cli descriptors search \"batteryManagement\"",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search the VSS descriptor catalogue")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("a search pattern is required"))
			}
			pattern := args[0]
			dbPath := resolveDBPath(flagDB)
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return configErr(fmt.Errorf("opening store: %w", err))
			}
			defer db.Close()
			if err := seedCardataCatalogue(db); err != nil {
				return configErr(fmt.Errorf("seeding catalogue: %w", err))
			}
			results, err := descriptorsByPrefix(db, pattern, flagLimit)
			if err != nil {
				return configErr(fmt.Errorf("searching catalogue: %w", err))
			}
			if wantsMachine(cmd, flags) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no descriptors matched %q\n", pattern)
				return nil
			}
			for _, r := range results {
				unit := r["unit"]
				if unit != "" {
					unit = " [" + unit + "]"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s%s\n    %s\n", r["path"], unit, r["description"])
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagDB, "db", "", "Database path")
	cmd.Flags().IntVar(&flagLimit, "limit", 50, "Maximum results to return")
	return cmd
}
