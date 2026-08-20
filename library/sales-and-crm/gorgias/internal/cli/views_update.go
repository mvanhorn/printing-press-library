// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

func newViewsUpdateCmd(flags *rootFlags) *cobra.Command {
	var bodyDecoration string
	var bodyFields string
	var bodyFilters string
	var bodyName string
	var bodyOrderBy string
	var bodyOrderDir string
	var bodySharedWithTeams string
	var bodySharedWithUsers string
	var bodySlug string
	var bodyType string
	var bodyVisibility string
	var stdinBody bool

	cmd := &cobra.Command{
		Use:         "update <id>",
		Short:       "Update a saved view (`id`) — change its filter criteria, name, or sharing.",
		Example:     "  gorgias-pp-cli views update 123456789",
		Annotations: map[string]string{"pp:endpoint": "views.update", "pp:method": "PUT", "pp:path": "/views/{id}"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			path := "/views/{id}"
			path = replacePathParam(path, "id", args[0])
			var body map[string]any
			if stdinBody {
				stdinData, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				var jsonBody map[string]any
				if err := json.Unmarshal(stdinData, &jsonBody); err != nil {
					return fmt.Errorf("parsing stdin JSON: %w", err)
				}
				body = jsonBody
			} else {
				body = map[string]any{}
				if bodyDecoration != "" {
					parsed, err := parseBodyJSONField("decoration", bodyDecoration)
					if err != nil {
						return err
					}
					body["decoration"] = parsed
				}
				if bodyFields != "" {
					parsed, err := parseBodyJSONField("fields", bodyFields)
					if err != nil {
						return err
					}
					body["fields"] = parsed
				}
				if bodyFilters != "" {
					body["filters"] = bodyFilters
				}
				if bodyName != "" {
					body["name"] = bodyName
				}
				if bodyOrderBy != "" {
					body["order_by"] = bodyOrderBy
				}
				if bodyOrderDir != "" {
					body["order_dir"] = bodyOrderDir
				}
				if bodySharedWithTeams != "" {
					parsed, err := parseBodyJSONField("shared_with_teams", bodySharedWithTeams)
					if err != nil {
						return err
					}
					body["shared_with_teams"] = parsed
				}
				if bodySharedWithUsers != "" {
					parsed, err := parseBodyJSONField("shared_with_users", bodySharedWithUsers)
					if err != nil {
						return err
					}
					body["shared_with_users"] = parsed
				}
				if bodySlug != "" {
					body["slug"] = bodySlug
				}
				if bodyType != "" {
					parsed, err := parseBodyJSONField("type", bodyType)
					if err != nil {
						return err
					}
					body["type"] = parsed
				}
				if bodyVisibility != "" {
					parsed, err := parseBodyJSONField("visibility", bodyVisibility)
					if err != nil {
						return err
					}
					body["visibility"] = parsed
				}
			}
			data, statusCode, err := c.Put(path, body)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				// Check if response contains an array (directly or wrapped in "data")
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						fmt.Fprintf(os.Stderr, "warning: table rendering failed, falling back to JSON: %v\n", err)
					} else {
						return nil
					}
				} else {
					var wrapped struct {
						Data []map[string]any `json:"data"`
					}
					if json.Unmarshal(data, &wrapped) == nil && len(wrapped.Data) > 0 {
						if err := printAutoTable(cmd.OutOrStdout(), wrapped.Data); err != nil {
							fmt.Fprintf(os.Stderr, "warning: table rendering failed, falling back to JSON: %v\n", err)
						} else {
							return nil
						}
					}
				}
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				if flags.quiet {
					return nil
				}
				// Apply --compact and --select to the API response before wrapping.
				// --select wins when both are set: explicit field choice trumps the
				// generic high-gravity allow-list. Otherwise --compact still applies
				// when --agent is on but the user did not name fields.
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				envelope := map[string]any{
					"action":   "put",
					"resource": "views",
					"path":     path,
					"status":   statusCode,
					"success":  statusCode >= 200 && statusCode < 300,
				}
				if flags.dryRun {
					envelope["dry_run"] = true
					envelope["status"] = 0
					envelope["success"] = false
				}
				if len(filtered) > 0 {
					var parsed any
					if err := json.Unmarshal(filtered, &parsed); err == nil {
						envelope["data"] = parsed
					}
				}
				envelopeJSON, err := json.Marshal(envelope)
				if err != nil {
					return err
				}
				return printOutput(cmd.OutOrStdout(), json.RawMessage(envelopeJSON), true)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&bodyDecoration, "decoration", "", "Object describing how the view appears in our applications.")
	cmd.Flags().StringVar(&bodyFields, "fields", "", "List of object's attributes to be displayed in the UI of our applications. (example: ['id', 'details', 'tags'])")
	cmd.Flags().StringVar(&bodyFilters, "filters", "", "The logic used to filter the items to be displayed in the view (as JavaScript code).")
	cmd.Flags().StringVar(&bodyName, "name", "", "The name of the view. (example: My view)")
	cmd.Flags().StringVar(&bodyOrderBy, "order-by", "", "Name of the object's attribute used to sort the items of the view. (example: updated_datetime)")
	cmd.Flags().StringVar(&bodyOrderDir, "order-dir", "", "Sort direction of the items displayed in the view. Options: `asc` or `desc`. (enum: asc, desc) (example: desc)")
	cmd.Flags().StringVar(&bodySharedWithTeams, "shared-with-teams", "", "IDs of teams this view is shared with. (example: [1, 2])")
	cmd.Flags().StringVar(&bodySharedWithUsers, "shared-with-users", "", "IDs of users this view is shared with. (example: [1, 2])")
	cmd.Flags().StringVar(&bodySlug, "slug", "", "DEPRECATED - URL-compatible name of the view. (example: my-tickets)")
	cmd.Flags().StringVar(&bodyType, "type", "", "Type of objects the view is applied on. (enum: ticket-list) (example: ticket-list)")
	cmd.Flags().StringVar(&bodyVisibility, "visibility", "", "Visibility of the view. (example: public)")
	cmd.Flags().BoolVar(&stdinBody, "stdin", false, "Read request body as JSON from stdin")

	return cmd
}
