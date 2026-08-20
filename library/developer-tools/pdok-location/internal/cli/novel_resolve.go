// Copyright 2026 markvandeven and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// newResolveCmd: suggest→lookup chain collapsed into a single command. Takes
// imprecise user text, calls /suggest top-1, then /lookup with that id, and
// returns the full canonical record. When --geojson is set the WKT geometry
// from /lookup is converted to a GeoJSON geometry object.
func newResolveCmd(flags *rootFlags) *cobra.Command {
	var asGeoJSON bool
	var typeFilter string
	cmd := &cobra.Command{
		Use:   "resolve [text]",
		Short: "Resolve free-text input to a canonical PDOK record (suggest → lookup)",
		Long: "Run the Dutch autocomplete chain in one call: /suggest returns the " +
			"best id for the input text, then /lookup fetches the full record. " +
			"With --geojson the WKT geometry is converted to a GeoJSON geometry " +
			"object so the output drops cleanly into mapping tools.",
		Example: "  pdok-location-pp-cli resolve 'Damrak Amsterdam' --geojson\n" +
			"  pdok-location-pp-cli resolve 'Hertog Aalbrechtweg 5 1823DL Alkmaar' --type adres",
		Annotations: map[string]string{"mcp:read-only": "true"},
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

			// Step 1: /suggest, top 1 result.
			sugParams := map[string]string{
				"q":    text,
				"rows": "1",
				"fl":   "id type weergavenaam score",
			}
			if typeFilter != "" {
				sugParams["fq"] = "type:" + typeFilter
			}
			sugData, err := c.Get("/bzk/locatieserver/search/v3_1/suggest", sugParams)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var sug lsResponse
			if err := json.Unmarshal(sugData, &sug); err != nil {
				return apiErr(fmt.Errorf("parse suggest response: %w", err))
			}
			if sug.Response.NumFound == 0 || len(sug.Response.Docs) == 0 {
				return notFoundErr(fmt.Errorf("no suggestion matched %q", text))
			}
			top := enrichLSDoc(sug.Response.Docs[0], false)
			if top.ID == "" {
				return apiErr(fmt.Errorf("suggest response missing id field"))
			}

			// Step 2: /lookup that id.
			lupData, err := c.Get("/bzk/locatieserver/search/v3_1/lookup", map[string]string{
				"id": top.ID,
				"fl": "*",
			})
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var lup lsResponse
			if err := json.Unmarshal(lupData, &lup); err != nil {
				return apiErr(fmt.Errorf("parse lookup response: %w", err))
			}
			if lup.Response.NumFound == 0 || len(lup.Response.Docs) == 0 {
				return notFoundErr(fmt.Errorf("lookup returned no record for id %s", top.ID))
			}
			doc := enrichLSDoc(lup.Response.Docs[0], false)

			// Build the result map. Always include the canonical fields and
			// the structured centroid; geometry conversion is opt-in via
			// --geojson because some lookup results have huge MULTIPOLYGON
			// bodies that bloat agent context.
			out := map[string]any{
				"query":         text,
				"suggest_score": top.Score,
				"id":            doc.ID,
				"weergavenaam":  doc.Weergavenaam,
				"type":          doc.Type,
				"bron":          doc.Bron,
				"centroide_ll":  doc.CentroideLL,
				"centroide_rd":  doc.CentroideRD,
			}
			if doc.Straatnaam != "" {
				out["straatnaam"] = doc.Straatnaam
			}
			if doc.Postcode != "" {
				out["postcode"] = doc.Postcode
			}
			if doc.Woonplaatsnaam != "" {
				out["woonplaatsnaam"] = doc.Woonplaatsnaam
			}
			if doc.Gemeentenaam != "" {
				out["gemeentenaam"] = doc.Gemeentenaam
			}
			if doc.Provincienaam != "" {
				out["provincienaam"] = doc.Provincienaam
			}
			if asGeoJSON {
				if wkt, ok := doc.GeometrieLL.(string); ok && wkt != "" {
					if g, err := wktToGeoJSON(wkt); err == nil {
						out["geometrie_ll"] = g
					} else {
						out["geometrie_ll_wkt"] = wkt
					}
				}
			} else if doc.GeometrieLL != nil {
				out["geometrie_ll_wkt"] = doc.GeometrieLL
			}

			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().BoolVar(&asGeoJSON, "geojson", false, "Convert WKT geometry to GeoJSON in the output")
	cmd.Flags().StringVar(&typeFilter, "type", "", "Restrict the suggest call to one type (adres, weg, gemeente, woonplaats, postcode, perceel, hectometerpaal)")
	return cmd
}
