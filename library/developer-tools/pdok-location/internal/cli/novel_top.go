// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// newTopCmd: /free top-1 with a confidence gate. Exits 0 when the top match
// clears --min-score and (optionally) matches --require-type; non-zero
// otherwise. Designed as a pipeline predicate.
func newTopCmd(flags *rootFlags) *cobra.Command {
	var minScore float64
	var requireType string
	cmd := &cobra.Command{
		Use:   "top [query]",
		Short: "Return the single best match for a query if it clears the score threshold",
		Long: "Calls Locatieserver /free with rows=1, then enforces two contracts " +
			"the bare endpoint cannot: --min-score (the Solr `score` field must " +
			"clear the threshold) and --require-type (the matched type must " +
			"equal the requested value). Either failure produces exit code 3 " +
			"(not-found) so the caller can branch.",
		Example: "  pdok-location-pp-cli top 'Hertog Aalbrechtweg 5 1823DL Alkmaar' --min-score 5.0 --require-type adres\n" +
			"  pdok-location-pp-cli top 'Damrak Amsterdam' --json --select id,weergavenaam,score",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,3"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			text := strings.TrimSpace(strings.Join(args, " "))
			if text == "" {
				return usageErr(fmt.Errorf("query text required"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			params := map[string]string{
				"q":    text,
				"rows": "1",
				"fl":   "id type weergavenaam bron straatnaam postcode woonplaatsnaam gemeentenaam provincienaam centroide_ll centroide_rd score",
			}
			if requireType != "" {
				params["fq"] = "type:" + requireType
			}
			data, err := c.Get("/bzk/locatieserver/search/v3_1/free", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var resp lsResponse
			if err := json.Unmarshal(data, &resp); err != nil {
				return apiErr(fmt.Errorf("parse free response: %w", err))
			}
			if resp.Response.NumFound == 0 || len(resp.Response.Docs) == 0 {
				return notFoundErr(fmt.Errorf("no match for %q", text))
			}
			doc := enrichLSDoc(resp.Response.Docs[0], false)
			if doc.Score < minScore {
				return notFoundErr(fmt.Errorf("top score %.2f below --min-score %.2f for %q", doc.Score, minScore, text))
			}
			if requireType != "" && doc.Type != requireType {
				return notFoundErr(fmt.Errorf("top match type %q does not satisfy --require-type %q", doc.Type, requireType))
			}
			return flags.printJSON(cmd, doc)
		},
	}
	cmd.Flags().Float64Var(&minScore, "min-score", 0, "Minimum Solr score the top match must clear (0 disables)")
	cmd.Flags().StringVar(&requireType, "require-type", "", "Require the top match to have this type (adres, weg, gemeente, woonplaats, postcode, perceel, hectometerpaal)")
	return cmd
}
