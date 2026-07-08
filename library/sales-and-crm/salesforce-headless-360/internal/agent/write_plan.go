package agent

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/security"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/trust"
)

var (
	ErrPlanExpired                   = errors.New("PLAN_EXPIRED")
	ErrPlanSignatureInvalid          = errors.New("PLAN_SIGNATURE_INVALID")
	ErrCountersignatureInvalid       = errors.New("COUNTERSIGNATURE_INVALID")
	ErrInsufficientCountersignatures = errors.New("INSUFFICIENT_COUNTERSIGNATURES")
)

type WritePlan struct {
	Version           int                     `json:"version"`
	PlanIntent        trust.WriteIntentClaims `json:"plan_intent"`
	PlanMetadata      PlanMetadata            `json:"plan_metadata"`
	PlanJWS           string                  `json:"plan_jws"`
	Countersignatures []Countersignature      `json:"countersignatures,omitempty"`
}

type PlanMetadata struct {
	CreatedByKID string                    `json:"created_by_kid"`
	CreatedAt    time.Time                 `json:"created_at"`
	Fields       map[string]map[string]any `json:"fields,omitempty"`
	AuthMode     string                    `json:"auth_mode"`
	ExecutePath  string                    `json:"execute_path"`
	HumanSummary string                    `json:"human_summary"`
	DryRun       bool                      `json:"dry_run,omitempty"`
}

type Countersignature struct {
	KID      string    `json:"kid"`
	SignedAt time.Time `json:"signed_at"`
	JWS      string    `json:"jws"`
}

type ExecuteOptions struct {
	MinCountersignatures   int
	Client                 WriteClient
	Filter                 security.WriteFilter
	Signer                 WriteIntentSigner
	AuditWriter            WriteAuditWriter
	AuthMethod             string
	ApexCompanionInstalled bool
	OrgAlias               string
	OrgID                  string
	ActingUser             string
	RunAsUser              string
	ForceStale             bool
	DryRun                 bool
	ConfirmBulk            int
	Now                    func() time.Time
	LogWarn                func(format string, args ...any)
}

type planSigner interface {
	Sign(payload []byte) ([]byte, error)
	KID() string
}

type planPayload struct {
	PlanIntent   trust.WriteIntentClaims `json:"plan_intent"`
	PlanMetadata PlanMetadata            `json:"plan_metadata"`
}

type countersignaturePayload struct {
	PlanJWS  string    `json:"plan_jws"`
	SignedAt time.Time `json:"signed_at"`
}

func BuildPlan(ctx context.Context, opts WriteOptions, authMode string, executePath string, humanSummary string) (*WritePlan, error) {
	opts = normalizeWriteOptions(opts)
	if err := validatePlanBuildOptions(opts); err != nil {
		return nil, err
	}
	signer, ok := opts.Signer.(planSigner)
	if !ok {
		return nil, fmt.Errorf("plan signer must support raw JWS signing")
	}

	before := map[string]any{}
	var beforeLMD time.Time
	var err error
	if opts.Operation == WriteOperationUpdate {
		before, beforeLMD, err = fetchBeforeState(ctx, opts)
		if err != nil {
			return nil, err
		}
		if opts.IfLastModified.IsZero() {
			opts.IfLastModified = beforeLMD
		}
	}

	filtered, _ := filterWritableFields(opts)
	if len(filtered) == 0 {
		return nil, newWriteError("FLS_WRITE_DENIED", http.StatusForbidden, opts, "no requested fields are writeable for this user", "")
	}
	if opts.Operation == WriteOperationCreate && opts.IdempotencyKey != "" && opts.SObject != "FeedItem" {
		filtered["SF360_Idempotency_Key__c"] = opts.IdempotencyKey
	}

	diffBefore := before
	if opts.Operation != WriteOperationUpdate {
		diffBefore = map[string]any{}
	}
	diff := ComputeFieldDiff(diffBefore, filtered)
	diffJSON, err := json.Marshal(diff)
	if err != nil {
		return nil, fmt.Errorf("marshal field diff: %w", err)
	}
	sum := sha256.Sum256(diffJSON)
	now := opts.Now().UTC()
	expiresIn := opts.PlanExpiresIn
	if expiresIn == 0 {
		expiresIn = time.Hour
	}
	if humanSummary == "" {
		humanSummary = defaultPlanSummary(opts, diff)
	}

	intent := trust.WriteIntentClaims{
		Iss:            firstNonEmptyString(opts.OrgAlias, opts.OrgID, "salesforce-headless-360"),
		Sub:            firstNonEmptyString(opts.ActingUser, opts.RunAsUser, "unknown-user"),
		Aud:            trust.WriteIntentAudience,
		Iat:            now.Unix(),
		Exp:            now.Add(expiresIn).Unix(),
		Jti:            firstNonEmptyString(opts.JTI, trust.GenerateTraceID()),
		SObject:        opts.SObject,
		RecordID:       opts.RecordID,
		Operation:      opts.Operation,
		DiffSha256:     hex.EncodeToString(sum[:]),
		IdempotencyKey: opts.IdempotencyKey,
		IfLastModified: formatOptionalTime(opts.IfLastModified),
	}
	metadata := PlanMetadata{
		CreatedByKID: signer.KID(),
		CreatedAt:    now,
		Fields:       diff,
		AuthMode:     authMode,
		ExecutePath:  executePath,
		HumanSummary: humanSummary,
		DryRun:       opts.DryRun,
	}
	jws, err := signPlanPayload(intent, metadata, signer)
	if err != nil {
		return nil, err
	}
	return &WritePlan{
		Version:           1,
		PlanIntent:        intent,
		PlanMetadata:      metadata,
		PlanJWS:           jws,
		Countersignatures: []Countersignature{},
	}, nil
}

