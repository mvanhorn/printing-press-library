package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/commerce/spothero/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/commerce/spothero/internal/consumer"
	"github.com/mvanhorn/printing-press-library/library/commerce/spothero/internal/session"
)

const bookConfirmPhrase = "PLACE SPOTHERO BOOKING"

// hideFromMCP keeps a command out of the MCP surface. place and cancel are CLI only:
// in MCP the `confirm` value is chosen by the model, which is not a human approval,
// so an agent over MCP can search and preview but never spend or refund.
func hideFromMCP(cmd *cobra.Command) {
	if cmd.Annotations == nil {
		cmd.Annotations = map[string]string{}
	}
	cmd.Annotations["mcp:hidden"] = "true"
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		root.AddCommand(newFindCmd(flags), newBookCmd(flags), newCancelCmd(flags), newAccountCmd(flags))
		// Old flag on doctor, which now always probes live. Keeps existing scripts
		// working.
		for _, c := range root.Commands() {
			if c.Name() == "doctor" {
				c.Flags().Bool("live", false, "Deprecated: doctor always probes the API")
				_ = c.Flags().MarkHidden("live")
			}
		}
	})
}

// inventoryBaseURL derives the public inventory host from the spec's facilities
// endpoint, so the spec stays the one place that names it. The env var lets tests point
// inventory at a fake server.
func inventoryBaseURL() string {
	if v := cliutil.EnvOverride("SPOTHERO_CRAIG_BASE_URL"); v != "" {
		return v
	}
	return strings.TrimSuffix(resourceDetailPaths["facilities"], "/search/transient/{id}")
}

// parseDollars converts --max-total, written in dollars, to cents. Rounding is by
// string so 30.10 cannot become 3009 through float error.
func parseDollars(s string) (int, error) {
	s = strings.TrimPrefix(strings.TrimSpace(s), "$")
	if s == "" {
		return 0, nil
	}
	whole, frac, _ := strings.Cut(s, ".")
	if len(frac) > 2 {
		return 0, fmt.Errorf("--max-total %q has more than two decimals", s)
	}
	frac = (frac + "00")[:2]
	dollars, err := strconv.Atoi(whole)
	if err != nil || dollars < 0 {
		return 0, fmt.Errorf("--max-total %q is not a dollar amount", s)
	}
	cents, err := strconv.Atoi(frac)
	if err != nil {
		return 0, fmt.Errorf("--max-total %q is not a dollar amount", s)
	}
	// Zero would read as "no cap" downstream, the opposite of what someone typing 0 means.
	if dollars*100+cents == 0 {
		return 0, fmt.Errorf("--max-total must be greater than zero")
	}
	return dollars*100 + cents, nil
}

func newFindCmd(flags *rootFlags) *cobra.Command {
	var address, starts, ends string
	var lat, lng float64
	cmd := &cobra.Command{
		Use:         "find",
		Short:       "Search live parking near an address or point for a time window",
		Long:        "Geocodes via spothero.com, then queries live inventory. Naive --starts/--ends use the search city's local time; pass RFC3339 with Z or an offset to override. Use this for live inventory; `search` searches only synced local data.",
		Example:     "  spothero-pp-cli find --address '233 S Wacker Dr, Chicago, IL' --starts 2027-01-15T09:00 --ends 2027-01-15T12:00 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "find")
			}
			if starts == "" || ends == "" {
				return usageErr(fmt.Errorf("--starts and --ends are required (for example 2027-01-15T09:00)"))
			}
			hasAddr := strings.TrimSpace(address) != ""
			hasPoint := lat != 0 || lng != 0
			if hasAddr == hasPoint {
				return usageErr(fmt.Errorf("provide exactly one of --address or --lat/--lng"))
			}
			c, _, err := consumerClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c.Ctx = ctx
			res, err := c.Search(consumer.SearchQuery{Address: address, Latitude: lat, Longitude: lng, Starts: starts, Ends: ends})
			if err != nil {
				return mapConsumerErr(err)
			}
			return printResult(cmd, flags, res, func() ([]string, [][]string) {
				rows := make([][]string, 0, len(res.Results))
				for _, s := range res.Results {
					rows = append(rows, []string{strconv.Itoa(s.FacilityID), s.Title, s.Price, strconv.Itoa(s.DistanceMeters), strconv.FormatBool(s.Available)})
				}
				return []string{"ID", "TITLE", "PRICE", "DIST_M", "AVAILABLE"}, rows
			})
		},
	}
	cmd.Flags().StringVar(&address, "address", "", "Street address or place to search near")
	cmd.Flags().Float64Var(&lat, "lat", 0, "Latitude")
	cmd.Flags().Float64Var(&lng, "lng", 0, "Longitude")
	cmd.Flags().StringVar(&starts, "starts", "", "Parking start (local or RFC3339)")
	cmd.Flags().StringVar(&ends, "ends", "", "Parking end (local or RFC3339)")
	return cmd
}

