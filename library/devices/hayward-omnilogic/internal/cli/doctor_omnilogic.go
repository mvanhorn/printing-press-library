// doctorOmnilogicAuth is the OmniLogic-specific auth probe that the generator's
// generic doctor calls. It checks HAYWARD_USER + HAYWARD_PW presence and reports
// the cached-token state without performing a live login (which would slow doctor
// down and pollute the token cache on a re-check).

package cli

import (
	"fmt"
	"os"
	"time"

	"github.com/mvanhorn/printing-press-library/library/devices/hayward-omnilogic/internal/store"
)

func doctorOmnilogicAuth(report map[string]any, flags *rootFlags) {
	envU := os.Getenv(envUser)
	envP := os.Getenv(envPW)
	switch {
	case envU == "" && envP == "":
		report["auth"] = fmt.Sprintf("missing: set %s and %s in your environment", envUser, envPW)
	case envU == "":
		report["auth"] = fmt.Sprintf("missing: %s is unset", envUser)
	case envP == "":
		report["auth"] = fmt.Sprintf("missing: %s is unset", envPW)
	default:
		report["auth"] = "env vars present"
	}
	report["env_vars"] = map[string]any{
		envUser: envU != "",
		envPW:   envP != "",
	}
	c := newOmnilogicClient(flags.timeout)
	st := c.AuthState()
	report["auth_cache_path"] = c.AuthCachePath()
	if st == nil {
		report["token_cache"] = "empty (next command will log in)"
	} else if st.Valid() {
		report["token_cache"] = fmt.Sprintf("cached (expires %s)", st.ExpiresAt.Format(time.RFC3339))
	} else {
		report["token_cache"] = "expired (next command will refresh or re-login)"
	}
	// Probe store reachability so the user knows whether transcendence
	// commands will have history to work with.
	s, err := store.Open("")
	if err != nil {
		report["store"] = fmt.Sprintf("error: %s", err)
	} else {
		var siteCount, telemetryCount, commandCount int
		_ = s.DB.QueryRow(`SELECT COUNT(*) FROM sites`).Scan(&siteCount)
		_ = s.DB.QueryRow(`SELECT COUNT(*) FROM telemetry_samples`).Scan(&telemetryCount)
		_ = s.DB.QueryRow(`SELECT COUNT(*) FROM command_log`).Scan(&commandCount)
		report["store"] = map[string]any{
			"path":              s.Path,
			"sites":             siteCount,
			"telemetry_samples": telemetryCount,
			"command_log_rows":  commandCount,
		}
		_ = s.Close()
	}
}
