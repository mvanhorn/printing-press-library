package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/loops/internal/client"
	"github.com/spf13/cobra"
)

// pp:data-source live
func init() {
	registerNovelCommand(installLoopsWorkflows)
}

func installLoopsWorkflows(root *cobra.Command, flags *rootFlags) {
	team := &cobra.Command{Use: "team", Short: "Verify the selected Loops team", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"}}
	team.AddCommand(&cobra.Command{
		Use: "verify", Short: "Check that --team or LOOPS_EXPECTED_TEAM matches the API key's team", Example: "  loops-pp-cli team verify --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if _, err := verifiedLoopsClient(cmd, flags); err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"ok": true, "effect": "reads_data", "teamVerified": true})
		},
	})
	root.AddCommand(team)

	audit := &cobra.Command{Use: "audit", Short: "Read-only account checks", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"}}
	audit.AddCommand(&cobra.Command{
		Use: "lifecycle", Short: "Count communication resources without retrieving contacts", Example: "  loops-pp-cli audit lifecycle --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := verifiedLoopsClient(cmd, flags)
			if err != nil {
				return err
			}
			counts := map[string]int{}
			for label, path := range map[string]string{
				"lists": "/v1/lists", "segments": "/v1/audience-segments", "campaigns": "/v1/campaigns",
				"workflows": "/v1/workflows", "eventPatterns": "/v1/event-patterns", "transactionalTemplates": "/v1/transactional-emails",
			} {
				rows, err := loopsListAll(cmd.Context(), c, path)
				if err != nil {
					return err
				}
				counts[label] = len(rows)
				if label == "campaigns" {
					for _, row := range rows {
						if row["status"] == "Draft" {
							counts["campaignDrafts"]++
						}
					}
				}
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
				"ok": true, "effect": "reads_data", "counts": counts,
				"limits": []string{"Loops has no bulk contacts list", "product activation needs a separate usage source"},
			})
		},
	})
	root.AddCommand(audit)

	for _, child := range root.Commands() {
		if child.Name() != "campaigns" {
			continue
		}
		var id string
		var latestDraft bool
		preflight := &cobra.Command{
			Use: "preflight", Short: "Check a campaign draft, audience target, and email Guardian", Example: "  loops-pp-cli campaigns preflight --latest-draft --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
			RunE: func(cmd *cobra.Command, _ []string) error {
				if (strings.TrimSpace(id) == "") == !latestDraft {
					return errors.New("provide exactly one of --id or --latest-draft")
				}
				c, err := verifiedLoopsClient(cmd, flags)
				if err != nil {
					return err
				}
				if latestDraft {
					rows, err := loopsListAll(cmd.Context(), c, "/v1/campaigns")
					if err != nil {
						return err
					}
					mostRecent := ""
					for _, row := range rows {
						if row["status"] == "Draft" {
							if candidate, ok := row["id"].(string); ok && candidate != "" {
								createdAt, _ := row["createdAt"].(string)
								if id == "" || createdAt > mostRecent {
									id, mostRecent = candidate, createdAt
								}
							}
						}
					}
					if id == "" {
						return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
							"ok": true, "effect": "reads_data", "apiChecksPassed": false,
							"issues": []string{"no campaign draft available for preflight"},
						})
					}
				}
				body, err := c.GetNoCache(cmd.Context(), "/v1/campaigns/"+url.PathEscape(id), nil)
				if err != nil {
					return err
				}
				var campaign map[string]any
				if json.Unmarshal(body, &campaign) != nil {
					return errors.New("Loops returned invalid campaign JSON")
				}
				issues := []string{}
				if campaign["status"] != "Draft" {
					issues = append(issues, "campaign is not a draft")
				}
				if campaign["mailingListId"] == nil && campaign["audienceSegmentId"] == nil && campaign["audienceFilter"] == nil {
					issues = append(issues, "campaign has no audience target")
				}
				messageID, _ := campaign["emailMessageId"].(string)
				guardianErrors, guardianWarnings := 0, 0
				if messageID == "" {
					issues = append(issues, "campaign has no email message")
				} else {
					body, err := c.GetNoCache(cmd.Context(), "/v1/email-messages/"+url.PathEscape(messageID)+"/guardian", nil)
					if err != nil {
						return err
					}
					var report struct {
						Errors   []json.RawMessage `json:"errors"`
						Warnings []json.RawMessage `json:"warnings"`
					}
					if json.Unmarshal(body, &report) != nil {
						return errors.New("Loops returned invalid Guardian JSON")
					}
					guardianErrors, guardianWarnings = len(report.Errors), len(report.Warnings)
					if guardianErrors > 0 {
						issues = append(issues, "Guardian found email content errors")
					}
				}
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
					"ok": true, "effect": "reads_data", "apiChecksPassed": len(issues) == 0,
					"issues": issues, "guardianErrors": guardianErrors, "guardianWarnings": guardianWarnings,
					"requiresHumanReview": []string{"audience consent", "sender domain", "links and copy", "send approval"},
				})
			},
		}
		preflight.Flags().StringVar(&id, "id", "", "Campaign ID (omitted from output)")
		preflight.Flags().BoolVar(&latestDraft, "latest-draft", false, "Check the newest API-listed draft without supplying an ID")
		child.AddCommand(preflight)
	}
}

