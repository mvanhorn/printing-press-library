// pp:data-source live
package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type adoComment struct {
	ID          int    `json:"id"`
	CreatedBy   string `json:"created_by"`
	CreatedDate string `json:"created_date"`
	Text        string `json:"text"`
}

func newWitCommentsCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "comments <work-item-id>",
		Short: "Show all comments on a work item",
		Example: strings.Trim(`
  azure-devops-pp-cli wit comments 3640
  azure-devops-pp-cli wit comments 3640 --json
  azure-devops-pp-cli wit comments 3640 --limit 5 --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch work item comments")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("work item ID is required"))
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

			wiID := args[0]
			path := adoAPIPath(flags, flags.project, fmt.Sprintf("wit/workItems/%s/comments", wiID))
			resp, err := c.Get(ctx, path, map[string]string{
				"api-version": "7.1-preview.3",
				"top":         fmt.Sprintf("%d", limit),
			})
			if err != nil {
				return fmt.Errorf("fetching comments for #%s: %w", wiID, err)
			}

			var result struct {
				TotalCount int `json:"totalCount"`
				Comments   []struct {
					ID          int    `json:"id"`
					Text        string `json:"text"`
					CreatedBy   struct {
						DisplayName string `json:"displayName"`
					} `json:"createdBy"`
					CreatedDate string `json:"createdDate"`
					ModifiedDate string `json:"modifiedDate"`
				} `json:"comments"`
			}
			if err := json.Unmarshal(resp, &result); err != nil {
				return fmt.Errorf("parsing comments: %w", err)
			}

			comments := make([]adoComment, 0, len(result.Comments))
			for _, c := range result.Comments {
				date := c.CreatedDate
				if t, err := time.Parse(time.RFC3339Nano, date); err == nil {
					date = t.Format("2006-01-02 15:04")
				}
				comments = append(comments, adoComment{
					ID:          c.ID,
					CreatedBy:   c.CreatedBy.DisplayName,
					CreatedDate: date,
					Text:        stripHTML(c.Text),
				})
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]interface{}{
					"work_item_id": wiID,
					"total_count":  result.TotalCount,
					"comments":     comments,
				})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Work item #%s — %d comment(s)\n\n", wiID, result.TotalCount)
			for _, c := range comments {
				fmt.Fprintf(cmd.OutOrStdout(), "── %s  %s\n%s\n\n",
					c.CreatedBy, c.CreatedDate, c.Text)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of comments to return")
	return cmd
}

type adoRevision struct {
	Rev         int    `json:"rev"`
	ChangedBy   string `json:"changed_by"`
	ChangedDate string `json:"changed_date"`
	State       string `json:"state,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

func newWitHistoryCmd(flags *rootFlags) *cobra.Command {
	var limit int

	cmd := &cobra.Command{
		Use:   "history <work-item-id>",
		Short: "Show revision history for a work item",
		Example: strings.Trim(`
  azure-devops-pp-cli wit history 3640
  azure-devops-pp-cli wit history 3640 --json
  azure-devops-pp-cli wit history 3640 --limit 10 --agent`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would fetch work item revision history")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("work item ID is required"))
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

			wiID := args[0]
			path := adoAPIPath(flags, flags.project, fmt.Sprintf("wit/workItems/%s/revisions", wiID))
			resp, err := c.Get(ctx, path, map[string]string{
				"api-version": "7.1",
				"top":         fmt.Sprintf("%d", limit),
				"$expand":     "fields",
			})
			if err != nil {
				return fmt.Errorf("fetching history for #%s: %w", wiID, err)
			}

			var result struct {
				Count int `json:"count"`
				Value []struct {
					Rev    int `json:"rev"`
					Fields map[string]interface{} `json:"fields"`
				} `json:"value"`
			}
			if err := json.Unmarshal(resp, &result); err != nil {
				return fmt.Errorf("parsing revisions: %w", err)
			}

			revisions := make([]adoRevision, 0, len(result.Value))
			for _, v := range result.Value {
				date := strField(v.Fields, "System.ChangedDate")
				if t, err := time.Parse(time.RFC3339Nano, date); err == nil {
					date = t.Format("2006-01-02 15:04")
				}
				changedByRaw := v.Fields["System.ChangedBy"]
				changedBy := ""
				if m, ok := changedByRaw.(map[string]interface{}); ok {
					if n, ok := m["displayName"].(string); ok {
						changedBy = n
					}
				}
				revisions = append(revisions, adoRevision{
					Rev:         v.Rev,
					ChangedBy:   changedBy,
					ChangedDate: date,
					State:       strField(v.Fields, "System.State"),
					Reason:      strField(v.Fields, "System.Reason"),
				})
			}

			if flags.asJSON || flags.agent || !isTerminal(cmd.OutOrStdout()) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]interface{}{
					"work_item_id":   wiID,
					"total_revisions": result.Count,
					"revisions":       revisions,
				})
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Work item #%s — %d revision(s)\n\n", wiID, result.Count)
			fmt.Fprintf(cmd.OutOrStdout(), "%-4s  %-16s  %-18s  %-25s  %s\n", "Rev", "Date", "Changed By", "State", "Reason")
			fmt.Fprintln(cmd.OutOrStdout(), strings.Repeat("-", 90))
			for _, r := range revisions {
				fmt.Fprintf(cmd.OutOrStdout(), "%-4d  %-16s  %-18s  %-25s  %s\n",
					r.Rev, r.ChangedDate, truncate(r.ChangedBy, 18),
					truncate(r.State, 25), r.Reason)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum number of revisions to return (newest first)")
	return cmd
}
