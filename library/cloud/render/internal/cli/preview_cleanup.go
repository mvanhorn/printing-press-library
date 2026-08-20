// Copyright 2026 Giuliano Giacaglia and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/render/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/cloud/render/internal/store"

	"github.com/spf13/cobra"
)

// previewService is a single delete-candidate row in the cleanup plan.
type previewService struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Type      string `json:"type"`
	UpdatedAt string `json:"updated_at,omitempty"`
	AgeDays   int    `json:"age_days"`
}

// previewCleanupResult is the JSON envelope returned by --json. The Apply
// flag is always echoed so the caller can confirm whether the run was a
// dry plan or an actual delete pass.
type previewCleanupResult struct {
	StaleDays int              `json:"stale_days"`
	Apply     bool             `json:"apply"`
	Plan      []previewService `json:"plan"`
	Results   []map[string]any `json:"results,omitempty"`
}

func newPreviewCleanupCmd(flags *rootFlags) *cobra.Command {
	var (
		staleDays int
		apply     bool
		dbPath    string
	)
	cmd := &cobra.Command{
		Use:   "preview-cleanup",
		Short: "List preview services older than --stale-days; delete in bulk on --confirm.",
		Example: strings.Trim(`
  render-pp-cli preview-cleanup
  render-pp-cli preview-cleanup --stale-days 21
  render-pp-cli preview-cleanup --confirm
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run": true, "command": "preview-cleanup"}`)
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("render-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nlocal cache empty — run 'render-pp-cli sync' first", err)
			}
			defer db.Close()

			candidates, err := loadStalePreviewServices(db, staleDays)
			if err != nil {
				return err
			}
			result := previewCleanupResult{StaleDays: staleDays, Apply: apply, Plan: candidates}

			// Mutating path requires --apply AND not in verify env. The
			// verify-env short-circuit is the floor that catches any
			// classifier miss.
			if apply && !cliutil.IsVerifyEnv() {
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				for _, s := range candidates {
					path := "/services/" + s.ID
					_, status, err := c.Delete(path)
					result.Results = append(result.Results, map[string]any{
						"id":     s.ID,
						"status": status,
						"error":  errString(err),
					})
				}
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			if len(candidates) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No preview services older than %d days found.\n", staleDays)
				return nil
			}
			mode := "DRY-RUN"
			if apply && !cliutil.IsVerifyEnv() {
				mode = "APPLIED"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s — %d preview services older than %d days:\n", mode, len(candidates), staleDays)
			fmt.Fprintf(cmd.OutOrStdout(), "%-18s %-8s %-30s %-25s %s\n", "ID", "AGE", "NAME", "TYPE", "UPDATED")
			for _, s := range candidates {
				name := s.Name
				if len(name) > 28 {
					name = name[:25] + "..."
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%-18s %-8d %-30s %-25s %s\n", s.ID, s.AgeDays, name, s.Type, s.UpdatedAt)
			}
			if !apply {
				fmt.Fprintln(cmd.OutOrStdout(), "\nRe-run with --confirm to delete these services.")
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&staleDays, "stale-days", 14, "Preview services with no update for this many days are candidates for cleanup")
	cmd.Flags().BoolVar(&apply, "confirm", false, "Actually delete the candidates (default: print plan)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/render-pp-cli/data.db)")
	return cmd
}

// loadStalePreviewServices selects service rows whose data.type marks them
// as a preview, filtered by updatedAt < now - staleDays. Render's preview
// type values vary by service kind; we match a small set defensively.
func loadStalePreviewServices(db *store.Store, staleDays int) ([]previewService, error) {
	cutoff := time.Now().AddDate(0, 0, -staleDays)
	rows, err := db.DB().Query(`SELECT id, data FROM resources WHERE resource_type = 'services'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []previewService{}
	for rows.Next() {
		var id string
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			continue
		}
		typ := strFromAny(obj["type"])
		if !looksLikePreviewType(typ, obj) {
			continue
		}
		updated := strFromAny(obj["updatedAt"])
		if updated == "" {
			updated = strFromAny(obj["updated_at"])
		}
		t, err := time.Parse(time.RFC3339, updated)
		if err != nil {
			continue
		}
		if !t.Before(cutoff) {
			continue
		}
		out = append(out, previewService{
			ID:        id,
			Name:      strFromAny(obj["name"]),
			Type:      typ,
			UpdatedAt: updated,
			AgeDays:   int(time.Since(t).Hours() / 24),
		})
	}
	return out, rows.Err()
}

// looksLikePreviewType matches Render's various preview type values plus
// the boolean "isPreview" hint some endpoints expose.
func looksLikePreviewType(typ string, obj map[string]any) bool {
	switch typ {
	case "preview", "web_service_preview", "static_site_preview":
		return true
	}
	if strings.Contains(strings.ToLower(typ), "preview") {
		return true
	}
	if v, ok := obj["isPreview"].(bool); ok && v {
		return true
	}
	return false
}
