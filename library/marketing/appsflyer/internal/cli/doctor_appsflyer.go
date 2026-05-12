package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mvanhorn/printing-press-library/library/marketing/appsflyer/internal/budget"
	"github.com/mvanhorn/printing-press-library/library/marketing/appsflyer/internal/client"
)

// reportAppsFlyerExtras enriches the doctor report with AppsFlyer-specific
// health signals not handled by the generator: dotenv file presence, the
// daily call-budget tracker state, and (opt-in) per-family permission
// probes. Each entry uses the same key conventions as the generated report
// so the renderer and JSON emitter pick them up consistently.
func reportAppsFlyerExtras(report map[string]any, c *client.Client, probeFamilies bool) {
	addDotenvStatus(report)
	addBudgetStatus(report, c)
	if probeFamilies && c != nil {
		addPermissionProbes(report, c)
	} else if c != nil {
		report["permission_probes"] = "skipped (pass --probe-families to consume one call per family)"
	}
}

func addDotenvStatus(report map[string]any) {
	dir := os.Getenv("APPSFLYER_CONFIG_DIR")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			report["dotenv"] = "unknown (no home dir)"
			return
		}
		dir = filepath.Join(home, ".config", "appsflyer-pp-cli")
	}
	envPath := filepath.Join(dir, ".env")
	if _, err := os.Stat(envPath); err == nil {
		report["dotenv"] = "loaded"
		report["dotenv_path"] = envPath
		return
	}
	report["dotenv"] = "not found"
	report["dotenv_path"] = envPath
	report["dotenv_hint"] = "create with: echo 'APPSFLYER_API_TOKEN=your-token-here' > " + envPath
}

func addBudgetStatus(report map[string]any, c *client.Client) {
	if c == nil {
		return
	}
	t := c.Budget()
	if t == nil {
		t2, err := budget.New("", 0)
		if err != nil {
			return
		}
		t = t2
	}
	s := t.Snapshot()
	report["budget"] = map[string]any{
		"used":      s.Used,
		"remaining": t.Remaining(),
		"limit":     t.Limit(),
		"date":      s.Date,
	}
}

// permissionProbe entries describe a single per-family probe. Each consumes
// one API call from the daily budget.
var permissionProbeTargets = []struct {
	family string
	path   string
	params map[string]string
}{
	{family: "apps", path: "/api/mng/apps", params: nil},
}

func addPermissionProbes(report map[string]any, c *client.Client) {
	results := make(map[string]string, len(permissionProbeTargets))
	for _, probe := range permissionProbeTargets {
		_, err := c.Get(probe.path, probe.params)
		switch e := err.(type) {
		case nil:
			results[probe.family] = "200 OK"
		case *client.APIError:
			switch e.StatusCode {
			case 401:
				results[probe.family] = "401 unauthorized (token invalid)"
			case 403:
				results[probe.family] = "403 forbidden (subscription does not entitle this family)"
			case 404:
				results[probe.family] = "404 not found (path drift — file a retro)"
			default:
				results[probe.family] = fmt.Sprintf("%d", e.StatusCode)
			}
		default:
			results[probe.family] = fmt.Sprintf("error: %v", err)
		}
	}
	report["permission_probes"] = results
}
