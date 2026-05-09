package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newQueryCmd() *cobra.Command {
	var operation, variablesJSON string
	cmd := &cobra.Command{
		Use:   "query <file.graphql>",
		Short: "Run a read-only GraphQL query from a file for advanced/debug workflows.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			b, err := os.ReadFile(args[0])
			if err != nil {
				return err
			}
			query := string(b)
			if strings.Contains(strings.ToLower(query), "mutation") {
				return fmt.Errorf("query refuses GraphQL mutations")
			}
			vars := map[string]any{}
			if strings.TrimSpace(variablesJSON) != "" {
				if err := json.Unmarshal([]byte(variablesJSON), &vars); err != nil {
					return fmt.Errorf("invalid --variables JSON: %w", err)
				}
			}
			data, err := graphql(operation, query, vars)
			if err != nil {
				return err
			}
			return printJSON(data)
		},
	}
	cmd.Flags().StringVar(&operation, "operation", "", "GraphQL operation name")
	cmd.Flags().StringVar(&variablesJSON, "variables", "", "GraphQL variables as a JSON object")
	return cmd
}
