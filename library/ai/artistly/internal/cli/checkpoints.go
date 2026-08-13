// Copyright 2026 Vincent Colombo and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored. `checkpoints list` surfaces Artistly's checkpoint (model)
// catalog so callers can resolve a model name to the integer id that
// 'generate --checkpoint-id' expects. The catalog is exposed via a POST to
// /api/checkpoints (no body), which Laravel guards with the same CSRF + AJAX
// headers as the other state-touching endpoints.
//
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/ai/artistly/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/artistly/internal/cliutil"

	"github.com/spf13/cobra"
)

type checkpointEntry struct {
	ID           int    `json:"id"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	TriggerWords string `json:"trigger_words"`
}

// fetchCheckpoints POSTs to /api/checkpoints and returns the model catalog.
// The endpoint takes no body but, like every state-touching Laravel route,
// requires the CSRF token + XMLHttpRequest marker that writeHeaders builds.
func fetchCheckpoints(ctx context.Context, c *client.Client) ([]checkpointEntry, error) {
	headers, err := writeHeaders(ctx, c)
	if err != nil {
		return nil, err
	}
	raw, _, err := c.PostWithHeaders(ctx, "/api/checkpoints", nil, headers)
	if err != nil {
		return nil, err
	}
	// An expired session is served the login HTML page (HTTP 200), not JSON.
	// Detect it and return a clean auth error instead of a cryptic JSON parse
	// failure — mirrors parseDesigns.
	if trimmed := strings.TrimSpace(string(raw)); strings.HasPrefix(trimmed, "<") {
		return nil, authErr(fmt.Errorf("not authenticated or session expired; run: artistly-pp-cli auth login --chrome"))
	}
	var cps []checkpointEntry
	if err := json.Unmarshal(raw, &cps); err != nil {
		return nil, fmt.Errorf("could not parse checkpoints response: %w", err)
	}
	return cps, nil
}

func newCheckpointsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "checkpoints",
		Short:       "Browse Artistly's checkpoint (model) catalog",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCheckpointsListCmd(flags))
	return cmd
}

func newCheckpointsListCmd(flags *rootFlags) *cobra.Command {
	var match string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List available checkpoints/models (optionally filtered with --match)",
		Long: trimLong(`
List the checkpoint (model) catalog Artistly uses for generation. Use --match to
fuzzy-filter by name or slug so you can resolve a human term (e.g. "comic") to the
integer id that 'generate --checkpoint-id' expects.`),
		Example:     "  artistly-pp-cli checkpoints list --match comic --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// fetchCheckpoints -> writeHeaders -> primeCSRF does a live GET to
			// mint a CSRF cookie, which the verify-mode short-circuit (gated on
			// mutating verbs only) does not catch. Guard like every other
			// CSRF-requiring command so verify runs don't dial the live app.
			if dryRunOK(flags) || cliutil.IsVerifyEnv() {
				return nil
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			cps, err := fetchCheckpoints(ctx, c)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if match != "" {
				filtered := cps[:0:0]
				for _, cp := range cps {
					if strings.Contains(strings.ToLower(cp.Name), strings.ToLower(match)) ||
						strings.Contains(strings.ToLower(cp.Slug), strings.ToLower(match)) {
						filtered = append(filtered, cp)
					}
				}
				cps = filtered
			}
			sort.Slice(cps, func(i, j int) bool { return cps[i].ID < cps[j].ID })
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), cps, flags)
			}
			if len(cps) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No checkpoints found.")
				return nil
			}
			for _, cp := range cps {
				fmt.Fprintf(cmd.OutOrStdout(), "%-4d %-22s %s\n", cp.ID, cp.Name, cp.Slug)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&match, "match", "", "Filter checkpoints whose name or slug contains this text")
	return cmd
}
