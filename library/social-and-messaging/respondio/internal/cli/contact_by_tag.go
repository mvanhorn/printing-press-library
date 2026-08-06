// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command contact by-tag: segment synced contacts by a tag cohort.

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/social-and-messaging/respondio/internal/store"
	"github.com/spf13/cobra"
)

// pp:data-source local

func newNovelContactByTagCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:         "by-tag <tag>",
		Short:       "List every contact that carries a given tag (VIP, unpaid, in-trial).",
		Long:        "Lists contacts from the local mirror whose tags array includes <tag>. Run 'respondio-pp-cli sync --resources contact' first.",
		Example:     "  respondio-pp-cli contact by-tag VIP --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "contact by-tag")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("missing required positional argument: <tag>"))
			}
			tag := args[0]
			ctx := cmd.Context()
			if dbPath == "" {
				dbPath = defaultDBPath("respondio-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: respondio-pp-cli sync --resources contact --db %s\n", dbPath, dbPath)
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), make([]map[string]any, 0), flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "No synced contacts yet.")
				return nil
			}
			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			rows, err := db.DB().QueryContext(ctx, `SELECT data FROM resources WHERE resource_type = 'contact'`)
			if err != nil {
				return fmt.Errorf("querying contacts: %w", err)
			}
			var datas [][]byte
			for rows.Next() {
				var data []byte
				if err := rows.Scan(&data); err != nil {
					_ = rows.Close()
					return fmt.Errorf("scan contact: %w", err)
				}
				datas = append(datas, data)
			}
			if err := rows.Err(); err != nil {
				_ = rows.Close()
				return fmt.Errorf("iterate contacts: %w", err)
			}
			_ = rows.Close()

			results := make([]map[string]any, 0)
			for _, raw := range datas {
				var c map[string]any
				if err := json.Unmarshal(raw, &c); err != nil {
					continue
				}
				if hasTag(c, tag) {
					results = append(results, c)
					if limit > 0 && len(results) >= limit {
						break
					}
				}
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			for _, c := range results {
				fmt.Fprintf(cmd.OutOrStdout(), "%v %v %v\n", c["id"], c["firstName"], c["email"])
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 100, "maximum contacts to return")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func hasTag(c map[string]any, tag string) bool {
	tags, ok := c["tags"].([]any)
	if !ok {
		return false
	}
	for _, t := range tags {
		if s, ok := t.(string); ok && s == tag {
			return true
		}
	}
	return false
}
