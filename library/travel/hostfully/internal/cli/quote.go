// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/mvanhorn/printing-press-library/library/travel/hostfully/internal/config"
	"github.com/spf13/cobra"
)

type quoteRequest struct {
	AgencyUID    string `json:"agencyUid"`
	LeadUID      string `json:"leadUid,omitempty"`
	PropertyUID  string `json:"propertyUid"`
	CheckInDate  string `json:"checkInDate"`
	CheckOutDate string `json:"checkOutDate"`
	Guests       int    `json:"guests"`
	Children     int    `json:"childrenCount,omitempty"`
	Pets         int    `json:"petCount,omitempty"`
}

type quoteAPIResponse struct {
	Quote struct {
		TotalAmount    float64 `json:"totalAmount"`
		TotalTaxAmount float64 `json:"totalTaxAmount"`
		Currency       string  `json:"currency"`
		CurrencySymbol string  `json:"currencySymbol"`
		Rent           struct {
			NetPrice       float64 `json:"netPrice"`
			TaxAmount      float64 `json:"taxAmount"`
			GrossPrice     float64 `json:"grossPrice"`
			RentBreakdowns []struct {
				NightlyAmount     float64 `json:"nightlyAmount"`
				NightlyBaseAmount float64 `json:"nightlyBaseAmount"`
				NightlyDate       string  `json:"nightlyDate"`
			} `json:"rentBreakdowns"`
		} `json:"rent"`
		Fees struct {
			CleaningFee quotePriceAPIResponse `json:"cleaningFee"`
			OtherFees   []quoteFeeAPIResponse `json:"otherFees"`
			TotalAmount float64               `json:"totalFeeAmount"`
		} `json:"fees"`
	} `json:"quote"`
}

type quotePriceAPIResponse struct {
	NetPrice   float64 `json:"netPrice"`
	TaxAmount  float64 `json:"taxAmount"`
	GrossPrice float64 `json:"grossPrice"`
}

type quoteFeeAPIResponse struct {
	Name       string  `json:"name"`
	Type       string  `json:"type"`
	NetPrice   float64 `json:"netPrice"`
	TaxAmount  float64 `json:"taxAmount"`
	GrossPrice float64 `json:"grossPrice"`
}

type quoteFeeResult struct {
	Name        string  `json:"name"`
	NetAmount   float64 `json:"net_amount"`
	TaxAmount   float64 `json:"tax_amount"`
	GrossAmount float64 `json:"gross_amount"`
}

type quoteNightResult struct {
	Date       string  `json:"date"`
	BaseAmount float64 `json:"base_amount"`
	Amount     float64 `json:"amount"`
}

type quoteResult struct {
	Unit                       int                `json:"unit"`
	CheckIn                    string             `json:"check_in"`
	CheckOut                   string             `json:"check_out"`
	Guests                     int                `json:"guests"`
	Children                   int                `json:"children"`
	Pets                       int                `json:"pets"`
	TotalAmount                float64            `json:"total_amount"`
	TotalTaxAmount             float64            `json:"total_tax_amount"`
	HostfullyReportedTaxAmount float64            `json:"hostfully_reported_tax_amount"`
	BaseRent                   float64            `json:"base_rent"`
	RentTax                    float64            `json:"rent_tax"`
	RentGross                  float64            `json:"rent_gross"`
	FeesNet                    float64            `json:"fees_net"`
	FeesTax                    float64            `json:"fees_tax"`
	FeesGross                  float64            `json:"fees_gross"`
	FeeBreakdown               []quoteFeeResult   `json:"fee_breakdown,omitempty"`
	NightlyRates               []quoteNightResult `json:"nightly_rates,omitempty"`
	Currency                   string             `json:"currency"`
	CurrencySymbol             string             `json:"currency_symbol"`
	Source                     string             `json:"source"`
}

type quoteLoader func(flags *rootFlags, request quoteRequest) (quoteAPIResponse, error)

func newNovelQuoteCmd(flags *rootFlags) *cobra.Command {
	return newNovelQuoteCmdWithLoader(flags, loadLiveQuote)
}

