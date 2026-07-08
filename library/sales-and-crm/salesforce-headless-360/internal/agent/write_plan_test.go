package agent

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/trust"
)

func TestBuildPlanSignsAndVerifies(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	signer := writePlanTestSigner(t, "005PLANNER")
	fake := &recordingWriteClient{
		getBody: json.RawMessage(`{"envelope":{"fields":{"Industry":{"value":"Tech"}},"id":"001ACME0001","lastModifiedDate":"2026-04-18T14:00:00Z"}}`),
	}

	plan, err := BuildPlan(contextWithTestDeadline(t), WriteOptions{
		Operation:     WriteOperationUpdate,
		SObject:       "Account",
		RecordID:      "001ACME0001",
		Fields:        map[string]any{"Industry": "Fintech"},
		Client:        fake,
		Filter:        allowAllWriteFilter{},
		Signer:        signer,
		OrgAlias:      "00D000000000001",
		ActingUser:    "005PLANNER",
		Now:           time.Now,
		PlanExpiresIn: time.Hour,
	}, "sf_fallthrough", "ui_api", "Update account industry")
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if plan.PlanJWS == "" {
		t.Fatal("expected plan_jws")
	}
	if plan.PlanIntent.Jti == "" {
		t.Fatal("expected plan jti")
	}
	if got := plan.PlanMetadata.Fields["Industry"]["before"]; got != "Tech" {
		t.Fatalf("before = %v, want Tech", got)
	}
	if err := VerifyPlan(plan, 0); err != nil {
		t.Fatalf("VerifyPlan: %v", err)
	}
}

func TestAppendCountersignatureAndVerifyMinimum(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	planner := writePlanTestSigner(t, "005PLANNER")
	counter := writePlanTestSigner(t, "005COUNTER")
	counter2 := writePlanTestSigner(t, "005COUNTER2")
	plan := signedTestPlan(t, planner, time.Now().UTC().Add(time.Hour))

	if err := AppendCountersignature(plan, counter); err != nil {
		t.Fatalf("AppendCountersignature: %v", err)
	}
	if len(plan.Countersignatures) != 1 {
		t.Fatalf("countersignatures = %d, want 1", len(plan.Countersignatures))
	}
	if err := VerifyPlan(plan, 1); err != nil {
		t.Fatalf("VerifyPlan with one countersig: %v", err)
	}
	if err := VerifyPlan(plan, 2); !errors.Is(err, ErrInsufficientCountersignatures) {
		t.Fatalf("VerifyPlan error = %v, want %v", err, ErrInsufficientCountersignatures)
	}
	if err := AppendCountersignature(plan, counter2); err != nil {
		t.Fatalf("AppendCountersignature second signer: %v", err)
	}
	if err := VerifyPlan(plan, 2); err != nil {
		t.Fatalf("VerifyPlan with two countersigs: %v", err)
	}
}

func TestVerifyPlanRejectsExpiredWrongAudienceAndTampering(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	signer := writePlanTestSigner(t, "005PLANNER")

	expired := signedTestPlan(t, signer, time.Now().UTC().Add(-time.Minute))
	if err := VerifyPlan(expired, 0); !errors.Is(err, ErrPlanExpired) {
		t.Fatalf("expired VerifyPlan error = %v, want %v", err, ErrPlanExpired)
	}

	wrongAudience := signedTestPlan(t, signer, time.Now().UTC().Add(time.Hour))
	wrongAudience.PlanIntent.Aud = "agent-context"
	resignPlanForTest(t, wrongAudience, signer)
	if err := VerifyPlan(wrongAudience, 0); !errors.Is(err, trust.ErrWrongAudience) {
		t.Fatalf("wrong audience VerifyPlan error = %v, want %v", err, trust.ErrWrongAudience)
	}

	tampered := signedTestPlan(t, signer, time.Now().UTC().Add(time.Hour))
	tampered.PlanIntent.DiffSha256 = strings.Repeat("b", 64)
	if err := VerifyPlan(tampered, 0); !errors.Is(err, ErrPlanSignatureInvalid) {
		t.Fatalf("tampered VerifyPlan error = %v, want %v", err, ErrPlanSignatureInvalid)
	}
}

func TestVerifyPlanRejectsInvalidCountersignature(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	planner := writePlanTestSigner(t, "005PLANNER")
	counter := writePlanTestSigner(t, "005COUNTER")
	plan := signedTestPlan(t, planner, time.Now().UTC().Add(time.Hour))
	if err := AppendCountersignature(plan, counter); err != nil {
		t.Fatalf("AppendCountersignature: %v", err)
	}
	plan.Countersignatures[0].JWS = plan.PlanJWS

	if err := VerifyPlan(plan, 1); !errors.Is(err, ErrCountersignatureInvalid) {
		t.Fatalf("VerifyPlan error = %v, want %v", err, ErrCountersignatureInvalid)
	}
}

