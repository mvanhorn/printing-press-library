# PR #639 Fix Handoff

> Paste this into a fresh agent working on `feat/auth-envvar-model` (branch head `4abd5e6d`). It has 5 bugs found during a dominos reprint stress-test of the typed AuthEnvVar model. Two of them have local template fixes that need to be folded into the PR; three need new code. Repo: `mvanhorn/cli-printing-press`. Worktree: `/Users/tmchow/Code/cli-printing-press/.claude/worktrees/noble-plotting-cupcake`.

## Context

PR #639 widens the auth env-var model from `[]string` to a typed `AuthEnvVar` struct with `Kind` (per_call / auth_flow_input / harvested), `Required`, `Sensitive`, `Description`, `Inferred`. The new struct flows through every downstream surface (templates, manifests, scorer). When stress-tested against a dominos spec that exercises all three kinds — `DOMINOS_USERNAME` (auth_flow_input), `DOMINOS_PASSWORD` (auth_flow_input, sensitive), `DOMINOS_TOKEN` (harvested, sensitive) — five regressions surfaced.

The bug definitions of record are in `library/dominos/proofs/2026-05-05-175548-fix-dominos-pp-cli-pr639-findings.md` (Phase 4 retro proofs from the dominos reprint run).

## Bug A — Harvested env vars leak into SKILL.md OpenClaw `envVars` (FIX APPLIED LOCALLY)

**Severity:** High. Exposes a never-user-set credential to ClawHub's install/config UI as if the user should provide it.

**Where:** `internal/generator/templates/skill.md.tmpl`, lines 17–34. The range over `Auth.EnvVarSpecs` had no kind filter, so `harvested` entries were rendered alongside `per_call` and `auth_flow_input`.

**Local fix already in working tree** (test against this exact patch):

```gotemplate
{{- if and (ne .Auth.Type "none") (or .Auth.EnvVarSpecs .Auth.EnvVars)}}
    envVars:
{{- if .Auth.EnvVarSpecs}}
{{- range .Auth.EnvVarSpecs}}
{{- /* Harvested env vars are populated by `auth login`, not user-supplied. They must not appear in user-facing config surfaces (OpenClaw envVars, ClawHub install UI). */}}
{{- if ne (printf "%s" .Kind) "harvested"}}
      - name: {{.Name}}
        required: {{.Required}}
{{- if and (eq .Kind "per_call") .Required}}
        description: "..."
{{- else if and (eq .Kind "per_call") (not .Required)}}
        description: "..."
{{- else if and (eq .Kind "auth_flow_input") (not .Sensitive)}}
        description: "..."
{{- else if and (eq .Kind "auth_flow_input") .Sensitive}}
        description: "Set during application setup."
{{- else}}
        description: "..."
{{- end}}
{{- end}}
{{- end}}
{{- else}}
{{- range .Auth.EnvVars}}
      - name: {{.}}
        required: true
        description: "{{.}} credential."
{{- end}}
{{- end}}
{{- end}}
```

The change is the `{{- if ne (printf "%s" .Kind) "harvested"}}` guard around the body. The old `harvested` description branch is removed (unreachable now). Verify by generating any spec with a `harvested` env_var_spec and grepping the resulting `SKILL.md` — the harvested var name should NOT appear under `envVars`. The legacy `env: [...]` line on `skill.md.tmpl:11` already filters to `(eq .Kind "per_call") .Required` and is correct.

**Test:** add a regression test under `internal/generator/templates_test.go` that asserts harvested entries don't render. Use the existing dominos spec layout (3 env_var_specs, one of each kind).

## Bug A-extended — Same harvested leak in MCP and agent-context templates (FIXES APPLIED LOCALLY)

The same range-without-kind-filter exists in two more templates. Both got the same `{{- if ne (printf "%s" .Kind) "harvested"}}` guard:

1. **`internal/generator/templates/agent_context.go.tmpl`** lines 105–130. The `envVars := []agentContextAuthEnvVar{...}` initializer iterates `Auth.EnvVarSpecs` and was including harvested entries, which surfaced as a "configurable env var" in `dominos-pp-cli agent-context auth --json` output that agents read. With the fix, agent-context still reports auth state correctly (the `authMode` and the present-token check via `cfg.AuthHeader()` are independent of this list), it just doesn't tell agents to set the harvested var.

