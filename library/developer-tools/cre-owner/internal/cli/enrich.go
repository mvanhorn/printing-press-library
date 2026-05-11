package cli

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/cre-owner/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/cre-owner/internal/store"

	"github.com/spf13/cobra"
)

func newEnrichCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "enrich [owner-name-or-id]",
		Short: "Hand off to contact-goat-pp-cli for phone/email enrichment",
		Long: `Takes an owner name or ID, looks them up in the local store, and shells out
to contact-goat-pp-cli for phone/email enrichment if it is installed.
If contact-goat-pp-cli is not on PATH, prints installation instructions.`,
		Example: strings.Trim(`
  cre-owner-pp-cli enrich "John Smith"
  cre-owner-pp-cli enrich owner-uuid-123 --json`, "\n"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}

			ownerName := args[0]

			// Short-circuit in verify mode to avoid side effects
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would enrich:", ownerName)
				return nil
			}

			if dbPath == "" {
				dbPath = defaultDBPath("cre-owner-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()

			// Look up the owner in local store
			resolvedName, err := resolveOwnerName(db, ownerName)
			if err != nil {
				return err
			}

			// Check if contact-goat-pp-cli is available
			goatPath, err := exec.LookPath("contact-goat-pp-cli")
			if err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Owner: %s\n\n", resolvedName)
				fmt.Fprintln(cmd.OutOrStdout(), "Install contact-goat-pp-cli for phone/email enrichment:")
				fmt.Fprintln(cmd.OutOrStdout(), "  go install github.com/mvanhorn/contact-goat-pp-cli/cmd/contact-goat-pp-cli@latest")
				return nil
			}

			// Shell out to contact-goat-pp-cli
			enrichCmd := exec.CommandContext(cmd.Context(), goatPath, "search", "--name", resolvedName, "--json")
			enrichCmd.Stdout = cmd.OutOrStdout()
			enrichCmd.Stderr = cmd.ErrOrStderr()
			return enrichCmd.Run()
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path")
	return cmd
}

func resolveOwnerName(db *store.Store, input string) (string, error) {
	// If it looks like a UUID, resolve to the owner name
	if store.IsUUID(input) {
		data, err := db.Get("owners", input)
		if err != nil {
			return "", fmt.Errorf("looking up owner: %w", err)
		}
		if data == nil {
			return input, nil
		}
		obj := parseJSON(string(data))
		if obj != nil {
			name := extractStringField(obj, "name", "owner_name", "ownerName", "full_name")
			if name != "" {
				return name, nil
			}
		}
		return input, nil
	}
	// Verify the owner exists in store
	rows, err := db.DB().Query(
		`SELECT data FROM resources WHERE resource_type = 'owners'
		 AND (LOWER(json_extract(data, '$.name')) LIKE LOWER(?)
		   OR LOWER(json_extract(data, '$.owner_name')) LIKE LOWER(?)
		   OR LOWER(json_extract(data, '$.full_name')) LIKE LOWER(?))
		 LIMIT 1`,
		"%"+input+"%", "%"+input+"%", "%"+input+"%",
	)
	if err != nil {
		return input, nil
	}
	defer rows.Close()
	if rows.Next() {
		var data string
		if rows.Scan(&data) == nil {
			obj := parseJSON(data)
			if obj != nil {
				name := extractStringField(obj, "name", "owner_name", "ownerName", "full_name")
				if name != "" {
					return name, nil
				}
			}
		}
	}
	return input, nil
}
