// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.
// Tests for the MCP output-scrubbing wrapper applied to tickets_list/orders_list
// (Task 10) and the shared JSON-text scrub used by search/sql (Tasks 12/13).
// Synthetic fixtures only (IETF example.com).
package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

const testSalt = "0123456789abcdef0123456789abcdef"

// A realistic GraphQL tickets envelope: data.tickets[].holder is PII.
const ticketsEnvelope = `{"data":{"tickets":[
  {"id":"tk-1","code":"AAA","ticketType":{"name":"GA"},"holder":{"id":"h1","email":"holder1@example.com","firstName":"Ann","phoneNumber":"5550111","dob":"1990-01-01"}},
  {"id":"tk-2","code":"BBB","ticketType":{"name":"VIP"},"holder":{"id":"h2","email":"holder2@example.com","firstName":"Bo","phoneNumber":"5550222","dob":"1991-02-02"}}
]}}`

// A realistic orders envelope: data.orders[].fan is PII.
const ordersEnvelope = `{"data":{"orders":[
  {"id":"o-1","total":2500,"event":{"name":"Show A"},"fan":{"id":"f1","email":"buyer1@example.com","firstName":"Cy","phoneNumber":"5550333","dob":"1992-03-03"}}
]}}`

// A realistic returns GraphQL envelope: data.returns[] carries linked ticket
// holder and order fan objects, both of which are PII containers.
const returnsEnvelope = `{"data":{"returns":[
  {"id":"ret-1","ticketId":"tk-1","returnedAt":"2026-05-01T12:00:00Z","reason":"fan_request",
   "ticket":{"id":"tk-1","code":"RET123","holder":{"id":"h-ret","firstName":"Rita","lastName":"Return","email":"rita.return@example.com","phoneNumber":"5550444"}},
   "order":{"id":"ord-ret","event":{"id":"evt-1","name":"Refund Show"},"fan":{"id":"fan-ret","firstName":"Omar","lastName":"Order","email":"omar.order@example.com"}}}
]}}`

// A realistic transfers GraphQL envelope plus the flat holder fields used by
// door-list style transfer output. The scrubber must catch both nested holder
// / fan containers and flat holder_name / holder_email fields.
const transfersEnvelope = `{"data":{"ticketTransfers":[
  {"id":"tr-1","transferredAt":"2026-05-02T12:00:00Z","holder_name":"Tess Transfer","holder_email":"tess.transfer@example.com",
   "tickets":[{"id":"tk-tr","holder":{"id":"h-tr","firstName":"Nina","lastName":"New","email":"nina.new@example.com","phoneNumber":"5550555"}}],
   "orders":[{"id":"ord-tr","event":{"id":"evt-2","name":"Transfer Show"},"fan":{"id":"fan-tr","firstName":"Frank","lastName":"From","email":"frank.from@example.com"}}]}
]}}`

// A realistic extras GraphQL envelope: data.extras[].holder uses extraSelection's
// full fanSelection fields while product and total are non-PII discovery fields.
const extrasEnvelope = `{"data":{"extras":[
  {"id":"ex-1","code":"EXT123","fullPrice":2000,"commission":100,"diceCommission":50,"total":2150,"hasSeparateAccessBarcode":true,
   "holder":{"id":"h-extra","firstName":"Ella","lastName":"Extra","email":"ella.extra@example.com","phoneNumber":"5550666","optInPartners":true},
   "product":{"id":"prod-1","name":"Poster"},
   "variant":{"id":"var-1","name":"Signed","size":"A2","sku":"POST-A2"},
   "ticket":{"id":"tk-extra"}}
]}}`

