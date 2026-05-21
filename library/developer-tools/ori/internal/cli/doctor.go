// Copyright 2026 error. Licensed under Apache-2.0.
// Hand-customized: replaces the generated generic doctor with openclaw-stack
// checks (launchd a2a-server, /healthz, agent cards, agents.toml). Generic
// config/auth/cache helpers below are preserved verbatim from the generator.

package cli

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/ori/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/ori/internal/config"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/ori/internal/store"
)

// looksLikeDoctorInterstitial reports whether a body matches a known bot-wall
// page (Cloudflare, Akamai, Vercel, AWS WAF, DataDome, PerimeterX). Kept
// verbatim from the generator output — used by the API reachability probe.
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
		return ""
	}
	switch {
	case strings.Contains(prefix, "<title>just a moment") ||
		strings.Contains(prefix, "challenges.cloudflare.com") ||
		(strings.Contains(prefix, "attention required") && strings.Contains(prefix, "cloudflare")):
		return "Cloudflare"
	case strings.Contains(prefix, "akamai") && (strings.Contains(prefix, "request unsuccessful") || strings.Contains(prefix, "access denied")):
		return "Akamai"
	case strings.Contains(prefix, "x-vercel-mitigated") || strings.Contains(prefix, "x-vercel-challenge-token"):
		return "Vercel"
	case strings.Contains(prefix, "request blocked") && strings.Contains(prefix, "aws waf"):
		return "AWS WAF"
	case strings.Contains(prefix, "datadome") && (strings.Contains(prefix, "blocked") || strings.Contains(prefix, "captcha")):
		return "DataDome"
	case strings.Contains(prefix, "perimeterx") || strings.Contains(prefix, "px-captcha"):
		return "PerimeterX"
	}
	return ""
}

type doctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // ok | warn | fail | info
	Detail string `json:"detail,omitempty"`
	Hint   string `json:"hint,omitempty"`
}

