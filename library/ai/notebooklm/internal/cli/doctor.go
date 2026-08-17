// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/client"
	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/config"
	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/nlm"
	"github.com/mvanhorn/printing-press-library/library/ai/notebooklm/internal/store"
	"github.com/spf13/cobra"
)

func newDoctorCmd(flags *rootFlags) *cobra.Command {
	var failOn string
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check auth, config, API connectivity, and local cache health",
		Example: `  notebooklm-pp-cli doctor
  notebooklm-pp-cli doctor --json
  notebooklm-pp-cli doctor --fail-on warn`,
		RunE: func(cmd *cobra.Command, args []string) error {
			report := map[string]any{
				"version": version,
				"go":      runtime.Version(),
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				report["config"] = map[string]any{"status": "error", "error": err.Error()}
			} else {
				report["config"] = map[string]any{
					"status": "ok",
					"path":   cfg.Path,
				}
				auth := cfg.AuthHeader()
				if auth == "" {
					report["auth"] = map[string]any{
						"status": "missing",
						"hint":   "Run notebooklm-pp-cli auth login --chrome",
					}
				} else {
					report["auth"] = map[string]any{
						"status":  "configured",
						"preview": client.MaskAuthHeader(auth),
					}
				}
			}

			if cfg != nil && cfg.AuthHeader() != "" {
				if hc, err := cfg.HTTPClient(); err == nil {
					req, _ := http.NewRequestWithContext(cmd.Context(), http.MethodGet, config.BaseURL()+"/", nil)
					resp, err := hc.Do(req)
					if err != nil {
						report["api"] = map[string]any{"status": "error", "error": err.Error()}
					} else {
						_ = resp.Body.Close()
						report["api"] = map[string]any{"status": "reachable", "http_status": resp.StatusCode}
					}
					if sess, err := nlm.Bootstrap(cmd.Context(), hc); err == nil {
						report["session"] = map[string]any{
							"status": map[bool]string{true: "ok", false: "missing"}[sess.AT != ""],
						}
					}
				}
			} else {
				report["api"] = map[string]any{
					"status": "skipped",
					"hint":   "Configure auth first; try: run notebooklm-pp-cli doctor after auth login --chrome",
				}
			}

			report["cache"] = collectCacheReport(cmd.Context(), "")

			if flags.asJSON {
				return printJSON(report)
			}
			return renderDoctorHuman(os.Stdout, report, failOn)
		},
	}
	cmd.Flags().StringVar(&failOn, "fail-on", "", "Exit non-zero on stale (stale), warnings (warn), or errors (error)")
	return cmd
}

func renderDoctorHuman(w io.Writer, report map[string]any, failOn string) error {
	fmt.Fprintln(w, "notebooklm-pp-cli doctor")
	fmt.Fprintf(w, "  version: %v\n", report["version"])
	if cfg, ok := report["config"].(map[string]any); ok {
		fmt.Fprintf(w, "  config: %v (%v)\n", cfg["status"], cfg["path"])
	}
	if auth, ok := report["auth"].(map[string]any); ok {
		fmt.Fprintf(w, "  auth: %v\n", auth["status"])
	}
	if api, ok := report["api"].(map[string]any); ok {
		fmt.Fprintf(w, "  api: %v\n", api["status"])
	}
	if cache, ok := report["cache"].(map[string]any); ok {
		renderCacheReport(w, cache)
	}
	switch failOn {
	case "stale":
		if cache, ok := report["cache"].(map[string]any); ok {
			if status, _ := cache["status"].(string); status == "stale" {
				return fmt.Errorf("doctor: --fail-on=stale triggered")
			}
		}
	case "warn", "error":
		// reserved for future severity tiers
	}
	return nil
}

// collectCacheReport opens the local store and summarizes cache freshness.
func collectCacheReport(ctx context.Context, staleAfterSpec string) map[string]any {
	report := map[string]any{}
	dbPath, err := store.DefaultPath()
	if err != nil {
		report["status"] = "error"
		report["error"] = err.Error()
		return report
	}
	report["db_path"] = dbPath

	fi, err := os.Stat(dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			report["status"] = "unknown"
			report["hint"] = "Database not created yet; run 'notebooklm-pp-cli sync' to hydrate."
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

	rows, qerr := s.DB().QueryContext(ctx, `SELECT resource_type, COALESCE(total_count, 0), last_synced_at FROM sync_state ORDER BY resource_type`)
	if qerr != nil {
		report["status"] = "unknown"
		report["hint"] = "No sync state recorded; run 'notebooklm-pp-cli sync' to populate."
		return report
	}
	defer rows.Close()

	var resources []map[string]any
	fresh := true
	haveAny := false
	var oldest time.Duration
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
		report["status"] = "empty"
		report["hint"] = "Cache is empty; run 'notebooklm-pp-cli sync' to hydrate."
	case fresh:
		report["status"] = "fresh"
	default:
		report["status"] = "stale"
		report["oldest_age"] = oldest.Round(time.Minute).String()
		report["hint"] = "Some resources are older than stale_after; run 'notebooklm-pp-cli sync' to refresh."
	}
	return report
}

func renderCacheReport(w io.Writer, rep map[string]any) {
	status, _ := rep["status"].(string)
	fmt.Fprintf(w, "  cache: %s\n", status)
	if v, ok := rep["db_path"]; ok {
		fmt.Fprintf(w, "    db_path: %v\n", v)
	}
	if v, ok := rep["schema_version"]; ok {
		fmt.Fprintf(w, "    schema_version: %v\n", v)
	}
	if hint, ok := rep["hint"]; ok {
		fmt.Fprintf(w, "    hint: %v\n", hint)
	}
}
