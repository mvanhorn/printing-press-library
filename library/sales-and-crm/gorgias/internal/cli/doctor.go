// Copyright 2026 chrisyoungcooks. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gorgias/internal/client"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gorgias/internal/config"
	"github.com/mvanhorn/printing-press-library/library/sales-and-crm/gorgias/internal/store"
	"github.com/spf13/cobra"
)

func newDoctorCmd(flags *rootFlags) *cobra.Command {
	var failOn string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check CLI health",
		Long: "Runs a series of environment checks and prints a structured report.\n\n" +
			"What's checked:\n" +
			"  * config file loads (path, base_url, tenant slug)\n" +
			"  * tenant slug looks like a real subdomain (not the `app` or `your-company` placeholder)\n" +
			"  * credentials are configured (Basic auth header derivable from email + API key)\n" +
			"  * an authenticated GET /account succeeds — `credentials: valid` only when this passes\n" +
			"  * local mirror DB exists and the configured resources have been synced\n\n" +
			"Output is a JSON envelope when `--json` is set. Exit code is 0 by default even on warnings;\n" +
			"use `--fail-on warn` to fail on warnings (e.g. tenant placeholder) and `--fail-on error`\n" +
			"to fail only on real errors. Useful as a CI gate or as the first command an agent runs\n" +
			"when wiring up a new tenant.",
		Example: `  gorgias-pp-cli doctor
  gorgias-pp-cli doctor --json
  gorgias-pp-cli doctor --fail-on warn`,
		RunE: func(cmd *cobra.Command, args []string) error {
			report := map[string]any{}

			// Check config
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				report["config"] = fmt.Sprintf("error: %s", err)
			} else {
				// Distinguish "no config file (env-var auth works)" from
				// "user-pointed path is a typo". The first is normal; the
				// second silently strips a saved API key — exactly the
				// surprise top-10% doctors warn about.
				switch {
				case cfg.PathExplicit && !cfg.PathExists:
					report["config"] = fmt.Sprintf("warning: GORGIAS_CONFIG or --config points at %q which does not exist; falling back to env vars", cfg.Path)
				case cfg.PathExists:
					report["config"] = "ok"
				default:
					report["config"] = "ok (no config file; using env vars)"
				}
				report["config_path"] = cfg.Path
				report["base_url"] = cfg.BaseURL
				// Tenant detection: doctor's #1 first-five-minutes failure
				// is a tenant-URL mismatch. The base_url should resolve to
				// `<tenant>.gorgias.com` (a real account), not the generic
				// `app.gorgias.com` placeholder shipped in the spec. Surface
				// the tenant slug as a separate field so an agent or a human
				// scanning the doctor output can immediately spot the case
				// where the env var is unset and the CLI is hitting the
				// public Gorgias landing page.
				if cfg.BaseURL != "" {
					tenant := extractTenantSlug(cfg.BaseURL)
					report["tenant"] = tenant
					if tenant == "app" || tenant == "your-company" || tenant == "" {
						report["tenant_hint"] = "base_url points at the generic Gorgias host. Set GORGIAS_BASE_URL=https://<your-tenant>.gorgias.com/api"
					}
				}
			}

			// Check auth
			authConfigured := false
			if cfg != nil {
				header := cfg.AuthHeader()
				if header == "" {
					report["auth"] = "not configured"
					report["auth_hint"] = "export GORGIAS_USERNAME=<email> GORGIAS_API_KEY=<key>"
					report["auth_key_url"] = "https://docs.gorgias.com/en-US/rest-api-208286"
					report["auth_instructions"] = "Settings → REST API → API key (use your email as username)"
				} else {
					authConfigured = true
					report["auth"] = "configured"
					report["auth_source"] = cfg.AuthSource
				}
			}

			// Check auth environment variables
			authEnvSet := []string{}
			authEnvRequiredMissing := []string{}
			authEnvInfo := []string{}
			authEnvOptionalNames := []string{}
			// Validation rejects multi-OR-group specs upstream, so the single optional-satisfied state is sufficient at runtime.
			authEnvOptionalSatisfied := false
			if os.Getenv("GORGIAS_USERNAME") != "" {
				authEnvSet = append(authEnvSet, "GORGIAS_USERNAME")
			} else if authConfigured {
				authSource, _ := report["auth_source"].(string)
				if authSource == "" {
					authSource = "config"
				}
				authEnvInfo = append(authEnvInfo, "credentials available from "+authSource)
			} else {
				authEnvRequiredMissing = append(authEnvRequiredMissing, "GORGIAS_USERNAME")
			}
			if os.Getenv("GORGIAS_API_KEY") != "" {
				authEnvSet = append(authEnvSet, "GORGIAS_API_KEY")
			} else if authConfigured {
				authSource, _ := report["auth_source"].(string)
				if authSource == "" {
					authSource = "config"
				}
				authEnvInfo = append(authEnvInfo, "credentials available from "+authSource)
			} else {
				authEnvRequiredMissing = append(authEnvRequiredMissing, "GORGIAS_API_KEY")
			}
			switch {
			case len(authEnvRequiredMissing) > 0:
				report["env_vars"] = "ERROR missing required: " + strings.Join(authEnvRequiredMissing, ", ")
			case len(authEnvOptionalNames) > 1 && !authEnvOptionalSatisfied:
				report["env_vars"] = "INFO set one of: " + strings.Join(authEnvOptionalNames, " or ")
			case len(authEnvInfo) > 0 && authConfigured:
				report["env_vars"] = "OK " + strings.Join(authEnvInfo, "; ")
			case len(authEnvInfo) > 0:
				report["env_vars"] = "INFO " + strings.Join(authEnvInfo, "; ")
			default:
				report["env_vars"] = fmt.Sprintf("OK %d/%d available", len(authEnvSet), 2)
			}

			// Check API connectivity and validate credentials.
			//
			// The doctor uses the same client every other command uses —
			// `flags.newClient()` returns a `*client.Client` over a stdlib
			// `*http.Client`. Going through `flags.newClient()` keeps the
			// doctor's reachability verdict aligned with what real commands
			// experience (timeout, jar, base URL all match).
			if cfg != nil && cfg.BaseURL != "" {
				c, clientErr := flags.newClient()
				if clientErr != nil {
					report["api"] = fmt.Sprintf("client init error: %s", clientErr)
				} else {
					// Step 1: basic reachability. Gorgias is a plain HTTPS REST API —
					// no bot-wall handling required; transport failures propagate as
					// network errors, and HTTP status codes carry the diagnosis.
					_, reachErr := c.Get("/", nil)
					var reachAPIErr *client.APIError
					switch {
					case reachErr == nil:
						report["api"] = "reachable"
					case errors.As(reachErr, &reachAPIErr):
						// Non-2xx from the server. The network reached, the
						// server responded — but the response code carries
						// information: 401/403 is a credential problem;
						// 404 at `/` likely means the base URL is wrong;
						// 5xx is Gorgias-side. We grade accordingly so the
						// doctor doesn't paint a 404 misconfiguration green.
						status := reachAPIErr.StatusCode
						if status == 404 {
							report["api"] = fmt.Sprintf("warning: HTTP 404 at / — base_url may be wrong. Check GORGIAS_BASE_URL=https://<your-tenant>.gorgias.com/api")
						} else if status >= 500 {
							report["api"] = fmt.Sprintf("server error: HTTP %d at / (Gorgias side)", status)
						} else {
							report["api"] = fmt.Sprintf("reachable (HTTP %d at /)", status)
						}
					default:
						// Network-level failure: DNS, connection refused, TLS,
						// transport init, etc. The transport itself didn't
						// connect.
						report["api"] = fmt.Sprintf("unreachable: %s", reachErr)
					}

					// Step 2: Validate credentials with an authenticated probe.
					authHeader := cfg.AuthHeader()
					if authHeader == "" {
						// No auth configured — skip credential validation
					} else if reachErr != nil && !errors.As(reachErr, &reachAPIErr) {
						report["credentials"] = "skipped (API unreachable)"
					} else {
						verifyPath := "/account"
						if !strings.HasPrefix(verifyPath, "/") {
							verifyPath = "/" + verifyPath
						}
						authParams := map[string]string{}
						authHeaders := map[string]string{}
						authHeaders["Authorization"] = authHeader
						_, authErr := c.GetWithHeaders(verifyPath, authParams, authHeaders)
						var authAPIErr *client.APIError
						switch {
						case authErr == nil:
							report["credentials"] = "valid"
						case errors.As(authErr, &authAPIErr):
							switch {
							case authAPIErr.StatusCode == 401:
								report["credentials"] = fmt.Sprintf("invalid (HTTP %d) — check your credentials", authAPIErr.StatusCode)
							case authAPIErr.StatusCode == 403:
								report["credentials"] = fmt.Sprintf("scope-limited (HTTP %d) — credentials are valid but lack permission for this endpoint. Check your dashboard's API key scope.", authAPIErr.StatusCode)
							default:
								// Non-auth HTTP error (404, 500, etc.) — don't blame credentials
								report["credentials"] = fmt.Sprintf("ok (HTTP %d from %s, but auth was accepted)", authAPIErr.StatusCode, verifyPath)
							}
						default:
							report["credentials"] = fmt.Sprintf("error: %s", authErr)
						}
					}
				}
			} else if cfg != nil && cfg.BaseURL == "" {
				report["api"] = "not configured (set base_url in config file)"
			}
			// Cache health: only reported when this CLI has a local store.
			// Surfaces rows + last_synced_at per resource, schema version,
			// and a fresh/stale/unknown verdict so agents can introspect
			// whether to trust the cached data before issuing queries.
			report["cache"] = collectCacheReport(cmd.Context(), "")

			report["version"] = Version()

			if flags.asJSON {
				// In JSON mode, the doctor report already carries every
				// diagnostic line a caller needs (`auth`, `env_vars`,
				// `api`, `credentials`, `cache.status`). When --fail-on
				// triggers, we embed `fail_on_triggered` + `error` inside
				// the same report so callers see ONE JSON document on
				// stdout rather than a report doc + an error-envelope doc
				// on stderr. `silenceEmissionErr` tells Execute() to skip
				// re-emitting the error envelope but keep the exit code.
				failErr := doctorExitForFailOn(failOn, report)
				if failErr != nil {
					report["fail_on_triggered"] = failOn
					report["error"] = failErr.Error()
				}
				if err := printJSONFiltered(cmd.OutOrStdout(), report, flags); err != nil {
					return err
				}
				if failErr != nil {
					return silenceEmission(failErr)
				}
				return nil
			}

			// Human-readable output with color
			w := cmd.OutOrStdout()
			checkKeys := []struct{ key, label string }{
				{"config", "Config"},
				{"auth", "Auth"},
				{"env_vars", "Env Vars"},
				{"api", "API"},
				{"credentials", "Credentials"},
			}
			for _, ck := range checkKeys {
				v, ok := report[ck.key]
				if !ok {
					continue
				}
				s := fmt.Sprintf("%v", v)
				indicator := green("OK")
				switch {
				case strings.HasPrefix(s, "INFO"):
					indicator = yellow("INFO")
				case strings.HasPrefix(s, "ERROR"):
					indicator = red("FAIL")
				case strings.HasPrefix(s, "optional"):
					// Optional-auth CLI with no key set — informational, not a failure.
					indicator = yellow("INFO")
				case strings.Contains(s, "scope-limited"):
					indicator = yellow("WARN")
				case strings.Contains(s, "error") || strings.Contains(s, "not configured") || strings.Contains(s, "unreachable") || strings.Contains(s, "invalid") || strings.Contains(s, "missing"):
					indicator = red("FAIL")
				case s == "not required":
					// Public APIs: no auth needed is a healthy state, not a warning.
					indicator = green("OK")
				case strings.Contains(s, "not ") || strings.Contains(s, "skipped") || strings.Contains(s, "inferred"):
					indicator = yellow("WARN")
				}
				fmt.Fprintf(w, "  %s %s: %s\n", indicator, ck.label, s)
			}
			// Print info keys without status indicator
			for _, key := range []string{"config_path", "base_url", "auth_source", "version"} {
				if v, ok := report[key]; ok {
					fmt.Fprintf(w, "  %s: %v\n", key, v)
				}
			}
			// Print auth setup hints (indented under Auth line)
			if hint, ok := report["auth_hint"]; ok {
				fmt.Fprintf(w, "  hint: %v\n", hint)
			}
			if keyURL, ok := report["auth_key_url"]; ok {
				fmt.Fprintf(w, "  Get a key at: %v\n", keyURL)
			}
			if instructions, ok := report["auth_instructions"]; ok {
				fmt.Fprintf(w, "  %v\n", instructions)
			}
			// Cache section: render after the primary health block so it
			// sits next to version info, mirroring the JSON report layout.
			if cacheAny, ok := report["cache"]; ok {
				if cacheRep, ok := cacheAny.(map[string]any); ok {
					renderCacheReport(w, cacheRep)
				}
			}
			return doctorExitForFailOn(failOn, report)
		},
	}
	cmd.Flags().StringVar(&failOn, "fail-on", "error", "Exit non-zero when a health level is reached (one of: error, stale, never). Default 'error' so CI/scripted setups treat a doctor FAIL as an actual failure.")
	return cmd
}

