// Copyright 2026 mark-van-de-ven. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/mvanhorn/printing-press-library/library/productivity/freshservice/internal/client"
	"github.com/spf13/cobra"
)

// PATCH(freshservice-assets-display-id-resolver): generator emits a path-only
// GET that hard-fails on the human-readable identifiers (name, asset_tag) users
// actually have; resolve to numeric display_id via a one-shot /assets filter.
func resolveAssetDisplayID(c *client.Client, input string) (string, error) {
	for _, field := range []string{"name", "asset_tag"} {
		// Freshservice asset filter syntax: the whole expression must be
		// surrounded by double quotes (same wire-shape quirk as
		// /tickets/filter), with each value's string in single quotes.
		// /assets?filter=... also rejects per_page (filter endpoint has its
		// own pagination defaults; per_page is not accepted there).
		filter := wrapFreshserviceFilterQuery(fmt.Sprintf("%s:'%s'", field, input))
		data, err := c.Get("/assets", map[string]string{"filter": filter})
		if err != nil {
			return "", fmt.Errorf("resolving asset %q by %s: %w", input, field, err)
		}
		var envelope struct {
			Assets []struct {
				DisplayID json.Number `json:"display_id"`
				Name      string      `json:"name"`
				AssetTag  string      `json:"asset_tag"`
			} `json:"assets"`
		}
		if err := json.Unmarshal(data, &envelope); err != nil {
			return "", fmt.Errorf("parsing /assets response: %w", err)
		}
		switch len(envelope.Assets) {
		case 0:
			continue
		case 1:
			return envelope.Assets[0].DisplayID.String(), nil
		default:
			var rows []string
			for _, a := range envelope.Assets {
				rows = append(rows, fmt.Sprintf("  display_id=%s name=%s asset_tag=%s", a.DisplayID.String(), a.Name, a.AssetTag))
			}
			return "", fmt.Errorf("ambiguous asset %q: %d matches by %s — pass the numeric display_id instead:\n%s",
				input, len(envelope.Assets), field, joinLines(rows))
		}
	}
	return "", fmt.Errorf("no asset found with name or asset_tag %q — use the numeric display_id or 'assets list --search ...' to discover", input)
}

func joinLines(rows []string) string {
	out := ""
	for _, r := range rows {
		out += r + "\n"
	}
	return out
}

func newAssetsGetCmd(flags *rootFlags) *cobra.Command {

	cmd := &cobra.Command{
		Use:   "get <display-id|name|asset-tag>",
		Short: "Get asset by display ID, asset name, or asset tag",
		Long: `Look up an asset by its numeric display_id (the path the API natively
accepts), by its human-readable name (e.g. LAPTOP-001), or by its asset_tag.
Non-numeric inputs trigger a one-shot /assets filter to resolve the
display_id before the canonical GET fires.`,
		Example: `  # Numeric display_id (no resolution; sent straight to the API)
  freshservice-pp-cli assets get 42

  # Asset name — auto-resolves via /assets?filter="name:'LAPTOP-001'"
  freshservice-pp-cli assets get LAPTOP-001

  # Asset tag — same resolver, tries asset_tag if name returned no hits
  freshservice-pp-cli assets get ASSET-42`,
		Annotations: map[string]string{"pp:endpoint": "assets.get", "pp:method": "GET", "pp:path": "/assets/{display_id}", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}

			displayID := args[0]
			// Auto-resolve human-readable identifiers (name, asset_tag) to the
			// numeric display_id the GET endpoint accepts. Skip the lookup when
			// the input parses as an integer or when the user passed --dry-run
			// so the dry-run still echoes what would be sent without firing a
			// real probe call.
			if _, intErr := strconv.Atoi(displayID); intErr != nil && !flags.dryRun {
				resolved, rerr := resolveAssetDisplayID(c, displayID)
				if rerr != nil {
					return classifyAPIError(rerr, flags)
				}
				displayID = resolved
			}

			path := "/assets/{display_id}"
			path = replacePathParam(path, "display_id", displayID)
			params := map[string]string{}
			data, prov, err := resolveRead(cmd.Context(), c, flags, "assets", false, path, params, nil)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			// Print provenance to stderr for human-facing output
			{
				var countItems []json.RawMessage
				_ = json.Unmarshal(data, &countItems)
				printProvenance(cmd, len(countItems), prov)
			}
			// For JSON output, wrap with provenance envelope before passing through flags.
			// --select wins over --compact when both are set; --compact only runs when
			// no explicit fields were requested. Explicit format flags (--csv, --quiet,
			// --plain) opt out of the auto-JSON path so piped consumers that asked for
			// a non-JSON format reach the standard pipeline below.
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				filtered := data
				if flags.selectFields != "" {
					filtered = filterFields(filtered, flags.selectFields)
				} else if flags.compact {
					filtered = compactFields(filtered)
				}
				wrapped, wrapErr := wrapWithProvenance(filtered, prov)
				if wrapErr != nil {
					return wrapErr
				}
				return printOutput(cmd.OutOrStdout(), wrapped, true)
			}
			// For all other output modes (table, csv, plain, quiet), use the standard pipeline
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				var items []map[string]any
				if json.Unmarshal(data, &items) == nil && len(items) > 0 {
					if err := printAutoTable(cmd.OutOrStdout(), items); err != nil {
						return err
					}
					if len(items) >= 25 {
						fmt.Fprintf(os.Stderr, "\nShowing %d results. To narrow: add --limit, --json --select, or filter flags.\n", len(items))
					}
					return nil
				}
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}

	return cmd
}
