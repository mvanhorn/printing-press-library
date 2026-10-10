package client

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/namecheap/internal/config"
)

// PATCH(namecheap-error-details-first): real-shaped namecheap.domains.getInfo
// envelopes. Values are synthetic; element and attribute layout matches live
// API responses.
const namecheapGetInfoOKFixture = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="OK" xmlns="http://api.namecheap.com/xml.response">
  <Errors />
  <Warnings />
  <RequestedCommand>namecheap.domains.getinfo</RequestedCommand>
  <CommandResponse Type="namecheap.domains.getInfo">
    <DomainGetInfoResult Status="Ok" ID="12345678" DomainName="example.com" OwnerName="exampleuser" IsOwner="true" IsPremium="false">
      <DomainDetails>
        <CreatedDate>01/01/2024</CreatedDate>
        <ExpiredDate>01/01/2027</ExpiredDate>
        <NumYears>0</NumYears>
      </DomainDetails>
      <LockDetails />
      <Whoisguard Enabled="True">
        <ID>87654321</ID>
        <ExpiredDate>01/01/2027</ExpiredDate>
        <EmailDetails WhoisGuardEmail="redacted@example.invalid" ForwardedTo="owner@example.invalid" LastAutoEmailChangeDate="" AutoEmailChangeFrequencyDays="0" />
      </Whoisguard>
      <PremiumDnsSubscription>
        <UseAutoRenew>false</UseAutoRenew>
        <SubscriptionId>-1</SubscriptionId>
        <CreatedDate>01/01/0001</CreatedDate>
        <ExpirationDate>01/01/0001</ExpirationDate>
        <IsActive>false</IsActive>
      </PremiumDnsSubscription>
      <DnsDetails ProviderType="CUSTOM" IsUsingOurDNS="false" HostCount="0" EmailType="" DynamicDNSStatus="false" IsFailover="false">
        <Nameserver>ns1.example.net</Nameserver>
        <Nameserver>ns2.example.net</Nameserver>
      </DnsDetails>
      <Modificationrights All="true" />
    </DomainGetInfoResult>
  </CommandResponse>
  <Server>PHX01APIEXT01</Server>
  <GMTTimeDifference>--5:00</GMTTimeDifference>
  <ExecutionTime>0.042</ExecutionTime>
</ApiResponse>`

// Namecheap returns Status="ERROR" plus an empty DomainGetInfoResult skeleton
// when DomainName is missing, so the CommandResponse alone looks like data.
const namecheapGetInfoErrorFixture = `<?xml version="1.0" encoding="utf-8"?>
<ApiResponse Status="ERROR" xmlns="http://api.namecheap.com/xml.response">
  <Errors>
    <Error Number="2010166">Parameter DomainName is Missing</Error>
  </Errors>
  <Warnings />
  <RequestedCommand>namecheap.domains.getinfo</RequestedCommand>
  <CommandResponse Type="namecheap.domains.getInfo">
    <DomainGetInfoResult ID="0" IsOwner="false" IsPremium="false">
      <DomainDetails>
        <NumYears>0</NumYears>
      </DomainDetails>
      <LockDetails />
      <Whoisguard>
        <ID>0</ID>
      </Whoisguard>
      <PremiumDnsSubscription>
        <UseAutoRenew>false</UseAutoRenew>
        <SubscriptionId>-1</SubscriptionId>
        <CreatedDate>01/01/0001</CreatedDate>
        <ExpirationDate>01/01/0001</ExpirationDate>
        <IsActive>false</IsActive>
      </PremiumDnsSubscription>
      <DnsDetails IsUsingOurDNS="false" HostCount="0" DynamicDNSStatus="false" IsFailover="false" />
      <Modificationrights />
    </DomainGetInfoResult>
  </CommandResponse>
  <Server>PHX01APIEXT01</Server>
  <GMTTimeDifference>--5:00</GMTTimeDifference>
  <ExecutionTime>0.010</ExecutionTime>
