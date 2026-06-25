// Copyright 2026 adbonnet and contributors. Licensed under Apache-2.0. See LICENSE.
//
// Hand-authored command (NOT generator-emitted). Mirrors newAuthSetTokenCmd for
// the PodcastIndex shared *secret* — the other half of the sha1(key+secret+date)
// signer, which auth set-token does not persist. Lives in its own file so it
// survives generator regen. Registered from auth.go.
// See .printing-press-patches/find-appearances-and-auth-config.json.

package cli

import (
	"fmt"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcastindex/internal/config"
	"github.com/spf13/cobra"
)

func newAuthSetSecretCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "set-secret <secret>",
		Short: "Save the PodcastIndex shared secret to the config file",
		Long: `PodcastIndex signs every request with sha1(key + secret + date), so both a
key and a secret are required. 'auth set-token' persists the key; this persists
the secret (to client_secret in the config file) so non-interactive / --agent
runs authenticate without exporting PODCASTINDEX_SECRET in every shell. The
PODCASTINDEX_SECRET environment variable still overrides the stored value.`,
		Example: "  podcastindex-pp-cli auth set-secret YOUR_SECRET_HERE",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}
			if err := cfg.SaveClientSecret(args[0]); err != nil {
				return configErr(fmt.Errorf("saving secret: %w", err))
			}

			// JSON envelope mirrors set-token: {saved, config_path}.
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"saved":       true,
					"config_path": cfg.Path,
				}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Secret saved to %s\n", cfg.Path)
			return nil
		},
	}
}
