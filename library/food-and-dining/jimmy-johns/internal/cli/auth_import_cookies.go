// Hand-authored — extends the generated auth flow with a `--from-file` adapter
// that consumes a `browser-use cookies export` JSON file.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"jimmy-johns-pp-cli/internal/config"
)

// browserUseCookie is the shape `browser-use cookies export <file>` writes.
// Many fields are passthrough from Playwright; we only need name/value/domain.
type browserUseCookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	Secure   bool   `json:"secure"`
	HttpOnly bool   `json:"httpOnly"`
}

func newAuthImportCookiesCmd(flags *rootFlags) *cobra.Command {
	var fromFile string
	cmd := &cobra.Command{
		Use:   "import-cookies",
		Short: "Import session cookies from a browser-use export",
		Long: `Import session cookies for jimmyjohns.com from a JSON file produced by
'browser-use cookies export'. Filters to jimmyjohns.com cookies, builds a
Cookie header, and saves it to config.Headers so every API request sends it.

Workflow:

  1. Open jimmyjohns.com in real Chrome, solve any PerimeterX challenge,
     browse naturally (search a store, browse menu, view rewards).
  2. Run: browser-use -b real --profile "Default" cookies export ~/jj-cookies.json
  3. Run: jimmy-johns-pp-cli auth import-cookies --from-file ~/jj-cookies.json
  4. Test: jimmy-johns-pp-cli stores list --address 98112 --json

Note: PerimeterX may invalidate the session if it detects automation between
steps 1 and 2. Run the export immediately after natural browsing.`,
		Example: `  jimmy-johns-pp-cli auth import-cookies --from-file ~/jj-cookies.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if fromFile == "" {
				return cmd.Help()
			}
			raw, err := os.ReadFile(fromFile)
			if err != nil {
				return fmt.Errorf("reading %s: %w", fromFile, err)
			}
			var cookies []browserUseCookie
			if err := json.Unmarshal(raw, &cookies); err != nil {
				return fmt.Errorf("parsing %s: %w", fromFile, err)
			}
			var pairs []string
			for _, c := range cookies {
				dom := strings.TrimPrefix(c.Domain, ".")
				if dom != "jimmyjohns.com" && !strings.HasSuffix(dom, ".jimmyjohns.com") {
					continue
				}
				if c.Name == "" {
					continue
				}
				pairs = append(pairs, c.Name+"="+c.Value)
			}
			if len(pairs) == 0 {
				return fmt.Errorf("no jimmyjohns.com cookies found in %s", fromFile)
			}
			cookieHeader := strings.Join(pairs, "; ")

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			if cfg.Headers == nil {
				cfg.Headers = map[string]string{}
			}
			cfg.Headers["Cookie"] = cookieHeader
			// Clear any stale AccessToken — cookie auth doesn't use Bearer.
			cfg.AccessToken = ""
			cfg.RefreshToken = ""
			if err := cfg.SaveHeaders(); err != nil {
				return configErr(fmt.Errorf("saving cookies: %w", err))
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "%s Imported %d jimmyjohns.com cookies (%d bytes)\n", green("OK"), len(pairs), len(cookieHeader))
			fmt.Fprintf(w, "Session saved to %s\n", cfg.Path)
			return nil
		},
	}
	cmd.Flags().StringVar(&fromFile, "from-file", "", "Path to browser-use cookies export JSON (required at runtime)")
	return cmd
}
