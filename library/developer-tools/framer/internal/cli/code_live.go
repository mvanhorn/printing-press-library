package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/framer/internal/client"

	"github.com/spf13/cobra"
)

func newCodeLiveCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "code",
		Short: "Code file management via the live Framer Server API",
		RunE:  parentNoSubcommandRunE(flags),
	}

	cmd.AddCommand(newCodeLiveListCmd(flags))
	cmd.AddCommand(newCodeLiveGetCmd(flags))
	return cmd
}

// codeEntry matches the bridge code-list response shape.
type codeEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

func newCodeLiveListCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all code files from the live Framer project",
		Example: `  framer-pp-cli code list
  framer-pp-cli code list --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would list code files from live Framer project")
				return nil
			}

			bc, err := client.NewBridgeClient()
			if err != nil {
				return err
			}

			raw, err := bc.Call("code-list")
			if err != nil {
				return fmt.Errorf("code-list failed: %w", err)
			}

			var entries []codeEntry
			if err := json.Unmarshal(raw, &entries); err != nil {
				return fmt.Errorf("parsing code-list response: %w", err)
			}

			if flags.asJSON {
				return flags.printJSON(cmd, entries)
			}

			headers := []string{"ID", "NAME", "PATH"}
			var rows [][]string
			for _, e := range entries {
				rows = append(rows, []string{e.ID, e.Name, e.Path})
			}
			return flags.printTable(cmd, headers, rows)
		},
	}
	return cmd
}

func newCodeLiveGetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <id-or-name>",
		Short: "Get a code file's content from the live Framer project",
		Example: `  framer-pp-cli code get 550e8400-e29b-41d4-a716-446655440000
  framer-pp-cli code get MyComponent
  framer-pp-cli code get MyComponent.tsx --json`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			identifier := args[0]

			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would get code file %q from live Framer project\n", identifier)
				return nil
			}

			bc, err := client.NewBridgeClient()
			if err != nil {
				return err
			}

			// Resolve identifier: if it looks like a name (not a UUID-ish string),
			// fetch the code-list and match by name with .tsx suffix normalization.
			id := identifier
			if !looksLikeID(identifier) {
				resolved, resolveErr := resolveCodeName(bc, identifier)
				if resolveErr != nil {
					return resolveErr
				}
				id = resolved
			}

			raw, err := bc.Call("code-get", id)
			if err != nil {
				return fmt.Errorf("code-get failed: %w", err)
			}

			var file struct {
				ID      string `json:"id"`
				Name    string `json:"name"`
				Path    string `json:"path"`
				Content string `json:"content"`
			}
			if err := json.Unmarshal(raw, &file); err != nil {
				return fmt.Errorf("parsing code-get response: %w", err)
			}

			if flags.asJSON {
				return flags.printJSON(cmd, file)
			}

			// Plain mode: just print the content for piping
			fmt.Fprint(cmd.OutOrStdout(), file.Content)
			return nil
		},
	}
	return cmd
}

// looksLikeID returns true if s looks like a UUID or other non-name identifier
// (contains dashes in UUID pattern or is all hex).
func looksLikeID(s string) bool {
	// Simple heuristic: UUIDs have dashes and are 36 chars, or hex strings >=16 chars
	if len(s) == 36 && strings.Count(s, "-") == 4 {
		return true
	}
	if len(s) >= 16 {
		for _, c := range s {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') || c == '-') {
				return false
			}
		}
		return true
	}
	return false
}

// resolveCodeName fetches the code-list from the bridge and finds the entry
// matching the given name, applying .tsx suffix normalization.
func resolveCodeName(bc *client.BridgeClient, identifier string) (string, error) {
	raw, err := bc.Call("code-list")
	if err != nil {
		return "", fmt.Errorf("code-list failed (for name lookup): %w", err)
	}

	var entries []codeEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return "", fmt.Errorf("parsing code-list response: %w", err)
	}

	// Build name variants with .tsx normalization
	variants := []string{identifier}
	if !strings.HasSuffix(identifier, ".tsx") && !strings.HasSuffix(identifier, ".ts") {
		variants = append(variants, identifier+".tsx")
	} else {
		bare := strings.TrimSuffix(strings.TrimSuffix(identifier, ".tsx"), ".ts")
		variants = append(variants, bare)
	}

	for _, entry := range entries {
		for _, v := range variants {
			if strings.EqualFold(entry.Name, v) {
				return entry.ID, nil
			}
		}
	}

	return "", fmt.Errorf("code file %q not found; use 'framer-pp-cli code list' to see available files", identifier)
}
