// pp:data-source auto

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/mvanhorn/printing-press-library/library/travel/hostfully/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/hostfully/internal/config"
	"github.com/mvanhorn/printing-press-library/library/travel/hostfully/internal/store"
	"github.com/spf13/cobra"
)

// whois resolves a phone number (or name fragment) to the guest in Hostfully:
// who they are and every stay on record, most recent first. Built for the
// front-desk question "who is this caller?" — feed it the number straight off
// caller ID or a RingCentral triage line.

type whoisStay struct {
	Name                       string   `json:"name"`
	Phone                      string   `json:"phone"`
	Email                      string   `json:"email,omitempty"`
	Unit                       int      `json:"unit"`
	CheckIn                    string   `json:"check_in"`
	CheckOut                   string   `json:"check_out"`
	Status                     string   `json:"status"`
	Stay                       string   `json:"stay"` // current | upcoming | past
	Channel                    string   `json:"channel"`
	BookingType                string   `json:"booking_type,omitempty"`
	Adults                     int      `json:"adults,omitempty"`
	Children                   int      `json:"children,omitempty"`
	Pets                       int      `json:"pets,omitempty"`
	Confirmation               string   `json:"confirmation,omitempty"`
	ArrivalNote                string   `json:"arrival_note,omitempty"`
	Paid                       *bool    `json:"paid"`
	PaymentStatus              string   `json:"payment_status"`
	TotalAmount                string   `json:"total_amount,omitempty"`
	AmountPaid                 string   `json:"amount_paid,omitempty"`
	Balance                    string   `json:"balance,omitempty"`
	Currency                   string   `json:"currency,omitempty"`
	PaymentTransactionStatuses []string `json:"payment_transaction_statuses,omitempty"`
	DoorPIN                    string   `json:"door_pin,omitempty"`
}

const liveWhoisConcurrency = 4

func newNovelWhoisCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "whois [number-or-name]",
		Short: "Resolve a phone number (or name) to the guest in Hostfully, with their stay history",
		Long: `Looks up who a phone number belongs to across every reservation — current,
upcoming, and past stays. The default reads Hostfully live and falls back to
the local mirror only when live access fails; --data-source live never falls
back. Accepts a phone number in any caller-ID format or a name fragment.`,
		Example: strings.Trim(`
  # Who is this caller?
  hostfully-pp-cli whois +15550100199

  # Agent-shaped
  hostfully-pp-cli whois 5550100199 --json

  # By name
  hostfully-pp-cli whois "Guestman"`, "\n"),
		Annotations: map[string]string{
			// A neutral name fragment keeps the live happy path fast and free of real
			// guest names or phone numbers.
			"pp:happy-args": "number-or-name=zq",
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "whois")
			}
			query := strings.Join(args, " ")
			if !validWhoisQuery(query) {
				return usageErr(fmt.Errorf("whois expects a phone number or a guest name, got %q", query))
			}
			var reservations []reservation
			var err error
			if mockMode() {
				reservations = mockHostfullyReservations(time.Now())
			} else if flags.dataSource != "local" {
				reservations, err = loadLiveWhoisReservations(flags, query)
				if err != nil && flags.dataSource == "live" {
					return fmt.Errorf("loading live reservations: %w", err)
				}
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: live Hostfully lookup failed; using local mirror: %v\n", err)
				}
			}
			if !mockMode() && (flags.dataSource == "local" || err != nil) {
				reservations, err = loadLocalWhoisReservations(cmd, dbPath)
				if err != nil {
					return err
				}
			}

			stays := findWhoisStays(reservations, query, time.Now())

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), stays, flags)
			}

			w := cmd.OutOrStdout()
			if len(stays) == 0 {
				fmt.Fprintf(w, "No guest matching %q in Hostfully.\n", query)
				return nil
			}
			fmt.Fprintf(w, "%s — %d reservation(s) on record\n", stays[0].Name, len(stays))
			if stays[0].Phone != "" || stays[0].Email != "" {
				fmt.Fprintf(w, "%s  %s\n", stays[0].Phone, stays[0].Email)
			}
			fmt.Fprintln(w)
			lastName := stays[0].Name
			for _, s := range stays {
				if s.Name != lastName {
					fmt.Fprintf(w, "\n%s  %s  %s\n", s.Name, s.Phone, s.Email)
					lastName = s.Name
				}
				marker := " "
				switch s.Stay {
				case "current":
					marker = "●"
				case "upcoming":
					marker = "○"
				}
				pin := ""
				if s.DoorPIN != "" {
					pin = " door_pin:" + s.DoorPIN
				}
				fmt.Fprintf(w, "%s Unit %-2d  %s → %s  %-9s %-8s %-12s payment:%s%s\n",
					marker, s.Unit, s.CheckIn, s.CheckOut, s.Stay, s.Status, s.Channel,
					s.PaymentStatus, pin)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func loadLocalWhoisReservations(cmd *cobra.Command, dbPath string) ([]reservation, error) {
	if dbPath == "" {
		dbPath = defaultDBPath("hostfully-pp-cli")
	}
	db, err := store.OpenWithContext(cmd.Context(), dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w (run 'hostfully-pp-cli fetch' first)", err)
	}
	defer db.Close()
	reservations, err := loadWhoisReservations(db)
	if err != nil {
		return nil, fmt.Errorf("loading reservations: %w", err)
	}
	return reservations, nil
}

// loadLiveWhoisReservations reads the complete history directly from
// Hostfully. A caller lookup must not turn an API failure into "unknown guest",
// so any failed unit rejects the whole live result rather than returning a
// convincing partial match set.
func loadLiveWhoisReservations(flags *rootFlags, query string) ([]reservation, error) {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return nil, err
	}
	if cfg.AgencyUID() == "" {
		return nil, fmt.Errorf("HOSTFULLY_AGENCY_ID environment variable is required")
	}
	if cfg.HostfullyApiKey == "" {
		return nil, fmt.Errorf("HOSTFULLY_API_KEY environment variable is required")
	}
	graphQLCfg := *cfg
	graphQLCfg.BaseURL = hostfullyGraphQLBaseURL(cfg.BaseURL)
	graphql := client.New(&graphQLCfg, flags.timeout, liveFanoutRateLimit(flags))
	graphql.NoCache = flags.noCache || flags.dataSource == "live"

	if _, err := requireUnits(); err != nil {
		return nil, err
	}
	ctx, cancel := boundCtx(context.Background(), flags)
	defer cancel()
	indexed, err := fetchWhoisReservationsAcrossUnits(
		bindLeadAPI(ctx, graphql), unitUIDMap(), liveWhoisConcurrency,
	)
	if err != nil {
		return nil, err
	}
	matched := make([]reservation, 0)
	uids := make([]string, 0)
	for _, reservation := range indexed {
		if whoisReservationMatches(reservation, query) {
			matched = append(matched, reservation)
			if reservation.UID != "" {
				uids = append(uids, reservation.UID)
			}
		}
	}
	if len(matched) == 0 {
		return matched, nil
	}
	if len(uids) != len(matched) {
		return nil, fmt.Errorf("Hostfully matched %d leads but %d lacked a UID", len(matched), len(matched)-len(uids))
	}

	// GraphQL is the fast complete-history index. Once the matching UIDs are
	// known, materialize their richer REST context and fetch their payment
	// orders concurrently. Joining by UID avoids loading order history for every
	// lead while preserving authoritative channel context and payment state.
	rest := client.New(cfg, flags.timeout, liveFanoutRateLimit(flags))
	rest.NoCache = graphql.NoCache
	var materialized []reservation
	var payments []reservation
	var materializeErr error
	var paymentErr error
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		materialized, materializeErr = fetchWhoisLeadsByUID(bindLeadAPI(ctx, rest), cfg.AgencyUID(), uids)
	}()
	go func() {
		defer wg.Done()
		payments, paymentErr = fetchWhoisSecureDetailsByUIDs(bindLeadAPI(ctx, graphql), uids)
	}()
	wg.Wait()
	if materializeErr != nil {
		return nil, materializeErr
	}
	if paymentErr != nil {
		return nil, fmt.Errorf("loading payment status: %w", paymentErr)
	}
	paymentByUID := make(map[string]reservation, len(payments))
	for _, payment := range payments {
		paymentByUID[payment.UID] = payment
	}
	for i := range materialized {
		if payment, ok := paymentByUID[materialized[i].UID]; ok {
			mergeReservationPayment(&materialized[i], payment)
		}
	}
	return materialized, nil
}

func mergeReservationPayment(target *reservation, indexed reservation) {
	target.Paid = indexed.Paid
	target.PaymentStatus = indexed.PaymentStatus
	target.TotalAmount = indexed.TotalAmount
	target.AmountPaid = indexed.AmountPaid
	target.Balance = indexed.Balance
	target.Currency = indexed.Currency
	target.PaymentTransactionStatuses = append(
		[]string(nil), indexed.PaymentTransactionStatuses...,
	)
	target.DoorPIN = indexed.DoorPIN
}

