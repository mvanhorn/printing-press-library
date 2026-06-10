# Plane CLI — Workspace Targeting & Onboarding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make workspace targeting a first-class, separately-configured axis (`--workspace` flag / `PLANE_SLUG` env / `default_workspace` config) with an explicit, probe-validated enrollment step, so the published `plane-pp-cli` reaches any of a user's workspaces out-of-the-box instead of silently returning the wrong one.

**Architecture:** The generated client already resolves `{slug}` in the templated base URL `https://<host>/api/v1/workspaces/{slug}` from `Config.TemplateVars["slug"]` (seeded from `PLANE_SLUG`). We add (A) a persistent `--workspace/-w` flag that overrides that var at top precedence, a persisted `default_workspace` + `[[workspaces]]` registry, and (B) hand-written novel commands (`init`, `workspaces add|use|list|current`) plus a `doctor` nudge. All edits to generated files are small, `PATCH(...)`-tagged, and indexed so they survive regen; everything substantial lives in a new novel file.

**Tech Stack:** Go 1.26, Cobra/pflag, go-toml/v2, the project's existing client/config/store packages.

---

## Key facts established during research (do not re-derive)

- `internal/config/config.go` — `Config.TemplateVars["slug"]` is the workspace axis. `Load()` sets it from `PLANE_SLUG` env, else `"my-workspace"`. `BaseURL` default `https://api.plane.so/api/v1/workspaces/{slug}`. `save()` is private; `SaveCredential(token)` persists the API key. Generated file (`DO NOT EDIT` header) — edits must be `PATCH`-tagged.
- `internal/client/url.go` — `buildURL` substitutes `{slug}`; `templateVarEnvNames["slug"]="PLANE_SLUG"`.
- `internal/cli/root.go` — `rootFlags` struct; `newClient()` (line ~278) does `config.Load` then `client.New`; novel commands registered with `// PATCH(novel):` comments; persistent flags block at lines ~158-180.
- `internal/cli/relations.go:34` — `const envWorkspaceSlug = "PLANE_SLUG"`. `relations.go:274 resolveSlug(flagVal)` = `flagVal` or `os.Getenv(envWorkspaceSlug)`.
- `internal/cli/modules_novel.go:48 applyClientSlug(c, slug)` writes `c.Config.TemplateVars["slug"]`. Novel-command pattern: `newXCmd(flags *rootFlags) *cobra.Command`, uses `flags.newClient()`, `flags.printJSON`, `classifyAPIError`, `isTerminal`.
- `internal/cli/auth.go` — `set-token` calls `cfg.SaveCredential(args[0])`; `setup` prints steps. `init` reuses these.
- `internal/cli/doctor.go:67 newDoctorCmd` — builds a `report map[string]any`; human render walks `checkKeys`. Nudge inserts a `report["workspace"]` entry + a checkKey row.
- The workspace API cannot be enumerated by API key (verified live): `/api/v1/workspaces/`→404; `/api/users/me/workspaces/`→401 under X-API-Key and Bearer. Slug is user-supplied. A project payload carries `.workspace` (UUID) so slug→id is possible once a slug is known.
- Env name stays **`PLANE_SLUG`** (already wired end-to-end). The user-facing flag is `--workspace`; do NOT introduce a second env var.
- Working tree for code + TDD commits: the library publish clone `library/project-management/plane` (git, branch off `feat/plane`). The installed binary is built from this tree after implementation; dogfood reconciliation is a final task.

---

## File Structure

- **Modify** `internal/config/config.go` — add `DefaultWorkspace` + `Workspaces []WorkspaceEntry` fields, slug precedence in `Load()`, public `Save()`. (`PATCH(workspace-registry)`)
- **Modify** `internal/cli/root.go` — add `workspace` to `rootFlags`, the `--workspace/-w` persistent flag, the override in `newClient()`, and two `AddCommand` registrations. (`PATCH(workspace-flag)` / `PATCH(novel)`)
- **Create** `internal/cli/workspaces_novel.go` — `init` + `workspaces add|use|list|current`, probe helper, config mutators, env-shadow warnings.
- **Create** `internal/cli/workspaces_novel_test.go` — config precedence + registry round-trip + resolver tests.
- **Modify** `internal/cli/doctor.go` — workspace-configured / base-url-migration nudge. (`PATCH(workspace-nudge)`)
- **Modify** (recipe repo `cli-printing-press`) `catalog/plane.yaml` — flip `auth_instructions`/`notes` from "bake the full `…/workspaces/<slug>` prefix" to "templated base + `--workspace`/`PLANE_SLUG`/`workspaces use`".
- **Update** `.printing-press-patches.json` (publish clone root) — index the new patches.

---

### Task 1: Config — workspace registry + slug precedence + public Save

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_workspace_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/config/config_workspace_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func writeTempConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadSlugPrecedence(t *testing.T) {
	t.Setenv("PLANE_SLUG", "")
	t.Setenv("PLANE_BASE_URL", "")
	p := writeTempConfig(t, "base_url = \"https://api.plane.so/api/v1/workspaces/{slug}\"\ndefault_workspace = \"acme\"\n")

	// default_workspace wins when PLANE_SLUG unset
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.TemplateVars["slug"]; got != "acme" {
		t.Fatalf("slug from default_workspace: got %q want acme", got)
	}

	// PLANE_SLUG env overrides default_workspace
	t.Setenv("PLANE_SLUG", "bravo")
	cfg, err = Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.TemplateVars["slug"]; got != "bravo" {
		t.Fatalf("slug from env: got %q want bravo", got)
	}
}

