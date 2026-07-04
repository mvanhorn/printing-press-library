// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// venueFlags holds the per-command venue selection inputs. `addVenueFlags`
// wires it into a cobra command; `resolveVenue` reads the values and
// returns the canonical venue string ("all", "polymarket", or "kalshi"),
// reporting conflicts as usage errors.
//
// The three flags compose like this: --venue is the canonical string
// flag (matches the existing convention from liquid/trending/movers/
// resolving/new). --polymarket and --kalshi are boolean shortcuts so an
// agent can scope a query to one venue without paying the cost of
// searching both. Default behavior (no flag) stays "all" so existing
// scripts that depend on cross-venue output keep working.
type venueFlags struct {
	venue      string
	polymarket bool
	kalshi     bool
}

// addVenueFlags registers --venue, --polymarket, --kalshi on cmd.
func addVenueFlags(cmd *cobra.Command, vf *venueFlags) {
	cmd.Flags().StringVar(&vf.venue, "venue", "all", "Venue: all, polymarket, kalshi")
	cmd.Flags().BoolVar(&vf.polymarket, "polymarket", false, "Shortcut for --venue=polymarket")
	cmd.Flags().BoolVar(&vf.kalshi, "kalshi", false, "Shortcut for --venue=kalshi")
}

// resolveVenue returns the canonical venue selection or a usage error
// naming the specific conflict. Default is "all".
func resolveVenue(vf venueFlags) (string, error) {
	if vf.polymarket && vf.kalshi {
		return "", fmt.Errorf("--polymarket and --kalshi are mutually exclusive (pick one, or drop both for cross-venue)")
	}

	// Treat the default value as unset so an explicit --venue=all stays
	// compatible with a shortcut flag, and an explicit --venue=kalshi
	// alongside --polymarket is the conflict the user actually cares
	// about.
	venueExplicit := vf.venue != "" && vf.venue != "all"

	if vf.polymarket {
		if venueExplicit && vf.venue != "polymarket" {
			return "", fmt.Errorf("--polymarket conflicts with --venue=%s", vf.venue)
		}
		return "polymarket", nil
	}
	if vf.kalshi {
		if venueExplicit && vf.venue != "kalshi" {
			return "", fmt.Errorf("--kalshi conflicts with --venue=%s", vf.venue)
		}
		return "kalshi", nil
	}

	switch vf.venue {
	case "", "all":
		return "all", nil
	case "polymarket", "kalshi":
		return vf.venue, nil
	default:
		return "", fmt.Errorf("invalid --venue %q: must be all, polymarket, or kalshi", vf.venue)
	}
}