func TestScrubJSONTextRedactsTicketHolderPII(t *testing.T) {
	out, changed := scrubJSONText(ticketsEnvelope, Opts{Salt: []byte(testSalt)})
	if !changed {
		t.Fatalf("scrubJSONText reported no change on a PII-bearing envelope")
	}
	for _, pii := range []string{"holder1@example.com", "holder2@example.com", "Ann", "Bo", "5550111", "1990-01-01"} {
		if strings.Contains(out, pii) {
			t.Errorf("scrubbed tickets output still contains PII %q:\n%s", pii, out)
		}
	}
	// Discovery + token survive.
	for _, keep := range []string{"GA", "VIP", "fan_ref", "AAA"} {
		if !strings.Contains(out, keep) {
			t.Errorf("scrubbed tickets output dropped expected token/field %q:\n%s", keep, out)
		}
	}
}

func TestScrubJSONTextRedactsOrderFanPII(t *testing.T) {
	out, _ := scrubJSONText(ordersEnvelope, Opts{Salt: []byte(testSalt)})
	for _, pii := range []string{"buyer1@example.com", "Cy", "5550333", "1992-03-03"} {
		if strings.Contains(out, pii) {
			t.Errorf("scrubbed orders output still contains PII %q:\n%s", pii, out)
		}
	}
	if !strings.Contains(out, "fan_ref") || !strings.Contains(out, "Show A") {
		t.Errorf("scrubbed orders output dropped fan_ref or discovery field:\n%s", out)
	}
}

func TestPseudonymizeHandlerRedactsReturnsListByDefault(t *testing.T) {
	out := callPseudonymizedFixture(t, returnsEnvelope, map[string]any{})
	assertNoRawPII(t, out, "rita.return@example.com", "Rita", "Return", "5550444", "omar.order@example.com", "Omar", "Order")
	for _, keep := range []string{"Refund Show", "fan_ref", "RET123"} {
		if !strings.Contains(out, keep) {
			t.Errorf("scrubbed returns output dropped expected field %q:\n%s", keep, out)
		}
	}

	ret := parsedPath(t, out, "data", "returns", 0).(map[string]any)
	ticketHolder := ret["ticket"].(map[string]any)["holder"].(map[string]any)
	orderFan := ret["order"].(map[string]any)["fan"].(map[string]any)
	for name, subtree := range map[string]map[string]any{"ticket.holder": ticketHolder, "order.fan": orderFan} {
		if _, ok := subtree["fan_ref"]; !ok {
			t.Fatalf("%s missing fan_ref after scrub: %+v", name, subtree)
		}
		if _, ok := subtree["email"]; ok {
			t.Fatalf("%s retained raw email after scrub: %+v", name, subtree)
		}
		if _, ok := subtree["firstName"]; ok {
			t.Fatalf("%s retained raw firstName after scrub: %+v", name, subtree)
		}
	}
}

func TestPseudonymizeHandlerReturnsListIncludePII(t *testing.T) {
	out := callPseudonymizedFixture(t, returnsEnvelope, map[string]any{"include_pii": true})
	for _, raw := range []string{"rita.return@example.com", "Rita", "Return", "5550444", "omar.order@example.com", "Omar", "Order"} {
		if !strings.Contains(out, raw) {
			t.Errorf("include_pii returns_list output dropped raw value %q:\n%s", raw, out)
		}
	}
	if !strings.Contains(out, "fan_ref") {
		t.Fatalf("include_pii returns_list output missing fan_ref:\n%s", out)
	}
}

