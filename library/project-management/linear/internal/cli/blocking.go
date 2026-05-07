package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

// newBlockingCmd shows issues you (or a named user) are blocking — i.e.,
// issues whose `inverseRelations` chain to another open issue assigned
// to someone else. Sorted by downstream impact.
func newBlockingCmd(flags *rootFlags) *cobra.Command {
	var jsonOut bool
	var assigneeFlag string
	cmd := &cobra.Command{
		Use:   "blocking",
		Short: "Show issues you are blocking — sorted by downstream impact",
		Long: `Lists issues that block other open issues. Walks Linear's issue-relation
graph live (the local store doesn't snapshot relations) and counts how many
downstream issues (and their priority) each blocker has. Devon's daily
ritual: 'what's stuck because of me?'

This command makes a live GraphQL query — no sync required, but it does
hit the API rate-limit budget.`,
		Example: `  linear-pp-cli blocking
  linear-pp-cli blocking --assignee me --json
  linear-pp-cli blocking --assignee 0a3a... --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Determine target user ID
			userID := assigneeFlag
			if userID == "" || userID == "me" {
				var viewer struct {
					Viewer struct {
						ID string `json:"id"`
					} `json:"viewer"`
				}
				if err := c.QueryInto(`query { viewer { id } }`, nil, &viewer); err != nil {
					return fmt.Errorf("fetching viewer: %w", err)
				}
				userID = viewer.Viewer.ID
			}

			// Query issues assigned to the user with their outbound
			// "blocks" relations expanded. We use the issue.relations
			// connection filtered to type=blocks; downstream issue is
			// .relatedIssue.
			const q = `query Blocking($assignee: ID!) {
				issues(filter: {assignee: {id: {eq: $assignee}}, state: {type: {nin: ["completed", "canceled"]}}}, first: 100) {
					nodes {
						id
						identifier
						title
						priority
						state { name type }
						relations(first: 50) {
							nodes {
								type
								relatedIssue {
									id
									identifier
									title
									priority
									state { name type }
									assignee { displayName name }
								}
							}
						}
					}
				}
			}`

			var resp struct {
				Issues struct {
					Nodes []struct {
						ID         string `json:"id"`
						Identifier string `json:"identifier"`
						Title      string `json:"title"`
						Priority   int    `json:"priority"`
						State      struct {
							Name string `json:"name"`
							Type string `json:"type"`
						} `json:"state"`
						Relations struct {
							Nodes []struct {
								Type         string `json:"type"`
								RelatedIssue struct {
									ID         string `json:"id"`
									Identifier string `json:"identifier"`
									Title      string `json:"title"`
									Priority   int    `json:"priority"`
									State      struct {
										Name string `json:"name"`
										Type string `json:"type"`
									} `json:"state"`
									Assignee struct {
										DisplayName string `json:"displayName"`
										Name        string `json:"name"`
									} `json:"assignee"`
								} `json:"relatedIssue"`
							} `json:"nodes"`
						} `json:"relations"`
					} `json:"nodes"`
				} `json:"issues"`
			}
			if err := c.QueryInto(q, map[string]any{"assignee": userID}, &resp); err != nil {
				return fmt.Errorf("blocking query: %w", err)
			}

			type row struct {
				Identifier      string           `json:"identifier"`
				Title           string           `json:"title"`
				State           string           `json:"state"`
				Priority        string           `json:"priority"`
				DownstreamCount int              `json:"downstream_count"`
				ImpactScore     float64          `json:"impact_score"`
				Downstream      []map[string]any `json:"downstream"`
			}

			var rows []row
			for _, n := range resp.Issues.Nodes {
				var downstream []map[string]any
				for _, r := range n.Relations.Nodes {
					if r.Type != "blocks" {
						continue
					}
					if r.RelatedIssue.State.Type == "completed" || r.RelatedIssue.State.Type == "canceled" {
						continue
					}
					assignee := r.RelatedIssue.Assignee.DisplayName
					if assignee == "" {
						assignee = r.RelatedIssue.Assignee.Name
					}
					downstream = append(downstream, map[string]any{
						"identifier": r.RelatedIssue.Identifier,
						"title":      r.RelatedIssue.Title,
						"priority":   priorityLabel(r.RelatedIssue.Priority),
						"state":      r.RelatedIssue.State.Name,
						"assignee":   assignee,
					})
				}
				if len(downstream) == 0 {
					continue
				}
				// Impact score: count + priority weighting (urgent=4, high=3, med=2, low=1, none=0.5)
				score := 0.0
				for _, d := range downstream {
					switch d["priority"] {
					case "URG":
						score += 4
					case "HI":
						score += 3
					case "MED":
						score += 2
					case "LOW":
						score += 1
					default:
						score += 0.5
					}
				}
				rows = append(rows, row{
					Identifier:      n.Identifier,
					Title:           n.Title,
					State:           n.State.Name,
					Priority:        priorityLabel(n.Priority),
					DownstreamCount: len(downstream),
					ImpactScore:     score,
					Downstream:      downstream,
				})
			}
			sort.Slice(rows, func(i, j int) bool {
				if rows[i].ImpactScore != rows[j].ImpactScore {
					return rows[i].ImpactScore > rows[j].ImpactScore
				}
				return rows[i].Identifier < rows[j].Identifier
			})

			if jsonOut || flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"assignee_id":    userID,
					"blocking_count": len(rows),
					"items":          rows,
				})
			}

			if len(rows) == 0 {
				fmt.Println("Nothing blocking other open issues. Nice!")
				return nil
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "ID\tPRI\tDOWNSTREAM\tIMPACT\tTITLE")
			for _, r := range rows {
				title := r.Title
				if len(title) > 40 {
					title = title[:37] + "..."
				}
				fmt.Fprintf(tw, "%s\t%s\t%d\t%.1f\t%s\n", r.Identifier, r.Priority, r.DownstreamCount, r.ImpactScore, title)
			}
			tw.Flush()
			totalDS := 0
			for _, r := range rows {
				totalDS += r.DownstreamCount
			}
			fmt.Fprintf(os.Stderr, "\n%d issues blocking %d downstream issues\n", len(rows), totalDS)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json-out", false, "Force JSON output (alias for --json)")
	cmd.Flags().StringVar(&assigneeFlag, "assignee", "me", "User UUID or 'me' to target a specific user's blocking queue")
	return cmd
}
