// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: novel-commands — see .printing-press-patches.json for the change-set rationale.
//
// `cancel` is a top-level transcendence command that cancels a reservation
// on either OpenTable or Tock. v0.2 supports OT cancel via GraphQL
// CancelReservation; Tock cancel is best-effort form-submit (untested in
// v0.2 due to the test-budget constraint of one fresh booking per platform).
//
// Per R7, cancel is NOT gated by TRG_ALLOW_BOOK (recovery action). The
// verify-mode floor (R12 / cliutil.IsVerifyEnv) is the only safety check —
// load-bearing because cancel is irreversible AND ungated otherwise.
//
// Compound argument shape: OT requires {confirmationNumber, securityToken,
// restaurantId} since the cancel mutation can't be addressed by confirmation
// alone. Tock requires {purchaseId, venueSlug}.
//
//	cancel opentable:<rid>:<confirmationNumber>:<securityToken>
//	cancel tock:<venueSlug>:<purchaseId>

// pp:client-call — `cancel` reaches the OpenTable and Tock clients through
// `internal/source/opentable` and `internal/source/tock`. Multi-segment
// internal paths require this carve-out per AGENTS.md.

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/auth"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/opentable"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/tock"
)

// cancelResult is the agent-friendly JSON shape emitted to stdout.
type cancelResult struct {
	Network            string `json:"network"`
	ReservationID      string `json:"reservation_id,omitempty"`
	ConfirmationNumber string `json:"confirmation_number,omitempty"`
	RestaurantSlug     string `json:"restaurant_slug,omitempty"`
	CanceledAt         string `json:"canceled_at,omitempty"`
	Source             string `json:"source"` // "cancel" | "dry_run"
	Hint               string `json:"hint,omitempty"`
	Error              string `json:"error,omitempty"`
}

// newCancelCmd constructs the `cancel` Cobra command.
func newCancelCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cancel <network>:<...id-fields>",
		Short: "Cancel a reservation on OpenTable or Tock",
		Long: `Cancels an existing reservation. Compound argument shape:

  opentable:<restaurantId>:<confirmationNumber>:<securityToken>
  tock:<venueSlug>:<purchaseId>

The compound parts are returned by the corresponding ` + "`book`" + ` command's JSON output:
  - OpenTable: restaurant_id (resolved), confirmation_number, security_token
  - Tock:      restaurant_slug, reservation_id (which is the purchaseId)

Cancel is NOT gated by TRG_ALLOW_BOOK (it's a recovery action). PRINTING_PRESS_VERIFY=1 short-circuits to dry-run regardless — verifier safety floor.`,
		Example: "  table-reservation-goat-pp-cli cancel opentable:1255093:114309:01Ozsdas9H1...",
		Args:    cobra.ExactArgs(1),
		Annotations: map[string]string{
			"pp:typed-exit-codes": "0,2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Step 1: Verify-mode floor (R12). The ONLY safety check on cancel.
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), cancelResult{
					Network: "<verify-mode>", Source: "dry_run",
					Hint: "PRINTING_PRESS_VERIFY=1 is set; cancel short-circuits without firing",
				}, flags)
			}

			network, parts, err := parseCancelArg(args[0])
			if err != nil {
				return printJSONFiltered(cmd.OutOrStdout(), cancelResult{
					Network: "<unparsed>", Error: "malformed_argument", Hint: err.Error(),
				}, flags)
			}

			session, err := auth.Load()
			if err != nil {
				return fmt.Errorf("loading session: %w", err)
			}

			result, _ := cancelOnNetwork(ctx, session, network, parts)
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	return cmd
}

// parseCancelArg splits `<network>:<rest>` and returns the network plus the
// remaining colon-separated parts.
func parseCancelArg(s string) (network string, parts []string, err error) {
	idx := strings.Index(s, ":")
	if idx < 0 {
		return "", nil, fmt.Errorf("expected '<network>:<id-fields>'; got %q", s)
	}
	network = strings.ToLower(s[:idx])
	rest := s[idx+1:]
	if rest == "" {
		return "", nil, fmt.Errorf("missing id fields after %q", network)
	}
	parts = strings.Split(rest, ":")
	if network != "opentable" && network != "tock" {
		return "", nil, fmt.Errorf("unknown network %q", network)
	}
	return network, parts, nil
}

