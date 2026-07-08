package security

import (
	"context"
	"strings"
	"testing"
)

func TestContentScanReplacesPIITokens(t *testing.T) {
	record := &Record{SObject: "Case", Fields: map[string]any{
		"Id":          "500",
		"Description": "Email john@example.com, SSN 123-45-6789, phone +1 415-555-1212, card 4111 1111 1111 1111.",
	}, Provenance: &Provenance{Redactions: map[string]int{}}}

	got := (ContentScanFilter{}).Apply(context.Background(), record)
	text := got.Fields["Description"].(string)
	for _, token := range []string{"{{PII:email}}", "{{PII:ssn}}", "{{PII:phone}}", "{{PII:credit_card}}"} {
		if !strings.Contains(text, token) {
			t.Fatalf("description %q missing token %s", text, token)
		}
	}
	if got.Provenance.Counters.ContentScan != 1 {
		t.Fatalf("content scan counter = %d, want 1", got.Provenance.Counters.ContentScan)
	}
}
