// PATCH: novel `db diff` — local supabase/migrations dir vs remote applied migration history. Composes /v1/projects/{ref}/database/migrations with a local dir walk.
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

var reMigrationVersion = regexp.MustCompile(`^(\d{8,})`)

func newDBDiffCmd(flags *rootFlags) *cobra.Command {
	var migrationsDir string

	cmd := &cobra.Command{
		Use:   "diff <ref>",
		Short: "Show local migrations not yet applied to the remote project",
		Long: `Compare the local supabase/migrations/ directory against the remote applied
migration history (/v1/projects/{ref}/database/migrations) and report which
local migration versions are unapplied. The pre-deploy "what would push do"
view, without invoking the official CLI.`,
		Example:     `  supabase-pp-cli db diff abcdefgh --json
  supabase-pp-cli db diff abcdefgh --dir ./supabase/migrations`,
		Annotations: map[string]string{"pp:method": "GET", "pp:path": "/v1/projects/{ref}/database/migrations", "mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			ref := args[0]
			if migrationsDir == "" {
				migrationsDir = "supabase/migrations"
			}

			localVersions, localErr := readLocalMigrations(migrationsDir)
			if localErr != nil {
				return localErr
			}

			if dryRunOK(flags) {
				return nil
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			path := replacePathParam("/v1/projects/{ref}/database/migrations", "ref", ref)
			data, gerr := c.Get(path, nil)
			if gerr != nil {
				return classifyAPIError(gerr, flags)
			}
			remoteVersions := parseRemoteVersions(data)

			remoteSet := map[string]bool{}
			for _, v := range remoteVersions {
				remoteSet[v] = true
			}
			var unapplied []string
			for v := range localVersions {
				if !remoteSet[v] {
					unapplied = append(unapplied, v)
				}
			}
			sort.Strings(unapplied)

			out := cmd.OutOrStdout()
			if flags.asJSON || (!isTerminal(out) && !flags.csv && !flags.quiet && !flags.plain) {
				payload := map[string]any{
					"project_ref":      ref,
					"migrations_dir":   migrationsDir,
					"local_count":      len(localVersions),
					"remote_count":     len(remoteVersions),
					"unapplied_count":  len(unapplied),
					"unapplied":        unapplied,
				}
				return printJSONFiltered(out, payload, flags)
			}
			if len(unapplied) == 0 {
				fmt.Fprintf(out, "In sync: %d local migration(s) all applied to %s.\n", len(localVersions), ref)
				return nil
			}
			fmt.Fprintf(out, "%d unapplied local migration(s) for %s:\n\n", len(unapplied), ref)
			for _, v := range unapplied {
				fmt.Fprintf(out, "  %s  %s\n", v, localVersions[v])
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&migrationsDir, "dir", "", "Local migrations directory (default: supabase/migrations)")
	return cmd
}

// readLocalMigrations walks the migrations dir, returning version -> filename.
func readLocalMigrations(dir string) (map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, notFoundErr(fmt.Errorf("migrations dir %q not found; pass --dir or run from a project root", dir))
		}
		return nil, fmt.Errorf("reading migrations dir: %w", err)
	}
	out := map[string]string{}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		base := filepath.Base(e.Name())
		if m := reMigrationVersion.FindStringSubmatch(base); len(m) >= 2 {
			out[m[1]] = base
		}
	}
	return out, nil
}

// parseRemoteVersions extracts migration versions from the API response, which
// is an array of objects each carrying a "version" field.
func parseRemoteVersions(data json.RawMessage) []string {
	var items []map[string]any
	if json.Unmarshal(data, &items) != nil {
		// Some responses wrap in {"migrations":[...]} or {"data":[...]}.
		var wrapped struct {
			Migrations []map[string]any `json:"migrations"`
			Data       []map[string]any `json:"data"`
		}
		if json.Unmarshal(data, &wrapped) == nil {
			if len(wrapped.Migrations) > 0 {
				items = wrapped.Migrations
			} else {
				items = wrapped.Data
			}
		}
	}
	var out []string
	for _, it := range items {
		if v, ok := it["version"].(string); ok && v != "" {
			out = append(out, v)
		}
	}
	return out
}
