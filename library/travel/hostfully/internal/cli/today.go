package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"regexp"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hostfully/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/hostfully/internal/config"
	"github.com/mvanhorn/printing-press-library/library/travel/hostfully/internal/store"
	"github.com/spf13/cobra"
)

type reservation struct {
	UID         string
	PropertyUID string
	Unit        int
	Status      string
	CheckIn     time.Time
	CheckOut    time.Time
	Channel     string
	GuestFirst  string
	GuestLast   string
	Phone       string
	CellPhone   string `json:"-"`
	Email       string
	Adults      int
	Children    int
	Pets        int
	// Context-only fields are deliberately omitted from the general reservation
	// JSON used by roster commands. whois exposes them explicitly.
	BookingType                string   `json:"-"`
	Confirmation               string   `json:"-"`
	ArrivalNote                string   `json:"-"`
	PaymentStatus              string   `json:"-"`
	Paid                       *bool    `json:"-"`
	TotalAmount                string   `json:"-"`
	AmountPaid                 string   `json:"-"`
	Balance                    string   `json:"-"`
	Currency                   string   `json:"-"`
	PaymentTransactionStatuses []string `json:"-"`
	DoorPIN                    string   `json:"-"`
}

type todaySummary struct {
	Date      string        `json:"date"`
	CheckIns  []reservation `json:"check_ins"`
	CheckOuts []reservation `json:"check_outs"`
	Occupied  []reservation `json:"occupied"`
	Vacant    []int         `json:"vacant"`
}

func parseReservation(data []byte) (reservation, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return reservation{}, err
	}

	r := reservation{PaymentStatus: "unverified"}
	r.UID, _ = raw["uid"].(string)
	r.PropertyUID, _ = raw["propertyUid"].(string)
	r.Unit = unitForProperty(r.PropertyUID)
	r.Status, _ = raw["status"].(string)
	r.Channel, _ = raw["channel"].(string)
	if r.Channel == "" {
		r.Channel, _ = raw["source"].(string)
	}
	r.BookingType, _ = raw["type"].(string)
	r.Confirmation, _ = raw["externalBookingId"].(string)
	noteParts := make([]string, 0, 2)
	for _, key := range []string{"notes", "extraNotes"} {
		if note, ok := raw[key].(string); ok && strings.TrimSpace(note) != "" {
			noteParts = append(noteParts, note)
		}
	}
	notes := strings.Join(noteParts, " ")
	if match := regexp.MustCompile(`(?i)time of arrival:?\s*([^\n]{0,40})`).FindStringSubmatch(notes); len(match) > 1 {
		r.ArrivalNote = strings.TrimSpace(match[1])
	}

	if ci, ok := raw["checkInLocalDateTime"].(string); ok {
		r.CheckIn, _ = time.Parse("2006-01-02T15:04:05", ci)
	}
	if co, ok := raw["checkOutLocalDateTime"].(string); ok {
		r.CheckOut, _ = time.Parse("2006-01-02T15:04:05", co)
	}

	if gi, ok := raw["guestInformation"].(map[string]any); ok {
		r.GuestFirst, _ = gi["firstName"].(string)
		r.GuestLast, _ = gi["lastName"].(string)
		r.Phone, _ = gi["phoneNumber"].(string)
		r.CellPhone, _ = gi["cellPhoneNumber"].(string)
		if r.Phone == "" {
			r.Phone = r.CellPhone
		}
		r.Email, _ = gi["email"].(string)
		if v, ok := gi["adultCount"].(float64); ok {
			r.Adults = int(v)
		}
		if v, ok := gi["childrenCount"].(float64); ok {
			r.Children = int(v)
		}
		if v, ok := gi["petCount"].(float64); ok {
			r.Pets = int(v)
		}
	}
	applyReservationPayment(&r, raw["order"])
	applyReservationDoorPIN(&r, raw["pinCode"])
	return r, nil
}