// printResult renders JSON for agents and a table for people.
func printResult(cmd *cobra.Command, flags *rootFlags, v any, table func() ([]string, [][]string)) error {
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) || table == nil {
		return flags.printJSON(cmd, v)
	}
	headers, rows := table()
	return flags.printTable(cmd, headers, rows)
}

func newBookCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "book", Short: "Preview or place a parking booking (placing is hard gated)"}
	cmd.AddCommand(newBookPreviewCmd(flags), newBookPlaceCmd(flags))
	return cmd
}

type bookInputs struct {
	facilityID                               int
	starts, ends, citySlug, rateID, email    string
	cardExternalID, phone, plate, plateState string
	vehicleProfileID, cardID                 int
	maxTotal                                 string
}

func (b *bookInputs) quote(c *consumer.Client) (consumer.BookPreview, error) {
	preview := consumer.BookPreview{FacilityID: b.facilityID, Starts: b.starts, Ends: b.ends}
	q, err := c.GetFacilityRates(consumer.FacilityRateQuery{FacilityID: b.facilityID, Starts: b.starts, Ends: b.ends, CitySlug: b.citySlug})
	if err != nil {
		return preview, mapConsumerErr(err)
	}
	preview.Title, preview.PeriodsUTC, preview.TimezoneNote = q.Title, q.PeriodsUTC, q.TimezoneNote
	if len(q.Rates) == 0 {
		return preview, nil
	}
	chosen := q.Rates[0]
	if b.rateID != "" {
		found := false
		for _, r := range q.Rates {
			if r.RateID == b.rateID {
				chosen, found = r, true
				break
			}
		}
		if !found {
			return preview, notFoundErr(fmt.Errorf("rate %q not found for facility %d", b.rateID, b.facilityID))
		}
	}
	consumer.ApplyBookPreviewFromRate(&preview, chosen)
	return preview, nil
}

func newBookPreviewCmd(flags *rootFlags) *cobra.Command {
	var in bookInputs
	cmd := &cobra.Command{
		Use:         "preview",
		Short:       "Quote a booking without charging",
		Long:        "Fetches a fresh quote. Never posts a checkout. Naive --starts/--ends use the facility's own time zone.",
		Example:     "  spothero-pp-cli book preview --facility-id 2175 --starts 2027-01-15T09:00 --ends 2027-01-15T12:00 --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--facility-id=2175;--starts=2027-01-15T09:00;--ends=2027-01-15T12:00"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "book preview")
			}
			if in.facilityID == 0 || in.starts == "" || in.ends == "" {
				return usageErr(fmt.Errorf("--facility-id, --starts and --ends are required"))
			}
			capCents, err := parseDollars(in.maxTotal)
			if err != nil {
				return usageErr(err)
			}
			c, _, err := consumerClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c.Ctx = ctx
			p, err := in.quote(c)
			if err != nil {
				return err
			}
			p.DryRun = true
			p.MaxTotalCents = capCents
			p.OverMaxTotal = capCents > 0 && p.PriceCents > capCents
			p.Message = "Preview only, no charge. To place: book place with --enable-live-booking --owner-approved --confirm \"" + bookConfirmPhrase + "\""
			return flags.printJSON(cmd, p)
		},
	}
	cmd.Flags().IntVar(&in.facilityID, "facility-id", 0, "Facility id from find results")
	cmd.Flags().StringVar(&in.starts, "starts", "", "Parking start (local or RFC3339)")
	cmd.Flags().StringVar(&in.ends, "ends", "", "Parking end (local or RFC3339)")
	cmd.Flags().StringVar(&in.citySlug, "city-slug", "", "Optional. Time zone fallback for naive datetimes when the facility publishes none")
	cmd.Flags().StringVar(&in.rateID, "rate-id", "", "A specific rate id from book preview")
	cmd.Flags().StringVar(&in.maxTotal, "max-total", "", "Refuse if the fresh quote total exceeds this many dollars (for example 30.00)")
	return cmd
}

