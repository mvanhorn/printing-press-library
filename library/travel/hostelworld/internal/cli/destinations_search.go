// Copyright 2026 zjsng and contributors. Licensed under Apache-2.0.
// pp:data-source live
package cli

import (
	"encoding/json"
	"fmt"
	hw "github.com/mvanhorn/printing-press-library/library/travel/hostelworld/internal/hostelworld"
	"github.com/spf13/cobra"
	"time"
)

func newDestinationsSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	cmd := &cobra.Command{Use: "search [query]", Short: "Resolve cities and properties without assuming an ambiguous name", Example: "  " + "hostelworld-pp-cli destinations search Osaka --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "query=Osaka"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 || len([]rune(args[0])) < 2 || len([]rune(args[0])) > 100 {
			return usageErr(fmt.Errorf("search requires one destination query of 2–100 characters"))
		}
		if limit < 1 || limit > 50 {
			return usageErr(fmt.Errorf("--limit must be 1–50"))
		}
		ctx, cancel := boundCtx(cmd.Context(), flags)
		defer cancel()
		c, err := flags.newClient()
		if err != nil {
			return err
		}
		data, err := c.Get(ctx, "/autocomplete-service/v1/autocomplete/web", map[string]string{"text": args[0], "v": "control"})
		if err != nil {
			return classifyAPIError(cmd.OutOrStdout(), err, flags)
		}
		var rows []map[string]any
		if err := json.Unmarshal(data, &rows); err != nil || rows == nil {
			return fmt.Errorf("destination response schema changed: expected array")
		}
		normalized := []any{}
		for _, r := range rows {
			if hw.Text(r["name"]) == "" {
				return fmt.Errorf("destination response schema changed: name missing")
			}
			normalized = append(normalized, map[string]any{"id": r["id"], "name": r["name"], "english_name": r["englishName"], "type": r["type"], "city": r["city"]})
			if len(normalized) == limit {
				break
			}
		}
		return flags.printJSON(cmd, map[string]any{"query": args[0], "results": normalized, "returned": len(normalized), "source_count": len(rows), "truncated": len(rows) > limit, "observed_at": time.Now().UTC().Format(time.RFC3339), "coverage": "provider suggestions only; select an exact city ID before searching"})
	}}
	decoratePlanning(cmd, flags)
	cmd.Flags().IntVar(&limit, "limit", 15, "Maximum returned source suggestions, 1–50")
	return cmd
}
