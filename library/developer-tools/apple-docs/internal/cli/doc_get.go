// Copyright 2026 joseph-alvin-castillo. Licensed under Apache-2.0. See LICENSE.

// pp:data-source live

package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/apple-docs/internal/applejson"

	"github.com/spf13/cobra"
)

func newNovelDocGetCmd(flags *rootFlags) *cobra.Command {
	var flagShape string
	var flagMarkdown bool

	cmd := &cobra.Command{
		Use:   "get <path>",
		Short: "Fetch an Apple doc page, with optional shape projection or Markdown rendering",
		Long: strings.TrimSpace(`
Fetch a documentation page by path and return it in the format your tool
actually wants.

The path is the lowercase-slashed identifier under /documentation/, e.g.
'swiftui', 'swiftui/view', or 'swiftui/view/onappear(perform:)'.

Output modes (mutually exclusive):
  --shape abstract    just title + abstract + identifier
  --shape signature   title + abstract + identifier + declaration
  --shape platforms   title + identifier + per-platform availability
  --shape min         all four of the above
  --markdown          full doc rendered as Markdown (sosumi-style)
  (default)           full raw DocC JSON

Use this command when an agent needs the minimal viable doc payload (abstract,
signature, platforms, or all three). Do NOT use it for the full rendered doc
page; pass --shape min for a compact summary or 'bundle' for a multi-page
context blob.
`),
		Example: strings.Trim(`
  apple-docs-pp-cli doc get swiftui/view --shape signature --agent
  apple-docs-pp-cli doc get foundation/url/init(string:) --shape min --json
  apple-docs-pp-cli doc get uikit/uiview --markdown
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would fetch doc page\n")
				return nil
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("<path> is required"))
			}
			if flagShape != "" && flagMarkdown {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--shape and --markdown are mutually exclusive"))
			}
			if flagShape != "" && !validShape(flagShape) {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--shape must be one of: abstract, signature, platforms, min"))
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			page, err := applejson.FetchDoc(cmd.Context(), c, args[0])
			if err != nil {
				return classifyAPIError(err, flags)
			}

			if flagMarkdown {
				out := renderMarkdown(page)
				fmt.Fprintln(cmd.OutOrStdout(), out)
				return nil
			}

			var payload any
			if flagShape == "" {
				// Default: emit the raw DocC JSON.
				var raw any
				if err := json.Unmarshal(page.RawJSON, &raw); err != nil {
					return err
				}
				payload = raw
			} else {
				payload = projectShape(page, flagShape)
			}
			return emitJSON(cmd, flags, payload)
		},
	}
	cmd.Flags().StringVar(&flagShape, "shape", "", "Project the doc page to a subset of fields: abstract|signature|platforms|min")
	cmd.Flags().BoolVar(&flagMarkdown, "markdown", false, "Render the doc as Markdown instead of JSON")
	return cmd
}

func validShape(s string) bool {
	switch s {
	case "abstract", "signature", "platforms", "min":
		return true
	}
	return false
}

func projectShape(p *applejson.DocPage, shape string) map[string]any {
	out := map[string]any{
		"identifier": p.Identifier,
		"title":      p.Title,
	}
	add := func(keys ...string) {
		for _, k := range keys {
			switch k {
			case "abstract":
				if p.Abstract != "" {
					out["abstract"] = p.Abstract
				}
			case "signature":
				if p.Declaration != "" {
					out["declaration"] = p.Declaration
				}
				if p.SymbolKind != "" {
					out["symbol_kind"] = p.SymbolKind
				}
			case "platforms":
				if len(p.Platforms) > 0 {
					out["platforms"] = p.Platforms
				}
			}
		}
	}
	switch shape {
	case "abstract":
		add("abstract")
	case "signature":
		add("abstract", "signature")
	case "platforms":
		add("platforms")
	case "min":
		add("abstract", "signature", "platforms")
	}
	return out
}

func renderMarkdown(p *applejson.DocPage) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "# %s\n\n", p.Title)
	if p.SymbolKind != "" {
		fmt.Fprintf(&sb, "_Kind: %s", p.SymbolKind)
		if len(p.Modules) > 0 {
			fmt.Fprintf(&sb, " · Module: %s", strings.Join(p.Modules, ", "))
		}
		sb.WriteString("_\n\n")
	}
	if p.Abstract != "" {
		fmt.Fprintf(&sb, "%s\n\n", p.Abstract)
	}
	if p.Declaration != "" {
		fmt.Fprintf(&sb, "```swift\n%s\n```\n\n", p.Declaration)
	}
	if len(p.Platforms) > 0 {
		sb.WriteString("## Platforms\n\n")
		for _, plat := range p.Platforms {
			fmt.Fprintf(&sb, "- **%s**", plat.Name)
			if plat.IntroducedAt != "" {
				fmt.Fprintf(&sb, " %s+", plat.IntroducedAt)
			}
			if plat.DeprecatedAt != "" {
				fmt.Fprintf(&sb, " — deprecated in %s", plat.DeprecatedAt)
			}
			if plat.Deprecated {
				sb.WriteString(" (deprecated)")
			}
			if plat.Unavailable {
				sb.WriteString(" (unavailable)")
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
	if p.URL != "" {
		fmt.Fprintf(&sb, "Source: %s\n", appleDocsSourceURL(p.URL))
	}
	return strings.TrimRight(sb.String(), "\n")
}

func emitJSON(cmd *cobra.Command, flags *rootFlags, v any) error {
	w := cmd.OutOrStdout()
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if flags.selectFields != "" {
		data = filterFields(data, flags.selectFields)
	} else if flags.compact {
		data = compactFields(data)
	}
	_, err = w.Write(append(data, '\n'))
	return err
}

// appleDocsSourceURL maps a DocC identifier like
// "doc://com.apple.SwiftUI/documentation/SwiftUI/View/onAppear(perform:)" to
// the canonical developer.apple.com URL the docs SPA serves at
// "https://developer.apple.com/documentation/swiftui/view/onappear(perform:)".
// Inputs that don't follow the DocC shape fall back to the host + URL
// stripped of the scheme prefix, which keeps the value visibly attributable
// without fabricating a path.
func appleDocsSourceURL(docURL string) string {
	const host = "https://developer.apple.com"
	const prefix = "doc://com.apple."
	if !strings.HasPrefix(docURL, prefix) {
		// Best-effort fallback: drop any scheme and join with the host.
		trimmed := strings.TrimPrefix(docURL, "doc://")
		if !strings.HasPrefix(trimmed, "/") {
			trimmed = "/" + trimmed
		}
		return host + strings.ToLower(trimmed)
	}
	// Drop "doc://com.apple." and the module segment that follows
	// ("SwiftUI", "UIKit", etc.) so the path starts at /documentation/...
	rest := docURL[len(prefix):]
	if idx := strings.Index(rest, "/"); idx >= 0 {
		rest = rest[idx:] // keep the leading "/"
	} else {
		// No module segment — treat the whole tail as the path.
		rest = "/" + rest
	}
	return host + strings.ToLower(rest)
}
