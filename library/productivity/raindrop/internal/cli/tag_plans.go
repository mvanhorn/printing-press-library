// Copyright 2026 srijits and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/spf13/cobra"
)

type tagMerge struct {
	From  []string `json:"from"`
	To    string   `json:"to"`
	Count int      `json:"count"`
}

func newTagPlanMergesCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{Use: "plan-merges", Short: "Persist deterministic tag merge candidates", Example: "  raindrop-pp-cli tag plan-merges --agent", Annotations: map[string]string{"mcp:local-write": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		db, _, err := openNovelStore(cmd.Context(), dbPath)
		if err != nil {
			return err
		}
		defer db.Close()
		items, err := loadLocalBookmarks(db)
		if err != nil {
			return err
		}
		variants := map[string]map[string]int{}
		for _, item := range items {
			for _, tag := range item.Tags {
				key := normalizedTag(tag)
				if variants[key] == nil {
					variants[key] = map[string]int{}
				}
				variants[key][tag]++
			}
		}
		var merges []tagMerge
		for key, names := range variants {
			if key == "" || len(names) < 2 {
				continue
			}
			ordered := sortedKeys(names)
			sort.SliceStable(ordered, func(i, j int) bool { return names[ordered[i]] > names[ordered[j]] })
			total := 0
			for _, c := range names {
				total += c
			}
			merges = append(merges, tagMerge{From: ordered[1:], To: ordered[0], Count: total})
		}
		sort.Slice(merges, func(i, j int) bool { return merges[i].To < merges[j].To })
		payload, _ := json.Marshal(merges)
		res, err := db.DB().ExecContext(cmd.Context(), `INSERT INTO cleanup_plans(kind,payload) VALUES('tags',?)`, string(payload))
		if err != nil {
			return err
		}
		id, _ := res.LastInsertId()
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"plan_id": id, "merges": merges, "count": len(merges)}, flags)
	}}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	return cmd
}

func newTagApplyPlanCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{Use: "apply-plan <plan-id>", Short: "Apply reviewed tag merge plan", Example: "  raindrop-pp-cli tag apply-plan 1 --dry-run --agent", Args: cobra.ExactArgs(1), Annotations: map[string]string{"mcp:destructive": "true"}, RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			return usageErr(err)
		}
		db, _, err := openNovelStore(cmd.Context(), dbPath)
		if err != nil {
			return err
		}
		defer db.Close()
		var payload, status string
		if err := db.DB().QueryRowContext(cmd.Context(), `SELECT payload,status FROM cleanup_plans WHERE id=? AND kind='tags'`, id).Scan(&payload, &status); err != nil {
			return err
		}
		if status != "planned" {
			return fmt.Errorf("plan %d is %s", id, status)
		}
		var merges []tagMerge
		if err := json.Unmarshal([]byte(payload), &merges); err != nil {
			return err
		}
		if flags.dryRun {
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"plan_id": id, "dry_run": true, "merges": merges}, flags)
		}
		if !flags.yes {
			return usageErr(fmt.Errorf("tag plan mutates remote tags; rerun with --yes"))
		}
		client, err := flags.newClient()
		if err != nil {
			return err
		}
		for _, merge := range merges {
			for _, from := range merge.From {
				if _, _, err := client.PutWithParams(cmd.Context(), "/tags/0", nil, map[string]any{"tags": []string{from}, "replace": merge.To}); err != nil {
					return classifyAPIError(err, flags)
				}
			}
		}
		_, err = db.DB().ExecContext(cmd.Context(), `UPDATE cleanup_plans SET status='applied',applied_at=? WHERE id=?`, time.Now().UTC().Format(time.RFC3339), id)
		if err != nil {
			return err
		}
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"plan_id": id, "status": "applied", "merges": len(merges)}, flags)
	}}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	return cmd
}
