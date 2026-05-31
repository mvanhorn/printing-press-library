// Hand-authored — NOT generated. Implements the watchlist trio:
// `ship pin <imo>` / `ship unpin <imo>` / `ship refresh`. Pinning marks a
// vessel for an active deal or story; refresh re-fetches pinned (or stale)
// vessels so the watchlist stays current.
package cli

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/gisis/internal/store"

	"github.com/spf13/cobra"
)

// imoNumberPattern matches a 7-digit IMO number (IMO ship identifiers are
// always 7 digits, e.g. 9866641). Used to reject user typos and runner
// sentinels (e.g. "__printing_press_invalid__") at the CLI boundary before
// they reach the local cache or the GISIS HTML scraper.
var imoNumberPattern = regexp.MustCompile(`^[0-9]{7}$`)

// isValidIMOFormat reports whether s is a syntactically valid IMO number.
// Pure + cheap; does NOT verify the IMO checksum (final digit) — that's a
// stricter rule than GISIS itself enforces in URL lookups.
func isValidIMOFormat(s string) bool {
	return imoNumberPattern.MatchString(strings.TrimSpace(s))
}

func newShipPinCmd(flags *rootFlags) *cobra.Command {
	var flagLabel string
	var flagList bool

	cmd := &cobra.Command{
		Use:     "pin <imo>",
		Short:   "Pin a vessel to your watchlist, optionally with a label.",
		Long:    "Adds an IMO to the local watchlist so 'ship refresh --pinned' can re-fetch it on demand. Re-pinning updates the label. Use 'ship pin --list' to see the watchlist.",
		Example: "  gisis-pp-cli ship pin 9866641 --label \"Lagos deal\"\n  gisis-pp-cli ship pin --list --json",
		// PATCH(pr-953 greptile): no mcp:read-only — pin writes the local watchlist.
		RunE: func(cmd *cobra.Command, args []string) error {
			if flagList {
				if dryRunOK(flags) {
					return nil
				}
				db, err := openStoreForRead(cmd.Context(), "gisis-pp-cli")
				if err != nil {
					return fmt.Errorf("opening local store: %w", err)
				}
				if db == nil {
					return printJSONFiltered(cmd.OutOrStdout(), []store.PinRow{}, flags)
				}
				defer db.Close()
				pins, err := db.ListPins()
				if err != nil {
					return fmt.Errorf("reading watchlist: %w", err)
				}
				return printJSONFiltered(cmd.OutOrStdout(), pins, flags)
			}

			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			imo := strings.TrimSpace(args[0])
			if imo == "" {
				return usageErr(fmt.Errorf("IMO number is required"))
			}
			if !isValidIMOFormat(imo) {
				return usageErr(fmt.Errorf("invalid IMO number %q: expected 7 digits (e.g. 9866641)", imo))
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("gisis-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()
			if err := db.PinShip(imo, flagLabel); err != nil {
				return fmt.Errorf("pinning %s: %w", imo, err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"status":     "pinned",
				"imo_number": imo,
				"label":      flagLabel,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&flagLabel, "label", "", "Optional label for this pin (e.g. a deal or story name)")
	cmd.Flags().BoolVar(&flagList, "list", false, "List the watchlist instead of pinning")
	return cmd
}

func newShipUnpinCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "unpin <imo>",
		Short:   "Remove a vessel from your watchlist.",
		Example: "  gisis-pp-cli ship unpin 9866641",
		// PATCH(pr-953 greptile): no mcp:read-only — unpin deletes from the local watchlist.
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			imo := strings.TrimSpace(args[0])
			if imo == "" {
				return usageErr(fmt.Errorf("IMO number is required"))
			}
			if !isValidIMOFormat(imo) {
				return usageErr(fmt.Errorf("invalid IMO number %q: expected 7 digits (e.g. 9866641)", imo))
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("gisis-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()
			removed, err := db.UnpinShip(imo)
			if err != nil {
				return fmt.Errorf("unpinning %s: %w", imo, err)
			}
			status := "unpinned"
			if !removed {
				status = "not_pinned"
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"status":     status,
				"imo_number": imo,
			}, flags)
		},
	}
	return cmd
}

// gatherRefreshTargets collects IMOs to refresh from the local watchlist and/or
// the stale set. It opens the store read-only and closes it before the caller
// re-opens it read-write for the fetch loop.
func gatherRefreshTargets(ctx context.Context, pinned bool, olderThan string) ([]string, error) {
	if !pinned && olderThan == "" {
		return nil, nil
	}
	db, err := openStoreForRead(ctx, "gisis-pp-cli")
	if err != nil {
		return nil, fmt.Errorf("opening local store: %w", err)
	}
	if db == nil {
		return nil, nil
	}
	defer db.Close()

	var out []string
	if pinned {
		p, err := db.PinnedIMOs()
		if err != nil {
			return nil, fmt.Errorf("reading watchlist: %w", err)
		}
		out = append(out, p...)
	}
	if olderThan != "" {
		dur, err := parseOlderThan(olderThan)
		if err != nil {
			return nil, usageErr(err)
		}
		stale, err := db.StaleShips(time.Now().UTC().Add(-dur), false)
		if err != nil {
			return nil, fmt.Errorf("querying stale vessels: %w", err)
		}
		for _, r := range stale {
			out = append(out, r.IMONumber)
		}
	}
	return out, nil
}

func newShipRefreshCmd(flags *rootFlags) *cobra.Command {
	var flagPinned bool
	var flagOlderThan string
	var flagThrottle time.Duration

	cmd := &cobra.Command{
		Use:     "refresh [imo...]",
		Short:   "Re-fetch vessels from GISIS and update the local cache.",
		Long:    "Re-fetches the given IMOs (positional args), all pinned vessels (--pinned), and/or cached vessels older than a threshold (--older-than), then updates the local cache. Forces a fresh fetch (bypasses the HTTP response cache) and honors the request throttle.",
		Example: "  gisis-pp-cli ship refresh 9866641\n  gisis-pp-cli ship refresh --pinned\n  gisis-pp-cli ship refresh --older-than 30d",
		// PATCH(pr-953 greptile): no mcp:read-only — refresh fetches from GISIS and writes the cache.
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			imos := append([]string{}, args...)
			extra, err := gatherRefreshTargets(cmd.Context(), flagPinned, flagOlderThan)
			if err != nil {
				return err
			}
			imos = dedupeIMOs(append(imos, extra...))
			if len(imos) == 0 {
				return usageErr(fmt.Errorf("nothing to refresh: pass IMO args, --pinned, or --older-than"))
			}
			// PATCH(pr-953 greptile): same 7-digit guard as batch — reject malformed
			// positional IMOs up front (pinned/stale targets are already valid).
			var badIMOs []string
			for _, imo := range imos {
				if !isValidIMOFormat(imo) {
					badIMOs = append(badIMOs, imo)
				}
			}
			if len(badIMOs) > 0 {
				return usageErr(fmt.Errorf("invalid IMO number(s): %s — each must be 7 digits", strings.Join(badIMOs, ", ")))
			}
			results, err := resolveIMOs(cmd, flags, imos, flagThrottle, true)
			return emitBatchResults(cmd, flags, results, err)
		},
	}
	cmd.Flags().BoolVar(&flagPinned, "pinned", false, "Refresh all pinned (watchlisted) vessels")
	cmd.Flags().StringVar(&flagOlderThan, "older-than", "", "Also refresh cached vessels older than this (e.g. 30d)")
	cmd.Flags().DurationVar(&flagThrottle, "throttle", 2*time.Second, "Delay between requests for GISIS politeness")
	return cmd
}
