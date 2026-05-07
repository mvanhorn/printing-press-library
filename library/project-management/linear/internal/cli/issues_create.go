package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/project-management/linear/internal/store"

	"github.com/spf13/cobra"
)

// newIssuesCreateCmd is registered as a subcommand of "issues" via wireIssuesCreate
// in init(). Calls Linear's issueCreate mutation and records the resulting issue
// into the local pp_created ledger so pp-cleanup can find it later.
func newIssuesCreateCmd(flags *rootFlags) *cobra.Command {
	var titleFlag, teamFlag, descFlag, assigneeFlag, projectFlag, stateFlag string
	var priorityFlag int
	var labelsFlag []string
	var dbPath string
	var session string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new Linear issue and record it in the pp_created ledger",
		Long: `Create a Linear issue via the issueCreate mutation. The new issue's ID is
written to the local pp_created table along with a session tag, so pp-test
list shows it and pp-cleanup can archive it without touching pre-existing
tickets in the workspace.`,
		Example: `  # Quick test ticket in team ENG
  linear-pp-cli issues create --title "pp-test sanity" --team ENG

  # Dry-run (shows the GraphQL request without sending)
  linear-pp-cli issues create --title "x" --team ENG --dry-run

  # JSON output (agent-mode)
  linear-pp-cli issues create --title "x" --team ENG --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if titleFlag == "" {
				return fmt.Errorf("--title is required")
			}
			if teamFlag == "" {
				return fmt.Errorf("--team is required (team key like ENG or team UUID)")
			}
			// trust-mode strict requires the create call to include a session
			// or the explicit --pp-test marker so the resulting fixture is
			// always recoverable by pp-cleanup.
			if flags.trustMode == "strict" {
				sess := resolvePPSession(flags, session)
				if sess == "" {
					return fmt.Errorf("trust-mode=strict: pass --session <tag> (or set PP_SESSION env) so this fixture is recoverable by pp-cleanup")
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			// Resolve team key/name to UUID via the local store if possible.
			teamID := teamFlag
			if dbPath == "" {
				dbPath = defaultDBPath("linear-pp-cli")
			}
			if db, dbErr := store.Open(dbPath); dbErr == nil {
				defer db.Close()
				if resolved, ok := resolveTeamID(db, teamFlag); ok {
					teamID = resolved
				}
			}

			input := map[string]any{
				"title":  titleFlag,
				"teamId": teamID,
			}
			if descFlag != "" {
				input["description"] = descFlag
			}
			if priorityFlag > 0 {
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

			const mutation = `mutation CreateIssue($input: IssueCreateInput!) {
				issueCreate(input: $input) {
					success
					issue { id identifier title url team { key } state { name } }
				}
			}`

			if flags.dryRun {
				out := map[string]any{
					"event":    "would_create_issue",
					"mutation": "issueCreate",
					"input":    input,
				}
				if flags.asJSON {
					enc := json.NewEncoder(os.Stdout)
					enc.SetIndent("", "  ")
					return enc.Encode(out)
				}
				fmt.Printf("Would create issue: title=%q team=%s\n", titleFlag, teamID)
				return nil
			}

			resp, err := c.Mutate(mutation, map[string]any{"input": input})
			if err != nil {
				return fmt.Errorf("issueCreate failed: %w", err)
			}
			var parsed struct {
				IssueCreate struct {
					Success bool `json:"success"`
					Issue   struct {
						ID         string `json:"id"`
						Identifier string `json:"identifier"`
						Title      string `json:"title"`
						URL        string `json:"url"`
						Team       struct {
							Key string `json:"key"`
						} `json:"team"`
						State struct {
							Name string `json:"name"`
						} `json:"state"`
					} `json:"issue"`
				} `json:"issueCreate"`
			}
			if err := json.Unmarshal(resp, &parsed); err != nil {
				return fmt.Errorf("parsing issueCreate response: %w", err)
			}
			if !parsed.IssueCreate.Success {
				return fmt.Errorf("Linear reported issueCreate success=false")
			}

			sess := resolvePPSession(flags, session)
			if sess == "" || sess == "current" {
				sess = ppCurrentSession()
			}
			if db, dbErr := store.Open(dbPath); dbErr == nil {
				defer db.Close()
				if recErr := db.RecordPPFixture(parsed.IssueCreate.Issue.ID, parsed.IssueCreate.Issue.Identifier, parsed.IssueCreate.Issue.Title, sess); recErr != nil {
					fmt.Fprintf(os.Stderr, "warning: pp_created ledger write failed: %v\n", recErr)
				}
			} else {
				fmt.Fprintf(os.Stderr, "warning: cannot open ledger at %s: %v\n", dbPath, dbErr)
			}

			if flags.asJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]any{
					"event":      "issue_created",
					"identifier": parsed.IssueCreate.Issue.Identifier,
					"id":         parsed.IssueCreate.Issue.ID,
					"title":      parsed.IssueCreate.Issue.Title,
					"team":       parsed.IssueCreate.Issue.Team.Key,
					"state":      parsed.IssueCreate.Issue.State.Name,
					"url":        parsed.IssueCreate.Issue.URL,
					"session":    sess,
				})
			}
			fmt.Printf("Created %s — %s\n", parsed.IssueCreate.Issue.Identifier, parsed.IssueCreate.Issue.Title)
			fmt.Printf("  URL: %s\n", parsed.IssueCreate.Issue.URL)
			fmt.Printf("  Recorded in pp_created (session=%s) for safe pp-cleanup.\n", sess)
			return nil
		},
	}
	cmd.Flags().StringVar(&titleFlag, "title", "", "Issue title (required)")
	cmd.Flags().StringVar(&teamFlag, "team", "", "Team key (e.g. ENG) or team UUID (required)")
	cmd.Flags().StringVar(&descFlag, "description", "", "Issue description (markdown)")
	cmd.Flags().IntVar(&priorityFlag, "priority", 0, "Priority: 1=Urgent, 2=High, 3=Medium, 4=Low (0=None)")
	cmd.Flags().StringVar(&assigneeFlag, "assignee", "", "Assignee user UUID")
	cmd.Flags().StringVar(&projectFlag, "project", "", "Project UUID")
	cmd.Flags().StringVar(&stateFlag, "state", "", "Workflow state UUID")
	cmd.Flags().StringSliceVar(&labelsFlag, "label", nil, "Label UUIDs (repeatable)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (for team-key resolution and pp_created ledger)")
	cmd.Flags().StringVar(&session, "session", "", "Session tag (defaults to PP_SESSION env or current run timestamp)")
	return cmd
}

// resolveTeamID maps a team key (ENG, OPS) to a team UUID using the local
// teams cache. Returns ("", false) if the key isn't recognized — in that
// case the caller passes through the user's input unchanged (it may already
// be a UUID).
func resolveTeamID(db *store.Store, keyOrID string) (string, bool) {
	teams, err := db.ListTeams()
	if err != nil {
		return "", false
	}
	for _, raw := range teams {
		var t struct {
			ID  string `json:"id"`
			Key string `json:"key"`
		}
		if err := json.Unmarshal(raw, &t); err != nil {
			continue
		}
		if t.Key == keyOrID || t.ID == keyOrID {
			return t.ID, true
		}
	}
	return "", false
}
