package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

const bulkOperationCurrentQuery = `query {
  currentBulkOperation {
    id
    status
    type
    createdAt
    completedAt
    objectCount
    rootObjectCount
    fileSize
    url
    partialDataUrl
    errorCode
  }
}`

const bulkOperationRunQueryMutation = `mutation($query: String!) {
  bulkOperationRunQuery(query: $query) {
    bulkOperation {
      id
      status
      type
      createdAt
    }
    userErrors {
      field
      message
    }
  }
}`

func newBulkOperationsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bulk-operations",
		Short: "Run and inspect Shopify Admin GraphQL bulk operations.",
	}

	cmd.AddCommand(newBulkOperationsCurrentCmd(flags))
	cmd.AddCommand(newBulkOperationsRunQueryCmd(flags))
	return cmd
}

func newBulkOperationsCurrentCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "current",
		Short: "Show the currently running or most recent Shopify bulk operation.",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			data, err := c.Query(bulkOperationCurrentQuery, nil)
			if err == nil && !flags.dryRun {
				data, err = extractGraphQLObject(data, "currentBulkOperation")
			}
			if err != nil {
				return classifyAPIError(err)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}

	return cmd
}

func newBulkOperationsRunQueryCmd(flags *rootFlags) *cobra.Command {
	var queryFile string
	var queryText string

	cmd := &cobra.Command{
		Use:   "run-query",
		Short: "Start a Shopify bulk operation from a GraphQL query string or file.",
		RunE: func(cmd *cobra.Command, args []string) error {
			query := queryText
			if queryFile != "" {
				data, err := os.ReadFile(queryFile)
				if err != nil {
					return fmt.Errorf("reading --query-file: %w", err)
				}
				query = string(data)
			}
			if query == "" {
				if flags.dryRun {
					query = "{ products { edges { node { id title } } } }"
				} else {
					return usageErr(fmt.Errorf("--query or --query-file is required"))
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			data, err := c.Mutate(bulkOperationRunQueryMutation, map[string]any{"query": query})
			if err == nil && !flags.dryRun {
				data, err = extractGraphQLObject(data, "bulkOperationRunQuery")
			}
			if err != nil {
				return classifyAPIError(err)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}

	cmd.Flags().StringVar(&queryFile, "query-file", "", "Path to a GraphQL query file to run as a bulk operation")
	cmd.Flags().StringVar(&queryText, "query", "", "GraphQL query text to run as a bulk operation")
	return cmd
}
