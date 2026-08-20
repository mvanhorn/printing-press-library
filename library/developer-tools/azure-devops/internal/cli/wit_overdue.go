// pp:data-source live
package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type overdueItem struct {
	ID            int               `json:"id"`
	Type          string            `json:"type"`
	Title         string            `json:"title"`
	State         string            `json:"state"`
	DueDate       string            `json:"due_date,omitempty"`
	DaysOverdue   int               `json:"days_overdue,omitempty"`
	AreaPath      string            `json:"area_path,omitempty"`
	Iteration     string            `json:"iteration,omitempty"`
	CommentCount  int               `json:"comment_count"`
	ChangedDate   string            `json:"last_changed,omitempty"`
	CustomFields  map[string]string `json:"custom_fields,omitempty"`
	LatestComment string            `json:"latest_comment,omitempty"`
}

func newWitOverdueCmd(flags *rootFlags) *cobra.Command {
	var withComments bool
	var typesFilter string
	var extraFields string
	var limit int

	cmd := &cobra.Command{
		Use:   "overdue",
		Short: "List work items assigned to you that are past their due date",
		Long: `Queries Azure DevOps for all active work items assigned to @me where
DueDate < today. Returns ID, title, type, state, days overdue, comment count,
area path, last changed date, and any extra fields requested via --extra-fields.

Use --with-comments to also fetch the text of the most recent comment.
Use --type to filter by work item type (e.g. "User Story,Bug,Task").
Use --extra-fields to include org-specific custom fields in the output.`,
		Example: strings.Trim(`
  azure-devops-pp-cli wit overdue
  azure-devops-pp-cli wit overdue --json
  azure-devops-pp-cli wit overdue --with-comments --agent
  azure-devops-pp-cli wit overdue --type "User Story,Bug"
  azure-devops-pp-cli wit overdue --extra-fields "Custom.Priority,Custom.Phase"`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would query overdue work items assigned to @me via WIQL")
				return nil
			}
			if flags.project == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--project is required (or set AZURE_DEVOPS_PROJECT)"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Build WIQL — filter by type if requested
			typeClause := ""
			if typesFilter != "" {
				types := strings.Split(typesFilter, ",")
				quoted := make([]string, len(types))
				for i, t := range types {
					quoted[i] = fmt.Sprintf("'%s'", strings.TrimSpace(t))
				}
				typeClause = fmt.Sprintf(" AND [System.WorkItemType] IN (%s)", strings.Join(quoted, ","))
			}

			wiqlPath := adoAPIPath(flags, flags.project, "wit/wiql") + "?api-version=7.1"
			wiqlResp, _, err := c.Post(ctx, wiqlPath, map[string]any{
				"query": fmt.Sprintf(
					"SELECT [System.Id] FROM WorkItems WHERE [System.AssignedTo] = @me"+
						" AND [System.State] NOT IN ('Done','Closed','Removed')"+
						" AND [Microsoft.VSTS.Scheduling.DueDate] < @today"+
						"%s"+
						" ORDER BY [Microsoft.VSTS.Scheduling.DueDate] ASC",
					typeClause,
				),
			})
			if err != nil {
				return fmt.Errorf("WIQL query failed: %w", err)
			}

			var wiqlResult struct {
				WorkItems []struct {
					ID  int    `json:"id"`
					URL string `json:"url"`
				} `json:"workItems"`
			}
			if err := json.Unmarshal(wiqlResp, &wiqlResult); err != nil {
				return fmt.Errorf("parsing WIQL response: %w", err)
			}

			if len(wiqlResult.WorkItems) == 0 {
				if flags.asJSON || flags.agent {
					fmt.Fprintln(cmd.OutOrStdout(), "[]")
				} else {
					fmt.Fprintln(cmd.OutOrStdout(), "No overdue work items found.")
				}
				return nil
			}

			// Batch-fetch details — ADO allows max 200 IDs per call
			ids := make([]string, 0, len(wiqlResult.WorkItems))
			for _, wi := range wiqlResult.WorkItems {
				if limit > 0 && len(ids) >= limit {
					break
				}
				ids = append(ids, fmt.Sprintf("%d", wi.ID))
			}

			coreFields := []string{
				"System.Id",
				"System.Title",
				"System.WorkItemType",
				"System.State",
				"Microsoft.VSTS.Scheduling.DueDate",
				"System.IterationPath",
				"System.AreaPath",
				"System.CommentCount",
				"System.ChangedDate",
			}
			// Append any org-specific custom fields requested via --extra-fields
			if extraFields != "" {
				for _, f := range strings.Split(extraFields, ",") {
					if f = strings.TrimSpace(f); f != "" {
						coreFields = append(coreFields, f)
					}
				}
			}
			fields := strings.Join(coreFields, ",")

			batchPath := adoAPIPath(flags, flags.project, "wit/workitems")
			batchResp, err := c.Get(ctx, batchPath, map[string]string{
				"ids":         strings.Join(ids, ","),
				"fields":      fields,
				"api-version": "7.1",
			})
			if err != nil {
				return fmt.Errorf("fetching work item details: %w", err)
			}

			var batchResult struct {
				Value []struct {
					ID     int                    `json:"id"`
					Fields map[string]interface{} `json:"fields"`
				} `json:"value"`
			}
			if err := json.Unmarshal(batchResp, &batchResult); err != nil {
				return fmt.Errorf("parsing batch response: %w", err)
			}

			now := time.Now().UTC()
			items := make([]overdueItem, 0, len(batchResult.Value))

			for _, wi := range batchResult.Value {
				f := wi.Fields
				item := overdueItem{
					ID:    wi.ID,
					Type:  strField(f, "System.WorkItemType"),
					Title: strField(f, "System.Title"),
					State: strField(f, "System.State"),
				}

				if due := strField(f, "Microsoft.VSTS.Scheduling.DueDate"); due != "" {
					dueT, err := time.Parse(time.RFC3339, due)
					if err == nil {
						item.DueDate = dueT.Format("2006-01-02")
						days := now.Sub(dueT).Hours() / 24
						item.DaysOverdue = int(math.Round(days))
					}
				}

				item.AreaPath = lastSegment(strField(f, "System.AreaPath"), "\\")
				item.Iteration = lastSegment(strField(f, "System.IterationPath"), "\\")

				// Collect any requested extra fields into a generic map
				if extraFields != "" {
					for _, ef := range strings.Split(extraFields, ",") {
						ef = strings.TrimSpace(ef)
						if ef == "" {
							continue
						}
						if v := strField(f, ef); v != "" {
							// Use the last segment of the field ref as the key (e.g. "Custom.Phase" → "Phase")
							key := lastSegment(ef, ".")
							if item.CustomFields == nil {
								item.CustomFields = make(map[string]string)
							}
							item.CustomFields[key] = v
						}
					}
				}

				if changed := strField(f, "System.ChangedDate"); changed != "" {
					if t, err := time.Parse(time.RFC3339, changed); err == nil {
						item.ChangedDate = t.Format("2006-01-02")
					}
				}

				if cc, ok := f["System.CommentCount"]; ok {
					switch v := cc.(type) {
					case float64:
						item.CommentCount = int(v)
					}
				}

				items = append(items, item)
			}

			// Optionally fetch latest comment for each item
			if withComments {
				commentsPath := adoAPIPath(flags, flags.project, "wit/workItems/%d/comments")
				for i := range items {
					path := fmt.Sprintf(commentsPath, items[i].ID)
					resp, err := c.Get(ctx, path, map[string]string{
						"api-version": "7.1-preview.3",
						"top":         "1",
					})
					if err != nil {
						continue
					}
					var cr struct {
						Comments []struct {
							Text      string `json:"text"`
							CreatedBy struct {
								DisplayName string `json:"displayName"`
							} `json:"createdBy"`
							CreatedDate string `json:"createdDate"`
						} `json:"comments"`
					}
					if err := json.Unmarshal(resp, &cr); err != nil || len(cr.Comments) == 0 {
						continue
					}
					latest := cr.Comments[0]
					// Strip HTML tags for clean text output
					text := stripHTML(latest.Text)
					if len(text) > 200 {
						text = text[:200] + "…"
					}
					items[i].LatestComment = fmt.Sprintf("[%s] %s", latest.CreatedBy.DisplayName, text)
				}
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(items)
			}

			// Human table
			fmt.Fprintf(cmd.OutOrStdout(), "Overdue work items assigned to you (%d total)\n\n", len(items))
			fmt.Fprintf(cmd.OutOrStdout(), "%-6s  %-12s  %-10s  %5s  %-22s  %-16s  %s\n",
				"ID", "Type", "Due Date", "Days", "State", "Phase/Sprint", "Title")
			fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 110))

			for _, item := range items {
				// Use the first custom field value as an extra label column, falling back to iteration
				phase := item.Iteration
				for _, v := range item.CustomFields {
					phase = v
					break
				}
				title := item.Title
				if len(title) > 55 {
					title = title[:52] + "..."
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-6d  %-12s  %-10s  %5d  %-22s  %-16s  %s\n",
					item.ID,
					truncate(item.Type, 12),
					item.DueDate,
					item.DaysOverdue,
					truncate(item.State, 22),
					truncate(phase, 16),
					title,
				)
				if withComments && item.LatestComment != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "       └─ %s\n", item.LatestComment)
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&withComments, "with-comments", false, "Also fetch the latest comment for each item")
	cmd.Flags().StringVar(&typesFilter, "type", "", "Filter by work item type(s), comma-separated (e.g. \"User Story,Bug\")")
	cmd.Flags().StringVar(&extraFields, "extra-fields", "", "Comma-separated org-specific field refs to include (e.g. \"Custom.Phase,Custom.TargetRelease\")")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum items to return")
	return cmd
}

// strField safely extracts a string field from a work item fields map.
func strField(fields map[string]interface{}, key string) string {
	v, ok := fields[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// lastSegment returns the last part of a path split by sep.
func lastSegment(path, sep string) string {
	parts := strings.Split(path, sep)
	if len(parts) == 0 {
		return path
	}
	return parts[len(parts)-1]
}

// stripHTML removes HTML tags from a string for clean terminal output.
func stripHTML(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			result.WriteRune(r)
		}
	}
	// Collapse whitespace
	out := strings.Join(strings.Fields(result.String()), " ")
	return out
}
