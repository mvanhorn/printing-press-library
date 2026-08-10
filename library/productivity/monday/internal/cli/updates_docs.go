// Updates and docs commands for monday.com.

package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

const queryUpdatesList = `query($limit: Int!, $page: Int!) {
  updates(limit: $limit, page: $page) {
    id
    body
    text_body
    created_at
    updated_at
    item_id
    creator_id
    replies { id body text_body created_at creator_id }
  }
}`

const queryItemUpdates = `query($ids: [ID!], $limit: Int!) {
  items(ids: $ids) {
    updates(limit: $limit) {
      id
      body
      text_body
      created_at
      updated_at
      item_id
      creator_id
    }
  }
}`

const mutationUpdatesCreate = `mutation($item_id: ID!, $body: String!) {
  create_update(item_id: $item_id, body: $body) {
    id
    body
    item_id
    creator_id
  }
}`

const queryDocsList = `query($limit: Int!, $page: Int!) {
  docs(limit: $limit, page: $page) {
    id
    object_id
    name
    doc_kind
    created_at
    created_by { id name }
    workspace_id
  }
}`

const queryDocGet = `query($ids: [ID!]) {
  docs(ids: $ids) {
    id
    object_id
    name
    doc_kind
    created_at
    created_by { id name }
    workspace_id
    blocks { id type content }
  }
}`

func newUpdatesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "updates",
		Short: "List and create item updates (the comment threads).",
	}
	cmd.AddCommand(newUpdatesListCmd(flags))
	cmd.AddCommand(newUpdatesCreateCmd(flags))
	return cmd
}

func newUpdatesListCmd(flags *rootFlags) *cobra.Command {
	var limit, page int
	var itemID string
	cmd := &cobra.Command{
		Use:     "list [--item <id>]",
		Short:   "List updates, optionally filtered to one item.",
		Example: "  monday-pp-cli updates list --item 12345 --json --select id,body,creator_id",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if itemID != "" {
				data, err := c.GraphQL(queryItemUpdates, map[string]any{"ids": []string{itemID}, "limit": limit})
				if err != nil {
					return classifyAPIError(err, flags)
				}
				if dryRunOK(flags) {
					return nil
				}
				items, err := pluckJSONField(data, "items")
				if err != nil {
					return err
				}
				view, err := pluckFirstFromArrayObject(items, "updates")
				if err != nil {
					return err
				}
				return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
			}
			data, err := c.GraphQL(queryUpdatesList, map[string]any{"limit": limit, "page": page})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			view, err := pluckJSONField(data, "updates")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Page size.")
	cmd.Flags().IntVar(&page, "page", 1, "Page number.")
	cmd.Flags().StringVar(&itemID, "item", "", "Filter to a single item ID (uses items.updates query).")
	return cmd
}

func newUpdatesCreateCmd(flags *rootFlags) *cobra.Command {
	var itemID, body string
	var fromStdin bool
	cmd := &cobra.Command{
		Use:     "create --item <id> --body <text>",
		Short:   "Add an update (comment) to an item.",
		Long:    "Pass body via --body, or pipe markdown via stdin with --stdin.",
		Example: "  monday-pp-cli updates create --item 12345 --body \"Stage moved to In Review\" --dry-run",
		RunE: func(cmd *cobra.Command, args []string) error {
			if itemID == "" {
				return usageErr(fmt.Errorf("--item is required"))
			}
			if fromStdin {
				b, err := io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("reading stdin: %w", err)
				}
				body = string(b)
			}
			if body == "" {
				return usageErr(fmt.Errorf("--body or --stdin is required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.GraphQL(mutationUpdatesCreate, map[string]any{"item_id": itemID, "body": body})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			view, err := pluckJSONField(data, "create_update")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&itemID, "item", "", "Item ID (required).")
	cmd.Flags().StringVar(&body, "body", "", "Update body (markdown allowed).")
	cmd.Flags().BoolVar(&fromStdin, "stdin", false, "Read the update body from stdin.")
	return cmd
}

func newDocsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "List and get monday.com docs (in-product wiki pages).",
	}
	cmd.AddCommand(newDocsListCmd(flags))
	cmd.AddCommand(newDocsGetCmd(flags))
	return cmd
}

func newDocsListCmd(flags *rootFlags) *cobra.Command {
	var limit, page int
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List docs the API token can see.",
		Example: "  monday-pp-cli docs list --json --select id,name,workspace_id",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.GraphQL(queryDocsList, map[string]any{"limit": limit, "page": page})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			view, err := pluckJSONField(data, "docs")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Page size.")
	cmd.Flags().IntVar(&page, "page", 1, "Page number.")
	return cmd
}

func newDocsGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "get <id>",
		Short:   "Fetch one doc (with blocks) by ID.",
		Example: "  monday-pp-cli docs get 12345 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if err := requireNumericID("doc id", args[0]); err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, err := c.GraphQL(queryDocGet, map[string]any{"ids": []string{args[0]}})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if dryRunOK(flags) {
				return nil
			}
			view, err := pluckFirstFromArrayField(data, "docs")
			if err != nil {
				return err
			}
			return printOutputWithFlags(cmd.OutOrStdout(), view, flags)
		},
	}
	return cmd
}
