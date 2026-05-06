# PR #639 Regression Findings — dominos reprint run 20260505-175548

> Captured during Phase 2 of the dominos reprint while exercising PR #639's typed AuthEnvVar model. The dominos spec declares `auth.type: bearer_token` with three `env_var_specs`: `DOMINOS_USERNAME` (kind=auth_flow_input), `DOMINOS_PASSWORD` (kind=auth_flow_input, sensitive), and `DOMINOS_TOKEN` (kind=harvested, sensitive). This shape — bearer-via-login-flow — is exactly the case PR #639 added the typed kinds to handle.

## Bug A (CRITICAL): harvested env vars leak into OpenClaw `envVars` block

**Severity:** High — exposes a never-user-set credential to ClawHub's install/config UI.

**Where:** Generated `SKILL.md` frontmatter, `metadata.openclaw.envVars` list.

**Evidence:**
```yaml
envVars:
  - name: DOMINOS_USERNAME       # kind=auth_flow_input — OK
    required: false
    description: "Only needed during `auth login`; not required for normal use. ..."
  - name: DOMINOS_PASSWORD       # kind=auth_flow_input — OK
    required: false
    description: "Set during application setup."
  - name: DOMINOS_TOKEN          # kind=harvested — LEAK
    required: false
    description: "Populated automatically by `auth login`."
```

The `harvested` var is supposed to be populated by the auth login flow, not provided by the user. Surfacing it in OpenClaw `envVars` (which ClawHub reads to ask the user for credentials at install) is wrong — the user has no way to obtain this value, and it shouldn't be on the user-facing config surface at all.

**Template source:** `internal/generator/templates/skill.md.tmpl`, lines 17–31. The range loop iterates over all `Auth.EnvVarSpecs` and the harvested branch (lines 29–30) only customizes the description; it doesn't filter the entry out.

**Machine fix (one-line):** Skip harvested entries in the range:
```
{{- range .Auth.EnvVarSpecs}}
{{- if eq .Kind "harvested"}}{{continue}}{{end}}
      - name: {{.Name}}
...
```

Or wrap the existing branch logic so harvested produces no output. Same effect.

**Verification of clean surfaces:**
- Top-level `manifest.json` (MCPB) — no `user_config` block; no leak. ✓
- MCPB inner manifest — no `user_config`; no leak. ✓
- README "## Authentication" — does not ask user to set DOMINOS_TOKEN. ✓
- `doctor.go` env-var loop — surfaces all three with INFO-level "populated by auth login" messages, which is the correct (state-reporting) behavior. ✓
- `config.go` — loads all three (implementation-internal). ✓

**Status:** Will fix the template in this run, regenerate dominos, and confirm the leak is gone.

## Bug B: doctor INFO references `auth login --chrome` for non-cookie auth

**Severity:** Medium — misleading hint pointing to a flag/flow that doesn't exist.

**Where:** Generated `internal/cli/doctor.go`, line 114:
```go
authEnvInfo = append(authEnvInfo, "DOMINOS_TOKEN populated automatically by auth login --chrome")
```

`--chrome` is a flag of the cookie/composed auth_browser template, not the simple/bearer auth path. dominos uses `bearer_token` and the simple template emits `auth set-token`, not `auth login --chrome`. The doctor's hint is template-leaked text from the cookie auth narrative.

**Template source:** `internal/generator/templates/doctor.go.tmpl`. The auth-info branch for harvested vars likely hardcodes `auth login --chrome` instead of detecting which auth flow template was used.

**Status:** Filing for retro. Not fixing in this run (cosmetic). The harvested-var INFO entry is correct in spirit; only the `--chrome` suffix is wrong.

## Bug C: auth parent command `Short` references one canonical env var by name

**Severity:** Low — cosmetic.

**Where:** Generated `internal/cli/auth.go`:
```go
cmd := &cobra.Command{
    Use:   "auth",
    Short: "Manage DOMINOS_USERNAME credentials",  // ← references USERNAME only
}
```

When env_var_specs has multiple entries, the parent Short should be a generic phrase ("Manage Domino's credentials"), not the canonical first entry's name. Picking USERNAME (the first auth_flow_input) is especially confusing — USERNAME is a credential input, not a credential.

**Template source:** `internal/generator/templates/auth_simple.go.tmpl` (and probably the other auth templates) — likely uses `Auth.CanonicalEnvVar().Name` in the Short. Should fall back to the API display_name.

**Status:** Filing for retro. Not fixing in this run.

## Bug D (FEATURE GAP): no template for `bearer_token + auth_flow_input + harvested` shape

**Severity:** Major UX gap — the typed model is purely declarative without a flow template to drive it.

**Where:** `internal/generator/generator.go:1658-1669` template dispatch:
```go
authTmpl := "auth_simple.go.tmpl"           // default — emits set-token (per_call)
authTmpl = "auth_client_credentials.go.tmpl" // OAuth2 client_credentials
authTmpl = "auth.go.tmpl"                    // oauth2 (full login)
authTmpl = "auth_browser.go.tmpl"            // cookie / composed
```

The dispatch keys off `Auth.Type`. For `bearer_token`, the default `auth_simple.go.tmpl` is selected — and that template emits only `auth set-token` (the per_call mode where the user pastes a pre-obtained token). It does NOT consume `auth_flow_input` env vars or write to `harvested` config fields, which is what PR #639 added the kinds for.

**Effect on dominos:** The shipped CLI has `auth set-token <token>` — but DOMINOS_TOKEN is harvested, so the user has no token to paste. The path that would actually work for dominos (read USERNAME/PASSWORD, POST to authproxy.dominos.com/auth-proxy-service/login, harvest the access_token to config) doesn't exist. The user falls back to obtaining the token via curl or browser, then `auth set-token` — but that's exactly the manual flow PR #639 was supposed to eliminate.

**Status:** Filing for retro. The fix is a new template — `auth_flow_input.go.tmpl` or similar — that detects the auth_flow_input + harvested pattern in `EnvVarSpecs` and emits an `auth login` command that:
1. Reads auth_flow_input vars (env or interactive prompt with `--no-input` opt-out)
2. POSTs to `Auth.TokenURL` with the credentials
3. Writes the response token to the harvested var's config field

For this run: the dominos CLI ships with the auth gap documented in README. Future reprint will pick up the new template once it lands.

## Bug E: `.printing-press.json` captures `null` for auth/env_vars

**Severity:** Low — manifest-only, not user-facing in any blocking way.

**Evidence:**
```bash
$ jq '. | {auth, env_vars, env_var_specs}' .printing-press.json
{
  "auth": null,
  "env_vars": null,
  "env_var_specs": null
}
```

The runtime manifest writer does not appear to capture the typed auth shape. Other surfaces (skill.md, config.go, doctor.go) DO read the rich shape correctly. This is likely an internal/pipeline/runtime.go gap.

**Status:** Filing for retro. Not blocking.

## Summary for retro

PR #639 widens the auth env-var model in the spec parser and most templates pick up the new shape, but two regression-grade issues exist:

| # | Bug | Severity | Fix scope |
|---|---|---|---|
| A | harvested vars leak into SKILL.md OpenClaw envVars | High | 1-line template guard |
| B | doctor INFO hardcodes `auth login --chrome` for non-cookie auth | Medium | template branch refinement |
| C | auth parent Short uses singular canonical env var name | Low | cosmetic template polish |
| D | no template for bearer + auth_flow_input + harvested flow | Major (feature gap) | new template |
| E | manifest writer drops typed auth fields | Low | pipeline/runtime.go fix |

Bug A is fix-now (1-line guard, regenerate, verify). Bugs B–E ride to retro.
