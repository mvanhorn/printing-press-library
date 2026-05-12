// Hand-authored: novel feature `replay` — single-file static HTML
// timeline of every action against a computer. Created during Phase 3
// build; survives regen because it lives in its own file.
package cli

import (
	"bytes"
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/orgo/internal/store"

	"github.com/spf13/cobra"
)

func newReplayCmd(flags *rootFlags) *cobra.Command {
	var since string
	var outPath string

	cmd := &cobra.Command{
		Use:   "replay <computer-id>",
		Short: "Generate a self-contained static HTML timeline of every action recorded against a computer.",
		Long: `Render every screenshot, bash command, click, exec, and key press
that orgo-pp-cli recorded against a computer into a single HTML file.

The output has zero external dependencies — no CDN fetches, no JS — so
you can email it, attach it to an issue, or open it offline.`,
		Example: strings.Trim(`
  # Last hour of actions, written to stdout (pipe to a file)
  orgo-pp-cli replay abc123 --since 1h > replay.html

  # Last 30 minutes, written to a path
  orgo-pp-cli replay abc123 --since 30m --out /tmp/last-session.html

  # 7-day timeline
  orgo-pp-cli replay abc123 --since 7d --out incident.html
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			computerID := args[0]
			filter := store.ActionFilter{
				ComputerID: computerID,
				Limit:      10000,
			}
			if since != "" {
				ts, err := parseSinceDuration(since)
				if err != nil {
					return usageErr(fmt.Errorf("invalid --since value %q: %w", since, err))
				}
				filter.Since = ts
			}

			dbPath := defaultDBPath("orgo-pp-cli")
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			actions, err := db.QueryActions(cmd.Context(), filter)
			if err != nil {
				return fmt.Errorf("querying actions: %w", err)
			}

			// QueryActions returns DESC; we want chronological in the report.
			reversed := make([]store.Action, len(actions))
			for i, a := range actions {
				reversed[len(actions)-1-i] = a
			}

			if len(reversed) == 0 {
				fmt.Fprintln(cmd.ErrOrStderr(), "No actions for this computer in the time window. Use --since 30d to widen, or run actions through the CLI first.")
				return notFoundErr(fmt.Errorf("no actions for computer %q in window", computerID))
			}

			rendered := renderReplayHTML(computerID, since, reversed)

			if outPath == "" {
				_, err := cmd.OutOrStdout().Write([]byte(rendered))
				return err
			}

			if dir := filepath.Dir(outPath); dir != "" && dir != "." {
				if err := os.MkdirAll(dir, 0o755); err != nil {
					return fmt.Errorf("creating output directory: %w", err)
				}
			}
			if err := os.WriteFile(outPath, []byte(rendered), 0o644); err != nil {
				return fmt.Errorf("writing replay HTML: %w", err)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "wrote %d events to %s\n", len(reversed), outPath)
			return nil
		},
	}

	cmd.Flags().StringVar(&since, "since", "1h", "Time window (e.g. 30m, 1h, 24h, 7d)")
	cmd.Flags().StringVar(&outPath, "out", "", "Write HTML to this path (default: stdout)")

	return cmd
}

// renderReplayHTML produces a self-contained static HTML page.
// No external CSS, no JS — copies cleanly to email or an issue body.
func renderReplayHTML(computerID, since string, actions []store.Action) string {
	var b bytes.Buffer

	first := actions[0].Timestamp.UTC().Format(time.RFC3339)
	last := actions[len(actions)-1].Timestamp.UTC().Format(time.RFC3339)

	b.WriteString("<!doctype html>\n<html lang=\"en\"><head><meta charset=\"utf-8\">")
	b.WriteString("<title>Replay — " + html.EscapeString(computerID) + "</title>")
	b.WriteString("<style>")
	b.WriteString(`
:root { color-scheme: light dark; }
body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif; margin: 2rem auto; max-width: 1100px; padding: 0 1rem; line-height: 1.5; }
header { border-bottom: 1px solid #ccc; padding-bottom: 0.75rem; margin-bottom: 1.5rem; }
header h1 { margin: 0 0 0.4rem 0; font-size: 1.4rem; font-weight: 600; }
header .meta { color: #666; font-size: 0.9rem; }
table { border-collapse: collapse; width: 100%; font-size: 0.875rem; }
th, td { text-align: left; padding: 0.5rem 0.7rem; border-bottom: 1px solid #ddd; vertical-align: top; }
th { background: rgba(0,0,0,0.03); font-weight: 600; }
.kind { font-family: ui-monospace, "SF Mono", Menlo, monospace; padding: 0.1rem 0.45rem; border-radius: 3px; background: rgba(0,0,0,0.06); font-size: 0.8rem; }
.kind-bash { background: #fef3c7; }
.kind-exec { background: #ddd6fe; }
.kind-click { background: #cffafe; }
.kind-screenshot { background: #d1fae5; }
.kind-key { background: #fee2e2; }
pre { margin: 0; white-space: pre-wrap; word-break: break-word; font-family: ui-monospace, "SF Mono", Menlo, monospace; font-size: 0.8rem; background: rgba(0,0,0,0.04); padding: 0.5rem; border-radius: 3px; max-height: 240px; overflow: auto; }
.note { color: #777; font-style: italic; }
@media (prefers-color-scheme: dark) {
  body { background: #1a1a1a; color: #eee; }
  header { border-color: #444; }
  th { background: rgba(255,255,255,0.05); }
  th, td { border-color: #333; }
  .kind { background: rgba(255,255,255,0.08); }
  .kind-bash { background: #92400e; color: #fef3c7; }
  .kind-exec { background: #5b21b6; color: #ede9fe; }
  .kind-click { background: #155e75; color: #cffafe; }
  .kind-screenshot { background: #065f46; color: #d1fae5; }
  .kind-key { background: #991b1b; color: #fee2e2; }
  pre { background: rgba(255,255,255,0.05); }
}
`)
	b.WriteString("</style></head><body>")
	b.WriteString("<header>")
	b.WriteString("<h1>Replay — " + html.EscapeString(computerID) + "</h1>")
	b.WriteString("<div class=\"meta\">")
	b.WriteString(fmt.Sprintf("%d events &middot; %s &rarr; %s",
		len(actions),
		html.EscapeString(first),
		html.EscapeString(last)))
	if since != "" {
		b.WriteString(" &middot; window: " + html.EscapeString(since))
	}
	b.WriteString("</div></header>")

	b.WriteString("<table><thead><tr><th>Time</th><th>Kind</th><th>Command / Args</th><th>Output</th></tr></thead><tbody>")
	for _, a := range actions {
		b.WriteString("<tr>")
		b.WriteString("<td>" + html.EscapeString(a.Timestamp.UTC().Format("2006-01-02 15:04:05Z")) + "</td>")
		b.WriteString("<td><span class=\"kind kind-" + html.EscapeString(a.Kind) + "\">" + html.EscapeString(a.Kind) + "</span></td>")

		switch a.Kind {
		case "bash", "exec":
			b.WriteString("<td><pre><code>")
			b.WriteString(html.EscapeString(a.Command))
			b.WriteString("</code></pre></td>")
		case "screenshot":
			b.WriteString("<td><span class=\"note\">screenshot taken (PNG body not stored in v1)</span></td>")
		default:
			argText := a.Command
			if argText == "" {
				argText = a.ArgsJSON
			}
			b.WriteString("<td><pre><code>" + html.EscapeString(argText) + "</code></pre></td>")
		}

		// Output cell
		if strings.TrimSpace(a.OutputSnippet) == "" {
			b.WriteString("<td><span class=\"note\">(no output recorded)</span></td>")
		} else {
			b.WriteString("<td><pre><code>" + html.EscapeString(truncate(a.OutputSnippet, 800)) + "</code></pre></td>")
		}
		b.WriteString("</tr>\n")
	}
	b.WriteString("</tbody></table>")
	b.WriteString("</body></html>\n")

	return b.String()
}
