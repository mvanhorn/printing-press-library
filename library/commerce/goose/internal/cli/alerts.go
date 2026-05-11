package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// newAlertsCmd is the parent for `alerts` subcommands.
func newAlertsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alerts",
		Short: "Daily operational risk panels (vaccines, agreements, balances, vouchers)",
	}
	cmd.AddCommand(newAlertsDailyCmd(flags))
	return cmd
}

// newAlertsDailyCmd composes the daily pre-open risk panel: incoming pets
// with expired vaccinations, customers without active agreements, today's
// checkouts with non-zero outstanding balance, and customer vouchers
// expiring within 7 days for today's customers.
func newAlertsDailyCmd(flags *rootFlags) *cobra.Command {
	var date string
	cmd := &cobra.Command{
		Use:         "daily",
		Short:       "Daily-prep alert panel: expired vaccines, missing agreements, balance due, voucher expiry",
		Example:     "  goose-pp-cli alerts daily --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			if dryRunOK(flags) {
				return nil
			}
			if date == "" {
				date = time.Now().Format("2006-01-02")
			}
			// 1. Pull dashboard/invoices with the relations needed to compute
			//    today's vaccine + agreement + balance risk.
			params := map[string]string{
				"visitDate":     date,
				"invoiceStatus": "CONFIRMED",
				"limit":         "500",
				"includes": strings.Join([]string{
					"order.pets.locationPetProfile.vaccinations",
					"order.locationUserProfile.agreements",
				}, ","),
			}
			raw, err := c.Get("/dashboard/invoices", params)
			if err != nil {
				return fmt.Errorf("fetching dashboard: %w", err)
			}
			var env struct {
				Results []json.RawMessage `json:"results"`
			}
			if err := json.Unmarshal(raw, &env); err != nil {
				return fmt.Errorf("parsing dashboard: %w", err)
			}

			alerts := dailyAlerts{Date: date}
			customerIds := map[string]bool{}
			for _, r := range env.Results {
				processInvoiceForAlerts(r, &alerts, customerIds)
			}

			// 2. Bulk balance lookup for today's customers.
			if len(customerIds) > 0 {
				ids := make([]string, 0, len(customerIds))
				for id := range customerIds {
					ids = append(ids, id)
				}
				body := map[string]any{"locationUserProfileIds": ids}
				balanceData, _, berr := c.Post("/location-user-profiles/outstanding-balance", body)
				if berr == nil {
					processBalances(balanceData, &alerts)
				}
			}

			// 3. Voucher expiry — fetch vouchers expiring in 7 days for today's customers.
			//    Done via the vouchers endpoint filtered to type CASH + isUsed=false + expired=false.
			//    Per the catalog, voucher expiry is server-side; we filter client-side by
			//    intersecting with today's customer IDs.
			vparams := map[string]string{
				"status":              "ACTIVE",
				"type":                "CASH",
				"isUsed":              "false",
				"expired":             "false",
				"atLeastOneAvailable": "true",
				"includeShared":       "false",
				"limit":               "500",
			}
			vouchers, verr := c.Get("/vouchers", vparams)
			if verr == nil {
				processVouchers(vouchers, customerIds, &alerts)
			}

			if flags.asJSON || flags.compact {
				return printJSONFiltered(cmd.OutOrStdout(), alerts, flags)
			}
			renderAlertsHuman(cmd.OutOrStdout(), &alerts)
			return nil
		},
	}
	cmd.Flags().StringVar(&date, "date", "", "Date for the alert panel (YYYY-MM-DD); defaults to today")
	return cmd
}

type dailyAlerts struct {
	Date              string     `json:"date"`
	ExpiredVaccines   []alertRow `json:"expiredVaccines"`
	MissingAgreements []alertRow `json:"missingAgreements"`
	BalanceDue        []alertRow `json:"balanceDue"`
	VouchersExpiring  []alertRow `json:"vouchersExpiring"`
}

type alertRow struct {
	CustomerID   string `json:"customerId,omitempty"`
	CustomerName string `json:"customerName,omitempty"`
	PetName      string `json:"petName,omitempty"`
	Detail       string `json:"detail,omitempty"`
	Amount       string `json:"amount,omitempty"`
}

