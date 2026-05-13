// Hand-written transcendence commands for pointhound-pp-cli.
//
// All commands here either:
//   - Call the real /api/offers endpoint via flags.newClient() and resolveRead
//   - Call scout.pointhound.com via internal/scout
//   - Read from the local store written by sync/watch
//
// // pp:client-call
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/enetx/g"
	"github.com/enetx/surf"
	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/pointhound/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/pointhound/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/travel/pointhound/internal/protobuf"
	"github.com/mvanhorn/printing-press-library/library/travel/pointhound/internal/scout"
	"github.com/mvanhorn/printing-press-library/library/travel/pointhound/internal/store"
)

// offerSummary is the projection used by every transcendence command that
// joins offers data.
type offerSummary struct {
	ID                string `json:"id"`
	OriginCode        string `json:"originCode"`
	DestinationCode   string `json:"destinationCode"`
	DepartsAt         string `json:"departsAt"`
	ArrivesAt         string `json:"arrivesAt"`
	CabinClass        string `json:"cabinClass"`
	TotalStops        int    `json:"totalStops"`
	TotalDuration     int    `json:"totalDuration"`
	PricePoints       int    `json:"pricePoints"`
	BestPricePoints   int    `json:"bestPricePoints"`
	PriceRetailTotal  string `json:"priceRetailTotal"`
	PriceCurrency     string `json:"priceRetailCurrency"`
	AirlinesList      string `json:"airlinesList"`
	FlightNumbers     string `json:"flightNumbers"`
	QuantityRemaining int    `json:"quantityRemaining"`
	SourceIdentifier  string `json:"sourceIdentifier"`
	RedeemProgramID   string `json:"redeemProgramId,omitempty"`
	RedeemProgramName string `json:"redeemProgramName,omitempty"`
}

type rawOffer struct {
	ID                  string               `json:"id"`
	OriginCode          string               `json:"originCode"`
	DestinationCode     string               `json:"destinationCode"`
	DepartsAt           string               `json:"departsAt"`
	ArrivesAt           string               `json:"arrivesAt"`
	CabinClass          string               `json:"cabinClass"`
	TotalStops          int                  `json:"totalStops"`
	TotalDuration       int                  `json:"totalDuration"`
	PricePoints         int                  `json:"pricePoints"`
	BestPricePoints     int                  `json:"bestPricePoints"`
	PriceRetailTotal    string               `json:"priceRetailTotal"`
	PriceCurrency       string               `json:"priceRetailCurrency"`
	PricePerPoint       string               `json:"pricePerPoint"`
	AirlinesList        string               `json:"airlinesList"`
	FlightNumbers       string               `json:"flightNumbers"`
	QuantityRemaining   int                  `json:"quantityRemaining"`
	SourceIdentifier    string               `json:"sourceIdentifier"`
	OfferFlightSegments []offerFlightSegment `json:"offerFlightSegments"`
	Source              struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		Identifier    string `json:"identifier"`
		RedeemProgram struct {
			ID              string                `json:"id"`
			Name            string                `json:"name"`
			TransferOptions []offerTransferOption `json:"transferOptions"`
		} `json:"redeemProgram"`
	} `json:"source"`
}

func (r rawOffer) summary() offerSummary {
	return offerSummary{
		ID:                r.ID,
		OriginCode:        r.OriginCode,
		DestinationCode:   r.DestinationCode,
		DepartsAt:         r.DepartsAt,
		ArrivesAt:         r.ArrivesAt,
		CabinClass:        r.CabinClass,
		TotalStops:        r.TotalStops,
		TotalDuration:     r.TotalDuration,
		PricePoints:       r.PricePoints,
		BestPricePoints:   r.BestPricePoints,
		PriceRetailTotal:  r.PriceRetailTotal,
		PriceCurrency:     r.PriceCurrency,
		AirlinesList:      r.AirlinesList,
		FlightNumbers:     r.FlightNumbers,
		QuantityRemaining: r.QuantityRemaining,
		SourceIdentifier:  r.SourceIdentifier,
		RedeemProgramID:   r.Source.RedeemProgram.ID,
		RedeemProgramName: r.Source.RedeemProgram.Name,
	}
}