func VerifyPlan(plan *WritePlan, minCountersigs int) error {
	if plan == nil {
		return ErrPlanSignatureInvalid
	}
	payload, err := verifyJWSFromLocalKey(plan.PlanJWS)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrPlanSignatureInvalid, err)
	}
	var signed planPayload
	if err := json.Unmarshal(payload, &signed); err != nil {
		return fmt.Errorf("%w: parse payload: %v", ErrPlanSignatureInvalid, err)
	}
	if signed.PlanIntent.Aud != trust.WriteIntentAudience {
		return trust.ErrWrongAudience
	}
	if signed.PlanIntent.Exp <= time.Now().UTC().Unix() {
		return ErrPlanExpired
	}
	if !jsonEqual(planPayload{PlanIntent: plan.PlanIntent, PlanMetadata: plan.PlanMetadata}, signed) {
		return ErrPlanSignatureInvalid
	}
	if len(plan.Countersignatures) < minCountersigs {
		return ErrInsufficientCountersignatures
	}
	for _, sig := range plan.Countersignatures {
		if err := verifyCountersignature(plan.PlanJWS, sig); err != nil {
			return err
		}
	}
	return nil
}

func AppendCountersignature(plan *WritePlan, signer trust.Signer) error {
	if plan == nil {
		return ErrPlanSignatureInvalid
	}
	if signer == nil {
		return fmt.Errorf("signer required")
	}
	signedAt := time.Now().UTC()
	payload, err := json.Marshal(countersignaturePayload{PlanJWS: plan.PlanJWS, SignedAt: signedAt})
	if err != nil {
		return err
	}
	jws, err := trust.SignJWS(signer, payload)
	if err != nil {
		return err
	}
	plan.Countersignatures = append(plan.Countersignatures, Countersignature{
		KID:      signer.KID(),
		SignedAt: signedAt,
		JWS:      jws,
	})
	return nil
}

func ExecutePlan(ctx context.Context, plan *WritePlan, opts ExecuteOptions) (*WriteResult, error) {
	if err := VerifyPlan(plan, opts.MinCountersignatures); err != nil {
		return nil, err
	}
	writeOpts, err := writeOptionsFromPlan(plan, opts)
	if err != nil {
		return nil, err
	}
	if plan.PlanIntent.SObject == "FeedItem" {
		return ExecuteNote(ctx, writeOpts)
	}
	if plan.PlanIntent.SObject == "Case" {
		if _, ok := writeOpts.Fields["Resolution__c"]; ok {
			return ExecuteCloseCase(ctx, writeOpts)
		}
	}
	return ExecuteWrite(ctx, writeOpts)
}

func signPlanPayload(intent trust.WriteIntentClaims, metadata PlanMetadata, signer planSigner) (string, error) {
	payload, err := json.Marshal(planPayload{PlanIntent: intent, PlanMetadata: metadata})
	if err != nil {
		return "", fmt.Errorf("marshal plan payload: %w", err)
	}
	return trust.SignJWS(signer, payload)
}

func verifyCountersignature(planJWS string, sig Countersignature) error {
	if sig.JWS == "" || sig.KID == "" {
		return ErrCountersignatureInvalid
	}
	payload, err := verifyJWSFromLocalKey(sig.JWS)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrCountersignatureInvalid, err)
	}
	var signed countersignaturePayload
	if err := json.Unmarshal(payload, &signed); err != nil {
		return fmt.Errorf("%w: parse payload: %v", ErrCountersignatureInvalid, err)
	}
	if signed.PlanJWS != planJWS || !signed.SignedAt.Equal(sig.SignedAt) {
		return ErrCountersignatureInvalid
	}
	kid, err := trust.ExtractKIDUnsafe(sig.JWS)
	if err != nil || kid != sig.KID {
		return ErrCountersignatureInvalid
	}
	return nil
}

