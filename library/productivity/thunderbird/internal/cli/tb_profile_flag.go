// pp:data-source local

package cli

import (
	"os"
	"strings"
	"sync"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile"
	"github.com/spf13/cobra"
)

// tbProfileSelectors maps a command tree's *rootFlags to its Thunderbird
// profile selector; keyed per tree because the MCP server builds several.
var tbProfileSelectors sync.Map

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		if f := root.PersistentFlags().Lookup("profile"); f != nil {
			f.Usage = "Thunderbird profile: directory path or profiles.ini name (env THUNDERBIRD_PROFILE); a saved run-profile name still applies that run profile"
		}
		orig := root.PersistentPreRunE
		root.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
			if name := strings.TrimSpace(flags.runProfileName); name != "" {
				if saved, err := GetProfile(name); err == nil && saved == nil {
					tbProfileSelectors.Store(flags, name)
					flags.runProfileName = ""
				}
			}
			if orig != nil {
				return orig(cmd, args)
			}
			return nil
		}
	})
}

// tbProfileSelector returns the --profile value (when it is not a saved run
// profile) or THUNDERBIRD_PROFILE.
func tbProfileSelector(flags *rootFlags) string {
	if v, ok := tbProfileSelectors.Load(flags); ok {
		return v.(string)
	}
	return strings.TrimSpace(os.Getenv(tbprofile.EnvProfile))
}

// resolveTBProfile resolves the Thunderbird profile directory. An explicit
// selector that cannot be resolved is an error; no installation at all
// returns tbprofile.ErrNoProfile.
func resolveTBProfile(flags *rootFlags) (string, error) {
	return tbprofile.Resolve(tbProfileSelector(flags), tbprofile.RootDir())
}