func newBookPlaceCmd(flags *rootFlags) *cobra.Command {
	var in bookInputs
	var enableLive, ownerApproved bool
	var confirm string
	cmd := &cobra.Command{
		Use:   "place",
		Short: "Place a live booking and charge the card (hard gated)",
		Long: "Charges a saved card, only with all of --enable-live-booking --owner-approved --confirm \"" + bookConfirmPhrase + "\".\n" +
			"With none of those it shows the preview and places nothing. A partial or wrong gate is refused with exit 2.\n" +
			"--dry-run builds the request with secrets masked and never posts. It is never retried: if the result is ambiguous it reads reservations and reports, and you must check before trying again.",
		Example: "  spothero-pp-cli book place --facility-id 2175 --starts 2027-01-15T09:00 --ends 2027-01-15T12:00 --max-total 30\n" +
			"  spothero-pp-cli book place --facility-id 2175 --starts 2027-01-15T09:00 --ends 2027-01-15T12:00 --dry-run",
		Annotations: map[string]string{"pp:preview-happy-path": "true", "pp:data-source": "live", "pp:happy-args": "--facility-id=2175;--starts=2027-01-15T09:00;--ends=2027-01-15T12:00"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// A bare book place with nothing set is a usage question, not a request.
			if !hasChangedLocalFlags(cmd) && !requiredInputWaived() && !dryRunOK(flags) {
				return cmd.Help()
			}
			if in.facilityID == 0 || in.starts == "" || in.ends == "" {
				if requiredInputWaived() || dryRunOK(flags) {
					return writeDryRun(cmd.OutOrStdout(), flags, "book place")
				}
				return usageErr(fmt.Errorf("--facility-id, --starts and --ends are required"))
			}
			capCents, err := parseDollars(in.maxTotal)
			if err != nil {
				return usageErr(err)
			}
			noGates := !enableLive && !ownerApproved && confirm == ""
			allGates := enableLive && ownerApproved && confirm == bookConfirmPhrase
			if !noGates && !allGates {
				return usageErr(fmt.Errorf("refusing live booking: require --enable-live-booking --owner-approved --confirm %q", bookConfirmPhrase))
			}
			if allGates && flags.runProfileName != "" && !flags.dryRun {
				// A saved profile can hold flag values, so a hand-edited one could
				// otherwise supply the gates. Approval has to be typed each time.
				return usageErr(fmt.Errorf("refusing live booking under --profile: type the gates on the command line without a profile"))
			}
			c, _, err := consumerClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c.Ctx = ctx
			// A verify harness must never reach the checkout, gates or not.
			// --dry-run wins over the preview so it always shows the masked request. The
			// harness has no fixture for it, so it keeps the plain preview below.
			if flags.dryRun && !harnessActive() {
				return dryRunCheckout(cmd, flags, c, in, capCents)
			}
			if noGates || harnessActive() {
				p, err := in.quote(c)
				if err != nil {
					return err
				}
				p.DryRun = true
				p.MaxTotalCents = capCents
				p.OverMaxTotal = capCents > 0 && p.PriceCents > capCents
				p.Message = "Not placed. Preview only: this command charges only with --enable-live-booking --owner-approved --confirm \"" + bookConfirmPhrase + "\""
				return flags.printJSON(cmd, p)
			}
			req, err := buildCheckout(c, in, capCents)
			if err != nil {
				return err
			}
			out, err := c.Checkout(req)
			if err != nil {
				if amb, ok := err.(*consumer.AmbiguousError); ok {
					return reportAmbiguousBooking(cmd, flags, c, req, amb)
				}
				return mapConsumerErr(fmt.Errorf("not placed: %w", err))
			}
			if !consumer.CheckoutSucceeded(out) {
				return reportAmbiguousBooking(cmd, flags, c, req, &consumer.AmbiguousError{Operation: "booking", Detail: "the response named no purchase or order"})
			}
			return flags.printJSON(cmd, map[string]any{"placed": true, "booked": true, "total_price_cents": req.TotalPrice, "result": safeResult(out, "order_id", "checkout_id", "status", "total_price", "currency")})
		},
	}
	cmd.Flags().IntVar(&in.facilityID, "facility-id", 0, "Facility id from find results")
	cmd.Flags().StringVar(&in.starts, "starts", "", "Parking start (local or RFC3339)")
	cmd.Flags().StringVar(&in.ends, "ends", "", "Parking end (local or RFC3339)")
	cmd.Flags().StringVar(&in.citySlug, "city-slug", "", "Optional. Time zone fallback for naive datetimes when the facility publishes none")
	cmd.Flags().StringVar(&in.rateID, "rate-id", "", "A specific rate id from book preview")
	cmd.Flags().StringVar(&in.maxTotal, "max-total", "", "Refuse if the fresh quote total exceeds this many dollars (for example 30.00)")
	cmd.Flags().StringVar(&in.email, "email", "", "Receipt email (default from the account)")
	cmd.Flags().IntVar(&in.vehicleProfileID, "vehicle-profile-id", 0, "Saved vehicle id")
	cmd.Flags().IntVar(&in.cardID, "card-id", 0, "Saved card id")
	cmd.Flags().StringVar(&in.cardExternalID, "card-external-id", "", "Saved card UUID (overrides --card-id)")
	cmd.Flags().StringVar(&in.phone, "phone", "", "E.164 phone (default from the account)")
	cmd.Flags().StringVar(&in.plate, "license-plate", "", "Ad hoc license plate")
	cmd.Flags().StringVar(&in.plateState, "license-plate-state", "", "Ad hoc license plate state")
	cmd.Flags().BoolVar(&enableLive, "enable-live-booking", false, "Explicit opt in to charging a card")
	cmd.Flags().BoolVar(&ownerApproved, "owner-approved", false, "The owner approved this exact booking")
	cmd.Flags().StringVar(&confirm, "confirm", "", "Must be exactly: "+bookConfirmPhrase)
	hideFromMCP(cmd)
	return cmd
}

