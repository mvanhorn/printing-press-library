// Copyright 2026 Trevin Chow and contributors. Licensed under Apache-2.0. See LICENSE.

package speccontract

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/client"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/config"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/mcp"
	"github.com/mvanhorn/printing-press-library/library/commerce/fedex/internal/workflow"
)

func TestSpecErrorExamplesAreSurfaced(t *testing.T) {
	docs, err := documents()
	if err != nil {
		t.Fatal(err)
	}
	var examples []errorExample
	for name, doc := range docs {
		examples = append(examples, collectErrorExamples(name, doc)...)
	}
	if len(examples) == 0 {
		t.Fatal("published specs contained no error examples")
	}
	for _, example := range examples {
		t.Run(example.name, func(t *testing.T) {
			payload, err := json.Marshal(example.body)
			if err != nil {
				t.Fatal(err)
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(example.status)
				_, _ = w.Write(payload)
			}))
			t.Cleanup(server.Close)
			fedex := testClient(t, server.URL)
			fedex.WriteApproved = true
			fedex.AllowDuplicate = true
			fedex.MutationLedgerDir = t.TempDir()
			_, status, callErr := fedex.Put("/ship/v1/shipments/cancel", map[string]any{"marker": example.name})
			if callErr == nil {
				t.Fatalf("status %d example was treated as success", status)
			}
			if status != example.status && example.status != 0 {
				t.Fatalf("status=%d, want %d, err=%v", status, example.status, callErr)
			}
			var apiErr *client.APIError
			if !errors.As(callErr, &apiErr) {
				t.Fatalf("error type=%T, want APIError in the chain: %v", callErr, callErr)
			}
			code := errorCode(example.body)
			if code != "" && !strings.Contains(callErr.Error(), code) {
				t.Fatalf("error %q dropped published code %s", callErr, code)
			}
		})
	}
}