// fetchOffers wraps a /api/offers call and returns the parsed offers list.
func fetchOffers(ctx context.Context, flags *rootFlags, searchID string, extra map[string]string) ([]rawOffer, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	params := map[string]string{
		"searchId":   searchID,
		"take":       "50",
		"offset":     "0",
		"sortOrder":  "asc",
		"sortBy":     "points",
		"cabins":     "economy",
		"passengers": "1",
	}
	for k, v := range extra {
		if v != "" {
			params[k] = v
		}
	}
	data, _, err := resolveRead(ctx, c, flags, "offers", false, "/api/offers", params, nil)
	if err != nil {
		return nil, classifyAPIError(err, flags)
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var env struct {
		Data []rawOffer `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decoding offers: %w", err)
	}
	return env.Data, nil
}

// PATCH: Batch is documented as snapshotting fetched offers, so persist the
// exact offer records after each fetch rather than relying only on best-effort
// write-through cache behavior.
func persistOffers(db *store.Store, offers []rawOffer) (int, error) {
	if len(offers) == 0 {
		return 0, nil
	}
	items := make([]json.RawMessage, 0, len(offers))
	for _, offer := range offers {
		item, err := json.Marshal(offer)
		if err != nil {
			return 0, fmt.Errorf("encoding offer for store: %w", err)
		}
		items = append(items, item)
	}
	stored, extractFailures, err := db.UpsertBatch("offers", items)
	if err != nil {
		return stored, fmt.Errorf("storing offers: %w", err)
	}
	if extractFailures > 0 {
		return stored, fmt.Errorf("storing offers: %d offer(s) missing ids", extractFailures)
	}
	return stored, nil
}

// ---------- explore-deal-rating ----------

func newExploreDealRatingCmd(flags *rootFlags) *cobra.Command {
	var metro string
	var minRating string
	var limit int
	var trackedOnly bool

	cmd := &cobra.Command{
		Use:   "explore-deal-rating",
		Short: "List airports with the highest Pointhound deal frequency near a metro",
		Long: strings.TrimSpace(`
Use Pointhound's Scout autocomplete to surface airports the service marks as
"high deal rating" — the same hint shown inline on the search page. Combine
with --metro to group multi-airport cities (e.g. NYC -> JFK/LGA/EWR) and
--tracked-only to keep only airports Pointhound actively tracks.
`),
		Example: strings.Trim(`
  pointhound-pp-cli explore-deal-rating --metro NYC --min-rating high --json
  pointhound-pp-cli explore-deal-rating --metro "san francisco" --tracked-only --limit 20
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if metro == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			c := scout.New("")
			resp, err := c.Search(cmd.Context(), scout.SearchOptions{
				Query: metro,
				Limit: limit,
				Metro: true,
				Bound: false,
				Live:  true,
			})
			if err != nil {
				return err
			}
			results := resp.Results
			if minRating != "" {
				filtered := results[:0]
				for _, r := range results {
					if dealRatingAtLeast(r.DealRating, minRating) {
						filtered = append(filtered, r)
					}
				}
				results = filtered
			}
			if trackedOnly {
				filtered := results[:0]
				for _, r := range results {
					if r.IsTracked {
						filtered = append(filtered, r)
					}
				}
				results = filtered
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), results, flags)
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No airports matched.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "CODE\tNAME\tCITY\tCOUNTRY\tDEAL\tTRACKED")
			for _, r := range results {
				tracked := "no"
				if r.IsTracked {
					tracked = "yes"
				}
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", r.Code, r.Name, r.City, r.CountryCode, r.DealRating, tracked)
			}
			_ = tw.Flush()
			return nil
		},
	}
	cmd.Flags().StringVar(&metro, "metro", "", "Metro or city name (e.g. NYC, 'san francisco').")
	cmd.Flags().StringVar(&minRating, "min-rating", "high", "Only show airports with at least this dealRating ('high' or 'low').")
	cmd.Flags().IntVar(&limit, "limit", 15, "Maximum results from Scout (1-50).")
	cmd.Flags().BoolVar(&trackedOnly, "tracked-only", true, "Only show airports Pointhound actively tracks.")
	return cmd
}

func dealRatingAtLeast(rating, minimum string) bool {
	minimum = strings.TrimSpace(minimum)
	if minimum == "" {
		return true
	}
	minWeight := dealWeight(minimum)
	if minWeight == 0 {
		return strings.EqualFold(rating, minimum)
	}
	return dealWeight(rating) >= minWeight
}

// ---------- compare-transfer ----------

