// Hand-authored: chrome read subcommands (read-page, find, page-text, screenshot).
package cli

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newChromeReadPageCmd(flags *rootFlags) *cobra.Command {
	var filter, refID string
	var depth, maxChars int
	cmd := &cobra.Command{
		Use:   "read-page <id>",
		Short: "Get an accessibility tree for the current Chrome page. Elements have refs (ref_N) usable by click/form-input.",
		Long: `Returns a structured tree of the current page with stable refs you can pass
to click and form-input. Much cheaper than a screenshot for navigation and
form filling; use --filter interactive for just buttons/links/inputs.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  orgo-pp-cli chrome read-page <id> --filter interactive
  orgo-pp-cli chrome read-page <id> --depth 8 --max-chars 20000`,
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			if filter != "" {
				body["filter"] = filter
			}
			if depth > 0 {
				body["depth"] = depth
			}
			if maxChars > 0 {
				body["max_chars"] = maxChars
			}
			if refID != "" {
				body["ref_id"] = refID
			}
			resp, err := runChromeCall(cmd, flags, args, "POST", "/read_page", body)
			if err != nil {
				return err
			}
			return printChromeResp(cmd, flags, resp)
		},
	}
	cmd.Flags().StringVar(&filter, "filter", "", "Filter: 'all' or 'interactive' (default: all)")
	cmd.Flags().IntVar(&depth, "depth", 0, "Max tree depth (default: 15)")
	cmd.Flags().IntVar(&maxChars, "max-chars", 0, "Max output characters (default: 50000)")
	cmd.Flags().StringVar(&refID, "ref-id", "", "Focus on a specific element subtree by ref")
	return cmd
}

func newChromeFindCmd(flags *rootFlags) *cobra.Command {
	var query string
	cmd := &cobra.Command{
		Use:         "find <id>",
		Short:       "Find elements on the Chrome page by text/purpose. Returns up to 20 matches with refs.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  orgo-pp-cli chrome find <id> --query "search bar"
  orgo-pp-cli chrome find <id> --query "login button"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := runChromeCall(cmd, flags, args, "POST", "/find", map[string]any{"query": query})
			if err != nil {
				return err
			}
			return printChromeResp(cmd, flags, resp)
		},
	}
	cmd.Flags().StringVar(&query, "query", "", "What to find (e.g., 'search bar', 'login button') (required)")
	_ = cmd.MarkFlagRequired("query")
	return cmd
}

func newChromePageTextCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "page-text <id>",
		Short:       "Extract raw text content from the current Chrome page (good for articles, blog posts).",
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example:     `  orgo-pp-cli chrome page-text <id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := runChromeCall(cmd, flags, args, "POST", "/page_text", map[string]any{})
			if err != nil {
				return err
			}
			return printChromeResp(cmd, flags, resp)
		},
	}
	return cmd
}

func newChromeScreenshotCmd(flags *rootFlags) *cobra.Command {
	var format string
	var quality int
	var out string
	cmd := &cobra.Command{
		Use:   "screenshot <id>",
		Short: "Take a screenshot of the current Chrome page. Returns base64 PNG/JPEG, or writes to a file with --out.",
		Long: `Returns a base64-encoded image in the response by default. Pass --out
<path> to decode and write the binary image directly to disk. MCP clients
that auto-render images receive the base64-as-text — decode client-side, or
upgrade to the Orgo typed screenshot MCP tool if inline rendering matters.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		Example: `  orgo-pp-cli chrome screenshot <id> --out /tmp/page.png
  orgo-pp-cli chrome screenshot <id> --format jpeg --quality 70 --out /tmp/page.jpg`,
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]any{}
			if format != "" {
				body["format"] = format
			}
			if quality > 0 {
				body["quality"] = quality
			}
			resp, err := runChromeCall(cmd, flags, args, "POST", "/screenshot", body)
			if err != nil {
				return err
			}
			if out == "" {
				return printChromeResp(cmd, flags, resp)
			}
			var parsed struct {
				Image string `json:"image"`
			}
			if err := json.Unmarshal(resp, &parsed); err != nil {
				return fmt.Errorf("parsing screenshot response: %w", err)
			}
			if parsed.Image == "" {
				return fmt.Errorf("screenshot response missing 'image' field: %s", string(resp))
			}
			raw, err := base64.StdEncoding.DecodeString(parsed.Image)
			if err != nil {
				return fmt.Errorf("decoding base64 screenshot: %w", err)
			}
			if err := os.WriteFile(out, raw, 0o644); err != nil {
				return fmt.Errorf("writing %s: %w", out, err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "{\"saved\": %q, \"bytes\": %d}\n", out, len(raw))
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "", "Image format: png or jpeg (default: png)")
	cmd.Flags().IntVar(&quality, "quality", 0, "JPEG quality 0-100 (default: 80, ignored for png)")
	cmd.Flags().StringVar(&out, "out", "", "Decode base64 and write to this file path")
	return cmd
}
