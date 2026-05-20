package cli

import (
	"encoding/json"
	"testing"
)

// PATCH: Generated dry-runs must expose the exact request query/body.
func TestGeneratedDeleteCommandsSendDeclaredParameters(t *testing.T) {
	cfgPath := writeBaseURLConfig(t, "https://example.test")
	t.Setenv("LUNCHMONEY_API_KEY", "test-token")

	stdout, stderr, err := executeForTest(t,
		"--config", cfgPath,
		"budgets", "delete",
		"--category-id", "74",
		"--start-date", "2026-05-01",
		"--json", "--dry-run",
	)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	env := decodeEnvelope(t, stdout)
	data := envData(t, env)
	query := envMap(t, data, "query")
	if query["category_id"] != "74" || query["start_date"] != "2026-05-01" {
		t.Fatalf("query = %#v", query)
	}
}

// PATCH: Generated POSTs with query flags must not discard those flags.
func TestGeneratedPlaidFetchSendsQueryParameters(t *testing.T) {
	cfgPath := writeBaseURLConfig(t, "https://example.test")
	t.Setenv("LUNCHMONEY_API_KEY", "test-token")

	stdout, stderr, err := executeForTest(t,
		"--config", cfgPath,
		"plaid-accounts", "trigger-fetch",
		"--id", "77",
		"--start-date", "2026-05-01",
		"--end-date", "2026-05-14",
		"--json", "--dry-run",
	)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	env := decodeEnvelope(t, stdout)
	data := envData(t, env)
	query := envMap(t, data, "query")
	if query["id"] != "77" || query["start_date"] != "2026-05-01" || query["end_date"] != "2026-05-14" {
		t.Fatalf("query = %#v", query)
	}
}

// PATCH: Explicit false/zero/empty/null values must survive generated body building.
func TestTransactionsUpdateIdPreservesExplicitValues(t *testing.T) {
	cfgPath := writeBaseURLConfig(t, "https://example.test")
	t.Setenv("LUNCHMONEY_API_KEY", "test-token")

	stdout, stderr, err := executeForTest(t,
		"--config", cfgPath,
		"transactions", "update-id", "123",
		"--notes", "",
		"--category-id", "0",
		"--tag-ids", "[]",
		"--custom-metadata", "null",
		"--is-pending=false",
		"--update-balance=false",
		"--json", "--dry-run",
	)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q", stderr)
	}
	env := decodeEnvelope(t, stdout)
	data := envData(t, env)
	query := envMap(t, data, "query")
	if query["update_balance"] != "false" {
		t.Fatalf("query = %#v, want update_balance=false", query)
	}
	body := envMap(t, data, "request_body")
	if body["notes"] != "" {
		t.Fatalf("notes = %#v, want empty string", body["notes"])
	}
	if body["category_id"] != float64(0) {
		t.Fatalf("category_id = %#v, want 0", body["category_id"])
	}
	if _, ok := body["custom_metadata"]; !ok || body["custom_metadata"] != nil {
		t.Fatalf("custom_metadata = %#v, want present null", body["custom_metadata"])
	}
	tags, ok := body["tag_ids"].([]any)
	if !ok || len(tags) != 0 {
		t.Fatalf("tag_ids = %#v, want empty array", body["tag_ids"])
	}
	if body["is_pending"] != false {
		t.Fatalf("is_pending = %#v, want explicit false", body["is_pending"])
	}
}

// PATCH: Category and tag updates need Changed-aware false/zero/null handling.
func TestCategoriesAndTagsUpdatePreserveExplicitValues(t *testing.T) {
	cfgPath := writeBaseURLConfig(t, "https://example.test")
	t.Setenv("LUNCHMONEY_API_KEY", "test-token")

	stdout, stderr, err := executeForTest(t,
		"--config", cfgPath,
		"categories", "update-category", "7",
		"--exclude-from-budget=false",
		"--order", "0",
		"--description", "",
		"--json", "--dry-run",
	)
	if err != nil {
		t.Fatalf("categories Execute: %v", err)
	}
	if stderr != "" {
		t.Fatalf("categories stderr = %q", stderr)
	}
	body := envMap(t, envData(t, decodeEnvelope(t, stdout)), "request_body")
	if body["exclude_from_budget"] != false || body["order"] != float64(0) || body["description"] != "" {
		t.Fatalf("category body = %#v", body)
	}

	stdout, stderr, err = executeForTest(t,
		"--config", cfgPath,
		"tags", "update", "9",
		"--archived=false",
		"--archived-at", "null",
		"--description", "",
		"--json", "--dry-run",
	)
	if err != nil {
		t.Fatalf("tags Execute: %v", err)
	}
	if stderr != "" {
		t.Fatalf("tags stderr = %q", stderr)
	}
	body = envMap(t, envData(t, decodeEnvelope(t, stdout)), "request_body")
	if body["archived"] != false || body["archived_at"] != nil || body["description"] != "" {
		t.Fatalf("tag body = %#v", body)
	}
}

func decodeEnvelope(t *testing.T, stdout string) map[string]any {
	t.Helper()
	var env map[string]any
	if err := json.Unmarshal([]byte(stdout), &env); err != nil {
		t.Fatalf("unmarshal envelope %q: %v", stdout, err)
	}
	return env
}

func envData(t *testing.T, env map[string]any) map[string]any {
	t.Helper()
	data, ok := env["data"].(map[string]any)
	if !ok {
		t.Fatalf("data = %#v, want object", env["data"])
	}
	return data
}

func envMap(t *testing.T, parent map[string]any, key string) map[string]any {
	t.Helper()
	got, ok := parent[key].(map[string]any)
	if !ok {
		t.Fatalf("%s = %#v, want object", key, parent[key])
	}
	return got
}
