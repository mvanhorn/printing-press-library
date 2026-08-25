// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-built: built-in starter page templates. Absorbs the nsmith/html skill's
// template concept; each template is self-contained (inline CSS/JS) so it
// publishes in a single request with no asset step. Survives generate --force.
// pp:data-source local

package cli

import (
	"embed"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

//go:embed templates_data/*.html
var templateFS embed.FS

// builtinTemplates maps a template name to a one-line description.
var builtinTemplates = map[string]string{
	"blank":         "Minimal, well-styled HTML skeleton",
	"landing":       "Hero landing page with a call to action",
	"slide-deck":    "Arrow-key navigable presentation",
	"status-report": "Weekly status update with metric cards",
	"doc":           "Long-form article / write-up / spec",
}

func templateNames() []string {
	names := make([]string, 0, len(builtinTemplates))
	for n := range builtinTemplates {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func newTemplatesCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "templates",
		Short:       "List the built-in starter page templates",
		Long:        trimNL("List the built-in, self-contained HTML templates. Use 'ht-ml-pp-cli new --template <name>' to scaffold one, then publish it."),
		Example:     "  ht-ml-pp-cli templates",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			type tmpl struct {
				Name        string `json:"name"`
				Description string `json:"description"`
			}
			out := make([]tmpl, 0, len(builtinTemplates))
			for _, n := range templateNames() {
				out = append(out, tmpl{Name: n, Description: builtinTemplates[n]})
			}
			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				tw := newTabWriter(cmd.OutOrStdout())
				fmt.Fprintln(tw, bold("TEMPLATE")+"\t"+bold("DESCRIPTION"))
				for _, t := range out {
					fmt.Fprintf(tw, "%s\t%s\n", t.Name, t.Description)
				}
				return tw.Flush()
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	return cmd
}

func newNewCmd(flags *rootFlags) *cobra.Command {
	var template, title, out string
	var publish bool

	cmd := &cobra.Command{
		Use:   "new --template <name> [--title <title>]",
		Short: "Scaffold a page from a built-in template (optionally publish it)",
		Long:  trimNL("Scaffold HTML from a built-in template. Writes to --out (or stdout), and with --publish sends it straight to ht-ml.app."),
		Example: trimNL(`
  ht-ml-pp-cli new --template slide-deck --title "Q3 Review" --out deck.html
  ht-ml-pp-cli new --template landing --title "Acme" --publish`),
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if template == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--template is required (see 'ht-ml-pp-cli templates')"))
			}
			if _, ok := builtinTemplates[template]; !ok {
				return usageErr(fmt.Errorf("unknown template %q; available: %s", template, strings.Join(templateNames(), ", ")))
			}
			raw, err := templateFS.ReadFile("templates_data/" + template + ".html")
			if err != nil {
				return fmt.Errorf("loading template %q: %w", template, err)
			}
			heading := title
			if heading == "" {
				heading = "Untitled"
			}
			docTitle := title
			if docTitle == "" {
				docTitle = template
			}
			html := strings.NewReplacer(
				"{{TITLE}}", docTitle,
				"{{HEADING}}", heading,
				"{{BODY}}", "Replace this with your content.",
			).Replace(string(raw))

			if publish {
				if htmlxWriteGuard(cmd, flags, "publish a "+template+" template") {
					return nil
				}
				return publishHTMLString(cmd, flags, html, docTitle)
			}

			if out != "" {
				if err := os.WriteFile(out, []byte(html), 0o644); err != nil {
					return fmt.Errorf("writing %s: %w", out, err)
				}
				if wantsHumanTable(cmd.OutOrStdout(), flags) {
					fmt.Fprintf(cmd.OutOrStdout(), "%s wrote %s template to %s\n", green("ok:"), template, out)
					return nil
				}
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"template": template, "out": out}, flags)
			}
			fmt.Fprint(cmd.OutOrStdout(), html)
			return nil
		},
	}
	cmd.Flags().StringVar(&template, "template", "", "Template name (see 'ht-ml-pp-cli templates')")
	cmd.Flags().StringVar(&title, "title", "", "Title/heading to fill into the template")
	cmd.Flags().StringVar(&out, "out", "", "Write the scaffolded HTML to this file (default: stdout)")
	cmd.Flags().BoolVar(&publish, "publish", false, "Publish the scaffolded HTML to ht-ml.app immediately")
	return cmd
}
