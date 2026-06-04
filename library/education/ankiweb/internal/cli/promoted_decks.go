// Copyright 2026 paul-bockewitz. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"net/http"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/education/ankiweb/internal/svc"
	"github.com/spf13/cobra"
)

const authHint = "no AnkiWeb session cookie configured.\n" +
	"      AnkiWeb has no API key — it authenticates with an HttpOnly session cookie.\n" +
	"      Set it with: export ANKIWEB_COOKIES='ankiweb=<your-session-cookie>'\n" +
	"      (copy the 'ankiweb' cookie from your logged-in browser session, or run 'ankiweb-pp-cli auth login --chrome')."

// authHintEditor guides the user to the ankiuser.net session cookie that the
// editor endpoints (notetypes, notes add) require. AnkiWeb issues a separate
// session per domain, so the ankiweb.net cookie does not work here.
const authHintEditor = "no ankiuser.net session cookie configured.\n" +
	"      The editor (notetypes, notes add) runs on ankiuser.net, which uses a different\n" +
	"      session cookie than ankiweb.net.\n" +
	"      Set it with: export ANKIUSER_COOKIES='ankiweb=<your-ankiuser.net-session-cookie>'\n" +
	"      (open https://ankiuser.net while logged in, then copy that domain's 'ankiweb' cookie)."

// newDecksCmd is the `decks` parent command grouping deck commands
// (currently `decks list`).
func newDecksCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "decks",
		Short: "Your cloud-synced decks (list)",
		Long:  "Commands for your cloud-synced decks. See 'decks list'.",
		RunE:  parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newDecksListCmd(flags))
	return cmd
}

func newDecksListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List your cloud-synced decks (requires an AnkiWeb session cookie)",
		Long:        "List the logged-in user's synced decks via /svc/decks/deck-list-info. Requires an AnkiWeb session cookie (ANKIWEB_COOKIES); returns 403 without one.",
		Example:     "  ankiweb-pp-cli decks list --json",
		Annotations: map[string]string{"pp:endpoint": "decks.list", "pp:method": "POST", "pp:path": "/svc/decks/deck-list-info", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if cliutil.IsVerifyEnv() {
				return printJSONFiltered(cmd.OutOrStdout(), []svc.MyDeck{}, flags)
			}

			c, cfg, err := flags.newSvcClient()
			if err != nil {
				return err
			}
			if strings.TrimSpace(cfg.AnkiwebCookies) == "" {
				return authErr(errAuth())
			}

			data, status, err := c.PostBytes(cmd.Context(), "/svc/decks/deck-list-info", []byte{})
			if err != nil {
				if status == http.StatusForbidden || status == http.StatusUnauthorized {
					return authErr(errAuth())
				}
				return classifyAPIError(err, flags)
			}
			decks, err := svc.DecodeDeckList(data)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			return printJSONFiltered(cmd.OutOrStdout(), decks, flags)
		},
	}
	return cmd
}

func errAuth() error {
	return &authHintError{}
}

// errAuthEditor reports a missing ankiuser.net session cookie for the editor
// commands (notetypes, notes add).
func errAuthEditor() error {
	return &authHintError{editor: true}
}

type authHintError struct{ editor bool }

func (e *authHintError) Error() string {
	if e.editor {
		return authHintEditor
	}
	return authHint
}
