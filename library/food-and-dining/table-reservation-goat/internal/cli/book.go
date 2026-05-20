// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

package cli

// PATCH: novel-commands — see .printing-press-patches.json for the change-set rationale.
//
// `book` is a top-level transcendence command that places a reservation on
// either OpenTable or Tock. v0.2 supports OT free reservations end-to-end;
// Tock surfaces a typed "use the website" error pointing at the venue URL
// (Tock's form-submit body shape was not captured in U1; future v0.2.1).
//
// Eight-step guard order in RunE:
//  1. Verify-mode floor (cliutil.IsVerifyEnv)
//  2. Required-arg validation (--date, --time, --party)
//  3. <network>:<slug> parse
//  4. Filesystem advisory lock (cross-process safety)
//  5. Idempotency pre-flight (ListUpcomingReservations + normalized match)
//  6. Dry-run / commit gate (--dry-run OR TRG_ALLOW_BOOK unset)
//  7. Commit (network's Book, with sanitized error mapping)
//  8. Result emission (agent-friendly JSON, raw chains stay on stderr)

// pp:client-call — `book` reaches the OpenTable and Tock clients through
// `internal/source/opentable` and `internal/source/tock`. Multi-segment
// internal paths require this carve-out per AGENTS.md.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/spf13/cobra"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/auth"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/opentable"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/resy"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/table-reservation-goat/internal/source/tock"
)

// bookResult is the agent-friendly JSON shape emitted to stdout.
// Field names are stable; nullable when not applicable.
type bookResult struct {
	Network              string `json:"network"`
	ReservationID        string `json:"reservation_id,omitempty"`
	ConfirmationNumber   string `json:"confirmation_number,omitempty"`
	RestaurantSlug       string `json:"restaurant_slug,omitempty"`
	RestaurantName       string `json:"restaurant_name,omitempty"`
	Date                 string `json:"date"`
	Time                 string `json:"time"`
	Party                int    `json:"party"`
	CancellationDeadline string `json:"cancellation_deadline,omitempty"`
	MatchedExisting      bool   `json:"matched_existing"`
	Source               string `json:"source"` // "book" | "matched_existing" | "dry_run"
	Hint                 string `json:"hint,omitempty"`
	BookURL              string `json:"book_url,omitempty"` // for Tock fallback to website
	Error                string `json:"error,omitempty"`    // typed category when Error is non-empty
}

