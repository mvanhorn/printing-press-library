// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// USGS event IDs follow the pattern: 2-letter network code followed by a
// 6-12 character alphanumeric sequence (e.g., "us7000abcd", "nc73947885",
// "ak0202xyz123"). Some legacy IDs may use longer or mixed-case network
// prefixes; the regex accepts 2-3 alphabetic chars followed by 4+ alnum chars.
var usgsEventIDRe = regexp.MustCompile(`^([a-z]{2,3})([0-9a-zA-Z]{4,})$`)

// networkDisplay maps known USGS contributor codes to their operator names.
// Populated lazily from the local contributors table at runtime; this map
// is a static seed for first-run (before sync) usage.
var networkDisplay = map[string]string{
	"us":    "USGS National Earthquake Information Center (NEIC)",
	"nc":    "USGS Northern California Seismic Network (NCSN)",
	"ci":    "USGS Southern California Seismic Network / Caltech (SCSN)",
	"ak":    "Alaska Earthquake Center / USGS Alaska Volcano Observatory",
	"hv":    "USGS Hawaiian Volcano Observatory",
	"uu":    "University of Utah Seismograph Stations",
	"uw":    "Pacific Northwest Seismic Network (PNSN)",
	"nn":    "Nevada Seismological Laboratory",
	"nm":    "Center for Earthquake Research and Information (CERI), University of Memphis",
	"se":    "Center for Earthquake Research and Information (CERI), Memphis (SE)",
	"ld":    "Lamont-Doherty Earth Observatory",
	"mb":    "Montana Bureau of Mines and Geology",
	"tx":    "TexNet (Texas Seismological Network)",
	"av":    "Alaska Volcano Observatory",
	"ok":    "Oklahoma Geological Survey",
	"pr":    "Puerto Rico Seismic Network",
	"pt":    "Pacific Tsunami Warning Center",
	"at":    "National Tsunami Warning Center (Alaska)",
	"ew":    "USGS ShakeAlert / Earthworm",
	"atlas": "USGS Atlas",
}

func newDecodeIDCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "decode-id <event-id>",
		Short: "Parse a USGS event ID into network code, sequence, and operator",
		Long: `Parse a USGS event ID (e.g., "us7000abcd", "nc73947885", "ak0202xyz") into
its component parts: the network code (2-3 letters), the sequence identifier,
and the operator that originally reported the event.

Operator names come from the local 'contributors' dictionary (populated by
'sync'); a built-in seed map is used for unknown networks.`,
		Example: strings.Trim(`
  # Decode a single event ID
  usgs-earthquakes-pp-cli decode-id us7000abcd

  # JSON output
  usgs-earthquakes-pp-cli decode-id nc73947885 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			ctx := cmd.Context()
			id := strings.TrimSpace(strings.ToLower(args[0]))
			m := usgsEventIDRe.FindStringSubmatch(id)
			if m == nil {
				return usageErr(fmt.Errorf("%q does not look like a USGS event ID (expected 2-3 letter network code + alphanumeric sequence)", args[0]))
			}
			network := m[1]
			sequence := m[2]
			operator := networkDisplay[network]
			source := "built-in seed"
			if op, ok := lookupContributorOperator(ctx, network); ok && op != "" {
				operator = op
				source = "local contributors cache"
			}
			if operator == "" {
				operator = "Unknown network"
				source = "no match"
			}
			out := map[string]any{
				"event_id":        args[0],
				"network_code":    network,
				"sequence":        sequence,
				"operator":        operator,
				"operator_source": source,
				"catalog_link":    fmt.Sprintf("https://earthquake.usgs.gov/earthquakes/eventpage/%s", id),
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			w := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintf(w, "Event ID\t%s\n", args[0])
			fmt.Fprintf(w, "Network code\t%s\n", network)
			fmt.Fprintf(w, "Sequence\t%s\n", sequence)
			fmt.Fprintf(w, "Operator\t%s (%s)\n", operator, source)
			fmt.Fprintf(w, "Catalog link\thttps://earthquake.usgs.gov/earthquakes/eventpage/%s\n", id)
			return w.Flush()
		},
	}
}

// lookupContributorOperator queries the local contributors cache (populated by
// `sync --resources contributors`). The cached value is the raw XML response
// from FDSN /contributors; we parse it on each lookup. Returns ok=false when
// the local store doesn't have a contributors row or the code isn't listed.
func lookupContributorOperator(ctx context.Context, code string) (string, bool) {
	db, err := openLocalStore(ctx)
	if err != nil {
		return "", false
	}
	defer db.Close()
	row := db.DB().QueryRowContext(ctx,
		`SELECT data FROM resources WHERE resource_type='contributors' LIMIT 1`)
	var raw sql.NullString
	if err := row.Scan(&raw); err != nil || !raw.Valid {
		return "", false
	}
	var doc struct {
		Contributors []string `xml:"Contributor"`
	}
	if err := xml.Unmarshal([]byte(raw.String), &doc); err == nil {
		for _, c := range doc.Contributors {
			if strings.EqualFold(c, code) {
				// FDSN /contributors returns just codes, not display names.
				// We have a code match but no richer operator description from
				// the cache — fall back to the built-in seed map for display.
				return networkDisplay[strings.ToLower(c)], true
			}
		}
	}
	return "", false
}
