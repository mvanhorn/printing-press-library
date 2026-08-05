// Copyright 2026 matthew.martin and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"strings"
	"testing"
	"time"
)

func TestNovelLocalReportsRejectLiveDataSource(t *testing.T) {
	tests := []struct {
		name string
		cmd  func(*rootFlags) interface{ Execute() error }
	}{
		{"reconcile close", func(flags *rootFlags) interface{ Execute() error } { return newNovelReconcileCloseCmd(flags) }},
		{"service review", func(flags *rootFlags) interface{ Execute() error } { return newNovelServiceReviewCmd(flags) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cmd(&rootFlags{dataSource: "live"}).Execute()
			if err == nil || !strings.Contains(err.Error(), "no live equivalent") {
				t.Fatalf("expected local-only error, got %v", err)
			}
		})
	}
}

func TestBookingWorkloadHelpers(t *testing.T) {
	data := map[string]any{
		"start_at": "2026-08-04T13:00:00Z",
		"appointment_segments": []any{
			map[string]any{"team_member_id": "tm-1", "duration_minutes": float64(30)},
			map[string]any{"team_member_id": "tm-1", "duration_minutes": float64(45)},
		},
	}
	if got := bookingStaffID(data); got != "tm-1" {
		t.Fatalf("bookingStaffID = %q, want tm-1", got)
	}
	if got := bookingDurationMinutes(data); got != 75 {
		t.Fatalf("bookingDurationMinutes = %d, want 75", got)
	}
	wantEnd := time.Date(2026, 8, 4, 14, 15, 0, 0, time.UTC)
	if got := bookingStart(data); !got.Equal(time.Date(2026, 8, 4, 13, 0, 0, 0, time.UTC)) {
		t.Fatalf("bookingStart = %s", got)
	}
	if got := bookingEnd(data); !got.Equal(wantEnd) {
		t.Fatalf("bookingEnd = %s, want %s", got, wantEnd)
	}
	if got := teamMemberName(map[string]any{"given_name": "Ada", "family_name": "Lovelace"}); got != "Ada Lovelace" {
		t.Fatalf("teamMemberName = %q", got)
	}
}

func TestBookingStaffWorkloadsSplitMultiStaffSegments(t *testing.T) {
	data := map[string]any{"appointment_segments": []any{
		map[string]any{"team_member_id": "tm-1", "duration_minutes": float64(30)},
		map[string]any{"team_member_id": "tm-2", "duration_minutes": float64(45)},
		map[string]any{"team_member_id": "tm-1", "duration_minutes": float64(15)},
	}}
	got := bookingStaffWorkloads(data)
	if len(got) != 2 {
		t.Fatalf("workloads = %#v, want two staff entries", got)
	}
	if got[0] != (bookingStaffWorkload{StaffID: "tm-1", Minutes: 45}) || got[1] != (bookingStaffWorkload{StaffID: "tm-2", Minutes: 45}) {
		t.Fatalf("workloads = %#v, want per-staff segment totals", got)
	}
}

func TestReconcileCloseRecordTimePrefersResourceUpdate(t *testing.T) {
	record := localSquareRecord{ResourceType: "payments", SyncedAt: time.Date(2026, 8, 5, 0, 0, 0, 0, time.UTC), Data: map[string]any{
		"created_at": "2026-07-01T12:00:00Z",
		"updated_at": "2026-08-04T18:30:00Z",
	}}
	want := time.Date(2026, 8, 4, 18, 30, 0, 0, time.UTC)
	if got := reconcileCloseRecordTime(record); !got.Equal(want) {
		t.Fatalf("reconcileCloseRecordTime = %s, want updated status time %s", got, want)
	}
}

