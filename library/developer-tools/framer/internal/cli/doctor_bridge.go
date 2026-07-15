package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/framer/internal/client"

	"github.com/spf13/cobra"
)

func newDoctorBridgeCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Test the live Framer Server API connection via the Node.js bridge",
		Example: `  framer-pp-cli doctor
  framer-pp-cli doctor --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			type check struct {
				Name   string `json:"name"`
				Status string `json:"status"`
				Value  string `json:"value,omitempty"`
				Fix    string `json:"fix,omitempty"`
			}

			var checks []check

			// 1. node binary on PATH
			nodePath, err := exec.LookPath("node")
			if err != nil {
				checks = append(checks, check{
					Name:   "node",
					Status: "FAIL",
					Fix:    "Install Node.js 18+ from https://nodejs.org",
				})
			} else {
				checks = append(checks, check{
					Name:   "node",
					Status: "PASS",
					Value:  nodePath,
				})
			}

			// 2. bridge/framer-bridge.mjs found
			bridgePath := findBridgePath()
			if bridgePath == "" {
				checks = append(checks, check{
					Name:   "bridge",
					Status: "FAIL",
					Fix:    "Ensure bridge/framer-bridge.mjs exists alongside the CLI binary or in ~/printing-press/library/framer/bridge/",
				})
			} else {
				checks = append(checks, check{
					Name:   "bridge",
					Status: "PASS",
					Value:  bridgePath,
				})
			}

			// 3. FRAMER_PROJECT_URL set
			projectURL := os.Getenv("FRAMER_PROJECT_URL")
			if projectURL == "" {
				checks = append(checks, check{
					Name:   "FRAMER_PROJECT_URL",
					Status: "FAIL",
					Fix:    "export FRAMER_PROJECT_URL=<your project URL from the Framer editor address bar>",
				})
			} else {
				checks = append(checks, check{
					Name:   "FRAMER_PROJECT_URL",
					Status: "PASS",
					Value:  projectURL,
				})
			}

			// 4. FRAMER_API_KEY set
			apiKey := os.Getenv("FRAMER_API_KEY")
			if apiKey == "" {
				checks = append(checks, check{
					Name:   "FRAMER_API_KEY",
					Status: "FAIL",
					Fix:    "export FRAMER_API_KEY=<key> — generate one in Framer > Site Settings > General",
				})
			} else {
				masked := "****"
				if len(apiKey) >= 4 {
					masked = "****" + apiKey[len(apiKey)-4:]
				}
				checks = append(checks, check{
					Name:   "FRAMER_API_KEY",
					Status: "PASS",
					Value:  masked,
				})
			}

			// 5. framer-api npm package installed
			npmOK := false
			if bridgePath != "" {
				bridgeDir := filepath.Dir(bridgePath)
				pkgPath := filepath.Join(bridgeDir, "node_modules", "framer-api")
				if _, err := os.Stat(pkgPath); err == nil {
					npmOK = true
				}
			}
			if npmOK {
				checks = append(checks, check{
					Name:   "framer-api",
					Status: "PASS",
				})
			} else {
				checks = append(checks, check{
					Name:   "framer-api",
					Status: "FAIL",
					Fix:    "cd bridge && npm install",
				})
			}

			// 6. API connection test
			bc, bcErr := client.NewBridgeClient()
			if bcErr != nil {
				checks = append(checks, check{
					Name:   "api-connection",
					Status: "FAIL",
					Value:  bcErr.Error(),
					Fix:    "Resolve the above issues first, then retry",
				})
			} else {
				raw, callErr := bc.Call("project-info")
				if callErr != nil {
					checks = append(checks, check{
						Name:   "api-connection",
						Status: "FAIL",
						Value:  callErr.Error(),
						Fix:    "Check that FRAMER_PROJECT_URL and FRAMER_API_KEY are correct and the project is accessible",
					})
				} else {
					var info struct {
						Name string `json:"name"`
						ID   string `json:"id"`
					}
					_ = json.Unmarshal(raw, &info)
					checks = append(checks, check{
						Name:   "api-connection",
						Status: "PASS",
						Value:  info.Name,
					})
				}
			}

			// Output
			if flags.asJSON {
				return flags.printJSON(cmd, checks)
			}

			w := cmd.OutOrStdout()
			for _, c := range checks {
				indicator := green("PASS")
				if c.Status == "FAIL" {
					indicator = red("FAIL")
				}
				line := fmt.Sprintf("  %s %s", indicator, c.Name)
				if c.Value != "" {
					line += ": " + c.Value
				}
				fmt.Fprintln(w, line)
				if c.Fix != "" {
					fmt.Fprintf(w, "       fix: %s\n", c.Fix)
				}
			}
			return nil
		},
	}
	return cmd
}

// findBridgePath locates framer-bridge.mjs using the same search logic as
// the bridge client but without failing on missing env vars.
func findBridgePath() string {
	execPath, err := os.Executable()
	if err == nil {
		execPath, _ = filepath.EvalSymlinks(execPath)
		dir := filepath.Dir(execPath)
		candidate := filepath.Join(dir, "bridge", "framer-bridge.mjs")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
		candidate = filepath.Join(dir, "..", "bridge", "framer-bridge.mjs")
		if abs, err := filepath.Abs(candidate); err == nil {
			if _, err := os.Stat(abs); err == nil {
				return abs
			}
		}
	}

	cwd, _ := os.Getwd()
	candidate := filepath.Join(cwd, "bridge", "framer-bridge.mjs")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}

	home, _ := os.UserHomeDir()
	candidate = filepath.Join(home, "printing-press", "library", "framer", "bridge", "framer-bridge.mjs")
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}

	return ""
}