// extractTenantSlug parses a Gorgias base URL like
// https://<tenant>.gorgias.com/api and returns "<tenant>". Returns "" when
// the URL doesn't match the gorgias.com hostname pattern, "app" for the
// generic placeholder shipped in the spec, and the literal subdomain
// otherwise. Used by doctor to surface tenant-URL mismatches up-front.
func extractTenantSlug(baseURL string) string {
	u := strings.TrimSpace(baseURL)
	u = strings.TrimPrefix(u, "https://")
	u = strings.TrimPrefix(u, "http://")
	if idx := strings.Index(u, "/"); idx >= 0 {
		u = u[:idx]
	}
	if !strings.HasSuffix(u, ".gorgias.com") {
		return ""
	}
	return strings.TrimSuffix(u, ".gorgias.com")
}

// doctorExitForFailOn returns a non-nil error when the report's worst
// status meets or exceeds the --fail-on threshold. "error" trips when any
// section reports an error; "stale" also trips when the cache section is
// stale; "never" never trips. The default is "error" so CI scripts that
// key off the exit code see a doctor FAIL as an actual failure — the
// human shell still gets the full diagnostic in stdout.
func doctorExitForFailOn(failOn string, report map[string]any) error {
	if failOn == "never" {
		return nil
	}
	worstError := false
	worstStale := false
	for _, v := range report {
		s, ok := v.(string)
		if ok {
			if strings.HasPrefix(s, "ERROR") || strings.Contains(s, "error") || strings.Contains(s, "unreachable") || strings.Contains(s, "invalid") || strings.Contains(s, "missing") || strings.Contains(s, "not configured") {
				worstError = true
			}
		}
		if m, ok := v.(map[string]any); ok {
			if st, _ := m["status"].(string); st == "error" {
				worstError = true
			} else if st == "stale" {
				worstStale = true
			}
		}
	}
	switch failOn {
	case "", "error":
		if worstError {
			return fmt.Errorf("doctor: --fail-on=error triggered")
		}
	case "stale":
		if worstError || worstStale {
			return fmt.Errorf("doctor: --fail-on=stale triggered")
		}
	default:
		return fmt.Errorf("doctor: unknown --fail-on value %q (valid: error, stale, never)", failOn)
	}
	return nil
}

