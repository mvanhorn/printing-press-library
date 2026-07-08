package schemas

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/trust"
)

func TestValidateWritePlanAcceptsMinimalValidPlan(t *testing.T) {
	data := validWritePlanJSON(t)
	if err := ValidateWritePlan(data); err != nil {
		t.Fatalf("ValidateWritePlan: %v", err)
	}
}

func TestValidateWritePlanRejectsMissingPlanJWS(t *testing.T) {
	root := validWritePlanMap(t)
	delete(root, "plan_jws")
	data, _ := json.Marshal(root)

	if err := ValidateWritePlan(data); err == nil || !strings.Contains(err.Error(), "plan_jws") {
		t.Fatalf("ValidateWritePlan error = %v, want plan_jws failure", err)
	}
}

func TestValidateWritePlanRejectsExtraTopLevelField(t *testing.T) {
	root := validWritePlanMap(t)
	root["unexpected"] = true
	data, _ := json.Marshal(root)

	if err := ValidateWritePlan(data); err == nil || !strings.Contains(err.Error(), "unexpected") {
		t.Fatalf("ValidateWritePlan error = %v, want unexpected field failure", err)
	}
}

func TestValidateWritePlanRejectsWrongTypes(t *testing.T) {
	root := validWritePlanMap(t)
	root["version"] = "1"
	data, _ := json.Marshal(root)

	if err := ValidateWritePlan(data); err == nil || !strings.Contains(err.Error(), "version") {
		t.Fatalf("ValidateWritePlan error = %v, want version type failure", err)
	}
}

func validWritePlanJSON(t *testing.T) []byte {
	t.Helper()
	data, err := json.Marshal(validWritePlanMap(t))
	if err != nil {
		t.Fatalf("marshal plan: %v", err)
	}
	return data
}

func validWritePlanMap(t *testing.T) map[string]any {
	t.Helper()
	now := time.Now().UTC()
	return map[string]any{
		"version": 1,
		"plan_intent": map[string]any{
			"iss":         "00D000000000001",
			"sub":         "005000000000001",
			"aud":         trust.WriteIntentAudience,
			"iat":         now.Unix(),
			"exp":         now.Add(time.Hour).Unix(),
			"jti":         "01JPLANSCHEMA",
			"sobject":     "Account",
			"record_id":   "001ACME0001",
			"operation":   "update",
			"diff_sha256": strings.Repeat("a", 64),
		},
		"plan_metadata": map[string]any{
			"created_by_kid": "kid-schema",
			"created_at":     now.Format(time.RFC3339),
			"fields": map[string]any{
				"Industry": map[string]any{"before": "Tech", "after": "Fintech"},
			},
			"auth_mode":     "sf_fallthrough",
			"execute_path":  "ui_api",
			"human_summary": "Update Account industry",
		},
		"plan_jws": "header.payload.signature",
		"countersignatures": []any{map[string]any{
			"kid":       "kid-counter",
			"signed_at": now.Format(time.RFC3339),
			"jws":       "header.payload.signature",
		}},
	}
}
