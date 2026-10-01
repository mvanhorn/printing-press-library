// Copyright 2026 Matt Anderson and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/actual"
	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/other/actual-budget/internal/config"
)

// addActualDoctorChecks reports the native read path (Actual sync server,
// login, local mirror) and the most common write-path failure: an
// actual-http-api sidecar whose bundled @actual-app/api version does not match
// the server (the cause of "no such column" and export breakage reports).
func addActualDoctorChecks(cmd *cobra.Command, flags *rootFlags, report map[string]any) {
	defer downgradeOptionalSidecar(flags, report)
	syncID := resolveSyncID(flags)
	if syncID == "" {
		report["actual_budget"] = "WARN no budget selected: set " + config.EnvSyncID + " (list ids with 'budgets-remote')"
	} else {
		report["actual_budget"] = "sync id " + syncID
	}
	if _, m, found, err := actual.LocateMirror(syncID); syncID != "" && err == nil {
		if !found {
			report["actual_mirror"] = "INFO not pulled yet — run 'mirror pull'"
		} else if m != nil {
			age := time.Since(m.PulledAt).Round(time.Minute)
			report["actual_mirror"] = fmt.Sprintf("pulled %s ago (%s)", age, m.Name)
		} else {
			report["actual_mirror"] = "present"
		}
	}
	serverURL := config.ActualServerURL()
	if serverURL == "" {
		report["actual_server"] = "WARN " + config.EnvServerURL + " not set — offline commands need 'mirror pull' from your Actual server"
		return
	}
	if dryRunOK(flags) {
		report["actual_server"] = "skipped (--dry-run)"
		return
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
	defer cancel()
	c := actual.New(serverURL, 10*time.Second)
	serverVersion, err := c.ServerVersion(ctx)
	if err != nil {
		report["actual_server"] = "ERROR unreachable: " + err.Error()
		return
	}
	report["actual_server"] = "reachable (Actual " + serverVersion + ")"
	report["actual_server_version"] = serverVersion
	if config.ActualPassword() == "" && config.ActualSessionToken() == "" {
		report["actual_login"] = "WARN " + config.EnvPassword + " not set"
	} else if err := actual.Authenticate(ctx, c, config.ActualPassword(), config.ActualSessionToken()); err != nil {
		report["actual_login"] = "ERROR " + err.Error()
	} else if _, err := c.ListFiles(ctx); err != nil {
		report["actual_login"] = "ERROR " + err.Error()
	} else {
		report["actual_login"] = "ok"
	}
	if sidecarDown(report) {
		report["sidecar_version_skew"] = "INFO actual-http-api sidecar not running (only needed for write commands)"
		return
	}
	report["sidecar_version_skew"] = sidecarSkew(ctx, flags, serverVersion)
}

// sidecarSkew compares actual-http-api's version with the Actual server.
// actual-http-api releases track @actual-app/api versions one-to-one.
func sidecarSkew(ctx context.Context, flags *rootFlags, serverVersion string) string {
	c, err := flags.newClient()
	if err != nil {
		return "INFO sidecar not configured (only needed for write commands)"
	}
	data, err := c.Get(ctx, "/actualhttpapiversion", nil)
	if err != nil {
		return "INFO actual-http-api sidecar not reachable at " + strings.TrimSuffix(actual.RedactURL(c.RequestBaseURL()), "/") + " (only needed for write commands)"
	}
	var v struct {
		Data struct {
			Version string `json:"version"`
		} `json:"data"`
		Version string `json:"version"`
	}
	_ = json.Unmarshal(data, &v)
	sidecar := v.Data.Version
	if sidecar == "" {
		sidecar = v.Version
	}
	if sidecar == "" {
		return "INFO sidecar reachable; version unknown"
	}
	if majorMinor(sidecar) != majorMinor(serverVersion) {
		return fmt.Sprintf("WARN actual-http-api %s vs Actual server %s — upgrade the sidecar image to match the server", sidecar, serverVersion)
	}
	return fmt.Sprintf("ok (actual-http-api %s, server %s)", sidecar, serverVersion)
}

func majorMinor(v string) string {
	parts := strings.SplitN(strings.TrimPrefix(v, "v"), ".", 3)
	if len(parts) < 2 {
		return v
	}
	return parts[0] + "." + parts[1]
}

// defaultSidecarBaseURL is the generated client's default actual-http-api URL.
const defaultSidecarBaseURL = "http://localhost:5007/v1"

func sidecarDown(report map[string]any) bool {
	api, _ := report["api"].(string)
	return strings.HasPrefix(api, "unreachable") || strings.HasPrefix(api, "not configured")
}

// downgradeOptionalSidecar turns an unreachable sidecar into INFO unless the
// user configured one (API key, or a base URL other than the default via env
// or config file). Reads come from the mirror, so an absent sidecar is not a
// failure; a configured one that is down still is.
func downgradeOptionalSidecar(flags *rootFlags, report map[string]any) {
	if !sidecarDown(report) {
		return
	}
	if cliutil.EnvOverride("ACTUAL_HTTP_API_KEY") != "" {
		return
	}
	if cfg, err := config.Load(flags.configPath); err == nil && (cfg.AuthHeader() != "" || strings.TrimRight(cfg.BaseURL, "/") != defaultSidecarBaseURL) {
		return
	}
	report["api"] = "INFO actual-http-api sidecar not running (optional; needed only for write and endpoint commands)"
}