// newBookCmd constructs the `book` Cobra command.
func newBookCmd(flags *rootFlags) *cobra.Command {
	var (
		date   string
		hhmm   string
		party  int
		dryRun bool
	)
	cmd := &cobra.Command{
		Use:     "book <network>:<slug>",
		Short:   "Place a reservation on OpenTable, Tock, or Resy",
		Long:    "Places a reservation for the given venue at the requested date/time/party. Free reservations only in v0.2; payment-required venues return a typed payment_required error pointing at v0.3.\n\nSafety: live commit fires only when TRG_ALLOW_BOOK=1 is set in the environment AND PRINTING_PRESS_VERIFY is unset (verify-mode floor). Without the env var, returns a dry-run envelope with a hint.",
		Example: "  TRG_ALLOW_BOOK=1 table-reservation-goat-pp-cli book opentable:water-grill-bellevue --date 2026-05-13 --time 19:00 --party 2 --agent",
		Args:    cobra.ExactArgs(1),
		Annotations: map[string]string{
			// Write command — no mcp:read-only annotation.
			// pp:typed-exit-codes accepts 0 (success/dry-run) and 2 (validation/lock errors).
			"pp:typed-exit-codes": "0,2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Step 1: Verify-mode floor (R12) — short-circuit BEFORE any other work.
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), bookResult{
					Network: "<verify-mode>", Date: date, Time: hhmm, Party: party,
					Source: "dry_run",
					Hint:   "PRINTING_PRESS_VERIFY=1 is set; CLI short-circuits to dry-run regardless of TRG_ALLOW_BOOK",
				}, flags)
			}

			// Step 2: Required-arg validation (R14) — happens AFTER R12, INSIDE RunE.
			if err := validateBookArgs(date, hhmm, party); err != nil {
				return printJSONFiltered(cmd.OutOrStdout(), bookResult{
					Network: "<unparsed>", Date: date, Time: hhmm, Party: party,
					Error: "missing_required_args", Hint: err.Error(),
				}, flags)
			}

			// Step 3: <network>:<slug> parse.
			network, slug, err := parseNetworkPrefix(args[0])
			if err != nil {
				return printJSONFiltered(cmd.OutOrStdout(), bookResult{
					Network: "<unparsed>", Date: date, Time: hhmm, Party: party,
					Error: "malformed_argument", Hint: err.Error(),
				}, flags)
			}

			// Step 4: Filesystem advisory lock — keyed on (network, slug, date, time, party).
			lockPath, releaseLock, err := acquireBookLock(network, slug, date, hhmm, party)
			if err != nil {
				return printJSONFiltered(cmd.OutOrStdout(), bookResult{
					Network: network, RestaurantSlug: slug, Date: date, Time: hhmm, Party: party,
					Error: "concurrent_invocation", Hint: "another book is in flight; retry after",
				}, flags)
			}
			defer releaseLock()
			_ = lockPath

			session, err := auth.Load()
			if err != nil {
				return fmt.Errorf("loading session: %w", err)
			}

			// Step 5+6+7+8: dispatch to network handler.
			result, err := bookOnNetwork(ctx, session, network, slug, date, hhmm, party, dryRun)
			if err != nil {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&date, "date", "", "Reservation date (YYYY-MM-DD); required")
	cmd.Flags().StringVar(&hhmm, "time", "", "Reservation time (HH:MM 24h); required")
	cmd.Flags().IntVar(&party, "party", 0, "Party size; required, must be > 0")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Return the would-book envelope without firing the book call")
	return cmd
}

// validateBookArgs returns an error naming the missing/invalid field, OR nil.
// Per R14: validation runs INSIDE RunE after the verify-mode floor, so
// `printing-press verify` can probe with --dry-run and missing args reach the
// short-circuit instead of failing on Cobra's MarkFlagRequired.
func validateBookArgs(date, hhmm string, party int) error {
	missing := []string{}
	if date == "" {
		missing = append(missing, "--date")
	}
	if hhmm == "" {
		missing = append(missing, "--time")
	}
	if party <= 0 {
		missing = append(missing, "--party (must be > 0)")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing required: %s", strings.Join(missing, ", "))
	}
	return nil
}

// parseNetworkPrefix splits a "<network>:<slug>" arg into its parts. Returns
// a typed error when the format is wrong or the network is unknown.
func parseNetworkPrefix(s string) (network, slug string, err error) {
	idx := strings.Index(s, ":")
	if idx < 0 {
		return "", "", fmt.Errorf("expected '<network>:<slug>' (e.g., 'opentable:canlis'); got %q", s)
	}
	network = strings.ToLower(s[:idx])
	slug = s[idx+1:]
	if slug == "" {
		return "", "", fmt.Errorf("empty slug in %q", s)
	}
	if network != "opentable" && network != "tock" && network != "resy" {
		return "", "", fmt.Errorf("unknown network %q (expected 'opentable', 'tock', or 'resy')", network)
	}
	return network, slug, nil
}

