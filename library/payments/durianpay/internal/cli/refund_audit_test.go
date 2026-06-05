// Copyright 2026 ardihanan and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestClassifyRefundAudit(t *testing.T) {
	refunds := []localRefund{
		// over-refund: two refunds totaling 120 > payment amount 100, with
		// distinct amounts so they are NOT also duplicates.
		{ID: "ref_a1", PaymentID: "pay_over", Amount: 70, Status: "completed"},
		{ID: "ref_a2", PaymentID: "pay_over", Amount: 50, Status: "completed"},
		// duplicate: same payment_id + same amount
		{ID: "ref_dup1", PaymentID: "pay_dup", Amount: 25, Status: "completed"},
		{ID: "ref_dup2", PaymentID: "pay_dup", Amount: 25, Status: "completed"},
		// missing payment row
		{ID: "ref_orphan", PaymentID: "pay_missing", Amount: 5, Status: "completed"},
		// no payment_id at all
		{ID: "ref_nopid", PaymentID: "", Amount: 1, Status: "completed"},
		// clean single refund within amount
		{ID: "ref_ok", PaymentID: "pay_ok", Amount: 10, Status: "completed"},
	}
	payments := map[string]float64{
		"pay_over": 100,
		"pay_dup":  500,
		"pay_ok":   50,
	}

	res := classifyRefundAudit(refunds, payments, len(payments), "30d")

	if len(res.OverRefunds) != 1 || res.OverRefunds[0].PaymentID != "pay_over" {
		t.Errorf("over_refunds = %+v, want [pay_over]", res.OverRefunds)
	}
	if len(res.OverRefunds) == 1 {
		if res.OverRefunds[0].RefundTotal != 120 || res.OverRefunds[0].RefundCount != 2 {
			t.Errorf("over refund total/count = %v/%d, want 120/2", res.OverRefunds[0].RefundTotal, res.OverRefunds[0].RefundCount)
		}
	}
	if len(res.RefundsWithoutPayment) != 2 {
		t.Errorf("refunds_without_payment = %d, want 2 (orphan + no_pid)", len(res.RefundsWithoutPayment))
	}
	if len(res.DuplicateRefunds) != 1 || res.DuplicateRefunds[0].PaymentID != "pay_dup" {
		t.Errorf("duplicate_refunds = %+v, want [pay_dup]", res.DuplicateRefunds)
	}
	if len(res.DuplicateRefunds) == 1 && len(res.DuplicateRefunds[0].RefundIDs) != 2 {
		t.Errorf("duplicate refund ids = %v, want 2", res.DuplicateRefunds[0].RefundIDs)
	}
	if res.ScannedRefunds != 7 || res.ScannedPayments != 3 {
		t.Errorf("scanned = %d/%d, want 7/3", res.ScannedRefunds, res.ScannedPayments)
	}
}

func TestClassifyRefundAuditEmptySlicesNotNil(t *testing.T) {
	res := classifyRefundAudit(nil, map[string]float64{}, 0, "30d")
	if res.OverRefunds == nil || res.RefundsWithoutPayment == nil || res.DuplicateRefunds == nil {
		t.Fatalf("empty result slices must be non-nil so they marshal as []")
	}
}