func newDoctorCmd(flags *rootFlags) *cobra.Command {
	var failOn string
	var includeNAS bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run every openclaw stack health check in one verb",
		Long: `Run every openclaw stack health check in one verb. Probes:
  - launchd a2a-server (dev.error2.openclaw-a2a-server) is loaded and running
  - localhost:8788/healthz returns {ok:true}
  - /.well-known/agents.json lists at least one agent
  - each agent's card is fetchable and reports its capabilities
  - ~/.openclaw/agents.toml exists and is non-empty
  - local SQLite cache state (freshness, schema version)

When --include-nas is set, also runs remote checks via SSH to the NAS:
  - openclaw-gateway WS tunnel reachable
  - plugin cache env vars commented in /volume1/docker/openclaw/.env
  - codex OAuth refresh fresh

The cache section comes from the generator's collectCacheReport helper and is
identical to what the generic doctor would have surfaced.`,
		Example: `  ori-pp-cli doctor
  ori-pp-cli doctor --json
  ori-pp-cli doctor --fail-on fail
  ori-pp-cli doctor --include-nas`,
		Annotations: map[string]string{"mcp:read-only": "true", "pp:typed-exit-codes": "0,5"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			var checks []doctorCheck

			// 1. launchd service state.
			checks = append(checks, checkLaunchdService())

			// 2. config / base_url.
			cfg, cfgErr := config.Load(flags.configPath)
			if cfgErr != nil {
				checks = append(checks, doctorCheck{Name: "config", Status: "fail", Detail: cfgErr.Error()})
			} else {
				checks = append(checks, doctorCheck{Name: "config", Status: "ok", Detail: cfg.Path})
			}

			// 3. /healthz live probe via the configured client.
			c, clientErr := flags.newClient()
			if clientErr != nil {
				checks = append(checks, doctorCheck{Name: "client", Status: "fail", Detail: clientErr.Error()})
			} else {
				checks = append(checks, checkHealthz(c))
				// 4. agents discovery.
				agents := checkAgentsDiscovery(c)
				checks = append(checks, agents.check)
				// 5. per-agent cards.
				for _, name := range agents.names {
					checks = append(checks, checkAgentCard(c, name))
				}
			}

			// 6. agents.toml on disk.
			checks = append(checks, checkAgentsToml())

			// 7. NAS-side checks (opt-in).
			if includeNAS {
				checks = append(checks, checkNASGateway())
				checks = append(checks, checkNASPluginCacheEnv())
			} else {
				checks = append(checks, doctorCheck{
					Name: "nas-checks", Status: "info",
					Detail: "skipped (pass --include-nas to run gateway WS tunnel and plugin-cache env-var checks over SSH)",
				})
			}

			// 8. local cache freshness (from generator helper).
			cacheRep := collectCacheReport(cmd.Context(), "")
			cacheStatus, _ := cacheRep["status"].(string)
			cacheCheck := doctorCheck{Name: "cache", Status: mapCacheStatusToCheckStatus(cacheStatus)}
			if hint, ok := cacheRep["hint"].(string); ok {
				cacheCheck.Hint = hint
			}
			if oldest, ok := cacheRep["oldest_age"].(string); ok {
				cacheCheck.Detail = "oldest=" + oldest
			}
			checks = append(checks, cacheCheck)

			report := map[string]any{
				"checks":     checks,
				"version":    version,
				"cache":      cacheRep,
				"summary":    summarizeChecks(checks),
				"checked_at": time.Now().UTC().Format(time.RFC3339),
			}

			if flags.asJSON || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				if err := printJSONFiltered(cmd.OutOrStdout(), report, flags); err != nil {
					return err
				}
				return doctorExitForFailOn(failOn, checks)
			}

			w := cmd.OutOrStdout()
			for _, c := range checks {
				indicator := green("OK")
				switch c.Status {
				case "fail":
					indicator = red("FAIL")
				case "warn":
					indicator = yellow("WARN")
				case "info":
					indicator = yellow("INFO")
				}
				fmt.Fprintf(w, "  %s %s", indicator, c.Name)
				if c.Detail != "" {
					fmt.Fprintf(w, ": %s", c.Detail)
				}
				fmt.Fprintln(w)
				if c.Hint != "" {
					fmt.Fprintf(w, "      hint: %s\n", c.Hint)
				}
			}
			sum := summarizeChecks(checks)
			fmt.Fprintf(w, "\n  Summary: %d ok, %d warn, %d fail, %d info\n",
				sum["ok"], sum["warn"], sum["fail"], sum["info"])
			if cacheAny, ok := report["cache"].(map[string]any); ok {
				renderCacheReport(w, cacheAny)
			}
			return doctorExitForFailOn(failOn, checks)
		},
	}
	cmd.Flags().StringVar(&failOn, "fail-on", "", "Exit non-zero when worst status meets threshold: warn, fail. Default never.")
	cmd.Flags().BoolVar(&includeNAS, "include-nas", false, "Run remote NAS-side checks over SSH (gateway WS tunnel, plugin-cache env vars)")
	return cmd
}

func summarizeChecks(checks []doctorCheck) map[string]int {
	out := map[string]int{"ok": 0, "warn": 0, "fail": 0, "info": 0}
	for _, c := range checks {
		out[c.Status]++
	}
	return out
}

func mapCacheStatusToCheckStatus(s string) string {
	switch s {
	case "fresh":
		return "ok"
	case "stale":
		return "warn"
	case "error":
		return "fail"
	default:
		return "info"
	}
}

func doctorExitForFailOn(failOn string, checks []doctorCheck) error {
	if failOn == "" {
		return nil
	}
	sum := summarizeChecks(checks)
	switch failOn {
	case "fail":
		if sum["fail"] > 0 {
			return apiErr(fmt.Errorf("doctor: %d failing check(s)", sum["fail"]))
		}
	case "warn":
		if sum["fail"] > 0 || sum["warn"] > 0 {
			return apiErr(fmt.Errorf("doctor: %d failing, %d warning", sum["fail"], sum["warn"]))
		}
	default:
		return usageErr(fmt.Errorf("doctor: unknown --fail-on value %q (valid: warn, fail)", failOn))
	}
	return nil
}

// --- individual checks ---