// buildCheckout assembles the request from a fresh quote and enforces the cap, shared by
// the real path and --dry-run so what is previewed is what would be sent.
func buildCheckout(c *consumer.Client, in bookInputs, capCents int) (consumer.CheckoutRequest, error) {
	if c.Session == nil || c.Session.CookieHeader() == "" {
		return consumer.CheckoutRequest{}, authErr(fmt.Errorf("a session is required; run auth login --chrome"))
	}
	req, _, err := c.BuildCheckout(consumer.BookPlaceInput{
		FacilityID: in.facilityID, Starts: in.starts, Ends: in.ends, CitySlug: in.citySlug, Email: in.email, SelectRateID: in.rateID,
		VehicleProfileID: in.vehicleProfileID, CardID: in.cardID, CardExternalID: in.cardExternalID, PhoneNumber: in.phone,
		LicensePlateStr: in.plate, LicensePlateState: in.plateState,
	})
	if err != nil {
		return consumer.CheckoutRequest{}, mapConsumerErr(err)
	}
	if capCents > 0 && req.TotalPrice > capCents {
		return consumer.CheckoutRequest{}, usageErr(fmt.Errorf("fresh quote total %d cents exceeds --max-total %d cents; not placed", req.TotalPrice, capCents))
	}
	return req, nil
}

func dryRunCheckout(cmd *cobra.Command, flags *rootFlags, c *consumer.Client, in bookInputs, capCents int) error {
	req, err := buildCheckout(c, in, capCents)
	if err != nil {
		return err
	}
	return flags.printJSON(cmd, map[string]any{"dry_run": true, "placed": false, "would_post": consumer.PathCheckout, "request": maskCheckout(req)})
}

// maskCheckout hides the signed quote and contact details from dry-run output, which
// tends to land in logs and transcripts. The shape stays inspectable.
func maskCheckout(req consumer.CheckoutRequest) consumer.CheckoutRequest {
	req.Email = "***"
	req.Items = append([]consumer.CheckoutItem(nil), req.Items...)
	for i := range req.Items {
		req.Items[i].ItemContext.QuoteToken = "***"
		req.Items[i].ItemContext.PhoneNumber = "***"
		if req.Items[i].ItemContext.LicensePlateStr != "" {
			req.Items[i].ItemContext.LicensePlateStr = "***"
		}
	}
	return req
}