func verifyJWSFromLocalKey(jws string) ([]byte, error) {
	kid, err := trust.ExtractKIDUnsafe(jws)
	if err != nil {
		return nil, err
	}
	record, err := trust.LoadKeyRecord(kid)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, trust.ErrUnknownKID
		}
		return nil, err
	}
	pub, err := trust.ParsePublicKeyPEM(record.PublicKeyPEM)
	if err != nil {
		return nil, err
	}
	payload, _, err := trust.VerifyJWS(jws, pub)
	return payload, err
}

func writeOptionsFromPlan(plan *WritePlan, opts ExecuteOptions) (WriteOptions, error) {
	fields := map[string]any{}
	for _, key := range sortedPlanFieldKeys(plan.PlanMetadata.Fields) {
		fields[key] = plan.PlanMetadata.Fields[key]["after"]
	}
	var ifLastModified time.Time
	var err error
	if plan.PlanIntent.IfLastModified != "" {
		ifLastModified, err = time.Parse(time.RFC3339, plan.PlanIntent.IfLastModified)
		if err != nil {
			return WriteOptions{}, fmt.Errorf("invalid plan if_last_modified: %w", err)
		}
	}
	return WriteOptions{
		Operation:              plan.PlanIntent.Operation,
		SObject:                plan.PlanIntent.SObject,
		RecordID:               plan.PlanIntent.RecordID,
		Fields:                 fields,
		IdempotencyKey:         plan.PlanIntent.IdempotencyKey,
		IfLastModified:         ifLastModified,
		ForceStale:             opts.ForceStale,
		DryRun:                 opts.DryRun || plan.PlanMetadata.DryRun,
		RunAsUser:              opts.RunAsUser,
		ConfirmBulk:            opts.ConfirmBulk,
		RecordCount:            1,
		Client:                 opts.Client,
		Filter:                 opts.Filter,
		Signer:                 opts.Signer,
		AuditWriter:            opts.AuditWriter,
		AuthMethod:             opts.AuthMethod,
		ApexCompanionInstalled: opts.ApexCompanionInstalled,
		OrgAlias:               firstNonEmptyString(opts.OrgAlias, plan.PlanIntent.Iss),
		OrgID:                  opts.OrgID,
		ActingUser:             firstNonEmptyString(opts.ActingUser, plan.PlanIntent.Sub),
		Now:                    opts.Now,
		LogWarn:                opts.LogWarn,
		JTI:                    plan.PlanIntent.Jti,
		AuditMetadata:          map[string]any{"plan_jti": plan.PlanIntent.Jti},
	}, nil
}

func validatePlanBuildOptions(opts WriteOptions) error {
	if opts.Operation != WriteOperationUpdate && opts.Operation != WriteOperationUpsert && opts.Operation != WriteOperationCreate {
		return fmt.Errorf("unsupported write operation %q", opts.Operation)
	}
	if opts.Client == nil {
		return fmt.Errorf("write client is required")
	}
	if opts.Signer == nil {
		return fmt.Errorf("write intent signer is required")
	}
	if opts.SObject == "" {
		return fmt.Errorf("--sobject is required")
	}
	if opts.Operation == WriteOperationUpdate && opts.RecordID == "" {
		return fmt.Errorf("record id is required for update")
	}
	if (opts.Operation == WriteOperationCreate || opts.Operation == WriteOperationUpsert) && opts.IdempotencyKey == "" && opts.SObject != "FeedItem" {
		return newWriteError("IDEMPOTENCY_KEY_REQUIRED", http.StatusBadRequest, opts, "--idempotency-key is required for create/upsert", "")
	}
	if len(opts.Fields) == 0 {
		return fmt.Errorf("at least one --field NAME=VALUE is required")
	}
	return nil
}

func defaultPlanSummary(opts WriteOptions, diff map[string]map[string]any) string {
	keys := sortedPlanFieldKeys(diff)
	target := firstNonEmptyString(opts.RecordID, opts.IdempotencyKey, "(new record)")
	if len(keys) == 0 {
		return fmt.Sprintf("%s %s %s", opts.Operation, opts.SObject, target)
	}
	return fmt.Sprintf("%s %s %s fields: %s", opts.Operation, opts.SObject, target, joinStrings(keys, ", "))
}

func sortedPlanFieldKeys(m map[string]map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func jsonEqual(a, b any) bool {
	aJSON, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bJSON, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return bytes.Equal(aJSON, bJSON)
}

func joinStrings(values []string, sep string) string {
	if len(values) == 0 {
		return ""
	}
	out := values[0]
	for _, value := range values[1:] {
		out += sep + value
	}
	return out
}
