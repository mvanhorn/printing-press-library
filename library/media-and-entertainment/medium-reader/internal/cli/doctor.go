// Copyright 2026 Maxime Delavergne and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/auth"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/source"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/medium-reader/internal/store"
	"github.com/spf13/cobra"
)

// looksLikeDoctorInterstitial reports whether the response body matches a known
// bot-detection challenge page (Cloudflare, Akamai, Vercel, AWS WAF, DataDome,
// PerimeterX). Only fires on the doctor probe — used to distinguish "transport
// reached the wall" from "transport failed entirely." Returns the vendor name
// when matched, or empty string when no match.
//
// Markers are anchored to <title> or vendor-specific strings to avoid
// false-positives on benign content. For example, a recipe titled "Just A
// Moment of Pause Cookies" must NOT match the Cloudflare challenge marker;
// only "<title>just a moment" (the actual interstitial title) does.
func looksLikeDoctorInterstitial(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	limit := len(body)
	if limit > 8192 {
		limit = 8192
	}
	prefix := strings.ToLower(string(body[:limit]))
	if !strings.Contains(prefix, "<title") {
		// Every bot interstitial we recognize sets a <title>; bodies without
		// one are body-only API responses, not challenge pages.
		return ""
	}
	switch {
	case strings.Contains(prefix, "<title>just a moment") || // CF JS challenge
		strings.Contains(prefix, "challenges.cloudflare.com") || // CF Turnstile
		(strings.Contains(prefix, "attention required") && strings.Contains(prefix, "cloudflare")):
		return "Cloudflare"
	case strings.Contains(prefix, "akamai") && (strings.Contains(prefix, "request unsuccessful") || strings.Contains(prefix, "access denied")):
		return "Akamai"
	case strings.Contains(prefix, "x-vercel-mitigated") || strings.Contains(prefix, "x-vercel-challenge-token") ||
		(strings.Contains(prefix, "vercel") && strings.Contains(prefix, "challenge")):
		return "Vercel"
	case strings.Contains(prefix, "request blocked") && strings.Contains(prefix, "aws waf"):
		return "AWS WAF"
	case strings.Contains(prefix, "datadome") && (strings.Contains(prefix, "blocked") || strings.Contains(prefix, "captcha") || strings.Contains(prefix, "challenge")):
		return "DataDome"
	case strings.Contains(prefix, "perimeterx") || strings.Contains(prefix, "px-captcha"):
		return "PerimeterX"
	}
	return ""
}

