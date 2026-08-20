// Copyright 2026 Giuliano Giacaglia and contributors. Licensed under Apache-2.0. See LICENSE.
//
// PATCH: Replaces the generator's generic missing-assignee orphan scan with a
// Render-specific orphan sweep — unattached disks, empty env-groups, dangling
// custom-domains, unused registry credentials, retention-violating snapshots.
// See .printing-press-patches.json (id: render-orphans-domain-rewrite).

package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/cloud/render/internal/store"

	"github.com/spf13/cobra"
)

// orphanReport is the structured shape returned by --json. The named fields
// match what the prompt requested verbatim so downstream callers can rely
// on a stable schema.
type orphanReport struct {
	OrphanDisks               []orphanItem `json:"orphan_disks"`
	OrphanEnvGroups           []orphanItem `json:"orphan_env_groups"`
	OrphanDomains             []orphanItem `json:"orphan_domains"`
	OrphanRegistryCredentials []orphanItem `json:"orphan_registry_credentials"`
	StaleSnapshots            []orphanItem `json:"stale_snapshots"`
}

type orphanItem struct {
	ID      string `json:"id"`
	Name    string `json:"name,omitempty"`
	Reason  string `json:"reason"`
	AgeDays int    `json:"age_days,omitempty"`
}

func newOrphansCmd(flags *rootFlags) *cobra.Command {
	var (
		dbPath   string
		keepDays int
	)
	cmd := &cobra.Command{
		Use:   "orphans",
		Short: "Find Render-side orphan resources: disks, env-groups, custom-domains, registry credentials, stale snapshots.",
		Example: strings.Trim(`
  render-pp-cli orphans
  render-pp-cli orphans --json
  render-pp-cli orphans --keep-days 60
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run": true, "command": "orphans"}`)
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

			rep, err := scanOrphans(db, keepDays)
			if err != nil {
				return err
			}
			total := len(rep.OrphanDisks) + len(rep.OrphanEnvGroups) + len(rep.OrphanDomains) + len(rep.OrphanRegistryCredentials) + len(rep.StaleSnapshots)
			if total == 0 && countResources(db) == 0 {
				return fmt.Errorf("local cache empty — run 'render-pp-cli sync' first")
			}

			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), rep, flags)
			}
			renderOrphansText(cmd, rep, total)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/render-pp-cli/data.db)")
	cmd.Flags().IntVar(&keepDays, "keep-days", 90, "Snapshots older than this number of days are flagged stale")
	return cmd
}

// scanOrphans runs the five Render-specific orphan checks and returns the
// composite report. Each check is independent; one failure on a missing
// table doesn't kill the others, since some store schemas may not have
// every dependent table populated yet.
func scanOrphans(db *store.Store, keepDays int) (orphanReport, error) {
	rep := orphanReport{
		OrphanDisks:               []orphanItem{},
		OrphanEnvGroups:           []orphanItem{},
		OrphanDomains:             []orphanItem{},
		OrphanRegistryCredentials: []orphanItem{},
		StaleSnapshots:            []orphanItem{},
	}

	serviceIDs, err := loadResourceIDs(db, "services")
	if err != nil {
		return rep, err
	}
	serviceDiskIDs, _ := loadServiceDiskIDs(db)
	serviceRegistryIDs, _ := loadServiceRegistryCredentialIDs(db)

	// Disks: not referenced by any service's data.diskId
	disks, _ := loadResourceItems(db, "disks")
	for _, d := range disks {
		if !serviceDiskIDs[d.ID] {
			rep.OrphanDisks = append(rep.OrphanDisks, orphanItem{ID: d.ID, Name: d.Name, Reason: "not referenced by any service.diskId"})
		}
	}

	// Env-groups: not in any env_groups_services row
	usedGroupIDs := map[string]bool{}
	if rows, err := db.DB().Query(`SELECT DISTINCT env_groups_id FROM env_groups_services`); err == nil {
		defer rows.Close()
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err == nil {
				usedGroupIDs[id] = true
			}
		}
	}
	groups, _ := loadResourceItems(db, "env-groups")
	for _, g := range groups {
		if !usedGroupIDs[g.ID] {
			rep.OrphanEnvGroups = append(rep.OrphanEnvGroups, orphanItem{ID: g.ID, Name: g.Name, Reason: "not linked to any service via env_groups_services"})
		}
	}

	// Custom domains: services_id points at a service id we don't have
	if rows, err := db.DB().Query(`SELECT id, services_id, data FROM custom_domains`); err == nil {
		defer rows.Close()
		for rows.Next() {
			var id, svcID string
			var raw []byte
			if err := rows.Scan(&id, &svcID, &raw); err != nil {
				continue
			}
			if serviceIDs[svcID] {
				continue
			}
			var obj map[string]any
			_ = json.Unmarshal(raw, &obj)
			rep.OrphanDomains = append(rep.OrphanDomains, orphanItem{
				ID:     id,
				Name:   strFromAny(obj["name"]),
				Reason: fmt.Sprintf("references missing service %s", svcID),
			})
		}
	}

	// Registry credentials: not referenced by any service.imageRegistryCredentialId
	creds, _ := loadResourceItems(db, "registrycredentials")
	for _, c := range creds {
		if !serviceRegistryIDs[c.ID] {
			rep.OrphanRegistryCredentials = append(rep.OrphanRegistryCredentials, orphanItem{ID: c.ID, Name: c.Name, Reason: "not referenced by any service.imageRegistryCredentialId"})
		}
	}

	// Stale snapshots: any row in `snapshots` older than keep-days
	cutoff := time.Now().AddDate(0, 0, -keepDays)
	if rows, err := db.DB().Query(`SELECT id, data FROM snapshots`); err == nil {
		defer rows.Close()
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
			created := strFromAny(obj["createdAt"])
			if created == "" {
				created = strFromAny(obj["created_at"])
			}
			t, err := time.Parse(time.RFC3339, created)
			if err != nil {
				continue
			}
			if t.Before(cutoff) {
				age := int(time.Since(t).Hours() / 24)
				rep.StaleSnapshots = append(rep.StaleSnapshots, orphanItem{
					ID:      id,
					Reason:  fmt.Sprintf("older than %d days", keepDays),
					AgeDays: age,
				})
			}
		}
	}
	return rep, nil
}

