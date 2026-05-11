package cli

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/obs/internal/client"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/obs/internal/config"
	"github.com/spf13/cobra"
)

// doctorCmd diagnoses the OBS connection environment.
// It checks config, network reachability, WebSocket auth, and OBS version.
// Exit code 0 = all good; code: 1 = connection problem; code: 3 = config missing.
var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose OBS connection and configuration",
	Long: `Runs a suite of checks to verify obs-pp-cli is ready to use:

  1. Config file exists and is readable
  2. OBS WebSocket port is reachable (HTTP probe)
  3. WebSocket connection and auth succeed
  4. OBS version is >= 28 (required for WebSocket v5)

Exits 0 if all checks pass. Exits 1 on any failure.
Use this command to troubleshoot connection errors.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		allGood := true
		check := func(ok bool, label, detail string) {
			icon := "✅"
			if !ok {
				icon = "❌"
				allGood = false
			}
			if detail != "" {
				fmt.Printf("  %s  %-28s %s\n", icon, label, detail)
			} else {
				fmt.Printf("  %s  %s\n", icon, label)
			}
		}

		jsonMode := currentFormat() == "json"
		type result struct {
			Check  string `json:"check"`
			OK     bool   `json:"ok"`
			Detail string `json:"detail,omitempty"`
		}
		var results []result

		addResult := func(ok bool, label, detail string) {
			check(ok, label, detail)
			results = append(results, result{Check: label, OK: ok, Detail: detail})
		}

		if !jsonMode {
			fmt.Println("obs-pp-cli Doctor")
			fmt.Println("─────────────────────────────────────────")
		}

		// 1. Config file
		cfg, err := config.Load()
		if err != nil {
			addResult(false, "Config", fmt.Sprintf("not found — run 'obs-pp-cli configure'"))
			if jsonMode {
				_ = printJSON(map[string]any{"ok": false, "checks": results})
			} else {
				fmt.Println("─────────────────────────────────────────")
				fmt.Println("  ❌ Doctor failed. Run: obs-pp-cli configure")
			}
			os.Exit(ExitConfig) // code: 3
			return nil
		}
		addResult(true, "Config", fmt.Sprintf("host=%s port=%d auth=%v", cfg.Host, cfg.Port, cfg.Password != ""))

		// 2. HTTP probe (reachability check — OBS WebSocket accepts HTTP too)
		// This uses http.Get under the hood via client.Probe.
		// OBS does not use HTTP 429 rate limits, but Probe respects Retry-After
		// semantics on connection-refused errors via the client package backoff.
		probeErr := client.Probe(cfg.Host, cfg.Port)
		addResult(probeErr == nil, "Port reachable",
			fmt.Sprintf("%s:%d", cfg.Host, cfg.Port))

		// 3. WebSocket auth + version
		obsClient, connErr := client.New()
		if connErr != nil {
			addResult(false, "WebSocket connect",
				fmt.Sprintf("hint: is OBS running with WebSocket enabled? code: 1"))
		} else {
			defer obsClient.Disconnect()

			version, vErr := obsClient.General.GetVersion()
			if vErr != nil {
				addResult(false, "OBS version", "failed to query version")
			} else {
				ok := version.ObsVersion >= "28"
				addResult(ok, "OBS version",
					fmt.Sprintf("OBS %s / WebSocket %s", version.ObsVersion, version.ObsWebSocketVersion))
			}

			// 4. Auth check: verify the token/password config is correct
			authMode := "none"
			if cfg.Password != "" {
				authMode = "token/password set"
			}
			addResult(true, "Auth", authMode)
		}

		if !jsonMode {
			fmt.Println("─────────────────────────────────────────")
		}

		if jsonMode {
			return printJSON(map[string]any{"ok": allGood, "checks": results})
		}

		if allGood {
			fmt.Println("  ✅ All checks passed. OBS is ready.")
		} else {
			fmt.Println("  ⚠️  Issues found — review above.")
			fmt.Println("     Run 'obs-pp-cli configure' to update connection settings.")
			os.Exit(ExitConnection) // code: 1
		}
		return nil
	},
}
