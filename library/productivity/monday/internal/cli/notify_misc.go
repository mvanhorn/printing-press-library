// Notifications, gql passthrough, and schema introspection for monday.com.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

const mutationCreateNotification = `mutation($user_id: ID!, $target_id: ID!, $text: String!, $target_type: NotificationTargetType!) {
  create_notification(user_id: $user_id, target_id: $target_id, text: $text, target_type: $target_type) {
    id
    text
  }
}`

const queryGraphQLSchema = `query {
  __schema {
    queryType { name }
    mutationType { name }
    types { name kind description }
  }
}`

func newNotifyCmd(flags *rootFlags) *cobra.Command {
	var userID, targetID, text, targetType string
	cmd := &cobra.Command{
		Use:     "notify --user <id> --target-id <id> --text <body>",
		Short:   "Send a notification to a monday.com user.",
		Long:    "Send a free-form notification (in-product bell) to one user, anchored to an item or update.",
		Example: "  monday-pp-cli notify --user 12345 --target-id 99999 --target-type Project --text \"Standup at 9am\" --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.GraphQL(mutationCreateNotification, map[string]any{
				"user_id":     userID,
				"target_id":   targetID,
				"text":        text,
				"target_type": targetType,
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			view, err := pluckJSONField(data, "create_notification")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&userID, "user", "", "Recipient user ID (required).")
	cmd.Flags().StringVar(&targetID, "target-id", "", "Target object ID — item or update (required).")
	cmd.Flags().StringVar(&text, "text", "", "Notification body (required).")
	cmd.Flags().StringVar(&targetType, "target-type", "Project", "Target type: Project (item) or Post (update).")
	_ = cmd.MarkFlagRequired("user")
	_ = cmd.MarkFlagRequired("target-id")
	_ = cmd.MarkFlagRequired("text")
	return cmd
}

func newGQLCmd(flags *rootFlags) *cobra.Command {
	var queryFile, operation, varsJSON string
	var fromStdin bool
	cmd := &cobra.Command{
		Use:     "gql [query]",
		Short:   "Run a raw GraphQL query or mutation against monday.com.",
		Long:    "Pass a query via --query <file>, --stdin, or as a positional. Variables can be supplied via --vars '<JSON>'. Output is the full GraphQL data envelope as JSON.",
		Example: "  monday-pp-cli gql --query ./me.graphql --json\n  echo 'query { me { id name } }' | monday-pp-cli gql --stdin",
		RunE: func(cmd *cobra.Command, args []string) error {
			var query string
			switch {
			case queryFile != "":
				b, err := os.ReadFile(queryFile)
				if err != nil {
					return fmt.Errorf("reading %s: %w", queryFile, err)
				}
				query = string(b)
			case fromStdin:
				b, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				query = string(b)
			case len(args) > 0:
				query = args[0]
			default:
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("provide a query via --query <file>, --stdin, or as a positional argument"))
			}
			vars := map[string]any{}
			if varsJSON != "" {
				if err := json.Unmarshal([]byte(varsJSON), &vars); err != nil {
					return usageErr(fmt.Errorf("parsing --vars JSON: %w", err))
				}
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.GraphQL(query, vars)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			_ = operation
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&queryFile, "query", "", "Path to a file containing a GraphQL query.")
	cmd.Flags().StringVar(&operation, "operation", "", "Operation name (when the query defines multiple).")
	cmd.Flags().StringVar(&varsJSON, "vars", "", "Variables as a JSON object (e.g. '{\"id\":12345}').")
	cmd.Flags().BoolVar(&fromStdin, "stdin", false, "Read the query from stdin.")
	return cmd
}

func newSchemaCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "schema",
		Short:   "Introspect monday.com's GraphQL schema (cached locally).",
		Example: "  monday-pp-cli schema --json | jq '.types[] | select(.name==\"Item\")'",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.GraphQL(queryGraphQLSchema, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			view, err := pluckJSONField(data, "__schema")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	return cmd
}
