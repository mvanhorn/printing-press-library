// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func newShipEtdCmd(flags *rootFlags) *cobra.Command {
	var (
		invoicePath     string
		origCountry     string
		destCountry     string
		recipientName   string
		recipientStreet string
		recipientCity   string
		recipientState  string
		recipientPostal string
		weightArg       string
		units           string
		serviceType     string
		account         string
		customsValue    float64
		currency        string
	)
	cmd := &cobra.Command{
		Use:   "etd",
		Short: "International ETD ship: embed commercial invoice + customs details into a single shipment call",
		Long: `Single-command Electronic Trade Documents shipping. Reads a commercial-invoice PDF
from disk, base64-encodes it into the shipment request as an embedded ETD document,
and creates the international shipment in one call. Note: this is the simplified flow —
the upstream FedEx documents/upload endpoint is not bundled in this spec, so the invoice
is embedded inline rather than uploaded separately.`,
		Example: strings.Trim(`
  fedex-pp-cli ship etd --invoice ./invoice.pdf --orig US --dest CA --weight 2 --customs-value 150
  fedex-pp-cli ship etd --invoice ./invoice.pdf --orig CN --dest US --weight 2kg --customs-value 250
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			weight, parsedUnits, err := parseWeightArg(weightArg, units)
			if err != nil {
				return usageErr(err)
			}
			if parsedUnits != "" {
				units = parsedUnits
			}
			if invoicePath == "" || origCountry == "" || destCountry == "" || weight <= 0 || customsValue <= 0 {
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

			invoiceBytes, err := os.ReadFile(invoicePath)
			if err != nil {
				return fmt.Errorf("reading invoice %s: %w", invoicePath, err)
			}
			encoded := base64.StdEncoding.EncodeToString(invoiceBytes)

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			body := map[string]any{
				"labelResponseOptions": "LABEL",
				"accountNumber":        map[string]any{"value": account},
				"requestedShipment": map[string]any{
					"shipper": map[string]any{
						"address": map[string]any{"countryCode": origCountry},
					},
					"recipients": []any{
						map[string]any{
							"contact": map[string]any{"personName": recipientName},
							"address": map[string]any{
								"streetLines":         []string{recipientStreet},
								"city":                recipientCity,
								"stateOrProvinceCode": recipientState,
								"postalCode":          recipientPostal,
								"countryCode":         destCountry,
							},
						},
					},
					"serviceType":            serviceType,
					"packagingType":          "YOUR_PACKAGING",
					"shippingChargesPayment": map[string]any{"paymentType": "SENDER"},
					"customsClearanceDetail": map[string]any{
						"dutiesPayment": map[string]any{"paymentType": "SENDER"},
						"commercialInvoice": map[string]any{
							"originatorName":                 "Shipper",
							"customerReferences":             []any{},
							"taxesOrMiscellaneousChargeType": "OTHER",
						},
						"customsValue": map[string]any{
							"amount":   customsValue,
							"currency": currency,
						},
						"commodities": []any{
							map[string]any{
								"description":          "Goods",
								"countryOfManufacture": origCountry,
								"weight":               map[string]any{"units": units, "value": weight},
								"quantity":             1,
								"quantityUnits":        "PCS",
								"unitPrice":            map[string]any{"amount": customsValue, "currency": currency},
								"customsValue":         map[string]any{"amount": customsValue, "currency": currency},
							},
						},
					},
					"shippingDocumentSpecification": map[string]any{
						"shippingDocumentTypes": []string{"COMMERCIAL_INVOICE"},
						"commercialInvoiceDetail": map[string]any{
							"documentFormat": map[string]any{"docType": "PDF"},
							"customerImageUsages": []any{
								map[string]any{
									"id":                "IMAGE_1",
									"type":              "LETTER_HEAD",
									"providedImageType": "LETTER_HEAD",
								},
							},
						},
					},
					"requestedPackageLineItems": []any{
						map[string]any{
							"weight": map[string]any{"units": units, "value": weight},
						},
					},
					// Embed the invoice as a customer-provided document so the ETD
					// flow does not require a separate documents/upload call.
					"etdDetail": map[string]any{
						"attachedDocuments": []any{
							map[string]any{
								"documentType":         "COMMERCIAL_INVOICE",
								"encodedDocumentBytes": encoded,
								"documentFormat":       "PDF",
							},
						},
					},
				},
			}

			data, _, err := c.Post("/ship/v1/shipments", body)
			if err != nil {
				return classifyAPIError(err)
			}
			tracking, net, ccy, _ := parseShipResponse(data)
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"tracking_number": tracking,
				"net_charge":      net,
				"currency":        ccy,
				"orig":            origCountry,
				"dest":            destCountry,
				"customs_value":   customsValue,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&invoicePath, "invoice", "", "Commercial invoice PDF path")
	cmd.Flags().StringVar(&origCountry, "orig", "", "Origin country code (ISO)")
	cmd.Flags().StringVar(&destCountry, "dest", "", "Destination country code (ISO)")
	cmd.Flags().StringVar(&recipientName, "recipient-name", "", "Recipient contact name")
	cmd.Flags().StringVar(&recipientStreet, "recipient-street", "", "Recipient street")
	cmd.Flags().StringVar(&recipientCity, "recipient-city", "", "Recipient city")
	cmd.Flags().StringVar(&recipientState, "recipient-state", "", "Recipient state code")
	cmd.Flags().StringVar(&recipientPostal, "recipient-postal", "", "Recipient postal")
	cmd.Flags().StringVar(&weightArg, "weight", "", "Package weight (e.g. 2, 2kg, 5lb). When the value carries a unit suffix it overrides --units.")
	cmd.Flags().StringVar(&units, "units", "KG", "Weight units")
	cmd.Flags().StringVar(&serviceType, "service", "FEDEX_INTERNATIONAL_PRIORITY", "Service type")
	cmd.Flags().StringVar(&account, "account", "", "FedEx account (defaults to env FEDEX_ACCOUNT_NUMBER)")
	cmd.Flags().Float64Var(&customsValue, "customs-value", 0, "Customs declared value")
	cmd.Flags().StringVar(&currency, "currency", "USD", "Currency for customs and unit price")
	return cmd
}