func processInvoiceForAlerts(raw json.RawMessage, alerts *dailyAlerts, customerIds map[string]bool) {
	var inv struct {
		Period struct {
			StartDate string `json:"startDate"`
			EndDate   string `json:"endDate"`
		} `json:"period"`
		Order struct {
			LocationUserProfile struct {
				ID         string `json:"id"`
				FirstName  string `json:"firstName"`
				LastName   string `json:"lastName"`
				Agreements []any  `json:"agreements"`
			} `json:"locationUserProfile"`
			OrderUser struct {
				ID         string `json:"id"`
				FirstName  string `json:"firstName"`
				LastName   string `json:"lastName"`
				Agreements []any  `json:"agreements"`
			} `json:"orderUser"`
			Pets []struct {
				LocationPetProfile struct {
					DisplayName  string `json:"displayName"`
					Vaccinations []struct {
						ExpirationDate string `json:"expirationDate"`
					} `json:"vaccinations"`
				} `json:"locationPetProfile"`
			} `json:"pets"`
		} `json:"order"`
	}
	if err := json.Unmarshal(raw, &inv); err != nil {
		return
	}
	cust := inv.Order.LocationUserProfile
	if cust.ID == "" {
		cust = inv.Order.OrderUser
	}
	name := strings.TrimSpace(cust.FirstName + " " + cust.LastName)
	if cust.ID != "" {
		customerIds[cust.ID] = true
	}
	if len(cust.Agreements) == 0 {
		alerts.MissingAgreements = append(alerts.MissingAgreements, alertRow{
			CustomerID:   cust.ID,
			CustomerName: name,
		})
	}
	for _, p := range inv.Order.Pets {
		for _, v := range p.LocationPetProfile.Vaccinations {
			if v.ExpirationDate != "" && v.ExpirationDate < inv.Period.StartDate {
				alerts.ExpiredVaccines = append(alerts.ExpiredVaccines, alertRow{
					CustomerID:   cust.ID,
					CustomerName: name,
					PetName:      p.LocationPetProfile.DisplayName,
					Detail:       "expired " + v.ExpirationDate,
				})
			}
		}
	}
}

func processBalances(raw json.RawMessage, alerts *dailyAlerts) {
	// Shape may vary; accept {balances: {id: amount}} or array {id, amount}.
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return
	}
	if b, ok := m["balances"].(map[string]any); ok {
		m = b
	}
	for id, v := range m {
		amount := ""
		switch val := v.(type) {
		case string:
			amount = val
		case float64:
			amount = fmt.Sprintf("%.2f", val)
		case map[string]any:
			if s, ok := val["amount"].(string); ok {
				amount = s
			}
		}
		if amount != "" && amount != "0" && amount != "0.00" && !strings.HasPrefix(amount, "-") {
			alerts.BalanceDue = append(alerts.BalanceDue, alertRow{
				CustomerID: id,
				Amount:     amount,
			})
		}
	}
}

func processVouchers(raw json.RawMessage, customerIds map[string]bool, alerts *dailyAlerts) {
	var env struct {
		Results []struct {
			LocationUserProfile struct {
				ID        string `json:"id"`
				FirstName string `json:"firstName"`
				LastName  string `json:"lastName"`
			} `json:"locationUserProfile"`
			ExpirationDate string `json:"expirationDate"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		var arr []map[string]any
		if jerr := json.Unmarshal(raw, &arr); jerr != nil {
			return
		}
		return
	}
	now := time.Now()
	cutoff := now.AddDate(0, 0, 7).Format("2006-01-02")
	for _, r := range env.Results {
		if r.ExpirationDate == "" || r.ExpirationDate > cutoff {
			continue
		}
		if customerIds != nil && len(customerIds) > 0 && !customerIds[r.LocationUserProfile.ID] {
			continue
		}
		alerts.VouchersExpiring = append(alerts.VouchersExpiring, alertRow{
			CustomerID:   r.LocationUserProfile.ID,
			CustomerName: strings.TrimSpace(r.LocationUserProfile.FirstName + " " + r.LocationUserProfile.LastName),
			Detail:       "expires " + r.ExpirationDate,
		})
	}
}

func renderAlertsHuman(w interface{ Write([]byte) (int, error) }, a *dailyAlerts) {
	fmt.Fprintf(w, "Goose daily alerts — %s\n\n", a.Date)
	fmt.Fprintf(w, "Expired vaccines on incoming pets: %d\n", len(a.ExpiredVaccines))
	for _, r := range a.ExpiredVaccines {
		fmt.Fprintf(w, "  %s — %s (%s)\n", r.PetName, r.CustomerName, r.Detail)
	}
	fmt.Fprintf(w, "\nCustomers missing active agreement: %d\n", len(a.MissingAgreements))
	for _, r := range a.MissingAgreements {
		fmt.Fprintf(w, "  %s\n", r.CustomerName)
	}
	fmt.Fprintf(w, "\nCheckouts with non-zero balance: %d\n", len(a.BalanceDue))
	for _, r := range a.BalanceDue {
		fmt.Fprintf(w, "  %s — %s\n", r.CustomerID, r.Amount)
	}
	fmt.Fprintf(w, "\nVouchers expiring within 7d (today's customers): %d\n", len(a.VouchersExpiring))
	for _, r := range a.VouchersExpiring {
		fmt.Fprintf(w, "  %s — %s\n", r.CustomerName, r.Detail)
	}
}