func TestPseudonymizeHandlerRedactsTransfersListByDefault(t *testing.T) {
	out := callPseudonymizedFixture(t, transfersEnvelope, map[string]any{})
	assertNoRawPII(t, out, "tess.transfer@example.com", "Tess Transfer", "nina.new@example.com", "Nina", "New", "5550555", "frank.from@example.com", "Frank", "From")
	for _, keep := range []string{"Transfer Show", "fan_ref", "tr-1"} {
		if !strings.Contains(out, keep) {
			t.Errorf("scrubbed transfers output dropped expected field %q:\n%s", keep, out)
		}
	}

	transfer := parsedPath(t, out, "data", "ticketTransfers", 0).(map[string]any)
	if _, ok := transfer["fan_ref"]; !ok {
		t.Fatalf("flat transfer holder fields did not produce fan_ref: %+v", transfer)
	}
	if _, ok := transfer["holder_email"]; ok {
		t.Fatalf("flat transfer holder_email survived scrub: %+v", transfer)
	}
	if _, ok := transfer["holder_name"]; ok {
		t.Fatalf("flat transfer holder_name survived scrub: %+v", transfer)
	}
	ticketHolder := transfer["tickets"].([]any)[0].(map[string]any)["holder"].(map[string]any)
	orderFan := transfer["orders"].([]any)[0].(map[string]any)["fan"].(map[string]any)
	for name, subtree := range map[string]map[string]any{"tickets[0].holder": ticketHolder, "orders[0].fan": orderFan} {
		if _, ok := subtree["fan_ref"]; !ok {
			t.Fatalf("%s missing fan_ref after scrub: %+v", name, subtree)
		}
		if _, ok := subtree["email"]; ok {
			t.Fatalf("%s retained raw email after scrub: %+v", name, subtree)
		}
	}
}

func TestPseudonymizeHandlerTransfersListIncludePII(t *testing.T) {
	out := callPseudonymizedFixture(t, transfersEnvelope, map[string]any{"include_pii": true})
	for _, raw := range []string{"tess.transfer@example.com", "Tess Transfer", "nina.new@example.com", "Nina", "New", "5550555", "frank.from@example.com", "Frank", "From"} {
		if !strings.Contains(out, raw) {
			t.Errorf("include_pii transfers_list output dropped raw value %q:\n%s", raw, out)
		}
	}
	if !strings.Contains(out, "fan_ref") {
		t.Fatalf("include_pii transfers_list output missing fan_ref:\n%s", out)
	}
}

func TestPseudonymizeHandlerRedactsExtrasListByDefault(t *testing.T) {
	out := callPseudonymizedFixture(t, extrasEnvelope, map[string]any{})
	assertNoRawPII(t, out, "ella.extra@example.com", "Ella", "Extra", "5550666")
	for _, keep := range []string{"Poster", "fan_ref", "EXT123"} {
		if !strings.Contains(out, keep) {
			t.Errorf("scrubbed extras output dropped expected field %q:\n%s", keep, out)
		}
	}

	extra := parsedPath(t, out, "data", "extras", 0).(map[string]any)
	holder := extra["holder"].(map[string]any)
	if _, ok := holder["fan_ref"]; !ok {
		t.Fatalf("extras holder missing fan_ref after scrub: %+v", holder)
	}
	for _, field := range []string{"email", "firstName", "lastName", "phoneNumber"} {
		if _, ok := holder[field]; ok {
			t.Fatalf("extras holder retained raw %s after scrub: %+v", field, holder)
		}
	}
	product := extra["product"].(map[string]any)
	if product["name"] != "Poster" {
		t.Fatalf("extras product.name = %v, want Poster", product["name"])
	}
	if extra["total"] != float64(2150) {
		t.Fatalf("extras total = %v, want 2150", extra["total"])
	}
}

func TestPseudonymizeHandlerExtrasListIncludePII(t *testing.T) {
	out := callPseudonymizedFixture(t, extrasEnvelope, map[string]any{"include_pii": true})
	for _, raw := range []string{"ella.extra@example.com", "Ella", "Extra", "5550666"} {
		if !strings.Contains(out, raw) {
			t.Errorf("include_pii extras_list output dropped raw value %q:\n%s", raw, out)
		}
	}
	for _, keep := range []string{"fan_ref", "Poster"} {
		if !strings.Contains(out, keep) {
			t.Errorf("include_pii extras_list output dropped expected field %q:\n%s", keep, out)
		}
	}
	extra := parsedPath(t, out, "data", "extras", 0).(map[string]any)
	product := extra["product"].(map[string]any)
	if product["name"] != "Poster" {
		t.Fatalf("include_pii extras product.name = %v, want Poster", product["name"])
	}
	if extra["total"] != float64(2150) {
		t.Fatalf("include_pii extras total = %v, want 2150", extra["total"])
	}
}

