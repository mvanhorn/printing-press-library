// Copyright 2026 srijits and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local
package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newDuplicatesApplyCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use: "apply <plan-id>", Short: "Apply a reviewed duplicate plan", Example: "  raindrop-pp-cli duplicates apply 1 --dry-run --agent", Args: cobra.ExactArgs(1),
		Annotations: map[string]string{"mcp:destructive": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			planID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("invalid plan id %q", args[0]))
			}
			db, _, err := openNovelStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			var payload, status string
			if err := db.DB().QueryRowContext(cmd.Context(), `SELECT payload,status FROM cleanup_plans WHERE id=? AND kind='duplicates'`, planID).Scan(&payload, &status); err != nil {
				return fmt.Errorf("loading plan: %w", err)
			}
			if status != "planned" {
				return fmt.Errorf("plan %d is %s", planID, status)
			}
			var groups []struct {
				Keep       string           `json:"keep"`
				Remove     []string         `json:"remove"`
				MergeTags  []string         `json:"merge_tags"`
				MergeNote  string           `json:"merge_note"`
				Highlights []map[string]any `json:"highlights"`
			}
			if err := json.Unmarshal([]byte(payload), &groups); err != nil {
				return err
			}
			if flags.dryRun {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"plan_id": planID, "dry_run": true, "groups": groups}, flags)
			}
			if !flags.yes {
				return usageErr(fmt.Errorf("duplicate apply is destructive; rerun with --yes after reviewing 'duplicates plan' output"))
			}
			client, err := flags.newClient()
			if err != nil {
				return err
			}
			deleted := 0
			alreadyDeleted := 0
			highlightsCreated := 0
			for _, group := range groups {
				bookmarkBody := map[string]any{"tags": group.MergeTags}
				if strings.TrimSpace(group.MergeNote) != "" {
					bookmarkBody["note"] = group.MergeNote
				}
				if _, _, err := client.PutWithParams(cmd.Context(), "/raindrop/"+group.Keep, nil, bookmarkBody); err != nil {
					return classifyAPIError(err, flags)
				}
				existing, err := client.GetNoCache(cmd.Context(), "/raindrop/"+group.Keep+"/highlights", nil)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				highlightSet := highlightSignaturesFromResponse(existing)
				for _, highlight := range group.Highlights {
					body := highlightMutationBody(highlight)
					if valueString(body["text"]) == "" {
						continue
					}
					signature := highlightSignature(body)
					if _, ok := highlightSet[signature]; ok {
						continue
					}
					bookmarkID, parseErr := strconv.ParseInt(group.Keep, 10, 64)
					if parseErr != nil {
						return fmt.Errorf("invalid keeper bookmark id %q: %w", group.Keep, parseErr)
					}
					body["raindrop"] = map[string]any{"$id": bookmarkID}
					if _, _, err := client.PostWithParams(cmd.Context(), "/highlights", nil, body); err != nil {
						return classifyAPIError(err, flags)
					}
					highlightSet[signature] = struct{}{}
					highlightsCreated++
				}
				for _, id := range group.Remove {
					if _, _, err := client.DeleteWithParams(cmd.Context(), "/raindrop/"+id, nil); err != nil {
						if strings.Contains(err.Error(), "HTTP 404") {
							alreadyDeleted++
							continue
						}
						return classifyDeleteError(err, flags)
					}
					deleted++
				}
			}
			_, err = db.DB().ExecContext(cmd.Context(), `UPDATE cleanup_plans SET status='applied',applied_at=? WHERE id=?`, time.Now().UTC().Format(time.RFC3339), planID)
			if err != nil {
				return err
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"plan_id": planID, "status": "applied", "groups": len(groups), "deleted": deleted, "already_deleted": alreadyDeleted, "highlights_created": highlightsCreated}, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database path")
	return cmd
}

func highlightMutationBody(highlight map[string]any) map[string]any {
	body := map[string]any{}
	for _, key := range []string{"text", "note", "color"} {
		if value := strings.TrimSpace(valueString(highlight[key])); value != "" {
			body[key] = value
		}
	}
	return body
}

func highlightSignature(highlight map[string]any) string {
	return strings.Join([]string{
		strings.TrimSpace(valueString(highlight["text"])),
		strings.TrimSpace(valueString(highlight["note"])),
		strings.TrimSpace(valueString(highlight["color"])),
	}, "\x00")
}

func highlightSignaturesFromResponse(data json.RawMessage) map[string]struct{} {
	var items []map[string]any
	if json.Unmarshal(data, &items) != nil {
		var envelope map[string]json.RawMessage
		if json.Unmarshal(data, &envelope) == nil {
			for _, key := range []string{"items", "data", "result"} {
				if raw := envelope[key]; len(raw) > 0 && json.Unmarshal(raw, &items) == nil {
					break
				}
			}
		}
	}
	result := make(map[string]struct{}, len(items))
	for _, item := range items {
		result[highlightSignature(item)] = struct{}{}
	}
	return result
}
