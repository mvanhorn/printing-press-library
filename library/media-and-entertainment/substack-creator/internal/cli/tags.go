package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

// newTagsCmd is the parent command for Substack publication tags
// (GET/POST /api/v1/publication/post-tag — per-publication subdomain).
func newTagsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tags",
		Short: "List or create publication post-tags.",
		Long: `Substack lets you tag posts with publication-scoped tags. Use 'tags list'
to fetch them, 'tags create' to add a new one. Both require --subdomain.`,
	}
	cmd.AddCommand(newTagsListCmd(flags))
	cmd.AddCommand(newTagsCreateCmd(flags))
	return cmd
}

func newTagsListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List all post-tags for a publication.",
		Example:     "  substack-creator-pp-cli tags list --subdomain mypub --json",
		Annotations: map[string]string{"pp:endpoint": "tags.list", "pp:method": "GET", "pp:path": "/publication/post-tag", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.Get("/publication/post-tag", nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	return cmd
}

func newTagsCreateCmd(flags *rootFlags) *cobra.Command {
	var name, slug string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new post-tag for the publication.",
		Example: `  substack-creator-pp-cli tags create --subdomain mypub --name "Markets" --slug markets --json`,
		Annotations: map[string]string{"pp:endpoint": "tags.create", "pp:method": "POST", "pp:path": "/publication/post-tag"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if name == "" {
				return usageErr(nil)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body := map[string]any{"name": name}
			if slug != "" {
				body["slug"] = slug
			}
			resp, status, err := c.Post("/publication/post-tag", body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			envelope := map[string]any{
				"action":   "post",
				"resource": "tags",
				"path":     "/publication/post-tag",
				"status":   status,
				"success":  status >= 200 && status < 300,
			}
			if len(resp) > 0 {
				var v any
				if json.Unmarshal(resp, &v) == nil {
					envelope["data"] = v
				}
			}
			out, _ := json.Marshal(envelope)
			return printOutput(cmd.OutOrStdout(), json.RawMessage(out), true)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Tag display name (required)")
	cmd.Flags().StringVar(&slug, "slug", "", "Tag URL slug (optional)")
	return cmd
}
