// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/store"

	"github.com/spf13/cobra"
)

// RankedQuote is one row in a rate-shop result, ranked by net cost or transit days.
type RankedQuote struct {
	ServiceType string  `json:"service_type"`
	NetAmount   float64 `json:"net_amount"`
	ListAmount  float64 `json:"list_amount"`
	Currency    string  `json:"currency"`
	TransitDays int     `json:"transit_days,omitempty"`
	DeliveryDay string  `json:"delivery_day,omitempty"`
	Selected    bool    `json:"selected,omitempty"`
}

// RateShopResult is the JSON envelope returned by `rate shop`.
type RateShopResult struct {
	Origin      string        `json:"origin"`
	Dest        string        `json:"dest"`
	WeightValue float64       `json:"weight_value"`
	WeightUnits string        `json:"weight_units"`
	Rates       []RankedQuote `json:"rates"`
	QuotedAt    string        `json:"quoted_at"`
}

func newRateCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rate",
		Short: "Rate-shop helpers (parallel multi-service quoting, ranking, persistence)",
	}
	cmd.AddCommand(newRateShopCmd(flags))
	return cmd
}

func newRateShopCmd(flags *rootFlags) *cobra.Command {
	var (
		fromPostal  string
		toPostal    string
		fromCountry string
		toCountry   string
		weightArg   string
		units       string
		account     string
		services    string
		rankBy      string
	)

	cmd := &cobra.Command{
		Use:   "shop",
		Short: "Quote rates across every applicable service type and rank by cost or transit",
		Long: `Quote rates across every applicable service type for a single shipment, persist
each quote to the local rate_quotes ledger, and rank by net cost or transit days.
The selected (cheapest, when ranking by cost) quote is marked selected=1 in the
ledger so spend/lane reports can attribute follow-on shipments back to the chosen rate.`,
		Example: strings.Trim(`
  fedex-pp-cli rate shop --from 98101 --to 10001 --weight 5
  fedex-pp-cli rate shop --from 98101 --to 10001 --weight 5lb --json
  fedex-pp-cli rate shop --from 98101 --to 10001 --weight 12 --rank-by transit --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			weight, parsedUnits, err := parseWeightArg(weightArg, units)
			if err != nil {
				return usageErr(err)
			}
			if parsedUnits != "" {
				units = parsedUnits
			}
			if fromPostal == "" || toPostal == "" || weight <= 0 {
				return cmd.Help()
			}
			if account == "" {
				account = os.Getenv("FEDEX_ACCOUNT_NUMBER")
			}
			if account == "" && !flags.dryRun {
				return usageErr(fmt.Errorf("--account is required (or set FEDEX_ACCOUNT_NUMBER)"))
			}
			if dryRunOK(flags) {
				return nil
			}

			svcList := []string{}
			for _, s := range strings.Split(services, ",") {
				s = strings.TrimSpace(s)
				if s != "" {
					svcList = append(svcList, s)
				}
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			body := map[string]any{
				"accountNumber": map[string]any{"value": account},
				"requestedShipment": map[string]any{
					"shipper": map[string]any{
						"address": map[string]any{"postalCode": fromPostal, "countryCode": fromCountry},
					},
					"recipient": map[string]any{
						"address": map[string]any{"postalCode": toPostal, "countryCode": toCountry},
					},
					"pickupType":      "DROPOFF_AT_FEDEX_LOCATION",
					"rateRequestType": []string{"LIST", "ACCOUNT"},
					"requestedPackageLineItems": []any{
						map[string]any{"weight": map[string]any{"units": units, "value": weight}},
					},
				},
			}

			data, _, err := c.Post("/rate/v1/rates/quotes", body)
			if err != nil {
				return classifyAPIError(err)
			}

			ranked := parseRateShopResponse(data, svcList)
			rankRateQuotes(ranked, rankBy)

			ctx := context.Background()
			if st, openErr := store.Open(""); openErr == nil {
				defer st.Close()
				for _, q := range ranked {
					_ = st.InsertRateQuote(ctx, store.RateQuote{
						OriginPostal:      fromPostal,
						OriginCountry:     fromCountry,
						DestPostal:        toPostal,
						DestCountry:       toCountry,
						WeightValue:       weight,
						WeightUnits:       units,
						ServiceType:       q.ServiceType,
						ListAmount:        q.ListAmount,
						NetAmount:         q.NetAmount,
						Currency:          q.Currency,
						TransitDays:       q.TransitDays,
						DeliveryDayOfWeek: q.DeliveryDay,
						Selected:          q.Selected,
					})
				}
			}

			result := RateShopResult{
				Origin:      fmt.Sprintf("%s,%s", fromPostal, fromCountry),
				Dest:        fmt.Sprintf("%s,%s", toPostal, toCountry),
				WeightValue: weight,
				WeightUnits: units,
				Rates:       ranked,
				QuotedAt:    time.Now().UTC().Format(time.RFC3339),
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				headers := []string{"SERVICE", "NET", "LIST", "CCY", "TRANSIT", "DELIVERY", "SELECTED"}
				rows := make([][]string, 0, len(ranked))
				for _, q := range ranked {
					sel := ""
					if q.Selected {
						sel = "*"
					}
					transit := ""
					if q.TransitDays > 0 {
						transit = fmt.Sprintf("%d", q.TransitDays)
					}
					rows = append(rows, []string{
						q.ServiceType,
						fmt.Sprintf("%.2f", q.NetAmount),
						fmt.Sprintf("%.2f", q.ListAmount),
						q.Currency,
						transit,
						q.DeliveryDay,
						sel,
					})
				}
				return flags.printTable(cmd, headers, rows)
			}
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}

	cmd.Flags().StringVar(&fromPostal, "from", "", "Origin postal code")
	cmd.Flags().StringVar(&toPostal, "to", "", "Destination postal code")
	cmd.Flags().StringVar(&fromCountry, "from-country", "US", "Origin country code")
	cmd.Flags().StringVar(&toCountry, "to-country", "US", "Destination country code")
	cmd.Flags().StringVar(&weightArg, "weight", "", "Package weight (e.g. 5, 5lb, 2.5kg). When the value carries a unit suffix it overrides --units.")
	cmd.Flags().StringVar(&units, "units", "LB", "Weight units (LB or KG)")
	cmd.Flags().StringVar(&account, "account", "", "FedEx account number (defaults to env FEDEX_ACCOUNT_NUMBER)")
	cmd.Flags().StringVar(&services, "services", "FEDEX_GROUND,FEDEX_2_DAY,STANDARD_OVERNIGHT,PRIORITY_OVERNIGHT,FEDEX_EXPRESS_SAVER,GROUND_HOME_DELIVERY", "Comma-separated service types to consider when ranking")
	cmd.Flags().StringVar(&rankBy, "rank-by", "cost", "Ranking field: cost or transit")
	return cmd
}

// parseRateShopResponse extracts service-keyed rates from the FedEx
// /rate/v1/rates/quotes response. Optional restrictTo list filters the output
// to the requested service types (FedEx returns every applicable service when
// the request omits a serviceType, so the client-side filter trims to what the
// caller asked about).
func parseRateShopResponse(data json.RawMessage, restrictTo []string) []RankedQuote {
	var resp struct {
		Output struct {
			RateReplyDetails []struct {
				ServiceType          string `json:"serviceType"`
				RatedShipmentDetails []struct {
					TotalNetCharge      float64 `json:"totalNetCharge"`
					TotalNetFedExCharge float64 `json:"totalNetFedExCharge"`
					TotalBaseCharge     float64 `json:"totalBaseCharge"`
					Currency            string  `json:"currency"`
					ShipmentRateDetail  struct {
						Currency       string  `json:"currency"`
						TotalNetCharge float64 `json:"totalNetCharge"`
					} `json:"shipmentRateDetail"`
				} `json:"ratedShipmentDetails"`
				OperationalDetail struct {
					TransitTime string `json:"transitTime"`
					DeliveryDay string `json:"deliveryDay"`
				} `json:"operationalDetail"`
				Commit struct {
					DateDetail struct {
						DayOfWeek string `json:"dayOfWeek"`
					} `json:"dateDetail"`
					TransitDays struct {
						Description        string `json:"description"`
						MinimumTransitTime string `json:"minimumTransitTime"`
						MaximumTransitTime string `json:"maximumTransitTime"`
					} `json:"transitDays"`
				} `json:"commit"`
			} `json:"rateReplyDetails"`
		} `json:"output"`
	}
	_ = json.Unmarshal(data, &resp)

	allow := map[string]bool{}
	for _, s := range restrictTo {
		allow[s] = true
	}

	out := []RankedQuote{}
	for _, d := range resp.Output.RateReplyDetails {
		if len(allow) > 0 && !allow[d.ServiceType] {
			continue
		}
		var netAmt, listAmt float64
		var ccy string
		if len(d.RatedShipmentDetails) > 0 {
			r := d.RatedShipmentDetails[0]
			netAmt = r.TotalNetCharge
			if netAmt == 0 {
				netAmt = r.ShipmentRateDetail.TotalNetCharge
			}
			listAmt = r.TotalBaseCharge
			ccy = r.Currency
			if ccy == "" {
				ccy = r.ShipmentRateDetail.Currency
			}
		}
		transit := transitDaysFromString(d.OperationalDetail.TransitTime)
		day := d.Commit.DateDetail.DayOfWeek
		if day == "" {
			day = d.OperationalDetail.DeliveryDay
		}
		out = append(out, RankedQuote{
			ServiceType: d.ServiceType,
			NetAmount:   netAmt,
			ListAmount:  listAmt,
			Currency:    ccy,
			TransitDays: transit,
			DeliveryDay: day,
		})
	}
	return out
}

// transitDaysFromString turns FedEx's word-form transit codes ("ONE_DAY",
// "TWO_DAYS", "THREE_DAYS"...) into the integer day count. Returns 0 when the
// code is unknown — callers treat 0 as "unspecified".
func transitDaysFromString(s string) int {
	switch strings.ToUpper(s) {
	case "ONE_DAY":
		return 1
	case "TWO_DAYS":
		return 2
	case "THREE_DAYS":
		return 3
	case "FOUR_DAYS":
		return 4
	case "FIVE_DAYS":
		return 5
	case "SIX_DAYS":
		return 6
	case "SEVEN_DAYS":
		return 7
	}
	return 0
}

func rankRateQuotes(qs []RankedQuote, by string) {
	switch by {
	case "transit":
		sort.SliceStable(qs, func(i, j int) bool {
			if qs[i].TransitDays == qs[j].TransitDays {
				return qs[i].NetAmount < qs[j].NetAmount
			}
			if qs[i].TransitDays == 0 {
				return false
			}
			if qs[j].TransitDays == 0 {
				return true
			}
			return qs[i].TransitDays < qs[j].TransitDays
		})
	default:
		sort.SliceStable(qs, func(i, j int) bool {
			return qs[i].NetAmount < qs[j].NetAmount
		})
	}
	if len(qs) > 0 {
		qs[0].Selected = true
	}
}
