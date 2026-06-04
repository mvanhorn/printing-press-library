// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored DICE output helpers (not generated; survives generate --force).
package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"
)

// dicePerPage is the GraphQL connection page size used by list/sync fetches.
const dicePerPage = 100

// diceDefaultListLimit caps interactive list commands so a single invocation
// against a large account does not page the whole connection. Pass --limit 0
// to fetch every page.
const diceDefaultListLimit = 200

// outputNodes emits a slice of GraphQL node payloads as a JSON array, routed
// through the generated output pipeline so --json / --csv / --select /
// --compact / --quiet all work. An empty result renders as [].
func outputNodes(cmd *cobra.Command, flags *rootFlags, nodes []json.RawMessage) error {
	if len(nodes) == 0 {
		return printOutputWithFlags(cmd.OutOrStdout(), json.RawMessage("[]"), flags)
	}
	arr, err := json.Marshal(nodes)
	if err != nil {
		return err
	}
	return printOutputWithFlags(cmd.OutOrStdout(), arr, flags)
}
