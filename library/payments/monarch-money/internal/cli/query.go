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
			if containsMutationOperation(query) {
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

func containsMutationOperation(query string) bool {
	depth := 0
	for _, token := range graphqlTokens(query) {
		switch token {
		case "{":
			depth++
		case "}":
			if depth > 0 {
				depth--
			}
		case "mutation":
			if depth == 0 {
				return true
			}
		}
	}
	return false
}

func graphqlTokens(query string) []string {
	tokens := []string{}
	inString := false
	inBlockString := false
	for i := 0; i < len(query); {
		if inBlockString {
			if strings.HasPrefix(query[i:], `"""`) {
				inBlockString = false
				i += 3
				continue
			}
			i++
			continue
		}
		if inString {
			if query[i] == '\\' && i+1 < len(query) {
				i += 2
				continue
			}
			if query[i] == '"' {
				inString = false
			}
			i++
			continue
		}
		if strings.HasPrefix(query[i:], `"""`) {
			inBlockString = true
			i += 3
			continue
		}
		switch c := query[i]; {
		case c == '#':
			for i < len(query) && query[i] != '\n' {
				i++
			}
		case c == '"':
			inString = true
			i++
		case c == '{' || c == '}':
			tokens = append(tokens, string(c))
			i++
		case isGraphQLNameStart(c):
			start := i
			i++
			for i < len(query) && isGraphQLNameContinue(query[i]) {
				i++
			}
			tokens = append(tokens, strings.ToLower(query[start:i]))
		default:
			i++
		}
	}
	return tokens
}

func isGraphQLNameStart(c byte) bool {
	return c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

func isGraphQLNameContinue(c byte) bool {
	return isGraphQLNameStart(c) || (c >= '0' && c <= '9')
}
