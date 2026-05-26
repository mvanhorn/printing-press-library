// Copyright 2026 markvandeven. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// newPerceelCmd parents the cadastral perceel commands. The interesting bit
// is `perceel lookup --aanduiding "AMR03 N 1234"` which parses a Dutch
// kadastrale aanduiding (kadastrale gemeente / sectie / perceelnummer) and
// queries Locatieserver /free with type:perceel filters.
func newPerceelCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "perceel",
		Short: "Look up Dutch cadastral parcels (BRK/DKK) by aanduiding or id",
		Long: "Helpers around perceel records from the BRK (Basisregistratie Kadaster). " +
			"Use `perceel lookup --aanduiding` to resolve a kadastrale aanduiding " +
			"like 'AMR03 N 1234' into a perceel record with centroid coords.",
	}
	cmd.AddCommand(newPerceelLookupCmd(flags))
	return cmd
}

// aanduidingRE matches a kadastrale aanduiding in the standard format:
// <kadastrale-gemeente-code (3-5 letters + optional digits)> <sectie (single
// letter A-Z)> <perceelnummer (1-7 digits)>. Whitespace is flexible.
//
// Examples:
//   - "AMR03 N 1234" -> gemeente=AMR03, sectie=N, nummer=1234
//   - "ADS00 A 9999" -> gemeente=ADS00, sectie=A, nummer=9999
var aanduidingRE = regexp.MustCompile(`^([A-Z]{2,5}[0-9]{0,3})\s+([A-Z])\s+([0-9]{1,7})$`)

func parseAanduiding(s string) (gem, sectie string, nummer int, err error) {
	s = strings.ToUpper(strings.Join(strings.Fields(s), " "))
	m := aanduidingRE.FindStringSubmatch(s)
	if m == nil {
		return "", "", 0, fmt.Errorf("not a kadastrale aanduiding (expected '<KAD-GEM> <SECTIE> <NUMMER>', e.g. 'AMR03 N 1234'): %q", s)
	}
	var n int
	fmt.Sscanf(m[3], "%d", &n)
	return m[1], m[2], n, nil
}

func newPerceelLookupCmd(flags *rootFlags) *cobra.Command {
	var aanduiding string
	var id string
	cmd := &cobra.Command{
		Use:   "lookup",
		Short: "Look up a perceel by kadastrale aanduiding or id",
		Long: "Resolve a Dutch parcel by its kadastrale aanduiding " +
			"(--aanduiding 'AMR03 N 1234') or by id (--id perceel-<uuid>). " +
			"The aanduiding form parses the three components and queries " +
			"Locatieserver /free with type:perceel filters.",
		Example:     "  pdok-location-pp-cli perceel lookup --aanduiding 'ASD02 A 4332' --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !cmd.Flags().Changed("aanduiding") && !cmd.Flags().Changed("id") {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			if id != "" {
				data, err := c.Get("/bzk/locatieserver/search/v3_1/lookup", map[string]string{
					"id": id,
					"fl": "*",
				})
				if err != nil {
					return classifyAPIError(err, flags)
				}
				var resp lsResponse
				if err := json.Unmarshal(data, &resp); err != nil {
					return apiErr(err)
				}
				if resp.Response.NumFound == 0 {
					return notFoundErr(fmt.Errorf("no perceel for id %s", id))
				}
				return flags.printJSON(cmd, enrichLSDoc(resp.Response.Docs[0], false))
			}

			gem, sectie, nummer, err := parseAanduiding(aanduiding)
			if err != nil {
				return usageErr(err)
			}
			params := map[string]string{
				"q":    fmt.Sprintf("kadastrale_gemeentecode:%s AND kadastrale_sectie:%s AND perceelnummer:%d", gem, sectie, nummer),
				"fq":   "type:perceel",
				"rows": "1",
				"fl":   "id type weergavenaam bron kadastrale_aanduiding kadastrale_gemeentecode kadastrale_gemeentenaam kadastrale_sectie perceelnummer kadastrale_grootte gemeentenaam provincienaam centroide_ll centroide_rd score",
			}
			data, err := c.Get("/bzk/locatieserver/search/v3_1/free", params)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			var resp lsResponse
			if err := json.Unmarshal(data, &resp); err != nil {
				return apiErr(err)
			}
			if resp.Response.NumFound == 0 {
				return notFoundErr(fmt.Errorf("no perceel for aanduiding %q (parsed as gemeente=%s sectie=%s nummer=%d)", aanduiding, gem, sectie, nummer))
			}
			doc := enrichLSDoc(resp.Response.Docs[0], false)
			out := map[string]any{
				"aanduiding": map[string]any{
					"input":               aanduiding,
					"kadastrale_gemeente": gem,
					"sectie":              sectie,
					"perceelnummer":       nummer,
				},
				"perceel": doc,
			}
			return flags.printJSON(cmd, out)
		},
	}
	cmd.Flags().StringVar(&aanduiding, "aanduiding", "", "Kadastrale aanduiding, e.g. 'AMR03 N 1234'")
	cmd.Flags().StringVar(&id, "id", "", "Direct perceel id (perceel-<uuid>)")
	return cmd
}
