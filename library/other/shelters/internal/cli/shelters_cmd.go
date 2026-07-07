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

// loadShelterFeed returns (source, shelters) for the OpenShelters feed. When
// fixture is set it reads the local file (or stdin via "-"); otherwise it
// fetches live through the generated client with the bound timeout context and
// the standard query params (all open shelters, no geometry, JSON).
func loadShelterFeed(cmd *cobra.Command, flags *rootFlags, fixture string) (string, []Shelter, error) {
	if fixture != "" {
		b, err := loadFixture(fixture)
		if err != nil {
			return "", nil, usageErr(err)
		}
		shelters, perr := parseShelters(b)
		if perr != nil {
			return "", nil, usageErr(perr)
		}
		return "fixture:" + fixture, shelters, nil
	}
	c, err := flags.newClient()
	if err != nil {
		return "", nil, err
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
		return "", nil, classifyAPIError(err, flags)
	}
	shelters, perr := parseShelters(data)
	if perr != nil {
		return "", nil, apiErr(perr)
	}
	source := c.RequestBaseURL() + openSheltersQuery
	if prov.Source == "local" {
		source = "local-store (synced); run 'shelters-pp-cli sync' to refresh"
	}
	return source, shelters, nil
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
