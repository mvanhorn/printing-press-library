// Copyright 2026 educrvz and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"testing"
	"time"
)

func summaryJSON(deliveryDate, message string) json.RawMessage {
	m := map[string]any{
		"results": map[string]any{
			"deliveryDate": deliveryDate,
			"message":      map[string]any{"text": message, "type": "opened"},
		},
	}
	b, _ := json.Marshal(m)
	return b
}

func calendarJSON(min, max string) json.RawMessage {
	m := map[string]any{
		"results": map[string]any{
			"calendar": map[string]any{
				"allowed": map[string]any{"min": min, "max": max},
			},
		},
	}
	b, _ := json.Marshal(m)
	return b
}

func TestBuildChargeCalendar(t *testing.T) {
	today := time.Now().Truncate(24 * time.Hour)
	future := today.Add(14 * 24 * time.Hour)

	t.Run("computes charge -7d and lock -5d", func(t *testing.T) {
		v := buildChargeCalendar(summaryJSON(future.Format("2006-01-02")+"T03:00:00.000Z", ""), nil, 0, false)
		if v.NextDelivery == nil {
			t.Fatal("expected a next_delivery entry")
		}
		wantCharge := future.Add(-7 * 24 * time.Hour).Format("2006-01-02")
		wantLock := future.Add(-5 * 24 * time.Hour).Format("2006-01-02")
		if v.NextDelivery.ChargeDate != wantCharge {
			t.Errorf("charge_date = %s, want %s", v.NextDelivery.ChargeDate, wantCharge)
		}
		if v.NextDelivery.EditLockDate != wantLock {
			t.Errorf("edit_lock_date = %s, want %s", v.NextDelivery.EditLockDate, wantLock)
		}
		if v.NextDelivery.Status != "editable" {
			t.Errorf("status = %s, want editable (14d out)", v.NextDelivery.Status)
		}
	})

	t.Run("horizon excludes far-future delivery", func(t *testing.T) {
		v := buildChargeCalendar(summaryJSON(future.Format("2006-01-02")+"T03:00:00.000Z", ""), nil, 7*24*time.Hour, false)
		if v.NextDelivery != nil {
			t.Errorf("expected no delivery within 7d horizon, got %+v", v.NextDelivery)
		}
		if v.Note == "" {
			t.Error("expected an honest note when no delivery is in range")
		}
	})

	t.Run("locking-soon filter drops a distant lock", func(t *testing.T) {
		v := buildChargeCalendar(summaryJSON(future.Format("2006-01-02")+"T03:00:00.000Z", ""), nil, 0, true)
		if v.NextDelivery != nil {
			t.Errorf("14d-out delivery should not pass --locking-soon, got %+v", v.NextDelivery)
		}
	})

	t.Run("reschedule window parsed from calendar", func(t *testing.T) {
		v := buildChargeCalendar(
			summaryJSON(future.Format("2006-01-02")+"T03:00:00.000Z", ""),
			calendarJSON("2026-06-09", "2026-08-08"), 0, false)
		if v.RescheduleWindow == nil {
			t.Fatal("expected a reschedule_window")
		}
		if v.RescheduleWindow.Earliest != "2026-06-09" || v.RescheduleWindow.Latest != "2026-08-08" {
			t.Errorf("window = %+v, want 2026-06-09..2026-08-08", v.RescheduleWindow)
		}
	})

	t.Run("message HTML is stripped", func(t *testing.T) {
		v := buildChargeCalendar(summaryJSON(future.Format("2006-01-02")+"T03:00:00.000Z",
			"O pedido <strong>fecha</strong> dia 18/06."), nil, 0, false)
		if v.Message != "O pedido fecha dia 18/06." {
			t.Errorf("message = %q, want stripped form", v.Message)
		}
	})
}

func TestStripHTMLTags(t *testing.T) {
	tests := []struct{ in, want string }{
		{"plain", "plain"},
		{"<b>bold</b>", "bold"},
		{"a <strong>b</strong> c", "a b c"},
		{"<br>line", "line"},
		{"  <p>trim</p>  ", "trim"},
	}
	for _, tc := range tests {
		if got := stripHTMLTags(tc.in); got != tc.want {
			t.Errorf("stripHTMLTags(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseShopperDate(t *testing.T) {
	if d, err := parseShopperDate("2026-06-23T03:00:00.000Z"); err != nil || d.Format("2006-01-02") != "2026-06-23" {
		t.Errorf("ISO parse failed: %v %v", d, err)
	}
	if d, err := parseShopperDate("2026-06-23"); err != nil || d.Format("2006-01-02") != "2026-06-23" {
		t.Errorf("bare date parse failed: %v %v", d, err)
	}
	if _, err := parseShopperDate("nope"); err == nil {
		t.Error("expected error for unparseable date")
	}
}
