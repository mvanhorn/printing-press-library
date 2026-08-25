// Copyright 2026 SomSamantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: schema import — the counterpart to schema export. Reads a
// bundle produced by `schema export` and creates any collections that don't
// already exist on the target cluster. Existing collections are skipped
// (Weaviate has no config-merge endpoint; overwriting a live collection's
// schema safely would require a delete+recreate that this CLI will not do
// silently).

package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// pp:data-source live
func newNovelSchemaImportCmd(flags *rootFlags) *cobra.Command {
	var flagInput string

	cmd := &cobra.Command{
		Use:     "import <bundle-file>",
		Short:   "Create collections from a bundle produced by 'schema export'. Existing collections are skipped.",
		Example: "  weaviate-collections-pp-cli schema import schema-bundle.json",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && flagInput == "" && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			path := flagInput
			if path == "" && len(args) > 0 {
				path = args[0]
			}
			if dryRunOK(flags) {
				fmt.Fprintf(cmd.OutOrStdout(), "would import collections from %s\n", path)
				return nil
			}
			if path == "" {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("bundle file path is required"))
			}

			// #nosec G304 -- path is a user-supplied CLI argument (positional
			// arg or --input flag) naming a local bundle file the invoking
			// user already has filesystem access to; this is standard CLI
			// file-argument handling, not attacker-controlled input.
			raw, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("reading bundle %s: %w", path, err)
			}
			var bundle schemaBundle
			if err := json.Unmarshal(raw, &bundle); err != nil {
				return fmt.Errorf("parsing bundle %s: %w", path, err)
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			existing, err := fetchAllClasses(ctx, flags)
			if err != nil {
				return err
			}
			existingNames := make(map[string]bool, len(existing))
			for _, c := range existing {
				existingNames[classNameOf(c)] = true
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}

			type importResult struct {
				Class  string `json:"class"`
				Status string `json:"status"`
				Error  string `json:"error,omitempty"`
			}
			results := make([]importResult, 0, len(bundle.Classes))
			for _, cls := range bundle.Classes {
				name := classNameOf(cls)
				if name == "" {
					continue
				}
				if existingNames[name] {
					results = append(results, importResult{Class: name, Status: "skipped-exists"})
					continue
				}
				_, status, postErr := c.PostWithParams(ctx, "/schema", nil, cls)
				if postErr != nil || status < 200 || status >= 300 {
					msg := ""
					if postErr != nil {
						msg = postErr.Error()
					} else {
						msg = fmt.Sprintf("HTTP %d", status)
					}
					results = append(results, importResult{Class: name, Status: "failed", Error: msg})
					continue
				}
				results = append(results, importResult{Class: name, Status: "created"})
			}

			created, skipped, failed := 0, 0, 0
			for _, r := range results {
				switch r.Status {
				case "created":
					created++
				case "skipped-exists":
					skipped++
				case "failed":
					failed++
				}
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				for _, r := range results {
					if r.Error != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "%s: %s (%s)\n", r.Class, r.Status, r.Error)
					} else {
						fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", r.Class, r.Status)
					}
				}
				fmt.Fprintf(cmd.OutOrStdout(), "\n%d created, %d skipped (already exist), %d failed\n", created, skipped, failed)
				if failed > 0 {
					return fmt.Errorf("%d collection(s) failed to import", failed)
				}
				return nil
			}
			if err := printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"results": results,
				"created": created,
				"skipped": skipped,
				"failed":  failed,
			}, flags); err != nil {
				return err
			}
			if failed > 0 {
				return fmt.Errorf("%d collection(s) failed to import", failed)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&flagInput, "input", "", "Bundle file to import (alternative to positional arg)")
	return cmd
}