func TestGeneratedSuccessSamplesKeepOperationalFields(t *testing.T) {
	docs, err := documents()
	if err != nil {
		t.Fatal(err)
	}
	ship := docs["ship"]
	labelDoc := propertyExample(ship, "LabelResponseVO", "encodedLabel")
	tracking := propertyExample(ship, "PieceResponse", "trackingNumber")
	if labelDoc == nil || tracking == nil {
		t.Fatal("Ship spec is missing the label or tracking examples")
	}
	labelBody, err := json.Marshal(map[string]any{
		"output": map[string]any{
			"transactionShipments": []any{map[string]any{
				"serviceType": "FEDEX_GROUND",
				"pieceResponses": []any{map[string]any{
					"trackingNumber": tracking,
					"packageDocuments": []any{map[string]any{
						"contentType":  "LABEL",
						"docType":      "PDF",
						"encodedLabel": labelDoc,
					}},
				}},
			}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := workflow.ParseLabelResponse(labelBody)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.TrackingNumber != tracking.(string) {
		t.Fatalf("tracking=%q, want spec example %v", parsed.TrackingNumber, tracking)
	}
	if _, _, err := workflow.ParseAndDecodeLabelResponse(labelBody, workflow.MaxPDFLabelBytes); err == nil {
		t.Fatal("spec placeholder encodedLabel was accepted as a PDF")
	}

	cancelSample := map[string]any{"output": generatedSample(ship, "CancelShipmentOutputVO")}
	cancelBody, _ := json.Marshal(cancelSample)
	cancelled, err := workflow.ParseCancelShipmentResponse(cancelBody)
	if err != nil {
		t.Fatal(err)
	}
	if !cancelled.SuccessKnown || !cancelled.Successful {
		t.Fatalf("cancel sample was not a known success: %+v sample=%s", cancelled, cancelBody)
	}

	pickup := docs["pickup"]
	createSample := map[string]any{"output": generatedSample(pickup, "CreatePickupOutputVO")}
	createBody, _ := json.Marshal(createSample)
	created, err := workflow.ParsePickupResponse(createBody)
	if err != nil {
		t.Fatal(err)
	}
	if created.ConfirmationNumber == "" || created.LocationCode == "" {
		t.Fatalf("pickup create sample dropped confirmation or location: %+v", created)
	}
	cancelPickupSample := map[string]any{"output": generatedSample(pickup, "CancelPickupOutputVO")}
	cancelPickupBody, _ := json.Marshal(cancelPickupSample)
	pickupCancelled, err := workflow.ParseCancelPickupResponse(cancelPickupBody)
	if err != nil {
		t.Fatal(err)
	}
	if !pickupCancelled.SuccessKnown || !pickupCancelled.Successful || pickupCancelled.ConfirmationNumber == "" {
		t.Fatalf("pickup cancel sample dropped the confirmation: %+v", pickupCancelled)
	}

	availability := map[string]any{"output": generatedSample(pickup, "PickupAvailabilityOutputVO")}
	availabilityBody, _ := json.Marshal(availability)
	minimized, err := mcp.MinimizeNarrowReadResponseForTest("pickup_availability", availabilityBody)
	if err != nil {
		t.Fatal(err)
	}
	text := mustJSON(minimized)
	for _, want := range []string{`"carrier":"FDXE"`, `"available":true`, `"pickup_date":"2019-01-20"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("availability sample %s missing %s", text, want)
		}
	}

	rateOutput, ok := schemaExample(docs["rate"], "RateOutputVO")
	if !ok {
		t.Fatal("Rate spec has no RateOutputVO example")
	}
	rateBody, _ := json.Marshal(map[string]any{"output": rateOutput})
	rates, err := mcp.MinimizeNarrowReadResponseForTest("get_rates", rateBody)
	if err != nil {
		t.Fatal(err)
	}
	rateText := mustJSON(rates)
	for _, want := range []string{`"service_type":"INTERNATIONAL_FIRST"`, `"currency":"CAD"`, `"code":"MONEYBACKGUARANTEE.NOT.ELIGIBLE"`} {
		if !strings.Contains(rateText, want) {
			t.Fatalf("rate example %s missing %s", rateText, want)
		}
	}

	address := map[string]any{"output": map[string]any{"resolvedAddresses": []any{generatedSample(docs["address"], "ResolvedAddress")}}}
	addressBody, _ := json.Marshal(address)
	resolved, err := mcp.MinimizeNarrowReadResponseForTest("validate_address", addressBody)
	if err != nil {
		t.Fatal(err)
	}
	addressText := mustJSON(resolved)
	for _, want := range []string{`"city":"IRVING"`, `"country_code":"US"`, `"classification":"BUSINESS"`, "PARKRIDGE"} {
		if !strings.Contains(addressText, want) {
			t.Fatalf("address sample %s missing %s", addressText, want)
		}
	}

	validation := map[string]any{"output": map[string]any{"alerts": []any{generatedSample(ship, "Alert")}}}
	validationBody, _ := json.Marshal(validation)
	validated, err := mcp.MinimizeNarrowReadResponseForTest("validate_shipment", validationBody)
	if err != nil {
		t.Fatal(err)
	}
	validationText := mustJSON(validated)
	if !strings.Contains(validationText, "SHIPMENT.VALIDATION.SUCCESS") || !strings.Contains(validationText, `"valid":true`) {
		t.Fatalf("validation alert sample dropped the success alert: %s", validationText)
	}
}

func TestOfflineSpecServerRunsLabelPickupRateAndAddress(t *testing.T) {
	docs, err := documents()
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/oauth/token":
			_, _ = io.WriteString(w, `{"access_token":"offline-token","expires_in":3600}`)
		case "/ship/v1/shipments":
			tracking := propertyExample(docs["ship"], "PieceResponse", "trackingNumber")
			label := propertyExample(docs["ship"], "LabelResponseVO", "encodedLabel")
			body, _ := json.Marshal(map[string]any{"output": map[string]any{"transactionShipments": []any{map[string]any{
				"pieceResponses": []any{map[string]any{"trackingNumber": tracking, "packageDocuments": []any{map[string]any{
					"contentType": "LABEL", "docType": "PDF", "encodedLabel": label,
				}}}},
			}}}})
			_, _ = w.Write(body)
		case "/pickup/v1/pickups/availabilities":
			body, _ := json.Marshal(map[string]any{"output": generatedSample(docs["pickup"], "PickupAvailabilityOutputVO")})
			_, _ = w.Write(body)
		case "/pickup/v1/pickups":
			body, _ := json.Marshal(map[string]any{"output": generatedSample(docs["pickup"], "CreatePickupOutputVO")})
			_, _ = w.Write(body)
		case "/pickup/v1/pickups/cancel":
			body, _ := json.Marshal(map[string]any{"output": generatedSample(docs["pickup"], "CancelPickupOutputVO")})
			_, _ = w.Write(body)
		case "/rate/v1/rates/quotes":
			example, _ := schemaExample(docs["rate"], "RateOutputVO")
			body, _ := json.Marshal(map[string]any{"output": example})
			_, _ = w.Write(body)
		case "/address/v1/addresses/resolve":
			body, _ := json.Marshal(map[string]any{"output": map[string]any{"resolvedAddresses": []any{generatedSample(docs["address"], "ResolvedAddress")}}})
			_, _ = w.Write(body)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	fedex := testClient(t, server.URL)
	fedex.WriteApproved = true
	fedex.AllowDuplicate = true
	fedex.MutationLedgerDir = t.TempDir()

	labelRaw, _, err := fedex.Post("/ship/v1/shipments", createLabelBody("FEDEX_GROUND", "US", true))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := workflow.ParseAndDecodeLabelResponse(labelRaw, workflow.MaxPDFLabelBytes); err == nil {
		t.Fatal("offline label create accepted the spec placeholder as a PDF")
	}

	availabilityRaw, _, err := fedex.Post("/pickup/v1/pickups/availabilities", availabilityBody("FDXE", "DOMESTIC", false))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mcp.MinimizeNarrowReadResponseForTest("pickup_availability", availabilityRaw); err != nil {
		t.Fatal(err)
	}
	scheduledRaw, _, err := fedex.Post("/pickup/v1/pickups", schedulePickupBody("FDXE", ""))
	if err != nil {
		t.Fatal(err)
	}
	scheduled, err := workflow.ParsePickupResponse(scheduledRaw)
	if err != nil || scheduled.ConfirmationNumber == "" {
		t.Fatalf("schedule=%+v err=%v", scheduled, err)
	}
	cancelledRaw, _, err := fedex.Put("/pickup/v1/pickups/cancel", map[string]any{
		"associatedAccountNumber": map[string]any{"value": "123456789"},
		"pickupConfirmationCode":  scheduled.ConfirmationNumber,
		"carrierCode":             "FDXE",
		"scheduledDate":           "2020-07-03",
		"location":                scheduled.LocationCode,
	})
	if err != nil {
		t.Fatal(err)
	}
	cancelled, err := workflow.ParseCancelPickupResponse(cancelledRaw)
	if err != nil || !cancelled.Successful {
		t.Fatalf("cancel=%+v err=%v", cancelled, err)
	}

	rateRaw, _, err := fedex.Post("/rate/v1/rates/quotes", map[string]any{"accountNumber": map[string]any{"value": "123456789"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mcp.MinimizeNarrowReadResponseForTest("get_rates", rateRaw); err != nil {
		t.Fatal(err)
	}
	addressRaw, _, err := fedex.Post("/address/v1/addresses/resolve", map[string]any{"addressesToValidate": []any{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mcp.MinimizeNarrowReadResponseForTest("validate_address", addressRaw); err != nil {
		t.Fatal(err)
	}
}

type errorExample struct {
	name   string
	status int
	body   any
}

func collectErrorExamples(docName string, doc *openapi3.T) []errorExample {
	if doc.Paths == nil {
		return nil
	}
	var out []errorExample
	for path, item := range doc.Paths.Map() {
		if item == nil {
			continue
		}
		for method, op := range item.Operations() {
			if op == nil || op.Responses == nil {
				continue
			}
			for status, resp := range op.Responses.Map() {
				code, err := strconv.Atoi(status)
				if err != nil || code < 400 || resp == nil || resp.Value == nil || resp.Value.Content == nil {
					continue
				}
				media := resp.Value.Content.Get("application/json")
				if media == nil || media.Example == nil {
					continue
				}
				out = append(out, errorExample{
					name:   fmt.Sprintf("%s %s %s %s", docName, method, path, status),
					status: code,
					body:   media.Example,
				})
			}
		}
	}
	return out
}

func propertyExample(doc *openapi3.T, schemaName, property string) any {
	schema, err := schemaNamed(doc, schemaName)
	if err != nil || schema.Properties == nil {
		return nil
	}
	prop := schema.Properties[property]
	if prop == nil || prop.Value == nil {
		return nil
	}
	return prop.Value.Example
}

func errorCode(body any) string {
	value, err := jsonRoundTrip(body)
	if err != nil {
		return ""
	}
	object, ok := value.(map[string]any)
	if !ok {
		return ""
	}
	if items, ok := object["errors"].([]any); ok && len(items) > 0 {
		if item, ok := items[0].(map[string]any); ok {
			if code, ok := item["code"].(string); ok {
				return code
			}
		}
	}
	if code, ok := object["error"].(string); ok {
		return code
	}
	return ""
}

func testClient(t *testing.T, baseURL string) *client.Client {
	t.Helper()
	t.Setenv("FEDEX_DATA_DIR", t.TempDir())
	cfg := &config.Config{
		BaseURL:      baseURL,
		AccessToken:  "test-access-token",
		TokenBaseURL: baseURL,
		TokenExpiry:  time.Now().Add(time.Hour),
	}
	return client.New(cfg, time.Second, 0)
}

func mustJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(data)
}

func TestClientWireMatchesPublishedContract(t *testing.T) {
	docs, err := documents()
	if err != nil {
		t.Fatal(err)
	}
	type recorded struct {
		method string
		path   string
		header http.Header
		body   []byte
	}
	var seen []recorded
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		seen = append(seen, recorded{method: r.Method, path: r.URL.Path, header: r.Header.Clone(), body: body})
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/oauth/token" {
			_, _ = io.WriteString(w, `{"access_token":"offline-token","expires_in":3600}`)
			return
		}
		_, _ = io.WriteString(w, `{}`)
	}))
	t.Cleanup(server.Close)

	t.Setenv("FEDEX_DATA_DIR", t.TempDir())
	cfg := &config.Config{BaseURL: server.URL, ClientID: "client-id", ClientSecret: "client-secret"}
	fedex := client.New(cfg, time.Second, 0)
	fedex.WriteApproved = true
	fedex.AllowDuplicate = true
	fedex.MutationLedgerDir = t.TempDir()

	label := createLabelBody("FEDEX_GROUND", "US", true)
	if _, _, err := fedex.Post("/ship/v1/shipments", label); err != nil {
		t.Fatal(err)
	}
	rate := map[string]any{"accountNumber": map[string]any{"value": "123456789"}}
	if _, _, err := fedex.Post("/rate/v1/rates/quotes", rate); err != nil {
		t.Fatal(err)
	}
	cancel := map[string]any{
		"associatedAccountNumber": map[string]any{"value": "123456789"},
		"pickupConfirmationCode":  "NQAA97",
		"scheduledDate":           "2020-07-03",
		"carrierCode":             "FDXE",
		"location":                "NQAA",
	}
	if _, _, err := fedex.Put("/pickup/v1/pickups/cancel", cancel); err != nil {
		t.Fatal(err)
	}

	if len(seen) != 4 {
		t.Fatalf("calls=%d, want oauth plus ship, rate, and pickup cancel", len(seen))
	}
	token := seen[0]
	if token.method != http.MethodPost || token.path != "/oauth/token" {
		t.Fatalf("token call %s %s", token.method, token.path)
	}
	if token.header.Get("Content-Type") != "application/x-www-form-urlencoded" {
		t.Fatalf("oauth content-type %q", token.header.Get("Content-Type"))
	}
	if token.header.Get("Authorization") != "" {
		t.Fatalf("oauth request sent Authorization %q", token.header.Get("Authorization"))
	}
	form, err := url.ParseQuery(string(token.body))
	if err != nil {
		t.Fatal(err)
	}
	formBody := map[string]any{}
	for key, values := range form {
		if len(values) == 1 {
			formBody[key] = values[0]
		}
	}
	if err := validateSchema(docs["authorization"], "FullSchema", formBody); err != nil {
		t.Fatalf("oauth form rejected by published schema: %v", err)
	}

	checks := []struct {
		call   recorded
		doc    string
		schema string
	}{
		{seen[1], "ship", shipSchema},
		{seen[3], "pickup", "Full_Schema_Cancel_Pickup"},
	}
	for _, check := range checks {
		if check.call.header.Get("Content-Type") != "application/json" {
			t.Errorf("%s content-type %q", check.call.path, check.call.header.Get("Content-Type"))
		}
		if check.call.header.Get("Authorization") != "Bearer offline-token" {
			t.Errorf("%s authorization %q", check.call.path, check.call.header.Get("Authorization"))
		}
		if check.call.header.Get("X-locale") != "en_US" {
			t.Errorf("%s x-locale %q", check.call.path, check.call.header.Get("X-locale"))
		}
		var payload any
		if err := json.Unmarshal(check.call.body, &payload); err != nil {
			t.Fatal(err)
		}
		if err := validateSchema(docs[check.doc], check.schema, payload); err != nil {
			t.Errorf("%s body rejected by %s: %v", check.call.path, check.schema, err)
		}
	}
	if seen[1].method != http.MethodPost || seen[1].path != "/ship/v1/shipments" {
		t.Fatalf("ship call %s %s", seen[1].method, seen[1].path)
	}
	if seen[2].method != http.MethodPost || seen[2].path != "/rate/v1/rates/quotes" {
		t.Fatalf("rate call %s %s", seen[2].method, seen[2].path)
	}
	if seen[2].header.Get("Authorization") != "Bearer offline-token" || seen[2].header.Get("Content-Type") != "application/json" {
		t.Fatalf("rate headers content-type=%q authorization=%q", seen[2].header.Get("Content-Type"), seen[2].header.Get("Authorization"))
	}
	if seen[3].method != http.MethodPut || seen[3].path != "/pickup/v1/pickups/cancel" {
		t.Fatalf("cancel call %s %s", seen[3].method, seen[3].path)
	}
}
