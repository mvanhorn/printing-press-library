// Copyright 2026 horknfbr and contributors. Licensed under Apache-2.0. See LICENSE.

// pp:data-source local

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/marketing/everbee/internal/store"

	"github.com/spf13/cobra"
)

// listingResult distinguishes the three answers an identity lookup can have.
// An agent must never confuse "this listing does not exist" with "we have no
// data on it yet" — they call for different next steps.
type listingResult struct {
	Input      string          `json:"input"`
	ListingID  int64           `json:"listing_id"`
	Resolved   bool            `json:"resolved"` // we parsed a real listing identity
	HasData    bool            `json:"has_data"` // we hold research evidence for it
	Listing    json.RawMessage `json:"listing,omitempty"`
	ShopName   string          `json:"shop_name,omitempty"`
	Warnings   []string        `json:"warnings"`
	NextAction string          `json:"next_action,omitempty"`
}

// etsyListingID pulls the numeric listing ID out of an Etsy URL
// (https://www.etsy.com/listing/4515173344/some-slug) or accepts a bare ID.
var etsyListingID = regexp.MustCompile(`^(?:https?://[^\s]*etsy\.com/(?:[a-z-]+/)?listing/(\d{6,})(?:[/?#].*)?|(\d{6,}))$`)

func newNovelResearchListingCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "listing [etsy-url-or-listing-id]",
		Short: "Resolve an Etsy listing URL or ID to what we actually know about it, and say so plainly when we know nothing.",
		Long: strings.Trim(`
Resolve an Etsy listing URL or ID against the local research store.

EverBee exposes no listing-detail endpoint, so a listing's identity is resolved
against the listings already synced or researched locally. That makes the honest
answer three-valued, and this command reports which one applies:

  - unresolved: the input is not a recognizable Etsy listing identity
  - resolved, no data: a real listing ID, but nothing has been researched about it
  - resolved, with data: the listing and its metrics

The middle case is the one that matters. The published CLI reported it as an empty
result, which reads as "this listing is worthless" when the truth is "we have not
looked yet". Here it comes back with resolved=true, has_data=false, and the sync
command that would fill the gap.
`, "\n"),
		Example: strings.Trim(`
  everbee-pp-cli research listing 4515173344 --agent
  everbee-pp-cli research listing https://www.etsy.com/listing/4515173344/funny-dad-shirt --json
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "etsy-url-or-listing-id=4515173344",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would resolve the listing identity against the local store")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("an Etsy listing URL or listing ID is required"))
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()

			input := args[0]
			res := listingResult{Input: input, Warnings: []string{}}

			m := etsyListingID.FindStringSubmatch(strings.TrimSpace(input))
			if m == nil {
				res.Warnings = append(res.Warnings,
					"input is not a recognizable Etsy listing URL or ID; nothing was looked up.")
				res.NextAction = "pass a listing ID (e.g. 4515173344) or an Etsy listing URL"
				if flags.asJSON || flags.agent {
					_ = printJSONFiltered(cmd.OutOrStdout(), res, flags)
				} else {
					fmt.Fprintln(cmd.ErrOrStderr(), res.Warnings[0])
				}
				return usageErr(fmt.Errorf("unresolved listing identity: %q", input))
			}
			raw := m[1]
			if raw == "" {
				raw = m[2]
			}
			id, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("listing id %q is not a number", raw))
			}
			res.ListingID = id
			res.Resolved = true

			if dbPath == "" {
				dbPath = defaultDBPath("everbee-pp-cli")
			}
			if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
				res.Warnings = append(res.Warnings,
					fmt.Sprintf("listing %d is a valid identity, but there is no local research store yet, so nothing is known about it.", id))
				res.NextAction = "everbee-pp-cli sync --resources products"
				fmt.Fprintf(cmd.ErrOrStderr(), "no local store at %s\nrun: everbee-pp-cli sync --resources products\n", dbPath)
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			}

			db, err := store.OpenReadOnlyContext(ctx, dbPath)
			if err != nil {
				return fmt.Errorf("opening local store at %s: %w", dbPath, err)
			}
			defer func() { _ = db.Close() }()

			if !hintIfUnsynced(cmd, db, "products") {
				hintIfStale(cmd, db, "products", flags.maxAge)
			}

			// Drain-first: scan the row fully and close before any follow-up query.
			var data sql.NullString
			row := db.DB().QueryRowContext(ctx,
				`SELECT data FROM resources WHERE resource_type IN ('products', 'product_analytics') AND id = ? LIMIT 1`,
				strconv.FormatInt(id, 10))
			switch err := row.Scan(&data); {
			case err == sql.ErrNoRows:
				// Resolved identity, zero evidence — the honest middle case.
				res.HasData = false
				res.Warnings = append(res.Warnings, fmt.Sprintf(
					"listing %d is a valid Etsy listing identity, but no research evidence for it is held locally. This is a gap in our data, not a verdict on the listing.", id))
				res.NextAction = "everbee-pp-cli sync --resources products"
				return printJSONFiltered(cmd.OutOrStdout(), res, flags)
			case err != nil:
				return fmt.Errorf("reading listing %d from local store: %w", id, err)
			}

			res.HasData = true
			if data.Valid {
				res.Listing = json.RawMessage(data.String)
				var probe struct {
					ShopName string `json:"shop_name"`
				}
				if err := json.Unmarshal([]byte(data.String), &probe); err == nil {
					res.ShopName = probe.ShopName
				}
			}
			return printJSONFiltered(cmd.OutOrStdout(), res, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local database path (default: the CLI's data directory)")
	return cmd
}
