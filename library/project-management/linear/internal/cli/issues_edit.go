package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

func newIssuesEditCmd(flags *rootFlags, dbPath *string) *cobra.Command {
	var titleFlag, descFlag, descFile, assigneeFlag, projectFlag, stateFlag, parentFlag string
	var stateNameFlag, stateTypeFlag string
	var descStdin bool
	var noParentFlag bool
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
description is fetched live and the uploaded media links are appended.

Use --parent with an issue identifier or UUID to set/change parentage. Use
--no-parent to clear parentage.`,
		Example: `  linear-pp-cli issues edit ENG-123 --description-file /tmp/body.md --agent
  linear-pp-cli issues edit ENG-123 --media /tmp/screenshot.png --agent
  linear-pp-cli issues edit ENG-123 --state <state-uuid> --project <project-uuid> --agent
  linear-pp-cli issues edit ENG-123 --state-name "In Progress" --agent
  linear-pp-cli issues edit ENG-123 --state-type started --agent
  linear-pp-cli issues edit ENG-123 --parent ENG-100 --agent
  linear-pp-cli issues edit ENG-123 --no-parent --agent`,
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
			stateSelectors := 0
			for _, v := range []string{stateFlag, stateNameFlag, stateTypeFlag} {
				if v != "" {
					stateSelectors++
				}
			}
			if stateSelectors > 1 {
				return usageErr(fmt.Errorf("pass exactly one of --state, --state-name, or --state-type"))
			}
			if stateFlag != "" {
				if !store.IsUUID(stateFlag) {
					return usageErr(fmt.Errorf("--state expects a workflow state UUID (got %q); use --state-name %q, or run 'linear-pp-cli workflow-states list --team <key>' to find the UUID", stateFlag, stateFlag))
				}
				input["stateId"] = stateFlag
			}
			if stateTypeFlag != "" {
				normalizedType, err := normalizeWorkflowStateType(stateTypeFlag)
				if err != nil {
					return err
				}
				stateTypeFlag = normalizedType
			}
			if parentFlag != "" && noParentFlag {
				return usageErr(fmt.Errorf("pass either --parent or --no-parent, not both"))
			}
			parentRef := parentFlag
			if parentFlag != "" {
				validatedParentRef, validateErr := validateParentIssueRef(parentFlag)
				if validateErr != nil {
					return validateErr
				}
				parentRef = validatedParentRef
				input["parentId"] = parentRef
			}
			if noParentFlag {
				input["parentId"] = nil
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
			if len(input) == 0 && len(mediaFlag) == 0 && stateNameFlag == "" && stateTypeFlag == "" {
				return usageErr(fmt.Errorf("no issue fields supplied; pass --title, --description-file, --media, --state, --state-name, --state-type, --project, --assignee, --priority, --label, --parent, or --no-parent"))
			}
			if flags.dryRun {
				out := map[string]any{"issue": args[0], "input": input}
				if len(mediaFlag) > 0 {
					out["media"] = mediaFlag
					out["media_public"] = mediaPublic
				}
				if stateNameFlag != "" {
					out["state_name"] = stateNameFlag
				}
				if stateTypeFlag != "" {
					out["state_type"] = stateTypeFlag
				}
				return renderMutationDryRun(cmd, flags, "would_update_issue", "issueUpdate", out)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if (len(mediaFlag) > 0 && !descSet) || len(labelsFlag) > 0 || stateNameFlag != "" || stateTypeFlag != "" {
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
			if parentFlag != "" {
				parentID, err := resolveParentIssueID(c, parentRef)
				if err != nil {
					return classifyLiveReadError(err, flags)
				}
				input["parentId"] = parentID
			}
			if len(labelsFlag) > 0 {
				if !issueMetaLoaded {
					return fmt.Errorf("internal error: label validation requires issue metadata")
				}
				if err := validateIssueLabelTeams(c, labelsFlag, issueTeam); err != nil {
					return classifyLiveReadError(err, flags)
				}
			}
			if stateNameFlag != "" || stateTypeFlag != "" {
				if !issueMetaLoaded {
					return fmt.Errorf("internal error: state resolution requires issue metadata")
				}
				stateID, err := resolveWorkflowState(c, issueTeam, stateNameFlag, stateTypeFlag)
				if err != nil {
					return classifyLiveReadError(err, flags)
				}
				input["stateId"] = stateID
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
						parent { id identifier title }
						children { nodes { id identifier title } }
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
	cmd.Flags().StringVar(&stateFlag, "state", "", "Workflow state UUID (see 'workflow-states list --team <key>')")
	cmd.Flags().StringVar(&stateNameFlag, "state-name", "", "Workflow state name (e.g. \"In Progress\"); resolved against the issue's team")
	cmd.Flags().StringVar(&stateTypeFlag, "state-type", "", "Workflow state type (triage, backlog, unstarted, started, completed, canceled, duplicate); resolved against the issue's team")
	cmd.Flags().StringVar(&parentFlag, "parent", "", "Parent issue identifier or UUID")
	cmd.Flags().BoolVar(&noParentFlag, "no-parent", false, "Clear issue parentage")
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
		fmt.Fprintf(os.Stderr, "warning: cannot open ledger at %s: %v\n", dbPath, err)
		return
	}
	defer db.Close()
	if err := db.UpsertIssue(issue.ID, issue.Identifier, issue.Title, raw); err != nil {
		fmt.Fprintf(os.Stderr, "warning: local store write-back failed: %v\n", err)
	}
}
