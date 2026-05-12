// Hand-authored: chrome navigation subcommands (navigate, tabs, new-tab, switch-tab).
package cli

import (
	"github.com/spf13/cobra"
)

func newChromeNavigateCmd(flags *rootFlags) *cobra.Command {
	var url string
	cmd := &cobra.Command{
		Use:   "navigate <id>",
		Short: "Navigate Chrome to a URL inside the Orgo VM, or 'back'/'forward' for history.",
		Long:  `Navigate the Chrome browser inside the targeted Orgo VM to a URL. Passing 'back' or 'forward' moves through history instead.`,
		Example: `  orgo-pp-cli chrome navigate <id> --url https://example.com
  orgo-pp-cli chrome navigate <id> --url back`,
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := runChromeCall(cmd, flags, args, "POST", "/navigate", map[string]any{"url": url})
			if err != nil {
				return err
			}
			return printChromeResp(cmd, flags, resp)
		},
	}
	cmd.Flags().StringVar(&url, "url", "", "URL to navigate to, or 'back'/'forward' (required)")
	_ = cmd.MarkFlagRequired("url")
	return cmd
}

func newChromeTabsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "tabs <id>",
		Short:       "List open Chrome tabs in the Orgo VM. Returns tab targetIds, titles, and URLs.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     `  orgo-pp-cli chrome tabs <id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := runChromeCall(cmd, flags, args, "POST", "/tabs", map[string]any{})
			if err != nil {
				return err
			}
			return printChromeResp(cmd, flags, resp)
		},
	}
	return cmd
}

func newChromeNewTabCmd(flags *rootFlags) *cobra.Command {
	var url string
	cmd := &cobra.Command{
		Use:     "new-tab <id>",
		Short:   "Open a new Chrome tab in the Orgo VM. Optional --url; defaults to about:blank.",
		Example: `  orgo-pp-cli chrome new-tab <id> --url https://news.ycombinator.com`,
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			if url != "" {
				body["url"] = url
			}
			resp, err := runChromeCall(cmd, flags, args, "POST", "/new_tab", body)
			if err != nil {
				return err
			}
			return printChromeResp(cmd, flags, resp)
		},
	}
	cmd.Flags().StringVar(&url, "url", "", "URL to open in the new tab (default: about:blank)")
	return cmd
}

func newChromeSwitchTabCmd(flags *rootFlags) *cobra.Command {
	var targetID string
	cmd := &cobra.Command{
		Use:     "switch-tab <id>",
		Short:   "Switch the active Chrome tab to a different targetId (from chrome tabs).",
		Example: `  orgo-pp-cli chrome switch-tab <id> --target-id ABC123`,
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := runChromeCall(cmd, flags, args, "POST", "/switch_tab", map[string]any{"targetId": targetID})
			if err != nil {
				return err
			}
			return printChromeResp(cmd, flags, resp)
		},
	}
	cmd.Flags().StringVar(&targetID, "target-id", "", "Target ID of the tab to switch to (from chrome tabs) (required)")
	_ = cmd.MarkFlagRequired("target-id")
	return cmd
}
