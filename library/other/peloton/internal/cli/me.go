// Copyright 2026 Todd Dailey. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/peloton/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/peloton/internal/config"
)

// meOutput is the on-the-wire shape for `peloton me`. Mirrors the fields
// auth_login harvests; harvested_at is what status-style consumers care
// about for "do I need to re-auth" decisions.
type meOutput struct {
	UserID      string    `json:"user_id"`
	Username    string    `json:"username"`
	HarvestedAt time.Time `json:"harvested_at"`
	TokenAgeSec int       `json:"token_age_seconds"`
}

func newMeCmd(flags *rootFlags) *cobra.Command {
	var refresh bool
	cmd := &cobra.Command{
		Use:   "me",
		Short: "Print the cached identity (user_id, username, token age)",
		Long: `Reads the saved config and prints the user_id + username harvested at
auth-login time, plus how old the bearer token is. With --refresh, re-fetches
/api/me from Peloton, updates the cached values, and prints the fresh result.

Exits 4 if no token is saved.`,
		Example: `  peloton-pp-cli me
  peloton-pp-cli me --json
  peloton-pp-cli me --refresh`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return Errf(CodeAPI, "loading config: %w", err)
			}
			if cfg.Token == "" {
				return Errf(CodeAuth, "no token saved — run `peloton-pp-cli auth login` first")
			}
			if refresh {
				c := client.New(cfg.Token)
				id, username, err := c.Me()
				if err != nil {
					return classify(err)
				}
				cfg.UserID = id
				cfg.Username = username
				if err := cfg.Save(); err != nil {
					return Errf(CodeAPI, "saving config: %w", err)
				}
			}
			out := meOutput{
				UserID:      cfg.UserID,
				Username:    cfg.Username,
				HarvestedAt: cfg.HarvestedAt,
				TokenAgeSec: int(time.Since(cfg.HarvestedAt).Round(time.Second).Seconds()),
			}
			wantJSON := flags.asJSON || flags.compact || !isStdoutTTY()
			if wantJSON {
				enc := json.NewEncoder(cmd.OutOrStdout())
				if !flags.compact {
					enc.SetIndent("", "  ")
				}
				return enc.Encode(out)
			}
			fmt.Fprintf(cmd.OutOrStdout(),
				"user_id=%s username=%s token_age=%s\n",
				out.UserID, out.Username,
				time.Since(cfg.HarvestedAt).Round(time.Second),
			)
			return nil
		},
	}
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Re-fetch /api/me and update the cached identity")
	return cmd
}