</ApiResponse>`

func newEnvelopeTestClient(t *testing.T, body string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/xml")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	cfg := &config.Config{BaseURL: srv.URL, APIUser: "user", APIKey: "key", ClientIP: "203.0.113.10"}
	c := New(cfg, time.Second, 0)
	c.NoCache = true
	return c
}

func TestNamecheapStatusOKEnvelopeIsSuccess(t *testing.T) {
	c := newEnvelopeTestClient(t, namecheapGetInfoOKFixture)
	data, err := c.Get("/xml.response/domains/get-info", map[string]string{"DomainName": "example.com"})
	if err != nil {
		t.Fatalf("Status=OK envelope returned error: %v", err)
	}
	var decoded map[string]map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("response is not JSON: %v", err)
	}
	api := decoded["ApiResponse"]
	if api["Status"] != "OK" {
		t.Fatalf("ApiResponse.Status = %#v, want OK (converter must keep root attributes)", api["Status"])
	}
	result := api["CommandResponse"].(map[string]any)["DomainGetInfoResult"].(map[string]any)
	if result["DomainName"] != "example.com" {
		t.Fatalf("DomainGetInfoResult.DomainName = %#v", result["DomainName"])
	}
}

func TestNamecheapStatusErrorEnvelopeSurfacesErrorNumberAndText(t *testing.T) {
	c := newEnvelopeTestClient(t, namecheapGetInfoErrorFixture)
	_, err := c.Get("/xml.response/domains/get-info", nil)
	if err == nil {
		t.Fatal("Status=ERROR envelope must return an error")
	}
	var ncErr *NamecheapAPIError
	if !errors.As(err, &ncErr) {
		t.Fatalf("error type = %T, want *NamecheapAPIError", err)
	}
	if ncErr.Status != "ERROR" {
		t.Fatalf("Status = %q, want ERROR", ncErr.Status)
	}
	if len(ncErr.Errors) != 1 || ncErr.Errors[0].Number != "2010166" || ncErr.Errors[0].Message != "Parameter DomainName is Missing" {
		t.Fatalf("Errors = %#v", ncErr.Errors)
	}
	msg := err.Error()
	if !strings.Contains(msg, "status ERROR: error 2010166: Parameter DomainName is Missing") {
		t.Fatalf("error message should lead with the Namecheap error, got %q", msg)
	}
	if strings.Contains(msg, "DomainGetInfoResult") {
		t.Fatalf("error message should not dump the empty CommandResponse skeleton, got %q", msg)
	}
}

func TestNamecheapStatusErrorEnvelopeWithMultipleErrors(t *testing.T) {
	body := `<ApiResponse Status="ERROR"><Errors><Error Number="1011102">API Key is invalid or API access has not been enabled</Error><Error Number="1011150">Invalid request IP: 198.51.100.7</Error></Errors><CommandResponse /></ApiResponse>`
	converted, ok := convertNamecheapXMLToJSON([]byte(body))
	if !ok {
		t.Fatal("expected XML body to convert")
	}
	err := detectNamecheapAPIError("GET", "/xml.response", converted)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{"error 1011102: API Key is invalid", "error 1011150: Invalid request IP"} {
		if !strings.Contains(msg, want) {
			t.Fatalf("message %q missing %q", msg, want)
		}
	}
}

func TestNamecheapStatusErrorWithoutDetailsFallsBackToBody(t *testing.T) {
	converted, ok := convertNamecheapXMLToJSON([]byte(`<ApiResponse Status="ERROR"><Errors /></ApiResponse>`))
	if !ok {
		t.Fatal("expected XML body to convert")
	}
	err := detectNamecheapAPIError("GET", "/xml.response", converted)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), `"Status":"ERROR"`) {
		t.Fatalf("fallback message should include body, got %q", err.Error())
	}
}