func newNovelQuoteCmdWithLoader(flags *rootFlags, load quoteLoader) *cobra.Command {
	var unit, guests, children, pets int
	var checkIn, checkOut, leadUID string
	var breakdown bool
	cmd := &cobra.Command{
		Use:     "quote",
		Example: "  hostfully-pp-cli quote --unit 1 --check-in 2026-11-06 --check-out 2026-11-08 --guests 2 --agent",
		Short:   "Calculate a fresh complete Hostfully stay quote",
		Long: `Calculate a current complete quote directly from Hostfully.

This command never reads pricing from the local mirror or HTTP cache. It is a
read-only calculation: it does not create, hold, or modify a reservation.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := requireUnits(); err != nil && !mockMode() {
				return err
			}
			propertyUID, ok := unitUID(unit)
			if !ok {
				return usageErr(fmt.Errorf("unknown unit %d", unit))
			}
			if guests < 1 {
				return usageErr(fmt.Errorf("--guests must be at least 1"))
			}
			if children < 0 || pets < 0 {
				return usageErr(fmt.Errorf("--children and --pets cannot be negative"))
			}
			arrival, err := time.Parse("2006-01-02", checkIn)
			if err != nil {
				return usageErr(fmt.Errorf("--check-in must be YYYY-MM-DD"))
			}
			departure, err := time.Parse("2006-01-02", checkOut)
			if err != nil {
				return usageErr(fmt.Errorf("--check-out must be YYYY-MM-DD"))
			}
			if !departure.After(arrival) {
				return usageErr(fmt.Errorf("--check-out must be after --check-in"))
			}
			request := quoteRequest{
				AgencyUID: quoteAgencyUID(flags), LeadUID: leadUID, PropertyUID: propertyUID,
				CheckInDate: checkIn, CheckOutDate: checkOut, Guests: guests,
				Children: children, Pets: pets,
			}
			if request.AgencyUID == "" {
				return usageErr(fmt.Errorf("HOSTFULLY_AGENCY_ID environment variable is required"))
			}
			var response quoteAPIResponse
			if mockMode() {
				response.Quote.TotalAmount = 134
				response.Quote.TotalTaxAmount = 14
				response.Quote.Rent.NetPrice = 100
				response.Quote.Rent.TaxAmount = 10
				response.Quote.Rent.GrossPrice = 110
				response.Quote.Fees.CleaningFee = quotePriceAPIResponse{NetPrice: 20, TaxAmount: 4, GrossPrice: 24}
				response.Quote.Fees.TotalAmount = 24
				response.Quote.Currency = "USD"
				response.Quote.CurrencySymbol = "$"
			} else {
				response, err = load(flags, request)
				if err != nil {
					return fmt.Errorf("calculating live Hostfully quote: %w", err)
				}
			}
			feeBreakdown, feesNet, feesTax, feesGross := quoteFeeBreakdown(response)
			nightlyRates := make([]quoteNightResult, 0, len(response.Quote.Rent.RentBreakdowns))
			for _, night := range response.Quote.Rent.RentBreakdowns {
				nightlyRates = append(nightlyRates, quoteNightResult{Date: night.NightlyDate, BaseAmount: night.NightlyBaseAmount, Amount: night.NightlyAmount})
			}
			lineItemTaxTotal := roundQuoteMoney(response.Quote.Rent.TaxAmount + feesTax)
			result := quoteResult{
				Unit: unit, CheckIn: checkIn, CheckOut: checkOut, Guests: guests,
				Children: children, Pets: pets, TotalAmount: response.Quote.TotalAmount,
				TotalTaxAmount:             lineItemTaxTotal,
				HostfullyReportedTaxAmount: response.Quote.TotalTaxAmount,
				BaseRent:                   response.Quote.Rent.NetPrice,
				RentTax:                    response.Quote.Rent.TaxAmount,
				RentGross:                  response.Quote.Rent.GrossPrice,
				FeesNet:                    feesNet,
				FeesTax:                    feesTax,
				FeesGross:                  feesGross,
				FeeBreakdown:               feeBreakdown,
				NightlyRates:               nightlyRates,
				Currency:                   response.Quote.Currency, CurrencySymbol: response.Quote.CurrencySymbol,
				Source: map[bool]string{true: "mock", false: "hostfully_live"}[mockMode()],
			}
			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 1, ' ', 0)
			if breakdown {
				fmt.Fprintf(w, "Component\tNet\tTax\tGross\n")
				fmt.Fprintf(w, "Base rent\t%s%.2f\t%s%.2f\t%s%.2f\n", result.CurrencySymbol, result.BaseRent, result.CurrencySymbol, result.RentTax, result.CurrencySymbol, result.RentGross)
				for _, fee := range result.FeeBreakdown {
					fmt.Fprintf(w, "%s\t%s%.2f\t%s%.2f\t%s%.2f\n", fee.Name, result.CurrencySymbol, fee.NetAmount, result.CurrencySymbol, fee.TaxAmount, result.CurrencySymbol, fee.GrossAmount)
				}
				fmt.Fprintf(w, "All fees\t%s%.2f\t%s%.2f\t%s%.2f\n", result.CurrencySymbol, result.FeesNet, result.CurrencySymbol, result.FeesTax, result.CurrencySymbol, result.FeesGross)
				fmt.Fprintf(w, "Total taxes\t\t%s%.2f\t\n", result.CurrencySymbol, result.TotalTaxAmount)
				fmt.Fprintf(w, "Complete total\t\t\t%s%.2f %s\n", result.CurrencySymbol, result.TotalAmount, result.Currency)
			} else {
				fmt.Fprintf(w, "Unit\tDates\tGuests\tComplete total\n")
				fmt.Fprintf(w, "%d\t%s → %s\t%d\t%s%.2f %s\n",
					unit, checkIn, checkOut, guests, result.CurrencySymbol, result.TotalAmount, result.Currency)
			}
			return w.Flush()
		},
	}
	cmd.Flags().IntVar(&unit, "unit", 0, "Unit number (see: hostfully-pp-cli units)")
	cmd.Flags().StringVar(&checkIn, "check-in", "", "Arrival date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&checkOut, "check-out", "", "Departure date (YYYY-MM-DD)")
	cmd.Flags().IntVar(&guests, "guests", 0, "Adult guest count")
	cmd.Flags().IntVar(&children, "children", 0, "Child guest count")
	cmd.Flags().IntVar(&pets, "pets", 0, "Pet count")
	cmd.Flags().StringVar(&leadUID, "lead-uid", "", "Existing reservation UID for an extension quote")
	cmd.Flags().BoolVar(&breakdown, "breakdown", false, "Show base rent, taxes, and fee line items")
	_ = cmd.MarkFlagRequired("unit")
	_ = cmd.MarkFlagRequired("check-in")
	_ = cmd.MarkFlagRequired("check-out")
	_ = cmd.MarkFlagRequired("guests")
	return cmd
}

func roundQuoteMoney(value float64) float64 {
	return math.Round(value*100) / 100
}

func quoteFeeBreakdown(response quoteAPIResponse) ([]quoteFeeResult, float64, float64, float64) {
	fees := make([]quoteFeeResult, 0, len(response.Quote.Fees.OtherFees)+1)
	net, tax, gross := 0.0, 0.0, 0.0
	cleaning := response.Quote.Fees.CleaningFee
	if cleaning.NetPrice != 0 || cleaning.TaxAmount != 0 || cleaning.GrossPrice != 0 {
		fees = append(fees, quoteFeeResult{Name: "Cleaning fee", NetAmount: cleaning.NetPrice, TaxAmount: cleaning.TaxAmount, GrossAmount: cleaning.GrossPrice})
		net += cleaning.NetPrice
		tax += cleaning.TaxAmount
		gross += cleaning.GrossPrice
	}
	for _, fee := range response.Quote.Fees.OtherFees {
		if strings.EqualFold(fee.Type, "TAX") {
			continue
		}
		name := strings.TrimSpace(fee.Name)
		if name == "" {
			name = "Other fee"
		}
		fees = append(fees, quoteFeeResult{Name: name, NetAmount: fee.NetPrice, TaxAmount: fee.TaxAmount, GrossAmount: fee.GrossPrice})
		net += fee.NetPrice
		tax += fee.TaxAmount
		gross += fee.GrossPrice
	}
	return fees, net, tax, gross
}

func quoteAgencyUID(flags *rootFlags) string {
	cfg, err := config.Load(flags.configPath)
	if err != nil {
		return ""
	}
	return cfg.AgencyUID()
}

func loadLiveQuote(flags *rootFlags, request quoteRequest) (quoteAPIResponse, error) {
	c, err := flags.newClient()
	if err != nil {
		return quoteAPIResponse{}, err
	}
	base := strings.TrimRight(c.BaseURL, "/")
	if strings.HasSuffix(base, "/api/v3") {
		base = strings.TrimSuffix(base, "/api/v3") + "/api/v3.3"
	}
	c.BaseURL = base
	c.NoCache = true
	ctx, cancel := boundCtx(context.Background(), flags)
	defer cancel()
	raw, _, err := c.PostQueryWithParams(ctx, "/quotes", nil, request)
	if err != nil {
		return quoteAPIResponse{}, classifyAPIErrorOnly(err)
	}
	var response quoteAPIResponse
	if err := json.Unmarshal(raw, &response); err != nil {
		return quoteAPIResponse{}, fmt.Errorf("parsing quote response: %w", err)
	}
	return response, nil
}
