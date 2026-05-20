// Copyright 2026 pejman-pour-moezzi. Licensed under Apache-2.0. See LICENSE.

// PATCH: resy-source-port — see .printing-press-patches.json for the change-set rationale.

package resy

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
)

// detailsResponse is the subset of /3/details we care about.
type detailsResponse struct {
	BookToken *struct {
		Value string `json:"value"`
	} `json:"book_token"`
	User *struct {
		PaymentMethods []paymentMethod `json:"payment_methods"`
	} `json:"user"`
	// Error envelopes seen in real responses when the slot has been taken
	// since the availability call — these tend to come back with a 200
	// status carrying an error field instead of a 410.
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
	Message string `json:"message"`
}

type paymentMethod struct {
	ID        json.RawMessage `json:"id"`
	IsDefault bool            `json:"is_default"`
}

// confirmResponse is the subset of /3/book we care about.
type confirmResponse struct {
	ResyToken     string          `json:"resy_token"`
	ReservationID json.RawMessage `json:"reservation_id"`
	Message       string          `json:"message"`
	Error         *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// Book performs Resy's two-step book flow:
//  1. POST /3/details (JSON body) — exchanges (config_id, day, party_size)
//     for a short-lived book_token plus the user's saved payment methods.
//  2. POST /3/book (form-encoded body) — commits book_token + payment
//     method to a reservation; returns resy_token.
//
// On success, returns the BookResponse with ResyToken populated.
// On a slot-taken response (either a 410 from /3/details or a 200 error
// envelope from either endpoint), returns ErrSlotTaken so the top-level
// command can map to a typed JSON error.
func (c *Client) Book(ctx context.Context, req BookRequest) (BookResponse, error) {
	out := BookResponse{Date: req.Date, Time: req.Time, PartySize: req.PartySize}
	if c.creds.AuthToken == "" {
		return out, ErrAuthMissing
	}
	if req.SlotToken == "" {
		return out, fmt.Errorf("resy: SlotToken is required (call Availability first)")
	}

	// Step 1: /3/details
	detailsBody, err := c.rawBookingDetails(ctx, req.SlotToken, req.Date, req.PartySize)
	if err != nil {
		return out, err
	}
	var details detailsResponse
	if jerr := json.Unmarshal(detailsBody, &details); jerr != nil {
		return out, fmt.Errorf("resy: parse details: %w", jerr)
	}
	// The presence of a usable book_token is the authoritative "this
	// response is good" signal. Resy sometimes piggybacks informational
	// policy notes on `message` (cancellation-window reminders,
	// dress-code blurbs, etc.) even on successful detail fetches, so
	// the previous shape — abort whenever `details.Message != ""` —
	// would bounce legitimate detail responses with no error envelope.
	// Now: explicit error envelope always aborts; bare `message`
	// only aborts when there is no book_token to proceed with.
	hasBookToken := details.BookToken != nil && details.BookToken.Value != ""
	if details.Error != nil {
		msg := details.Error.Message
		if isSlotTakenMessage(msg) {
			return out, ErrSlotTaken
		}
		return out, fmt.Errorf("resy: /3/details error: %s", msg)
	}
	if !hasBookToken {
		if details.Message != "" {
			if isSlotTakenMessage(details.Message) {
				return out, ErrSlotTaken
			}
			return out, fmt.Errorf("resy: /3/details error: %s", details.Message)
		}
		return out, ErrCanaryUnrecognizedBody
	}

	// Step 1.5: pick payment method
	paymentMethodID, err := pickPaymentMethod(details.User, req.PaymentMethodID)
	if err != nil {
		return out, err
	}

	// Step 2: /3/book
	bookBody, err := c.rawConfirmBooking(ctx, details.BookToken.Value, paymentMethodID, "")
	if err != nil {
		return out, err
	}
	var confirmed confirmResponse
	if jerr := json.Unmarshal(bookBody, &confirmed); jerr != nil {
		return out, fmt.Errorf("resy: parse book response: %w", jerr)
	}
	if confirmed.Error != nil || (confirmed.ResyToken == "" && confirmed.Message != "") {
		msg := confirmed.Message
		if confirmed.Error != nil && confirmed.Error.Message != "" {
			msg = confirmed.Error.Message
		}
		if isSlotTakenMessage(msg) {
			return out, ErrSlotTaken
		}
		return out, fmt.Errorf("resy: /3/book error: %s", msg)
	}
	if confirmed.ResyToken == "" {
		return out, ErrCanaryUnrecognizedBody
	}
	out.ResyToken = confirmed.ResyToken
	if len(confirmed.ReservationID) > 0 {
		out.ReservationID = unquoteJSON(confirmed.ReservationID)
	}
	return out, nil
}

// pickPaymentMethod selects the payment method for the book request. The
// caller-supplied override wins; otherwise prefer is_default=true, then fall
// back to the first method. Returns ErrNoPaymentMethod when the account has
// none on file.
func pickPaymentMethod(user *struct {
	PaymentMethods []paymentMethod `json:"payment_methods"`
}, override string) (string, error) {
	if override != "" {
		return override, nil
	}
	if user == nil || len(user.PaymentMethods) == 0 {
		return "", ErrNoPaymentMethod
	}
	for _, pm := range user.PaymentMethods {
		if pm.IsDefault && len(pm.ID) > 0 {
			return paymentMethodIDString(pm.ID), nil
		}
	}
	if len(user.PaymentMethods[0].ID) > 0 {
		return paymentMethodIDString(user.PaymentMethods[0].ID), nil
	}
	return "", ErrNoPaymentMethod
}

func paymentMethodIDString(raw json.RawMessage) string {
	s := unquoteJSON(raw)
	if s == "" {
		return ""
	}
	// If it parses as a positive integer, keep the canonical decimal form
	// (Resy's web client sends the numeric id without quotes).
	if n, err := strconv.ParseInt(s, 10, 64); err == nil && n > 0 {
		return strconv.FormatInt(n, 10)
	}
	return s
}

// isSlotTakenMessage classifies the family of error envelopes Resy returns
// when the slot has expired or been booked by someone else since the
// availability call. Pattern-matches over a small allowlist rather than
// guessing — false positives here would mask legitimate book failures.
func isSlotTakenMessage(msg string) bool {
	patterns := []string{
		"slot no longer available",
		"slot is no longer available",
		"no longer available",
		"reservation is no longer available",
		"invalid book token",
		"invalid configuration id",
	}
	lower := toLower(msg)
	for _, p := range patterns {
		if containsSubstring(lower, p) {
			return true
		}
	}
	return false
}

// toLower / containsSubstring are kept here to avoid importing strings
// just for two single-purpose calls. Tested via TestIsSlotTakenMessage.
func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func containsSubstring(haystack, needle string) bool {
	if needle == "" {
		return true
	}
	if len(needle) > len(haystack) {
		return false
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
