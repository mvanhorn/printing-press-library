// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/store"

	"github.com/spf13/cobra"
)

func newReturnCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "return",
		Short: "Generate Ground Call Tag (return label) shipments",
	}
	cmd.AddCommand(newReturnCreateCmd(flags))
	return cmd
}

func newReturnCreateCmd(flags *rootFlags) *cobra.Command {
	var (
		tracking      string
		reason        string
		account       string
		email         string
		fromSaved     string
		pickupName    string
		pickupStreet  string
		pickupCity    string
		pickupState   string
		pickupPostal  string
		pickupCountry string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a Ground Call Tag (return label) referencing an existing shipment",
		Example: strings.Trim(`
  fedex-pp-cli return create --tracking 794633071234 --reason "wrong size" --from-saved customer-jane
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if tracking == "" {
				return cmd.Help()
			}
			if account == "" {
				account = os.Getenv("FEDEX_ACCOUNT_NUMBER")
			}
			if account == "" && !flags.dryRun {
				return usageErr(fmt.Errorf("--account is required (or set FEDEX_ACCOUNT_NUMBER)"))
			}
			if pickupCountry == "" {
				pickupCountry = "US"
			}
			if dryRunOK(flags) {
				return nil
			}

			if fromSaved != "" {
				st, err := store.Open("")
				if err == nil {
					defer st.Close()
					a, _ := st.GetAddress(context.Background(), fromSaved)
					if a != nil {
						pickupName = a.ContactName
						if pickupName == "" {
							pickupName = a.Name
						}
						pickupStreet = a.Street
						pickupCity = a.City
						pickupState = a.State
						pickupPostal = a.Postal
						pickupCountry = a.Country
					}
				}
			}
			if pickupStreet == "" || pickupCity == "" || pickupPostal == "" {
				return usageErr(fmt.Errorf("pickup address required (use --from-saved or --pickup-* flags)"))
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body := map[string]any{
				"accountNumber": map[string]any{"value": account},
				"requestedShipment": map[string]any{
					"shipper": map[string]any{
						"contact": map[string]any{
							"personName":   pickupName,
							"emailAddress": email,
						},
						"address": map[string]any{
							"streetLines":         []string{pickupStreet},
							"city":                pickupCity,
							"stateOrProvinceCode": pickupState,
							"postalCode":          pickupPostal,
							"countryCode":         pickupCountry,
						},
					},
					"pickupDetail": map[string]any{
						"packageLocation": "FRONT",
						"buildingPart":    "",
						"requestType":     "FUTURE_DAY",
					},
					"originalTrackingNumber": tracking,
					"returnReason":           reason,
					"shipmentType":           "RETURN",
					"serviceType":            "FEDEX_GROUND",
					"packagingType":          "YOUR_PACKAGING",
					"requestedPackageLineItems": []any{
						map[string]any{
							"weight": map[string]any{"units": "LB", "value": 1},
						},
					},
				},
			}
			data, _, err := c.Post("/ship/v1/shipments/tag", body)
			if err != nil {
				return classifyAPIError(err)
			}
			result := parseReturnCreateResponse(data)
			result["original_tracking"] = tracking
			result["reason"] = reason
			return printJSONFiltered(cmd.OutOrStdout(), result, flags)
		},
	}
	cmd.Flags().StringVar(&tracking, "tracking", "", "Original tracking number to reference")
	cmd.Flags().StringVar(&reason, "reason", "", "Return reason note")
	cmd.Flags().StringVar(&account, "account", "", "FedEx account number (defaults to env FEDEX_ACCOUNT_NUMBER)")
	cmd.Flags().StringVar(&email, "email", "", "Notification email for the recipient")
	cmd.Flags().StringVar(&fromSaved, "from-saved", "", "Pull pickup address from address book by name")
	cmd.Flags().StringVar(&pickupName, "pickup-name", "", "Pickup contact name")
	cmd.Flags().StringVar(&pickupStreet, "pickup-street", "", "Pickup street")
	cmd.Flags().StringVar(&pickupCity, "pickup-city", "", "Pickup city")
	cmd.Flags().StringVar(&pickupState, "pickup-state", "", "Pickup state code")
	cmd.Flags().StringVar(&pickupPostal, "pickup-postal", "", "Pickup postal")
	cmd.Flags().StringVar(&pickupCountry, "pickup-country", "US", "Pickup country code")
	return cmd
}

func parseReturnCreateResponse(data json.RawMessage) map[string]any {
	out := map[string]any{}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err == nil {
		out["raw"] = raw
	}
	var resp struct {
		Output struct {
			TransactionShipments []struct {
				MasterTrackingNumber string `json:"masterTrackingNumber"`
				PieceResponses       []struct {
					TrackingNumber string `json:"trackingNumber"`
				} `json:"pieceResponses"`
			} `json:"transactionShipments"`
		} `json:"output"`
	}
	_ = json.Unmarshal(data, &resp)
	if len(resp.Output.TransactionShipments) > 0 {
		ts := resp.Output.TransactionShipments[0]
		out["call_tag_tracking_number"] = ts.MasterTrackingNumber
		if len(ts.PieceResponses) > 0 && ts.PieceResponses[0].TrackingNumber != "" {
			out["call_tag_tracking_number"] = ts.PieceResponses[0].TrackingNumber
		}
	}
	return out
}