// loadResourceIDs returns a set of ids for the given resource_type. Used
// to test foreign-key liveness for orphan detection.
func loadResourceIDs(db *store.Store, resType string) (map[string]bool, error) {
	out := map[string]bool{}
	rows, err := db.DB().Query(`SELECT id FROM resources WHERE resource_type = ?`, resType)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			out[id] = true
		}
	}
	return out, rows.Err()
}

type idAndName struct {
	ID   string
	Name string
}

// loadResourceItems pulls id+name pairs for the given resource_type. Name
// is best-effort from the data JSON; absent → empty string.
func loadResourceItems(db *store.Store, resType string) ([]idAndName, error) {
	out := []idAndName{}
	rows, err := db.DB().Query(`SELECT id, data FROM resources WHERE resource_type = ?`, resType)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var raw []byte
		if err := rows.Scan(&id, &raw); err != nil {
			return out, err
		}
		var obj map[string]any
		_ = json.Unmarshal(raw, &obj)
		out = append(out, idAndName{ID: id, Name: strFromAny(obj["name"])})
	}
	return out, rows.Err()
}

// loadServiceDiskIDs walks services and collects every diskId mentioned.
// Returns a set keyed by disk id.
func loadServiceDiskIDs(db *store.Store) (map[string]bool, error) {
	out := map[string]bool{}
	rows, err := db.DB().Query(`SELECT data FROM resources WHERE resource_type = 'services'`)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			continue
		}
		// disk id may live at root or under serviceDetails
		if id := strFromAny(obj["diskId"]); id != "" {
			out[id] = true
		}
		if details, ok := obj["serviceDetails"].(map[string]any); ok {
			if id := strFromAny(details["diskId"]); id != "" {
				out[id] = true
			}
			if disk, ok := details["disk"].(map[string]any); ok {
				if id := strFromAny(disk["id"]); id != "" {
					out[id] = true
				}
			}
		}
	}
	return out, rows.Err()
}

// loadServiceRegistryCredentialIDs collects every registry credential id
// referenced by a service.
func loadServiceRegistryCredentialIDs(db *store.Store) (map[string]bool, error) {
	out := map[string]bool{}
	rows, err := db.DB().Query(`SELECT data FROM resources WHERE resource_type = 'services'`)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			continue
		}
		if id := strFromAny(obj["imageRegistryCredentialId"]); id != "" {
			out[id] = true
		}
		if details, ok := obj["serviceDetails"].(map[string]any); ok {
			if id := strFromAny(details["imageRegistryCredentialId"]); id != "" {
				out[id] = true
			}
		}
	}
	return out, rows.Err()
}

// countResources returns total cached resources across all types. Used to
// distinguish "no orphans because nothing's cached" from "no orphans because
// the workspace is clean."
func countResources(db *store.Store) int {
	var n int
	_ = db.DB().QueryRow(`SELECT COUNT(*) FROM resources`).Scan(&n)
	return n
}

func renderOrphansText(cmd *cobra.Command, rep orphanReport, total int) {
	if total == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No orphans found.")
		return
	}
	section := func(label string, items []orphanItem) {
		if len(items) == 0 {
			return
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s (%d):\n", label, len(items))
		for _, it := range items {
			line := "  " + it.ID
			if it.Name != "" {
				line += " " + it.Name
			}
			line += " — " + it.Reason
			if it.AgeDays > 0 {
				line += fmt.Sprintf(" (%dd)", it.AgeDays)
			}
			fmt.Fprintln(cmd.OutOrStdout(), line)
		}
	}
	section("Orphan disks", rep.OrphanDisks)
	section("Orphan env-groups", rep.OrphanEnvGroups)
	section("Orphan custom domains", rep.OrphanDomains)
	section("Orphan registry credentials", rep.OrphanRegistryCredentials)
	section("Stale snapshots", rep.StaleSnapshots)
}
