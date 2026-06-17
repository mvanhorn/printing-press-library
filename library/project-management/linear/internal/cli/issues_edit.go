package cli

import (
	"encoding/json"
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

func newIssuesEditCmd(flags *rootFlags, dbPath *string) *cobra.Command {
	var titleFlag, descFlag, descFile, assigneeFlag, projectFlag, stateFlag string
	var descStdin bool
	var priorityFlag int
	var labelsFlag []string
	var mediaFlag []string
	var mediaPublic bool
	cmd := &cobra.Command{
		Use:   "edit <issue-id>",
		Short: "Edit a Linear issue",
		Long: `Edit a Linear issue via issueUpdate. Use file/stdin flags for Markdown
descriptions so shell commands, backticks, and GraphQL snippets are preserved
literally. If --media is supplied without a description source, the existing
description is fetched live and the uploaded media links are appended.`,
		Example: `  linear-pp-cli issues edit ENG-123 --description-file /tmp/body.md --agent
  linear-pp-cli issues edit ENG-123 --media /tmp/screenshot.png --agent
  linear-pp-cli issues edit ENG-123 --state <state-uuid> --project <project-uuid> --agent`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			input := map[string]any{}
			var issueID string
			var issueTeam issueTeamInfo
			var issueMetaLoaded bool
			if cmd.Flags().Changed("title") {
				input["title"] = titleFlag
			}
			if cmd.Flags().Changed("priority") {
				input["priority"] = priorityFlag
			}
			if assigneeFlag != "" {
				input["assigneeId"] = assigneeFlag
			}
			if projectFlag != "" {
				input["projectId"] = projectFlag
			}
			if stateFlag != "" {
				input["stateId"] = stateFlag
			}
			if len(labelsFlag) > 0 {
				input["labelIds"] = labelsFlag
			}

			descBody, descSet, err := readMarkdownBody(cmd, markdownBodySpec{
				InlineFlag: "description",
				Inline:     descFlag,
				FileFlag:   "description-file",
				File:       descFile,
				StdinFlag:  "description-stdin",
				Stdin:      descStdin,
				Label:      "description",
			})
			if err != nil {
				return err
			}
			if descSet {
				input["description"] = descBody
			}
			if len(input) == 0 && len(mediaFlag) == 0 {
				return usageErr(fmt.Errorf("no issue fields supplied; pass --title, --description-file, --media, --state, --project, --assignee, --priority, or --label"))
			}
			if flags.dryRun {
				out := map[string]any{"issue": args[0], "input": input}
				if len(mediaFlag) > 0 {
					out["media"] = mediaFlag
					out["media_public"] = mediaPublic
				}
				return renderMutationDryRun(cmd, flags, "would_update_issue", "issueUpdate", out)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if (len(mediaFlag) > 0 && !descSet) || len(labelsFlag) > 0 {
				existing, err := fetchIssueLive(c, args[0])
				if err != nil {
					return classifyLiveReadError(err, flags)
				}
				var issue struct {
					ID          string `json:"id"`
					Description string `json:"description"`
					Team        struct {
						ID  string `json:"id"`
						Key string `json:"key"`
					} `json:"team"`
				}
				if err := json.Unmarshal(existing, &issue); err != nil {
					return fmt.Errorf("parsing existing issue: %w", err)
				}
				if issue.ID == "" {
					return fmt.Errorf("issue %q did not include an id", args[0])
				}
				issueID = issue.ID
				issueTeam = issueTeamInfo{ID: issue.Team.ID, Key: issue.Team.Key}
				issueMetaLoaded = true
				if len(mediaFlag) > 0 && !descSet {
					descBody = issue.Description
					descSet = true
				}
			} else {
				var err error
				issueID, err = resolveIssueID(c, args[0])
				if err != nil {
					return classifyLiveReadError(err, flags)
				}
			}
			if len(labelsFlag) > 0 {
				if !issueMetaLoaded {
					return fmt.Errorf("internal error: label validation requires issue metadata")
				}
				if err := validateIssueLabelTeams(c, labelsFlag, issueTeam); err != nil {
					return classifyLiveReadError(err, flags)
				}
			}
			descBody, uploaded, err := uploadMediaAndAppend(c, descBody, mediaFlag, mediaPublic)
			if err != nil {
				return mediaUploadFailure(err, uploaded)
			}
			if descSet {
				input["description"] = descBody
			}

			const mutation = `mutation($id: String!, $input: IssueUpdateInput!) {
				issueUpdate(id: $id, input: $input) {
					success
					issue {
						id identifier title description url priority estimate dueDate createdAt updatedAt
						state { id name type }
						team { id key name }
						project { id name }
						assignee { id name displayName email }
					}
				}
			}`
			resp, err := c.Mutate(mutation, map[string]any{"id": issueID, "input": input})
			if err != nil {
				return classifyMutationError("issueUpdate", err, flags, uploaded)
			}
			var parsed struct {
				IssueUpdate struct {
					Success bool            `json:"success"`
					Issue   json.RawMessage `json:"issue"`
				} `json:"issueUpdate"`
			}
			if err := json.Unmarshal(resp, &parsed); err != nil {
				return fmt.Errorf("parsing issueUpdate response: %w", err)
			}
			if !parsed.IssueUpdate.Success {
				return apiErr(fmt.Errorf("Linear reported issueUpdate success=false"))
			}
			writeIssueBack(resolveDBPath(*dbPath), parsed.IssueUpdate.Issue)
			return renderLiveObject(cmd, flags, parsed.IssueUpdate.Issue, "issues")
		},
	}
	cmd.Flags().StringVar(&titleFlag, "title", "", "Issue title")
	cmd.Flags().StringVar(&descFlag, "description", "", "Issue description markdown")
	cmd.Flags().StringVar(&descFile, "description-file", "", "Read issue description markdown from file")
	cmd.Flags().BoolVar(&descStdin, "description-stdin", false, "Read issue description markdown from stdin")
	cmd.Flags().IntVar(&priorityFlag, "priority", 0, "Priority: 1=Urgent, 2=High, 3=Medium, 4=Low")
	cmd.Flags().StringVar(&assigneeFlag, "assignee", "", "Assignee user UUID")
	cmd.Flags().StringVar(&projectFlag, "project", "", "Project UUID")
	cmd.Flags().StringVar(&stateFlag, "state", "", "Workflow state UUID")
	cmd.Flags().StringSliceVar(&labelsFlag, "label", nil, "Replacement label UUIDs (repeatable)")
	cmd.Flags().StringSliceVar(&mediaFlag, "media", nil, "Upload file and append it to the description markdown (repeatable)")
	cmd.Flags().BoolVar(&mediaPublic, "media-public", false, "Request public Linear asset URLs for uploaded media")
	return cmd
}

func writeIssueBack(dbPath string, raw json.RawMessage) {
	var issue struct {
		ID         string `json:"id"`
		Identifier string `json:"identifier"`
		Title      string `json:"title"`
	}
	if err := json.Unmarshal(raw, &issue); err != nil || issue.ID == "" {
		return
	}
	db, err := store.Open(dbPath)
	if err != nil {
		return
	}
	defer db.Close()
	_ = db.UpsertIssue(issue.ID, issue.Identifier, issue.Title, raw)
}