func TestSaveWorkspaceRegistryRoundTrip(t *testing.T) {
	t.Setenv("PLANE_SLUG", "")
	p := writeTempConfig(t, "base_url = \"https://api.plane.so/api/v1/workspaces/{slug}\"\n")
	cfg, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	cfg.DefaultWorkspace = "acme"
	cfg.Workspaces = []WorkspaceEntry{{Slug: "acme", ID: "uuid-1"}}
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.DefaultWorkspace != "acme" || len(reloaded.Workspaces) != 1 || reloaded.Workspaces[0].ID != "uuid-1" {
		t.Fatalf("round trip lost data: %+v", reloaded)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/config/ -run TestLoadSlugPrecedence -v`
Expected: FAIL — `WorkspaceEntry` undefined / `DefaultWorkspace` undefined / slug is `my-workspace` not `acme`.

- [ ] **Step 3: Implement the config changes**

In `internal/config/config.go`, add the type above `type Config struct`:

```go
// WorkspaceEntry is one enrolled workspace in the local registry. Slug is the
// user-supplied workspace identifier (from the browser URL); ID caches the
// workspace UUID discovered during the enrollment probe (best-effort, may be
// empty). PATCH(workspace-registry): hand-added; re-apply after regen.
type WorkspaceEntry struct {
	Slug string `toml:"slug"`
	ID   string `toml:"id,omitempty"`
}
```

Add these two fields to `Config` (after `TemplateVars`):

```go
	// PATCH(workspace-registry): default_workspace seeds {slug} when PLANE_SLUG
	// is unset; workspaces is the locally-enrolled registry shown by
	// `workspaces list` (the public API cannot enumerate workspaces by key).
	DefaultWorkspace string           `toml:"default_workspace,omitempty"`
	Workspaces       []WorkspaceEntry `toml:"workspaces,omitempty"`
```

In `Load()`, replace the slug-resolution block:

```go
	if v := strings.TrimSpace(os.Getenv("PLANE_SLUG")); v != "" {
		cfg.TemplateVars["slug"] = normalizeEndpointTemplateValue(v)
	} else {
		cfg.TemplateVars["slug"] = "my-workspace"
	}
```

with (PATCH-tagged):

```go
	// PATCH(workspace-registry): precedence PLANE_SLUG env > default_workspace
	// (persisted) > "my-workspace" sentinel. The flag layer (--workspace) sits
	// above this and overrides TemplateVars["slug"] in rootFlags.newClient().
	if v := strings.TrimSpace(os.Getenv("PLANE_SLUG")); v != "" {
		cfg.TemplateVars["slug"] = normalizeEndpointTemplateValue(v)
	} else if cfg.DefaultWorkspace != "" {
		cfg.TemplateVars["slug"] = cfg.DefaultWorkspace
	} else {
		cfg.TemplateVars["slug"] = "my-workspace"
	}
```

Add a public Save wrapper after `func (c *Config) save() error { ... }`:

```go
// Save persists the config to disk. PATCH(workspace-registry): exposes the
// private save() so novel workspace commands can mutate DefaultWorkspace /
// Workspaces and write back without duplicating the toml round-trip.
func (c *Config) Save() error { return c.save() }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/config/ -run 'TestLoadSlugPrecedence|TestSaveWorkspaceRegistryRoundTrip' -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_workspace_test.go
git commit -m "feat(config): default_workspace + workspaces registry, slug precedence"
```

---

### Task 2: Root — `--workspace/-w` flag, newClient override, registrations

**Files:**
- Modify: `internal/cli/root.go`
- Test: `internal/cli/workspace_flag_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/cli/workspace_flag_test.go`:

```go
package cli

import "testing"

// The --workspace flag must override PLANE_SLUG/default_workspace by writing
// the client's TemplateVars["slug"]. We assert at the client level the dry-run
// URL targets the flag's workspace.
func TestWorkspaceFlagOverridesSlug(t *testing.T) {
	t.Setenv("PLANE_SLUG", "envslug")
	t.Setenv("PLANE_BASE_URL", "https://example.test/api/v1/workspaces/{slug}")
	f := &rootFlags{workspace: "flagslug", dryRun: true}
	c, err := f.newClient()
	if err != nil {
		t.Fatal(err)
	}
	if got := c.Config.TemplateVars["slug"]; got != "flagslug" {
		t.Fatalf("--workspace override: got %q want flagslug", got)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestWorkspaceFlagOverridesSlug -v`
Expected: FAIL — `rootFlags` has no field `workspace`.

- [ ] **Step 3: Implement**

In `internal/cli/root.go`, add to the `rootFlags` struct (near `selectFields string`):

```go
	// PATCH(workspace-flag): top-precedence workspace selector; overrides
	// PLANE_SLUG and default_workspace by writing TemplateVars["slug"] in
	// newClient(). Re-apply after regen.
	workspace string
```

Add the persistent flag (after the `--profile` line, before `--deliver`):

```go
	rootCmd.PersistentFlags().StringVarP(&flags.workspace, "workspace", "w", "", "Workspace slug to target (overrides PLANE_SLUG and default_workspace)")
```

In `newClient()`, after `c := client.New(...)` and before `return c, nil`, insert:

```go
	// PATCH(workspace-flag): --workspace is the top of the slug precedence
	// chain. config.Load already resolved PLANE_SLUG > default_workspace >
	// sentinel into TemplateVars["slug"]; the flag overrides that here.
	if f.workspace != "" {
		if cfg.TemplateVars == nil {
			cfg.TemplateVars = map[string]string{}
		}
		cfg.TemplateVars["slug"] = f.workspace
	}
```

(Note: `client.New(cfg, ...)` holds a pointer to `cfg`, so mutating `cfg.TemplateVars` after construction is observed by `buildURL`. Verified by the test.)

Register the novel commands in `newRootCmd`, next to the other `PATCH(novel)` lines (after `newAttachFileCmd`):

```go
	// PATCH(novel): workspace targeting + onboarding (init, workspaces use/add/list/current); see .printing-press-patches/.
	rootCmd.AddCommand(newInitCmd(flags))
	rootCmd.AddCommand(newWorkspacesCmd(flags))
```

- [ ] **Step 4: Run test to verify it fails to compile (commands not defined yet)**

Run: `go build ./... 2>&1 | head`
Expected: FAIL — `newInitCmd` / `newWorkspacesCmd` undefined. (Task 3 defines them.) Leave the registrations in place; proceed to Task 3, then this compiles.

- [ ] **Step 5: Commit (after Task 3 compiles — see Task 3 Step 7)**

Deferred to Task 3 so the tree builds before committing.

---

### Task 3: Novel — `workspaces` command group + probe + mutators

**Files:**
- Create: `internal/cli/workspaces_novel.go`
- Test: `internal/cli/workspaces_novel_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/cli/workspaces_novel_test.go`:

```go
package cli

import (
	"strings"
	"testing"

	"plane-pp-cli/internal/config"
)

func TestResolveWorkspaceSource(t *testing.T) {
	t.Setenv("PLANE_SLUG", "")
	cfg := &config.Config{DefaultWorkspace: "acme", TemplateVars: map[string]string{"slug": "acme"}}

	slug, src := resolveWorkspace(&rootFlags{}, cfg)
	if slug != "acme" || src != "config:default_workspace" {
		t.Fatalf("config source: got %q/%q", slug, src)
	}
	slug, src = resolveWorkspace(&rootFlags{workspace: "flag"}, cfg)
	if slug != "flag" || src != "flag:--workspace" {
		t.Fatalf("flag source: got %q/%q", slug, src)
	}
	t.Setenv("PLANE_SLUG", "envv")
	slug, src = resolveWorkspace(&rootFlags{}, cfg)
	if slug != "envv" || src != "env:PLANE_SLUG" {
		t.Fatalf("env source: got %q/%q", slug, src)
	}
}

func TestUpsertWorkspace(t *testing.T) {
	cfg := &config.Config{}
	upsertWorkspace(cfg, "acme", "id1")
	upsertWorkspace(cfg, "acme", "id2") // update id, no dup
	upsertWorkspace(cfg, "bravo", "")
	if len(cfg.Workspaces) != 2 {
		t.Fatalf("want 2 entries got %d", len(cfg.Workspaces))
	}
	if cfg.Workspaces[0].ID != "id2" {
		t.Fatalf("upsert should update id, got %q", cfg.Workspaces[0].ID)
	}
}

func TestBaseURLHasLiteralSlug(t *testing.T) {
	if !baseURLHasLiteralSlug("https://h/api/v1/workspaces/acme") {
		t.Fatal("literal slug not detected")
	}
	if baseURLHasLiteralSlug("https://h/api/v1/workspaces/{slug}") {
		t.Fatal("templated base wrongly flagged")
	}
}

func TestWorkspacesListBanner(t *testing.T) {
	var b strings.Builder
	cfg := &config.Config{
		DefaultWorkspace: "acme",
		Workspaces:       []config.WorkspaceEntry{{Slug: "acme", ID: "id1"}, {Slug: "bravo"}},
	}
	renderWorkspaceList(&b, cfg, "acme")
	out := b.String()
	if !strings.Contains(out, "acme") || !strings.Contains(out, "bravo") {
		t.Fatalf("list missing entries: %q", out)
	}
	if !strings.Contains(out, "cannot enumerate") {
		t.Fatalf("list missing API-limitation banner: %q", out)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run 'TestResolveWorkspaceSource|TestUpsertWorkspace|TestBaseURLHasLiteralSlug|TestWorkspacesListBanner' -v`
Expected: FAIL — undefined: `resolveWorkspace`, `upsertWorkspace`, `baseURLHasLiteralSlug`, `renderWorkspaceList`.

- [ ] **Step 3: Implement the novel file**

Create `internal/cli/workspaces_novel.go`:

```go
// Copyright 2026 The plane-pp-cli authors. Licensed under Apache-2.0. See LICENSE.
//
// Novel (hand-written) command — not regenerated by the press.
//
// Workspace targeting & onboarding. Plane's public REST API is strictly
// workspace-scoped and cannot enumerate a user's workspaces by API key
// (/api/v1/workspaces/ -> 404; the app's /api/users/me/workspaces/ rejects the
// key). The slug is therefore user-supplied input. These commands make it a
// first-class axis: an explicit, probe-validated enrollment (init / workspaces
// add), a persisted default (workspaces use), and inspection (list / current).
// The slug flows into the generated client's BaseURL template var {slug}
// (see config.TemplateVars and client/url.go buildURL).

package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"plane-pp-cli/internal/config"

	"github.com/spf13/cobra"
)

// literalSlugBase matches a base_url whose /workspaces/ segment is a concrete
// slug rather than the {slug} template var — the legacy anti-pattern that
// pins the CLI to one workspace and makes --workspace/default_workspace inert.
var literalSlugBase = regexp.MustCompile(`/workspaces/[^/{][^/]*`)

func baseURLHasLiteralSlug(base string) bool {
	return literalSlugBase.MatchString(base)
}

// resolveWorkspace returns the active slug and a human-readable source label,
// following the precedence flag --workspace > env PLANE_SLUG > config
// default_workspace > "" (none). cfg may be nil.
func resolveWorkspace(flags *rootFlags, cfg *config.Config) (slug, source string) {
	if flags != nil && flags.workspace != "" {
		return flags.workspace, "flag:--workspace"
	}
	if v := strings.TrimSpace(os.Getenv(envWorkspaceSlug)); v != "" {
		return v, "env:" + envWorkspaceSlug
	}
	if cfg != nil && cfg.DefaultWorkspace != "" {
		return cfg.DefaultWorkspace, "config:default_workspace"
	}
	return "", "unset"
}

// upsertWorkspace inserts or updates a workspace registry entry. A non-empty
// id overwrites an existing blank/old id; an empty id never clobbers a known one.
func upsertWorkspace(cfg *config.Config, slug, id string) {
	for i := range cfg.Workspaces {
		if cfg.Workspaces[i].Slug == slug {
			if id != "" {
				cfg.Workspaces[i].ID = id
			}
			return
		}
	}
	cfg.Workspaces = append(cfg.Workspaces, config.WorkspaceEntry{Slug: slug, ID: id})
}

// probeWorkspace validates access to a workspace by GETting its members list
// with the active credentials and the candidate slug. Returns the workspace
// UUID (best-effort, from a project payload) when reachable, or an error
// classified for exit codes. A 200 (even empty) means access is granted.
func probeWorkspace(ctx context.Context, flags *rootFlags, slug string) (string, error) {
	c, err := flags.newClient()
	if err != nil {
		return "", err
	}
	applyClientSlug(c, slug)
	if _, err := c.Get(ctx, "/members/", nil); err != nil {
		return "", classifyAPIError(err, flags)
	}
	// Best-effort workspace id: a project payload carries a `workspace` UUID.
	id := ""
	if data, perr := c.Get(ctx, "/projects/", map[string]string{"per_page": "1"}); perr == nil {
		id = extractWorkspaceID(data)
	}
	return id, nil
}

// extractWorkspaceID pulls the `workspace` UUID out of a projects-list payload,
// tolerating both a bare array and a {"results":[...]} envelope.
func extractWorkspaceID(data json.RawMessage) string {
	var arr []struct {
		Workspace string `json:"workspace"`
	}
	if json.Unmarshal(data, &arr) == nil {
		for _, p := range arr {
			if p.Workspace != "" {
				return p.Workspace
			}
		}
	}
	var env struct {
		Results []struct {
			Workspace string `json:"workspace"`
		} `json:"results"`
	}
	if json.Unmarshal(data, &env) == nil {
		for _, p := range env.Results {
			if p.Workspace != "" {
				return p.Workspace
			}
		}
	}
	return ""
}

// warnEnvShadow prints a stderr warning when an env var would shadow the
// freshly-written config default (so `workspaces use` doesn't appear to no-op).
func warnEnvShadow(w io.Writer, slug string) {
	if v := strings.TrimSpace(os.Getenv("PLANE_BASE_URL")); v != "" && baseURLHasLiteralSlug(v) {
		fmt.Fprintf(w, "warning: PLANE_BASE_URL=%s pins a literal workspace and OVERRIDES config; unset it or set it to a host with /workspaces/{slug}.\n", v)
	}
	if v := strings.TrimSpace(os.Getenv(envWorkspaceSlug)); v != "" && v != slug {
		fmt.Fprintf(w, "warning: %s=%s is set and overrides default_workspace; this session still targets %q. Unset %s to use the saved default.\n", envWorkspaceSlug, v, v, envWorkspaceSlug)
	}
}

func newWorkspacesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workspaces",
		Short: "Target and switch between workspaces (the slug Plane's API cannot enumerate by key).",
		Long: `Plane's public API is workspace-scoped and cannot list your workspaces
from an API key, so you enroll each slug once (from your browser URL) and the
CLI remembers it. The active workspace is chosen by precedence:
--workspace flag > $PLANE_SLUG > default_workspace (config).`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWorkspacesAddCmd(flags))
	cmd.AddCommand(newWorkspacesUseCmd(flags))
	cmd.AddCommand(newWorkspacesListCmd(flags))
	cmd.AddCommand(newWorkspacesCurrentCmd(flags))
	return cmd
}

func newWorkspacesAddCmd(flags *rootFlags) *cobra.Command {
	var makeDefault bool
	cmd := &cobra.Command{
		Use:     "add <slug> [<slug>...]",
		Short:   "Enroll one or more workspace slugs (each is access-probed before saving).",
		Example: "  plane-pp-cli workspaces add acme\n  plane-pp-cli workspaces add acme bravo --default acme",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			added := []map[string]any{}
			for _, slug := range args {
				id, perr := probeWorkspace(cmd.Context(), flags, slug)
				if perr != nil {
					return fmt.Errorf("workspace %q is not reachable with the current credentials: %w", slug, perr)
				}
				upsertWorkspace(cfg, slug, id)
				added = append(added, map[string]any{"slug": slug, "id": id})
			}
			if makeDefault || cfg.DefaultWorkspace == "" {
				cfg.DefaultWorkspace = args[0]
			}
			if err := cfg.Save(); err != nil {
				return configErr(err)
			}
			warnEnvShadow(cmd.ErrOrStderr(), cfg.DefaultWorkspace)
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, map[string]any{"added": added, "default": cfg.DefaultWorkspace})
			}
			for _, a := range added {
				fmt.Fprintf(cmd.OutOrStdout(), "✓ %s (id %s)\n", a["slug"], a["id"])
			}
			fmt.Fprintf(cmd.OutOrStdout(), "default workspace: %s\n", cfg.DefaultWorkspace)
			return nil
		},
	}
	cmd.Flags().BoolVar(&makeDefault, "default", false, "Make the first enrolled slug the default workspace")
	return cmd
}

func newWorkspacesUseCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "use <slug>",
		Short:   "Probe a workspace and set it as the default for future commands.",
		Example: "  plane-pp-cli workspaces use acme",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			slug := args[0]
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			id, perr := probeWorkspace(cmd.Context(), flags, slug)
			if perr != nil {
				return fmt.Errorf("workspace %q is not reachable with the current credentials: %w", slug, perr)
			}
			upsertWorkspace(cfg, slug, id)
			cfg.DefaultWorkspace = slug
			if err := cfg.Save(); err != nil {
				return configErr(err)
			}
			warnEnvShadow(cmd.ErrOrStderr(), slug)
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, map[string]any{"default": slug, "id": id})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "✓ now targeting %s (id %s)\n", slug, id)
			return nil
		},
	}
	return cmd
}

func newWorkspacesListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List enrolled workspaces (the API cannot enumerate them by key).",
		Example: "  plane-pp-cli workspaces list",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			current, _ := resolveWorkspace(flags, cfg)
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, map[string]any{
					"workspaces": cfg.Workspaces,
					"current":    current,
					"note":       "Plane's public API cannot enumerate workspaces by API key; this is the locally enrolled set.",
				})
			}
			renderWorkspaceList(cmd.OutOrStdout(), cfg, current)
			return nil
		},
	}
	return cmd
}

