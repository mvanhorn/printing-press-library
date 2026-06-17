package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func renderMutationDryRun(cmd *cobra.Command, flags *rootFlags, event, mutation string, fields map[string]any) error {
	out := map[string]any{
		"event":    event,
		"mutation": mutation,
	}
	for key, value := range fields {
		out[key] = value
	}
	if flags.asJSON {
		enc := json.NewEncoder(cmd.OutOrStdout())
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "%s\n", event)
	return nil
}
