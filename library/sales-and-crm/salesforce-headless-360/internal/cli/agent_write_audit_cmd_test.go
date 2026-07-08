package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/store"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/trust"
)

func TestAgentWriteAuditListFiltersAndJSON(t *testing.T) {
	signer := setupWriteAuditRows(t, []store.WriteAuditRow{
		{JTI: "01JLIST001", TargetSObject: "Account", TargetRecordID: "001A", Operation: "update", ExecutionStatus: "executed", GeneratedAt: "2026-04-22T18:03:00Z"},
		{JTI: "01JLIST002", TargetSObject: "Contact", TargetRecordID: "003A", Operation: "update", ExecutionStatus: "rejected", GeneratedAt: "2026-04-22T18:02:00Z", ExecutionError: "FLS_WRITE_DENIED"},
		{JTI: "01JLIST003", TargetSObject: "Account", TargetRecordID: "001B", Operation: "create", ExecutionStatus: "executed", GeneratedAt: "2026-04-22T18:01:00Z"},
	})

	out, err := runWriteAuditCommand(t, false, "list")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if !strings.Contains(out, "01JLIST001") || !strings.Contains(out, "01JLIST002") || !strings.Contains(out, "01JLIST003") {
		t.Fatalf("list output missing rows:\n%s", out)
	}
	if strings.Index(out, "01JLIST001") > strings.Index(out, "01JLIST003") {
		t.Fatalf("rows not ordered by generated_at desc:\n%s", out)
	}

	out, err = runWriteAuditCommand(t, false, "list", "--status", "executed")
	if err != nil {
		t.Fatalf("list --status: %v", err)
	}
	if strings.Contains(out, "01JLIST002") || strings.Count(out, "executed") != 2 {
		t.Fatalf("status filter output mismatch:\n%s", out)
	}

	out, err = runWriteAuditCommand(t, false, "list", "--sobject", "Account", "--limit", "1")
	if err != nil {
		t.Fatalf("list --sobject --limit: %v", err)
	}
	if !strings.Contains(out, "01JLIST001") || strings.Contains(out, "01JLIST003") {
		t.Fatalf("sobject limit output mismatch:\n%s", out)
	}

	out, err = runWriteAuditCommand(t, true, "list", "--kid", signer.KID())
	if err != nil {
		t.Fatalf("list --json: %v", err)
	}
	var rows []store.WriteAuditRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("json output invalid: %v\n%s", err, out)
	}
	if len(rows) != 3 || rows[0].IntentJWS == "" {
		t.Fatalf("json rows = %#v", rows)
	}
}

func TestAgentWriteAuditInspectShowsDecodedJWSAndMissingRow(t *testing.T) {
	setupWriteAuditRows(t, []store.WriteAuditRow{
		{JTI: "01JINSPECT", TargetSObject: "Account", TargetRecordID: "001A", Operation: "update", ExecutionStatus: "executed", GeneratedAt: "2026-04-22T18:03:00Z"},
	})

	out, err := runWriteAuditCommand(t, false, "inspect", "01JINSPECT")
	if err != nil {
		t.Fatalf("inspect: %v", err)
	}
	for _, want := range []string{"jti: 01JINSPECT", "intent_jws:", "header:", "claims:", "field_diff:"} {
		if !strings.Contains(out, want) {
			t.Fatalf("inspect output missing %q:\n%s", want, out)
		}
	}

	_, err = runWriteAuditCommand(t, false, "inspect", "missing-jti")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing inspect error = %v, want not found", err)
	}
}