func newCompareTransferCmd(flags *rootFlags) *cobra.Command {
	var searchID string
	var earnProgram string

	cmd := &cobra.Command{
		Use:   "compare-transfer <earn-program>",
		Short: "Rank offers by source-program points spent (uses Pointhound's per-offer transferOptions)",
		Long: strings.TrimSpace(`
Multiply each offer's points cost by the real transfer ratio to your chosen
earn program and sort by lowest source-program-points-spent. Pointhound's
own UI ranks by destination program points; this flips it to "cheapest in
the points I actually have."
`),
		Example: strings.Trim(`
  pointhound-pp-cli compare-transfer chase-ultimate-rewards --search-id ofs_xxx --json
  pointhound-pp-cli compare-transfer "amex membership rewards" --search-id ofs_xxx
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && searchID == "" {
				return cmd.Help()
			}
			if len(args) > 0 {
				earnProgram = args[0]
			}
			if dryRunOK(flags) {
				return nil
			}
			if earnProgram == "" {
				return usageErr(fmt.Errorf("missing earn program (positional arg or --earn-program)"))
			}
			if searchID == "" {
				return usageErr(fmt.Errorf("required flag --search-id not set"))
			}
			offers, err := fetchOffers(cmd.Context(), flags, searchID, nil)
			if err != nil {
				return err
			}
			type ranked struct {
				offerSummary
				EarnProgram     string `json:"earnProgram"`
				TransferRatio   string `json:"transferRatio"`
				EffectivePoints int    `json:"effectivePoints"`
				TransferTime    string `json:"transferTime"`
			}
			needle := strings.ToLower(earnProgram)
			// Initialize as empty slice (not nil) so JSON output is "[]" not "null"
			// when no rows match — agents pipe through jq and "null" trips them up.
			rows := []ranked{}
			for _, o := range offers {
				for _, to := range o.Source.RedeemProgram.TransferOptions {
					ep := to.EarnProgram
					match := strings.Contains(strings.ToLower(ep.Name), needle) ||
						strings.EqualFold(ep.Identifier, earnProgram) ||
						ep.ID == earnProgram
					if !match {
						continue
					}
					ratio, _ := strconv.ParseFloat(to.TotalTransferRatio, 64)
					if ratio <= 0 {
						continue
					}
					eff := int(float64(o.PricePoints) / ratio)
					rows = append(rows, ranked{
						offerSummary:    o.summary(),
						EarnProgram:     ep.Name,
						TransferRatio:   to.TotalTransferRatio,
						EffectivePoints: eff,
						TransferTime:    to.TransferTime,
					})
					break
				}
			}
			sort.SliceStable(rows, func(i, j int) bool {
				return rows[i].EffectivePoints < rows[j].EffectivePoints
			})
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No offers transfer from %q. Try a more common earn program (chase-ultimate-rewards, amex-membership-rewards, capital-one-rewards, bilt-rewards, citi-thankyou-points).\n", earnProgram)
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "ROUTE\tAIRLINES\tSTOPS\tDEST_POINTS\tRATIO\tSOURCE_POINTS\tTIME")
			for _, r := range rows {
				route := r.OriginCode + "→" + r.DestinationCode
				fmt.Fprintf(tw, "%s\t%s\t%d\t%d\t%s\t%d\t%s\n",
					route, r.AirlinesList, r.TotalStops, r.PricePoints, r.TransferRatio, r.EffectivePoints, r.TransferTime)
			}
			_ = tw.Flush()
			return nil
		},
	}
	cmd.Flags().StringVar(&searchID, "search-id", "", "Pointhound search session id (ofs_*). Required.")
	return cmd
}

// ---------- from-home ----------

func newFromHomeCmd(flags *rootFlags) *cobra.Command {
	var origin string
	var balanceStr string
	var searchIDs string
	var cabin string
	var month string
	var limit int

	cmd := &cobra.Command{
		Use:   "from-home [origin]",
		Short: "Where can I fly with the points I actually hold? (balance-aware)",
		Long: strings.TrimSpace(`
For one or more existing search sessions, filter offers down to those reachable
within your supplied points balance, ranked by lowest effective spend. Balance
is a comma-separated map of earn-program-identifier:points pairs (e.g. ur:
chase-ultimate-rewards, mr: amex-membership-rewards, bilt:, c1: capital-one-rewards,
ty: citi-thankyou-points). Aliases are mapped automatically.

Run this AFTER you've kicked off the searches you want to compare. Pair with
the search-ids flag to consider multiple destinations at once.
`),
		Example: strings.Trim(`
  pointhound-pp-cli from-home SFO --balance "ur:250000,mr:80000,bilt:120000" \
      --search-ids ofs_xxx,ofs_yyy --cabin business --json
  pointhound-pp-cli from-home --balance "chase-ultimate-rewards:300000" --search-ids ofs_xxx
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && balanceStr == "" && searchIDs == "" {
				return cmd.Help()
			}
			if len(args) > 0 && origin == "" {
				origin = args[0]
			}
			if dryRunOK(flags) {
				return nil
			}
			if balanceStr == "" {
				return usageErr(fmt.Errorf("required flag --balance not set"))
			}
			if searchIDs == "" {
				return usageErr(fmt.Errorf("required flag --search-ids not set"))
			}
			balances, err := parseBalanceMap(balanceStr)
			if err != nil {
				return err
			}
			type row struct {
				offerSummary
				EarnProgram     string `json:"earnProgram"`
				TransferRatio   string `json:"transferRatio"`
				EffectivePoints int    `json:"effectivePoints"`
				WithinBalance   bool   `json:"withinBalance"`
			}
			// Initialize as empty slice so JSON output is "[]" not "null" when
			// no reachable offers match.
			rows := []row{}
			for _, sid := range strings.Split(searchIDs, ",") {
				sid = strings.TrimSpace(sid)
				if sid == "" {
					continue
				}
				extra := map[string]string{}
				if cabin != "" {
					extra["cabins"] = cabin
				}
				offers, err := fetchOffers(cmd.Context(), flags, sid, extra)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: search %s failed: %v\n", sid, err)
					continue
				}
				for _, o := range offers {
					if origin != "" && !strings.EqualFold(o.OriginCode, origin) {
						continue
					}
					if month != "" && !strings.HasPrefix(o.DepartsAt, month) {
						continue
					}
					best := row{offerSummary: o.summary(), EffectivePoints: int(^uint(0) >> 1)}
					found := false
					for _, to := range o.Source.RedeemProgram.TransferOptions {
						have, ok := balances[normalizeProgramKey(to.EarnProgram.Identifier, to.EarnProgram.Name)]
						if !ok {
							continue
						}
						ratio, _ := strconv.ParseFloat(to.TotalTransferRatio, 64)
						if ratio <= 0 {
							continue
						}
						eff := int(float64(o.PricePoints) / ratio)
						if eff > have {
							continue
						}
						// PATCH: Pick the cheapest affordable transfer, not the cheapest
						// transfer overall, so alternate balances can still make an offer reachable.
						if eff < best.EffectivePoints {
							best = row{
								offerSummary:    o.summary(),
								EarnProgram:     to.EarnProgram.Name,
								TransferRatio:   to.TotalTransferRatio,
								EffectivePoints: eff,
								WithinBalance:   true,
							}
							found = true
						}
					}
					if found {
						rows = append(rows, best)
					}
				}
			}
			sort.SliceStable(rows, func(i, j int) bool {
				return rows[i].EffectivePoints < rows[j].EffectivePoints
			})
			if limit > 0 && len(rows) > limit {
				rows = rows[:limit]
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No reachable offers within those balances.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "ROUTE\tDEPART\tCABIN\tEARN\tRATIO\tCOST_POINTS\tDEST_POINTS")
			for _, r := range rows {
				route := r.OriginCode + "→" + r.DestinationCode
				fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%d\t%d\n",
					route, shortDate(r.DepartsAt), r.CabinClass, r.EarnProgram, r.TransferRatio, r.EffectivePoints, r.PricePoints)
			}
			_ = tw.Flush()
			return nil
		},
	}
	cmd.Flags().StringVar(&balanceStr, "balance", "", "Comma-separated earn-program:points map. Required. Example: ur:250000,mr:80000,bilt:120000")
	cmd.Flags().StringVar(&searchIDs, "search-ids", "", "Comma-separated Pointhound search session ids (ofs_*). Required.")
	cmd.Flags().StringVar(&cabin, "cabin", "", "Limit to one cabin class (economy, premium_economy, business, first).")
	cmd.Flags().StringVar(&month, "month", "", "Limit to a YYYY-MM month (e.g. 2026-10).")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum reachable offers to return.")
	return cmd
}

func parseBalanceMap(s string) (map[string]int, error) {
	out := map[string]int{}
	for _, pair := range strings.Split(s, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid balance pair %q (expected program:points)", pair)
		}
		key := normalizeProgramKey(parts[0], parts[0])
		n, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return nil, fmt.Errorf("invalid points count in %q: %w", pair, err)
		}
		out[key] = n
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("balance is empty")
	}
	return out, nil
}