func renderWorkspaceList(w io.Writer, cfg *config.Config, current string) {
	if len(cfg.Workspaces) == 0 {
		fmt.Fprintln(w, "No workspaces enrolled yet. Run 'plane-pp-cli init' or 'plane-pp-cli workspaces add <slug>'.")
	}
	for _, ws := range cfg.Workspaces {
		marker := "  "
		if ws.Slug == current {
			marker = "* "
		}
		fmt.Fprintf(w, "%s%s\n", marker, ws.Slug)
	}
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "(Plane's public API cannot enumerate workspaces by API key — this is your locally enrolled set.)")
}

func newWorkspacesCurrentCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "current",
		Short:   "Show the active workspace and where it was resolved from.",
		Example: "  plane-pp-cli workspaces current",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			slug, source := resolveWorkspace(flags, cfg)
			if slug == "" {
				if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
					return flags.printJSON(cmd, map[string]any{"workspace": nil, "source": source})
				}
				fmt.Fprintln(cmd.OutOrStdout(), "no workspace set — run 'plane-pp-cli init' or pass --workspace")
				return nil
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				return flags.printJSON(cmd, map[string]any{"workspace": slug, "source": source})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s   (source: %s)\n", slug, source)
			return nil
		},
	}
	return cmd
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/ -run 'TestResolveWorkspaceSource|TestUpsertWorkspace|TestBaseURLHasLiteralSlug|TestWorkspacesListBanner' -v`
Expected: PASS

- [ ] **Step 5: Add the `init` command to the same file**

Append to `internal/cli/workspaces_novel.go`:

```go
// newInitCmd is the interactive onboarding entry point. Because the API cannot
// enumerate workspaces, init asks the user for their slug(s) (from the browser
// URL), probes each, and writes config (api key + host base_url + registry +
// default). In --no-input/agent mode it is non-interactive: every value must
// come from a flag.
func newInitCmd(flags *rootFlags) *cobra.Command {
	var apiKey, host, defaultWS string
	var slugFlags []string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "First-run setup: API key, host, and your workspace slug(s).",
		Long: `Interactive onboarding. Plane's API cannot list your workspaces from a
key, so you provide the slug(s) from your browser URL (app.plane.so/<slug>/);
each is access-probed before being saved. Use flags for non-interactive setup.`,
		Example: "  plane-pp-cli init\n  plane-pp-cli init --api-key $KEY --host https://plane.acme.com --workspace acme --workspace bravo",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			interactive := !flags.noInput && isTerminal(cmd.OutOrStdout())
			r := bufio.NewReader(cmd.InOrStdin())
			out := cmd.OutOrStdout()

			if apiKey == "" && interactive {
				fmt.Fprint(out, "API key (X-API-Key): ")
				apiKey = readLine(r)
			}
			if apiKey != "" {
				if err := cfg.SaveCredential(apiKey); err != nil {
					return configErr(fmt.Errorf("saving api key: %w", err))
				}
			}
			if host == "" && interactive {
				fmt.Fprint(out, "Host [https://api.plane.so]: ")
				host = readLine(r)
			}
			if host == "" {
				host = "https://api.plane.so"
			}
			cfg.BaseURL = normalizeHost(host) + "/api/v1/workspaces/{slug}"

			slugs := slugFlags
			if len(slugs) == 0 && interactive {
				fmt.Fprint(out, "Workspace slug(s), comma-separated (from app URL, e.g. app.plane.so/<slug>/): ")
				for _, s := range strings.Split(readLine(r), ",") {
					if s = strings.TrimSpace(s); s != "" {
						slugs = append(slugs, s)
					}
				}
			}
			if len(slugs) == 0 {
				return usageErr(fmt.Errorf("no workspace slug provided (pass --workspace or run interactively)"))
			}
			// Reload so the saved api key + base_url are in the client config.
			cfg, err = config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			cfg.BaseURL = normalizeHost(host) + "/api/v1/workspaces/{slug}"
			var enrolled []string
			for _, slug := range slugs {
				id, perr := probeWorkspace(cmd.Context(), flags, slug)
				if perr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "✗ %s: %v\n", slug, perr)
					continue
				}
				upsertWorkspace(cfg, slug, id)
				enrolled = append(enrolled, slug)
				fmt.Fprintf(out, "✓ %s (id %s)\n", slug, id)
			}
			if len(enrolled) == 0 {
				return apiErr(fmt.Errorf("no workspace could be reached with the provided credentials"))
			}
			if defaultWS == "" {
				defaultWS = enrolled[0]
			}
			cfg.DefaultWorkspace = defaultWS
			if err := cfg.Save(); err != nil {
				return configErr(err)
			}
			warnEnvShadow(cmd.ErrOrStderr(), defaultWS)
			if flags.asJSON {
				return flags.printJSON(cmd, map[string]any{"enrolled": enrolled, "default": defaultWS, "config": cfg.Path})
			}
			fmt.Fprintf(out, "Wrote %s (%d workspace(s), default=%s)\n", cfg.Path, len(enrolled), defaultWS)
			return nil
		},
	}
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key (X-API-Key); prompts if omitted and interactive")
	cmd.Flags().StringVar(&host, "host", "", "Plane host base, e.g. https://api.plane.so or https://plane.acme.com")
	cmd.Flags().StringArrayVar(&slugFlags, "workspace", nil, "Workspace slug to enroll (repeatable)")
	cmd.Flags().StringVar(&defaultWS, "default", "", "Which enrolled slug becomes the default (first if unset)")
	return cmd
}