func TestAgentWriteAuditVerifyValidTamperedAndPlanReference(t *testing.T) {
	setupWriteAuditRows(t, []store.WriteAuditRow{
		{JTI: "01JVERIFYOK", TargetSObject: "Account", TargetRecordID: "001A", Operation: "update", ExecutionStatus: "executed", GeneratedAt: "2026-04-22T18:03:00Z"},
		{JTI: "01JVERIFYBAD", TargetSObject: "Account", TargetRecordID: "001B", Operation: "update", ExecutionStatus: "executed", GeneratedAt: "2026-04-22T18:02:00Z"},
		{JTI: "01JVERIFYPLAN", TargetSObject: "Account", TargetRecordID: "001C", Operation: "update", ExecutionStatus: "executed", GeneratedAt: "2026-04-22T18:01:00Z", FieldDiff: `{"plan_jti":"01JVERIFYPLAN"}`},
	})
	tamperWriteAuditJWS(t, "01JVERIFYBAD")

	out, err := runWriteAuditCommand(t, true, "verify", "01JVERIFYOK")
	if err != nil {
		t.Fatalf("verify valid: %v", err)
	}
	var valid writeAuditVerifyResult
	if err := json.Unmarshal([]byte(out), &valid); err != nil {
		t.Fatalf("valid verify json: %v\n%s", err, out)
	}
	if !valid.SignatureValid || !valid.AudienceValid || !valid.NotExpired {
		t.Fatalf("valid verify result = %#v", valid)
	}

	out, err = runWriteAuditCommand(t, true, "verify", "01JVERIFYBAD")
	if err != nil {
		t.Fatalf("verify tampered: %v", err)
	}
	var tampered writeAuditVerifyResult
	if err := json.Unmarshal([]byte(out), &tampered); err != nil {
		t.Fatalf("tampered verify json: %v\n%s", err, out)
	}
	if tampered.SignatureValid {
		t.Fatalf("tampered signature_valid = true")
	}

	out, err = runWriteAuditCommand(t, true, "verify", "01JVERIFYPLAN")
	if err != nil {
		t.Fatalf("verify plan: %v", err)
	}
	var plan writeAuditVerifyResult
	if err := json.Unmarshal([]byte(out), &plan); err != nil {
		t.Fatalf("plan verify json: %v\n%s", err, out)
	}
	if plan.PlanJTI != "01JVERIFYPLAN" || plan.PlanSignatureValid == nil || !*plan.PlanSignatureValid {
		t.Fatalf("plan verify result = %#v", plan)
	}
}

func TestAgentWriteAuditHelpRenders(t *testing.T) {
	for _, args := range [][]string{
		{"list", "--help"},
		{"inspect", "--help"},
		{"verify", "--help"},
	} {
		out, err := runWriteAuditCommand(t, false, args...)
		if err != nil {
			t.Fatalf("%v help: %v", args, err)
		}
		if !strings.Contains(out, "Usage:") {
			t.Fatalf("%v help missing Usage:\n%s", args, out)
		}
	}
}

func runWriteAuditCommand(t *testing.T, asJSON bool, args ...string) (string, error) {
	t.Helper()
	flags := &rootFlags{asJSON: asJSON}
	cmd := newAgentWriteAuditCmd(flags)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func setupWriteAuditRows(t *testing.T, rows []store.WriteAuditRow) *trust.FileSigner {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	signer, err := trust.NewFileSignerWithIdentity("prod", "host123456", "005USER")
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
		IssuerUserID:    "005USER",
		RegisteredAt:    time.Now().UTC(),
		Source:          "local-generated",
	}); err != nil {
		t.Fatalf("SaveKeyRecord: %v", err)
	}
	db, err := store.Open(defaultDBPath("salesforce-headless-360-pp-cli"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer db.Close()
	for _, row := range rows {
		row.ActingUser = "005USER"
		row.ActingKID = signer.KID()
		if row.FieldDiff == "" {
			row.FieldDiff = `{"Name":{"before":"Old","after":"New"}}`
		}
		if row.WriteStatus == "" {
			row.WriteStatus = "executed"
		}
		claims := trust.WriteIntentClaims{
			Iss:        "prod",
			Sub:        "005USER",
			Aud:        trust.WriteIntentAudience,
			Iat:        time.Now().UTC().Add(-time.Minute).Unix(),
			Exp:        time.Now().UTC().Add(time.Hour).Unix(),
			Jti:        row.JTI,
			SObject:    row.TargetSObject,
			RecordID:   row.TargetRecordID,
			Operation:  row.Operation,
			DiffSha256: strings.Repeat("a", 64),
		}
		jws, err := trust.SignWriteIntent(signer, claims)
		if err != nil {
			t.Fatalf("SignWriteIntent: %v", err)
		}
		row.IntentJWS = string(jws)
		if err := db.InsertWriteAudit(row); err != nil {
			t.Fatalf("InsertWriteAudit(%s): %v", row.JTI, err)
		}
	}
	return signer
}

func tamperWriteAuditJWS(t *testing.T, jti string) {
	t.Helper()
	db, err := store.Open(defaultDBPath("salesforce-headless-360-pp-cli"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	defer db.Close()
	row, err := db.GetWriteAudit(jti)
	if err != nil {
		t.Fatalf("GetWriteAudit: %v", err)
	}
	if strings.HasSuffix(row.IntentJWS, "A") {
		row.IntentJWS = row.IntentJWS[:len(row.IntentJWS)-1] + "B"
	} else {
		row.IntentJWS = row.IntentJWS[:len(row.IntentJWS)-1] + "A"
	}
	if err := db.InsertWriteAudit(row); err != nil {
		t.Fatalf("InsertWriteAudit tampered: %v", err)
	}
}
