package cli

import (
	"github.com/mvanhorn/printing-press-library/library/marketing/sendfox/internal/contract"
	"github.com/mvanhorn/printing-press-library/library/marketing/sendfox/internal/evidence"
	"testing"
)

func TestAuditCSVContactsFindsInvalidAndDuplicateEmails(t *testing.T) {
	r := evidence.ReconcileCSV([]evidence.Row{{"email": "Reader@example.com"}, {"email": "bad-email"}, {"email": "reader@example.com"}}, evidence.Snapshot{})
	reasons := map[string]int{}
	for _, row := range r.Data["skips"].([]evidence.Row) {
		reasons[row["reason"].(string)]++
	}
	if reasons["invalid_email"] != 1 || reasons["duplicate_input"] != 1 || r.Data["to_create_count"] != 1 {
		t.Fatal(r.Data)
	}
}
func TestCapabilitiesReflectCurrentCampaignAPIAndWebhookGap(t *testing.T) {
	if op, _ := contract.Match("POST", "/campaigns"); op == nil {
		t.Fatal("current campaign creation missing")
	}
	for _, method := range []string{"GET", "POST", "DELETE"} {
		if op, _ := contract.Match(method, "/webhooks"); op != nil {
			t.Fatal("invented webhook capability")
		}
	}
}
