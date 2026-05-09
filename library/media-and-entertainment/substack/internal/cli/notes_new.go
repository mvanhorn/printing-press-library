// Phase 3 hand-authored novel command. Not generator-emitted.

package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/substack/internal/notebuilder"

	"github.com/spf13/cobra"
)

// newNotesNewCmd adds a friendly Markdown-driven Note creator on top of the
// generated POST /comment/feed handler. Internally it converts Markdown via
// notebuilder.BuildProseMirrorJSON and posts the resulting ProseMirror doc.
//
// Why this exists alongside the generated `notes create`:
//   - `notes create` accepts ProseMirror JSON via --body-json (raw, accurate).
//   - `notes new --body <markdown>` is the documented agent-friendly path.
//
// Both wire into the same endpoint; this is the convenience surface.
func newNotesNewCmd(flags *rootFlags) *cobra.Command {
	var body string
	var bodyFile string
	var contentType string
	var tabID string

	cmd := &cobra.Command{
		Use:   "new",
		Short: "Post a new Note from Markdown (auto-converts to ProseMirror)",
		Example: strings.Trim(`
  substack-pp-cli notes new --body "Stop refreshing the feed. Spend 15 minutes replying to commenters."
  substack-pp-cli notes new --body-file /tmp/note.md --json
  echo "today's hot take" | substack-pp-cli notes new --body-file -
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// Verify-mode short-circuit: emit a dry-run shape and exit 0.
			if cliutil.IsVerifyEnv() {
				envelope := map[string]any{
					"action":                   "post",
					"resource":                 "notes",
					"path":                     "/comment/feed",
					"verify_env_short_circuit": true,
					"success":                  false,
					"dry_run":                  true,
				}
				out, _ := json.Marshal(envelope)
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				fmt.Fprintln(os.Stderr, "verify mode short-circuit: would build ProseMirror from --body and POST /comment/feed")
				return nil
			}

			// Resolve the Markdown source: --body wins, then --body-file (-/stdin or path).
			md := body
			if md == "" && bodyFile != "" {
				if bodyFile == "-" {
					data, err := io.ReadAll(os.Stdin)
					if err != nil {
						return fmt.Errorf("reading stdin: %w", err)
					}
					md = string(data)
				} else {
					data, err := os.ReadFile(bodyFile)
					if err != nil {
						return fmt.Errorf("reading %s: %w", bodyFile, err)
					}
					md = string(data)
				}
			}
			if strings.TrimSpace(md) == "" {
				if flags.dryRun {
					return cmd.Help()
				}
				return fmt.Errorf("--body or --body-file is required (use --dry-run or --help to inspect)")
			}

			pmJSON, err := notebuilder.BuildProseMirrorJSON(md)
			if err != nil {
				return fmt.Errorf("converting Markdown to ProseMirror: %w", err)
			}
			var pmAny any
			if err := json.Unmarshal(pmJSON, &pmAny); err != nil {
				return fmt.Errorf("re-decoding ProseMirror JSON: %w", err)
			}

			// Build the request body shape the generated handler expects.
			reqBody := map[string]any{
				"bodyJson": pmAny,
			}
			if contentType != "" {
				reqBody["type"] = contentType
			}
			if tabID != "" {
				reqBody["tabId"] = tabID
			}

			// Dry-run: print envelope without firing.
			if flags.dryRun {
				envelope := map[string]any{
					"action":   "post",
					"resource": "notes",
					"path":     "/comment/feed",
					"status":   0,
					"success":  false,
					"dry_run":  true,
					"data":     reqBody,
				}
				out, _ := json.Marshal(envelope)
				fmt.Fprintln(cmd.OutOrStdout(), string(out))
				return nil
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			data, statusCode, err := c.Post("/comment/feed", reqBody)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				if flags.quiet {
					return nil
				}
				envelope := map[string]any{
					"action":   "post",
					"resource": "notes",
					"path":     "/comment/feed",
					"status":   statusCode,
					"success":  statusCode >= 200 && statusCode < 300,
				}
				if len(data) > 0 {
					var parsed any
					if err := json.Unmarshal(data, &parsed); err == nil {
						envelope["data"] = parsed
					}
				}
				envBytes, _ := json.Marshal(envelope)
				return printOutput(cmd.OutOrStdout(), json.RawMessage(envBytes), true)
			}
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVar(&body, "body", "", "Note body in Markdown (auto-converted to ProseMirror)")
	cmd.Flags().StringVar(&bodyFile, "body-file", "", "Path to a Markdown file (use - for stdin)")
	cmd.Flags().StringVar(&contentType, "type", "", "Content type (default `feed`)")
	cmd.Flags().StringVar(&tabID, "tab-id", "", "UI tab the Note is posted from (default `for-you`)")
	return cmd
}
