package cli

// pp:novel-static-reference

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type snippetResult struct {
	Kind      string   `json:"kind"`
	Language  string   `json:"language"`
	ProjectID string   `json:"project_id,omitempty"`
	Snippet   string   `json:"snippet"`
	Notes     []string `json:"notes,omitempty"`
	Source    string   `json:"source"`
}

const clarityAPISource = "https://learn.microsoft.com/en-us/clarity/setup-and-installation/clarity-api"

func newSnippetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "snippet",
		Short: "Render Microsoft Clarity client API snippets",
		Long:  "Render copy-pasteable Microsoft Clarity tracking, JavaScript API, and HTML attribute snippets.",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
	}

	cmd.AddCommand(newSnippetInstallCmd(flags))
	cmd.AddCommand(newSnippetConsentCmd(flags))
	cmd.AddCommand(newSnippetIdentifyCmd(flags))
	cmd.AddCommand(newSnippetSetCmd(flags))
	cmd.AddCommand(newSnippetEventCmd(flags))
	cmd.AddCommand(newSnippetUpgradeCmd(flags))
	cmd.AddCommand(newSnippetMaskCmd(flags))
	return cmd
}

func newSnippetInstallCmd(flags *rootFlags) *cobra.Command {
	format := "html"
	cmd := &cobra.Command{
		Use:   "install <project_id>",
		Short: "Render the Clarity install snippet",
		Example: `  clarity-pp-cli snippet install abc123
  clarity-pp-cli snippet install abc123 --format js`,
		Args: cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			projectID := strings.TrimSpace(args[0])
			if projectID == "" {
				return usageErr(fmt.Errorf("project_id is required"))
			}
			snippet, language, err := renderInstallSnippet(projectID, format)
			if err != nil {
				return usageErr(err)
			}
			return printSnippetResult(cmd, flags, snippetResult{
				Kind:      "install",
				Language:  language,
				ProjectID: projectID,
				Snippet:   snippet,
				Notes: []string{
					"Paste the HTML snippet in the page head.",
					"Do not use Clarity on websites or apps targeting users under 18.",
				},
				Source: "https://learn.microsoft.com/en-us/clarity/setup-and-installation/clarity-setup",
			})
		},
	}
	cmd.Flags().StringVar(&format, "format", "html", "Snippet format: html or js")
	return cmd
}

func newSnippetConsentCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "consent",
		Short:   "Render the Clarity cookie consent call",
		Example: `  clarity-pp-cli snippet consent`,
		Args:    cobra.NoArgs,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			return printSnippetResult(cmd, flags, jsSnippet("consent", `window.clarity("consent");`))
		},
	}
}

func newSnippetIdentifyCmd(flags *rootFlags) *cobra.Command {
	var sessionID, pageID, friendlyName string
	cmd := &cobra.Command{
		Use:     "identify <custom_id>",
		Short:   "Render the Clarity custom identifiers call",
		Example: `  clarity-pp-cli snippet identify user-42 --session-id sess-9 --page-id checkout --friendly-name "Paid customer"`,
		Args:    cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			values := []string{args[0], sessionID, pageID, friendlyName}
			last := len(values) - 1
			for last > 0 && values[last] == "" {
				last--
			}
			quoted := make([]string, 0, last+1)
			for _, v := range values[:last+1] {
				quoted = append(quoted, strconv.Quote(v))
			}
			return printSnippetResult(cmd, flags, jsSnippet("identify", fmt.Sprintf("window.clarity(%s, %s);", strconv.Quote("identify"), strings.Join(quoted, ", "))))
		},
	}
	cmd.Flags().StringVar(&sessionID, "session-id", "", "Optional custom session ID")
	cmd.Flags().StringVar(&pageID, "page-id", "", "Optional custom page ID")
	cmd.Flags().StringVar(&friendlyName, "friendly-name", "", "Optional friendly display name")
	return cmd
}

func newSnippetSetCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value> [value...]",
		Short: "Render the Clarity custom tags call",
		Example: `  clarity-pp-cli snippet set experiment experiment1
  clarity-pp-cli snippet set flight flight1 flight2`,
		Args: cobra.ArbitraryArgs,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			value := strconv.Quote(args[1])
			if len(args) > 2 {
				items := make([]string, 0, len(args)-1)
				for _, v := range args[1:] {
					items = append(items, strconv.Quote(v))
				}
				value = "[" + strings.Join(items, ", ") + "]"
			}
			return printSnippetResult(cmd, flags, jsSnippet("set", fmt.Sprintf("window.clarity(%s, %s, %s);", strconv.Quote("set"), strconv.Quote(args[0]), value)))
		},
	}
}

func newSnippetEventCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "event <name>",
		Short:   "Render the Clarity custom event call",
		Example: `  clarity-pp-cli snippet event newsletterSignup`,
		Args:    cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			return printSnippetResult(cmd, flags, jsSnippet("event", fmt.Sprintf("window.clarity(%s, %s);", strconv.Quote("event"), strconv.Quote(args[0]))))
		},
	}
}

func newSnippetUpgradeCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "upgrade <reason>",
		Short:   "Render the Clarity session-priority call",
		Example: `  clarity-pp-cli snippet upgrade "button click"`,
		Args:    cobra.MaximumNArgs(1),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			return printSnippetResult(cmd, flags, jsSnippet("upgrade", fmt.Sprintf("window.clarity(%s, %s);", strconv.Quote("upgrade"), strconv.Quote(args[0]))))
		},
	}
}

func newSnippetMaskCmd(flags *rootFlags) *cobra.Command {
	var unmask bool
	var tag string
	cmd := &cobra.Command{
		Use:   "mask",
		Short: "Render Clarity mask or unmask HTML attributes",
		Example: `  clarity-pp-cli snippet mask
  clarity-pp-cli snippet mask --unmask --tag article`,
		Args: cobra.NoArgs,
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			kind := "mask"
			attr := `data-clarity-mask="true"`
			if unmask {
				kind = "unmask"
				attr = `data-clarity-unmask="true"`
			}
			tag = strings.TrimSpace(tag)
			snippet := attr
			language := "html-attribute"
			if tag != "" {
				snippet = fmt.Sprintf("<%s %s></%s>", tag, attr, tag)
				language = "html"
			}
			return printSnippetResult(cmd, flags, snippetResult{
				Kind:     kind,
				Language: language,
				Snippet:  snippet,
				Source:   clarityAPISource,
			})
		},
	}
	cmd.Flags().BoolVar(&unmask, "unmask", false, "Render data-clarity-unmask instead of data-clarity-mask")
	cmd.Flags().StringVar(&tag, "tag", "", "Optional HTML tag wrapper to render")
	return cmd
}

func renderInstallSnippet(projectID, format string) (string, string, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "html":
		return fmt.Sprintf(`<script type="text/javascript">
(function(c,l,a,r,i,t,y){
    c[a]=c[a]||function(){(c[a].q=c[a].q||[]).push(arguments)};
    t=l.createElement(r);t.async=1;t.src="https://www.clarity.ms/tag/"+i;
    y=l.getElementsByTagName(r)[0];y.parentNode.insertBefore(t,y);
})(window, document, "clarity", "script", %s);
</script>`, strconv.Quote(projectID)), "html", nil
	case "js", "javascript":
		return fmt.Sprintf(`(function(c,l,a,r,i,t,y){
    c[a]=c[a]||function(){(c[a].q=c[a].q||[]).push(arguments)};
    t=l.createElement(r);t.async=1;t.src="https://www.clarity.ms/tag/"+i;
    y=l.getElementsByTagName(r)[0];y.parentNode.insertBefore(t,y);
})(window, document, "clarity", "script", %s);`, strconv.Quote(projectID)), "javascript", nil
	default:
		return "", "", fmt.Errorf("invalid --format %q: must be html or js", format)
	}
}

func jsSnippet(kind, snippet string) snippetResult {
	return snippetResult{
		Kind:     kind,
		Language: "javascript",
		Snippet:  snippet,
		Source:   clarityAPISource,
	}
}

func printSnippetResult(cmd *cobra.Command, flags *rootFlags, result snippetResult) error {
	if shouldPrintStructured(cmd, flags) {
		return printJSONFiltered(cmd.OutOrStdout(), result, flags)
	}
	fmt.Fprintln(cmd.OutOrStdout(), result.Snippet)
	return nil
}

func shouldPrintStructured(cmd *cobra.Command, flags *rootFlags) bool {
	return flags.asJSON || flags.csv || flags.compact || flags.quiet || flags.plain || flags.selectFields != "" || !isTerminal(cmd.OutOrStdout())
}
