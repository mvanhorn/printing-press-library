// Copyright 2026 salmonumbrella. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestNewReservationCreateBodyUsesJSONAPIEnvelope(t *testing.T) {
	body := newReservationCreateBody("class-123", "pay-456", "standard")
	attrs := reservationAttributesForTest(t, body)

	if got := attrs["class_session_id"]; got != "class-123" {
		t.Fatalf("class_session_id = %v, want class-123", got)
	}
	if got := attrs["payment_option_id"]; got != "pay-456" {
		t.Fatalf("payment_option_id = %v, want pay-456", got)
	}
	if got := attrs["reservation_type"]; got != "standard" {
		t.Fatalf("reservation_type = %v, want standard", got)
	}
}

func TestNewReservationCreateBodyFromGeneratedBodyNormalizesIDs(t *testing.T) {
	body := newReservationCreateBodyFromGeneratedBody(map[string]any{
		"class_session":    map[string]any{"id": "class-123", "name": "ignored"},
		"payment_option":   map[string]any{"id": "pay-456"},
		"reservation_type": "standard",
		"guest_email":      "guest@example.com",
	})
	attrs := reservationAttributesForTest(t, body)

	if _, ok := attrs["class_session"]; ok {
		t.Fatal("class_session nested object should not be sent")
	}
	if _, ok := attrs["payment_option"]; ok {
		t.Fatal("payment_option nested object should not be sent")
	}
	if got := attrs["class_session_id"]; got != "class-123" {
		t.Fatalf("class_session_id = %v, want class-123", got)
	}
	if got := attrs["payment_option_id"]; got != "pay-456" {
		t.Fatalf("payment_option_id = %v, want pay-456", got)
	}
	if got := attrs["guest_email"]; got != "guest@example.com" {
		t.Fatalf("guest_email = %v, want guest@example.com", got)
	}
}

func reservationAttributesForTest(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("data envelope missing or wrong type: %#v", body["data"])
	}
	if got := data["type"]; got != "reservation" {
		t.Fatalf("data.type = %v, want reservation", got)
	}
	attrs, ok := data["attributes"].(map[string]any)
	if !ok {
		t.Fatalf("attributes missing or wrong type: %#v", data["attributes"])
	}
	return attrs
}