// reportAmbiguousBooking reads state rather than retrying. A retry here could charge
// twice, so even a clean-looking read leaves the decision with the owner.
func reportAmbiguousBooking(cmd *cobra.Command, flags *rootFlags, c *consumer.Client, req consumer.CheckoutRequest, amb *consumer.AmbiguousError) error {
	report := map[string]any{"placed": "unknown", "error": amb.Error(), "retried": false}
	// The command's timeout may be what just expired, and this read is the one thing
	// the owner most needs, so it gets its own budget.
	lookupCtx, cancelLookup := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelLookup()
	c.Ctx = lookupCtx
	ctx := req.Items[0].ItemContext
	matches, err := c.MatchingReservations(ctx.Facility, ctx.Starts)
	switch {
	case err != nil:
		report["reservation_lookup"] = "failed: " + err.Error()
	default:
		ids := make([]string, 0, len(matches))
		for _, r := range matches {
			ids = append(ids, r.ID)
		}
		report["matching_reservations"] = ids
		report["reservation_lookup"] = "ok"
	}
	_ = flags.printJSON(cmd, report)
	return apiErr(amb)
}

func newCancelCmd(flags *rootFlags) *cobra.Command {
	var confirm string
	cmd := &cobra.Command{
		Use:   "cancel [reservation-id]",
		Short: "Cancel a reservation (needs a token from cancel preview)",
		Long: "Run `cancel preview <id>` first, then `cancel <id> --yes --confirm <token>`.\n" +
			"The token is single use, expires in 15 minutes, and fixes the reservation, reason and refund destination shown by the preview.\n" +
			"Never retried: an ambiguous result reads the reservation and reports.",
		Example:     "  spothero-pp-cli cancel 12345678 --dry-run\n  spothero-pp-cli cancel 12345678 --yes --confirm TOKEN_FROM_PREVIEW",
		Args:        cobra.MaximumNArgs(1),
		Annotations: map[string]string{"pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return writeDryRun(cmd.OutOrStdout(), flags, "cancel")
				}
				return usageErr(fmt.Errorf("provide a reservation id, or use: cancel preview <id>"))
			}
			id := args[0]
			// Answered before confirmHome: with no token this only prints "Not cancelled" and
			// reaches neither the session nor the network, so a bad cookie must not fail it.
			if flags.dryRun && confirm == "" {
				return flags.printJSON(cmd, map[string]any{"dry_run": true, "cancelled": false, "reservation_id": id, "message": "Not cancelled. Run cancel preview, then --yes --confirm <token>."})
			}
			home, err := confirmHome(flags)
			if err != nil {
				return err
			}
			if !flags.yes || confirm == "" {
				return usageErr(fmt.Errorf("refusing to cancel: run `cancel preview %s` first, then pass --yes --confirm <token>", id))
			}
			if flags.runProfileName != "" && !flags.dryRun {
				// Same rule as book place: a saved profile must not supply approval.
				return usageErr(fmt.Errorf("refusing to cancel under --profile: type --yes and --confirm without a profile"))
			}
			binding, err := session.PeekCancelBinding(home, confirm, id)
			if err != nil {
				return usageErr(err)
			}
			c, _, err := consumerClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c.Ctx = ctx
			res, err := c.GetReservation(id)
			if err != nil {
				return mapConsumerErr(err)
			}
			req := consumer.RefundRequest{Reason: binding.Reason, RefundAsCredit: binding.RefundAsCredit, AccessKey: res.AccessKey}
			if flags.dryRun || harnessActive() {
				return flags.printJSON(cmd, map[string]any{"dry_run": true, "cancelled": false, "would_post": fmt.Sprintf(consumer.PathReservationRefund, id), "reason": req.Reason, "refund_as_credit": req.RefundAsCredit})
			}
			// Re-check at the last moment: eligibility can lapse after the preview, and
			// the token is only spent once the request is really going out.
			elig, err := c.GetRefundEligibility(id)
			if err != nil {
				return mapConsumerErr(err)
			}
			if !refundRouteOpen(elig, binding.RefundAsCredit) {
				return usageErr(fmt.Errorf("reservation %s is no longer refundable by this route; run cancel preview again", id))
			}
			if err := session.ConsumeCancelToken(home, confirm, id); err != nil {
				return usageErr(err)
			}
			out, err := c.Refund(id, req)
			if err != nil {
				if amb, ok := err.(*consumer.AmbiguousError); ok {
					return reportAmbiguousCancel(cmd, flags, c, id, amb)
				}
				return mapConsumerErr(fmt.Errorf("not cancelled: %w", err))
			}
			// The site's own client ignores this response body, so it proves nothing.
			// The reservation's status is the evidence.
			after, gerr := c.GetReservation(id)
			if gerr != nil || !strings.EqualFold(after.Status, "cancelled") {
				return reportAmbiguousCancel(cmd, flags, c, id, &consumer.AmbiguousError{Operation: "cancellation", Detail: "the reservation does not yet show as cancelled"})
			}
			return flags.printJSON(cmd, map[string]any{"cancelled": true, "refund_as_credit": req.RefundAsCredit, "status_now": after.Status, "result": safeResult(out, "status", "refund_amount", "amount", "message")})
		},
	}
	cmd.Flags().StringVar(&confirm, "confirm", "", "Token from cancel preview")
	hideFromMCP(cmd)
	cmd.AddCommand(newCancelPreviewCmd(flags))
	return cmd
}