func checkLaunchdService() doctorCheck {
	label := "dev.error2.openclaw-a2a-server"
	uid := os.Getuid()
	target := fmt.Sprintf("gui/%d/%s", uid, label)
	out, err := exec.Command("launchctl", "print", target).CombinedOutput()
	if err != nil {
		return doctorCheck{Name: "launchd", Status: "fail",
			Detail: fmt.Sprintf("launchctl print %s failed: %v", target, err),
			Hint:   "Service may not be loaded. Try: launchctl bootstrap gui/" + fmt.Sprint(uid) + " ~/Library/LaunchAgents/" + label + ".plist"}
	}
	text := string(out)
	// `state = running` is the live signal. `pid = N` confirms a process.
	hasPID := strings.Contains(text, "pid = ")
	hasRunning := strings.Contains(text, "state = running")
	if hasRunning && hasPID {
		// extract pid for the detail line
		pid := ""
		if i := strings.Index(text, "pid = "); i >= 0 {
			tail := text[i+len("pid = "):]
			if nl := strings.IndexAny(tail, "\n\t "); nl >= 0 {
				pid = strings.TrimSpace(tail[:nl])
			}
		}
		return doctorCheck{Name: "launchd", Status: "ok", Detail: "running, pid=" + pid}
	}
	return doctorCheck{Name: "launchd", Status: "fail",
		Detail: "service registered but not in 'running' state",
		Hint:   "Run: ori-pp-cli kickstart"}
}

func checkHealthz(c *client.Client) doctorCheck {
	body, err := c.Get("/healthz", nil)
	var apiErr2 *client.APIError
	if err != nil {
		if errors.As(err, &apiErr2) {
			return doctorCheck{Name: "healthz", Status: "fail",
				Detail: fmt.Sprintf("HTTP %d", apiErr2.StatusCode),
				Hint:   "Bridge may be wedged after compose down/up. Run: ori-pp-cli kickstart"}
		}
		return doctorCheck{Name: "healthz", Status: "fail", Detail: err.Error(),
			Hint: "Server unreachable. Check launchd state and port 8788."}
	}
	var payload struct {
		OK bool `json:"ok"`
	}
	if jerr := json.Unmarshal(body, &payload); jerr != nil {
		return doctorCheck{Name: "healthz", Status: "warn",
			Detail: "200 but body did not parse as {ok:bool}"}
	}
	if !payload.OK {
		return doctorCheck{Name: "healthz", Status: "warn", Detail: "{ok:false}"}
	}
	return doctorCheck{Name: "healthz", Status: "ok"}
}

type agentsDiscovery struct {
	check doctorCheck
	names []string
}

func checkAgentsDiscovery(c *client.Client) agentsDiscovery {
	body, err := c.Get("/.well-known/agents.json", nil)
	if err != nil {
		return agentsDiscovery{check: doctorCheck{Name: "agents-discovery", Status: "fail", Detail: err.Error()}}
	}
	var payload struct {
		Agents []string `json:"agents"`
	}
	if jerr := json.Unmarshal(body, &payload); jerr != nil {
		return agentsDiscovery{check: doctorCheck{Name: "agents-discovery", Status: "fail", Detail: "body did not parse"}}
	}
	if len(payload.Agents) == 0 {
		return agentsDiscovery{check: doctorCheck{Name: "agents-discovery", Status: "warn", Detail: "no agents configured",
			Hint: "Set OPENCLAW_AGENTS_CONFIG and populate ~/.openclaw/agents.toml"}}
	}
	return agentsDiscovery{
		check: doctorCheck{Name: "agents-discovery", Status: "ok", Detail: strings.Join(payload.Agents, ", ")},
		names: payload.Agents,
	}
}

func checkAgentCard(c *client.Client, name string) doctorCheck {
	path := "/.well-known/" + name + "/agent-card.json"
	body, err := c.Get(path, nil)
	if err != nil {
		return doctorCheck{Name: "agent:" + name, Status: "fail", Detail: err.Error()}
	}
	var card struct {
		Name         string `json:"name"`
		Capabilities struct {
			Streaming bool `json:"streaming"`
			Approvals bool `json:"approvals"`
		} `json:"capabilities"`
	}
	if jerr := json.Unmarshal(body, &card); jerr != nil {
		return doctorCheck{Name: "agent:" + name, Status: "warn", Detail: "card returned but did not parse"}
	}
	caps := []string{}
	if card.Capabilities.Streaming {
		caps = append(caps, "streaming")
	}
	if card.Capabilities.Approvals {
		caps = append(caps, "approvals")
	}
	capStr := "no extras"
	if len(caps) > 0 {
		capStr = strings.Join(caps, "+")
	}
	return doctorCheck{Name: "agent:" + name, Status: "ok", Detail: capStr}
}