2. **`internal/generator/templates/mcp_tools.go.tmpl`** has *two* range loops over `Auth.EnvVarSpecs`:
   - Lines 422–451 (top-level `auth.env_vars` block in the agent-context tool definition)
   - Lines 462–477 (per-tier `tier_routing.tiers.<name>.env_vars` block when tier routing is on)
   Both got the same harvested guard. The MCP server's tool discovery now correctly omits harvested vars from the "what env vars do I need" advertisement that hosts like Claude Desktop read.

**Verification command** (works without auth):

```bash
# Generate any spec with a harvested env_var_spec, then:
grep -n harvested $OUTPUT_DIR/SKILL.md $OUTPUT_DIR/internal/cli/agent_context.go $OUTPUT_DIR/internal/mcp/tools.go
# Expect: zero matches in the user-facing render. The token NAME still appears in
# config.go (AccessToken loading), helpers.go (AuthHeader build), and doctor.go INFO
# output — those are correct introspection surfaces.
```

## Bug B — `doctor.go` INFO hardcodes `auth login --chrome` for non-cookie auth (NEEDS FIX)

**Severity:** Medium. Misleading hint pointing at a flag/flow that doesn't exist for the chosen auth shape.

**Where:** `internal/generator/templates/doctor.go.tmpl`. The auth-info INFO line for harvested env vars looks like:

```go
authEnvInfo = append(authEnvInfo, "DOMINOS_TOKEN populated automatically by auth login --chrome")
```

The `--chrome` suffix only makes sense when the auth template selected was `auth_browser.go.tmpl` (cookie/composed auth). For `bearer_token` with `auth_flow_input` env vars, the simple template was selected and emits `auth set-token`, not `auth login --chrome`. The doctor should detect the auth template that will be emitted and produce the appropriate hint.

**Suggested fix:** The auth template dispatch in `internal/generator/generator.go:1658-1669` already classifies the auth shape into one of `auth_simple`, `auth_client_credentials`, `auth.go` (oauth2), `auth_browser`. Compute the same classification once, store it on the template data, and have `doctor.go.tmpl` switch on that classification:

| Auth template | Hint suffix |
|---|---|
| `auth_simple.go.tmpl` | `set this with: <cli> auth set-token <token>` |
| `auth.go.tmpl` (oauth2) | `populated by: <cli> auth login` |
| `auth_browser.go.tmpl` | `populated by: <cli> auth login --chrome` |
| `auth_client_credentials.go.tmpl` | `populated by: <cli> auth login --client-id ... --client-secret ...` |

Until Bug D's `auth_flow_input` template lands, harvested-via-auth_simple is impossible, so the simple-template hint can be a dead branch (or, better, fall back to the same per-kind narrative the SKILL uses).

## Bug C — Auth parent command `Short` references the canonical env var by name (LOW PRIORITY)

**Severity:** Low. Cosmetic.

**Where:** Generated `internal/cli/auth.go`:

```go
cmd := &cobra.Command{
    Use:   "auth",
    Short: "Manage DOMINOS_USERNAME credentials",   // ← USERNAME-only
}
```

