// Hand-authored tags delete command. Wraps the Monarch GraphQL mutation
// `Common_DeleteHouseholdTransactionTag` discovered via HAR capture from the
// Monarch web app — it takes a single scalar `tagId: ID!` argument (no input
// wrapper, unlike most delete mutations in the schema).

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/payments/monarch/internal/client"
)

func newTagsDeleteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "delete <tag_id>",
		Short:       "Delete a transaction tag",
		Example:     "  monarch-pp-cli tags delete 243399414671595971",
		Annotations: map[string]string{"pp:endpoint": "tags.delete", "pp:method": "POST", "pp:path": "/graphql"},
		Args:        cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, statusCode, err := gqlPost(c, client.TagsDeleteMutation, map[string]any{"tagId": args[0]})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			envelope := map[string]any{
				"action":   "post",
				"resource": "tags",
				"path":     "/graphql",
				"status":   statusCode,
				"success":  statusCode >= 200 && statusCode < 300,
			}
			if len(data) > 0 {
				var parsed any
				if err := json.Unmarshal(data, &parsed); err == nil {
					envelope["data"] = parsed
				}
			}
			out, err := json.MarshalIndent(envelope, "", "  ")
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(out))
			return nil
		},
	}
	return cmd
}