// acquireBookLock creates an exclusive lock file via O_CREATE|O_EXCL.
// Returns a release callback that deletes the lock file on call. The lock
// file lives under <UserCacheDir>/table-reservation-goat-pp-cli/book-locks/.
func acquireBookLock(network, slug, date, hhmm string, party int) (string, func(), error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", func() {}, fmt.Errorf("user cache dir: %w", err)
	}
	dir := filepath.Join(cacheDir, "table-reservation-goat-pp-cli", "book-locks")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", func() {}, fmt.Errorf("creating lock dir: %w", err)
	}
	keyStr := fmt.Sprintf("%s|%s|%s|%s|%d", network, normalizeSlug(slug), date, hhmm, party)
	sum := sha256.Sum256([]byte(keyStr))
	lockPath := filepath.Join(dir, hex.EncodeToString(sum[:16])+".lock")
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		// Most likely cause: another process is mid-book on the same key.
		// Stale locks (process crashed): file exists for >10 min — opportunistic stale-clean.
		if info, statErr := os.Stat(lockPath); statErr == nil && time.Since(info.ModTime()) > 10*time.Minute {
			_ = os.Remove(lockPath)
			f, err = os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		}
		if err != nil {
			return "", func() {}, fmt.Errorf("lock acquire failed: %w", err)
		}
	}
	_, _ = f.WriteString(fmt.Sprintf("pid=%d started=%s\n", os.Getpid(), time.Now().UTC().Format(time.RFC3339)))
	_ = f.Close()
	return lockPath, func() { _ = os.Remove(lockPath) }, nil
}