// reportAmbiguousCancel reads state instead of retrying, on a fresh budget for the same
// reason as the booking path.
func reportAmbiguousCancel(cmd *cobra.Command, flags *rootFlags, c *consumer.Client, id string, amb *consumer.AmbiguousError) error {
	report := map[string]any{"cancelled": "unknown", "error": amb.Error(), "retried": false}
	lookupCtx, cancelLookup := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancelLookup()
	c.Ctx = lookupCtx
	if after, err := c.GetReservation(id); err == nil {
		report["status_now"] = after.Status
	}
	_ = flags.printJSON(cmd, report)
	return apiErr(amb)
}

func refundRouteOpen(e consumer.RefundEligibility, asCredit bool) bool {
	if asCredit {
		return e.CreditOnly
	}
	return e.OriginalSource
}

// confirmHome keeps tokens next to the config so preview and cancel can run in
// separate processes.
func confirmHome(flags *rootFlags) (string, error) {
	_, cfg, err := consumerClient(flags)
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(cfg.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", configErr(err)
	}
	return dir, nil
}

func newCancelPreviewCmd(flags *rootFlags) *cobra.Command {
	var reason string
	var asCredit bool
	cmd := &cobra.Command{
		Use:     "preview <reservation-id>",
		Short:   "See whether a reservation can be refunded and get a confirm token",
		Long:    "Reads refund eligibility and issues a single-use token bound to this reservation, reason and refund destination. Changes nothing at SpotHero.",
		Example: "  spothero-pp-cli cancel preview 12345678 --agent",
		Args:    cobra.MaximumNArgs(1),
		// An account-specific id cannot be a portable fixture, so the live matrix
		// records this as a blocked fixture rather than baking in a real reservation.
		Annotations: map[string]string{"mcp:read-only": "false", "mcp:local-write": "true", "pp:data-source": "live", "pp:happy-args": "reservation-id=12345678"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				if dryRunOK(flags) {
					return writeDryRun(cmd.OutOrStdout(), flags, "cancel preview")
				}
				return usageErr(fmt.Errorf("provide a reservation id"))
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "cancel preview")
			}
			id := args[0]
			home, err := confirmHome(flags)
			if err != nil {
				return err
			}
			c, _, err := consumerClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c.Ctx = ctx
			res, err := c.GetReservation(id)
			if err != nil {
				return mapConsumerErr(err)
			}
			elig, err := c.GetRefundEligibility(id)
			if err != nil {
				return mapConsumerErr(err)
			}
			if !elig.Eligible() {
				return usageErr(fmt.Errorf("reservation %s is not refundable; cancel not available", id))
			}
			if !refundRouteOpen(elig, asCredit) {
				if !asCredit {
					return usageErr(fmt.Errorf("reservation %s can only be refunded as SpotHero credit; rerun with --refund-as-credit", id))
				}
				return usageErr(fmt.Errorf("reservation %s cannot be refunded as credit", id))
			}
			chosen, err := chooseReason(elig, reason)
			if err != nil {
				return usageErr(err)
			}
			token, err := session.IssueBoundCancelToken(home, id, session.CancelBinding{Reason: chosen, RefundAsCredit: asCredit})
			if err != nil {
				return configErr(err)
			}
			return flags.printJSON(cmd, consumer.CancelPreview{
				ReservationID: id, Status: res.Status, Refundable: true,
				RefundToOriginal: elig.OriginalSource, RefundAsCreditAvailable: elig.CreditOnly, RefundAsCredit: asCredit,
				Reason: chosen, AvailableReasons: elig.ReasonNames(), PriceCents: res.PriceCents,
				ConfirmToken: token,
				Message:      "Nothing changed yet. To cancel: spothero-pp-cli cancel " + id + " --yes --confirm <confirm_token>",
			})
		},
	}
	cmd.Flags().StringVar(&reason, "reason", "", "Cancellation reason, from available_reasons")
	cmd.Flags().BoolVar(&asCredit, "refund-as-credit", false, "Refund as SpotHero credit instead of to the original payment")
	return cmd
}

