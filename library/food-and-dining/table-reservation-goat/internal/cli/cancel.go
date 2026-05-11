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
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/resy"
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
		Short: "Cancel a reservation on OpenTable, Tock, or Resy",
		Long: `Cancels an existing reservation. Compound argument shape:

  opentable:<restaurantId>:<confirmationNumber>:<securityToken>
  tock:<venueSlug>:<purchaseId>
  resy:<resyToken>

The compound parts are returned by the corresponding ` + "`book`" + ` command's JSON output:
  - OpenTable: restaurant_id (resolved), confirmation_number, security_token
  - Tock:      restaurant_slug, reservation_id (which is the purchaseId)
  - Resy:      reservation_id (resy_token) — a single opaque field, no compound parts

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
	if network != "opentable" && network != "tock" && network != "resy" {
		return "", nil, fmt.Errorf("unknown network %q", network)
	}
	// Resy reservation tokens are opaque single-field strings; splitting on
	// ":" would truncate any token whose value contains a colon (e.g.
	// "rgs://venue/20:30:00/..."). Keep the whole rest as one part. OT
	// (triple) and Tock (pair) still split because their compound shapes
	// are well-defined and colon-free.
	if network == "resy" {
		return network, []string{rest}, nil
	}
	parts = strings.Split(rest, ":")
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
	case "resy":
		return cancelOnResy(ctx, session, parts, out)
	}
	out.Error = "unknown_network"
	return out, fmt.Errorf("unknown network %q", network)
}

// cancelOnResy expects parts = [resyToken] (a single opaque string). Resy's
// cancel endpoint requires only the resy_token returned by `book` — no
// venue or confirmation number triple like OT.
func cancelOnResy(ctx context.Context, session *auth.Session, parts []string, out cancelResult) (cancelResult, error) {
	if session == nil || session.Resy == nil || session.Resy.AuthToken == "" {
		out.Error = "auth_required"
		out.Hint = "run `auth login --resy --email <you@example.com>` first"
		return out, fmt.Errorf("resy not authenticated")
	}
	if len(parts) < 1 || parts[0] == "" {
		out.Error = "malformed_argument"
		out.Hint = "Resy cancel requires resy:<resyToken>"
		return out, fmt.Errorf("missing resy token")
	}
	resyToken := parts[0]
	out.ReservationID = resyToken
	client := resy.New(resy.Credentials{
		APIKey:    session.Resy.APIKey,
		AuthToken: session.Resy.AuthToken,
		Email:     session.Resy.Email,
	})
	resp, err := client.Cancel(ctx, resyToken)
	if err != nil {
		switch {
		case errors.Is(err, resy.ErrAuthExpired):
			// /3/cancel returns 401/419 in two distinct scenarios:
			// the JWT is rejected (genuine auth_expired) OR the
			// resy_token in the request body is unknown to Resy
			// (bad reservation id). The wire layer can't tell them
			// apart from status alone, so we probe Whoami here —
			// only fires on the cancel-error path, no happy-path
			// latency cost. If Whoami succeeds, the JWT is fine and
			// the failure must be a bad/expired reservation token.
			if _, werr := client.Whoami(ctx); werr == nil {
				out.Error = "invalid_reservation"
				out.Hint = "resy_token not recognized — verify the token from a recent `book` envelope (cancel of an already-cancelled reservation also returns this)"
				return out, fmt.Errorf("resy: reservation token rejected by /3/cancel (auth probe ok)")
			}
			out.Error = "auth_expired"
		case errors.Is(err, resy.ErrAuthMissing):
			out.Error = "auth_required"
		default:
			out.Error = "network_error"
		}
		out.Hint = err.Error()
		return out, err
	}
	out.Source = "cancel"
	if resp.CancelToken != "" {
		out.ConfirmationNumber = resp.CancelToken
	}
	out.CanceledAt = time.Now().UTC().Format(time.RFC3339)
	return out, nil
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