// cancelOnNetwork dispatches to the network-specific cancel flow.
func cancelOnNetwork(ctx context.Context, session *auth.Session, network string, parts []string) (cancelResult, error) {
	out := cancelResult{Network: network}
	switch network {
	case "opentable":
		return cancelOnOpenTable(ctx, session, parts, out)
	case "tock":
		return cancelOnTock(ctx, session, parts, out)
	}
	out.Error = "unknown_network"
	return out, fmt.Errorf("unknown network %q", network)
}

// cancelOnOpenTable expects parts = [restaurantId, confirmationNumber, securityToken].
func cancelOnOpenTable(ctx context.Context, session *auth.Session, parts []string, out cancelResult) (cancelResult, error) {
	if len(parts) < 3 {
		out.Error = "malformed_argument"
		out.Hint = "OT cancel requires opentable:<restaurantId>:<confirmationNumber>:<securityToken>"
		return out, fmt.Errorf("missing OT cancel triple")
	}
	rid, err1 := strconv.Atoi(parts[0])
	cn, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		out.Error = "malformed_argument"
		out.Hint = "restaurantId and confirmationNumber must be integers"
		return out, fmt.Errorf("integer parse: rid=%v cn=%v", err1, err2)
	}
	st := parts[2]
	if st == "" {
		out.Error = "malformed_argument"
		out.Hint = "securityToken is empty"
		return out, fmt.Errorf("empty securityToken")
	}
	c, err := opentable.New(session)
	if err != nil {
		out.Error = "client_init_failed"
		out.Hint = err.Error()
		return out, err
	}
	resp, err := c.Cancel(ctx, opentable.CancelRequest{
		RestaurantID: rid, ConfirmationNumber: cn, SecurityToken: st,
	})
	if err != nil {
		switch {
		case errors.Is(err, opentable.ErrAuthExpired):
			out.Error = "auth_expired"
		case errors.Is(err, opentable.ErrPastCancellationWindow):
			out.Error = "past_cancellation_window"
			out.Hint = "this reservation can no longer be canceled via the API"
		case errors.Is(err, opentable.ErrCanaryUnrecognizedBody):
			out.Error = "discriminator_drift"
		default:
			out.Error = "network_error"
		}
		return out, err
	}
	out.Source = "cancel"
	out.ReservationID = fmt.Sprintf("%d", resp.ReservationID)
	out.ConfirmationNumber = fmt.Sprintf("%d", resp.ConfirmationNumber)
	out.CanceledAt = time.Now().UTC().Format(time.RFC3339)
	return out, nil
}

// cancelOnTock expects parts = [venueSlug, purchaseId].
func cancelOnTock(ctx context.Context, session *auth.Session, parts []string, out cancelResult) (cancelResult, error) {
	if len(parts) < 2 {
		out.Error = "malformed_argument"
		out.Hint = "Tock cancel requires tock:<venueSlug>:<purchaseId>"
		return out, fmt.Errorf("missing Tock cancel pair")
	}
	slug := parts[0]
	pid, err := strconv.Atoi(parts[1])
	if err != nil || pid == 0 {
		out.Error = "malformed_argument"
		out.Hint = "purchaseId must be a non-zero integer"
		return out, fmt.Errorf("purchaseId parse: %w", err)
	}
	c, err := tock.New(session)
	if err != nil {
		out.Error = "client_init_failed"
		out.Hint = err.Error()
		return out, err
	}
	out.RestaurantSlug = slug
	resp, err := c.Cancel(ctx, tock.CancelRequest{VenueSlug: slug, PurchaseID: pid})
	if err != nil {
		switch {
		case errors.Is(err, tock.ErrPastCancellationWindow):
			out.Error = "past_cancellation_window"
		case errors.Is(err, tock.ErrCanaryUnrecognizedBody):
			out.Error = "discriminator_drift"
		default:
			out.Error = "network_error"
		}
		out.Hint = err.Error()
		return out, err
	}
	out.Source = "cancel"
	out.ReservationID = fmt.Sprintf("%d", resp.PurchaseID)
	out.CanceledAt = time.Now().UTC().Format(time.RFC3339)
	return out, nil
}