// normalizeProgramKey maps program aliases to canonical identifiers used in
// Pointhound's transferOptions data. Accepts shortcodes (ur, mr, bilt, c1, ty)
// and identifier slugs (chase-ultimate-rewards, amex-membership-rewards, etc.).
func normalizeProgramKey(identifier, name string) string {
	id := strings.ToLower(strings.TrimSpace(identifier))
	name = strings.ToLower(strings.TrimSpace(name))
	aliases := map[string]string{
		"ur":                      "chase-ultimate-rewards",
		"chase":                   "chase-ultimate-rewards",
		"chase-ultimate-rewards":  "chase-ultimate-rewards",
		"mr":                      "amex-membership-rewards",
		"amex":                    "amex-membership-rewards",
		"amex-membership-rewards": "amex-membership-rewards",
		"membership-rewards":      "amex-membership-rewards",
		"bilt":                    "bilt-rewards",
		"bilt-rewards":            "bilt-rewards",
		"c1":                      "capital-one-rewards",
		"capone":                  "capital-one-rewards",
		"capital-one":             "capital-one-rewards",
		"capital-one-rewards":     "capital-one-rewards",
		"capital-one-miles":       "capital-one-rewards",
		"ty":                      "citi-thankyou-points",
		"citi":                    "citi-thankyou-points",
		"citi-thankyou-points":    "citi-thankyou-points",
		"thankyou":                "citi-thankyou-points",
		"marriott":                "marriott-bonvoy",
		"marriott-bonvoy":         "marriott-bonvoy",
		"bonvoy":                  "marriott-bonvoy",
		"rove":                    "rove-miles",
		"rove-miles":              "rove-miles",
		"hyatt":                   "world-of-hyatt-loyalty-program",
		"world-of-hyatt":          "world-of-hyatt-loyalty-program",
	}
	if v, ok := aliases[id]; ok {
		return v
	}
	if v, ok := aliases[name]; ok {
		return v
	}
	return id
}

// ---------- watch / drift ----------

type watchRecord struct {
	ID               string        `json:"id"`
	Origin           string        `json:"origin"`
	Destination      string        `json:"destination"`
	Date             string        `json:"date"`
	Cabin            string        `json:"cabin"`
	Passengers       int           `json:"passengers"`
	SearchID         string        `json:"searchId"`
	CreatedAt        string        `json:"createdAt"`
	PreviousSnapshot *snapshotData `json:"previousSnapshot,omitempty"`
	LastSnapshot     snapshotData  `json:"lastSnapshot"`
	LastRunAt        string        `json:"lastRunAt"`
	LastExitCode     int           `json:"lastExitCode"`
}

type snapshotData struct {
	CapturedAt string         `json:"capturedAt"`
	Offers     []offerSummary `json:"offers"`
}

