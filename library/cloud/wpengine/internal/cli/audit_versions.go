// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.
// Novel command: fleet-wide PHP/WordPress version distribution + drift.
// pp:data-source local

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type versionOutlierRow struct {
	Install     string `json:"install"`
	InstallID   string `json:"install_id"`
	Environment string `json:"environment,omitempty"`
	PHPVersion  string `json:"php_version,omitempty"`
	WPVersion   string `json:"wp_version,omitempty"`
}

type versionDriftRow struct {
	SiteID   string              `json:"site_id"`
	Site     string              `json:"site,omitempty"`
	Installs []versionOutlierRow `json:"installs"`
	Reason   string              `json:"reason"`
}

type auditVersionsView struct {
	PHPDistribution map[string]int      `json:"php_distribution"`
	WPDistribution  map[string]int      `json:"wp_distribution"`
	Outliers        []versionOutlierRow `json:"outliers,omitempty"`
	// UnknownVersionInstalls counts installs excluded from the outlier filter
	// because the relevant version field is empty in the mirror — they cannot
	// be proven above the threshold.
	UnknownVersionInstalls int               `json:"unknown_version_installs,omitempty"`
	Drift                  []versionDriftRow `json:"drift,omitempty"`
	ScannedInstalls        int               `json:"scanned_installs"`
}

