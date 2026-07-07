package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newProjectUpdatesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "project-updates",
		Short:       "List and create Linear project updates",
		Annotations: map[string]string{"pp:typed-exit-codes": "0,2,3,4,5,7"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newProjectUpdatesListCmd(flags))
	cmd.AddCommand(newProjectUpdatesCreateCmd(flags))
	return cmd
}

func newProjectUpdatesListCmd(flags *rootFlags) *cobra.Command {
	var project, projectName string
	var limit int
	var after string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List project updates for a Linear project",
		Example: `  linear-pp-cli project-updates list --project <project-uuid> --agent
  linear-pp-cli project-updates list --project-name "My Project" --limit 10 --agent`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" && projectName == "" {
				return usageErr(fmt.Errorf("pass --project <uuid> or --project-name <name>"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			projectID, err := resolveProjectFlag(c, project, projectName, "", flags)
			if err != nil {
				return classifyLiveReadError(err, flags)
			}
			if projectID == "" {
				return usageErr(fmt.Errorf("could not resolve project; pass --project <uuid> or --project-name <name>"))
			}
			if limit <= 0 {
				limit = 25
			}
			const query = `query($projectId: String!, $first: Int!, $after: String) {
				project(id: $projectId) {
					id name
					projectUpdates(first: $first, after: $after) {
						nodes {
							id body health createdAt updatedAt url
							user { id name displayName email }
						}
						pageInfo { hasNextPage endCursor }
					}
				}
			}`
			var resp struct {
				Project struct {
					ID   string `json:"id"`
					Name string `json:"name"`
					ProjectUpdates struct {
						Nodes    []json.RawMessage `json:"nodes"`
						PageInfo struct {
							HasNextPage bool   `json:"hasNextPage"`
							EndCursor   string `json:"endCursor"`
						} `json:"pageInfo"`
					} `json:"projectUpdates"`
				} `json:"project"`
			}
			vars := map[string]any{"projectId": projectID, "first": limit, "after": nil}
			if after != "" {
				vars["after"] = after
			}
			if err := c.QueryInto(query, vars, &resp); err != nil {
				return classifyAPIError(err, flags)
			}
			out, err := json.Marshal(map[string]any{
				"project": map[string]any{
					"id":   resp.Project.ID,
					"name": resp.Project.Name,
				},
				"projectUpdates": resp.Project.ProjectUpdates.Nodes,
				"pageInfo":       resp.Project.ProjectUpdates.PageInfo,
			})
			if err != nil {
				return err
			}
			return renderLivePayload(cmd, flags, out, "project-updates", false)
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "Project UUID")
	cmd.Flags().StringVar(&projectName, "project-name", "", "Project name (resolved live to UUID)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum project updates to return")
	cmd.Flags().StringVar(&after, "after", "", "Cursor from pageInfo.endCursor for the next page")
	return cmd
}

func newProjectUpdatesCreateCmd(flags *rootFlags) *cobra.Command {
	var project, projectName string
	var bodyFlag, bodyFile string
	var bodyStdin bool
	var health string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new Linear project update",
		Long: `Create a project update (status post) on a Linear project via the
projectUpdateCreate mutation. The update carries a markdown body and an optional
health value (onTrack, atRisk, offTrack).

Use --body-file or --body-stdin for multi-line Markdown so shell metacharacters
stay literal.`,
		Example: `  linear-pp-cli project-updates create --project <project-uuid> --body-file /tmp/update.md --health onTrack --agent
  linear-pp-cli project-updates create --project-name "My Project" --body "Sprint went well." --health onTrack --agent
  linear-pp-cli project-updates create --project <project-uuid> --body-stdin --health atRisk --agent < /tmp/update.md
  linear-pp-cli project-updates create --project <project-uuid> --body "Blocked on infra." --health offTrack --dry-run`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if project == "" && projectName == "" {
				return usageErr(fmt.Errorf("pass --project <uuid> or --project-name <name>"))
			}
			if health != "" {
				switch health {
				case "onTrack", "atRisk", "offTrack":
					// valid enum values from Linear GraphQL schema
				default:
					return usageErr(fmt.Errorf("invalid --health value %q: must be onTrack, atRisk, or offTrack", health))
				}
			}
			body, bodySet, err := readMarkdownBody(cmd, markdownBodySpec{
				InlineFlag: "body",
				Inline:     bodyFlag,
				FileFlag:   "body-file",
				File:       bodyFile,
				StdinFlag:  "body-stdin",
				Stdin:      bodyStdin,
				Label:      "body",
			})
			if err != nil {
				return err
			}
			if !bodySet && health == "" {
				return usageErr(fmt.Errorf("specify at least one of --body, --body-file, --body-stdin, or --health"))
			}
			input := map[string]any{}
			if bodySet {
				input["body"] = body
			}
			if health != "" {
				input["health"] = health
			}
			if project != "" {
				input["projectId"] = project
			}
			if flags.dryRun {
				dryOut := map[string]any{
					"projectName": projectName,
					"input":       input,
				}
				return renderMutationDryRun(cmd, flags, "would_create_project_update", "projectUpdateCreate", dryOut)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			projectID, err := resolveProjectFlag(c, project, projectName, "", flags)
			if err != nil {
				return classifyLiveReadError(err, flags)
			}
			if projectID == "" {
				return usageErr(fmt.Errorf("could not resolve project; pass --project <uuid> or --project-name <name>"))
			}
			input["projectId"] = projectID
			const mutation = `mutation($input: ProjectUpdateCreateInput!) {
				projectUpdateCreate(input: $input) {
					success
					projectUpdate {
						id body health createdAt updatedAt url
						user { id name displayName email }
						project { id name }
					}
				}
			}`
			resp, err := c.Mutate(mutation, map[string]any{"input": input})
			if err != nil {
				return classifyMutationError("projectUpdateCreate", err, flags, nil)
			}
			projectUpdate, err := extractMutationObject(resp, "projectUpdateCreate", "projectUpdate")
			if err != nil {
				return err
			}
			return renderLiveObject(cmd, flags, projectUpdate, "project-updates")
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "Project UUID")
	cmd.Flags().StringVar(&projectName, "project-name", "", "Project name (resolved live to UUID)")
	cmd.Flags().StringVar(&bodyFlag, "body", "", "Project update body markdown")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "Read project update body markdown from file")
	cmd.Flags().BoolVar(&bodyStdin, "body-stdin", false, "Read project update body markdown from stdin")
	cmd.Flags().StringVar(&health, "health", "", "Project health status: onTrack, atRisk, or offTrack")
	return cmd
}
