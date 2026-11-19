// Copyright 2026 Vinny Pasceri and contributors. Licensed under Apache-2.0. See LICENSE.

// Hand-authored: `webhook` command group — manage delivery targets for watch
// alerts. add/list/remove/ping. URLs are SSRF-validated; secrets are never
// printed. Mutating subcommands are not exposed on the MCP surface.

package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/readwise/internal/store"
)

func newWebhookCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use: "webhook",
		Example: "  readwise-pp-cli webhook list --json\n" +
			"  readwise-pp-cli webhook add https://example.com/hooks/readwise",
		Short: "Manage webhooks that receive new-item alerts from 'watch run'",
		// off the agent MCP surface; exposes/mutates operator webhook config (URLs embed delivery tokens)
		// webhook group itself only prints help: read-only so live dogfood runs its first (list) Example for real
		Annotations: map[string]string{"mcp:hidden": "true", "mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newWebhookAddCmd(flags))
	cmd.AddCommand(newWebhookListCmd(flags))
	cmd.AddCommand(newWebhookRemoveCmd(flags))
	cmd.AddCommand(newWebhookPingCmd(flags))
	return cmd
}

func newWebhookAddCmd(flags *rootFlags) *cobra.Command {
	var dbPath, secret, events string
	var includeText, secretStdin bool
	cmd := &cobra.Command{
		Use:     "add <url>",
		Short:   "Register an https webhook URL for watch alerts",
		Long:    "Register an https webhook URL. Every payload sends kind, watch, count, generated_at and item id, title, author, url. These fields disclose reading history. Highlight text / document summary is sent only with --include-text and recipient consent.",
		Example: "  readwise-pp-cli webhook add https://example.com/hooks/readwise",
		// Stores a delivery URL + signing secret — off MCP surface
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "webhook add")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("webhook add requires a <url>"))
			}
			if err := validWebhookURL(args[0]); err != nil {
				return usageErr(err)
			}
			reader := bufio.NewReader(io.LimitReader(cmd.InOrStdin(), 64*1024+1))
			if secretStdin {
				if cmd.Flags().Changed("secret") {
					return usageErr(fmt.Errorf("choose --secret or --secret-stdin"))
				}
				line, err := reader.ReadString('\n')
				if err != nil && err != io.EOF {
					return fmt.Errorf("reading signing secret failed")
				}
				if len(line) > 64*1024 {
					return usageErr(fmt.Errorf("signing secret exceeds 64 KiB"))
				}
				secret = strings.TrimSpace(line)
				if secret == "" {
					return usageErr(fmt.Errorf("signing secret is empty"))
				}
			}
			// Disclose reading-history metadata as well as optional text before consent.
			if !flags.yes {
				if flags.noInput {
					return usageErr(fmt.Errorf("webhook registration requires consent to reading-history fields; pass --yes"))
				}
				if includeText {
					fmt.Fprintln(cmd.ErrOrStderr(), "This endpoint WILL receive highlight text / document summaries.")
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "This endpoint receives kind, watch, count, generated_at and item id, title, author, url (reading history); text / summary only with --include-text. Confirm [y/N]: ")
				line, _ := reader.ReadString('\n')
				line = strings.TrimSpace(strings.ToLower(line))
				if line != "y" && line != "yes" {
					fmt.Fprintln(cmd.ErrOrStderr(), "aborted")
					return usageErr(fmt.Errorf("webhook registration requires confirmation (pass --yes to skip)"))
				}
			}
			db, err := openWatchStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			id, err := db.AddWebhook(cmd.Context(), args[0], secret, events, includeText)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "added webhook #%d\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	cmd.Flags().BoolVar(&secretStdin, "secret-stdin", false, "Read signing secret from the first stdin line (avoids argv exposure)")
	cmd.Flags().StringVar(&secret, "secret", "", "Optional HMAC-SHA256 signing secret (sent as X-Readwise-Signature)")
	cmd.Flags().StringVar(&events, "events", "*", "Event types to deliver (csv of docs,highlights, or * for all)")
	cmd.Flags().BoolVar(&includeText, "include-text", false, "Include highlight text / doc summary in payloads (requires confirmation)")
	return cmd
}

func newWebhookListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	// pp:data-source local
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List configured webhooks (secrets are never shown)",
		Example:     "  readwise-pp-cli webhook list --json",
		Annotations: map[string]string{"mcp:hidden": "true", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "webhook list")
			}
			db, err := openWatchStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			hooks, err := db.ListWebhooks(cmd.Context())
			if err != nil {
				return err
			}
			if hooks == nil {
				hooks = []store.Webhook{}
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), hooks, flags)
			}
			if len(hooks) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no webhooks configured")
				return nil
			}
			for _, h := range hooks {
				signed := "unsigned"
				if h.Signed {
					signed = "signed"
				}
				textFlag := ""
				if h.IncludeText {
					textFlag = " [text: on]"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "#%d\t%s\tevents=%s\t%s%s\n",
					h.ID, redactWebhookURL(h.URL), h.Events, signed, textFlag)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newWebhookRemoveCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use: "remove <id>",
		Example: strings.Trim(`
  readwise-pp-cli webhook remove 1 --dry-run --json
  readwise-pp-cli webhook remove 1
`, "\n"),
		Aliases:     []string{"rm"},
		Short:       "Hard-delete a webhook by id",
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "webhook remove")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("webhook remove requires an <id>"))
			}
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("invalid webhook id %q", args[0]))
			}
			db, err := openWatchStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			n, err := db.RemoveWebhook(cmd.Context(), id)
			if err != nil {
				return err
			}
			if n == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "no webhook #%d\n", id)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "removed webhook #%d\n", id)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func newWebhookPingCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	cmd := &cobra.Command{
		Use:         "ping <id>",
		Short:       "Send a test payload to a webhook to confirm delivery",
		Example:     "  readwise-pp-cli webhook ping 1",
		Annotations: map[string]string{"mcp:hidden": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "webhook ping")
			}
			if len(args) < 1 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("webhook ping requires an <id>"))
			}
			id, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("invalid webhook id %q", args[0]))
			}
			db, err := openWatchStore(cmd.Context(), dbPath)
			if err != nil {
				return err
			}
			defer db.Close()
			hook, err := db.GetWebhook(cmd.Context(), id)
			if err != nil {
				return notFoundErr(err)
			}
			// Build a test ping payload
			pingItems := []watchDocItem{{
				ID:    "ping-0",
				Title: "readwise-pp-cli test ping",
				URL:   "https://readwise.io",
			}}
			p := watchPayload{
				Kind:        "ping",
				Watch:       "ping",
				Count:       1,
				GeneratedAt: time.Now().UTC().Format(time.RFC3339),
				Items:       pingItems,
			}
			body, err := json.Marshal(p)
			if err != nil {
				return fmt.Errorf("build ping payload: %w", err)
			}
			// Use postWebhook with the hardened safeWebhookClient (not http.DefaultClient)
			if err := postWebhook(cmd.Context(), hook.URL, hook.Secret, body); err != nil {
				return fmt.Errorf("ping webhook #%d: %v", id, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "ping sent to webhook #%d (%s)\n", id, redactWebhookURL(hook.URL))
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}
