// Hand-authored — NOT generated. Implements the `auth ping` novel feature.
// Single fast GET to /Public/SHIPS/Default.aspx; reuses the parser's
// login-wall detection. Exits 0 if session is live, non-zero with a clear
// reason if re-login is needed. Designed for cron/launchd integration to
// detect ASP.NET session timeout (the brief's defining auth pain).
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/spf13/cobra"
)

type authPingResult struct {
	Status          string `json:"status"`
	Reason          string `json:"reason,omitempty"`
	AuthenticatedAs string `json:"authenticated_as,omitempty"`
	CheckedAt       string `json:"checked_at"`
	URL             string `json:"url"`
}

// attachAuthPing wires the `ping` novel subcommand under the auth parent
// command. The generator created the stub at auth_ping.go but didn't add it
// to newAuthCmd's subcommand list — this function bridges that gap and also
// replaces the stub's "TODO" RunE with the real implementation below.
// Called from internal/cli/auth.go via a one-line edit. Survives regen.
func attachAuthPing(auth *cobra.Command, flags *rootFlags) {
	ping := newNovelAuthPingCmd(flags)
	ping.RunE = runAuthPing(flags)
	auth.AddCommand(ping)
}

func runAuthPing(flags *rootFlags) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		const probePath = "/SHIPS/Default.aspx"
		const probeURL = "https://gisis.imo.org/Public" + probePath

		if dryRunOK(flags) {
			fmt.Fprintf(cmd.OutOrStdout(), "GET %s\n(dry run - no request sent)\n", probeURL)
			return nil
		}

		c, err := flags.newClient()
		if err != nil {
			return err
		}

		body, err := c.Get(cmd.Context(), probePath, nil)
		if err != nil {
			result := authPingResult{
				Status:    "error",
				Reason:    err.Error(),
				CheckedAt: time.Now().UTC().Format(time.RFC3339),
				URL:       probeURL,
			}
			_ = writePingJSON(cmd, result, flags)
			return &cliError{code: 5, err: fmt.Errorf("auth ping: transport error: %w", err)}
		}

		doc, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("auth ping: parsing response: %w", err)
		}

		if isLoginWall(doc) {
			result := authPingResult{
				Status:    "expired",
				Reason:    "GISIS returned the login form — session is dead. Re-run press-auth login (or refresh your Brave session, then re-extract cookies).",
				CheckedAt: time.Now().UTC().Format(time.RFC3339),
				URL:       probeURL,
			}
			_ = writePingJSON(cmd, result, flags)
			return &cliError{code: 4, err: fmt.Errorf("session expired — re-login required")}
		}

		result := authPingResult{
			Status:    "ok",
			CheckedAt: time.Now().UTC().Format(time.RFC3339),
			URL:       probeURL,
		}
		// Extract the logged-in user's name (visible in the GISIS header).
		if name := doc.Find("div.imo-theme-login.userdetails .imo-theme-user-name span").First().Text(); name != "" {
			result.AuthenticatedAs = name
		}
		return writePingJSON(cmd, result, flags)
	}
}

func writePingJSON(cmd *cobra.Command, result authPingResult, flags *rootFlags) error {
	// Emit JSON by default for cron/launchd consumers. Pretty-print to stderr-style
	// when stdout is a TTY by going through the press's printJSONFiltered helper.
	if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
		return printJSONFiltered(cmd.OutOrStdout(), result, flags)
	}
	// Human-friendly one-line for TTY: status + reason
	out := result.Status
	if result.AuthenticatedAs != "" {
		out = fmt.Sprintf("%s (authenticated as %s)", out, result.AuthenticatedAs)
	}
	if result.Reason != "" {
		out = fmt.Sprintf("%s: %s", out, result.Reason)
	}
	fmt.Fprintln(cmd.OutOrStdout(), out)
	// Also write the structured form to stderr for downstream consumers.
	if pj, err := json.Marshal(result); err == nil {
		fmt.Fprintln(cmd.ErrOrStderr(), string(pj))
	}
	return nil
}
