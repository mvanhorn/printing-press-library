// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package mcp

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/store"
)

// archiveShipment records a shipment created through MCP in the local ledger
// so archive, spend_report, manifest, and track_diff see it, the same way the
// CLI's ship bulk does. Only operational fields are stored; the raw response
// and encoded labels are not. Failures are reported but never undo the
// FedEx-side result.
func archiveShipment(ctx context.Context, request map[string]any, response json.RawMessage) error {
	shipment := shipmentFromExchange(request, response)
	if shipment.TrackingNumber == "" {
		return nil
	}
	st, err := store.Open("")
	if err != nil {
		return err
	}
	defer st.Close()
	_, err = st.InsertShipment(ctx, shipment)
	return err
}

func shipmentFromExchange(request map[string]any, response json.RawMessage) store.Shipment {
	var resp struct {
		Output struct {
			TransactionShipments []struct {
				MasterTrackingNumber string `json:"masterTrackingNumber"`
				ServiceType          string `json:"serviceType"`
				PieceResponses       []struct {
					TrackingNumber string  `json:"trackingNumber"`
					NetCharge      float64 `json:"netCharge"`
					Currency       string  `json:"currency"`
				} `json:"pieceResponses"`
			} `json:"transactionShipments"`
		} `json:"output"`
	}
	_ = json.Unmarshal(response, &resp)

	var sp store.Shipment
	if len(resp.Output.TransactionShipments) > 0 {
		ts := resp.Output.TransactionShipments[0]
		sp.MasterTrackingNumber = ts.MasterTrackingNumber
		sp.ServiceType = ts.ServiceType
		if len(ts.PieceResponses) > 0 {
			piece := ts.PieceResponses[0]
			sp.TrackingNumber = piece.TrackingNumber
			sp.NetChargeAmount = piece.NetCharge
			sp.NetChargeCurrency = piece.Currency
		}
		if sp.TrackingNumber == "" {
			sp.TrackingNumber = ts.MasterTrackingNumber
		}
	}

	sp.Account = str(obj(request, "accountNumber"), "value")
	shipment := obj(request, "requestedShipment")
	if sp.ServiceType == "" {
		sp.ServiceType = str(shipment, "serviceType")
	}
	sp.PackagingType = str(shipment, "packagingType")

	shipper := obj(shipment, "shipper")
	sp.ShipperName = contactName(obj(shipper, "contact"))
	sp.ShipperPostal = str(obj(shipper, "address"), "postalCode")
	sp.ShipperCountry = str(obj(shipper, "address"), "countryCode")

	if recipients, _ := shipment["recipients"].([]any); len(recipients) > 0 {
		recipient, _ := recipients[0].(map[string]any)
		address := obj(recipient, "address")
		sp.RecipientName = contactName(obj(recipient, "contact"))
		if lines, _ := address["streetLines"].([]any); len(lines) > 0 {
			parts := make([]string, 0, len(lines))
			for _, line := range lines {
				if text, ok := line.(string); ok && strings.TrimSpace(text) != "" {
					parts = append(parts, strings.TrimSpace(text))
				}
			}
			sp.RecipientAddress = strings.Join(parts, ", ")
		}
		sp.RecipientCity = str(address, "city")
		sp.RecipientState = str(address, "stateOrProvinceCode")
		sp.RecipientPostal = str(address, "postalCode")
		sp.RecipientCountry = str(address, "countryCode")
	}

	if packages, _ := shipment["requestedPackageLineItems"].([]any); len(packages) > 0 {
		first, _ := packages[0].(map[string]any)
		weight := obj(first, "weight")
		sp.WeightUnits = str(weight, "units")
		switch value := weight["value"].(type) {
		case float64:
			sp.WeightValue = value
		case string:
			sp.WeightValue, _ = strconv.ParseFloat(value, 64)
		}
		if refs, _ := first["customerReferences"].([]any); len(refs) > 0 {
			ref, _ := refs[0].(map[string]any)
			sp.Reference = str(ref, "value")
		}
	}
	return sp
}

func obj(value map[string]any, key string) map[string]any {
	child, _ := value[key].(map[string]any)
	if child == nil {
		return map[string]any{}
	}
	return child
}

func str(value map[string]any, key string) string {
	text, _ := value[key].(string)
	return strings.TrimSpace(text)
}

func contactName(contact map[string]any) string {
	if name := str(contact, "personName"); name != "" {
		return name
	}
	return str(contact, "companyName")
}
