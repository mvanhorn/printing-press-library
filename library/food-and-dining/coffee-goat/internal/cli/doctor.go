// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/config"
	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

// doctorCheck is one row in the doctor report. status is one of
// "PASS", "WARN", "FAIL", or "INFO".
type doctorCheck struct {
	Name    string `json:"name"`
	Status  string `json:"status"`
	Detail  string `json:"detail,omitempty"`
	Hint    string `json:"hint,omitempty"`
	Elapsed string `json:"elapsed,omitempty"`
}

// representativeShopifyURL is the URL the doctor probes to verify the
// CLI can reach a Shopify storefront over the public internet. Onyx
// was chosen because it is the highest-traffic Shopify storefront in
// the registry and rate-limits gracefully under probe load.
const representativeShopifyURL = "https://onyxcoffeelab.com/products.json?limit=1"

func newDoctorCmd(flags *rootFlags) *cobra.Command {
	var failOn string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check coffee-goat CLI health: config, store, representative roaster, and youtube-pp-cli helper",
		Example: `  coffee-goat-pp-cli doctor
  coffee-goat-pp-cli doctor --json
  coffee-goat-pp-cli doctor --fail-on warn`,
		RunE: func(cmd *cobra.Command, args []string) error {
			checks := runDoctorChecks(cmd.Context(), flags)
			if flags.asJSON {
				if err := printJSONFiltered(cmd.OutOrStdout(), map[string]any{"checks": checks, "version": version}, flags); err != nil {
					return err
				}
				return doctorExitForChecks(failOn, checks)
			}

			w := cmd.OutOrStdout()
			for _, c := range checks {
				ind := green("PASS")
				switch c.Status {
				case "WARN":
					ind = yellow("WARN")
				case "FAIL":
					ind = red("FAIL")
				case "INFO":
					ind = yellow("INFO")
				}
				if c.Elapsed != "" {
					fmt.Fprintf(w, "  %s %s [%s]: %s\n", ind, c.Name, c.Elapsed, c.Detail)
				} else {
					fmt.Fprintf(w, "  %s %s: %s\n", ind, c.Name, c.Detail)
				}
				if c.Hint != "" {
					fmt.Fprintf(w, "      hint: %s\n", c.Hint)
				}
			}
			fmt.Fprintf(w, "  version: %s\n", version)
			return doctorExitForChecks(failOn, checks)
		},
	}
	cmd.Flags().StringVar(&failOn, "fail-on", "", "Exit non-zero when worst status reaches the level: warn, error.")
	cmd.Annotations = map[string]string{"mcp:read-only": "true"}
	return cmd
}

// runDoctorChecks executes every health check and returns them as
// structured rows. The order is the order they will be printed.
func runDoctorChecks(ctx context.Context, flags *rootFlags) []doctorCheck {
	var checks []doctorCheck

	// 1. Config
	cfg, cfgErr := config.Load(flags.configPath)
	switch {
	case cfgErr != nil:
		checks = append(checks, doctorCheck{Name: "Config", Status: "FAIL", Detail: cfgErr.Error()})
	default:
		checks = append(checks, doctorCheck{Name: "Config", Status: "PASS", Detail: cfg.Path})
	}

	// 2. Local store
	dbPath := defaultDBPath("coffee-goat-pp-cli")
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			checks = append(checks, doctorCheck{
				Name: "Local store", Status: "INFO",
				Detail: "not created yet",
				Hint:   "run 'coffee-goat-pp-cli sync' to hydrate",
			})
		} else {
			checks = append(checks, doctorCheck{Name: "Local store", Status: "FAIL", Detail: err.Error()})
		}
	} else {
		s, oerr := store.OpenWithContext(ctx, dbPath)
		if oerr != nil {
			checks = append(checks, doctorCheck{Name: "Local store", Status: "FAIL", Detail: oerr.Error()})
		} else {
			defer s.Close()
			var productCount, brewCount int
			_ = s.DB().QueryRow(`SELECT COUNT(*) FROM roaster_products`).Scan(&productCount)
			_ = s.DB().QueryRow(`SELECT COUNT(*) FROM brews`).Scan(&brewCount)
			checks = append(checks, doctorCheck{
				Name: "Local store", Status: "PASS",
				Detail: fmt.Sprintf("%s (%d products, %d brews)", dbPath, productCount, brewCount),
			})
		}
	}

	// 3. Representative roaster reachability — short-circuit under
	// verify so a sandboxed test environment without network doesn't
	// fail the gate.
	if cliutil.IsVerifyEnv() {
		checks = append(checks, doctorCheck{
			Name: "Roaster reachability", Status: "INFO",
			Detail: "skipped under PRINTING_PRESS_VERIFY",
		})
	} else {
		checks = append(checks, probeShopifyEndpoint(ctx))
	}

	// 4. youtube-pp-cli helper on PATH
	checks = append(checks, probeYoutubePPCli())

	// 5. Cache freshness — surface per-resource sync_state age so agents
	// can decide whether to trust cached reads before issuing queries.
	checks = append(checks, cacheFreshnessCheck(ctx))

	// 6. Optional auth tokens — coffee-goat is unauthenticated for read
	// paths (every roaster is a public Shopify endpoint), but OPENROUTER_API_KEY
	// unlocks the LLM-backed `scan` bag-photo helper. Surface its presence
	// so agents can detect which capabilities are reachable.
	checks = append(checks, authTokensCheck())

	return checks
}

