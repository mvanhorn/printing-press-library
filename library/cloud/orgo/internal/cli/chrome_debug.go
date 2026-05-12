// Hand-authored: chrome debug subcommands (evaluate, console, network).
package cli

import (
	"github.com/spf13/cobra"
)

func newChromeEvaluateCmd(flags *rootFlags) *cobra.Command {
	var expression string
	var awaitPromise bool
	cmd := &cobra.Command{
		Use:   "evaluate <id>",
		Short: "Run JavaScript in the Chrome page context and return the last expression's result.",
		Long: `Do NOT include 'return' — pass the expression to evaluate directly
(e.g., 'document.title'). The bridge wraps it appropriately.`,
		Example: `  orgo-pp-cli chrome evaluate <id> --expression "document.title"
  orgo-pp-cli chrome evaluate <id> --expression "fetch('/api/me').then(r => r.json())" --await-promise`,
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{"expression": expression}
			if awaitPromise {
				body["awaitPromise"] = true
			}
			resp, err := runChromeCall(cmd, flags, args, "POST", "/evaluate", body)
			if err != nil {
				return err
			}
			return printChromeResp(cmd, flags, resp)
		},
	}
	cmd.Flags().StringVar(&expression, "expression", "", "JavaScript to evaluate in the page context (required)")
	cmd.Flags().BoolVar(&awaitPromise, "await-promise", false, "Await the result if it's a Promise")
	_ = cmd.MarkFlagRequired("expression")
	return cmd
}

func newChromeConsoleCmd(flags *rootFlags) *cobra.Command {
	var pattern string
	var limit int
	var onlyErrors, clear bool
	cmd := &cobra.Command{
		Use:         "console <id>",
		Short:       "Read buffered Chrome console messages (log, error, warn). Optional regex --pattern filter.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  orgo-pp-cli chrome console <id> --only-errors
  orgo-pp-cli chrome console <id> --pattern "MyApp" --limit 50`,
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			if pattern != "" {
				body["pattern"] = pattern
			}
			if onlyErrors {
				body["onlyErrors"] = true
			}
			if limit > 0 {
				body["limit"] = limit
			}
			if clear {
				body["clear"] = true
			}
			resp, err := runChromeCall(cmd, flags, args, "POST", "/console", body)
			if err != nil {
				return err
			}
			return printChromeResp(cmd, flags, resp)
		},
	}
	cmd.Flags().StringVar(&pattern, "pattern", "", "Regex pattern to filter messages")
	cmd.Flags().BoolVar(&onlyErrors, "only-errors", false, "Only return error/exception messages")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max messages to return (default: 100)")
	cmd.Flags().BoolVar(&clear, "clear", false, "Clear the buffer after reading")
	return cmd
}

func newChromeNetworkCmd(flags *rootFlags) *cobra.Command {
	var urlPattern string
	var limit int
	var clear bool
	cmd := &cobra.Command{
		Use:         "network <id>",
		Short:       "Read buffered Chrome network requests (XHR, Fetch, docs). Optional --url-pattern substring filter.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  orgo-pp-cli chrome network <id> --url-pattern "/api/"
  orgo-pp-cli chrome network <id> --limit 25 --clear`,
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			if urlPattern != "" {
				body["urlPattern"] = urlPattern
			}
			if limit > 0 {
				body["limit"] = limit
			}
			if clear {
				body["clear"] = true
			}
			resp, err := runChromeCall(cmd, flags, args, "POST", "/network", body)
			if err != nil {
				return err
			}
			return printChromeResp(cmd, flags, resp)
		},
	}
	cmd.Flags().StringVar(&urlPattern, "url-pattern", "", "URL substring to filter requests")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max requests to return (default: 100)")
	cmd.Flags().BoolVar(&clear, "clear", false, "Clear the buffer after reading")
	return cmd
}