// collectCacheReport opens the local store, reads per-resource sync state,
// and returns a map summarising cache health. Never panics on missing DB
// or open failure; returns a map with status=unknown or status=error so the
// caller can render and agents can interpret.
//
// staleAfterSpec is the CLI's configured threshold (e.g. "6h"); empty means
// use the runtime default. The default is deliberately conservative (6h)
// because the alternative is no freshness story at all.
func collectCacheReport(ctx context.Context, staleAfterSpec string) map[string]any {
	report := map[string]any{}
	dbPath := defaultDBPath("gorgias-pp-cli")
	report["db_path"] = dbPath

	fi, err := os.Stat(dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			report["status"] = "unknown"
			report["hint"] = "Database not created yet; run 'gorgias-pp-cli sync' to hydrate."
			return report
		}
		report["status"] = "error"
		report["error"] = err.Error()
		return report
	}
	report["db_bytes"] = fi.Size()

	s, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		report["status"] = "error"
		report["error"] = err.Error()
		return report
	}
	defer s.Close()

	if v, verr := s.SchemaVersion(); verr == nil {
		report["schema_version"] = v
	}

	staleAfter := 6 * time.Hour
	if staleAfterSpec != "" {
		if d, derr := time.ParseDuration(staleAfterSpec); derr == nil {
			staleAfter = d
		}
	}

	rows, qerr := s.DB().Query(`SELECT resource_type, COALESCE(total_count, 0), last_synced_at FROM sync_state ORDER BY resource_type`)
	if qerr != nil {
		// sync_state may not exist on a fresh DB that has migrated but not
		// yet had any sync runs — treat as unknown rather than error.
		report["status"] = "unknown"
		report["hint"] = "No sync state recorded; run 'gorgias-pp-cli sync' to populate."
		return report
	}
	defer rows.Close()

	var resources []map[string]any
	fresh := true
	haveAny := false
	oldest := time.Duration(0)
	for rows.Next() {
		var rtype string
		var count int64
		var lastSynced sql.NullTime
		if err := rows.Scan(&rtype, &count, &lastSynced); err != nil {
			continue
		}
		r := map[string]any{"type": rtype, "rows": count}
		if lastSynced.Valid {
			haveAny = true
			r["last_synced_at"] = lastSynced.Time.UTC().Format(time.RFC3339)
			age := time.Since(lastSynced.Time)
			r["staleness"] = age.Round(time.Minute).String()
			if age > staleAfter {
				fresh = false
			}
			if age > oldest {
				oldest = age
			}
		} else {
			r["staleness"] = "never"
			fresh = false
		}
		resources = append(resources, r)
	}
	report["resources"] = resources
	report["stale_after"] = staleAfter.String()

	switch {
	case !haveAny && len(resources) == 0:
		report["status"] = "unknown"
		report["hint"] = "sync_state is empty; run 'gorgias-pp-cli sync' to hydrate."
	case fresh:
		report["status"] = "fresh"
	default:
		report["status"] = "stale"
		report["oldest_age"] = oldest.Round(time.Minute).String()
		report["hint"] = "Some resources are older than stale_after; run 'gorgias-pp-cli sync' to refresh."
	}
	return report
}