func newWatchCmd(flags *rootFlags) *cobra.Command {
	var searchID string
	var origin, dest, date, cabin string
	var passengers int

	cmd := &cobra.Command{
		Use:   "watch <origin> <dest> <date>",
		Short: "Poll a saved route and exit 2 when a new or cheaper deal appears",
		Long: strings.TrimSpace(`
Register or refresh a saved route. Each run fetches /api/offers for the given
searchId, compares the result to the last snapshot stored locally, and exits
with code 0 (no change), 2 (new/cheaper deal exists), or non-zero on error.

The first run records a baseline and exits 0. Provide --search-id from an
existing Pointhound search URL (or from 'top-deals-matrix' once cookies are
imported).

Designed for cron:
    pointhound-pp-cli watch SFO HND 2026-12-22 --search-id ofs_xxx --quiet && notify "new HND deal"
`),
		Example: strings.Trim(`
  pointhound-pp-cli watch SFO HND 2026-12-22 --search-id ofs_xxx --cabin business
  pointhound-pp-cli watch SFO LIS 2026-06-15 --search-id ofs_xxx --quiet
`, "\n"),
		Annotations: map[string]string{
			"pp:typed-exit-codes": "0,2",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && searchID == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 3 {
				return usageErr(fmt.Errorf("expected <origin> <dest> <date>, got %d args", len(args)))
			}
			if searchID == "" {
				return usageErr(fmt.Errorf("required flag --search-id not set"))
			}
			origin, dest, date = strings.ToUpper(args[0]), strings.ToUpper(args[1]), args[2]
			offers, err := fetchOffers(cmd.Context(), flags, searchID, map[string]string{
				"cabins":     emptyDefault(cabin, "economy"),
				"passengers": fmt.Sprintf("%d", passengers),
			})
			if err != nil {
				return err
			}
			snap := snapshotData{CapturedAt: time.Now().UTC().Format(time.RFC3339)}
			for _, o := range offers {
				snap.Offers = append(snap.Offers, o.summary())
			}
			watchID := fmt.Sprintf("%s|%s|%s|%s", origin, dest, date, cabin)
			db, err := openWatchStore(cmd.Context(), flags)
			if err != nil {
				return err
			}
			defer db.Close()
			prev, _ := db.Get("watch", watchID)
			var prior watchRecord
			if prev != nil {
				_ = json.Unmarshal(prev, &prior)
			}
			var previousSnapshot *snapshotData
			if prior.LastSnapshot.CapturedAt != "" || len(prior.LastSnapshot.Offers) > 0 {
				previous := prior.LastSnapshot
				previousSnapshot = &previous
			}
			rec := watchRecord{
				ID:               watchID,
				Origin:           origin,
				Destination:      dest,
				Date:             date,
				Cabin:            cabin,
				Passengers:       passengers,
				SearchID:         searchID,
				CreatedAt:        firstNonEmpty(prior.CreatedAt, snap.CapturedAt),
				PreviousSnapshot: previousSnapshot,
				LastSnapshot:     snap,
				LastRunAt:        snap.CapturedAt,
			}
			diff := computeDiff(prior.LastSnapshot.Offers, snap.Offers)
			out := map[string]any{
				"watchId":    watchID,
				"capturedAt": snap.CapturedAt,
				"diff":       diff,
				"isBaseline": prev == nil,
			}
			if prev == nil {
				rec.LastExitCode = 0
			} else if diff.HasChanges() {
				rec.LastExitCode = 2
			} else {
				rec.LastExitCode = 0
			}
			payload, _ := json.Marshal(rec)
			if err := db.Upsert("watch", watchID, payload); err != nil {
				return fmt.Errorf("recording watch: %w", err)
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				if err := printJSONFiltered(cmd.OutOrStdout(), out, flags); err != nil {
					return err
				}
			} else {
				if prev == nil {
					fmt.Fprintf(cmd.OutOrStdout(), "baseline recorded for %s→%s on %s (%d offers)\n", origin, dest, date, len(snap.Offers))
				} else if diff.HasChanges() {
					fmt.Fprintf(cmd.OutOrStdout(), "CHANGE on %s→%s %s: %d new, %d cheaper, %d disappeared\n",
						origin, dest, date, len(diff.New), len(diff.Cheaper), len(diff.Disappeared))
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "no change on %s→%s %s (%d offers)\n", origin, dest, date, len(snap.Offers))
				}
			}
			if rec.LastExitCode == 2 {
				flags.exitCode = 2
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&searchID, "search-id", "", "Pointhound search session id (ofs_*). Required.")
	cmd.Flags().StringVar(&cabin, "cabin", "economy", "Cabin class to watch (economy, premium_economy, business, first).")
	cmd.Flags().IntVar(&passengers, "passengers", 1, "Passenger count.")
	return cmd
}

type offerDiff struct {
	New         []offerSummary `json:"new,omitempty"`
	Cheaper     []offerSummary `json:"cheaper,omitempty"`
	Disappeared []offerSummary `json:"disappeared,omitempty"`
	Unchanged   int            `json:"unchanged"`
}

func (d offerDiff) HasChanges() bool {
	return len(d.New) > 0 || len(d.Cheaper) > 0 || len(d.Disappeared) > 0
}

func computeDiff(prev, curr []offerSummary) offerDiff {
	prevMap := map[string]offerSummary{}
	for _, o := range prev {
		prevMap[o.ID] = o
	}
	currMap := map[string]offerSummary{}
	for _, o := range curr {
		currMap[o.ID] = o
	}
	var d offerDiff
	for _, o := range curr {
		p, ok := prevMap[o.ID]
		if !ok {
			d.New = append(d.New, o)
			continue
		}
		if o.PricePoints < p.PricePoints {
			d.Cheaper = append(d.Cheaper, o)
			continue
		}
		d.Unchanged++
	}
	for _, o := range prev {
		if _, ok := currMap[o.ID]; !ok {
			d.Disappeared = append(d.Disappeared, o)
		}
	}
	return d
}

func newDriftCmd(flags *rootFlags) *cobra.Command {
	var origin, dest, date, cabin string
	var since string

	cmd := &cobra.Command{
		Use:   "drift <origin> <dest> <date>",
		Short: "Show the per-offer delta for a watched route since the last snapshot",
		Long: strings.TrimSpace(`
Report which offers are new, cheaper, disappeared, or unchanged for a watched
route since its previous snapshot. Requires that 'watch' has been run at least
twice on this route.
`),
		Example: strings.Trim(`
  pointhound-pp-cli drift SFO HND 2026-12-22 --cabin business --json
  pointhound-pp-cli drift SFO LIS 2026-06-15 --since "1 hour ago"
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			_ = since // documented but not used for v1 — last snapshot only
			if dryRunOK(flags) {
				return nil
			}
			if len(args) < 3 {
				return usageErr(fmt.Errorf("expected <origin> <dest> <date>, got %d args", len(args)))
			}
			origin, dest, date = strings.ToUpper(args[0]), strings.ToUpper(args[1]), args[2]
			watchID := fmt.Sprintf("%s|%s|%s|%s", origin, dest, date, cabin)
			db, err := openWatchStore(cmd.Context(), flags)
			if err != nil {
				return err
			}
			defer db.Close()
			raw, err := db.Get("watch", watchID)
			if err != nil {
				return fmt.Errorf("loading watch %q: %w", watchID, err)
			}
			if raw == nil {
				return fmt.Errorf("no watch found for %s→%s %s (run `pointhound-pp-cli watch ...` first)", origin, dest, date)
			}
			var rec watchRecord
			if err := json.Unmarshal(raw, &rec); err != nil {
				return err
			}
			if rec.PreviousSnapshot == nil {
				return fmt.Errorf("watch %s has no previous snapshot yet (run `pointhound-pp-cli watch ...` at least twice)", watchID)
			}
			diff := computeDiff(rec.PreviousSnapshot.Offers, rec.LastSnapshot.Offers)
			out := map[string]any{
				"watchId":            watchID,
				"previousCapturedAt": rec.PreviousSnapshot.CapturedAt,
				"lastRunAt":          rec.LastRunAt,
				"diff":               diff,
				"offerCount":         len(rec.LastSnapshot.Offers),
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "watch %s drift from %s to %s: %d new, %d cheaper, %d disappeared, %d unchanged.\n",
				watchID, rec.PreviousSnapshot.CapturedAt, rec.LastRunAt, len(diff.New), len(diff.Cheaper), len(diff.Disappeared), diff.Unchanged)
			return nil
		},
	}
	cmd.Flags().StringVar(&cabin, "cabin", "economy", "Cabin class on the watched route.")
	cmd.Flags().StringVar(&since, "since", "", "Filter snapshots to those captured after this time (e.g. '7 days ago').")
	return cmd
}

// ---------- batch ----------

func newBatchCmd(flags *rootFlags) *cobra.Command {
	var searchIDsFile string
	var searchIDs string
	var cabin string
	var passengers int
	var throttle time.Duration
	var concurrency int

	cmd := &cobra.Command{
		Use:   "batch",
		Short: "Fan-out fetch /api/offers across N existing search sessions",
		Long: strings.TrimSpace(`
Run /api/offers against a list of existing search sessions (one per line in a
file, or comma-separated via --search-ids), recording results to the local
store. Designed to populate the store for cross-route queries (from-home,
compare-transfer, calendar).
`),
		Example: strings.Trim(`
  pointhound-pp-cli batch --search-ids ofs_aaa,ofs_bbb,ofs_ccc --cabin business
  pointhound-pp-cli batch --search-ids-file ./searches.txt --throttle 2s --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			ids, err := readSearchIDs(searchIDs, searchIDsFile)
			if err != nil {
				return err
			}
			if len(ids) == 0 {
				return cmd.Help()
			}
			type result struct {
				SearchID   string `json:"searchId"`
				OfferCount int    `json:"offerCount"`
				Stored     int    `json:"stored"`
				MinPoints  int    `json:"minPoints"`
				Error      string `json:"error,omitempty"`
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("pointhound-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening local store: %w", err)
			}
			defer db.Close()
			limiter := newBatchThrottle(throttle)
			// PATCH: Use bounded fan-out so batch matches its documented parallel behavior.
			fanoutRows, fanoutErrs := cliutil.FanoutRun(cmd.Context(), ids,
				func(sid string) string { return sid },
				func(ctx context.Context, sid string) (result, error) {
					if err := limiter.Wait(ctx); err != nil {
						return result{SearchID: sid, Error: err.Error()}, nil
					}
					extra := map[string]string{
						"cabins":     emptyDefault(cabin, "economy"),
						"passengers": fmt.Sprintf("%d", passengers),
					}
					offers, err := fetchOffers(ctx, flags, sid, extra)
					r := result{SearchID: sid, OfferCount: len(offers)}
					if err != nil {
						r.Error = err.Error()
					} else {
						best := int(^uint(0) >> 1)
						for _, o := range offers {
							if o.PricePoints > 0 && o.PricePoints < best {
								best = o.PricePoints
							}
						}
						if best == int(^uint(0)>>1) {
							best = 0
						}
						r.MinPoints = best
						stored, err := persistOffers(db, offers)
						r.Stored = stored
						if err != nil {
							r.Error = err.Error()
						}
					}
					return r, nil
				},
				cliutil.WithConcurrency(concurrency),
			)
			cliutil.FanoutReportErrors(os.Stderr, fanoutErrs)
			// Initialize as empty slice so JSON output is "[]" not "null" on
			// dry-run or when every search session returns zero offers.
			rows := []result{}
			for _, r := range fanoutRows {
				rows = append(rows, r.Value)
			}
			for _, err := range fanoutErrs {
				rows = append(rows, result{SearchID: err.Source, Error: err.Err.Error()})
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "SEARCH_ID\tOFFERS\tSTORED\tMIN_POINTS\tERROR")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%s\n", r.SearchID, r.OfferCount, r.Stored, r.MinPoints, r.Error)
			}
			_ = tw.Flush()
			return nil
		},
	}
	cmd.Flags().StringVar(&searchIDsFile, "search-ids-file", "", "Path to a newline-separated list of ofs_* search ids.")
	cmd.Flags().StringVar(&searchIDs, "search-ids", "", "Comma-separated list of ofs_* search ids.")
	cmd.Flags().StringVar(&cabin, "cabin", "", "Cabin class filter for all searches.")
	cmd.Flags().IntVar(&passengers, "passengers", 1, "Passenger count for all searches.")
	cmd.Flags().IntVar(&concurrency, "concurrency", 4, "Maximum concurrent offer fetches.")
	cmd.Flags().DurationVar(&throttle, "throttle", time.Second, "Wait between requests to avoid rate limits.")
	return cmd
}

