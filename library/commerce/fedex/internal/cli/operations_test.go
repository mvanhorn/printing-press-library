// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/approval"
)

func TestOperationsReconcileRequiresBoundConfirmation(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir, err := approval.DefaultStoreDir()
	if err != nil {
		t.Fatal(err)
	}
	state := approval.NewStore(dir, time.Minute)
	mutation := approval.Mutation{Action: "create_label", Origin: "https://apis-sandbox.fedex.com:443", Method: "POST", Path: "/ship/v1/shipments", Request: map[string]any{"request": "same"}}
	target, err := state.Create(mutation, approval.ReviewSummary{})
	if err != nil {
		t.Fatal(err)
	}
	_, permit, err := state.Consume(target.ID, target.ConfirmationDigest, mutation)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.Complete(target.ID, approval.StatusOutcomeUnknown, "outcome_unknown"); err != nil {
		t.Fatal(err)
	}
	permit.Release()

	const reason = "FedEx support confirmed no shipment was created"
	var previewOut bytes.Buffer
	previewFlags := &rootFlags{}
	preview := newRootCmd(previewFlags)
	preview.SetOut(&previewOut)
	preview.SetErr(&previewOut)
	preview.SetArgs([]string{"operations", "reconcile", target.ID, "--resolution", "not_executed", "--reason", reason, "--json"})
	if err := preview.Execute(); err != nil {
		t.Fatalf("preview: %v\n%s", err, previewOut.String())
	}
	var previewResult struct {
		Status             string                 `json:"status"`
		OperationID        string                 `json:"operation_id"`
		ConfirmationDigest string                 `json:"confirmation_digest"`
		Review             approval.ReviewSummary `json:"review"`
	}
	if err := json.Unmarshal(previewOut.Bytes(), &previewResult); err != nil {
		t.Fatalf("decode preview: %v\n%s", err, previewOut.String())
	}
	if previewResult.Status != approval.StatusPending || previewResult.Review.ReconciliationTarget != target.ID {
		t.Fatalf("preview record=%+v", previewResult)
	}

	var confirmOut bytes.Buffer
	confirmFlags := &rootFlags{}
	confirm := newRootCmd(confirmFlags)
	confirm.SetOut(&confirmOut)
	confirm.SetErr(&confirmOut)
	confirm.SetArgs([]string{"operations", "reconcile", target.ID, "--resolution", "not_executed", "--reason", reason, "--yes", "--operation-id", previewResult.OperationID, "--confirmation-digest", previewResult.ConfirmationDigest, "--json"})
	if err := confirm.Execute(); err != nil {
		t.Fatalf("confirm: %v\n%s", err, confirmOut.String())
	}
	resolved, err := state.Get(target.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Status != approval.StatusReconciledNotExecuted {
		t.Fatalf("resolved status=%s", resolved.Status)
	}
	reasonDigest := sha256.Sum256([]byte(reason))
	if !strings.Contains(resolved.ErrorClass, hex.EncodeToString(reasonDigest[:])) || strings.Contains(resolved.ErrorClass, reason) {
		t.Fatalf("unsafe reconciliation evidence=%q", resolved.ErrorClass)
	}
}