// chooseReason picks what the site's dialog would send. The API wants a value from
// its own list, so a free-form reason is refused rather than sent.
func chooseReason(e consumer.RefundEligibility, wanted string) (string, error) {
	names := e.ReasonNames()
	if wanted != "" {
		for _, n := range names {
			if strings.EqualFold(n, wanted) {
				return n, nil
			}
		}
		return "", fmt.Errorf("--reason %q is not offered; choose one of: %s", wanted, strings.Join(names, "; "))
	}
	for _, n := range names {
		if strings.EqualFold(n, "No longer need parking") {
			return n, nil
		}
	}
	if len(names) > 0 {
		return names[0], nil
	}
	return "", fmt.Errorf("the API offered no cancellation reasons")
}

func newAccountCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{Use: "account", Short: "Read saved cards and vehicles"}
	cards := &cobra.Command{
		Use:         "cards",
		Short:       "List saved payment cards (last4 and ids only)",
		Example:     "  spothero-pp-cli account cards --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "account cards")
			}
			c, _, err := consumerClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c.Ctx = ctx
			me, err := c.GetMe()
			if err != nil {
				return mapConsumerErr(err)
			}
			if me, err = c.EnrichCreditCards(me); err != nil {
				return mapConsumerErr(err)
			}
			return flags.printJSON(cmd, map[string]any{"count": len(me.CreditCards), "cards": me.CreditCards})
		},
	}
	vehicles := &cobra.Command{
		Use:         "vehicles",
		Short:       "List saved vehicles with their ids and license plates",
		Example:     "  spothero-pp-cli account vehicles --agent",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "account vehicles")
			}
			c, _, err := consumerClient(flags)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c.Ctx = ctx
			me, err := c.GetMe()
			if err != nil {
				return mapConsumerErr(err)
			}
			list, err := c.ListVehicles(me.ID)
			if err != nil {
				return mapConsumerErr(err)
			}
			return flags.printJSON(cmd, map[string]any{"count": len(list), "vehicles": list})
		},
	}
	cmd.AddCommand(cards, vehicles)
	return cmd
}

// upcoming keeps valid reservations that have not ended, soonest first.
func upcoming(list []consumer.Reservation, now time.Time) []consumer.Reservation {
	var out []consumer.Reservation
	for _, r := range list {
		end, err := time.Parse(time.RFC3339, r.Ends)
		if err != nil || !end.After(now) || !strings.EqualFold(r.Status, "valid") {
			continue
		}
		out = append(out, r)
	}
	sortReservationsByStart(out)
	return out
}

func sortReservationsByStart(rs []consumer.Reservation) {
	for i := 1; i < len(rs); i++ {
		for j := i; j > 0 && rs[j].Starts < rs[j-1].Starts; j-- {
			rs[j], rs[j-1] = rs[j-1], rs[j]
		}
	}
}

// harnessActive is true under mock verify or live dogfood. Those harnesses sweep every
// command, so this is a second lock: they can never reach a checkout or a refund, even
// if a future happy-path fixture carried every gate.
func harnessActive() bool { return cliutil.IsVerifyEnv() || cliutil.IsDogfoodEnv() }

// safeResult keeps only the named fields of a checkout or refund response. The raw
// bodies carry an order access key and a saved card token, which must not reach
// stdout, logs or an agent's transcript.
func safeResult(in map[string]any, keep ...string) map[string]any {
	out := make(map[string]any, len(keep))
	for _, k := range keep {
		if v, ok := in[k]; ok {
			out[k] = v
		}
	}
	return out
}
