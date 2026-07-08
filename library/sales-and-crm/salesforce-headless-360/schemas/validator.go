package schemas

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/salesforce-headless-360/internal/trust"
)

var hexSHA256 = regexp.MustCompile(`^[a-f0-9]{64}$`)

// ValidateBundleValue validates a bundle-like value by first JSON encoding it.
func ValidateBundleValue(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal bundle for schema validation: %w", err)
	}
	return ValidateBundle(data)
}

// ValidateBundle validates the emitted bundle against the PP-core envelope
// contract and the Salesforce-specific manifest contract. It intentionally
// avoids external JSON Schema dependencies; the checked subset covers required
// fields, primitive types, enum values, and file attestation shape.
func ValidateBundle(data []byte) error {
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return schemaErr("parse bundle JSON: %w", err)
	}

	envelope, ok := object(root["envelope"])
	if !ok {
		return schemaErr("envelope must be an object")
	}
	manifest, ok := object(root["manifest"])
	if !ok {
		return schemaErr("manifest must be an object")
	}
	signature, _ := stringValue(root["signature"])
	if signature == "" {
		return schemaErr("signature must be a non-empty compact JWS")
	}
	kid, err := trust.ExtractKIDUnsafe(signature)
	if err != nil {
		return schemaErr("signature.kid missing or invalid: %w", err)
	}

	if stringRequired(envelope, "org_id") == "" {
		return schemaErr("provenance.org_id is required")
	}
	if stringRequired(envelope, "user_id") == "" {
		return schemaErr("provenance.user_id is required")
	}
	if _, err := time.Parse(time.RFC3339, stringRequired(envelope, "generated_at")); err != nil {
		return schemaErr("provenance.generated_at must be RFC3339: %w", err)
	}
	if _, ok := array(envelope["sources_used"]); !ok {
		return schemaErr("provenance.sources_used must be an array")
	}
	if kid == "" {
		return schemaErr("signature.kid is required")
	}
	if err := validateManifest(manifest); err != nil {
		return err
	}
	return nil
}

// ValidateWritePlan validates a v1 write-plan document using the same
// dependency-free subset of JSON Schema checks as bundle validation.
func ValidateWritePlan(data []byte) error {
	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return schemaErr("parse write plan JSON: %w", err)
	}
	if err := rejectUnknownKeys(root, "write_plan", []string{"version", "plan_intent", "plan_metadata", "plan_jws", "countersignatures"}); err != nil {
		return err
	}
	version, ok := number(root["version"])
	if !ok || version != 1 {
		return schemaErr("write_plan.version must be number 1")
	}
	intent, ok := object(root["plan_intent"])
	if !ok {
		return schemaErr("write_plan.plan_intent must be an object")
	}
	metadata, ok := object(root["plan_metadata"])
	if !ok {
		return schemaErr("write_plan.plan_metadata must be an object")
	}
	if stringRequired(root, "plan_jws") == "" {
		return schemaErr("write_plan.plan_jws is required")
	}
	if err := validateWritePlanIntent(intent); err != nil {
		return err
	}
	if err := validateWritePlanMetadata(metadata); err != nil {
		return err
	}
	countersigs, ok := array(root["countersignatures"])
	if !ok {
		return schemaErr("write_plan.countersignatures must be an array")
	}
	for i, item := range countersigs {
		sig, ok := object(item)
		if !ok {
			return schemaErr("write_plan.countersignatures[%d] must be an object", i)
		}
		if err := rejectUnknownKeys(sig, fmt.Sprintf("write_plan.countersignatures[%d]", i), []string{"kid", "signed_at", "jws"}); err != nil {
			return err
		}
		if stringRequired(sig, "kid") == "" {
			return schemaErr("write_plan.countersignatures[%d].kid is required", i)
		}
		if _, err := time.Parse(time.RFC3339, stringRequired(sig, "signed_at")); err != nil {
			return schemaErr("write_plan.countersignatures[%d].signed_at must be RFC3339: %w", i, err)
		}
		if stringRequired(sig, "jws") == "" {
			return schemaErr("write_plan.countersignatures[%d].jws is required", i)
		}
	}
	return nil
}