func TestExecutePlanUsesPlanJTIAndAuditMetadata(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	signer := writePlanTestSigner(t, "005PLANNER")
	audit := &recordingWriteAudit{}
	fake := &recordingWriteClient{
		getBody:      json.RawMessage(`{"envelope":{"fields":{"Industry":{"value":"Tech"}},"id":"001ACME0001","lastModifiedDate":"2026-04-18T14:00:00Z"}}`),
		patchBody:    json.RawMessage(`{"envelope":{"id":"001ACME0001","success":true}}`),
		patchStatus:  http.StatusOK,
		patchHeaders: http.Header{},
	}
	plan, err := BuildPlan(contextWithTestDeadline(t), WriteOptions{
		Operation:     WriteOperationUpdate,
		SObject:       "Account",
		RecordID:      "001ACME0001",
		Fields:        map[string]any{"Industry": "Fintech"},
		Client:        fake,
		Filter:        allowAllWriteFilter{},
		Signer:        signer,
		OrgAlias:      "00D000000000001",
		ActingUser:    "005PLANNER",
		Now:           time.Now,
		PlanExpiresIn: time.Hour,
	}, "sf_fallthrough", "ui_api", "")
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}

	result, err := ExecutePlan(contextWithTestDeadline(t), plan, ExecuteOptions{
		Client:      fake,
		Filter:      allowAllWriteFilter{},
		Signer:      signer,
		AuditWriter: audit,
		OrgAlias:    "00D000000000001",
		ActingUser:  "005PLANNER",
		Now:         time.Now,
	})
	if err != nil {
		t.Fatalf("ExecutePlan: %v", err)
	}
	if result.JTI != plan.PlanIntent.Jti {
		t.Fatalf("result JTI = %s, want plan JTI %s", result.JTI, plan.PlanIntent.Jti)
	}
	if audit.pendingJTI != plan.PlanIntent.Jti {
		t.Fatalf("audit pending JTI = %s, want %s", audit.pendingJTI, plan.PlanIntent.Jti)
	}
	if audit.fieldDiff["plan_jti"] != plan.PlanIntent.Jti {
		t.Fatalf("audit plan_jti = %v, want %s", audit.fieldDiff["plan_jti"], plan.PlanIntent.Jti)
	}
}

func writePlanTestSigner(t *testing.T, userID string) *trust.FileSigner {
	t.Helper()
	signer, err := trust.NewFileSignerWithIdentity("prod", "host123456", userID)
	if err != nil {
		t.Fatalf("NewFileSignerWithIdentity: %v", err)
	}
	if err := trust.SaveKeyRecord(trust.KeyRecord{
		KID:             signer.KID(),
		OrgAlias:        "prod",
		OrgID:           "00D000000000001",
		Algorithm:       "Ed25519",
		PublicKeyPEM:    signer.PublicKeyPEM(),
		HostFingerprint: "host123456",
		IssuerUserID:    userID,
		RegisteredAt:    time.Now().UTC(),
		Source:          "local-generated",
	}); err != nil {
		t.Fatalf("SaveKeyRecord: %v", err)
	}
	return signer
}

func signedTestPlan(t *testing.T, signer *trust.FileSigner, exp time.Time) *WritePlan {
	t.Helper()
	plan := &WritePlan{
		Version: 1,
		PlanIntent: trust.WriteIntentClaims{
			Iss:        "00D000000000001",
			Sub:        "005PLANNER",
			Aud:        trust.WriteIntentAudience,
			Iat:        time.Now().UTC().Add(-time.Minute).Unix(),
			Exp:        exp.UTC().Unix(),
			Jti:        "01JPLANTEST",
			SObject:    "Account",
			RecordID:   "001ACME0001",
			Operation:  WriteOperationUpdate,
			DiffSha256: strings.Repeat("a", 64),
		},
		PlanMetadata: PlanMetadata{
			CreatedByKID: signer.KID(),
			CreatedAt:    time.Now().UTC(),
			Fields: map[string]map[string]any{
				"Industry": {"before": "Tech", "after": "Fintech"},
			},
			AuthMode:     "sf_fallthrough",
			ExecutePath:  "ui_api",
			HumanSummary: "Update Account industry",
		},
	}
	resignPlanForTest(t, plan, signer)
	return plan
}

func resignPlanForTest(t *testing.T, plan *WritePlan, signer *trust.FileSigner) {
	t.Helper()
	jws, err := signPlanPayload(plan.PlanIntent, plan.PlanMetadata, signer)
	if err != nil {
		t.Fatalf("signPlanPayload: %v", err)
	}
	plan.PlanJWS = jws
}
