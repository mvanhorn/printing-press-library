package parser

import (
	"strings"
)

// ShipTrack is the parsed /gp/your-account/ship-track page.
type ShipTrack struct {
	OrderID         string `json:"orderId,omitempty"`
	Status          string `json:"status,omitempty"`      // "Out for delivery", "Delivered", "In transit", "Arriving"
	ETADate         string `json:"etaDate,omitempty"`     // ISO YYYY-MM-DD
	DeliveredOn     string `json:"deliveredOn,omitempty"` // ISO YYYY-MM-DD
	Carrier         string `json:"carrier,omitempty"`     // "UPS", "USPS", "Amazon Logistics", "FedEx"
	TrackingNumber  string `json:"trackingNumber,omitempty"`
	CarrierTrackURL string `json:"carrierTrackUrl,omitempty"`
	LastUpdate      string `json:"lastUpdate,omitempty"` // free-form latest event description
}

// ParseShipTrack parses a ship-track HTML page.
func ParseShipTrack(htmlBytes []byte) (*ShipTrack, error) {
	doc, err := Parse(htmlBytes)
	if err != nil {
		return nil, err
	}
	t := &ShipTrack{}
	docText := Text(doc)

	t.OrderID = ExtractOrderID(docText)
	t.Status, t.ETADate, t.DeliveredOn = extractStatus(docText)

	// Carrier: pick out a known carrier name when adjacent to "Tracking ID" or "Carrier" labels.
	knownCarriers := []string{"Amazon Logistics", "AMZL", "UPS", "USPS", "FedEx", "OnTrac", "DHL", "LaserShip"}
	for _, c := range knownCarriers {
		if strings.Contains(docText, c) {
			t.Carrier = c
			break
		}
	}

	// Tracking ID extraction: look for "Tracking ID" / "Tracking Number" labels.
	for _, label := range []string{"Tracking ID", "Tracking Number", "Tracking number"} {
		if i := strings.Index(docText, label); i >= 0 {
			window := docText[i+len(label) : min(i+len(label)+60, len(docText))]
			window = strings.TrimSpace(strings.TrimLeft(window, ":"))
			// First whitespace-bounded token of length >= 6 is the tracking number.
			parts := strings.Fields(window)
			for _, p := range parts {
				p = strings.TrimSpace(strings.Trim(p, "()[]"))
				if len(p) >= 6 && hasAlnum(p) {
					t.TrackingNumber = p
					break
				}
			}
			if t.TrackingNumber != "" {
				break
			}
		}
	}

	// Last update: a "label" near words like "last seen", "in transit since", or "ordered" can be useful.
	for _, sentinel := range []string{"Last update", "Latest update", "Update"} {
		if i := strings.Index(docText, sentinel); i >= 0 {
			window := docText[i:min(i+200, len(docText))]
			// Cut at the next double-space or known section header.
			for _, end := range []string{"  ", "Items in", "Shipping address"} {
				if j := strings.Index(window, end); j > 0 {
					window = window[:j]
					break
				}
			}
			t.LastUpdate = strings.TrimSpace(window)
			if t.LastUpdate != "" {
				break
			}
		}
	}

	return t, nil
}

func hasAlnum(s string) bool {
	for _, r := range s {
		if (r >= '0' && r <= '9') || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			return true
		}
	}
	return false
}
