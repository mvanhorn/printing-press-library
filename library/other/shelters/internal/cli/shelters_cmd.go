// Copyright 2026 Abe Diaz (@abe238) and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Cobra glue shared by the novel shelters commands: feed loading (--fixture /
// stdin / live), the {source, fetched_at, data} envelope emitter, and human
// rendering. Kept separate from shelters_parse.go so the parsers stay pure and
// network-free for unit testing.

package cli

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// shelterFeed bundles a loaded feed: its source label, the flattened shelters
// (already merged with FEMA_NSS/0 enrichment when applicable), and the
// enrichment status so a consumer can tell genuine nulls from a missed fetch.
type shelterFeed struct {
	Source    string
	Shelters  []Shelter
	Enrich    enrichState
	RedCross  enrichState
	Occupancy enrichState
	Hidden    enrichState
}

// loadShelterFeed returns the OpenShelters feed (the spine). When fixture is set
// it reads the local file (or stdin via "-") and skips enrichment; otherwise it
// fetches live through the generated client with the bound timeout context and
// the standard query params (all open shelters, no geometry, JSON), then
// best-effort merges the FEMA_NSS/0 enrichment fields (unless --no-enrich /
// --data-source local).
func loadShelterFeed(cmd *cobra.Command, flags *rootFlags, fixture string) (shelterFeed, error) {
	if fixture != "" {
		b, err := loadFixture(fixture)
		if err != nil {
			return shelterFeed{}, usageErr(err)
		}
		shelters, perr := parseShelters(b)
		if perr != nil {
			return shelterFeed{}, usageErr(perr)
		}
		return shelterFeed{
			Source:    "fixture:" + fixture,
			Shelters:  shelters,
			Enrich:    enrichState{Note: "Enrichment (FEMA_NSS/0) is skipped in --fixture mode; the fixture is the OpenShelters spine only."},
			RedCross:  enrichState{Note: "Red Cross union is skipped in --fixture mode; the fixture is the OpenShelters spine only."},
			Occupancy: enrichState{Note: "Live occupancy (Open_Shelters) is skipped in --fixture mode; the fixture is the OpenShelters spine only."},
			Hidden:    enrichState{Note: "Red Cross hidden-shelter filtering is skipped in --fixture mode; the fixture is the OpenShelters spine only."},
		}, nil
	}
	c, err := flags.newClient()
	if err != nil {
		return shelterFeed{}, err
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	params := map[string]string{
		"where":          "1=1",
		"outFields":      "*",
		"returnGeometry": "false",
		"f":              "json",
	}
	// Route through the data-source strategy so --data-source, the response
	// cache, and offline local-store fallback all apply. The returned bytes are
	// the full ArcGIS envelope (live/auto) or the bare cached item array
	// (local); parseShelters accepts both shapes.
	data, prov, err := resolveReadWithStrategy(ctx, c, flags, "auto", "shelters", true, openSheltersQuery, params, nil, cmd.ErrOrStderr())
	if err != nil {
		return shelterFeed{}, classifyAPIError(err, flags)
	}
	shelters, perr := parseShelters(data)
	if perr != nil {
		return shelterFeed{}, apiErr(perr)
	}
	source := c.RequestBaseURL() + openSheltersQuery
	if prov.Source == "local" {
		source = "local-store (synced); run 'shelters-pp-cli sync' to refresh"
	}
	feed := shelterFeed{Source: source, Shelters: shelters}
	// Enrich the FEMA spine first (FEMA_NSS/0 fields by shelter_id), THEN union
	// in Red Cross. Enriching before the union keeps enrichment on the FEMA-id'd
	// rows only and lets Red Cross fill any fields still empty afterward (notably
	// coordinates), without an empty enrichment value clobbering a Red Cross one.
	feed.Enrich = applyEnrichment(ctx, flags, feed.Shelters, prov.Source)
	feed.RedCross = applyRedCrossUnion(ctx, flags, &feed, prov.Source)
	// Overlay live occupancy LAST: the Open_Shelters layer is the only feed with a
	// real population, so it fills the headcount/capacity onto FEMA and Red Cross
	// rows alike (joined by name+state+ZIP, since it has no FEMA shelter_id). It is
	// fill-only: Open_Shelters is the Red Cross operational roster and includes
	// sites kept off the public map, so a row with no match in either public feed is
	// withheld rather than added; the CLI shows only publicly listed shelters.
	feed.Occupancy = applyOccupancyOverlay(ctx, flags, &feed, prov.Source)
	// Suppress LAST: drop any shelter the Red Cross keeps off its public map
	// (hide_from_public != 'No'), even one FEMA's public feed lists, so the CLI
	// never surfaces a site the Red Cross has hidden. Conservative by design.
	feed.Hidden = applyHiddenSuppression(ctx, flags, &feed, prov.Source)
	return feed, nil
}

// emitEnvelopeHuman writes the {source, fetched_at, data} envelope, with an
// optional human renderer. The human renderer fires only when wantsHumanTable
// is true (interactive terminal, no machine-format flag); otherwise the JSON
// envelope is emitted. --select operates on the FULL envelope (so the dotted
// paths an agent sees in the output, e.g. data.shelters / source / fetched_at,
// all work); --compact trims the inner payload. --select wins over --compact.
func emitEnvelopeHuman(cmd *cobra.Command, flags *rootFlags, source string, data any, human func() string) error {
	if flags.quiet {
		return nil
	}
	if human != nil && wantsHumanTable(cmd.OutOrStdout(), flags) {
		fmt.Fprint(cmd.OutOrStdout(), human())
		return nil
	}
	var payload any = data
	if flags.selectFields == "" && flags.compact {
		inner, err := json.Marshal(data)
		if err != nil {
			return err
		}
		payload = compactFields(json.RawMessage(inner))
	}
	env := newEnvelope(source, payload)
	raw, err := json.Marshal(env)
	if err != nil {
		return err
	}
	if flags.selectFields != "" {
		raw = filterFields(json.RawMessage(raw), flags.selectFields)
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, raw, "", "  "); err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), pretty.String())
	return nil
}

// emitData emits an already-built typed value through the standard
// printJSONFiltered pipeline (no envelope). Used by credits and gis-links whose
// contract is a bare object rather than the {source, fetched_at, data} envelope.
func emitData(cmd *cobra.Command, flags *rootFlags, v any) error {
	return printJSONFiltered(cmd.OutOrStdout(), v, flags)
}