func verifiedLoopsClient(cmd *cobra.Command, flags *rootFlags) (*client.Client, error) {
	team, _ := cmd.Root().PersistentFlags().GetString("team")
	if strings.TrimSpace(team) == "" {
		team = os.Getenv("LOOPS_EXPECTED_TEAM")
	}
	if strings.TrimSpace(team) == "" {
		return nil, errors.New("--team or LOOPS_EXPECTED_TEAM is required for this account check")
	}
	if flags.dataSource == "local" {
		return nil, errors.New("this command requires live Loops data")
	}
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	data, err := c.GetNoCache(cmd.Context(), "/v1/api-key", nil)
	if err != nil {
		return nil, err
	}
	var reply struct {
		Success  bool   `json:"success"`
		TeamName string `json:"teamName"`
	}
	if json.Unmarshal(data, &reply) != nil || !reply.Success || reply.TeamName == "" || reply.TeamName != team {
		return nil, errors.New("selected Loops team does not match --team")
	}
	return c, nil
}

func loopsListAll(ctx context.Context, c *client.Client, path string) ([]map[string]any, error) {
	if path == "/v1/lists" {
		data, err := c.GetNoCache(ctx, path, nil)
		if err != nil {
			return nil, err
		}
		var rows []map[string]any
		if json.Unmarshal(data, &rows) != nil || rows == nil {
			return nil, errors.New("Loops returned invalid list JSON")
		}
		return rows, nil
	}
	seen := map[string]bool{}
	cursor := ""
	rows := []map[string]any{}
	for page := 0; page < 100; page++ {
		params := map[string]string{"perPage": "50"}
		if cursor != "" {
			params["cursor"] = cursor
		}
		data, err := c.GetNoCache(ctx, path, params)
		if err != nil {
			return nil, err
		}
		var reply struct {
			Data       *[]map[string]any `json:"data"`
			Pagination *struct {
				NextCursor string `json:"nextCursor"`
			} `json:"pagination"`
		}
		if json.Unmarshal(data, &reply) != nil || reply.Data == nil || reply.Pagination == nil {
			return nil, errors.New("Loops returned invalid paginated JSON")
		}
		rows = append(rows, (*reply.Data)...)
		cursor = reply.Pagination.NextCursor
		if cursor == "" {
			return rows, nil
		}
		if seen[cursor] {
			return nil, errors.New("Loops repeated a pagination cursor")
		}
		seen[cursor] = true
	}
	return nil, fmt.Errorf("Loops pagination for %s exceeded 100 pages", path)
}
