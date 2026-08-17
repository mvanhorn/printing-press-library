// Copyright 2026 ioncom. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/framer/internal/store"
	"github.com/spf13/cobra"
)

func newCodePullCmd(flags *rootFlags) *cobra.Command {
	var outputFile string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "code-pull <code_file_id_or_name>",
		Short: "Pull a code file from the local store to a local file",
		Long: strings.Trim(`
Read a code file from the synced local store and write its content to a local
file. The code file can be identified by ID or name.

The content field from the stored JSON data is extracted and written as-is,
making it ready to edit in your local editor.`, "\n"),
		Example: strings.Trim(`
  # Pull a code file by ID to stdout
  framer-pp-cli code-pull abc123

  # Pull a code file to a local file
  framer-pp-cli code-pull abc123 -o MyComponent.tsx

  # Pull by name
  framer-pp-cli code-pull "MyComponent" -o MyComponent.tsx

  # Pull as JSON metadata
  framer-pp-cli code-pull abc123 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			identifier := args[0]

			if dbPath == "" {
				dbPath = defaultDBPath("framer-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'framer-pp-cli sync' first to populate the local database.", err)
			}
			defer db.Close()

			// Try direct ID lookup first
			raw, err := db.Get("code", identifier)
			if err != nil {
				// Try name-based resolution with .tsx suffix normalization
				variants := []string{identifier}
				if !strings.HasSuffix(identifier, ".tsx") && !strings.HasSuffix(identifier, ".ts") && !strings.HasSuffix(identifier, ".js") {
					variants = append(variants, identifier+".tsx")
				} else {
					bare := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(identifier, ".tsx"), ".ts"), ".js")
					variants = append(variants, bare)
				}
				var resolved string
				var resolveErr error
				for _, v := range variants {
					resolved, resolveErr = db.ResolveByName("code", v, "name", "title", "fileName")
					if resolveErr == nil {
						break
					}
				}
				if resolveErr != nil {
					return notFoundErr(fmt.Errorf("code file %q not found: %w", identifier, resolveErr))
				}
				raw, err = db.Get("code", resolved)
				if err != nil {
					return notFoundErr(fmt.Errorf("code file %q not found after resolve: %w", identifier, err))
				}
			}

			// Parse the stored JSON
			var obj map[string]any
			if err := json.Unmarshal(raw, &obj); err != nil {
				return fmt.Errorf("parsing stored code file: %w", err)
			}

			if flags.asJSON {
				return flags.printJSON(cmd, obj)
			}

			// Extract content from the data
			content, _ := obj["content"].(string)
			if content == "" {
				// Try nested data.content
				if data, ok := obj["data"].(map[string]any); ok {
					content, _ = data["content"].(string)
				}
			}
			if content == "" {
				// Try code field
				content, _ = obj["code"].(string)
			}

			if content == "" {
				return fmt.Errorf("no content field found in code file %q; use --json to inspect the full structure", identifier)
			}

			if outputFile != "" {
				if err := os.WriteFile(outputFile, []byte(content), 0o644); err != nil {
					return fmt.Errorf("writing output file: %w", err)
				}
				fmt.Fprintf(cmd.ErrOrStderr(), "Written to %s (%d bytes)\n", outputFile, len(content))
				return nil
			}

			fmt.Fprint(cmd.OutOrStdout(), content)
			return nil
		},
	}

	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Output file path (default: stdout)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/framer-pp-cli/data.db)")

	return cmd
}
