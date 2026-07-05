// Copyright 2026 jimpresting. Licensed under Apache-2.0. See LICENSE.

package airbnb

import (
	"encoding/json"
	"fmt"
)

// Contact starts a conversation with a listing's host by sending an inquiry —
// the "contact host" flow. Airbnb requires trip dates + guest count for an
// inquiry, so Checkin/Checkout/Adults are required. Returns the created
// contact-host response. The variables shape is the one the web contact-host
// form sends (SendContactHostMessageMutation), captured from live traffic.
func (c *Client) Contact(p ContactParams) (json.RawMessage, error) {
	if p.Message == "" {
		return nil, fmt.Errorf("contact requires a message")
	}
	if p.Checkin == "" || p.Checkout == "" {
		return nil, fmt.Errorf("contact requires --checkin and --checkout (Airbnb host inquiries need trip dates)")
	}
	adults := p.Adults
	if adults <= 0 {
		adults = 1
	}
	vars := map[string]any{
		"input": map[string]any{
			"checkIn":       p.Checkin,
			"checkOut":      p.Checkout,
			"adults":        adults,
			"stayListingId": EncodeGlobalID("StayListing", NumericID(p.ListingID)),
			"message":       p.Message,
		},
		"contactHostSectionsRequest": map[string]any{
			"checkIn":  p.Checkin,
			"checkOut": p.Checkout,
			"adults":   adults,
			"layouts":  []string{"SIDEBAR", "SINGLE_COLUMN"},
		},
	}
	return c.Mutation("SendContactHostMessageMutation", vars)
}

// ContactParams describes a new-conversation inquiry to a host.
type ContactParams struct {
	ListingID string
	Message   string
	Checkin   string
	Checkout  string
	Adults    int
}