type batchThrottle struct {
	interval time.Duration
	mu       sync.Mutex
	next     time.Time
}

func newBatchThrottle(interval time.Duration) *batchThrottle {
	return &batchThrottle{interval: interval}
}

func (t *batchThrottle) Wait(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if t == nil || t.interval <= 0 {
		return nil
	}

	t.mu.Lock()
	now := time.Now()
	start := now
	if t.next.After(now) {
		start = t.next
	}
	t.next = start.Add(t.interval)
	t.mu.Unlock()

	if delay := time.Until(start); delay > 0 {
		return sleepContext(ctx, delay)
	}
	return ctx.Err()
}

func sleepContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// ---------- calendar ----------

func newCalendarCmd(flags *rootFlags) *cobra.Command {
	var searchIDs string
	var cabin string

	cmd := &cobra.Command{
		Use:   "calendar",
		Short: "Best month for a route across multiple search sessions (groupby month -> min points)",
		Long: strings.TrimSpace(`
For one or more existing search sessions covering the same route at different
dates, aggregate by year-month and surface the cheapest deal in each month.

Run one search per month-end-of-interest first (or use top-deals-matrix with
authenticated cookies), then point calendar at the resulting search ids.
`),
		Example: strings.Trim(`
  pointhound-pp-cli calendar --search-ids ofs_a,ofs_b,ofs_c --cabin business --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if searchIDs == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			type cell struct {
				Month       string `json:"month"`
				MinPoints   int    `json:"minPoints"`
				BestRoute   string `json:"bestRoute"`
				OfferID     string `json:"offerId"`
				Airlines    string `json:"airlines"`
				SearchID    string `json:"searchId"`
				DepartsAt   string `json:"departsAt"`
				CabinClass  string `json:"cabinClass"`
				TotalOffers int    `json:"totalOffersInMonth"`
			}
			byMonth := map[string]*cell{}
			for _, sid := range strings.Split(searchIDs, ",") {
				sid = strings.TrimSpace(sid)
				if sid == "" {
					continue
				}
				extra := map[string]string{}
				if cabin != "" {
					extra["cabins"] = cabin
				}
				offers, err := fetchOffers(cmd.Context(), flags, sid, extra)
				if err != nil {
					fmt.Fprintf(os.Stderr, "warning: %s: %v\n", sid, err)
					continue
				}
				for _, o := range offers {
					month := o.DepartsAt
					if len(month) >= 7 {
						month = month[:7]
					}
					c, ok := byMonth[month]
					if !ok {
						c = &cell{Month: month, MinPoints: int(^uint(0) >> 1)}
						byMonth[month] = c
					}
					c.TotalOffers++
					if o.PricePoints > 0 && o.PricePoints < c.MinPoints {
						c.MinPoints = o.PricePoints
						c.BestRoute = o.OriginCode + "→" + o.DestinationCode
						c.OfferID = o.ID
						c.Airlines = o.AirlinesList
						c.SearchID = sid
						c.DepartsAt = o.DepartsAt
						c.CabinClass = o.CabinClass
					}
				}
			}
			rows := make([]cell, 0, len(byMonth))
			for _, c := range byMonth {
				if c.MinPoints == int(^uint(0)>>1) {
					c.MinPoints = 0
				}
				rows = append(rows, *c)
			}
			sort.SliceStable(rows, func(i, j int) bool { return rows[i].Month < rows[j].Month })
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				if len(rows) == 0 {
					// Distinguish "feature works, no data" from "broken inputs":
					// emit an explicit empty-result envelope with a hint instead
					// of bare null.
					type empty struct {
						Months  []cell `json:"months"`
						Warning string `json:"warning,omitempty"`
					}
					return printJSONFiltered(cmd.OutOrStdout(), empty{
						Months:  []cell{},
						Warning: "no offers found for the supplied search-ids; verify ids with `searches list`",
					}, flags)
				}
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			if len(rows) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No offers across the supplied searches.")
				return nil
			}
			tw := newTabWriter(cmd.OutOrStdout())
			fmt.Fprintln(tw, "MONTH\tBEST_POINTS\tROUTE\tAIRLINES\tOFFERS_TOTAL")
			for _, r := range rows {
				fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%d\n", r.Month, r.MinPoints, r.BestRoute, r.Airlines, r.TotalOffers)
			}
			_ = tw.Flush()
			return nil
		},
	}
	cmd.Flags().StringVar(&searchIDs, "search-ids", "", "Comma-separated list of ofs_* search ids (typically one per month).")
	cmd.Flags().StringVar(&cabin, "cabin", "", "Cabin class to filter (economy, business, first).")
	return cmd
}

// ---------- top-deals-matrix ----------

func newTopDealsMatrixCmd(flags *rootFlags) *cobra.Command {
	var origins, dests, months, cabin string

	cmd := &cobra.Command{
		Use:   "top-deals-matrix",
		Short: "Run a multi-origin × multi-destination × month-range matrix search",
		Long: strings.TrimSpace(`
Pointhound's flagship Top Deals product takes up to 6 origins × 6 destinations
× a year-range and returns the best deals across the matrix. This command runs
one representative date per cell against Pointhound's /flights search-create
endpoint, extracts the resulting ofs_* search id, and reports the matrix status.

The search-create endpoint is Cloudflare-gated; run 'pointhound-pp-cli auth
login --chrome' first so the CLI can replay cf_clearance + ph_session cookies.
`),
		Example: strings.Trim(`
  pointhound-pp-cli top-deals-matrix --origins SFO,LAX --dests LIS,FCO,LHR --months 2026-10,2026-11,2026-12 --cabin business
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if origins == "" || dests == "" || months == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			cabinCode, err := protobuf.CabinForString(cabin)
			if err != nil {
				return usageErr(err)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			cookieHeader := ""
			if c.Config != nil {
				cookieHeader = c.Config.AuthHeader()
			}
			// Pointhound uses Cloudflare clearance + Cloudflare bot-management cookies
			// (cf_clearance, __cf_bm). There is no separate ph_session cookie —
			// app-layer auth lives in localStorage (Supabase JWT) and isn't required
			// for the public search-create endpoint.
			cookiesConfigured := strings.Contains(cookieHeader, "cf_clearance=")
			splitCSV := func(s string) []string {
				var out []string
				for _, p := range strings.Split(s, ",") {
					if p = strings.TrimSpace(p); p != "" {
						out = append(out, p)
					}
				}
				return out
			}
			origs := splitCSV(origins)
			ds := splitCSV(dests)
			ms := splitCSV(months)
			rows := []topDealsMatrixCell{}
			successful := 0
			failed := 0
			cellIndex := 0
			for _, o := range origs {
				for _, d := range ds {
					for _, m := range ms {
						if cellIndex > 0 && cookiesConfigured {
							if err := sleepContext(cmd.Context(), time.Second); err != nil {
								return err
							}
						}
						cellIndex++
						cell := runTopDealsMatrixCell(cmd.Context(), c, strings.ToUpper(o), strings.ToUpper(d), m, cabin, cabinCode, cookiesConfigured)
						if cell.Status == "ok" {
							successful++
						} else {
							failed++
						}
						rows = append(rows, cell)
					}
				}
			}
			out := map[string]any{
				"cells":            rows,
				"cellCount":        len(rows),
				"executable":       true,
				"successful_cells": successful,
				"failed_cells":     failed,
				"note":             "Pipe searchIds to `pointhound-pp-cli deals --search-id ofs_xxx` for the full deal report.",
			}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			for _, row := range rows {
				if row.Status == "ok" {
					fmt.Fprintf(cmd.OutOrStdout(), "%s → %s %s (%s): searchId=%s\n", row.Origin, row.Dest, row.Month, row.DateUsed, row.SearchID)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "%s → %s %s (%s): error=%s\n", row.Origin, row.Dest, row.Month, row.DateUsed, row.Error)
				}
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Pipe searchIds to `pointhound-pp-cli deals --search-id ofs_xxx` for the full deal report")
			return nil
		},
	}
	cmd.Flags().StringVar(&origins, "origins", "", "Comma-separated origin IATA codes (up to 6).")
	cmd.Flags().StringVar(&dests, "dests", "", "Comma-separated destination IATA codes (up to 6).")
	cmd.Flags().StringVar(&months, "months", "", "Comma-separated year-month values (e.g. 2026-10,2026-11).")
	cmd.Flags().StringVar(&cabin, "cabin", "economy", "Cabin class for the matrix.")
	return cmd
}

var topDealsSearchIDPattern = regexp.MustCompile(`ofs_[A-Za-z0-9]+`)

type topDealsMatrixCell struct {
	Origin   string `json:"origin"`
	Dest     string `json:"dest"`
	Month    string `json:"month"`
	DateUsed string `json:"date_used"`
	Cabin    string `json:"cabin"`
	URL      string `json:"would_post"`
	SearchID string `json:"searchId"`
	Status   string `json:"status"`
	Error    string `json:"error,omitempty"`
	Note     string `json:"note,omitempty"`
}

func runTopDealsMatrixCell(ctx context.Context, c *client.Client, origin, dest, month, cabin string, cabinCode int, cookiesConfigured bool) topDealsMatrixCell {
	cell := topDealsMatrixCell{
		Origin: strings.ToUpper(strings.TrimSpace(origin)),
		Dest:   strings.ToUpper(strings.TrimSpace(dest)),
		Month:  strings.TrimSpace(month),
		Cabin:  cabin,
	}
	dateUsed, err := topDealsRepresentativeDate(cell.Month)
	if err != nil {
		cell.Status = "error"
		cell.Error = err.Error()
		return cell
	}
	cell.DateUsed = dateUsed
	q, err := protobuf.SearchQuery{
		OriginCode:      cell.Origin,
		DestinationCode: cell.Dest,
		Date:            dateUsed,
		Cabin:           cabinCode,
		Passengers:      1,
	}.Encode()
	if err != nil {
		cell.Status = "error"
		cell.Error = err.Error()
		return cell
	}
	params := url.Values{}
	params.Set("q", q)
	path := "/flights?" + params.Encode()
	cell.URL = c.BaseURL + path
	if !cookiesConfigured {
		cell.Status = "error"
		cell.Error = "cookies missing — run auth login --chrome"
		cell.Note = "search-create requires `auth login --chrome` to import cf_clearance + ph_session"
		return cell
	}
	cookieHeader := ""
	if c.Config != nil {
		cookieHeader = c.Config.AuthHeader()
	}
	// PATCH: Propagate Cobra's command context into the Cloudflare-gated POST path.
	body, status, err := postFlightsViaSurf(ctx, cell.URL, cookieHeader)
	if err != nil {
		cell.Status = "error"
		cell.Error = fmt.Sprintf("surf POST failed: %v", err)
		cell.Note = "search-create transport error"
		return cell
	}
	if status == http.StatusForbidden {
		cell.Status = "error"
		cell.Error = "cookies rejected by Cloudflare — refresh with `pointhound-pp-cli auth login --chrome`"
		cell.Note = "stale cf_clearance"
		return cell
	}
	if status >= 400 {
		cell.Status = "error"
		cell.Error = fmt.Sprintf("HTTP %d: %s", status, string(body[:min(200, len(body))]))
		cell.Note = "search-create request failed"
		return cell
	}
	match := topDealsSearchIDPattern.FindString(string(body))
	if match == "" {
		cell.Status = "error"
		cell.Error = "search id not found in Pointhound response"
		cell.Note = "response did not include an ofs_* id"
		return cell
	}
	cell.SearchID = match
	cell.Status = "ok"
	cell.Note = "search-create executed"
	return cell
}

// postFlightsViaSurf POSTs the search-create URL using Chrome-impersonated
// TLS so Cloudflare's bot defense accepts the request. The stdlib HTTP
// client used everywhere else in this CLI gets 403 here because its TLS
// handshake doesn't match Chrome's JA3/JA4 fingerprint.
//
// cookieHeader is the full "Cookie: name=value; name=value" header value
// the configured cookie auth would normally send (read from c.Config.AuthHeader()
// upstream). Pass it through so the cf_clearance + Cloudflare bot-management
// cookies imported via `auth login --chrome` are included.
//
// Returns (responseBody, status, error). Status is 0 on transport error.
func postFlightsViaSurf(ctx context.Context, urlStr, cookieHeader string) ([]byte, int, error) {
	client, err := surf.NewClient().
		Builder().
		Impersonate().
		Chrome().
		Timeout(30 * time.Second).
		Build().
		Result()
	if err != nil {
		return nil, 0, fmt.Errorf("building surf client: %w", err)
	}
	req := client.Post(g.String(urlStr)).WithContext(ctx)
	if cookieHeader != "" {
		req = req.AddHeaders("Cookie", cookieHeader)
	}
	// Cloudflare's bot challenge accepts the cf_clearance cookie only when
	// the request looks like a real same-origin form submit from the /search
	// page. Setting Sec-Fetch-* and Referer/Origin matches what Chrome sends.
	req = req.
		AddHeaders("Referer", "https://www.pointhound.com/search").
		AddHeaders("Origin", "https://www.pointhound.com").
		AddHeaders("Sec-Fetch-Site", "same-origin").
		AddHeaders("Sec-Fetch-Mode", "navigate").
		AddHeaders("Sec-Fetch-Dest", "document").
		AddHeaders("Sec-Fetch-User", "?1").
		AddHeaders("Upgrade-Insecure-Requests", "1").
		AddHeaders("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8").
		AddHeaders("Accept-Language", "en-US,en;q=0.9")
	resp, err := req.Do().Result()
	if err != nil {
		return nil, 0, fmt.Errorf("surf POST: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body.Reader)
	if err != nil {
		return nil, int(resp.StatusCode), fmt.Errorf("reading surf response: %w", err)
	}
	return body, int(resp.StatusCode), nil
}

func topDealsRepresentativeDate(month string) (string, error) {
	t, err := time.Parse("2006-01", strings.TrimSpace(month))
	if err != nil {
		return "", fmt.Errorf("invalid month %q (expected YYYY-MM)", month)
	}
	return time.Date(t.Year(), t.Month(), 15, 0, 0, 0, 0, time.UTC).Format("2006-01-02"), nil
}

func topDealsRequestError(status int, err error) string {
	switch status {
	case 401, 403:
		return "cookies rejected — run auth login --chrome"
	case 429:
		return "rate limited by Pointhound"
	}
	if err != nil {
		return err.Error()
	}
	return fmt.Sprintf("HTTP %d", status)
}

// ---------- helpers ----------

func openWatchStore(ctx context.Context, _ *rootFlags) (*store.Store, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".pointhound-pp-cli")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	s, err := store.OpenWithContext(ctx, filepath.Join(dir, "store.db"))
	if err != nil {
		return nil, fmt.Errorf("opening store: %w", err)
	}
	return s, nil
}

func readSearchIDs(csv, file string) ([]string, error) {
	var ids []string
	if csv != "" {
		for _, p := range strings.Split(csv, ",") {
			if s := strings.TrimSpace(p); s != "" {
				ids = append(ids, s)
			}
		}
	}
	if file != "" {
		path, err := expandHome(file)
		if err != nil {
			return nil, err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Split(string(b), "\n") {
			if s := strings.TrimSpace(line); s != "" && !strings.HasPrefix(s, "#") {
				ids = append(ids, s)
			}
		}
	}
	return ids, nil
}

// expandHome expands a leading ~ in a file path so canonical examples
// like ~/routes.txt work without manual expansion by the caller.
func expandHome(p string) (string, error) {
	if p == "" || (p[0] != '~') {
		return p, nil
	}
	if p == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return home, nil
	}
	if strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return home + p[1:], nil
	}
	return p, nil
}

func shortDate(s string) string {
	if len(s) < 10 {
		return s
	}
	return s[:10]
}

func emptyDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