func applyReservationPayment(r *reservation, rawOrder any) {
	order, ok := rawOrder.(map[string]any)
	if !ok {
		return
	}
	r.TotalAmount = paymentScalar(order["totalAmount"])
	r.AmountPaid = paymentScalar(order["amountPaid"])
	r.Balance = paymentScalar(order["balance"])
	r.Currency = paymentScalar(order["currency"])
	if transactions, ok := order["transactions"].([]any); ok {
		for _, raw := range transactions {
			transaction, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			if status := paymentScalar(transaction["status"]); status != "" {
				r.PaymentTransactionStatuses = append(r.PaymentTransactionStatuses, status)
			}
		}
	}

	total, totalOK := new(big.Rat).SetString(r.TotalAmount)
	amountPaid, paidOK := new(big.Rat).SetString(r.AmountPaid)
	balance, balanceOK := new(big.Rat).SetString(r.Balance)
	if !totalOK || !paidOK || !balanceOK {
		return
	}
	paid := balance.Sign() == 0 && amountPaid.Cmp(total) >= 0
	r.Paid = &paid
	if paid {
		r.PaymentStatus = "paid"
	} else {
		r.PaymentStatus = "balance_due"
	}
}

func applyReservationDoorPIN(r *reservation, rawPinCode any) {
	pinCode, ok := rawPinCode.(map[string]any)
	if !ok {
		return
	}
	r.DoorPIN = paymentScalar(pinCode["pinCode"])
}

func paymentScalar(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case json.Number:
		return v.String()
	default:
		return ""
	}
}

func formatChannel(ch string) string {
	switch ch {
	case "AIRBNB":
		return "Airbnb"
	case "BOOKING_COM":
		return "Booking.com"
	case "DIRECT":
		return "Direct"
	case "VRBO":
		return "VRBO"
	default:
		return ch
	}
}

func formatPhone(phone string) string {
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, phone)
	if len(digits) == 11 && digits[0] == '1' {
		return fmt.Sprintf("+1 (%s) %s-%s", digits[1:4], digits[4:7], digits[7:11])
	}
	if len(digits) == 10 {
		return fmt.Sprintf("(%s) %s-%s", digits[0:3], digits[3:6], digits[6:10])
	}
	return phone
}