func fetchWhoisLeadsByUID(api hostfullyLeadAPI, agencyUID string, uids []string) ([]reservation, error) {
	const batchSize = 100
	reservations := make([]reservation, 0, len(uids))
	wanted := make(map[string]bool, len(uids))
	for _, uid := range uids {
		wanted[uid] = true
	}
	seen := make(map[string]bool, len(uids))
	for start := 0; start < len(uids); start += batchSize {
		end := start + batchSize
		if end > len(uids) {
			end = len(uids)
		}
		data, err := api.GetNoRetry("/leads", map[string]string{
			"agencyUid": agencyUID,
			"leadUids":  strings.Join(uids[start:end], ","),
			"_limit":    fmt.Sprintf("%d", end-start),
		})
		if err != nil {
			return nil, fmt.Errorf("materializing matched Hostfully leads: %w", err)
		}
		leads, err := decodeHostfullyLeads(data)
		if err != nil {
			return nil, fmt.Errorf("materializing matched Hostfully leads: %w", err)
		}
		for _, raw := range leads {
			reservation, err := parseReservation(raw)
			if err != nil || !wanted[reservation.UID] || seen[reservation.UID] {
				continue
			}
			seen[reservation.UID] = true
			reservations = append(reservations, reservation)
		}
	}
	if len(seen) != len(wanted) {
		return nil, fmt.Errorf("Hostfully returned %d of %d matched leads", len(seen), len(wanted))
	}
	return reservations, nil
}

func fetchWhoisSecureDetailsByUIDs(api hostfullyLeadAPI, uids []string) ([]reservation, error) {
	const batchSize = 100
	payments := make([]reservation, 0, len(uids))
	for start := 0; start < len(uids); start += batchSize {
		end := start + batchSize
		if end > len(uids) {
			end = len(uids)
		}
		var selections strings.Builder
		for i, uid := range uids[start:end] {
			fmt.Fprintf(
				&selections,
				"lead%d: lead(uid:%s) { %s }\n",
				i, strconv.Quote(uid), hostfullyGraphQLSecureFields,
			)
		}
		query := "query {\n" + selections.String() + "}"
		data, _, err := api.Post("/graphql", map[string]any{"query": query})
		if err != nil {
			return nil, fmt.Errorf("fetching matched Hostfully payment orders: %w", err)
		}
		var response struct {
			Data   map[string]json.RawMessage `json:"data"`
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		}
		if err := json.Unmarshal(data, &response); err != nil {
			return nil, fmt.Errorf("parsing Hostfully payment response: %w", err)
		}
		if len(response.Errors) > 0 {
			return nil, fmt.Errorf("Hostfully GraphQL payment error: %s", response.Errors[0].Message)
		}
		for i, expectedUID := range uids[start:end] {
			alias := fmt.Sprintf("lead%d", i)
			raw, ok := response.Data[alias]
			if !ok || len(raw) == 0 || string(raw) == "null" {
				return nil, fmt.Errorf("Hostfully payment response omitted lead %s", expectedUID)
			}
			payment, err := parseReservation(raw)
			if err != nil {
				return nil, fmt.Errorf("parsing Hostfully payment for lead %s: %w", expectedUID, err)
			}
			if payment.UID != expectedUID {
				return nil, fmt.Errorf("Hostfully payment response returned lead %s for %s", payment.UID, expectedUID)
			}
			payments = append(payments, payment)
		}
	}
	return payments, nil
}

// fetchWhoisReservationsAcrossUnits keeps the complete per-property history
// but overlaps independent property reads. Hostfully exposes no phone/name
// filter on its lead search, so serially walking nine properties only added
// latency; it did not make the result more targeted or more correct. GraphQL
// is deliberate here: it avoids the known REST serialization poison records
// while returning the complete history and all fields whois renders.
func fetchWhoisReservationsAcrossUnits(
	graphql hostfullyLeadAPI,
	properties map[int]string,
	concurrency int,
) ([]reservation, error) {
	units := make([]int, 0, len(properties))
	for unit := range properties {
		units = append(units, unit)
	}
	sort.Ints(units)
	if len(units) == 0 {
		return []reservation{}, nil
	}
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > len(units) {
		concurrency = len(units)
	}

	type unitResult struct {
		unit  int
		leads []json.RawMessage
		err   error
	}
	jobs := make(chan int, len(units))
	results := make(chan unitResult, len(units))
	for _, unit := range units {
		jobs <- unit
	}
	close(jobs)

	var workers sync.WaitGroup
	workers.Add(concurrency)
	for i := 0; i < concurrency; i++ {
		go func() {
			defer workers.Done()
			for unit := range jobs {
				leads, err := fetchHostfullyGraphQLLeads(graphql, properties[unit])
				results <- unitResult{unit: unit, leads: leads, err: err}
			}
		}()
	}
	workers.Wait()
	close(results)

	byUnit := make(map[int][]json.RawMessage, len(units))
	errorsByUnit := make(map[int]error)
	for result := range results {
		if result.err != nil {
			errorsByUnit[result.unit] = result.err
			continue
		}
		byUnit[result.unit] = result.leads
	}
	for _, unit := range units {
		if err := errorsByUnit[unit]; err != nil {
			return nil, fmt.Errorf("unit %d: %w", unit, err)
		}
	}

	reservations := make([]reservation, 0)
	for _, unit := range units {
		for _, raw := range byUnit[unit] {
			reservation, err := parseReservation(raw)
			if err == nil {
				reservations = append(reservations, reservation)
			}
		}
	}
	return reservations, nil
}