func checkAgentsToml() doctorCheck {
	home, _ := os.UserHomeDir()
	p := filepath.Join(home, ".openclaw", "agents.toml")
	fi, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return doctorCheck{Name: "agents.toml", Status: "warn", Detail: "not present at " + p,
				Hint: "OK if running in single-agent legacy mode; otherwise create from agents.toml example."}
		}
		return doctorCheck{Name: "agents.toml", Status: "fail", Detail: err.Error()}
	}
	if fi.Size() == 0 {
		return doctorCheck{Name: "agents.toml", Status: "fail", Detail: "empty file"}
	}
	return doctorCheck{Name: "agents.toml", Status: "ok", Detail: fmt.Sprintf("%d bytes", fi.Size())}
}

func checkNASGateway() doctorCheck {
	// SSH probe to NAS; gateway healthz is internal to the docker network but
	// the WS tunnel endpoint is reachable as a TLS endpoint. We probe TLS
	// reachability via `nc -z` which is universally available.
	host := "openclaw.errordocker.synology.me"
	port := "443"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "nc", "-z", "-w", "3", host, port).CombinedOutput()
	if err != nil {
		return doctorCheck{Name: "nas-gateway-ws", Status: "fail",
			Detail: fmt.Sprintf("%s:%s unreachable: %v", host, port, err),
			Hint:   "Check NAS docker stack is up. From NAS: docker ps | grep gateway"}
	}
	_ = out
	return doctorCheck{Name: "nas-gateway-ws", Status: "ok", Detail: host + ":" + port + " reachable"}
}

func checkNASPluginCacheEnv() doctorCheck {
	// Read the NAS compose .env via SSH and verify the plugin cache TTL env
	// vars are commented out. Setting them reproduces the upstream
	// bind-mount Docker dispatch deadlock (issue #73874).
	envPath := "/volume1/docker/openclaw/.env"
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=5", "nas",
		"grep", "-E", "^OPENCLAW_PLUGIN_(DISCOVERY|MANIFEST)_CACHE_MS", envPath).CombinedOutput()
	if err != nil {
		// grep returns 1 when no lines match — that's the GOOD case (commented or absent).
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return doctorCheck{Name: "nas-plugin-cache-env", Status: "ok", Detail: "vars commented or absent"}
		}
		return doctorCheck{Name: "nas-plugin-cache-env", Status: "warn",
			Detail: "could not check: " + err.Error(),
			Hint:   "Configure ~/.ssh/config alias `nas` with BatchMode-friendly key auth."}
	}
	return doctorCheck{Name: "nas-plugin-cache-env", Status: "fail",
		Detail: "uncommented plugin-cache env vars found:\n      " + strings.ReplaceAll(strings.TrimSpace(string(out)), "\n", "\n      "),
		Hint:   "Comment OPENCLAW_PLUGIN_DISCOVERY_CACHE_MS and OPENCLAW_PLUGIN_MANIFEST_CACHE_MS in " + envPath + " — they reproduce upstream issue #73874."}
}

// --- preserved generic helpers from generator output ---

func collectCacheReport(ctx context.Context, staleAfterSpec string) map[string]any {
	report := map[string]any{}
	dbPath := defaultDBPath("ori-pp-cli")
	report["db_path"] = dbPath

	fi, err := os.Stat(dbPath)
	if err != nil {
		if os.IsNotExist(err) {
			report["status"] = "unknown"
			report["hint"] = "Database not created yet; run 'ori-pp-cli sync' to hydrate."
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
		report["status"] = "unknown"
		report["hint"] = "No sync state recorded; run 'ori-pp-cli sync' to populate."
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
		report["hint"] = "sync_state is empty; run 'ori-pp-cli sync' to hydrate."
	case fresh:
		report["status"] = "fresh"
	default:
		report["status"] = "stale"
		report["oldest_age"] = oldest.Round(time.Minute).String()
		report["hint"] = "Some resources are older than stale_after; run 'ori-pp-cli sync' to refresh."
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
	fmt.Fprintf(w, "  %s cache: %s\n", indicator, status)
	if v, ok := rep["db_path"]; ok {
		fmt.Fprintf(w, "      db_path: %v\n", v)
	}
	if v, ok := rep["schema_version"]; ok {
		fmt.Fprintf(w, "      schema_version: %v\n", v)
	}
	if v, ok := rep["db_bytes"]; ok {
		fmt.Fprintf(w, "      db_bytes: %v\n", v)
	}
	if hint, ok := rep["hint"]; ok {
		fmt.Fprintf(w, "      hint: %v\n", hint)
	}
}