func readLine(r *bufio.Reader) string {
	s, _ := r.ReadString('\n')
	return strings.TrimSpace(s)
}

// normalizeHost strips a trailing slash and any accidental /api/... suffix so
// the caller can append the canonical /api/v1/workspaces/{slug} tail exactly once.
func normalizeHost(h string) string {
	h = strings.TrimRight(strings.TrimSpace(h), "/")
	if i := strings.Index(h, "/api/"); i >= 0 {
		h = h[:i]
	}
	return h
}
```

Note: `init` defines a local `--workspace` flag on its own command; this shadows the persistent root `--workspace` for `init` only, which is intended (here it is repeatable enrollment input, not the global selector). Cobra resolves the local flag first.

- [ ] **Step 6: Build the whole tree (Task 2 registrations now resolve)**

Run: `go build ./...`
Expected: success (no undefined symbols).

- [ ] **Step 7: Run the full cli + config test packages**

Run: `go test ./internal/cli/ ./internal/config/`
Expected: PASS (including Task 2's `TestWorkspaceFlagOverridesSlug`).

- [ ] **Step 8: Commit Tasks 2 + 3 together**

```bash
git add internal/cli/root.go internal/cli/workspace_flag_test.go internal/cli/workspaces_novel.go internal/cli/workspaces_novel_test.go
git commit -m "feat(cli): --workspace flag + workspaces/init onboarding commands"
```

---

### Task 4: Doctor — workspace nudge + base_url migration warning

**Files:**
- Modify: `internal/cli/doctor.go`
- Test: `internal/cli/doctor_workspace_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `internal/cli/doctor_workspace_test.go`:

```go
package cli

import "testing"

func TestWorkspaceDoctorVerdict(t *testing.T) {
	// no workspace configured, sentinel slug
	if got := workspaceDoctorVerdict("my-workspace", "https://api.plane.so/api/v1/workspaces/{slug}", nil); got == "" {
		t.Fatal("expected a nudge when no workspace configured")
	}
	// configured workspace, templated base -> OK (empty verdict)
	cfg := []string{"acme"}
	if got := workspaceDoctorVerdict("acme", "https://api.plane.so/api/v1/workspaces/{slug}", cfg); got != "" {
		t.Fatalf("configured+templated should be OK, got %q", got)
	}
	// literal-slug base -> migration warning regardless of slug
	if got := workspaceDoctorVerdict("acme", "https://h/api/v1/workspaces/acme", cfg); got == "" {
		t.Fatal("expected a migration warning for literal-slug base_url")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/cli/ -run TestWorkspaceDoctorVerdict -v`
Expected: FAIL — `workspaceDoctorVerdict` undefined.

- [ ] **Step 3: Implement**

In `internal/cli/doctor.go`, add the helper at the end of the file:

```go
// workspaceDoctorVerdict returns a non-empty doctor message when workspace
// targeting needs attention: a literal-slug base_url (legacy anti-pattern that
// pins one workspace) takes priority, then "no workspace configured". Empty
// means healthy. PATCH(workspace-nudge): hand-added; re-apply after regen.
func workspaceDoctorVerdict(slug, baseURL string, enrolled []string) string {
	if baseURLHasLiteralSlug(baseURL) {
		return "WARN base_url pins a literal workspace (" + baseURL + ") — use /workspaces/{slug} + 'plane-pp-cli workspaces use <slug>' so --workspace works."
	}
	if slug == "" || slug == "my-workspace" {
		if len(enrolled) == 0 {
			return "INFO no workspace configured — run 'plane-pp-cli init' or pass --workspace"
		}
	}
	return ""
}
```

In `newDoctorCmd`'s `RunE`, after the cache report line (`report["cache"] = collectCacheReport(...)`), insert:

```go
		// PATCH(workspace-nudge): surface workspace-targeting health.
		if cfg != nil {
			enrolled := make([]string, 0, len(cfg.Workspaces))
			for _, ws := range cfg.Workspaces {
				enrolled = append(enrolled, ws.Slug)
			}
			if v := workspaceDoctorVerdict(cfg.TemplateVars["slug"], cfg.BaseURL, enrolled); v != "" {
				report["workspace"] = v
			}
		}
```

Add `{"workspace", "Workspace"}` to the `checkKeys` slice (after `{"config", "Config"}`):

```go
				{"config", "Config"},
				{"workspace", "Workspace"},
```

(The existing human-render switch already maps `INFO`/`WARN` prefixes to the right indicator.)

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/cli/ -run TestWorkspaceDoctorVerdict -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/cli/doctor.go internal/cli/doctor_workspace_test.go
git commit -m "feat(doctor): workspace-targeting nudge + base_url migration warning"
```

---

### Task 5: Full build, install (rename-swap), and live verification

**Files:** none (build/run only). Run from the publish-clone plane dir unless noted.

- [ ] **Step 1: Vet + full test suite**

Run: `go vet ./... && go test ./...`
Expected: PASS across all packages.

- [ ] **Step 2: Build both binaries to a scratch path**

```bash
go build -o /tmp/plane-pp-cli.exe ./cmd/plane-pp-cli
go build -o /tmp/plane-pp-mcp.exe ./cmd/plane-pp-mcp
```
Expected: two binaries, no errors.

- [ ] **Step 3: Dry-run proves the flag drives the URL (no install yet)**

```bash
PLANE_BASE_URL="https://plane.bbm.academy/api/v1/workspaces/{slug}" \
  /tmp/plane-pp-cli.exe projects list --workspace bbm --dry-run --agent