// authTokensCheck reports whether optional auth tokens are present. Coffee-goat
// itself needs no auth — the read path is unauthenticated against public
// Shopify endpoints, the Coffee Review WP REST API, and YouTube transcripts.
// OPENROUTER_API_KEY is the one optional token; presence enables `scan`.
func authTokensCheck() doctorCheck {
	if os.Getenv("OPENROUTER_API_KEY") != "" {
		return doctorCheck{
			Name: "Auth tokens", Status: "PASS",
			Detail: "OPENROUTER_API_KEY present (scan enabled)",
		}
	}
	return doctorCheck{
		Name: "Auth tokens", Status: "INFO",
		Detail: "OPENROUTER_API_KEY unset — read paths still work; scan disabled",
		Hint:   "export OPENROUTER_API_KEY=... to enable the bag-photo scanner",
	}
}

// cacheFreshnessCheck wraps collectCacheReport into a doctorCheck row.
// The threshold passed here is a coarse global default for the human-readable
// row; collectCacheReport itself applies the per-resource overrides from
// cachePolicy() (products 6h, reviews 24h, videos 48h) when computing the
// individual resource staleness fields.
func cacheFreshnessCheck(ctx context.Context) doctorCheck {
	rep := collectCacheReport(ctx, "")
	status, _ := rep["status"].(string)
	switch status {
	case "fresh":
		return doctorCheck{Name: "Cache", Status: "PASS", Detail: cacheDetail(rep)}
	case "stale":
		return doctorCheck{
			Name: "Cache", Status: "WARN",
			Detail: cacheDetail(rep),
			Hint:   "run 'coffee-goat-pp-cli sync' to refresh",
		}
	case "unknown":
		hint, _ := rep["hint"].(string)
		return doctorCheck{Name: "Cache", Status: "INFO", Detail: cacheDetail(rep), Hint: hint}
	case "error":
		errStr, _ := rep["error"].(string)
		return doctorCheck{Name: "Cache", Status: "FAIL", Detail: errStr}
	default:
		return doctorCheck{Name: "Cache", Status: "INFO", Detail: cacheDetail(rep)}
	}
}

func cacheDetail(rep map[string]any) string {
	parts := []string{}
	if v, ok := rep["status"].(string); ok {
		parts = append(parts, v)
	}
	if v, ok := rep["oldest_age"].(string); ok {
		parts = append(parts, "oldest="+v)
	}
	if v, ok := rep["stale_after"].(string); ok {
		parts = append(parts, "stale_after="+v)
	}
	return strings.Join(parts, " ")
}

