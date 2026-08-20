// Hand-written novel command.
//
//	// pp:client-call
package cli

import (
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type offerFlightSegment struct {
	Order           int    `json:"order"`
	OriginCode      string `json:"originCode"`
	DestinationCode string `json:"destinationCode"`
	AircraftName    string `json:"aircraftName"`
	FlightNumber    string `json:"flightNumber"`
	AirlineCode     string `json:"airlineCode"`
	Airline         struct {
		Name string `json:"name"`
	} `json:"airline"`
}

type offerEarnProgram struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Identifier string `json:"identifier"`
}

type offerTransferOption struct {
	ID                 string           `json:"id"`
	EarnProgramID      string           `json:"earnProgramId"`
	RedeemProgramID    string           `json:"redeemProgramId"`
	TransferRatio      string           `json:"transferRatio"`
	TotalTransferRatio string           `json:"totalTransferRatio"`
	TransferTime       string           `json:"transferTime"`
	BonusTransferRatio string           `json:"bonusTransferRatio"`
	EarnProgram        offerEarnProgram `json:"earnProgram"`
}

type ProgramCost struct {
	ProgramID       string `json:"programId"`
	ProgramName     string `json:"programName"`
	Identifier      string `json:"identifier,omitempty"`
	EffectivePoints int    `json:"effectivePoints"`
	TransferRatio   string `json:"transferRatio"`
	TransferTime    string `json:"transferTime"`
	HasBonus        bool   `json:"hasBonus,omitempty"`
}

type dealView struct {
	ID                 string        `json:"id"`
	FlightNumbers      string        `json:"flightNumbers"`
	Airline            string        `json:"airline"`
	Aircraft           string        `json:"aircraft"`
	DepartsAt          string        `json:"departsAt"`
	ArrivesAt          string        `json:"arrivesAt"`
	TotalDuration      string        `json:"totalDuration"`
	Route              string        `json:"route"`
	TotalStops         int           `json:"totalStops"`
	PricePoints        int           `json:"pricePoints"`
	PriceRetail        float64       `json:"priceRetail"`
	PriceCurrency      string        `json:"priceCurrency"`
	CPP                float64       `json:"cpp"`
	DealClassification string        `json:"dealClassification"`
	QuantityRemaining  int           `json:"quantityRemaining"`
	RedeemProgram      string        `json:"redeemProgram"`
	EarnPrograms       []string      `json:"earnPrograms"`
	ProgramCosts       []ProgramCost `json:"programCosts,omitempty"`
}

