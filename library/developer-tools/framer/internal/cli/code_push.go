// Copyright 2026 ioncom. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/framer/internal/client"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/framer/internal/store"
	"github.com/spf13/cobra"
)

type codePushDiff struct {
	Action  string `json:"action,omitempty"`
	Name    string `json:"name"`
	File    string `json:"file,omitempty"`
	Status  string `json:"status"` // "new", "modified", "unchanged", "would_create_new_file"
	OldSize int    `json:"old_size,omitempty"`
	NewSize int    `json:"new_size"`
	Diff    string `json:"diff,omitempty"`
}

func newCodePushCmd(flags *rootFlags) *cobra.Command {
	var name string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "code-push <file.tsx>",
		Short: "Show what a code file push would change (dry-run only for now)",
		Long: strings.Trim(`
Read a local TSX/JS file and compare it against the stored version in the local
SQLite store. Shows a diff of what would change if pushed.

Live push requires the WebSocket bridge (not yet implemented). For now, this
command only previews changes.`, "\n"),
		Example: strings.Trim(`
  # Preview what would change
  framer-pp-cli code-push MyComponent.tsx --name MyComponent --dry-run

  # JSON output for automation
  framer-pp-cli code-push MyComponent.tsx --name MyComponent --dry-run --json`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if name == "" {
				return usageErr(fmt.Errorf("--name is required"))
			}

			// Normalize name: try both with and without .tsx suffix
			// Framer stores files as "Name.tsx" but users type "Name"
			nameVariants := []string{name}
			if !strings.HasSuffix(name, ".tsx") && !strings.HasSuffix(name, ".ts") && !strings.HasSuffix(name, ".js") {
				nameVariants = append(nameVariants, name+".tsx")
			} else {
				// Also try without suffix
				bare := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(name, ".tsx"), ".ts"), ".js")
				nameVariants = append(nameVariants, bare)
			}

			localFile := args[0]
			localContent, err := os.ReadFile(localFile)
			if err != nil {
				return fmt.Errorf("reading local file: %w", err)
			}

			if dbPath == "" {
				dbPath = defaultDBPath("framer-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'framer-pp-cli sync' first to populate the local database.", err)
			}
			defer db.Close()

			// Try to find the existing code file by name (with suffix normalization)
			diff := codePushDiff{
				Name:    name,
				NewSize: len(localContent),
			}

			var resolved string
			var resolveErr error
			for _, variant := range nameVariants {
				resolved, resolveErr = db.ResolveByName("code", variant, "name", "title", "fileName")
				if resolveErr == nil {
					break
				}
			}
			if resolveErr != nil {
				// New file — no existing version
				diff.Status = "new"
			} else {
				raw, getErr := db.Get("code", resolved)
				if getErr != nil {
					diff.Status = "new"
				} else {
					var obj map[string]any
					if json.Unmarshal(raw, &obj) == nil {
						existingContent := extractCodeContent(obj)
						diff.OldSize = len(existingContent)

						if existingContent == string(localContent) {
							diff.Status = "unchanged"
						} else {
							diff.Status = "modified"
							diff.Diff = simpleLineDiff(existingContent, string(localContent))
						}
					} else {
						diff.Status = "new"
					}
				}
			}

			// Populate dry-run metadata for new files
			if dryRunOK(flags) && diff.Status == "new" {
				diff.Action = "create"
				diff.File = localFile
				diff.Status = "would_create_new_file"
			}

			if flags.asJSON {
				return flags.printJSON(cmd, diff)
			}

			w := cmd.OutOrStdout()
			switch diff.Status {
			case "would_create_new_file":
				fmt.Fprintf(w, "No existing code file '%s' in local store. Would create new file.\n", diff.Name)
			case "new":
				fmt.Fprintf(w, "NEW: %s (%d bytes)\n", diff.Name, diff.NewSize)
			case "unchanged":
				fmt.Fprintf(w, "UNCHANGED: %s (%d bytes)\n", diff.Name, diff.NewSize)
			case "modified":
				fmt.Fprintf(w, "MODIFIED: %s (%d -> %d bytes)\n", diff.Name, diff.OldSize, diff.NewSize)
				if diff.Diff != "" {
					fmt.Fprintln(w)
					fmt.Fprintln(w, diff.Diff)
				}
			}

			if dryRunOK(flags) {
				return nil
			}

			// Live push via bridge
			bc, err := client.NewBridgeClient()
			if err != nil {
				return fmt.Errorf("cannot push live: %w", err)
			}

			// Find the code file ID from the store
			var codeFileID string
			if resolveErr == nil {
				codeFileID = resolved
			}

			if codeFileID != "" && diff.Status == "modified" {
				// Update existing file via bridge
				// The framer-api uses setFileContent on the CodeFile object
				arg, _ := json.Marshal(map[string]string{
					"id":      codeFileID,
					"content": string(localContent),
				})
				_, err := bc.Call("code-update", string(arg))
				if err != nil {
					return fmt.Errorf("push failed: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Updated %s in Framer (%d bytes pushed, file ID: %s)\n", name, len(localContent), codeFileID)
			} else {
				// Create new file via bridge
				arg, _ := json.Marshal(map[string]string{
					"name":    name,
					"content": string(localContent),
				})
				result, err := bc.Call("code-create", string(arg))
				if err != nil {
					return fmt.Errorf("create failed: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Created %s in Framer: %s\n", name, string(result))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Code file name in Framer (required)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/framer-pp-cli/data.db)")

	return cmd
}

// extractCodeContent pulls the code content string from a stored code object.
func extractCodeContent(obj map[string]any) string {
	if content, ok := obj["content"].(string); ok && content != "" {
		return content
	}
	if data, ok := obj["data"].(map[string]any); ok {
		if content, ok := data["content"].(string); ok && content != "" {
			return content
		}
	}
	if code, ok := obj["code"].(string); ok && code != "" {
		return code
	}
	return ""
}

// simpleLineDiff produces a minimal line-level diff summary.
func simpleLineDiff(old, new string) string {
	oldLines := strings.Split(old, "\n")
	newLines := strings.Split(new, "\n")

	var out strings.Builder
	maxLines := len(oldLines)
	if len(newLines) > maxLines {
		maxLines = len(newLines)
	}

	added, removed := 0, 0
	for i := 0; i < maxLines; i++ {
		var oldLine, newLine string
		if i < len(oldLines) {
			oldLine = oldLines[i]
		}
		if i < len(newLines) {
			newLine = newLines[i]
		}
		if oldLine != newLine {
			if i < len(oldLines) && oldLine != "" {
				removed++
				if removed+added <= 20 {
					fmt.Fprintf(&out, "- %s\n", truncate(oldLine, 80))
				}
			}
			if i < len(newLines) && newLine != "" {
				added++
				if removed+added <= 20 {
					fmt.Fprintf(&out, "+ %s\n", truncate(newLine, 80))
				}
			}
		}
	}

	if removed+added > 20 {
		fmt.Fprintf(&out, "... and %d more changes\n", removed+added-20)
	}

	fmt.Fprintf(&out, "\nSummary: +%d added, -%d removed lines", added, removed)
	return out.String()
}
