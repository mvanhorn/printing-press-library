package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/travel/masterpark/internal/client"
	"github.com/mvanhorn/printing-press-library/library/travel/masterpark/internal/config"
)

type reserveFlags struct {
	lot             string
	dropoff         string
	pickup          string
	vehicleType     string
	promoCode       string
	quoteID         int
	firstName       string
	lastName        string
	email           string
	phone           string
	vehMake         string
	vehModel        string
	vehColor        string
	plate           string
	submit          bool
	yes             bool
	useSavedProfile bool
	creds           credFlags
}

func newReserveCmd(g *globalOpts) *cobra.Command {
	rf := &reserveFlags{quoteID: -1}
	cmd := &cobra.Command{
		Use:   "reserve",
		Short: "Compose a reservation (dry-run by default; --submit --yes to book)",
		Long: "Compose a MasterPark reservation from flags and a selected quote.\n" +
			"By default this prints a dry-run summary and does NOT book.\n" +
			"Missing customer/vehicle fields are filled from the saved profile\n" +
			"(see `auth sync-profile`) unless --use-saved-profile=false.\n" +
			"Pass --submit --yes to call saveReservation for real.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if rf.vehicleType != "standard" && rf.vehicleType != "oversize" {
				return fmt.Errorf("--vehicle-type must be standard or oversize")
			}
			if rf.dropoff == "" || rf.pickup == "" {
				return fmt.Errorf("--dropoff and --pickup are required (format \"YYYY-MM-DD HH:MM\")")
			}
			codeID, err := client.ResolveLot(rf.lot)
			if err != nil {
				return err
			}

			// Load config once; used for saved-profile defaults and login.
			// A missing file yields an empty File with no error; only a real
			// read/parse failure surfaces here and must not be swallowed, or
			// saved-profile defaults and configured credentials are silently
			// skipped.
			f, err := g.loadConfig()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			if rf.useSavedProfile && f != nil && f.Profile != nil {
				rf.applySavedProfile(f.Profile)
			}

			summary := map[string]interface{}{
				"lot":          rf.lot,
				"location":     codeID,
				"dropoff":      rf.dropoff,
				"pickup":       rf.pickup,
				"vehicle_type": rf.vehicleType,
				"promo_code":   rf.promoCode,
				"quote":        rf.quoteID,
				"customer": map[string]string{
					"first_name": rf.firstName,
					"last_name":  rf.lastName,
					"email":      rf.email,
					"phone":      rf.phone,
				},
				"vehicle": map[string]string{
					"make":  rf.vehMake,
					"model": rf.vehModel,
					"color": rf.vehColor,
					"plate": rf.plate,
				},
			}

			// Dry-run path (default).
			if !rf.submit {
				summary["mode"] = "dry-run"
				summary["note"] = "no reservation submitted; pass --submit --yes to book"
				if g.json {
					return printJSON(summary)
				}
				printReserveSummary(summary, "DRY-RUN (not submitted)")
				return nil
			}

			// Submit requested: require confirmation.
			if !rf.yes {
				return fmt.Errorf("--submit requires --yes to confirm a real booking")
			}

			// Validate required booking fields before any side effect.
			if missing := rf.missingBookingFields(); len(missing) > 0 {
				return fmt.Errorf("cannot submit reservation: missing required fields: %s",
					strings.Join(missing, ", "))
			}

			// Verifier guard: never call live endpoints under PRINTING_PRESS_VERIFY.
			if IsVerifyEnv() {
				summary["mode"] = "verify-noop"
				summary["verify_noop"] = true
				summary["success"] = false
				if g.json {
					return printJSON(summary)
				}
				printReserveSummary(summary, "VERIFY NO-OP (PRINTING_PRESS_VERIFY=1, not submitted)")
				return nil
			}

			payload := buildSaveReservationPayload(codeID, rf)
			ctx, cancel := g.ctx()
			defer cancel()

			// Authenticate if credentials are available; saveReservation may
			// require a session. A failing --username-command/--password-command
			// (or a configured command ref) must abort instead of silently
			// continuing unauthenticated.
			creds, err := config.Resolve(ctx, f, config.OnePassword{}, rf.creds.input())
			if err != nil {
				return fmt.Errorf("resolve credentials: %w", err)
			}
			c := g.newClient()
			if creds.Username != "" && creds.Password != "" {
				ok, lerr := verifyLoginWithClient(ctx, c, creds.Username, creds.Password, codeID)
				if lerr != nil {
					return fmt.Errorf("login before booking failed: %w", lerr)
				}
				if !ok {
					return fmt.Errorf("login before booking rejected credentials")
				}
			}

			resp, err := c.Ajax(ctx, payload)
			if err != nil {
				return fmt.Errorf("saveReservation failed: %w", err)
			}
			if len(resp.Errors) > 0 {
				return fmt.Errorf("saveReservation returned errors: %v", resp.Errors)
			}
			if g.json {
				return printRawJSON(resp.Data)
			}
			fmt.Println("Reservation submitted.")
			return printRawJSON(resp.Data)
		},
	}

	cmd.Flags().StringVar(&rf.lot, "lot", "", "lot: B, G, or a codeID")
	cmd.Flags().StringVar(&rf.dropoff, "dropoff", "", "drop-off \"YYYY-MM-DD HH:MM\"")
	cmd.Flags().StringVar(&rf.pickup, "pickup", "", "pick-up \"YYYY-MM-DD HH:MM\"")
	cmd.Flags().StringVar(&rf.vehicleType, "vehicle-type", "standard", "standard or oversize")
	cmd.Flags().StringVar(&rf.promoCode, "promo-code", "", "optional promo code")
	cmd.Flags().IntVar(&rf.quoteID, "quote", -1, "selected quote id")
	cmd.Flags().StringVar(&rf.firstName, "first-name", "", "customer first name")
	cmd.Flags().StringVar(&rf.lastName, "last-name", "", "customer last name")
	cmd.Flags().StringVar(&rf.email, "email", "", "customer email")
	cmd.Flags().StringVar(&rf.phone, "phone", "", "customer phone")
	cmd.Flags().StringVar(&rf.vehMake, "vehicle-make", "", "vehicle make")
	cmd.Flags().StringVar(&rf.vehModel, "vehicle-model", "", "vehicle model")
	cmd.Flags().StringVar(&rf.vehColor, "vehicle-color", "", "vehicle color")
	cmd.Flags().StringVar(&rf.plate, "plate", "", "vehicle license plate")
	cmd.Flags().BoolVar(&rf.submit, "submit", false, "actually submit the reservation")
	cmd.Flags().BoolVar(&rf.yes, "yes", false, "confirm a real booking (required with --submit)")
	cmd.Flags().BoolVar(&rf.useSavedProfile, "use-saved-profile", true, "fill missing customer/vehicle fields from the saved profile")
	addCredFlags(cmd, &rf.creds)
	_ = cmd.MarkFlagRequired("lot")
	return cmd
}