// ManifestSHA returns the canonical JSON sha256 used by the signing claims.
func ManifestSHA(manifest any) (string, error) {
	data, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func validateManifest(manifest map[string]any) error {
	if files, ok := array(manifest["files"]); ok {
		for i, item := range files {
			file, ok := object(item)
			if !ok {
				return schemaErr("manifest.files[%d] must be an object", i)
			}
			if stringRequired(file, "name") == "" {
				return schemaErr("manifest.files[%d].name is required", i)
			}
			sha := stringRequired(file, "sha256")
			if !hexSHA256.MatchString(sha) {
				return schemaErr("manifest.files[%d].sha256 must be a lowercase sha256 hex string", i)
			}
			if stringRequired(file, "sf_content_version_id") == "" {
				return schemaErr("manifest.files[%d].sf_content_version_id is required", i)
			}
			size, ok := number(file["size_bytes"])
			if !ok || size < 0 || size != float64(int64(size)) {
				return schemaErr("manifest.files[%d].size_bytes must be a non-negative integer", i)
			}
		}
	}
	return nil
}

func validateWritePlanIntent(intent map[string]any) error {
	if err := rejectUnknownKeys(intent, "write_plan.plan_intent", []string{"iss", "sub", "aud", "iat", "exp", "jti", "sobject", "record_id", "operation", "diff_sha256", "idempotency_key", "if_last_modified"}); err != nil {
		return err
	}
	for _, key := range []string{"iss", "sub", "aud", "jti", "sobject", "operation"} {
		if stringRequired(intent, key) == "" {
			return schemaErr("write_plan.plan_intent.%s is required", key)
		}
	}
	for _, key := range []string{"iat", "exp"} {
		value, ok := number(intent[key])
		if !ok || value != float64(int64(value)) {
			return schemaErr("write_plan.plan_intent.%s must be an integer", key)
		}
	}
	if diffSHA := stringRequired(intent, "diff_sha256"); diffSHA != "" && !hexSHA256.MatchString(diffSHA) {
		return schemaErr("write_plan.plan_intent.diff_sha256 must be a lowercase sha256 hex string")
	}
	return nil
}

func validateWritePlanMetadata(metadata map[string]any) error {
	if err := rejectUnknownKeys(metadata, "write_plan.plan_metadata", []string{"created_by_kid", "created_at", "fields", "auth_mode", "execute_path", "human_summary", "dry_run"}); err != nil {
		return err
	}
	for _, key := range []string{"created_by_kid", "created_at", "auth_mode", "execute_path", "human_summary"} {
		if stringRequired(metadata, key) == "" {
			return schemaErr("write_plan.plan_metadata.%s is required", key)
		}
	}
	if _, err := time.Parse(time.RFC3339, stringRequired(metadata, "created_at")); err != nil {
		return schemaErr("write_plan.plan_metadata.created_at must be RFC3339: %w", err)
	}
	if fields, ok := object(metadata["fields"]); metadata["fields"] != nil && !ok {
		return schemaErr("write_plan.plan_metadata.fields must be an object")
	} else if ok {
		for key, value := range fields {
			field, ok := object(value)
			if !ok {
				return schemaErr("write_plan.plan_metadata.fields.%s must be an object", key)
			}
			if err := rejectUnknownKeys(field, "write_plan.plan_metadata.fields."+key, []string{"before", "after"}); err != nil {
				return err
			}
			if _, ok := field["after"]; !ok {
				return schemaErr("write_plan.plan_metadata.fields.%s.after is required", key)
			}
		}
	}
	if _, ok := metadata["dry_run"].(bool); metadata["dry_run"] != nil && !ok {
		return schemaErr("write_plan.plan_metadata.dry_run must be a boolean")
	}
	return nil
}

func schemaErr(format string, args ...any) error {
	return fmt.Errorf("SCHEMA_INVALID: "+format, args...)
}

func object(v any) (map[string]any, bool) {
	m, ok := v.(map[string]any)
	return m, ok
}

func array(v any) ([]any, bool) {
	if v == nil {
		return nil, true
	}
	a, ok := v.([]any)
	return a, ok
}

func stringValue(v any) (string, bool) {
	s, ok := v.(string)
	return s, ok
}

func stringRequired(m map[string]any, key string) string {
	s, _ := stringValue(m[key])
	return strings.TrimSpace(s)
}

func number(v any) (float64, bool) {
	n, ok := v.(float64)
	return n, ok
}

func rejectUnknownKeys(m map[string]any, path string, allowed []string) error {
	allow := map[string]bool{}
	for _, key := range allowed {
		allow[key] = true
	}
	for key := range m {
		if !allow[key] {
			return schemaErr("%s.%s is not allowed", path, key)
		}
	}
	return nil
}
