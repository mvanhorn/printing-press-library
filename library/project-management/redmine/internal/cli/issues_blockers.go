// Copyright 2026 and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/project-management/redmine/internal/cliutil"
)

type blockerNode struct {
	ID      int64  `json:"id"`
	Subject string `json:"subject"`
	Status  string `json:"status"`
	Depth   int    `json:"depth"`
	Blocks  int64  `json:"blocks"`
}

type blockerFetchFailure struct {
	IssueID int64  `json:"issue_id"`
	Error   string `json:"error"`
}

type blockersView struct {
	IssueID       int64                 `json:"issue_id"`
	Blockers      []blockerNode         `json:"blockers"`
	IssuesScanned int                   `json:"issues_scanned"`
	MaxDepth      int                   `json:"max_depth"`
	FetchFailures []blockerFetchFailure `json:"fetch_failures,omitempty"`
	Note          string                `json:"note,omitempty"`
}

const issuesBlockersMaxVisited = 200

func newNovelIssuesBlockersCmd(flags *rootFlags) *cobra.Command {
	var flagDepth int

	cmd := &cobra.Command{
		Use:     "blockers <issue_id>",
		Short:   "Walk the full multi-hop 'blocks'/'blocked by' dependency chain for an issue, not just its direct relations.",
		Long:    "Use this for the full transitive dependency chain across multiple issues. Do NOT use it for a single issue's direct relations; use 'issues relations-json get-issue-relations <id>' instead (one hop only).",
		Example: "  redmine-pp-cli issues blockers 3 --depth 3 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "issue_id=3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "issues blockers")
			}
			if flags.dataSource == "local" {
				return usageErr(fmt.Errorf("issues blockers has no local data source; drop --data-source local"))
			}
			if len(args) < 1 || args[0] == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("issue_id is required\nUsage: %s <issue_id>", cmd.CommandPath()))
			}
			startID, parseErr := strconv.ParseInt(args[0], 10, 64)
			if parseErr != nil || startID <= 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("issue_id must be a positive integer, got %q", args[0]))
			}
			maxDepth := flagDepth
			if maxDepth <= 0 {
				maxDepth = 5
			}
			if cliutil.IsDogfoodEnv() && maxDepth > 2 {
				maxDepth = 2
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type relation struct {
				ID           int64  `json:"id"`
				IssueID      int64  `json:"issue_id"`
				IssueToID    int64  `json:"issue_to_id"`
				RelationType string `json:"relation_type"`
			}
			type issueEnvelope struct {
				Issue struct {
					ID      int64  `json:"id"`
					Subject string `json:"subject"`
					Status  struct {
						Name string `json:"name"`
					} `json:"status"`
					Relations []relation `json:"relations"`
				} `json:"issue"`
			}

			// blockedBy maps a discovered blocker issue ID to the issue ID it
			// directly blocks (its parent in the BFS), so the output can report
			// each blocker's target without a second lookup.
			visited := map[int64]bool{startID: true}
			blockedBy := map[int64]int64{}
			frontier := []int64{startID}
			blockers := make([]blockerNode, 0)
			fetchFailures := make([]blockerFetchFailure, 0)
			scanned := 0
			scanCapHit := false

			for depth := 1; depth <= maxDepth && len(frontier) > 0; depth++ {
				var next []int64
				for _, issueID := range frontier {
					if scanned >= issuesBlockersMaxVisited {
						scanCapHit = true
						break
					}
					path := replacePathParam("/issues/{issue_id}.json", "issue_id", strconv.FormatInt(issueID, 10))
					body, err := c.Get(ctx, path, map[string]string{"include": "relations"})
					scanned++
					if err != nil {
						fetchFailures = append(fetchFailures, blockerFetchFailure{IssueID: issueID, Error: err.Error()})
						continue
					}
					var env issueEnvelope
					if err := json.Unmarshal(body, &env); err != nil {
						fetchFailures = append(fetchFailures, blockerFetchFailure{IssueID: issueID, Error: fmt.Sprintf("parsing response: %v", err)})
						continue
					}
					for _, rel := range env.Issue.Relations {
						if rel.RelationType != "blocks" || rel.IssueToID != issueID {
							continue
						}
						blockerID := rel.IssueID
						if visited[blockerID] {
							continue
						}
						visited[blockerID] = true
						blockedBy[blockerID] = issueID
						next = append(next, blockerID)
					}
				}
				if scanCapHit {
					break
				}
				// Fetch subject/status for the newly discovered blockers at this depth.
				for _, blockerID := range next {
					if scanned >= issuesBlockersMaxVisited {
						scanCapHit = true
						break
					}
					path := replacePathParam("/issues/{issue_id}.json", "issue_id", strconv.FormatInt(blockerID, 10))
					body, err := c.Get(ctx, path, nil)
					scanned++
					if err != nil {
						fetchFailures = append(fetchFailures, blockerFetchFailure{IssueID: blockerID, Error: err.Error()})
						continue
					}
					var env issueEnvelope
					if err := json.Unmarshal(body, &env); err != nil {
						fetchFailures = append(fetchFailures, blockerFetchFailure{IssueID: blockerID, Error: fmt.Sprintf("parsing response: %v", err)})
						continue
					}
					blockers = append(blockers, blockerNode{
						ID: env.Issue.ID, Subject: env.Issue.Subject, Status: env.Issue.Status.Name, Depth: depth,
						Blocks: blockedBy[blockerID],
					})
				}
				frontier = next
			}

			view := blockersView{IssueID: startID, Blockers: blockers, IssuesScanned: scanned, MaxDepth: maxDepth, FetchFailures: fetchFailures}
			switch {
			case scanCapHit:
				view.Note = fmt.Sprintf("stopped after scanning %d issues (fixed safety cap); narrow --depth to see a partial chain sooner", scanned)
			case len(fetchFailures) > 0:
				view.Note = fmt.Sprintf("%d issue(s) could not be fetched during the walk; the chain below may be incomplete", len(fetchFailures))
			case len(blockers) == 0:
				view.Note = fmt.Sprintf("issue #%d has no blockers within depth %d", startID, maxDepth)
			}

			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			if view.Note != "" {
				fmt.Fprintln(cmd.OutOrStdout(), view.Note)
			}
			for _, b := range blockers {
				fmt.Fprintf(cmd.OutOrStdout(), "depth %d: #%d [%s] %s (blocks #%d)\n", b.Depth, b.ID, b.Status, b.Subject, b.Blocks)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&flagDepth, "depth", 5, "Maximum hops to walk the blocker chain")
	return cmd
}
