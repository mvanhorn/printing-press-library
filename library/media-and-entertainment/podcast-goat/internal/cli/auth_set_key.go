// Copyright 2026 mvanhorn. Licensed under Apache-2.0. See LICENSE.
// auth set-key — persist a paid-provider API key into config.toml so users
// don't need to re-export env vars across every shell session.

package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/podcast-goat/internal/config"
)

func newAuthSetKeyCmd(flags *rootFlags) *cobra.Command {
	var (
		flagProvider string
		flagValue    string
		flagFromEnv  string
		flagUnset    bool
		flagStdin    bool
	)
	cmd := &cobra.Command{
		Use:   "set-key",
		Short: "Persist a paid-provider API key to config so future invocations skip the env-var dance",
		Long: `Save a paid-provider API key (spoken, taddy, openai, deepgram, elevenlabs) to
the local config file. After this runs, adapters consult env first then this
config — so a one-time set-key removes the need to export the env var in every
shell.

Value source (pick one):
  --value <key>            paste the key directly
  --from-env <ENV_NAME>    read from an env var (preserves the env var; does not clear it)
  --stdin                  read from stdin (good for piping: pbpaste | ... set-key --provider X --stdin)

Use --unset to clear a previously-saved key.`,
		Example: `  podcast-goat-pp-cli auth set-key --provider spoken --value pt_abc123...
  podcast-goat-pp-cli auth set-key --provider spoken --from-env SPOKEN_API_KEY
  pbpaste | podcast-goat-pp-cli auth set-key --provider spoken --stdin
  podcast-goat-pp-cli auth set-key --provider taddy --unset`,
		Annotations: map[string]string{"mcp:read-only": "false"}, // writes to disk
		RunE: func(cmd *cobra.Command, _ []string) error {
			if flagProvider == "" {
				return fmt.Errorf("--provider is required (valid: %s)", strings.Join(config.KnownProviders, ", "))
			}
			if config.EnvVarFor(flagProvider) == "" {
				return fmt.Errorf("unknown provider %q (valid: %s)", flagProvider, strings.Join(config.KnownProviders, ", "))
			}

			var value string
			modeCount := 0
			if flagUnset {
				modeCount++
			}
			if flagValue != "" {
				value = flagValue
				modeCount++
			}
			if flagFromEnv != "" {
				v := os.Getenv(flagFromEnv)
				if v == "" {
					return fmt.Errorf("--from-env %s is not set in this shell", flagFromEnv)
				}
				value = v
				modeCount++
			}
			if flagStdin {
				reader := bufio.NewReader(os.Stdin)
				line, err := reader.ReadString('\n')
				if err != nil && line == "" {
					return fmt.Errorf("--stdin: read empty input")
				}
				value = strings.TrimSpace(line)
				if value == "" {
					return fmt.Errorf("--stdin: input was empty after trim")
				}
				modeCount++
			}
			if modeCount == 0 {
				return fmt.Errorf("specify exactly one of --value, --from-env, --stdin, or --unset")
			}
			if modeCount > 1 {
				return fmt.Errorf("specify exactly one of --value, --from-env, --stdin, --unset (got %d)", modeCount)
			}

			path, err := config.SetKey(flagProvider, value)
			if err != nil {
				return err
			}
			w := cmd.OutOrStdout()
			if flagUnset {
				fmt.Fprintf(w, "cleared %s key in %s\n", flagProvider, path)
			} else {
				envName := config.EnvVarFor(flagProvider)
				fmt.Fprintf(w, "saved %s key (%d chars) to %s\n", flagProvider, len(value), path)
				fmt.Fprintf(w, "future invocations will use this when %s is not set in the environment.\n", envName)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagProvider, "provider", "", "Provider slug: spoken|taddy|taddy_user_id|openai|deepgram|elevenlabs")
	cmd.Flags().StringVar(&flagValue, "value", "", "Key value (use --stdin or --from-env for safer piping)")
	cmd.Flags().StringVar(&flagFromEnv, "from-env", "", "Read key from this env var (env var stays set; not cleared)")
	cmd.Flags().BoolVar(&flagStdin, "stdin", false, "Read key from stdin (one line)")
	cmd.Flags().BoolVar(&flagUnset, "unset", false, "Clear the key for this provider")
	return cmd
}