func renderCacheReport(w io.Writer, rep map[string]any) {
	status, _ := rep["status"].(string)
	indicator := green("OK")
	switch status {
	case "stale":
		indicator = yellow("WARN")
	case "error":
		indicator = red("FAIL")
	case "unknown":
		indicator = yellow("INFO")
	}
	fmt.Fprintf(w, "  %s Cache: %s\n", indicator, status)
	if v, ok := rep["db_path"]; ok {
		fmt.Fprintf(w, "    db_path: %v\n", v)
	}
	if v, ok := rep["schema_version"]; ok {
		fmt.Fprintf(w, "    schema_version: %v\n", v)
	}
	if v, ok := rep["db_bytes"]; ok {
		fmt.Fprintf(w, "    db_bytes: %v\n", v)
	}
	if v, ok := rep["stale_after"]; ok {
		fmt.Fprintf(w, "    stale_after: %v\n", v)
	}
	if v, ok := rep["oldest_age"]; ok {
		fmt.Fprintf(w, "    oldest_age: %v\n", v)
	}
	if resourcesAny, ok := rep["resources"]; ok {
		if resources, ok := resourcesAny.([]map[string]any); ok && len(resources) > 0 {
			fmt.Fprintf(w, "    resources:\n")
			for _, r := range resources {
				rtype, _ := r["type"].(string)
				rows := r["rows"]
				staleness, _ := r["staleness"].(string)
				fmt.Fprintf(w, "      - %s: %v rows, %s\n", rtype, rows, staleness)
			}
		}
	}
	if hint, ok := rep["hint"]; ok {
		fmt.Fprintf(w, "    hint: %v\n", hint)
	}
}