PLANE_BASE_URL="https://plane.bbm.academy/api/v1/workspaces/{slug}" \
  /tmp/plane-pp-cli.exe projects list --workspace doctor-school --dry-run --agent
```
Expected: first prints `GET …/workspaces/bbm/projects/`, second `…/workspaces/doctor-school/projects/`. **This is the core regression fixed** — distinct slugs now produce distinct URLs.

- [ ] **Step 4: Install via rename-swap (running .exe can be renamed, not overwritten)**

```powershell
$bin = "$env:USERPROFILE\go\bin"
Move-Item "$bin\plane-pp-cli.exe" "$bin\plane-pp-cli.exe.bak-20260610" -Force
Copy-Item /tmp/plane-pp-cli.exe "$bin\plane-pp-cli.exe"
Move-Item "$bin\plane-pp-mcp.exe" "$bin\plane-pp-mcp.exe.bak-20260610" -Force
Copy-Item /tmp/plane-pp-mcp.exe "$bin\plane-pp-mcp.exe"
```
Expected: installed binaries swapped; backups kept. (MCP picks up the new binary only after Claude Code restarts its MCP server.)

- [ ] **Step 5: Live-verify enrollment + targeting against BBM**

Temporarily unset the literal-slug env so config drives targeting:

```bash
unset PLANE_BASE_URL PLANE_SLUG
plane-pp-cli workspaces use bbm          # probes …/workspaces/bbm/members/ -> ✓
plane-pp-cli workspaces use doctor-school
plane-pp-cli workspaces list             # both shown, doctor-school current
plane-pp-cli workspaces current          # doctor-school (source: config:default_workspace)
plane-pp-cli members --workspace bbm --agent --select display_name   # Anton
plane-pp-cli members --agent --select display_name                   # doctor-school members
```
Expected: `use` validates and switches; `members --workspace bbm` returns bbm's members (Anton) while default returns doctor-school's — **distinct, correct, no false positive.** Record outputs.

- [ ] **Step 6: Verify the false-positive is gone and bad slug fails loudly**

```bash
plane-pp-cli workspaces use no-such-workspace   # must ERROR (probe 404), exit nonzero
echo "exit: $?"
```
Expected: a clear "not reachable with the current credentials" error, non-zero exit — not a silent fall-through.

- [ ] **Step 7: doctor reflects state**

```bash
unset PLANE_BASE_URL PLANE_SLUG; plane-pp-cli doctor
PLANE_BASE_URL="https://plane.bbm.academy/api/v1/workspaces/doctor-school" plane-pp-cli doctor
```
Expected: first shows `OK/INFO Workspace` (configured); second shows `WARN Workspace: base_url pins a literal workspace …` (migration warning).

- [ ] **Step 8: No commit (build artifacts only). Note results in the PR description draft.**

---

### Task 6: Recipe — correct the catalog guidance

**Files (recipe repo `C:\Users\sidor\repos\cli-printing-press`, branch off `feat/plane-catalog`):**
- Modify: `catalog/plane.yaml`

- [ ] **Step 1: Read the current auth_instructions / notes**

Run: `git -C C:/Users/sidor/repos/cli-printing-press log --oneline -3 -- catalog/plane.yaml` then open `catalog/plane.yaml`.
Find the `auth_instructions` and `notes` blocks that currently tell self-hosted users to set the base/server URL to the full `https://<host>/api/v1/workspaces/<slug>` prefix.

- [ ] **Step 2: Flip the guidance**

Replace the "full `…/workspaces/<slug>` prefix" instruction with the templated form. The base/server URL must be `https://<host>/api/v1/workspaces/{slug}` (literal `{slug}`), and the workspace is selected by one of: `--workspace <slug>`, `PLANE_SLUG`, or `plane-pp-cli workspaces use <slug>` / `plane-pp-cli init`. Keep one sentence noting the public API cannot enumerate workspaces by key, so the slug is user-supplied. Do NOT re-introduce the literal-prefix wording.

- [ ] **Step 3: Golden check**