// loadWhoisReservations intentionally does not use loadReservations: the
// operational loader excludes CLOSED/CANCELLED/NEW leads, while caller
// identification needs the complete contact history, regardless of booking
// outcome. Status is returned so the operator can interpret the match.
func loadWhoisReservations(db *store.Store) ([]reservation, error) {
	rows, err := db.DB().Query(`
		SELECT data FROM resources
		WHERE resource_type IN ('leads', 'leads_leads')
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reservations := make([]reservation, 0)
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			continue
		}
		r, err := parseReservation(data)
		if err != nil {
			continue
		}
		reservations = append(reservations, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return reservations, nil
}

func findWhoisStays(reservations []reservation, query string, now time.Time) []whoisStay {
	stays := make([]whoisStay, 0)
	for _, r := range reservations {
		if !whoisReservationMatches(r, query) {
			continue
		}
		stay := "past"
		switch {
		case !r.CheckIn.After(now) && r.CheckOut.After(now):
			stay = "current"
		case r.CheckIn.After(now):
			stay = "upcoming"
		}
		stays = append(stays, whoisStay{
			Name:          strings.TrimSpace(r.GuestFirst + " " + r.GuestLast),
			Phone:         formatPhone(r.Phone),
			Email:         r.Email,
			Unit:          r.Unit,
			CheckIn:       r.CheckIn.Format("2006-01-02"),
			CheckOut:      r.CheckOut.Format("2006-01-02"),
			Status:        r.Status,
			Stay:          stay,
			Channel:       formatChannel(r.Channel),
			BookingType:   r.BookingType,
			Adults:        r.Adults,
			Children:      r.Children,
			Pets:          r.Pets,
			Confirmation:  r.Confirmation,
			ArrivalNote:   r.ArrivalNote,
			Paid:          r.Paid,
			PaymentStatus: r.PaymentStatus,
			TotalAmount:   r.TotalAmount,
			AmountPaid:    r.AmountPaid,
			Balance:       r.Balance,
			Currency:      r.Currency,
			PaymentTransactionStatuses: append(
				[]string(nil), r.PaymentTransactionStatuses...,
			),
			DoorPIN: r.DoorPIN,
		})
	}
	sort.Slice(stays, func(i, j int) bool { return stays[i].CheckIn > stays[j].CheckIn })
	return stays
}

func whoisReservationMatches(r reservation, query string) bool {
	queryDigits := digitsOnly(query)
	if len(queryDigits) >= 7 {
		return phoneDigitsMatch(digitsOnly(r.Phone), queryDigits) ||
			phoneDigitsMatch(digitsOnly(r.CellPhone), queryDigits)
	}
	lq := strings.ToLower(strings.TrimSpace(query))
	name := strings.ToLower(strings.TrimSpace(r.GuestFirst + " " + r.GuestLast))
	return lq != "" && strings.Contains(name, lq)
}

func digitsOnly(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
}

// phoneDigitsMatch compares numbers on their last 10 digits so +1 prefixes
// and formatting differences don't matter.
func phoneDigitsMatch(a, b string) bool {
	trim := func(d string) string {
		if len(d) > 10 {
			return d[len(d)-10:]
		}
		return d
	}
	a, b = trim(a), trim(b)
	return a != "" && a == b
}

// validWhoisQuery accepts a phone number (digits with +, spaces, dots,
// dashes, parentheses) or a name (letters, spaces, apostrophes, hyphens,
// periods).
func validWhoisQuery(q string) bool {
	q = strings.TrimSpace(q)
	if q == "" {
		return false
	}
	for _, r := range q {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
		case strings.ContainsRune(" +-().'", r):
		default:
			return false
		}
	}
	return true
}
