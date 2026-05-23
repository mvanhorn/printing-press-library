package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/vault"
)

func newDailyCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daily",
		Short: "Operate on today's (or another date's) daily note.",
	}
	cmd.AddCommand(newDailyAppendCmd(flags))
	cmd.AddCommand(newDailyGetCmd(flags))
	return cmd
}

func newDailyAppendCmd(flags *rootFlags) *cobra.Command {
	var section, dateFlag, folder string
	cmd := &cobra.Command{
		Use:   "append [text]",
		Short: "Append text to today's daily note (creating it from a protocol template if missing).",
		Long: "Resolve today's daily-note path under the configured Daily folder\n" +
			"(`Daily/YYYY-MM-DD.md` by default), create the note from a\n" +
			"protocol-compliant template if it doesn't exist, then append the\n" +
			"text under the named section (defaults to `## Notes`).",
		Example:     "  obsidian-pp-cli daily append 'Talked to Mark about pricing'\n  obsidian-pp-cli daily append 'Idea: pull-stream FIFOs' --section '## Ideas'",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}
			text := args[0]
			v, _, err := openVaultOnly()
			if err != nil {
				return err
			}
			date := dateFlag
			if date == "" {
				date = time.Now().Format("2006-01-02")
			}
			folderName := folder
			if folderName == "" {
				folderName = "Daily"
			}
			rel := filepath.Join(folderName, date+".md")
			abs, err := v.ResolveAbs(rel)
			if err != nil {
				return usageErr(err)
			}
			if _, err := os.Stat(abs); os.IsNotExist(err) {
				// Create from protocol template.
				fm := vault.Frontmatter{
					Type:        "journal",
					Date:        date,
					Description: "Daily journal entry for " + date,
					Status:      "active",
				}
				body := "\n# " + date + "\n\n## Notes\n\n"
				if section != "" && section != "## Notes" {
					body += section + "\n\n"
				}
				data, err := vault.AssembleNote(fm, []byte(body))
				if err != nil {
					return apiErr(err)
				}
				if err := vault.AtomicWrite(abs, data, 0o644); err != nil {
					return apiErr(err)
				}
			} else if err != nil {
				return apiErr(err)
			}
			// Read, append under section, write.
			data, err := os.ReadFile(abs)
			if err != nil {
				return apiErr(err)
			}
			sectionHeader := section
			if sectionHeader == "" {
				sectionHeader = "## Notes"
			}
			appended := appendUnderSection(string(data), sectionHeader, text)
			if err := vault.AtomicWrite(abs, []byte(appended), 0o644); err != nil {
				return apiErr(err)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]string{
					"path":    rel,
					"section": sectionHeader,
					"status":  "appended",
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "appended to %s under %s\n", rel, sectionHeader)
			return nil
		},
	}
	cmd.Flags().StringVar(&section, "section", "## Notes", "Section header under which to append")
	cmd.Flags().StringVar(&dateFlag, "date", "", "Target date (YYYY-MM-DD; defaults to today)")
	cmd.Flags().StringVar(&folder, "folder", "Daily", "Folder containing daily notes")
	return cmd
}

func newDailyGetCmd(flags *rootFlags) *cobra.Command {
	var dateFlag, folder string
	cmd := &cobra.Command{
		Use:         "get",
		Short:       "Print the path (and body, with --json) of today's daily note.",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}
			v, _, err := openVaultOnly()
			if err != nil {
				return err
			}
			date := dateFlag
			if date == "" {
				date = time.Now().Format("2006-01-02")
			}
			folderName := folder
			if folderName == "" {
				folderName = "Daily"
			}
			rel := filepath.Join(folderName, date+".md")
			abs, err := v.ResolveAbs(rel)
			if err != nil {
				return usageErr(err)
			}
			if _, err := os.Stat(abs); err != nil {
				return notFoundErr(fmt.Errorf("daily note not found: %s", rel))
			}
			if flags.asJSON {
				data, _ := os.ReadFile(abs)
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]string{
					"path": rel,
					"body": string(data),
				})
			}
			fmt.Fprintln(cmd.OutOrStdout(), rel)
			return nil
		},
	}
	cmd.Flags().StringVar(&dateFlag, "date", "", "Target date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&folder, "folder", "Daily", "Folder containing daily notes")
	return cmd
}

// appendUnderSection inserts text directly under the given section header
// in body. If the section doesn't exist, it's appended at the end of the file.
func appendUnderSection(body, header, text string) string {
	// Trim leading whitespace for cleanliness.
	text = strings.TrimSpace(text)
	if text == "" {
		return body
	}
	headerLine := strings.TrimSpace(header)
	lines := strings.Split(body, "\n")
	for i, l := range lines {
		if strings.TrimSpace(l) == headerLine {
			// Insert after the header line, and after any consecutive blank lines.
			insertAt := i + 1
			for insertAt < len(lines) && strings.TrimSpace(lines[insertAt]) == "" {
				insertAt++
			}
			prefix := strings.Join(lines[:insertAt], "\n")
			suffix := strings.Join(lines[insertAt:], "\n")
			ts := time.Now().Format("15:04")
			entry := "- " + ts + " — " + text
			out := prefix
			if !strings.HasSuffix(prefix, "\n") {
				out += "\n"
			}
			out += entry + "\n"
			if suffix != "" {
				out += suffix
			}
			return out
		}
	}
	// Section not found — append both.
	ts := time.Now().Format("15:04")
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	body += "\n" + headerLine + "\n\n- " + ts + " — " + text + "\n"
	return body
}
