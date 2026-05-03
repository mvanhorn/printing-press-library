// Copyright 2026 trevin-chow. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/store"

	"github.com/spf13/cobra"
)

func newShipCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "ship",
		Short: "Bulk and ETD shipping helpers backed by the local archive",
	}
	cmd.AddCommand(newShipBulkCmd(flags))
	cmd.AddCommand(newShipEtdCmd(flags))
	return cmd
}

type shipBulkRow struct {
	OrderID        string
	RecipientName  string
	RecipientCo    string
	RecipientPhone string
	RecipientEmail string
	Street         string
	Street2        string
	City           string
	State          string
	Postal         string
	Country        string
	WeightValue    float64
	WeightUnits    string
	Reference      string
	ServiceType    string
	ToSaved        string
}

type shipBulkResult struct {
	OrderID        string  `json:"order_id"`
	Status         string  `json:"status"`
	TrackingNumber string  `json:"tracking_number,omitempty"`
	NetCharge      float64 `json:"net_charge,omitempty"`
	Currency       string  `json:"currency,omitempty"`
	LabelPath      string  `json:"label_path,omitempty"`
	Error          string  `json:"error,omitempty"`
}

func newShipBulkCmd(flags *rootFlags) *cobra.Command {
	var (
		csvPath        string
		defaultService string
		account        string
		fromName       string
		fromStreet     string
		fromCity       string
		fromState      string
		fromPostal     string
		fromCountry    string
		fromPhone      string
		outputDir      string
		resume         bool
		concurrency    int
		ledgerPath     string
	)

	cmd := &cobra.Command{
		Use:   "bulk",
		Short: "Ship a CSV of orders, write per-row PASS/FAIL ledger, persist successes to the archive",
		Long: `Read a CSV of orders, ship each row through FedEx, write per-row results to a ledger
CSV (PASS/FAIL plus tracking number and net charge), and persist successful shipments
to the local archive. Optionally save labels as PDFs. Resumable via --resume which skips
rows whose order_id is already in the archive (matched on the reference column).`,
		Example: strings.Trim(`
  fedex-pp-cli ship bulk --csv orders.csv --service FEDEX_GROUND --output ./labels
  fedex-pp-cli ship bulk --csv orders.csv --resume --concurrency 3
`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if csvPath == "" {
				return cmd.Help()
			}
			if account == "" {
				account = os.Getenv("FEDEX_ACCOUNT_NUMBER")
			}
			if account == "" && !flags.dryRun {
				return usageErr(fmt.Errorf("--account is required (or set FEDEX_ACCOUNT_NUMBER)"))
			}
			if fromName == "" {
				fromName = os.Getenv("FEDEX_SHIPPER_NAME")
			}
			if fromStreet == "" {
				fromStreet = os.Getenv("FEDEX_SHIPPER_STREET")
			}
			if fromCity == "" {
				fromCity = os.Getenv("FEDEX_SHIPPER_CITY")
			}
			if fromState == "" {
				fromState = os.Getenv("FEDEX_SHIPPER_STATE")
			}
			if fromPostal == "" {
				fromPostal = os.Getenv("FEDEX_SHIPPER_POSTAL")
			}
			if fromCountry == "" {
				fromCountry = os.Getenv("FEDEX_SHIPPER_COUNTRY")
				if fromCountry == "" {
					fromCountry = "US"
				}
			}
			if fromPhone == "" {
				fromPhone = os.Getenv("FEDEX_SHIPPER_PHONE")
			}
			if dryRunOK(flags) {
				return nil
			}

			if concurrency < 1 {
				concurrency = 1
			}
			if concurrency > 5 {
				concurrency = 5
			}
			if ledgerPath == "" {
				base := strings.TrimSuffix(csvPath, filepath.Ext(csvPath))
				ledgerPath = base + ".results.csv"
			}

			rows, err := readShipBulkCSV(csvPath)
			if err != nil {
				return err
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			ctx := context.Background()
			st, _ := store.Open("")
			if st != nil {
				defer st.Close()
			}

			results := make([]shipBulkResult, len(rows))
			sem := make(chan struct{}, concurrency)
			var wg sync.WaitGroup
			var mu sync.Mutex

			for i, row := range rows {
				if resume && st != nil && row.Reference != "" {
					if exists, _ := shipmentExistsByReference(ctx, st, row.Reference); exists {
						results[i] = shipBulkResult{OrderID: row.OrderID, Status: "SKIP", Error: "already shipped"}
						continue
					}
				}

				if row.ToSaved != "" && st != nil {
					if a, _ := st.GetAddress(ctx, row.ToSaved); a != nil {
						row = applySavedAddress(row, *a)
					}
				}
				if row.ServiceType == "" {
					row.ServiceType = defaultService
				}
				if row.WeightUnits == "" {
					row.WeightUnits = "LB"
				}

				wg.Add(1)
				sem <- struct{}{}
				go func(idx int, r shipBulkRow) {
					defer wg.Done()
					defer func() { <-sem }()
					res := shipOneRow(ctx, c, st, r, account, outputDir, fromName, fromStreet, fromCity, fromState, fromPostal, fromCountry, fromPhone)
					mu.Lock()
					results[idx] = res
					mu.Unlock()
				}(i, row)
			}
			wg.Wait()

			if err := writeShipBulkLedger(ledgerPath, results); err != nil {
				fmt.Fprintf(os.Stderr, "warning: writing ledger %s failed: %v\n", ledgerPath, err)
			}

			summary := summarizeShipBulk(results)
			summary["ledger"] = ledgerPath
			return printJSONFiltered(cmd.OutOrStdout(), summary, flags)
		},
	}

	cmd.Flags().StringVar(&csvPath, "csv", "", "Path to orders CSV")
	cmd.Flags().StringVar(&defaultService, "service", "FEDEX_GROUND", "Default service type when row's service_type is empty")
	cmd.Flags().StringVar(&account, "account", "", "FedEx account number (defaults to env FEDEX_ACCOUNT_NUMBER)")
	cmd.Flags().StringVar(&fromName, "from-name", "", "Shipper contact name (defaults to env FEDEX_SHIPPER_NAME)")
	cmd.Flags().StringVar(&fromStreet, "from-street", "", "Shipper street (defaults to env FEDEX_SHIPPER_STREET)")
	cmd.Flags().StringVar(&fromCity, "from-city", "", "Shipper city (defaults to env FEDEX_SHIPPER_CITY)")
	cmd.Flags().StringVar(&fromState, "from-state", "", "Shipper state code (defaults to env FEDEX_SHIPPER_STATE)")
	cmd.Flags().StringVar(&fromPostal, "from-postal", "", "Shipper postal (defaults to env FEDEX_SHIPPER_POSTAL)")
	cmd.Flags().StringVar(&fromCountry, "from-country", "", "Shipper country code (defaults to env FEDEX_SHIPPER_COUNTRY or US)")
	cmd.Flags().StringVar(&fromPhone, "from-phone", "", "Shipper phone (defaults to env FEDEX_SHIPPER_PHONE)")
	cmd.Flags().StringVar(&outputDir, "output", "", "Directory to write label PDFs (none if empty)")
	cmd.Flags().BoolVar(&resume, "resume", false, "Skip rows already shipped (matched by reference)")
	cmd.Flags().IntVar(&concurrency, "concurrency", 1, "Parallel ship calls (1-5)")
	cmd.Flags().StringVar(&ledgerPath, "ledger", "", "Per-row PASS/FAIL ledger CSV (default <csv-basename>.results.csv)")
	return cmd
}

func readShipBulkCSV(path string) ([]shipBulkRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("%s: need header + at least one row", path)
	}
	header := records[0]
	idx := map[string]int{}
	for i, h := range header {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	get := func(rec []string, key string) string {
		if i, ok := idx[key]; ok && i < len(rec) {
			return strings.TrimSpace(rec[i])
		}
		return ""
	}
	out := make([]shipBulkRow, 0, len(records)-1)
	for _, rec := range records[1:] {
		w, _ := strconv.ParseFloat(get(rec, "weight_value"), 64)
		row := shipBulkRow{
			OrderID:        get(rec, "order_id"),
			RecipientName:  get(rec, "recipient_name"),
			RecipientCo:    get(rec, "recipient_company"),
			RecipientPhone: get(rec, "recipient_phone"),
			RecipientEmail: get(rec, "recipient_email"),
			Street:         get(rec, "street"),
			Street2:        get(rec, "street2"),
			City:           get(rec, "city"),
			State:          get(rec, "state"),
			Postal:         get(rec, "postal"),
			Country:        get(rec, "country"),
			WeightValue:    w,
			WeightUnits:    get(rec, "weight_units"),
			Reference:      get(rec, "reference"),
			ServiceType:    get(rec, "service_type"),
			ToSaved:        get(rec, "to_saved"),
		}
		if row.Country == "" {
			row.Country = "US"
		}
		out = append(out, row)
	}
	return out, nil
}

// shipBulkRow.Reference is exposed via the Reference field; this method exists
// purely so resume/exists checks compile against the same type the parser
// returns.
func (r shipBulkRow) ReferenceValue() string { return r.Reference }

func applySavedAddress(row shipBulkRow, a store.Address) shipBulkRow {
	row.RecipientName = a.ContactName
	if row.RecipientName == "" {
		row.RecipientName = a.Name
	}
	row.RecipientCo = a.Company
	row.RecipientPhone = a.Phone
	row.RecipientEmail = a.Email
	row.Street = a.Street
	row.Street2 = a.Street2
	row.City = a.City
	row.State = a.State
	row.Postal = a.Postal
	row.Country = a.Country
	return row
}

// Reference is not part of the original CSV header set; it's pulled from the
// optional `reference` column in readShipBulkCSV.
func shipmentExistsByReference(ctx context.Context, st *store.Store, ref string) (bool, error) {
	row := st.DB().QueryRowContext(ctx, `SELECT 1 FROM shipments WHERE reference = ? LIMIT 1`, ref)
	var x int
	if err := row.Scan(&x); err != nil {
		return false, nil
	}
	return true, nil
}

func shipOneRow(ctx context.Context, c clientShipper, st *store.Store, row shipBulkRow, account, outputDir,
	fromName, fromStreet, fromCity, fromState, fromPostal, fromCountry, fromPhone string,
) shipBulkResult {
	body := buildShipBody(row, account, fromName, fromStreet, fromCity, fromState, fromPostal, fromCountry, fromPhone)
	data, _, err := c.Post("/ship/v1/shipments", body)
	if err != nil {
		return shipBulkResult{OrderID: row.OrderID, Status: "FAIL", Error: err.Error()}
	}
	tracking, net, ccy, label := parseShipResponse(data)
	labelPath := ""
	if outputDir != "" && label != "" {
		labelPath = writeLabelPDF(outputDir, tracking, label)
	}
	if st != nil && tracking != "" {
		_, _ = st.InsertShipment(ctx, store.Shipment{
			TrackingNumber:    tracking,
			Account:           account,
			ServiceType:       row.ServiceType,
			ShipperName:       fromName,
			ShipperPostal:     fromPostal,
			ShipperCountry:    fromCountry,
			RecipientName:     row.RecipientName,
			RecipientAddress:  row.Street,
			RecipientCity:     row.City,
			RecipientState:    row.State,
			RecipientPostal:   row.Postal,
			RecipientCountry:  row.Country,
			WeightValue:       row.WeightValue,
			WeightUnits:       row.WeightUnits,
			Reference:         row.Reference,
			NetChargeAmount:   net,
			NetChargeCurrency: ccy,
			LabelPath:         labelPath,
			RawResponse:       string(data),
		})
	}
	return shipBulkResult{
		OrderID:        row.OrderID,
		Status:         "PASS",
		TrackingNumber: tracking,
		NetCharge:      net,
		Currency:       ccy,
		LabelPath:      labelPath,
	}
}

// clientShipper is the subset of *client.Client we need; declaring it as an
// interface keeps the helper testable without standing up the full HTTP client.
type clientShipper interface {
	Post(path string, body any) (json.RawMessage, int, error)
}

func buildShipBody(row shipBulkRow, account, fromName, fromStreet, fromCity, fromState, fromPostal, fromCountry, fromPhone string) map[string]any {
	streetLines := []string{row.Street}
	if row.Street2 != "" {
		streetLines = append(streetLines, row.Street2)
	}
	return map[string]any{
		"labelResponseOptions": "LABEL",
		"accountNumber":        map[string]any{"value": account},
		"requestedShipment": map[string]any{
			"shipper": map[string]any{
				"contact": map[string]any{
					"personName":  fromName,
					"phoneNumber": fromPhone,
				},
				"address": map[string]any{
					"streetLines":         []string{fromStreet},
					"city":                fromCity,
					"stateOrProvinceCode": fromState,
					"postalCode":          fromPostal,
					"countryCode":         fromCountry,
				},
			},
			"recipients": []any{
				map[string]any{
					"contact": map[string]any{
						"personName":   row.RecipientName,
						"companyName":  row.RecipientCo,
						"phoneNumber":  row.RecipientPhone,
						"emailAddress": row.RecipientEmail,
					},
					"address": map[string]any{
						"streetLines":         streetLines,
						"city":                row.City,
						"stateOrProvinceCode": row.State,
						"postalCode":          row.Postal,
						"countryCode":         row.Country,
					},
				},
			},
			"shipDatestamp": "",
			"serviceType":   row.ServiceType,
			"packagingType": "YOUR_PACKAGING",
			"pickupType":    "USE_SCHEDULED_PICKUP",
			"shippingChargesPayment": map[string]any{
				"paymentType": "SENDER",
			},
			"labelSpecification": map[string]any{
				"imageType":      "PDF",
				"labelStockType": "PAPER_85X11_TOP_HALF_LABEL",
			},
			"requestedPackageLineItems": []any{
				map[string]any{
					"weight": map[string]any{
						"units": row.WeightUnits,
						"value": row.WeightValue,
					},
					"customerReferences": []any{
						map[string]any{
							"customerReferenceType": "CUSTOMER_REFERENCE",
							"value":                 row.Reference,
						},
					},
				},
			},
		},
	}
}

func parseShipResponse(data json.RawMessage) (tracking string, net float64, ccy string, encodedLabel string) {
	var resp struct {
		Output struct {
			TransactionShipments []struct {
				MasterTrackingNumber string `json:"masterTrackingNumber"`
				PieceResponses       []struct {
					TrackingNumber   string  `json:"trackingNumber"`
					NetCharge        float64 `json:"netCharge"`
					BaseCharge       float64 `json:"baseCharge"`
					Currency         string  `json:"currency"`
					PackageDocuments []struct {
						EncodedLabel string `json:"encodedLabel"`
						URL          string `json:"url"`
					} `json:"packageDocuments"`
				} `json:"pieceResponses"`
			} `json:"transactionShipments"`
		} `json:"output"`
	}
	_ = json.Unmarshal(data, &resp)
	if len(resp.Output.TransactionShipments) == 0 {
		return
	}
	ts := resp.Output.TransactionShipments[0]
	if len(ts.PieceResponses) > 0 {
		pr := ts.PieceResponses[0]
		tracking = pr.TrackingNumber
		net = pr.NetCharge
		ccy = pr.Currency
		if len(pr.PackageDocuments) > 0 {
			encodedLabel = pr.PackageDocuments[0].EncodedLabel
		}
	}
	if tracking == "" {
		tracking = ts.MasterTrackingNumber
	}
	return
}

func writeLabelPDF(dir, tracking, encoded string) string {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ""
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return ""
	}
	name := tracking
	if name == "" {
		name = "label"
	}
	path := filepath.Join(dir, name+".pdf")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		return ""
	}
	return path
}

func writeShipBulkLedger(path string, results []shipBulkResult) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write([]string{"order_id", "status", "tracking_number", "net_charge", "currency", "label_path", "error"}); err != nil {
		return err
	}
	for _, r := range results {
		if err := w.Write([]string{
			r.OrderID, r.Status, r.TrackingNumber,
			fmt.Sprintf("%.2f", r.NetCharge), r.Currency, r.LabelPath, r.Error,
		}); err != nil {
			return err
		}
	}
	return nil
}

func summarizeShipBulk(results []shipBulkResult) map[string]any {
	pass, fail, skip := 0, 0, 0
	for _, r := range results {
		switch r.Status {
		case "PASS":
			pass++
		case "FAIL":
			fail++
		case "SKIP":
			skip++
		}
	}
	return map[string]any{
		"total":   len(results),
		"passed":  pass,
		"failed":  fail,
		"skipped": skip,
		"results": results,
	}
}
