package cli

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestCancelDryRunJSONIsParseable(t *testing.T) {
	root := RootCmd()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"cancel",
		"--reservation-id", "11189637",
		"--dry-run",
		"--json",
		"--no-learn",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("cancel dry-run: %v (stderr=%q)", err, stderr.String())
	}
	var got struct {
		DryRun bool   `json:"dry_run"`
		Action string `json:"action"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatalf("cancel --dry-run --json emitted invalid JSON %q: %v", stdout.String(), err)
	}
	if !got.DryRun || got.Action != "cancel booking" {
		t.Fatalf("cancel dry-run payload = %+v", got)
	}
}

func TestBuildCancelPayload(t *testing.T) {
	var b upcomingBooking
	// Mirror a real upcoming-bookings record (Barton Springs, reservation 11189637).
	raw := `{
	  "uuid": "11189637",
	  "startsAt": "2026-08-18T08:30:00.000Z",
	  "endsAt": "2026-08-18T17:00:00.000Z",
	  "bookingSourceType": 0,
	  "IsBookingApprovalOn": false,
	  "creditOrder": {"price": "1.88"},
	  "reservable": {
	    "name": "DD315",
	    "uuid": "17f295a3-ca79-4f21-986e-c215a471b51c",
	    "KubeId": "70149",
	    "location": {"uuid": "e0317ab1-39a8-4024-ae12-6260b5470295", "name": "801 Barton Springs Rd", "accountType": 2}
	  },
	  "order": {"grandTotal": {"amount": 47.0, "currency": "$"}}
	}`
	if err := json.Unmarshal([]byte(raw), &b); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	p := buildCancelPayload(b, "note")

	// The three id fields all carry the reservation id.
	for _, k := range []string{"bookingId", "reservationId", "bookingExternaluuid"} {
		if p[k] != "11189637" {
			t.Fatalf("%s = %v, want 11189637", k, p[k])
		}
	}
	if p["spaceId"] != "70149" {
		t.Fatalf("spaceId = %v, want 70149 (kubeSpaceId)", p["spaceId"])
	}
	if p["reservableId"] != "17f295a3-ca79-4f21-986e-c215a471b51c" {
		t.Fatalf("reservableId = %v", p["reservableId"])
	}
	if p["locationId"] != "e0317ab1-39a8-4024-ae12-6260b5470295" {
		t.Fatalf("locationId = %v", p["locationId"])
	}
	if p["bookingLocationType"] != "2" {
		t.Fatalf("bookingLocationType = %v, want \"2\"", p["bookingLocationType"])
	}
	if p["bookingType"] != "0" {
		t.Fatalf("bookingType = %v, want \"0\"", p["bookingType"])
	}
	if c, ok := p["creditsUsed"].(float64); !ok || c != 1.88 {
		t.Fatalf("creditsUsed = %v, want 1.88", p["creditsUsed"])
	}
	if p["startTime"] != "2026-08-18T08:30:00.000Z" || p["endTime"] != "2026-08-18T17:00:00.000Z" {
		t.Fatalf("times: start=%v end=%v", p["startTime"], p["endTime"])
	}
	if p["cancellationNote"] != "note" {
		t.Fatalf("note = %v", p["cancellationNote"])
	}
}