The `catalog list` golden renders only name+description, so a `notes`/`auth_instructions` edit should not change it. Confirm:
Run: `cd C:/Users/sidor/repos/cli-printing-press && go test ./... -run Golden 2>&1 | tail -20` (or the repo's documented golden command).
Expected: golden tests PASS with no diff. If a golden does change, regenerate it per the repo's golden-update procedure and include it.

- [ ] **Step 4: Commit on a fresh branch**

```bash
cd C:/Users/sidor/repos/cli-printing-press
git checkout -b feat/plane-workspace-guidance feat/plane-catalog
git add catalog/plane.yaml
git commit -m "docs(plane): templated base_url + --workspace/PLANE_SLUG workspace selection"
```

---

### Task 7: Index patches, mirror to library, dogfood reconcile, open PRs

**Files:**
- Modify: `.printing-press-patches.json` (publish clone root)
- Sync: dogfood tree `C:\Users\sidor\printing-press\library\plane`

- [ ] **Step 1: Index the new patches**

Open `.printing-press-patches.json` at the publish-clone root. Add entries describing the regen-fragile edits so a future regen re-applies them: `config.go` (`PATCH(workspace-registry)`), `root.go` (`PATCH(workspace-flag)` + the two `PATCH(novel)` registrations), `doctor.go` (`PATCH(workspace-nudge)`), and the two new novel files (`workspaces_novel.go`, `workspaces_novel_test.go`) as additive novel sources. Mirror the shape of the existing `relations`/`module`/`attach-file` entries.

- [ ] **Step 2: Reconcile the dogfood tree**

The installed binary was rebuilt from the publish clone in Task 5. Bring the dogfood tree to parity so future local `go install` from it keeps the feature:
```bash
cp internal/config/config.go             C:/Users/sidor/printing-press/library/plane/internal/config/config.go
cp internal/cli/root.go                  C:/Users/sidor/printing-press/library/plane/internal/cli/root.go
cp internal/cli/doctor.go                C:/Users/sidor/printing-press/library/plane/internal/cli/doctor.go
cp internal/cli/workspaces_novel.go      C:/Users/sidor/printing-press/library/plane/internal/cli/workspaces_novel.go
cp internal/cli/workspaces_novel_test.go C:/Users/sidor/printing-press/library/plane/internal/cli/workspaces_novel_test.go
cp internal/cli/workspace_flag_test.go   C:/Users/sidor/printing-press/library/plane/internal/cli/workspace_flag_test.go
cp internal/config/config_workspace_test.go C:/Users/sidor/printing-press/library/plane/internal/config/config_workspace_test.go
cp internal/cli/doctor_workspace_test.go C:/Users/sidor/printing-press/library/plane/internal/cli/doctor_workspace_test.go
```
Then `cd C:/Users/sidor/printing-press/library/plane && go build ./... && go test ./internal/cli/ ./internal/config/`.
Expected: dogfood builds + passes. (Dogfood retains its own `attach_file_novel.go`, which the library clone still lacks — that drift is tracked separately and is out of scope for this PR.)

- [ ] **Step 3: Commit the patch index on the library branch**

```bash
# in the publish clone
git checkout -b feat/plane-workspaces feat/plane
git add .printing-press-patches.json docs/superpowers/specs docs/superpowers/plans
git commit -m "chore(plane): index workspace-targeting patches + design docs"
```
(The code commits from Tasks 1-4 are already on this branch if you branched before them; if you implemented on `feat/plane` directly, cherry-pick or rebase them onto `feat/plane-workspaces`.)

- [ ] **Step 4: Push and open the two PRs**

```bash
# Library (printing-press-library)
git push origin feat/plane-workspaces
gh pr create --repo mvanhorn/printing-press-library --head sidorovanthon:feat/plane-workspaces \
  --title "feat(plane): workspace targeting (--workspace, init, workspaces use/add/list/current)" \
  --body-file <draft including Task 5 live-verify outputs>

# Recipe (cli-printing-press)
cd C:/Users/sidor/repos/cli-printing-press && git push fork feat/plane-workspace-guidance
gh pr create --repo mvanhorn/cli-printing-press --head sidorovanthon:feat/plane-workspace-guidance \
  --title "docs(plane): templated base_url + --workspace workspace selection" \
  --body-file <draft referencing the library PR + engine issue #2599>
```
Expected: two PRs open. Note in the library PR body that this is the user-facing resolution of engine issue #2599 (slug as a flag-driven server var) implemented in the recipe+library layer.

- [ ] **Step 5: Update memory**

Update `plane_cli_notes.md`: the inert-positional/false-positive entry now has a fix — `--workspace` flag + `default_workspace`/`[[workspaces]]` registry + `init`/`workspaces` commands; env stays `PLANE_SLUG`; base_url must be templated `…/workspaces/{slug}`. Update `plane_catalog_upstream_pr.md` with the two new PRs.

---

## Self-Review

**Spec coverage:**
- Slug as separate axis / templated base → Task 1 (precedence) + Task 2 (flag) + Task 6 (recipe). ✓
- `--workspace` / `PLANE_SLUG` / `default_workspace` precedence → Task 1 + Task 2 + `resolveWorkspace` (Task 3). ✓
- Explicit probe-validated enrollment → `probeWorkspace` + `init` + `workspaces add` (Task 3). ✓
- `workspaces use/current/list` + API-can't-enumerate banner → Task 3. ✓
- doctor nudge + base_url migration → Task 4. ✓
- Out-of-the-box for any user → templated default base_url already shipped; `init`/`--workspace` are host-agnostic. ✓
- Fix the build (regen-durable) → PATCH tags + `.printing-press-patches.json` (Task 7). ✓
- Rebuild tool + live verify → Task 5. ✓
- Fix the recipe → Task 6. ✓
- Self-hosted `workspaces discover --via-db` — **intentionally deferred** (spec marked it optional/out of core); not in this plan.

**Placeholder scan:** No TBD/TODO; the only `<...>` placeholders are PR `--body-file` drafts (authored at Task 7 from real Task 5 output) — acceptable.

**Type consistency:** `WorkspaceEntry{Slug,ID}`, `Config.DefaultWorkspace`/`Config.Workspaces`, `rootFlags.workspace`, `resolveWorkspace(*rootFlags,*config.Config)(string,string)`, `upsertWorkspace(*config.Config,string,string)`, `probeWorkspace(ctx,*rootFlags,string)(string,error)`, `baseURLHasLiteralSlug(string)bool`, `renderWorkspaceList(io.Writer,*config.Config,string)`, `workspaceDoctorVerdict(string,string,[]string)string` — names consistent across tasks. `applyClientSlug`, `classifyAPIError`, `envWorkspaceSlug`, `dryRunOK`, `isTerminal`, `flags.printJSON`, `parentNoSubcommandRunE` reused from existing code with verified signatures.

**Env naming:** Reconciled to existing `PLANE_SLUG` (not the spec's aspirational `PLANE_WORKSPACE`); flag is `--workspace`. Update the spec's wording when convenient.