The Short is being built from `Auth.CanonicalEnvVar().Name`, which for the dominos shape returns `DOMINOS_USERNAME` (the first auth_flow_input). USERNAME isn't a credential — it's a credential input. The Short should say something like `Manage <DisplayName> credentials` (using the spec's `display_name`) or `Manage authentication for <DisplayName>`.

**Suggested fix:** Change the Short rendering in `auth_simple.go.tmpl`, `auth.go.tmpl`, `auth_client_credentials.go.tmpl`, and `auth_browser.go.tmpl` from `"Manage {{envName .Name}} credentials"` (or wherever the canonical-name string lives) to `"Manage {{.ProseName}} credentials"` or `"Manage authentication for {{.ProseName}}"`. The display name flows through `.ProseName`/`.DisplayName`/`.Name`-derived prose helpers depending on the spec — pick the one that's already plumbed.

## Bug D — No template emits `auth login` for `bearer_token + auth_flow_input + harvested` (BIGGEST GAP, FILE AS FEATURE REQUEST)

**Severity:** Major UX gap. PR #639 added the `auth_flow_input` and `harvested` kinds purely declaratively — no template uses them. The dispatcher routes any spec that declares this shape to `auth_simple.go.tmpl`, which only emits `auth set-token`. The result: a CLI claims via SKILL/manifest that USERNAME/PASSWORD are auth_flow_input vars, but the binary has no command that consumes them.

**Where:** `internal/generator/generator.go:1658-1669`:

```go
authTmpl := "auth_simple.go.tmpl"           // default — emits set-token (per_call)
authTmpl = "auth_client_credentials.go.tmpl" // OAuth2 client_credentials
authTmpl = "auth.go.tmpl"                    // oauth2 (full login)
authTmpl = "auth_browser.go.tmpl"            // cookie / composed
```

The dispatcher keys off `Auth.Type`. For a spec with `auth.type: bearer_token` AND `EnvVarSpecs` containing `auth_flow_input` entries, `auth_simple.go.tmpl` is wrong: it's the per_call path. We need a new template — call it `auth_flow_input.go.tmpl` or `auth_password_grant.go.tmpl` — that:

1. Emits an `auth login` subcommand that takes `--username` / `--password` (or reads them from the auth_flow_input env vars).
2. POSTs to `Auth.TokenURL` (an existing field on `AuthConfig`) with the credentials.
3. Parses the standard OAuth password-grant response (`{access_token, refresh_token, expires_in, token_type}`).
4. Writes the harvested `access_token` to the harvested env var's config field via `cfg.SaveTokens(...)`.

**Suggested dispatcher rule:**

```go
authTmpl := "auth_simple.go.tmpl"
hasAuthFlowInput := false
hasHarvested := false
for _, ev := range g.Spec.Auth.EnvVarSpecs {
    if ev.Kind == spec.AuthEnvVarKindAuthFlowInput { hasAuthFlowInput = true }
    if ev.Kind == spec.AuthEnvVarKindHarvested { hasHarvested = true }
}
if g.Spec.Auth.Type == "bearer_token" && hasAuthFlowInput && hasHarvested && g.Spec.Auth.TokenURL != "" {
    authTmpl = "auth_flow_input.go.tmpl"
}
```

A reference implementation of the body lives at `library/dominos/internal/cli/auth_login.go` (the dominos reprint hand-coded it because the template was missing). Use that as the starting point — it has the form-encoded POST, `client_id` plumbing, response parsing, and `cfg.SaveTokens(...)` harvest pattern. Templatize the API-specific bits: client_id default (could come from a new `auth.client_id` spec field), the `client_id` env var name (`<API>_CLIENT_ID` falls out of the existing env-name macro), URL (already in `Auth.TokenURL`).

**Caveat for the agent picking this up:** Many bearer-via-password-grant APIs (including dominos itself, ironically) gate the actual password endpoint behind WAF / captcha. The template will produce a CORRECT `auth login` for friendly APIs (most enterprise SaaS) but won't bypass Akamai-style protection on consumer-facing services. That's a transport problem, not an auth-template problem. Document this in the template's docstring.

## Bug F — Generator emits `truncateJSONArray()` call without emitting the helper (UNRELATED to PR #639, surfaced during stress-test)

**Severity:** High — generated code fails `go vet` and `go build` for any spec where a non-paginated endpoint declares a `limit` query param.

**Where:** Generator's endpoint-command template (`internal/generator/templates/endpoint.go.tmpl` or wherever the GET-with-limit branch lives) emits this code:

```go
// The API doesn't declare a paginator but accepts a limit query param.
// Truncate client-side so --limit N is honored regardless.
data = truncateJSONArray(data, flagLimit)
```

But `truncateJSONArray` is never emitted into `internal/cli/helpers.go`. Result: every CLI generated from a spec with a non-paginated `limit`-bearing endpoint fails to compile.

**Repro:** Add an endpoint with a `limit` integer param but no `pagination` block:

```yaml
endpoints:
  list_orders:
    method: GET
    path: /api/customer/{id}/order
    params:
      - name: limit
        type: integer
        default: 5
    response: { type: array, item: Order }
    pagination: null
```

Generate. The resulting `customer_list_orders.go:51` references `truncateJSONArray` which doesn't exist. `go build` fails:

```
internal/cli/customer_list_orders.go:51:11: undefined: truncateJSONArray
```

**Reference fix** (apply to the helpers.go.tmpl):

```go
// truncateJSONArray honors --limit N for endpoints whose API silently ignores
// a limit query param. Returns the original bytes unchanged if data is not a
// JSON array, limit is non-positive, or the array already has <= limit items.
func truncateJSONArray(data []byte, limit int) []byte {
	if limit <= 0 {
		return data
	}
	var items []json.RawMessage
	if err := json.Unmarshal(data, &items); err != nil {
		return data
	}
	if len(items) <= limit {
		return data
	}
	out, err := json.Marshal(items[:limit])
	if err != nil {
		return data
	}
	return out
}
```

This is unrelated to PR #639's auth-envvar widening but surfaced while stress-testing the dominos spec's customer.list_orders endpoint. Worth folding into a separate fix PR (or this one if scope expansion is OK).

## Bug E — RETRACTED (false positive)

Original claim was that `.printing-press.json` had null `auth`, `env_vars`, `env_var_specs`. Wrong. The actual schema (defined at `internal/pipeline/climanifest.go:66-68`) uses **flat** field names — `auth_type`, `auth_env_vars`, `auth_env_var_specs` — not a nested `auth` block or top-level `env_vars`. Those flat fields ARE populated correctly. The original `jq '. | {auth, env_vars, env_var_specs}'` query was looking for non-existent keys and returned null for all three; that was misread as "the writer is dropping the fields."

Confirmation from a dominos run on PR #639 head:

```bash
$ jq '{auth_type, auth_env_vars, auth_env_var_specs}' .printing-press.json
{
  "auth_type": "bearer_token",
  "auth_env_vars": ["DOMINOS_USERNAME", "DOMINOS_PASSWORD", "DOMINOS_TOKEN"],
  "auth_env_var_specs": [
    {"name": "DOMINOS_USERNAME", "kind": "auth_flow_input", "required": false, "sensitive": false, "description": "..."},
    {"name": "DOMINOS_PASSWORD", "kind": "auth_flow_input", "required": false, "sensitive": true,  "description": "..."},
    {"name": "DOMINOS_TOKEN",    "kind": "harvested",       "required": false, "sensitive": true,  "description": "..."}
  ]
}
```

Manifest writer is fine. No fix needed for Bug E.

## Test for all five fixes

There's a stress-test spec already on disk at `library/dominos/spec.yaml` that exercises every PR #639 surface (bearer_token + 2 auth_flow_input + 1 harvested + 19 typed endpoints + 1 mcp.transport=http hint). Generate against it after each fix:

```bash
cd /tmp && rm -rf dominos-test && mkdir dominos-test
$WORKTREE/printing-press generate \
  --spec ~/printing-press/library/dominos/spec.yaml \
  --output /tmp/dominos-test \
  --research-dir ~/printing-press/manuscripts/dominos/20260505-175548 \
  --traffic-analysis ~/printing-press/manuscripts/dominos/20260505-175548/discovery/traffic-analysis.json \
  --force --lenient --validate

# Then check:
grep -n harvested /tmp/dominos-test/SKILL.md /tmp/dominos-test/internal/cli/agent_context.go /tmp/dominos-test/internal/mcp/tools.go
# Expect: zero hits in user-facing surfaces.

grep -n "auth login --chrome" /tmp/dominos-test/internal/cli/doctor.go
# Expect: zero hits (we're using auth_simple route, not auth_browser).

grep "Short:" /tmp/dominos-test/internal/cli/auth.go | head -1
# Expect: "Manage Domino's credentials" (or similar prose name), NOT "Manage DOMINOS_USERNAME credentials".

ls /tmp/dominos-test/internal/cli/auth_login.go 2>/dev/null
# After Bug D template lands, this should exist with the password-grant flow.

jq '{auth, env_vars}' /tmp/dominos-test/.printing-press.json
# After Bug E: auth and env_vars are populated, not null.
```

## What's already in the working tree (cherry-pick these into the PR branch)

The two template patches for Bug A and Bug A-extended are sitting in the worktree at `internal/generator/templates/`:

```
internal/generator/templates/skill.md.tmpl       (envVars block harvested-skip)
internal/generator/templates/agent_context.go.tmpl (envVars init harvested-skip)
internal/generator/templates/mcp_tools.go.tmpl   (two range loops harvested-skip)
```

`git diff origin/main..HEAD -- internal/generator/templates/` from the worktree should show the relevant hunks. Cherry-pick or patch in.
