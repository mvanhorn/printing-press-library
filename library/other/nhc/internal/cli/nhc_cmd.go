// Copyright 2026 Abe Diaz (@abe238) and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Cobra glue shared by the novel NHC commands: input loading (--fixture /
// stdin / live), the {source, fetched_at, data} envelope emitter, and human
// rendering. Kept separate from nhc_parse.go so the parsers stay pure and
// network-free for unit testing.

package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// loadStormFeed returns (source, rawBytes) for the CurrentStorms feed. When
// fixture is set it reads the local file (or stdin via "-"); otherwise it
// fetches live through the generated client with the bound timeout context.
func loadStormFeed(cmd *cobra.Command, flags *rootFlags, fixture string) (string, []byte, error) {
	if fixture != "" {
		b, err := loadFixture(fixture)
		if err != nil {
			return "", nil, usageErr(err)
		}
		return "fixture:" + fixture, b, nil
	}
	c, err := flags.newClient()
	if err != nil {
		return "", nil, err
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	data, err := c.Get(ctx, "/CurrentStorms.json", nil)
	if err != nil {
		return "", nil, classifyAPIError(err, flags)
	}
	return c.RequestBaseURL() + "/CurrentStorms.json", data, nil
}

// emitEnvelope writes the {source, fetched_at, data} envelope. In machine mode
// the inner data is filtered by --select/--compact BEFORE nesting under data
// (so an agent can narrow the payload), then the whole envelope is emitted as
// JSON. When stdout is an interactive terminal and no machine-format flag is
// set, a caller-supplied human renderer is used instead.
//
// Note (documented deviation): because acceptance-tests.md asserts every value
// under `.data.*`, --select narrows the data sub-tree via dotted paths
// (e.g. --select data.name) rather than top-level keys.
func emitEnvelope(cmd *cobra.Command, flags *rootFlags, source string, data any) error {
	return emitEnvelopeHuman(cmd, flags, source, data, nil)
}

// emitEnvelopeHuman is emitEnvelope with an optional human renderer. The human
// renderer fires only when wantsHumanTable is true (interactive terminal, no
// machine-format flag); otherwise the JSON envelope is emitted.
func emitEnvelopeHuman(cmd *cobra.Command, flags *rootFlags, source string, data any, human func() string) error {
	if flags.quiet {
		return nil
	}
	if human != nil && wantsHumanTable(cmd.OutOrStdout(), flags) {
		fmt.Fprint(cmd.OutOrStdout(), human())
		return nil
	}
	// Apply --select/--compact to the inner data first (mirrors promoted_storms).
	inner, err := json.Marshal(data)
	if err != nil {
		return err
	}
	filtered := json.RawMessage(inner)
	if flags.selectFields != "" {
		filtered = filterFields(filtered, flags.selectFields)
	} else if flags.compact {
		filtered = compactFields(filtered)
	}
	env := newEnvelope(source, filtered)
	out, err := json.MarshalIndent(env, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), string(out))
	return nil
}

// emitData emits an already-built typed value through the standard
// printJSONFiltered pipeline (no envelope). Used by commands whose contract is
// a bare object rather than the {source, fetched_at, data} envelope (credits).
func emitData(cmd *cobra.Command, flags *rootFlags, v any) error {
	return printJSONFiltered(cmd.OutOrStdout(), v, flags)
}

// loadOutlookBody returns the TWO product body for a basin. When fixture is set
// it reads the local file; otherwise it fetches the corrected basin text URL.
func loadOutlookBody(cmd *cobra.Command, flags *rootFlags, basin, fixture string) ([]byte, error) {
	if fixture != "" {
		b, err := loadFixture(fixture)
		if err != nil {
			return nil, usageErr(err)
		}
		return b, nil
	}
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	path, ok := twoTextURL(basin)
	if !ok {
		return nil, usageErr(fmt.Errorf("invalid basin %q", basin))
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	body, err := httpGetText(ctx, c.RequestBaseURL()+path)
	if err != nil {
		return nil, apiErr(err)
	}
	return body, nil
}

// loadAlerts returns the parsed active tropical alerts. When fixture is set it
// parses the local GeoJSON; otherwise it fetches the single-call OR filter from
// api.weather.gov (the client always sends the mandatory User-Agent).
func loadAlerts(cmd *cobra.Command, flags *rootFlags, fixture string) (*alertsResult, error) {
	if fixture != "" {
		b, err := loadFixture(fixture)
		if err != nil {
			return nil, usageErr(err)
		}
		return parseAlerts(b, "", false)
	}
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	ctx, cancel := boundCtx(cmd.Context(), flags)
	defer cancel()
	data, err := c.Get(ctx, "https://api.weather.gov/alerts/active", map[string]string{
		"event": tropicalAlertEventList(false),
	})
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	return parseAlerts(data, "", false)
}
