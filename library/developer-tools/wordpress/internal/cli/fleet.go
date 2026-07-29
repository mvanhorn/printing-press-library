// pp:data-source local
// Copyright 2026 bobe and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/wordpress/internal/config"
	"github.com/mvanhorn/printing-press-library/library/developer-tools/wordpress/internal/store"

	"github.com/spf13/cobra"
)

var fleetUnsafeSiteChars = regexp.MustCompile(`[^a-z0-9-]+`)

type fleetSiteResult struct {
	Name           string `json:"name"`
	URL            string `json:"url"`
	Status         string `json:"status"`
	Message        string `json:"message,omitempty"`
	LastSync       string `json:"last_sync,omitempty"`
	LastSyncAge    string `json:"last_sync_age,omitempty"`
	Posts          *int   `json:"posts,omitempty"`
	Pages          *int   `json:"pages,omitempty"`
	Media          *int   `json:"media,omitempty"`
	Users          *int   `json:"users,omitempty"`
	Administrators *int   `json:"administrators,omitempty"`
	ActivePlugins  *int   `json:"active_plugins,omitempty"`
	TotalPlugins   *int   `json:"total_plugins,omitempty"`
	ActiveTheme    string `json:"active_theme,omitempty"`
	sortKey        string
}

type fleetSiteError struct {
	Site    string `json:"site"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

type fleetTotals struct {
	ReadableSites  int `json:"readable_sites"`
	Posts          int `json:"posts"`
	Pages          int `json:"pages"`
	Media          int `json:"media"`
	Users          int `json:"users"`
	Administrators int `json:"administrators"`
	ActivePlugins  int `json:"active_plugins"`
	TotalPlugins   int `json:"total_plugins"`
}

type fleetOutput struct {
	Sites           []fleetSiteResult `json:"sites"`
	Errors          []fleetSiteError  `json:"errors"`
	ConfiguredSites int               `json:"configured_sites"`
	Totals          fleetTotals       `json:"totals"`
}

// newNovelFleetCmd keeps compatibility with older generated root wiring. The
// additive hook in queue.go replaces this instance with the durable command.
func newNovelFleetCmd(flags *rootFlags) *cobra.Command {
	return newFleetCmd(flags)
}

func newFleetCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fleet [--json]",
		Short: "Roll up every configured site's local mirror",
		Long:  "Use this command for a cross-site rollup of every site already in the local store. Do NOT use this command to investigate why one specific site is failing or unreachable; use 'diagnose' instead.",
		Example: "  wordpress-pp-cli fleet --json\n" +
			"  wordpress-pp-cli fleet --agent",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			// No bare-invocation help branch: `fleet` takes no required input,
			// so running it bare must produce the rollup, matching the
			// framework's own zero-arg list commands (e.g. `profile list`).
			if len(args) != 0 {
				return usageErr(fmt.Errorf("fleet accepts flags only"))
			}
			if strings.EqualFold(strings.TrimSpace(flags.dataSource), "live") {
				return usageErr(fmt.Errorf("fleet has no live equivalent; sync each site and use its local mirror"))
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would query local store")
				return nil
			}

			registry, err := config.LoadSites(flags.configPath)
			if err != nil {
				return configErr(fmt.Errorf("load WordPress site registry: %w", err))
			}
			if registry == nil {
				return configErr(fmt.Errorf("load WordPress site registry: empty registry"))
			}
			anchorDBPath := wordpressDBPath(flags)
			activeSiteName := registry.Active
			if active, ok := registry.Sites[registry.Active]; ok && strings.TrimSpace(active.Name) != "" {
				activeSiteName = active.Name
			}

			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			sites := make([]fleetSiteResult, 0, len(registry.Sites))
			errors := make([]fleetSiteError, 0)
			totals := fleetTotals{}
			for registryKey, site := range registry.Sites {
				name := strings.TrimSpace(site.Name)
				if name == "" {
					name = registryKey
				}
				result := fleetSiteResult{
					Name:    name,
					URL:     fleetSiteURL(site),
					Status:  "ok",
					sortKey: registryKey,
				}
				dbPath := fleetSiteDBPath(anchorDBPath, activeSiteName, name, registryKey == registry.Active)
				if _, statErr := os.Stat(dbPath); os.IsNotExist(statErr) {
					message := fmt.Sprintf("no local mirror at %s", dbPath)
					fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s\nrun: wordpress-pp-cli sync --resources posts,pages,media --db %s\n", dbPath, dbPath)
					result.Status = "no-mirror"
					result.Message = message
					sites = append(sites, result)
					errors = append(errors, fleetSiteError{Site: name, Status: result.Status, Message: message})
					continue
				} else if statErr != nil {
					message := fmt.Sprintf("inspect local mirror %s: %v", dbPath, statErr)
					result.Status = "error"
					result.Message = message
					sites = append(sites, result)
					errors = append(errors, fleetSiteError{Site: name, Status: result.Status, Message: message})
					continue
				}

				db, openErr := store.OpenWithContext(ctx, dbPath)
				if openErr != nil {
					message := fmt.Sprintf("open local mirror %s: %v", dbPath, openErr)
					result.Status = "error"
					result.Message = message
					sites = append(sites, result)
					errors = append(errors, fleetSiteError{Site: name, Status: result.Status, Message: message})
					continue
				}
				if !hintIfUnsynced(cmd, db, "posts") {
					hintIfStale(cmd, db, "posts", flags.maxAge)
				}

				readErr := populateFleetSite(ctx, db, &result)
				closeErr := db.Close()
				if readErr == nil && closeErr != nil {
					readErr = fmt.Errorf("close local mirror: %w", closeErr)
				}
				if readErr != nil {
					result.Status = "error"
					result.Message = readErr.Error()
					clearFleetCounts(&result)
					sites = append(sites, result)
					errors = append(errors, fleetSiteError{Site: name, Status: result.Status, Message: result.Message})
					continue
				}
				addFleetTotals(&totals, result)
				sites = append(sites, result)
			}

			sort.SliceStable(sites, func(i, j int) bool {
				left := strings.ToLower(sites[i].Name)
				right := strings.ToLower(sites[j].Name)
				if left == right {
					return sites[i].sortKey < sites[j].sortKey
				}
				return left < right
			})
			sort.SliceStable(errors, func(i, j int) bool {
				return strings.ToLower(errors[i].Site) < strings.ToLower(errors[j].Site)
			})
			output := fleetOutput{
				Sites:           sites,
				Errors:          errors,
				ConfiguredSites: len(registry.Sites),
				Totals:          totals,
			}
			if flags.asJSON || flags.agent {
				return printJSONFiltered(cmd.OutOrStdout(), output, flags)
			}
			return printFleetTable(cmd, output)
		},
	}
	return cmd
}

func fleetSiteURL(site config.Site) string {
	if strings.TrimSpace(site.SiteURL) != "" {
		return site.SiteURL
	}
	return site.BaseURL
}

func fleetSiteDBPath(anchorPath, activeSiteName, siteName string, active bool) string {
	if active {
		return anchorPath
	}
	extension := filepath.Ext(anchorPath)
	stem := strings.TrimSuffix(anchorPath, extension)
	activeSlug := sanitizeFleetSiteName(activeSiteName)
	if activeSlug != "" {
		stem = strings.TrimSuffix(stem, "-"+activeSlug)
	}
	return stem + "-" + sanitizeFleetSiteName(siteName) + extension
}

func sanitizeFleetSiteName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	name = fleetUnsafeSiteChars.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if name == "" {
		return "site"
	}
	return name
}

func populateFleetSite(ctx context.Context, db *store.Store, result *fleetSiteResult) error {
	counts := make(map[string]int, 6)
	for _, resourceType := range []string{"posts", "pages", "media", "users"} {
		count, err := fleetResourceCount(ctx, db, resourceType)
		if err != nil {
			return err
		}
		counts[resourceType] = count
	}
	result.Posts = intPointer(counts["posts"])
	result.Pages = intPointer(counts["pages"])
	result.Media = intPointer(counts["media"])
	result.Users = intPointer(counts["users"])

	var administrators int
	if err := db.DB().QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM resources
		WHERE resource_type IN ('users')
		  AND EXISTS (
			SELECT 1
			FROM json_each(COALESCE(json_extract(data, '$.roles'), '[]'))
			WHERE value = 'administrator'
		  )`).Scan(&administrators); err != nil {
		return fmt.Errorf("count administrators: %w", err)
	}
	result.Administrators = intPointer(administrators)

	var activePlugins int
	var totalPlugins int
	if err := db.DB().QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN COALESCE(json_extract(data, '$.status'), '') = 'active' THEN 1 ELSE 0 END), 0),
			COUNT(*)
		FROM resources
		WHERE resource_type IN ('plugins')`).Scan(&activePlugins, &totalPlugins); err != nil {
		return fmt.Errorf("count plugins: %w", err)
	}
	result.ActivePlugins = intPointer(activePlugins)
	result.TotalPlugins = intPointer(totalPlugins)

	var activeTheme sql.NullString
	err := db.DB().QueryRowContext(ctx, `
		SELECT COALESCE(
			json_extract(data, '$.name.rendered'),
			json_extract(data, '$.name'),
			json_extract(data, '$.stylesheet'),
			''
		)
		FROM resources
		WHERE resource_type IN ('themes')
		  AND COALESCE(json_extract(data, '$.status'), '') = 'active'
		LIMIT 1`).Scan(&activeTheme)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("read active theme: %w", err)
	}
	if activeTheme.Valid {
		result.ActiveTheme = activeTheme.String
	}

	result.LastSync = latestFleetSync(db)
	result.LastSyncAge = fleetSyncAge(result.LastSync, time.Now())
	return nil
}

func fleetResourceCount(ctx context.Context, db *store.Store, resourceType string) (int, error) {
	var count int
	if err := db.DB().QueryRowContext(ctx, `SELECT COUNT(*) FROM resources WHERE resource_type IN (?)`, resourceType).Scan(&count); err != nil {
		return 0, fmt.Errorf("count %s: %w", resourceType, err)
	}
	return count, nil
}

func latestFleetSync(db *store.Store) string {
	latestRaw := ""
	var latest time.Time
	for _, resourceType := range []string{"posts", "pages", "media", "users", "plugins", "themes"} {
		raw := strings.TrimSpace(db.GetLastSyncedAt(resourceType))
		if raw == "" {
			continue
		}
		parsed, ok := parseWordPressTime(raw, time.Local)
		if !ok {
			if latestRaw == "" {
				latestRaw = raw
			}
			continue
		}
		if latest.IsZero() || parsed.After(latest) {
			latest = parsed
			latestRaw = raw
		}
	}
	return latestRaw
}

func fleetSyncAge(lastSync string, now time.Time) string {
	lastSync = strings.TrimSpace(lastSync)
	if lastSync == "" {
		return "never"
	}
	parsed, ok := parseWordPressTime(lastSync, now.Location())
	if !ok {
		return "unknown"
	}
	age := now.Sub(parsed)
	if age < 0 {
		age = 0
	}
	if age < time.Minute {
		return "<1m"
	}
	if age < time.Hour {
		return fmt.Sprintf("%dm", int(age/time.Minute))
	}
	if age < 24*time.Hour {
		return fmt.Sprintf("%dh", int(age/time.Hour))
	}
	return fmt.Sprintf("%dd", int(age/(24*time.Hour)))
}

func intPointer(value int) *int {
	copy := value
	return &copy
}

func addFleetTotals(totals *fleetTotals, site fleetSiteResult) {
	totals.ReadableSites++
	totals.Posts += *site.Posts
	totals.Pages += *site.Pages
	totals.Media += *site.Media
	totals.Users += *site.Users
	totals.Administrators += *site.Administrators
	totals.ActivePlugins += *site.ActivePlugins
	totals.TotalPlugins += *site.TotalPlugins
}

func clearFleetCounts(site *fleetSiteResult) {
	site.Posts = nil
	site.Pages = nil
	site.Media = nil
	site.Users = nil
	site.Administrators = nil
	site.ActivePlugins = nil
	site.TotalPlugins = nil
	site.ActiveTheme = ""
	site.LastSync = ""
	site.LastSyncAge = ""
}

func printFleetTable(cmd *cobra.Command, output fleetOutput) error {
	rows := make([]map[string]any, 0, len(output.Sites))
	for _, site := range output.Sites {
		rows = append(rows, map[string]any{
			"site":           site.Name,
			"url":            site.URL,
			"status":         site.Status,
			"last_sync":      site.LastSync,
			"sync_age":       site.LastSyncAge,
			"posts":          fleetOptionalInt(site.Posts),
			"pages":          fleetOptionalInt(site.Pages),
			"media":          fleetOptionalInt(site.Media),
			"users":          fleetOptionalInt(site.Users),
			"administrators": fleetOptionalInt(site.Administrators),
			"active_plugins": fleetOptionalInt(site.ActivePlugins),
			"total_plugins":  fleetOptionalInt(site.TotalPlugins),
			"active_theme":   site.ActiveTheme,
			"message":        site.Message,
		})
	}
	return printAutoTable(cmd.OutOrStdout(), rows)
}

func fleetOptionalInt(value *int) any {
	if value == nil {
		return "-"
	}
	return *value
}
