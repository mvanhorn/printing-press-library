// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAgentModeDoesNotAuthorizeMutations(t *testing.T) {
	var flags rootFlags
	cmd := newRootCmd(&flags)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--agent", "version"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if flags.yes {
		t.Fatal("--agent must not imply --yes")
	}
	if !flags.agent || !flags.asJSON || !flags.compact || !flags.noInput {
		t.Fatalf("agent-safe output defaults were not applied: %#v", flags)
	}
}

func TestExplicitYesStillAuthorizesMutations(t *testing.T) {
	var flags rootFlags
	cmd := newRootCmd(&flags)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--agent", "--yes", "version"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !flags.yes {
		t.Fatal("explicit --yes must remain set")
	}
}

func TestShipmentCreateRequiresBoundPreviewBeforeNetwork(t *testing.T) {
	calls := 0
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"transactionId":"tx-create","output":{"transactionShipments":[{"masterTrackingNumber":"123456789012","serviceType":"FEDEX_GROUND","pieceResponses":[{"trackingNumber":"123456789012","packageDocuments":[{"contentType":"application/pdf","docType":"LABEL","encodedLabel":"JVBERi0xLjQKJSVFT0YK"}]}]}]}}`))
	}))
	t.Cleanup(api.Close)
	t.Setenv("HOME", t.TempDir())
	setCLITestAuth(t, api.URL)

	shipmentArgs := []string{
		"--json",
		"shipments", "create",
		"--requested-shipment", `{"shipper":{"contact":{"personName":"Sender","phoneNumber":"5550000000"},"address":{"streetLines":["1 Origin St"],"city":"Origin","postalCode":"00000","countryCode":"US"}},"recipients":[{"contact":{"personName":"Recipient","phoneNumber":"5550000001"},"address":{"streetLines":["2 Destination St"],"city":"Destination","postalCode":"00001","countryCode":"US"}}],"serviceType":"FEDEX_GROUND","packagingType":"YOUR_PACKAGING","requestedPackageLineItems":[{"weight":{"units":"LB","value":1}}],"labelSpecification":{"imageType":"PDF"}}`,
		"--label-response-options", "LABEL",
		"--account-number", `{"value":"123456789"}`,
	}
	var bareFlags rootFlags
	bare := newRootCmd(&bareFlags)
	bare.SetOut(io.Discard)
	bare.SetErr(io.Discard)
	bare.SetArgs(append([]string{"--yes"}, shipmentArgs...))
	if err := bare.Execute(); err == nil {
		t.Fatal("bare --yes executed a protected mutation without a bound preview")
	}
	if calls != 0 {
		t.Fatalf("bare --yes emitted %d requests, want 0", calls)
	}

	var previewOut bytes.Buffer
	var flags rootFlags
	cmd := newRootCmd(&flags)
	cmd.SetOut(&previewOut)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(shipmentArgs)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("preview shipment: %v", err)
	}
	if calls != 0 {
		t.Fatalf("unconfirmed shipment emitted %d requests, want 0", calls)
	}

	var preview struct {
		OperationID        string `json:"operation_id"`
		ConfirmationDigest string `json:"confirmation_digest"`
	}
	if err := json.Unmarshal(previewOut.Bytes(), &preview); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if preview.OperationID == "" || preview.ConfirmationDigest == "" {
		t.Fatalf("preview missing bound confirmation: %s", previewOut.String())
	}

	flags = rootFlags{}
	cmd = newRootCmd(&flags)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs(append([]string{"--yes", "--operation-id", preview.OperationID, "--confirmation-digest", preview.ConfirmationDigest}, shipmentArgs...))
	if err := cmd.Execute(); err != nil {
		t.Fatalf("confirmed shipment: %v", err)
	}
	if calls != 1 {
		t.Fatalf("confirmed shipment emitted %d requests, want 1", calls)
	}
}