func TestBookingPaymentOrOrderEvidence(t *testing.T) {
	booking := localSquareRecord{ResourceType: "bookings", Data: map[string]any{
		"customer_id": "customer-1", "location_id": "location-1", "start_at": "2026-08-04T13:00:00Z",
	}}
	completedPayment := localSquareRecord{ResourceType: "payments", Data: map[string]any{
		"customer_id": "customer-1", "location_id": "location-1", "status": "COMPLETED", "created_at": "2026-08-04T15:00:00Z",
	}}
	failedPayment := completedPayment
	failedPayment.Data = map[string]any{
		"customer_id": "customer-1", "location_id": "location-1", "status": "FAILED", "created_at": "2026-08-04T15:00:00Z",
	}
	if !bookingHasPaymentOrOrderEvidence(booking, []localSquareRecord{completedPayment}, nil) {
		t.Fatal("expected same-customer, same-location completed payment to count as evidence")
	}
	if bookingHasPaymentOrOrderEvidence(booking, []localSquareRecord{failedPayment}, nil) {
		t.Fatal("failed payment must not count as payment evidence")
	}
	direct := booking
	direct.Data = map[string]any{"order_id": "order-1"}
	if !bookingHasPaymentOrOrderEvidence(direct, nil, nil) {
		t.Fatal("direct booking order ID should count as evidence")
	}
}

func TestSameServiceDayIncludesMonth(t *testing.T) {
	august := time.Date(2026, 8, 4, 10, 0, 0, 0, time.UTC)
	september := time.Date(2026, 9, 4, 10, 0, 0, 0, time.UTC)
	if sameServiceDay(august, september) {
		t.Fatal("same day-of-month in different months must not match")
	}
	if !sameServiceDay(august, august.Add(8*time.Hour)) {
		t.Fatal("timestamps on the same date should match")
	}
}

func TestSameServiceDayUsesBookingZoneAndBoundedDay(t *testing.T) {
	bookingAt, err := time.Parse(time.RFC3339, "2026-08-04T23:30:00-04:00")
	if err != nil {
		t.Fatal(err)
	}
	if !sameServiceDay(bookingAt, time.Date(2026, 8, 5, 2, 0, 0, 0, time.UTC)) {
		t.Fatal("02:00 UTC is still August 4 in the booking zone and should match")
	}
	if sameServiceDay(bookingAt, time.Date(2026, 8, 5, 5, 0, 0, 0, time.UTC)) {
		t.Fatal("01:00 in the booking zone is outside the bounded August 4 service day")
	}
}

func TestReconcileMoneyHelpers(t *testing.T) {
	data := map[string]any{
		"total_money": map[string]any{"amount": float64(1250), "currency": "USD"},
	}
	if got := firstInt(data, []string{"amount_money", "amount"}, []string{"total_money", "amount"}); got != 1250 {
		t.Fatalf("firstInt = %d, want 1250", got)
	}
	summary := &reconcileCloseSummary{}
	setReconcileCurrency(summary, data, []string{"total_money", "currency"})
	if summary.Currency != "USD" {
		t.Fatalf("currency = %q, want USD", summary.Currency)
	}
}

func TestReconcilePaymentMoneyPrefersTotalAndKeepsMatchingCurrency(t *testing.T) {
	data := map[string]any{
		"amount_money": map[string]any{"amount": float64(1000), "currency": "USD"},
		"total_money":  map[string]any{"amount": float64(1250), "currency": "CAD"},
	}
	amount, currency := reconcilePaymentMoney(data)
	if amount != 1250 || currency != "CAD" {
		t.Fatalf("reconcilePaymentMoney = (%d, %q), want total (1250, CAD)", amount, currency)
	}
	amount, currency = reconcilePaymentMoney(map[string]any{
		"amount_money": map[string]any{"amount": float64(1000), "currency": "USD"},
	})
	if amount != 1000 || currency != "USD" {
		t.Fatalf("fallback reconcilePaymentMoney = (%d, %q), want (1000, USD)", amount, currency)
	}
}

func TestReconcileRefundRequiresExplicitCompletedStatus(t *testing.T) {
	tests := []struct {
		name string
		data map[string]any
		want bool
	}{
		{"completed", map[string]any{"status": "COMPLETED"}, true},
		{"completed case insensitive", map[string]any{"status": "completed"}, true},
		{"pending", map[string]any{"status": "PENDING"}, false},
		{"missing", map[string]any{}, false},
	}
	for _, tt := range tests {
		if got := reconcileRefundCompleted(tt.data); got != tt.want {
			t.Errorf("%s: reconcileRefundCompleted = %v, want %v", tt.name, got, tt.want)
		}
	}
}
