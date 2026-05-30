// Hand-authored helpers shared by the StackAdapt GraphQL commands. No generated
// header: preserved across `generate --force`.
package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/marketing/stackadapt/internal/config"
	"github.com/mvanhorn/printing-press-library/library/marketing/stackadapt/internal/sagraphql"
)

// saClient builds the StackAdapt GraphQL client from the resolved token.
func saClient(flags *rootFlags) (*sagraphql.Client, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, configErr(err)
	}
	if cfg.StackadaptApiToken == "" {
		return nil, authErr(fmt.Errorf("no StackAdapt token: set STACKADAPT_API_TOKEN (a GraphQL API token from your account manager; the legacy REST key will not work)"))
	}
	return sagraphql.New(cfg.StackadaptApiToken, ""), nil
}

// runQuery builds the client and executes a query, returning the `data` payload.
func runQuery(ctx context.Context, flags *rootFlags, query string, vars map[string]any) (json.RawMessage, error) {
	c, err := saClient(flags)
	if err != nil {
		return nil, err
	}
	data, err := c.Query(ctx, query, vars)
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	return data, nil
}

// nodesAt extracts data.<root>.nodes as a slice of raw JSON objects. Works for
// the Relay connection shape StackAdapt uses ({ <root>: { nodes: [...] } }).
func nodesAt(data json.RawMessage, root string) ([]json.RawMessage, json.RawMessage, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(data, &top); err != nil {
		return nil, nil, fmt.Errorf("parsing response: %w", err)
	}
	conn, ok := top[root]
	if !ok {
		return nil, nil, fmt.Errorf("no %q in response", root)
	}
	var connObj map[string]json.RawMessage
	if err := json.Unmarshal(conn, &connObj); err != nil {
		return nil, nil, fmt.Errorf("parsing %q: %w", root, err)
	}
	var nodes []json.RawMessage
	if n, ok := connObj["nodes"]; ok {
		_ = json.Unmarshal(n, &nodes)
	}
	return nodes, connObj["pageInfo"], nil
}

// emitView prints v as JSON, honoring --select/--compact/--quiet.
func emitView(cmd *cobra.Command, flags *rootFlags, v any) error {
	return printJSONFiltered(cmd.OutOrStdout(), v, flags)
}

// emitDryRun reports a dry-run intent as JSON when --json is set (parseable for
// agents and verifiers) and plain text otherwise.
func emitDryRun(cmd *cobra.Command, flags *rootFlags, command, msg string) error {
	if flags.asJSON {
		return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
			"dry_run": true,
			"command": command,
			"message": msg,
		}, flags)
	}
	fmt.Fprintln(cmd.OutOrStdout(), msg)
	return nil
}
