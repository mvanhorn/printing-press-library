// Eager migration of the v0.1 ~/.config/tesla-pp-cli/auth.json into the
// config.toml token slots. Fires once from Execute() before cobra parses
// args, so commands that never load tokens (e.g. `--version`, `--help`,
// `agent-context`) still bridge legacy installs.
//
// Hand-coded; not generator-owned. Lives outside the generator's emit set so
// it survives `printing-press generate --force` regens.
//
// Silent-on-no-op contract: the function emits nothing to stdout or stderr
// when the legacy file is absent OR the config already carries tokens. Only
// the actual migration emits the existing one-line stderr breadcrumb that
// migrateLegacyAuthJSON in tesla_auth_storage.go already prints. This
// preserves the stderr-quiet expectation that MCP hosts hold for spawned
// `tesla agent-context` invocations.
package cli

import (
	"fmt"
	"os"

	"github.com/mvanhorn/printing-press-library/library/devices/tesla/internal/config"
)

// eagerMigrateLegacyAuth runs the legacy auth.json -> config.toml migration
// at CLI startup if and only if config has no tokens AND the legacy file is
// present. Best-effort: any error is logged to stderr and swallowed; the
// caller flow never blocks on this.
//
// The function loads a fresh config object using the default config path
// (TESLA_CONFIG env if set, otherwise ~/.config/tesla-pp-cli/config.toml).
// It does NOT honor any per-invocation --config flag, because Execute()
// fires it before cobra parses flags. Users who keep their config at a
// non-default location are not migrating from v0.1 (v0.1 always wrote to
// the default path), so this is the correct trade-off.
func eagerMigrateLegacyAuth() {
	cfg, err := config.Load("")
	if err != nil {
		// Config load failure shouldn't block the CLI from running -- the
		// user might be invoking `tesla --version` on a machine that has
		// never had a config file. Skip the migration and let the normal
		// command flow surface any real config errors.
		return
	}
	if cfg.AccessToken != "" || cfg.RefreshToken != "" {
		// Already bridged, or never had a legacy file. No-op, silent.
		return
	}
	if _, mErr := migrateLegacyAuthJSON(cfg); mErr != nil {
		// migrateLegacyAuthJSON already emits its own stderr warnings for
		// the recoverable cases (malformed legacy file, rename failure).
		// A hard error here means SaveTokens itself failed; surface that
		// once and continue. Don't return the error -- the command the
		// user actually ran might not need tokens, and blocking
		// `tesla --version` because we couldn't write a TOML file is
		// hostile.
		fmt.Fprintf(os.Stderr, "warning: eager auth migration failed: %v\n", mErr)
	}
}
