package cli

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/wordpress/internal/config"
)

// wordpressDBPath returns the local SQLite path for the active site. The mirror
// MUST be keyed per site: syncing site B over site A's store silently corrupts
// data because both write the same generic resource_type rows.
func wordpressDBPath(flags *rootFlags) string {
	if explicit := explicitWordPressDBPath(os.Args[1:]); explicit != "" {
		return explicit
	}
	configPath := ""
	if flags != nil {
		configPath = flags.configPath
	}
	// The generated `sync` command defaults to the framework data directory
	// (defaultDBPath), not a per-site subdirectory, and its default cannot be
	// changed without editing generated code. If novel commands read only the
	// per-site path, a plain `sync` writes one file and every read command
	// looks at another — the mirror appears permanently empty.
	//
	// Resolution order keeps both models coherent:
	//   1. A per-site mirror that actually exists wins, so operators who
	//      isolate sites (via `--db` or a per-site `--home`) get true
	//      per-site separation and never cross-contaminate stores.
	//   2. Otherwise fall back to the framework default, which is what a
	//      plain `sync` just wrote.
	registry, err := config.LoadSites(configPath)
	if err == nil {
		if site, ok := registry.ActiveSite(); ok {
			name := sanitizeWordPressSiteName(site.Name)
			if name != "" {
				perSite := wordpressSiteDBPath(name)
				if _, statErr := os.Stat(perSite); statErr == nil {
					return perSite
				}
			}
		}
	}
	return defaultDBPath("wordpress-pp-cli")
}

func wordpressSiteDBPath(siteName string) string {
	siteName = sanitizeWordPressSiteName(siteName)
	if siteName == "" {
		return defaultDBPath("wordpress-pp-cli")
	}
	requested := defaultDBPath("wordpress-pp-cli-" + siteName)
	generic := defaultDBPath("wordpress-pp-cli")
	if requested != generic {
		return requested
	}
	// This generated defaultDBPath currently resolves the app's data directory
	// before consulting its name argument. Preserve the required per-site
	// isolation even on that implementation.
	return filepath.Join(filepath.Dir(generic), "wordpress-pp-cli-"+siteName, filepath.Base(generic))
}

func explicitWordPressDBPath(args []string) string {
	result := ""
	for index := 0; index < len(args); index++ {
		argument := args[index]
		if strings.HasPrefix(argument, "--db=") {
			if value := strings.TrimSpace(strings.TrimPrefix(argument, "--db=")); value != "" {
				result = value
			}
			continue
		}
		if argument == "--db" && index+1 < len(args) {
			index++
			if value := strings.TrimSpace(args[index]); value != "" {
				result = value
			}
		}
	}
	return result
}
