// Copyright 2026 omarshahine. Licensed under Apache-2.0. See LICENSE.
// Hand-authored authenticated read surface (not generator output).
// PATCH: account-scoped reads (me / bookings / wallet) over guest-api +
// graphql.blacklane.com using the cracked auth recipe. All read-only.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// PATCH: emptyOnNoSession lets account reads degrade gracefully. When the
// Blacklane Auth0 session is missing or expired, agent/json callers get an
// empty result and exit 0 (the printed-CLI missing-mirror convention) while
// humans still see the re-auth hint on stderr. Returns handled=true when it
// consumed the error. This keeps `me`/`bookings`/`wallet` agent-friendly and
// non-fatal in unauthenticated environments (e.g. live-dogfood sandboxes).
func emptyOnNoSession(cmd *cobra.Command, flags *rootFlags, err error, empty string) (bool, error) {
	if err == nil {
		return false, nil
	}
	msg := err.Error()
	if !strings.Contains(msg, "session expired") &&
		!strings.Contains(msg, "not authenticated") &&
		!strings.Contains(msg, "not logged in") {
		return false, nil
	}
	fmt.Fprintln(cmd.ErrOrStderr(), msg)
	if flags.asJSON || flags.agent {
		fmt.Fprintln(cmd.OutOrStdout(), empty)
	}
	return true, nil
}

// Compact GraphQL documents (full field sets live in the web bundle; these keep
// the high-gravity fields agents and users actually need).
const gqlGetNewBookings = `query getNewBookings($filter: BookingFilters!, $limit: Int = 25, $offset: Int = 0) {
  bookings(filter: $filter, limit: $limit, offset: $offset) {
    items {
      id number status
      price { currency totalAmount }
      services {
        ... on Ride {
          category date status carRideStatus
          pickup { address airportIata }
          dropoff { address airportIata }
          chauffeur { firstName }
        }
      }
    }
  }
}`

const gqlWallet = `query Wallet($input: WalletInput!) {
  wallet(input: $input) {
    credits {
      code type bookingValidFrom bookingValidTo
      credit { unit amount balance }
      campaign { name description }
    }
  }
}`

func newNovelMeCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "me",
		Short:       "Show your Blacklane account profile (requires auth login)",
		Example:     "  blacklane-pp-cli me --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			data, err := authedGuestGet("/api/v1/users/me", flags.timeout)
			if err != nil {
				if handled, e := emptyOnNoSession(cmd, flags, err, "{}"); handled {
					return e
				}
				return err
			}
			return emitDomainList(cmd, flags, json.RawMessage(data))
		},
	}
}

func newNovelBookingsCmd(flags *rootFlags) *cobra.Command {
	var when string
	var filterJSON string
	var limit, offset int
	cmd := &cobra.Command{
		Use:   "bookings",
		Short: "List your Blacklane bookings (requires auth login)",
		Long:  "List your rides. --when upcoming|past selects the time frame; pass --filter-json to override the raw BookingFilters object if needed.",
		Example: strings.TrimSpace(`
  blacklane-pp-cli bookings --when upcoming
  blacklane-pp-cli bookings --when past --limit 50 --agent`),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			var filter map[string]any
			if filterJSON != "" {
				if err := json.Unmarshal([]byte(filterJSON), &filter); err != nil {
					return fmt.Errorf("invalid --filter-json: %w", err)
				}
			} else {
				filter = map[string]any{"timeState": when}
			}
			data, err := authedGraphQL("getNewBookings", gqlGetNewBookings, map[string]any{
				"filter": filter, "limit": limit, "offset": offset,
			}, flags.timeout)
			if err != nil {
				if handled, e := emptyOnNoSession(cmd, flags, err, "[]"); handled {
					return e
				}
				return err
			}
			// Unwrap { bookings: { items: [...] } } to the items array.
			var wrap struct {
				Bookings struct {
					Items json.RawMessage `json:"items"`
				} `json:"bookings"`
			}
			out := data
			if json.Unmarshal(data, &wrap) == nil && len(wrap.Bookings.Items) > 0 {
				out = wrap.Bookings.Items
			}
			return emitDomainList(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&when, "when", "upcoming", "Time frame: upcoming or past")
	cmd.Flags().StringVar(&filterJSON, "filter-json", "", "Raw BookingFilters JSON (overrides --when)")
	cmd.Flags().IntVar(&limit, "limit", 25, "Max bookings to return")
	cmd.Flags().IntVar(&offset, "offset", 0, "Pagination offset")
	return cmd
}

func newNovelWalletCmd(flags *rootFlags) *cobra.Command {
	var inputJSON string
	cmd := &cobra.Command{
		Use:         "wallet",
		Short:       "Show your Blacklane wallet credits and vouchers (requires auth login)",
		Example:     "  blacklane-pp-cli wallet --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			input := map[string]any{}
			if inputJSON != "" {
				if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
					return fmt.Errorf("invalid --input-json: %w", err)
				}
			}
			data, err := authedGraphQL("Wallet", gqlWallet, map[string]any{"input": input}, flags.timeout)
			if err != nil {
				if handled, e := emptyOnNoSession(cmd, flags, err, "[]"); handled {
					return e
				}
				return err
			}
			var wrap struct {
				Wallet struct {
					Credits json.RawMessage `json:"credits"`
				} `json:"wallet"`
			}
			out := data
			if json.Unmarshal(data, &wrap) == nil && len(wrap.Wallet.Credits) > 0 {
				out = wrap.Wallet.Credits
			}
			return emitDomainList(cmd, flags, out)
		},
	}
	cmd.Flags().StringVar(&inputJSON, "input-json", "", "Raw WalletInput JSON (default: {})")
	return cmd
}