// applySavedProfile fills empty customer/vehicle fields from a saved profile.
// Explicit flag values always take precedence (only blanks are filled).
func (rf *reserveFlags) applySavedProfile(p *config.Profile) {
	fill := func(dst *string, src string) {
		if strings.TrimSpace(*dst) == "" && src != "" {
			*dst = src
		}
	}
	fill(&rf.firstName, p.FirstName)
	fill(&rf.lastName, p.LastName)
	fill(&rf.email, p.Email)
	fill(&rf.phone, p.Phone)
	if len(p.Vehicles) > 0 {
		v := p.Vehicles[0]
		fill(&rf.vehMake, v.Make)
		fill(&rf.vehModel, v.Model)
		fill(&rf.vehColor, v.Color)
		fill(&rf.plate, v.License)
	}
}

func (rf *reserveFlags) missingBookingFields() []string {
	required := []struct {
		name string
		val  string
	}{
		{"--first-name", rf.firstName},
		{"--last-name", rf.lastName},
		{"--email", rf.email},
		{"--phone", rf.phone},
		{"--vehicle-make", rf.vehMake},
		{"--vehicle-model", rf.vehModel},
		{"--vehicle-color", rf.vehColor},
		{"--plate", rf.plate},
	}
	var missing []string
	for _, r := range required {
		if strings.TrimSpace(r.val) == "" {
			missing = append(missing, r.name)
		}
	}
	if rf.quoteID < 0 {
		missing = append(missing, "--quote")
	}
	return missing
}

func buildSaveReservationPayload(codeID string, rf *reserveFlags) map[string]interface{} {
	// The website Ge model is flat: customer + vehicle fields live directly on
	// the reservation. A nested vehicle object may also be present but must not
	// replace the flat fields the backend reads.
	reservation := map[string]interface{}{
		"start_date": rf.dropoff,
		"end_date":   rf.pickup,
		"promo_code": rf.promoCode,
		"source":     "website",
		"source_id":  "",
		"quote":      rf.quoteID,
		"services":   []interface{}{},
		"first_name": rf.firstName,
		"last_name":  rf.lastName,
		"email":      rf.email,
		"phone":      rf.phone,
		"license":    rf.plate,
		"make":       rf.vehMake,
		"model":      rf.vehModel,
		"color":      rf.vehColor,
		"vehicle": map[string]interface{}{
			"type":    rf.vehicleType,
			"make":    rf.vehMake,
			"model":   rf.vehModel,
			"color":   rf.vehColor,
			"license": rf.plate,
		},
	}
	return map[string]interface{}{
		"action":      "np_ajax",
		"method":      "saveReservation",
		"reservation": reservation,
		"location":    codeID,
	}
}

func printReserveSummary(s map[string]interface{}, header string) {
	fmt.Printf("=== Reservation %s ===\n", header)
	fmt.Printf("Lot:      %v (%v)\n", s["lot"], s["location"])
	fmt.Printf("Drop-off: %v\n", s["dropoff"])
	fmt.Printf("Pick-up:  %v\n", s["pickup"])
	fmt.Printf("Vehicle:  %v\n", s["vehicle_type"])
	fmt.Printf("Quote:    %v\n", s["quote"])
	if cust, ok := s["customer"].(map[string]string); ok {
		fmt.Printf("Customer: %s %s <%s> %s\n", cust["first_name"], cust["last_name"], cust["email"], cust["phone"])
	}
	if note, ok := s["note"]; ok {
		fmt.Printf("Note:     %v\n", note)
	}
}