// normalizeSlug applies R13's normalization for the lock key (and for
// idempotency comparison): lowercase, trimmed.
func normalizeSlug(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// normalizeTime returns "HH:MM" (24h) for any of "19:00", "7:00 PM", "07:00 pm".
// Used for idempotency comparison (R13). Returns the input unchanged if
// parse fails — caller falls back to exact-string compare.
func normalizeTime(s string) string {
	s = strings.TrimSpace(s)
	if len(s) == 0 {
		return s
	}
	// Already 24h?
	if t, err := time.Parse("15:04", s); err == nil {
		return t.Format("15:04")
	}
	if t, err := time.Parse("3:04 PM", s); err == nil {
		return t.Format("15:04")
	}
	if t, err := time.Parse("3:04 pm", strings.ToLower(s)); err == nil {
		return t.Format("15:04")
	}
	return s
}

// bookOnNetwork dispatches to the network-specific book flow. Returns the
// agent-friendly result struct PLUS a non-nil error when the result has an
// Error category (so the caller can propagate exit code).
func bookOnNetwork(ctx context.Context, session *auth.Session, network, slug, date, hhmm string, party int, dryRun bool) (bookResult, error) {
	out := bookResult{Network: network, RestaurantSlug: slug, Date: date, Time: hhmm, Party: party}
	switch network {
	case "opentable":
		return bookOnOpenTable(ctx, session, slug, date, hhmm, party, dryRun, out)
	case "tock":
		return bookOnTock(ctx, session, slug, date, hhmm, party, dryRun, out)
	case "resy":
		return bookOnResy(ctx, session, slug, date, hhmm, party, dryRun, out)
	}
	out.Error = "unknown_network"
	return out, fmt.Errorf("unknown network %q", network)
}

// bookOnResy mirrors the OT/Tock dispatch shape but talks to Resy's
// /4/find → /3/details → /3/book chain. The `slug` argument here is Resy's
// numeric venue id stringified (e.g. "1387") — Resy's URLs use a slug+code
// pair (`/cities/<cityCode>/<slug>`) that is not a stable booking key.
// Agents looking up venues should use `search` or `goat --network resy`,
// then feed the numeric `id` field into book.
func bookOnResy(ctx context.Context, session *auth.Session, venueID, date, hhmm string, party int, dryRun bool, out bookResult) (bookResult, error) {
	// Validate the venue id BEFORE the auth check so a malformed call
	// gets a "fix your input" error regardless of session state. Resy
	// venues are addressed by numeric id (Resy's `id` field from
	// search). Slug-shaped inputs (`book resy:le-bernardin`) used to
	// yield a successful dry-run envelope but live booking later failed
	// with an opaque "invalid venue_id" from Resy. earliest/watch/drift
	// already validate this; book now matches.
	if _, err := strconv.Atoi(venueID); err != nil {
		out.Error = "malformed_argument"
		out.Hint = fmt.Sprintf("resy: %q is not a numeric venue id; use the `id` field from `goat <name> --network resy` output (e.g. resy:1387)", venueID)
		return out, fmt.Errorf("resy venue id not numeric: %q", venueID)
	}
	if session == nil || session.Resy == nil || session.Resy.AuthToken == "" {
		out.Error = "auth_required"
		out.Hint = "run `auth login --resy --email <you@example.com>` first"
		return out, fmt.Errorf("resy not authenticated")
	}
	client := resy.New(resy.Credentials{
		APIKey:    session.Resy.APIKey,
		AuthToken: session.Resy.AuthToken,
		Email:     session.Resy.Email,
	})

	// Step 5: idempotency pre-flight via /3/user/reservations.
	upcoming, listErr := client.ListReservations(ctx)
	if listErr != nil {
		switch {
		case errors.Is(listErr, resy.ErrAuthExpired):
			out.Error = "auth_expired"
		case errors.Is(listErr, resy.ErrAuthMissing):
			out.Error = "auth_required"
		default:
			out.Error = "preflight_failed"
		}
		out.Hint = listErr.Error()
		return out, listErr
	}
	normTime := normalizeTime(hhmm)
	for _, r := range upcoming {
		// Filter terminal-state rows first. /3/user/reservations returns
		// both active AND historical rows (Completed, Cancelled,
		// No-show), so matching on (venueID, date, time, party) alone
		// would let a same-day cancellation silently abort a legitimate
		// re-booking. We only consider rows whose status is empty OR
		// indicates "still active" — anything matching a known terminal
		// label is excluded.
		if isResyTerminalStatus(r.Status) {
			continue
		}
		if r.PartySize != party || r.Date != date {
			continue
		}
		if normalizeTime(r.Time) != normTime {
			continue
		}
		// Venue ID is REQUIRED for a positive match. Resy's
		// /3/user/reservations sometimes returns rows with an empty
		// `venue.id` (the venue name lives in share.generic_message and
		// the API has been observed omitting the structured venue
		// object). Treating "empty VenueID + date/time/party match" as
		// a hit would falsely claim a reservation at a DIFFERENT venue
		// already exists and silently abort the new booking. Skip
		// venue-ambiguous rows entirely — better to risk a duplicate
		// pre-flight match miss (Resy's own /3/book will surface that
		// as a typed error) than to suppress a legitimate booking.
		if r.VenueID == "" || r.VenueID != venueID {
			continue
		}
		out.MatchedExisting = true
		out.Source = "matched_existing"
		out.ReservationID = r.ID
		out.ConfirmationNumber = r.ID
		out.RestaurantName = r.VenueName
		return out, nil
	}

	// Step 6: dry-run / commit gate.
	if dryRun || os.Getenv("TRG_ALLOW_BOOK") != "1" {
		out.Source = "dry_run"
		// No BookURL synthesis: Resy URLs are /cities/<cityCode>/<slug>,
		// but the book command only carries the numeric venue ID. The
		// previous shape (`/cities/ny/<numericID>`) returned a 404 and
		// hardcoded NYC for venues elsewhere. Agents wanting a clickable
		// fallback should call `goat <name> --network resy` which
		// returns properly-constructed venue URLs.
		if !dryRun {
			out.Hint = "set TRG_ALLOW_BOOK=1 to commit via the Resy API"
		}
		return out, nil
	}

	// Step 7: resolve a slot token for the requested (date, time, party).
	slots, availErr := client.Availability(ctx, resy.AvailabilityParams{
		VenueID:   venueID,
		Date:      date,
		PartySize: party,
	})
	if availErr != nil {
		out.Error = "availability_fetch_failed"
		out.Hint = availErr.Error()
		return out, availErr
	}
	var slot *resy.Slot
	for i := range slots {
		if normalizeTime(slots[i].Time) == normTime {
			slot = &slots[i]
			break
		}
	}
	if slot == nil {
		out.Error = "slot_not_found"
		out.Hint = fmt.Sprintf("no available Resy slot at %s on %s for party %d at venue %s", hhmm, date, party, venueID)
		return out, fmt.Errorf("no slot found")
	}

	// Step 8: fire the two-step book.
	resp, bookErr := client.Book(ctx, resy.BookRequest{
		VenueID:   venueID,
		Date:      date,
		Time:      hhmm,
		PartySize: party,
		SlotToken: slot.Token,
	})
	if bookErr != nil {
		switch {
		case errors.Is(bookErr, resy.ErrSlotTaken):
			out.Error = "slot_taken"
			out.Hint = "the slot was claimed between availability and book — run `earliest` for a fresh slot"
		case errors.Is(bookErr, resy.ErrNoPaymentMethod):
			out.Error = "payment_required"
			out.Hint = "add a payment method at resy.com/account/payment before booking"
		case errors.Is(bookErr, resy.ErrAuthExpired):
			out.Error = "auth_expired"
			out.Hint = "run `auth login --resy --email <you@example.com>` to refresh the token"
		case errors.Is(bookErr, resy.ErrCanaryUnrecognizedBody):
			out.Error = "discriminator_drift"
			out.Hint = "Resy response shape may have changed; please report"
		default:
			out.Error = "network_error"
			out.Hint = bookErr.Error()
		}
		return out, bookErr
	}
	out.Source = "book"
	out.ReservationID = resp.ResyToken
	out.ConfirmationNumber = resp.ResyToken
	return out, nil
}

// bookOnOpenTable handles steps 5–8 for OT.
func bookOnOpenTable(ctx context.Context, session *auth.Session, slug, date, hhmm string, party int, dryRun bool, out bookResult) (bookResult, error) {
	c, err := opentable.New(session)
	if err != nil {
		out.Error = "client_init_failed"
		out.Hint = err.Error()
		return out, err
	}

	// Step 5: idempotency pre-flight via ListUpcomingReservations.
	upcoming, listErr := c.ListUpcomingReservations(ctx)
	if listErr != nil {
		// Pre-flight failure aborts (R5). Don't fall through to book.
		switch {
		case errors.Is(listErr, opentable.ErrAuthExpired):
			out.Error = "auth_expired"
		default:
			out.Error = "preflight_failed"
		}
		out.Hint = listErr.Error()
		return out, listErr
	}
	for _, r := range upcoming {
		if matchedExistingOT(r, slug, date, hhmm, party) {
			out.MatchedExisting = true
			out.Source = "matched_existing"
			out.ReservationID = fmt.Sprintf("%d", r.ConfirmationNumber)
			out.ConfirmationNumber = fmt.Sprintf("%d", r.ConfirmationNumber)
			out.RestaurantName = r.RestaurantName
			return out, nil
		}
	}

	// Step 6: dry-run / commit gate.
	if dryRun || os.Getenv("TRG_ALLOW_BOOK") != "1" {
		out.Source = "dry_run"
		out.MatchedExisting = false
		if !dryRun {
			out.Hint = "set TRG_ALLOW_BOOK=1 to commit"
		}
		return out, nil
	}

	// Step 7a: resolve restaurant ID + slot tokens via Autocomplete + RestaurantsAvailability.
	restaurantID, restaurantName, _, err := c.RestaurantIDFromQuery(ctx, slug, 47.6062, -122.3321)
	if err != nil || restaurantID == 0 {
		out.Error = "restaurant_not_found"
		out.Hint = "could not resolve OT restaurant ID for slug: " + slug
		return out, fmt.Errorf("ot restaurantId resolve: %w", err)
	}
	out.RestaurantName = restaurantName

	avails, err := c.RestaurantsAvailability(ctx, []int{restaurantID}, date, hhmm, party, 0, 30, 4, false)
	if err != nil {
		out.Error = "availability_fetch_failed"
		out.Hint = err.Error()
		return out, err
	}
	slot := findMatchingSlot(avails, restaurantID, date, hhmm)
	if slot == nil {
		out.Error = "slot_not_found"
		out.Hint = fmt.Sprintf("no available slot at %s on %s for party %d at %s", hhmm, date, party, slug)
		return out, fmt.Errorf("no slot found")
	}

	// Step 7b: fetch profile, build book request.
	profile, err := c.MyProfile(ctx)
	if err != nil {
		out.Error = "profile_fetch_failed"
		out.Hint = err.Error()
		return out, err
	}

	// Step 7c: fire book.
	br := opentable.BookRequest{
		RestaurantID:          restaurantID,
		ReservationDateTime:   date + "T" + hhmm,
		PartySize:             party,
		SlotHash:              slot.SlotHash,
		SlotAvailabilityToken: slot.SlotAvailabilityToken,
		Points:                slot.PointsValue,
		PointsType:            slot.PointsType,
		DiningAreaID:          1,
		FirstName:             profile.FirstName,
		LastName:              profile.LastName,
		Email:                 profile.Email,
		PhoneNumber:           profile.PhoneNumber,
		PhoneNumberCountryID:  profile.PhoneNumberCountryID,
	}
	resp, bookErr := c.Book(ctx, br)
	if bookErr != nil {
		// Map typed errors to JSON categories.
		switch {
		case errors.Is(bookErr, opentable.ErrSlotTaken):
			out.Error = "slot_taken"
			out.Hint = "try `earliest` for a fresh slot"
		case errors.Is(bookErr, opentable.ErrPaymentRequired):
			out.Error = "payment_required"
			out.Hint = "Tock prepaid / OT paid experiences are deferred to v0.3"
		case errors.Is(bookErr, opentable.ErrAuthExpired):
			out.Error = "auth_expired"
		case errors.Is(bookErr, opentable.ErrCanaryUnrecognizedBody):
			out.Error = "discriminator_drift"
			out.Hint = "API error shape may have changed; please report"
		default:
			out.Error = "network_error"
		}
		return out, bookErr
	}

	// Step 7d: optional cancel-cutoff fetch (best-effort).
	cutoff, _ := c.FetchCancelCutoff(ctx, resp.RestaurantID, resp.ConfirmationNumber, resp.SecurityToken)

	out.Source = "book"
	out.ReservationID = fmt.Sprintf("%d", resp.ConfirmationNumber)
	out.ConfirmationNumber = fmt.Sprintf("%d", resp.ConfirmationNumber)
	out.CancellationDeadline = cutoff
	out.MatchedExisting = false
	return out, nil
}

// bookOnTock handles steps 5–8 for Tock via chromedp-attach (real Chrome).
// Card-required venues prompt the user for CVC interactively before
// driving the browser through the click-flow.
func bookOnTock(ctx context.Context, session *auth.Session, slug, date, hhmm string, party int, dryRun bool, out bookResult) (bookResult, error) {
	c, err := tock.New(session)
	if err != nil {
		out.Error = "client_init_failed"
		out.Hint = err.Error()
		return out, err
	}
	// Step 5: pre-flight (best-effort — Tock list-upcoming may return empty
	// or shape-changed sentinel; non-fatal).
	upcoming, _ := c.ListUpcomingReservations(ctx)
	for _, r := range upcoming {
		if matchedExistingTock(r, slug, date, hhmm, party) {
			out.MatchedExisting = true
			out.Source = "matched_existing"
			out.ReservationID = fmt.Sprintf("%d", r.PurchaseID)
			out.ConfirmationNumber = r.ConfirmationNumber
			out.RestaurantName = r.VenueName
			return out, nil
		}
	}
	out.MatchedExisting = false
	// Step 6: dry-run / commit gate.
	if dryRun || os.Getenv("TRG_ALLOW_BOOK") != "1" {
		out.Source = "dry_run"
		out.BookURL = fmt.Sprintf("https://www.exploretock.com/%s?date=%s&size=%d&time=%s", slug, date, party, hhmm)
		if !dryRun {
			out.Hint = "set TRG_ALLOW_BOOK=1 to commit via chromedp-attach (Chrome must be running with --remote-debugging-port=9222)"
		}
		return out, nil
	}
	// Step 7: prompt for CVC (Tock card-required venues need it; free
	// venues ignore the value). User can skip by pressing Enter.
	cvc := promptCVC(os.Stdin, os.Stderr)
	resp, bookErr := c.Book(ctx, tock.BookRequest{
		VenueSlug:       slug,
		ReservationDate: date,
		ReservationTime: hhmm,
		PartySize:       party,
		CVC:             cvc,
	})
	if bookErr != nil {
		switch {
		case errors.Is(bookErr, tock.ErrPaymentRequired):
			out.Error = "payment_required"
			out.Hint = "venue requires full prepayment (v0.3 work)"
		case errors.Is(bookErr, tock.ErrCanaryUnrecognizedBody):
			out.Error = "discriminator_drift"
		default:
			out.Error = "chromedp_book_failed"
			out.Hint = bookErr.Error()
			out.BookURL = fmt.Sprintf("https://www.exploretock.com/%s?date=%s&size=%d&time=%s", slug, date, party, hhmm)
		}
		return out, bookErr
	}
	out.Source = "book"
	if resp != nil {
		if resp.PurchaseID > 0 {
			out.ReservationID = fmt.Sprintf("%d", resp.PurchaseID)
		}
		out.ConfirmationNumber = resp.ConfirmationNumber
		out.RestaurantName = resp.VenueName
		out.BookURL = resp.ReceiptURL
	}
	return out, nil
}

// promptCVC reads a CVC from the given reader, displaying the prompt to
// stderr (so it doesn't pollute the JSON output on stdout). Returns empty
// string on read error or when the user skips by pressing Enter — Tock free
// venues don't need a CVC, so empty is acceptable.
//
// Per system rules + user direction: only the CVC (3-4 digits) is prompted;
// the full credit card number is never asked.
func promptCVC(in *os.File, errOut *os.File) string {
	// Skip prompt entirely in non-interactive contexts (e.g., MCP tool calls).
	// Detect via TRG_TOCK_CVC env var: if set, use that value; else prompt.
	if v := os.Getenv("TRG_TOCK_CVC"); v != "" {
		return v
	}
	stat, err := in.Stat()
	if err != nil || (stat.Mode()&os.ModeCharDevice) == 0 {
		// Non-interactive (piped stdin) — skip prompt; proceed without CVC.
		return ""
	}
	fmt.Fprint(errOut, "Tock card-required venues need CVC re-entry per booking. Enter CVC (or press Enter to skip): ")
	var cvc string
	_, _ = fmt.Fscanln(in, &cvc)
	return strings.TrimSpace(cvc)
}

// matchedExistingTock applies R13 normalization for Tock upcoming-reservations.
func matchedExistingTock(r tock.UpcomingReservation, slug, date, hhmm string, party int) bool {
	if r.PartySize != party {
		return false
	}
	if normalizeSlug(r.VenueSlug) != normalizeSlug(slug) {
		return false
	}
	if r.ReservationDate != date {
		return false
	}
	return normalizeTime(r.ReservationTime) == normalizeTime(hhmm)
}

// findMatchingSlot picks the slot whose date+time matches the request.
// The new RestaurantsAvailability gateway returns dayOffset (days from
// request date) + timeOffsetMinutes (minutes from request time); we look
// for the (0, 0) slot — the requested-date-and-time match.
func findMatchingSlot(avails []opentable.RestaurantAvailability, restaurantID int, date, hhmm string) *opentable.AvailabilitySlot {
	for _, a := range avails {
		if a.RestaurantID != restaurantID {
			continue
		}
		for _, d := range a.AvailabilityDays {
			if d.DayOffset != 0 {
				continue
			}
			for _, s := range d.Slots {
				if s.IsAvailable && s.TimeOffsetMinutes == 0 && s.SlotHash != "" && s.SlotAvailabilityToken != "" {
					return &s
				}
			}
		}
	}
	return nil
}

// matchedExistingOT applies R13 normalization + comparison.
func matchedExistingOT(r opentable.UpcomingReservation, slug, date, hhmm string, party int) bool {
	if r.PartySize != party {
		return false
	}
	// DateTime is "YYYY-MM-DDTHH:MM:SS" or "YYYY-MM-DDTHH:MM"; split on T.
	idx := strings.Index(r.DateTime, "T")
	if idx < 0 {
		return false
	}
	rDate := r.DateTime[:idx]
	rTime := r.DateTime[idx+1:]
	if len(rTime) > 5 {
		rTime = rTime[:5]
	}
	if rDate != date || normalizeTime(rTime) != normalizeTime(hhmm) {
		return false
	}
	// OT's UpcomingReservation doesn't carry a canonical slug. Fall back to
	// requiring every non-empty slug-token to appear in the alphanumeric-only
	// normalization of RestaurantName. For "water-grill-bellevue" that
	// produces {water, grill, bellevue} all matching "watergrillbellevue";
	// "canlis" would not match, so a Canlis booking on the same slot doesn't
	// silently report matched_existing for Water Grill.
	normName := normalizeForSlugMatch(r.RestaurantName)
	if normName == "" {
		return false
	}
	for _, tok := range strings.Split(slug, "-") {
		if tok == "" {
			continue
		}
		if !strings.Contains(normName, tok) {
			return false
		}
	}
	return true
}

// asciiFold runs NFKD decomposition then removes combining marks so accented
// runes ("é", "ñ", "ü") fold to their ASCII bases ("e", "n", "u") before the
// alphanumeric filter. Without this, "Café du Monde" normalizes to
// "cafdumonde" instead of "cafedumonde", breaking idempotency match against
// slug "cafe-du-monde".
var asciiFold = transform.Chain(norm.NFKD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

// isResyTerminalStatus reports whether a Resy reservation's Status string
// indicates the row is no longer a binding live reservation — i.e. it's
// safe to disregard for idempotency-match purposes.
//
// Resy's /3/user/reservations interleaves active and historical rows, so
// (date, time, party, venue) alone is not a unique key — a cancelled
// reservation at the same slot would falsely set matched_existing=true
// and silently abort a legitimate re-booking. We classify here rather than
// at the parser layer because the parser preserves the wire string
// faithfully (some callers need that), but the booking pre-flight needs
// a "is this terminal?" boolean.
//
// Known terminal labels (observed across modern + legacy payloads):
//   - "Cancelled" / "Canceled" (US/UK spellings both appear)
//   - "Completed"  — parser also emits this when structured `finished: 1`
//   - "No-show"    — parser also emits this when structured `no_show: 1`
//
// An empty Status is treated as ACTIVE (the most common shape for live
// rows in modern payloads where the parser couldn't fold a structured
// status into a label).
func isResyTerminalStatus(status string) bool {
	if status == "" {
		return false
	}
	lower := strings.ToLower(strings.TrimSpace(status))
	switch lower {
	case "cancelled", "canceled", "completed", "no-show", "no show", "noshow":
		return true
	}
	// Defensive: match the past-tense terminal forms followed by a
	// word boundary so we catch "cancelled by user", "completed (paid)",
	// "no-show — fee assessed" without misclassifying hypothetical
	// active states like "cancellable" or "completable". Boundary is
	// any non-letter rune or end-of-string.
	for _, prefix := range []string{"cancelled", "canceled", "completed", "no-show", "noshow"} {
		if !strings.HasPrefix(lower, prefix) {
			continue
		}
		if len(lower) == len(prefix) {
			return true
		}
		next := lower[len(prefix)]
		if (next >= 'a' && next <= 'z') || (next >= '0' && next <= '9') {
			// Continued by another letter/digit — different word
			// (e.g., "cancellable" would not match because the next
			// char after "cancel" is 'l'... but the past-tense
			// prefix "cancelled" already has its trailing 'd' so any
			// continuation past the boundary indicates a longer
			// non-terminal word).
			continue
		}
		return true
	}
	return false
}

// normalizeForSlugMatch lowercases s, folds non-ASCII letters to their ASCII
// bases, and strips non-alphanumeric runes so slug tokens can be matched
// against display-name strings.
func normalizeForSlugMatch(s string) string {
	folded, _, err := transform.String(asciiFold, s)
	if err != nil {
		// Fold failures are rare (transform errors are mostly EOF/buffer);
		// fall back to the raw string so we still produce useful output.
		folded = s
	}
	var b strings.Builder
	b.Grow(len(folded))
	for _, r := range strings.ToLower(folded) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