func loadReservations(db *store.Store) ([]reservation, error) {
	rows, err := db.DB().Query(`
		SELECT data FROM resources
		WHERE resource_type IN ('leads', 'leads_leads')
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reservations []reservation
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			continue
		}
		r, err := parseReservation(data)
		if err != nil {
			continue
		}
		if !operationalReservation(r) {
			continue
		}
		reservations = append(reservations, r)
	}
	return reservations, nil
}

func operationalReservation(r reservation) bool {
	// Hostfully uses BLOCKED leads for owner/maintenance calendar blocks. They
	// have check-in/out timestamps but no guest and must never appear as an
	// arrival, departure, occupancy, or cleaning turnover. Operational guest
	// stays are BOOKED; fail closed on every other status.
	return strings.EqualFold(strings.TrimSpace(r.Status), "BOOKED")
}

func summarizeReservations(reservations []reservation, day time.Time) todaySummary {
	dateText := day.Format("2006-01-02")
	summary := todaySummary{Date: dateText}
	occupiedUnits := map[int]bool{}
	for _, r := range reservations {
		if !operationalReservation(r) {
			continue
		}
		checkIn := r.CheckIn.Format("2006-01-02")
		checkOut := r.CheckOut.Format("2006-01-02")
		if checkIn == dateText {
			summary.CheckIns = append(summary.CheckIns, r)
		}
		if checkOut == dateText {
			summary.CheckOuts = append(summary.CheckOuts, r)
		}
		if checkIn <= dateText && checkOut > dateText {
			summary.Occupied = append(summary.Occupied, r)
			occupiedUnits[r.Unit] = true
		}
	}
	for _, unit := range unitNumbers() {
		if !occupiedUnits[unit] {
			summary.Vacant = append(summary.Vacant, unit)
		}
	}
	return summary
}

func loadLiveReservationsForDate(flags *rootFlags, day time.Time) ([]reservation, error) {
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
	rest := client.New(cfg, flags.timeout, liveFanoutRateLimit(flags))
	rest.NoCache = true
	graphQLCfg := *cfg
	graphQLCfg.BaseURL = hostfullyGraphQLBaseURL(cfg.BaseURL)
	graphql := client.New(&graphQLCfg, flags.timeout, liveFanoutRateLimit(flags))
	graphql.NoCache = true

	if _, err := requireUnits(); err != nil {
		return nil, err
	}
	units := unitNumbers()
	ctx, cancel := boundCtx(context.Background(), flags)
	defer cancel()
	reservations := make([]reservation, 0)
	for _, unit := range units {
		result, err := fetchHostfullyUnitLeadsForDate(
			bindLeadAPI(ctx, rest), bindLeadAPI(ctx, graphql), cfg.AgencyUID(), mustUnitUID(unit), day,
		)
		if err != nil {
			return nil, fmt.Errorf("unit %d: %w", unit, err)
		}
		for _, raw := range result.Leads {
			parsed, err := parseReservation(raw)
			if err == nil {
				reservations = append(reservations, parsed)
			}
		}
	}
	return reservations, nil
}

func newTodayCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var showCheckins bool
	var showCheckouts bool
	var showVacant bool
	var dateText string

	cmd := &cobra.Command{
		Use:   "today",
		Short: "Today's operations summary: check-ins, check-outs, current guests, vacant units",
		Example: strings.Trim(`
  hostfully-pp-cli today
  hostfully-pp-cli today --checkins
  hostfully-pp-cli today --vacant --json`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "today")
			}

			location, err := propertyLocation()
			if err != nil {
				return err
			}
			day := time.Now().In(location)
			if dateText != "" {
				day, err = time.ParseInLocation("2006-01-02", dateText, location)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --date %q: expected YYYY-MM-DD", dateText))
				}
			}

			var reservations []reservation
			if flags.dataSource != "local" {
				reservations, err = loadLiveReservationsForDate(flags, day)
				if err != nil && flags.dataSource == "live" {
					return fmt.Errorf("loading live reservations for %s: %w", day.Format("2006-01-02"), err)
				}
				if err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: live Hostfully date lookup failed; using local mirror: %v\n", err)
				}
			}
			if flags.dataSource == "local" || err != nil {
				if dbPath == "" {
					dbPath = defaultDBPath("hostfully-pp-cli")
				}
				db, openErr := store.OpenWithContext(cmd.Context(), dbPath)
				if openErr != nil {
					return fmt.Errorf("opening database: %w (run 'hostfully-pp-cli fetch' first)", openErr)
				}
				defer db.Close()
				reservations, err = loadReservations(db)
				if err != nil {
					return fmt.Errorf("loading reservations: %w", err)
				}
			}

			summary := summarizeReservations(reservations, day)

			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
			}

			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)

			if !showCheckouts && !showVacant {
				fmt.Fprintf(cmd.OutOrStdout(), "## Hostfully — %s\n\n", day.Format("Mon Jan 2, 2006"))
			}

			if showCheckins || (!showCheckouts && !showVacant) {
				fmt.Fprintf(w, "### Check-ins Today (%d)\n", len(summary.CheckIns))
				if len(summary.CheckIns) > 0 {
					fmt.Fprintln(w, "Unit\tGuest\tAdults\tChannel\tPhone")
					for _, r := range summary.CheckIns {
						fmt.Fprintf(w, "#%d\t%s %s\t%d\t%s\t%s\n",
							r.Unit, r.GuestFirst, r.GuestLast, r.Adults, formatChannel(r.Channel), formatPhone(r.Phone))
					}
				} else {
					fmt.Fprintln(w, "None")
				}
				fmt.Fprintln(w)
			}

			if showCheckouts || (!showCheckins && !showVacant) {
				fmt.Fprintf(w, "### Check-outs Today (%d)\n", len(summary.CheckOuts))
				if len(summary.CheckOuts) > 0 {
					fmt.Fprintln(w, "Unit\tGuest\tChannel")
					for _, r := range summary.CheckOuts {
						fmt.Fprintf(w, "#%d\t%s %s\t%s\n",
							r.Unit, r.GuestFirst, r.GuestLast, formatChannel(r.Channel))
					}
				} else {
					fmt.Fprintln(w, "None")
				}
				fmt.Fprintln(w)
			}

			if showVacant || (!showCheckins && !showCheckouts) {
				fmt.Fprintf(w, "### Vacant Tonight (%d units)\n", len(summary.Vacant))
				if len(summary.Vacant) > 0 {
					units := make([]string, len(summary.Vacant))
					for i, u := range summary.Vacant {
						units[i] = fmt.Sprintf("#%d", u)
					}
					fmt.Fprintln(w, strings.Join(units, ", "))
				} else {
					fmt.Fprintln(w, "All units occupied")
				}
			}

			return w.Flush()
		},
	}

	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().BoolVar(&showCheckins, "checkins", false, "Show only check-ins")
	cmd.Flags().BoolVar(&showCheckouts, "checkouts", false, "Show only check-outs")
	cmd.Flags().BoolVar(&showVacant, "vacant", false, "Show only vacant units")
	cmd.Flags().StringVar(&dateText, "date", "", "Property-local date to query (YYYY-MM-DD; default: today in the property time zone)")

	return cmd
}