// collectCacheReport opens the local store, reads per-resource sync_state,
// and returns a map summarising cache health. Per-resource staleness uses
// the same thresholds as auto_refresh's cachePolicy() so doctor and the
// runtime auto-refresh hook agree on what "stale" means. Never panics on
// missing DB or open failure; returns a map with status=unknown or
// status=error so the caller can render and agents can interpret.
//
// staleAfterSpec, when non-empty, overrides the per-resource thresholds
// with a single coarse value — useful for ad-hoc human inspection. The
// default ("") falls through to cachePolicy()'s per-resource map.
func collectCacheReport(ctx context.Context, staleAfterSpec string) map[string]any {
	report := map[string]any{}
	dbPath := defaultDBPath("coffee-goat-pp-cli")
	report["db_path"] = dbPath

	fi, err := os.Stat(dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			report["status"] = "unknown"
			report["hint"] = "Database not created yet; run 'coffee-goat-pp-cli sync' to hydrate."
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

	policy := cachePolicy()
	thresholdFor := func(resourceType string) time.Duration {
		if staleAfterSpec != "" {
			if d, derr := time.ParseDuration(staleAfterSpec); derr == nil {
				return d
			}
		}
		if d, ok := policy.PerResource[resourceType]; ok {
			return d
		}
		return policy.StaleAfter
	}

	rows, qerr := s.DB().QueryContext(ctx, `SELECT resource_type, COALESCE(total_count, 0), last_synced_at FROM sync_state ORDER BY resource_type`)
	if qerr != nil {
		report["status"] = "unknown"
		report["hint"] = "No sync state recorded; run 'coffee-goat-pp-cli sync' to populate."
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
		threshold := thresholdFor(rtype)
		r := map[string]any{"type": rtype, "rows": count, "stale_after": threshold.String()}
		if lastSynced.Valid {
			haveAny = true
			r["last_synced_at"] = lastSynced.Time.UTC().Format(time.RFC3339)
			age := time.Since(lastSynced.Time)
			r["staleness"] = age.Round(time.Minute).String()
			if age > threshold {
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
	if err := rows.Err(); err != nil {
		report["status"] = "error"
		report["error"] = fmt.Sprintf("iterate sync_state rows: %v", err)
		return report
	}
	report["resources"] = resources
	if staleAfterSpec != "" {
		if d, derr := time.ParseDuration(staleAfterSpec); derr == nil {
			report["stale_after"] = d.String()
		}
	} else {
		report["stale_after"] = "per-resource (see cachePolicy)"
	}

	switch {
	case !haveAny && len(resources) == 0:
		report["status"] = "unknown"
		report["hint"] = "sync_state is empty; run 'coffee-goat-pp-cli sync' to hydrate."
	case fresh:
		report["status"] = "fresh"
	default:
		report["status"] = "stale"
		report["oldest_age"] = oldest.Round(time.Minute).String()
		report["hint"] = "Some resources are older than their per-resource stale_after; run 'coffee-goat-pp-cli sync' to refresh."
	}
	return report
}

// probeShopifyEndpoint hits the representative Shopify storefront
// with a 5s timeout. PASS on 200, WARN on a non-2xx, FAIL on a
// transport error.
func probeShopifyEndpoint(ctx context.Context) doctorCheck {
	c := &http.Client{Timeout: 5 * time.Second}
	started := time.Now()
	req, err := http.NewRequestWithContext(ctx, "GET", representativeShopifyURL, nil)
	if err != nil {
		return doctorCheck{Name: "Roaster reachability", Status: "FAIL", Detail: err.Error()}
	}
	req.Header.Set("User-Agent", "coffee-goat-pp-cli/"+version+" (+https://github.com/justinwfu/coffee-goat-pp-cli)")
	resp, err := c.Do(req)
	elapsed := time.Since(started)
	if err != nil {
		return doctorCheck{
			Name: "Roaster reachability", Status: "FAIL",
			Detail:  fmt.Sprintf("could not reach onyxcoffeelab.com: %v", err),
			Elapsed: elapsed.Round(time.Millisecond).String(),
			Hint:    "check your network connection",
		}
	}
	defer resp.Body.Close()
	// Drain a small prefix so connection reuse works; never reads the
	// whole body.
	_, _ = io.CopyN(io.Discard, resp.Body, 4096)
	if resp.StatusCode == 200 {
		return doctorCheck{
			Name: "Roaster reachability", Status: "PASS",
			Detail:  fmt.Sprintf("onyxcoffeelab.com HTTP %d", resp.StatusCode),
			Elapsed: elapsed.Round(time.Millisecond).String(),
		}
	}
	return doctorCheck{
		Name: "Roaster reachability", Status: "WARN",
		Detail:  fmt.Sprintf("onyxcoffeelab.com HTTP %d", resp.StatusCode),
		Elapsed: elapsed.Round(time.Millisecond).String(),
	}
}

// probeYoutubePPCli reports whether the optional `youtube-pp-cli`
// helper is on PATH. Missing is a WARN (creator-review still works
// over the synced corpus; sync just can't fetch new transcripts).
func probeYoutubePPCli() doctorCheck {
	path, err := exec.LookPath("youtube-pp-cli")
	if err != nil {
		return doctorCheck{
			Name: "youtube-pp-cli helper", Status: "WARN",
			Detail: "not on PATH",
			Hint:   "install with 'coffee-goat-pp-cli sync --source youtube' to enable; install via Printing Press public library",
		}
	}
	return doctorCheck{
		Name: "youtube-pp-cli helper", Status: "PASS",
		Detail: path,
	}
}

// doctorExitForChecks returns a non-nil error when the worst-status
// among checks meets or exceeds the --fail-on threshold. "" never
// fails; "warn" fails on WARN or worse; "error" fails on FAIL only.
func doctorExitForChecks(failOn string, checks []doctorCheck) error {
	if failOn == "" {
		return nil
	}
	worstFail, worstWarn := false, false
	for _, c := range checks {
		switch c.Status {
		case "FAIL":
			worstFail = true
		case "WARN":
			worstWarn = true
		}
	}
	switch strings.ToLower(failOn) {
	case "error", "fail":
		if worstFail {
			return fmt.Errorf("doctor: --fail-on=error triggered")
		}
	case "warn":
		if worstFail || worstWarn {
			return fmt.Errorf("doctor: --fail-on=warn triggered")
		}
	default:
		return fmt.Errorf("doctor: unknown --fail-on value %q (valid: warn, error)", failOn)
	}
	return nil
}

// Use sql import so the linter is happy when the optional cache
// branch is exercised. Kept as a no-op blank reference.
var _ = sql.ErrNoRows