func TestSameHolderSameTokenAcrossTools(t *testing.T) {
	// holder email in tickets and fan id-resolved token: ensure a holder's token
	// is derived from email (holder identity) and stable.
	out, _ := scrubJSONText(ticketsEnvelope, Opts{Salt: []byte(testSalt)})
	var parsed map[string]any
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("re-parse scrubbed: %v", err)
	}
	tickets := parsed["data"].(map[string]any)["tickets"].([]any)
	h1 := tickets[0].(map[string]any)["holder"].(map[string]any)
	want := Token([]byte(testSalt), "holder1@example.com")
	if h1["fan_ref"] != want {
		t.Errorf("holder1 fan_ref = %v, want %v (token keyed on holder email)", h1["fan_ref"], want)
	}
}

func callPseudonymizedFixture(t *testing.T, body string, args map[string]any) string {
	t.Helper()
	withTempStore(t)
	handler := pseudonymizeHandler(func(_ context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		if _, ok := req.GetArguments()["include_pii"]; ok {
			t.Fatalf("include_pii was forwarded to inner handler")
		}
		return mcplib.NewToolResultText(body), nil
	})
	res, err := handler(context.Background(), mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{Arguments: args},
	})
	if err != nil {
		t.Fatalf("pseudonymizeHandler: %v", err)
	}
	if res == nil || res.IsError {
		t.Fatalf("pseudonymizeHandler returned error result: %#v", res)
	}
	return toolResultText(t, res)
}

func toolResultText(t *testing.T, res *mcplib.CallToolResult) string {
	t.Helper()
	if len(res.Content) != 1 {
		t.Fatalf("tool result content len = %d, want 1", len(res.Content))
	}
	tc, ok := res.Content[0].(mcplib.TextContent)
	if !ok {
		t.Fatalf("tool result content type = %T, want TextContent", res.Content[0])
	}
	return tc.Text
}

func parsedPath(t *testing.T, text string, path ...any) any {
	t.Helper()
	var cur any
	if err := json.Unmarshal([]byte(text), &cur); err != nil {
		t.Fatalf("unmarshal scrubbed output: %v\n%s", err, text)
	}
	for _, p := range path {
		switch key := p.(type) {
		case string:
			m, ok := cur.(map[string]any)
			if !ok {
				t.Fatalf("path %v expected object before key %q, got %T", path, key, cur)
			}
			cur = m[key]
		case int:
			a, ok := cur.([]any)
			if !ok {
				t.Fatalf("path %v expected array before index %d, got %T", path, key, cur)
			}
			if key < 0 || key >= len(a) {
				t.Fatalf("path %v index %d out of range len %d", path, key, len(a))
			}
			cur = a[key]
		default:
			t.Fatalf("unsupported path segment %T", p)
		}
	}
	return cur
}

func TestScrubJSONTextNonJSONPassthrough(t *testing.T) {
	in := "authentication error: check your token"
	out, changed := scrubJSONText(in, Opts{Salt: []byte(testSalt)})
	if changed || out != in {
		t.Errorf("non-JSON text was altered: changed=%v out=%q", changed, out)
	}
}

func TestPIIToolDescriptionsCarryNotice(t *testing.T) {
	// The shared notice must mention personal data + include_pii so a host can
	// gate auto-approval.
	for _, sub := range []string{"personal data", "include_pii", "pseudonymized"} {
		if !strings.Contains(piiToolNotice, sub) {
			t.Errorf("piiToolNotice missing %q: %q", sub, piiToolNotice)
		}
	}
}

func TestArgIsTrue(t *testing.T) {
	cases := map[any]bool{true: true, false: false, "true": true, "1": true, "false": false, "": false, nil: false, 0: false}
	for in, want := range cases {
		if got := argIsTrue(in); got != want {
			t.Errorf("argIsTrue(%v) = %v, want %v", in, got, want)
		}
	}
}