func newDealsCmd(flags *rootFlags) *cobra.Command {
	var searchID, cabins, sortBy, airlines, cardPrograms, airlinePrograms string
	var take, passengers int
	var minCPP float64
	var showAllAirlines bool

	cmd := &cobra.Command{
		Use:   "deals",
		Short: "Render a human-friendly Pointhound deals report from /api/offers",
		Long: strings.TrimSpace(`
Render Pointhound offer cards as a readable deals report from the live /api/offers
response for an existing ofs_* search session.

Each deal includes a programCosts array showing how many points you'd need from
each transferable earn program (Chase UR, Amex MR, Bilt, Capital One, etc.),
computed by dividing the offer's destination-program price by the transfer
ratio. Sort is ascending by effective cost, so the cheapest program is always
first.
`),
		Example: strings.Trim(`
  pointhound-pp-cli deals --search-id ofs_xxx
  pointhound-pp-cli deals --search-id ofs_xxx --min-cpp 2.0 --take 20
  pointhound-pp-cli deals --search-id ofs_xxx --card-programs pep_LJ3oxvytYb --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if searchID == "" {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			if !strings.HasPrefix(searchID, "ofs_") {
				return usageErr(fmt.Errorf("required flag --search-id must be an ofs_* Pointhound search id"))
			}
			if take <= 0 || passengers < 1 || passengers > 9 {
				return usageErr(fmt.Errorf("--take must be >0 and --passengers must be 1-9"))
			}
			params := map[string]string{"take": strconv.Itoa(take), "passengers": strconv.Itoa(passengers), "sortBy": sortBy}
			for k, v := range map[string]string{"cabins": cabins, "airlines": airlines, "cardPrograms": cardPrograms, "airlinePrograms": airlinePrograms} {
				if v != "" {
					params[k] = v
				}
			}
			offers, err := fetchOffers(cmd.Context(), flags, searchID, params)
			if err != nil {
				return err
			}
			deals := makeDeals(offers, showAllAirlines, minCPP, !flags.compact)
			out := struct {
				Deals []dealView `json:"deals"`
				Total int        `json:"total"`
			}{deals, len(deals)}
			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !humanFriendly && !flags.compact) {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(deals) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No deals matched the supplied filters.")
			} else if flags.compact {
				printCompactDeals(cmd.OutOrStdout(), deals)
			} else {
				printDealBlocks(cmd.OutOrStdout(), deals)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&searchID, "search-id", "", "Pointhound search session id (ofs_*). Required.")
	cmd.Flags().IntVar(&take, "take", 10, "Maximum number of offers to return.")
	cmd.Flags().StringVar(&cabins, "cabins", "", "Comma-separated cabin classes to include: economy, premium_economy, business, first.")
	cmd.Flags().IntVar(&passengers, "passengers", 1, "Passenger count (1-9).")
	cmd.Flags().StringVar(&sortBy, "sort-by", "points", "Sort field: points, duration, or departsAt.")
	cmd.Flags().StringVar(&airlines, "airlines", "", "Comma-separated airline IDs (aln_* values from filter-options).")
	cmd.Flags().StringVar(&cardPrograms, "card-programs", "", "Comma-separated transferable points program IDs (pep_* values).")
	cmd.Flags().StringVar(&airlinePrograms, "airline-programs", "", "Comma-separated airline frequent flyer program IDs (prp_* values).")
	cmd.Flags().Float64Var(&minCPP, "min-cpp", 0, "Only show deals at or above this cents-per-point value.")
	cmd.Flags().BoolVar(&showAllAirlines, "show-all-airlines", true, "Show the full segment airline chain instead of only the primary airline.")
	return cmd
}

func makeDeals(offers []rawOffer, showAll bool, minCPP float64, includeProgramCosts bool) []dealView {
	deals := make([]dealView, 0, len(offers))
	for _, o := range offers {
		retail := moneyValue(o.PriceRetailTotal)
		cpp := cppValue(retail, o.PricePoints, o.PricePerPoint)
		if minCPP > 0 && cpp < minCPP {
			continue
		}
		deal := dealView{
			ID: o.ID, FlightNumbers: flightNumbers(o), Airline: airlineName(o, showAll), Aircraft: aircraftName(o),
			DepartsAt: formatDealTime(o.DepartsAt), ArrivesAt: formatDealTime(o.ArrivesAt),
			TotalDuration: formatDealDuration(o.TotalDuration), Route: routeString(o), TotalStops: o.TotalStops,
			PricePoints: o.PricePoints, PriceRetail: retail, PriceCurrency: o.PriceCurrency, CPP: cpp,
			DealClassification: dealClass(cpp), QuantityRemaining: o.QuantityRemaining,
			RedeemProgram: firstValue(o.Source.RedeemProgram.Name, o.Source.Name, o.SourceIdentifier),
			EarnPrograms:  earnProgramStrings(o),
		}
		if includeProgramCosts {
			deal.ProgramCosts = computeProgramCosts(o.PricePoints, o.Source.RedeemProgram.TransferOptions)
		}
		deals = append(deals, deal)
	}
	return deals
}

func printDealBlocks(w io.Writer, deals []dealView) {
	for i, d := range deals {
		if i > 0 {
			fmt.Fprintln(w)
		}
		fmt.Fprintf(w, "#%d  %s    [%s]    %s\n", i+1, d.FlightNumbers, d.Airline, d.Aircraft)
		fmt.Fprintf(w, "      %s → %s  ·  %s  ·  %s  (%s)\n", d.DepartsAt, d.ArrivesAt, d.TotalDuration, d.Route, stopsLabel(d.TotalStops))
		fmt.Fprintf(w, "       %s pts  +  %s  ·  %.2f cpp  ·  %s  ·  %s\n", points(d.PricePoints), moneyString(d.PriceRetail, d.PriceCurrency), d.CPP, dealClassForOutput(w, d.DealClassification), seatsLabel(d.QuantityRemaining))
		fmt.Fprintf(w, "      Redeem via %s\n", d.RedeemProgram)
		fmt.Fprintf(w, "      Costs in your points:  %s\n", formatProgramCosts(d.ProgramCosts))
	}
}

func printCompactDeals(w io.Writer, deals []dealView) {
	for _, d := range deals {
		fmt.Fprintf(w, "%s  ·  %s  ·  %s  ·  %s  ·  %s pts + %s  ·  %s  ·  %s\n", d.FlightNumbers, d.Route, d.DepartsAt, d.TotalDuration, points(d.PricePoints), moneyString(d.PriceRetail, d.PriceCurrency), d.DealClassification, seatsLabel(d.QuantityRemaining))
	}
}

func sortedSegments(o rawOffer) []offerFlightSegment {
	segs := append([]offerFlightSegment(nil), o.OfferFlightSegments...)
	sort.SliceStable(segs, func(i, j int) bool { return segs[i].Order < segs[j].Order })
	return segs
}

func routeString(o rawOffer) string {
	segs := sortedSegments(o)
	if len(segs) == 0 {
		return joinArrow(o.OriginCode, o.DestinationCode)
	}
	parts := []string{segs[0].OriginCode}
	for _, s := range segs {
		parts = append(parts, s.DestinationCode)
	}
	return joinArrow(parts...)
}

func airlineName(o rawOffer, all bool) string {
	var names []string
	for _, s := range sortedSegments(o) {
		name := firstValue(s.Airline.Name, s.AirlineCode)
		if name == "" {
			continue
		}
		if !all {
			return name
		}
		if len(names) == 0 || names[len(names)-1] != name {
			names = append(names, name)
		}
	}
	if len(names) > 0 {
		return strings.Join(names, " → ")
	}
	parts := csvParts(o.AirlinesList)
	if len(parts) == 0 {
		return "-"
	}
	if !all {
		return parts[0]
	}
	return strings.Join(parts, " → ")
}

func aircraftName(o rawOffer) string {
	for _, s := range sortedSegments(o) {
		if s.AircraftName != "" {
			return s.AircraftName
		}
	}
	return "-"
}

func flightNumbers(o rawOffer) string {
	if o.FlightNumbers != "" {
		return o.FlightNumbers
	}
	var out []string
	for _, s := range sortedSegments(o) {
		if s.FlightNumber != "" {
			out = append(out, s.FlightNumber)
		}
	}
	if len(out) == 0 {
		return "-"
	}
	return strings.Join(out, ", ")
}

func earnProgramStrings(o rawOffer) []string {
	var out []string
	seen := map[string]bool{}
	for _, opt := range o.Source.RedeemProgram.TransferOptions {
		key := firstValue(opt.EarnProgram.ID, opt.EarnProgram.Identifier, opt.EarnProgram.Name)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		name := firstValue(opt.EarnProgram.Name, opt.EarnProgram.Identifier)
		if ratio := firstValue(opt.TotalTransferRatio, opt.TransferRatio); ratio != "" {
			name += " (" + trimRatio(ratio) + ":1)"
		}
		out = append(out, name)
	}
	if len(out) == 0 {
		return []string{"-"}
	}
	return out
}

func computeProgramCosts(pricePoints int, transferOptions []offerTransferOption) []ProgramCost {
	if pricePoints <= 0 {
		return nil
	}
	var out []ProgramCost
	for _, opt := range transferOptions {
		ratioStr := opt.TotalTransferRatio
		if ratioStr == "" {
			ratioStr = opt.TransferRatio
		}
		ratio, err := strconv.ParseFloat(ratioStr, 64)
		if err != nil || ratio <= 0 {
			continue
		}
		ep := opt.EarnProgram
		if ep.ID == "" {
			continue
		}
		effective := int(math.Ceil(float64(pricePoints) / ratio))
		hasBonus := strings.TrimSpace(opt.BonusTransferRatio) != ""
		out = append(out, ProgramCost{
			ProgramID:       ep.ID,
			ProgramName:     ep.Name,
			Identifier:      ep.Identifier,
			EffectivePoints: effective,
			TransferRatio:   ratioStr,
			TransferTime:    opt.TransferTime,
			HasBonus:        hasBonus,
		})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].EffectivePoints != out[j].EffectivePoints {
			return out[i].EffectivePoints < out[j].EffectivePoints
		}
		return out[i].TransferTime < out[j].TransferTime
	})
	return out
}

func formatProgramCosts(costs []ProgramCost) string {
	if len(costs) == 0 {
		return "-"
	}
	limit := len(costs)
	more := 0
	if limit > 6 {
		limit = 6
		more = len(costs) - limit
	}
	parts := make([]string, 0, limit)
	for _, cost := range costs[:limit] {
		label := fmt.Sprintf("%s %s", shortenProgramName(cost.ProgramName), points(cost.EffectivePoints))
		if cost.HasBonus {
			label += "*"
		}
		if cost.TransferTime != "" && cost.TransferTime != "instant" {
			label += fmt.Sprintf(" (%s)", cost.TransferTime)
		}
		parts = append(parts, label)
	}
	out := strings.Join(parts, "  ·  ")
	if more > 0 {
		out += fmt.Sprintf("  +%d more", more)
	}
	return out
}

func shortenProgramName(name string) string {
	name = strings.TrimSpace(name)
	shortNames := map[string]string{
		"Chase Ultimate Rewards":         "Chase UR",
		"Amex Membership Rewards":        "Amex MR",
		"Citi ThankYou Points":           "Citi TY",
		"Capital One Rewards":            "Capital One",
		"Bilt Rewards":                   "Bilt",
		"Rove Miles":                     "Rove",
		"Marriott Bonvoy":                "Marriott Bonvoy",
		"World of Hyatt Loyalty Program": "Hyatt",
		"IHG One Rewards":                "IHG",
		"Brex Rewards":                   "Brex",
		"Wells Fargo Autograph Points":   "Wells Fargo",
		"British Airways Avios":          "BA Avios",
	}
	if short, ok := shortNames[name]; ok {
		return short
	}
	for _, suffix := range []string{" Loyalty Program", " Rewards", " Points", " Miles"} {
		if strings.HasSuffix(name, suffix) {
			return strings.TrimSpace(strings.TrimSuffix(name, suffix))
		}
	}
	return name
}

func moneyValue(s string) float64 {
	v, _ := strconv.ParseFloat(strings.NewReplacer("$", "", ",", "", " ", "").Replace(s), 64)
	return v
}

func cppValue(retail float64, pricePoints int, pricePerPoint string) float64 {
	if retail > 0 && pricePoints > 0 {
		return retail * 100 / float64(pricePoints)
	}
	v, _ := strconv.ParseFloat(pricePerPoint, 64)
	if v < 1 {
		return v * 100
	}
	return v
}

func dealClass(cpp float64) string {
	switch {
	case cpp >= 4:
		return "GREAT deal"
	case cpp >= 2:
		return "Good deal"
	case cpp >= 1.3:
		return "Okay deal"
	default:
		return "Weak deal"
	}
}

func dealClassForOutput(w io.Writer, s string) string {
	if s == "GREAT deal" && humanFriendly && !noColor && isTerminal(w) && os.Getenv("NO_COLOR") == "" && os.Getenv("TERM") != "dumb" {
		return "\033[1m" + s + "\033[0m"
	}
	return s
}

func formatDealTime(s string) string {
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.Format("2006-01-02 15:04")
	}
	s = strings.ReplaceAll(s, "T", " ")
	if len(s) > 16 {
		return s[:16]
	}
	return s
}

func formatDealDuration(minutes int) string {
	if minutes <= 0 {
		return "-"
	}
	return fmt.Sprintf("%dh%02dm", minutes/60, minutes%60)
}

func moneyString(v float64, currency string) string {
	if currency == "" {
		currency = "USD"
	}
	return fmt.Sprintf("$%.2f %s", v, currency)
}

func points(n int) string {
	s := strconv.Itoa(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

func trimRatio(s string) string {
	if v, err := strconv.ParseFloat(s, 64); err == nil {
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return s
}

func stopsLabel(n int) string {
	if n == 1 {
		return "1 stop"
	}
	return fmt.Sprintf("%d stops", n)
}

func seatsLabel(n int) string {
	if n == 1 {
		return "1 seat left"
	}
	return fmt.Sprintf("%d seats left", n)
}

func csvParts(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func joinArrow(values ...string) string {
	var out []string
	for _, v := range values {
		if v != "" {
			out = append(out, v)
		}
	}
	return strings.Join(out, " → ")
}

func firstValue(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
