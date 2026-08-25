// Copyright 2026 Nate Olson and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

// printgoatConfigKeys are the local download-preference keys `config
// set`/`config get` manage. Deliberately narrow, and deliberately NOT for
// API credentials: THINGIVERSE_TOKEN, CULTS3D_USERNAME, and CULTS3D_API_KEY
// always come from the environment (see internal/client/host_auth.go) and
// are never read from or written to this store.
var printgoatConfigKeys = map[string]bool{
	"download_dir":    true,
	"default_formats": true,
}

func newConfigCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Local download preferences (download_dir, default_formats)",
		Long: `Stores local download preferences in printgoat's SQLite database.

This is NOT for API credentials. THINGIVERSE_TOKEN, CULTS3D_USERNAME, and
CULTS3D_API_KEY always come from the environment and are never read from or
written to this store — see 'doctor' to check credential status.

Known keys:
  download_dir      default directory 'download' writes files to when -o is omitted
  default_formats   comma-separated default value for 'download'/'files' filtering`,
		Example: `  printgoat-pp-cli config set download_dir ~/3d-prints
  printgoat-pp-cli config get download_dir
  printgoat-pp-cli config get --json`,
		RunE: parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newConfigSetCmd(flags))
	cmd.AddCommand(newConfigGetCmd(flags))
	return cmd
}

func validPrintgoatConfigKeys() []string {
	keys := make([]string, 0, len(printgoatConfigKeys))
	for k := range printgoatConfigKeys {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func validatePrintgoatConfigKey(key string) error {
	if !printgoatConfigKeys[key] {
		return fmt.Errorf("unknown config key %q: known keys are %s", key, strings.Join(validPrintgoatConfigKeys(), ", "))
	}
	return nil
}