func newNovelAuditVersionsCmd(flags *rootFlags) *cobra.Command {
	var flagPhpBelow string
	var flagWpBelow string
	var flagDrift bool
	var dbPath string

	cmd := &cobra.Command{
		Use:   "versions",
		Short: "Fleet-wide PHP and WordPress version distribution, outlier filters, and per-site drift",
		Long: strings.Trim(`
Aggregates php_version and wp_version across every synced install, fleet-wide.
--php-below / --wp-below list the installs running below a threshold.
--drift groups installs by site and flags sites whose environments disagree
on PHP or WordPress version (staging drifting from production).

Requires a synced mirror: run 'wpengine-pp-cli sync' first.
`, "\n"),
		Example: strings.Trim(`
  # Version distribution for the whole fleet
  wpengine-pp-cli audit versions

  # Which installs run PHP below 8.2
  wpengine-pp-cli audit versions --php-below 8.2 --agent

  # Sites whose staging and production versions disagree
  wpengine-pp-cli audit versions --drift
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would aggregate PHP/WP versions across the local mirror")
				return nil
			}
			if err := rejectLiveDataSource(flags); err != nil {
				return err
			}
			db, handled, err := openAuditStore(cmd, flags, dbPath, "installs", "{}")
			if err != nil || handled {
				return err
			}
			defer db.Close()

			ctx := cmd.Context()
			rows, err := db.DB().QueryContext(ctx, `
				SELECT id, COALESCE(name, ''), COALESCE(environment, ''),
				       COALESCE(php_version, ''), COALESCE(wp_version, ''), data
				FROM installs`)
			if err != nil {
				return fmt.Errorf("querying installs: %w", err)
			}
			type installRow struct {
				id, name, environment, php, wp string
				data                           []byte
			}
			installs := make([]installRow, 0)
			if err := drainRows(rows, "install", func(rs *sql.Rows) error {
				var r installRow
				if err := rs.Scan(&r.id, &r.name, &r.environment, &r.php, &r.wp, &r.data); err != nil {
					return err
				}
				installs = append(installs, r)
				return nil
			}); err != nil {
				return err
			}

			view := auditVersionsView{
				PHPDistribution: map[string]int{},
				WPDistribution:  map[string]int{},
				ScannedInstalls: len(installs),
			}
			for _, ins := range installs {
				php := ins.php
				if php == "" {
					php = "unknown"
				}
				wp := ins.wp
				if wp == "" {
					wp = "unknown"
				}
				view.PHPDistribution[php]++
				view.WPDistribution[wp]++
			}

			if flagPhpBelow != "" || flagWpBelow != "" {
				view.Outliers = make([]versionOutlierRow, 0)
				for _, ins := range installs {
					phpHit := flagPhpBelow != "" && versionBelow(ins.php, flagPhpBelow)
					wpHit := flagWpBelow != "" && versionBelow(ins.wp, flagWpBelow)
					if phpHit || wpHit {
						view.Outliers = append(view.Outliers, versionOutlierRow{
							Install: ins.name, InstallID: ins.id, Environment: ins.environment,
							PHPVersion: ins.php, WPVersion: ins.wp,
						})
					}
					// Unknown versions can't be compared; surface the count so
					// an empty outlier list is never mistaken for full coverage.
					if (flagPhpBelow != "" && ins.php == "") || (flagWpBelow != "" && ins.wp == "") {
						view.UnknownVersionInstalls++
					}
				}
				sort.Slice(view.Outliers, func(i, j int) bool { return view.Outliers[i].Install < view.Outliers[j].Install })
			}

			if flagDrift {
				bySite := map[string][]installRow{}
				for _, ins := range installs {
					var d struct {
						Site struct {
							ID string `json:"id"`
						} `json:"site"`
					}
					if err := json.Unmarshal(ins.data, &d); err != nil || d.Site.ID == "" {
						continue
					}
					bySite[d.Site.ID] = append(bySite[d.Site.ID], ins)
				}
				// The install payload carries only the site id; resolve names
				// from the synced sites table (installs rows are fully drained).
				// Best-effort name enrichment: a failed sites lookup degrades to
				// empty names rather than failing the drift report.
				siteNames := map[string]string{}
				if srows, err := db.DB().QueryContext(ctx, `SELECT id, COALESCE(name, '') FROM sites`); err == nil {
					_ = drainRows(srows, "site", func(rs *sql.Rows) error {
						var id, name string
						if err := rs.Scan(&id, &name); err != nil {
							return err
						}
						siteNames[id] = name
						return nil
					})
				}
				view.Drift = make([]versionDriftRow, 0)
				for siteID, members := range bySite {
					if len(members) < 2 {
						continue
					}
					phpSet := map[string]bool{}
					wpSet := map[string]bool{}
					for _, m := range members {
						if m.php != "" {
							phpSet[m.php] = true
						}
						if m.wp != "" {
							wpSet[m.wp] = true
						}
					}
					var reasons []string
					if len(phpSet) > 1 {
						reasons = append(reasons, "php_version differs across environments")
					}
					if len(wpSet) > 1 {
						reasons = append(reasons, "wp_version differs across environments")
					}
					if len(reasons) == 0 {
						continue
					}
					row := versionDriftRow{SiteID: siteID, Site: siteNames[siteID], Reason: strings.Join(reasons, "; ")}
					for _, m := range members {
						row.Installs = append(row.Installs, versionOutlierRow{
							Install: m.name, InstallID: m.id, Environment: m.environment,
							PHPVersion: m.php, WPVersion: m.wp,
						})
					}
					sort.Slice(row.Installs, func(i, j int) bool { return row.Installs[i].Environment < row.Installs[j].Environment })
					view.Drift = append(view.Drift, row)
				}
				sort.Slice(view.Drift, func(i, j int) bool { return view.Drift[i].SiteID < view.Drift[j].SiteID })
			}

			return printJSONFiltered(cmd.OutOrStdout(), view, flags)
		},
	}
	cmd.Flags().StringVar(&flagPhpBelow, "php-below", "", "List installs whose PHP version is below this threshold (e.g. 8.2)")
	cmd.Flags().StringVar(&flagWpBelow, "wp-below", "", "List installs whose WordPress version is below this threshold (e.g. 6.4)")
	cmd.Flags().BoolVar(&flagDrift, "drift", false, "Flag sites whose environments disagree on PHP or WP version")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: local data dir)")
	return cmd
}