func newDoctorCmd(flags *rootFlags) *cobra.Command {
	var failOn string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check CLI health",
		Example: `  medium-reader-pp-cli doctor
  medium-reader-pp-cli doctor --json
  medium-reader-pp-cli doctor --fail-on error`,
		RunE: func(cmd *cobra.Command, args []string) error {
			report := map[string]any{}

			// v2 is $0/no-key: there is no API key to validate. Health is
			// (1) can the Surf transport reach Medium and clear the bot wall
			// anonymously (Tier 0), and (2) is an optional Tier-1 session
			// cookie configured.

			// Transport reachability — one GET to Medium's home over the same
			// Surf (Chrome-impersonation) client every command uses, so the
			// verdict matches what feed/read/search actually experience. A
			// probe failure is reported, never fatal: doctor stays exit 0
			// unless --fail-on is set.
			httpClient := source.NewHTTPClient(flags.timeout)
			req, reqErr := http.NewRequestWithContext(cmd.Context(), http.MethodGet, "https://medium.com/", nil)
			if reqErr != nil {
				report["medium"] = fmt.Sprintf("probe init error: %s", reqErr)
			} else {
				resp, rerr := httpClient.Do(req)
				if rerr != nil {
					report["medium"] = fmt.Sprintf("unreachable: %s", rerr)
				} else {
					body, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
					_ = resp.Body.Close()
					if vendor := looksLikeDoctorInterstitial(body); vendor != "" {
						report["medium"] = fmt.Sprintf("blocked by %s interstitial (HTTP %d) — try a different network or wait for the IP-level rate limit to clear", vendor, resp.StatusCode)
					} else {
						report["medium"] = fmt.Sprintf("reachable (HTTP %d)", resp.StatusCode)
					}
				}
			}

			// Session tier — the optional, user-supplied Medium cookie. None
			// configured is the default healthy state (Tier 0, anonymous); a
			// configured cookie is reported with the token masked so scripted
			// stdout never leaks the session.
			cookies, cookieErr := auth.Load(auth.Options{CookieFile: flags.cookieFile})
			switch {
			case cookieErr != nil:
				report["session"] = fmt.Sprintf("error loading cookie: %s", cookieErr)
			case cookies.IsZero():
				report["session"] = "Tier 0 (anonymous) — no cookie configured; member-locked articles return previews"
			default:
				report["session"] = fmt.Sprintf("Tier 1 — session cookie present (sid: %s)", masked(cookies.Sid))
			}

			// Cache health: per-resource rows + freshness from the local store
			// populated by author-archive. Reported only when a store exists.
			report["cache"] = collectCacheReport(cmd.Context(), "720h")

			// Verify-mode foot-gun detector (inherited): surfaces an accidental
			// PRINTING_PRESS_VERIFY=1 inherited from a parent shell / CI runner.
			if cliutil.IsVerifyEnv() {
				if cliutil.IsVerifyLiveHTTPEnv() {
					report["verify_mode"] = "INFO ACTIVE — live HTTP opt-in"
				} else {
					report["verify_mode"] = "INFO ACTIVE — mutating HTTP verbs short-circuit (PRINTING_PRESS_VERIFY=1)"
				}
			} else {
				report["verify_mode"] = "normal operation"
			}

			report["version"] = version

			if flags.asJSON {
				if err := printJSONFiltered(cmd.OutOrStdout(), report, flags); err != nil {
					return err
				}
				return doctorExitForFailOn(failOn, report)
			}

			// Human-readable output with color.
			w := cmd.OutOrStdout()
			checkKeys := []struct{ key, label string }{
				{"medium", "Medium"},
				{"session", "Session"},
				{"verify_mode", "Verify Mode"},
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
				case strings.Contains(s, "unreachable") || strings.Contains(s, "error"):
					indicator = red("FAIL")
				case strings.Contains(s, "blocked by"):
					indicator = yellow("WARN")
				}
				fmt.Fprintf(w, "  %s %s: %s\n", indicator, ck.label, s)
			}
			fmt.Fprintf(w, "  version: %v\n", report["version"])
			// Cache section: render after the primary health block so it sits
			// next to version info, mirroring the JSON report layout.
			if cacheAny, ok := report["cache"]; ok {
				if cacheRep, ok := cacheAny.(map[string]any); ok {
					renderCacheReport(w, cacheRep)
				}
			}
			return doctorExitForFailOn(failOn, report)
		},
	}
	cmd.Flags().StringVar(&failOn, "fail-on", "", "Exit non-zero when a health level is reached: stale, error. Default is never.")
	return cmd
}

// doctorExitForFailOn returns a non-nil error when the report's worst
// status meets or exceeds the --fail-on threshold. "error" always trips
// when any section reports an error; "stale" also trips when the cache
// section is stale. The default empty string means never fail on status.
func doctorExitForFailOn(failOn string, report map[string]any) error {
	if failOn == "" {
		return nil
	}
	worstError := false
	worstStale := false
	for _, v := range report {
		s, ok := v.(string)
		if ok {
			if strings.Contains(s, "error") || strings.Contains(s, "unreachable") || strings.Contains(s, "invalid") || strings.Contains(s, "missing") {
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
	case "error":
		if worstError {
			return fmt.Errorf("doctor: --fail-on=error triggered")
		}
	case "stale":
		if worstError || worstStale {
			return fmt.Errorf("doctor: --fail-on=stale triggered")
		}
	default:
		return fmt.Errorf("doctor: unknown --fail-on value %q (valid: stale, error)", failOn)
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
	dbPath := defaultDBPath("medium-reader-pp-cli")
	report["db_path"] = dbPath

	fi, err := os.Stat(dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			report["status"] = "unknown"
			report["hint"] = "Database not created yet; run 'medium-reader-pp-cli author-archive <author>' to populate it."
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
		// yet had any archive runs — treat as unknown rather than error.
		report["status"] = "unknown"
		report["hint"] = "No archive state recorded; run 'medium-reader-pp-cli author-archive <author>' to populate it."
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
		report["hint"] = "Archive state is empty; run 'medium-reader-pp-cli author-archive <author>' to populate it."
	case fresh:
		report["status"] = "fresh"
	default:
		report["status"] = "stale"
		report["oldest_age"] = oldest.Round(time.Minute).String()
		report["hint"] = "Some resources are older than stale_after; re-run 'medium-reader-pp-cli author-archive <author>' to refresh."
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
