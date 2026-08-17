// PATCH: novel auth-admin lookup/recent commands wrapping Supabase Auth Admin endpoints with optional PostgREST context-table join.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/supabase/internal/store"
	"github.com/spf13/cobra"
)

// newAuthAdminCmd is the top-level `auth-admin` parent — Supabase Auth Admin
// surface for user CRUD across a project. Distinct from the framework `auth`
// command (which manages this CLI's own credential state).
//
// All subcommands require SUPABASE_SERVICE_ROLE_KEY (or SUPABASE_SECRET_KEY) —
// Auth Admin endpoints reject the publishable key.
func newAuthAdminCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth-admin",
		Short: "Supabase Auth Admin: user lookup + cross-project recent signups",
		Long: `Auth Admin surface that the official supabase CLI and the supabase-community
MCP don't cover. Requires SUPABASE_SERVICE_ROLE_KEY (or SUPABASE_SECRET_KEY)
since these endpoints bypass RLS and operate on the auth.users system table.`,
	}
	cmd.AddCommand(newAuthAdminLookupCmd(flags))
	cmd.AddCommand(newAuthAdminRecentCmd(flags))
	return cmd
}

func newAuthAdminLookupCmd(flags *rootFlags) *cobra.Command {
	var contextTable string
	var contextKey string
	var dbPath string

	cmd := &cobra.Command{
		Use:   "lookup <email>",
		Short: "Look up an Auth user by email; optionally join their row from a PostgREST context table",
		Long: `Traverses Auth Admin GET /auth/v1/admin/users with the documented page and
per_page parameters, then requires exactly one normalized email match before returning
any user record. Zero, duplicate, malformed, repeated, truncated, provider-error, and
page-limit states fail closed without printing unrelated users. If --context-table is
given, the exact matched user's row is joined through PostgREST. Requires service_role.`,
		Example: strings.Trim(`
  # Just the Auth user
  supabase-pp-cli auth-admin lookup user@example.com --json

  # User + their profile row from a 'profiles' table
  supabase-pp-cli auth-admin lookup user@example.com --context-table profiles --context-key user_id --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			email := strings.TrimSpace(args[0])
			if email == "" || !strings.Contains(email, "@") {
				return usageErr(fmt.Errorf("first argument must be an email (got %q)", args[0]))
			}

			ps, err := newProjectSurface(true) // requireSecret = true
			if err != nil {
				return err
			}

			out := cmd.OutOrStdout()

			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
			defer cancel()
			matchedUser, err := lookupAuthAdminUser(ctx, ps, email, authAdminLookupOptions{
				PerPage:  authAdminLookupPerPage,
				MaxPages: authAdminLookupMaxPages,
			})
			if err != nil {
				if errors.Is(err, errAuthAdminUserNotFound) {
					return notFoundErr(err)
				}
				return apiErr(err)
			}

			var user map[string]any
			if err := json.Unmarshal(matchedUser, &user); err != nil {
				return fmt.Errorf("decoding user record: %w", err)
			}
			userID, _ := user["id"].(string)

			result := map[string]any{
				"email": email,
				"found": true,
				"user":  user,
			}

			// Optional context join via PostgREST
			if contextTable != "" {
				if userID == "" {
					return apiErr(fmt.Errorf("Auth user record missing 'id' field; cannot join %s", contextTable))
				}
				key := contextKey
				if key == "" {
					key = "user_id"
				}
				pgPath := fmt.Sprintf("/rest/v1/%s?%s=eq.%s",
					url.PathEscape(contextTable),
					url.QueryEscape(key),
					url.QueryEscape(userID),
				)
				pgBody, _, pgErr := ps.do(ctx, "GET", pgPath, nil, true) // service_role for cross-RLS read
				if pgErr != nil {
					// Surface in result but don't fail the lookup
					result["context_error"] = pgErr.Error()
				} else {
					var rows []map[string]any
					if uerr := json.Unmarshal(pgBody, &rows); uerr == nil {
						result["context"] = rows
						result["context_count"] = len(rows)
					} else {
						result["context_error"] = "PostgREST returned non-array body"
						result["context_raw"] = string(pgBody)
					}
				}
				result["context_table"] = contextTable
				result["context_key"] = key
			}

			if flags.asJSON {
				return printJSONFiltered(out, result, flags)
			}
			fmt.Fprintf(out, "User: %s (id=%s)\n", email, userID)
			if v, ok := user["created_at"]; ok {
				fmt.Fprintf(out, "  created_at:      %v\n", v)
			}
			if v, ok := user["last_sign_in_at"]; ok {
				fmt.Fprintf(out, "  last_sign_in_at: %v\n", v)
			}
			if v, ok := user["email_confirmed_at"]; ok {
				fmt.Fprintf(out, "  email_confirmed: %v\n", v)
			}
			if contextTable != "" {
				if ctxRows, ok := result["context"].([]map[string]any); ok {
					fmt.Fprintf(out, "\nContext from %s (%d row(s)):\n", contextTable, len(ctxRows))
					for i, row := range ctxRows {
						b, _ := json.MarshalIndent(row, "  ", "  ")
						fmt.Fprintf(out, "  [%d] %s\n", i, string(b))
					}
				} else if errStr, ok := result["context_error"].(string); ok {
					fmt.Fprintf(out, "\nContext fetch failed: %s\n", errStr)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&contextTable, "context-table", "", "PostgREST table to join on user_id (e.g., profiles, memberships)")
	cmd.Flags().StringVar(&contextKey, "context-key", "", "Column in --context-table to match user.id against (default: user_id)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (unused for live lookups)")
	return cmd
}

const (
	authAdminLookupPerPage  = 1000
	authAdminLookupMaxPages = 100
)

var errAuthAdminUserNotFound = errors.New("Auth user not found")

type authAdminLookupOptions struct {
	PerPage  int
	MaxPages int
}

// lookupAuthAdminUser traverses the documented Auth Admin list-users surface.
// It retains only the exact matching raw record and never includes unrelated
// records or provider response bodies in returned errors.
func lookupAuthAdminUser(ctx context.Context, ps *projectSurface, email string, options authAdminLookupOptions) (json.RawMessage, error) {
	normalizedEmail := strings.ToLower(strings.TrimSpace(email))
	if normalizedEmail == "" || options.PerPage < 1 || options.MaxPages < 1 {
		return nil, fmt.Errorf("invalid Auth Admin lookup configuration")
	}

	var expectedTotal int
	totalInitialized := false
	seenIDs := make(map[string]struct{})
	var matched json.RawMessage
	scanned := 0

	for page := 1; page <= options.MaxPages; page++ {
		authPath := fmt.Sprintf("/auth/v1/admin/users?page=%d&per_page=%d", page, options.PerPage)
		body, status, headers, err := ps.doWithHeaders(ctx, "GET", authPath, nil, true)
		if err != nil {
			return nil, fmt.Errorf("Auth Admin list-users request failed on page %d (HTTP %d)", page, status)
		}

		totalText := strings.TrimSpace(headers.Get("x-total-count"))
		total, parseErr := strconv.Atoi(totalText)
		if parseErr != nil || total < 0 {
			return nil, fmt.Errorf("Auth Admin list-users returned invalid pagination metadata on page %d", page)
		}
		if !totalInitialized {
			expectedTotal = total
			totalInitialized = true
			if expectedTotal > options.PerPage*options.MaxPages {
				return nil, fmt.Errorf("Auth Admin list-users traversal exceeds the %d-page safety limit", options.MaxPages)
			}
		} else if total != expectedTotal {
			return nil, fmt.Errorf("Auth Admin list-users pagination total changed during traversal")
		}

		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(body, &envelope); err != nil {
			return nil, fmt.Errorf("Auth Admin list-users returned a malformed page %d", page)
		}
		usersRaw, ok := envelope["users"]
		if !ok || string(usersRaw) == "null" {
			return nil, fmt.Errorf("Auth Admin list-users returned a malformed page %d", page)
		}
		var users []json.RawMessage
		if err := json.Unmarshal(usersRaw, &users); err != nil || len(users) > options.PerPage {
			return nil, fmt.Errorf("Auth Admin list-users returned a malformed page %d", page)
		}
		if len(users) == 0 && scanned < expectedTotal {
			return nil, fmt.Errorf("Auth Admin list-users traversal ended before all users were scanned")
		}

		for _, rawUser := range users {
			var identity struct {
				ID    string `json:"id"`
				Email string `json:"email"`
			}
			if err := json.Unmarshal(rawUser, &identity); err != nil || strings.TrimSpace(identity.ID) == "" || strings.TrimSpace(identity.Email) == "" {
				return nil, fmt.Errorf("Auth Admin list-users returned a malformed user record on page %d", page)
			}
			if _, duplicate := seenIDs[identity.ID]; duplicate {
				return nil, fmt.Errorf("Auth Admin list-users repeated a user during pagination")
			}
			seenIDs[identity.ID] = struct{}{}
			scanned++
			if scanned > expectedTotal {
				return nil, fmt.Errorf("Auth Admin list-users returned more users than its pagination total")
			}
			if strings.ToLower(strings.TrimSpace(identity.Email)) == normalizedEmail {
				if len(matched) != 0 {
					return nil, fmt.Errorf("Auth Admin lookup found multiple users with the requested email")
				}
				matched = append(json.RawMessage(nil), rawUser...)
			}
		}

		if scanned == expectedTotal {
			if len(matched) == 0 {
				return nil, errAuthAdminUserNotFound
			}
			return matched, nil
		}
	}

	return nil, fmt.Errorf("Auth Admin list-users traversal did not complete within the page limit")
}

func newAuthAdminRecentCmd(flags *rootFlags) *cobra.Command {
	var since string
	var perPage int
	var dbPath string
	var maxProjects int

	cmd := &cobra.Command{
		Use:   "recent",
		Short: "Cross-project Auth signups within a time window",
		Long: `Iterates locally-synced projects and calls Auth Admin GET /auth/v1/admin/users
against each one (requires service_role per project — note: the user's
SUPABASE_SERVICE_ROLE_KEY env var is reused for every project; in practice this
only works for projects sharing the same key, typically self-hosted setups or
single-project users). Aggregates users created_at within --since window.`,
		Example: strings.Trim(`
  # Signups across all synced projects in the last 7 days
  supabase-pp-cli auth-admin recent --since 7d --json

  # Last 24 hours, paginated to 200 per project
  supabase-pp-cli auth-admin recent --since 24h --per-page 200 --json
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			d, err := parseDayDuration(since)
			if err != nil {
				return usageErr(err)
			}
			cutoff := time.Now().Add(-d).UTC()

			if dbPath == "" {
				dbPath = defaultDBPath("supabase-pp-cli")
			}
			db, err := store.OpenWithContext(cmd.Context(), dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'supabase-pp-cli sync' first.", err)
			}
			defer db.Close()

			ps, err := newProjectSurface(true)
			if err != nil {
				return err
			}

			// Fetch synced project refs.
			rows, err := db.Query(`SELECT COALESCE(ref, '') AS ref, COALESCE(name, '') AS name FROM projects WHERE ref != '' ORDER BY name LIMIT ?`, maxProjects)
			if err != nil {
				return fmt.Errorf("listing projects: %w", err)
			}
			defer rows.Close()

			type projRef struct{ Ref, Name string }
			var projects []projRef
			for rows.Next() {
				var p projRef
				if err := rows.Scan(&p.Ref, &p.Name); err == nil {
					projects = append(projects, p)
				}
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterating projects: %w", err)
			}

			out := cmd.OutOrStdout()

			// If no projects in the store yet, fall back to the env-configured
			// single project (the user's own SUPABASE_URL).
			if len(projects) == 0 {
				projects = append(projects, projRef{Ref: ps.ProjectRef, Name: ps.ProjectRef + " (from SUPABASE_URL)"})
			}

			type recentUser struct {
				ID         string `json:"id"`
				Email      string `json:"email"`
				CreatedAt  string `json:"created_at"`
				ProjectRef string `json:"project_ref"`
			}
			var results []recentUser
			var errors []map[string]string

			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()

			for _, p := range projects {
				// Build a per-project base URL since SUPABASE_URL only points
				// at one project; for true multi-project fan-out the user
				// would need to rotate URLs. For now use the env URL for all
				// projects whose ref matches; otherwise note as unreachable.
				if p.Ref != ps.ProjectRef && len(projects) > 1 {
					errors = append(errors, map[string]string{
						"project_ref": p.Ref,
						"reason":      "SUPABASE_URL targets a different project; multi-project fan-out requires per-project credentials",
					})
					continue
				}
				// GoTrue /admin/users paginates via page/per_page and returns users
				// newest-first by created_at. Page until either the page returns
				// fewer than per_page (last page reached) or until every user on
				// a page is older than cutoff (window exhausted). maxPages caps
				// runaway iteration for very large projects.
				const maxPages = 50
				stopProject := false
				for page := 1; page <= maxPages && !stopProject; page++ {
					path := fmt.Sprintf("/auth/v1/admin/users?page=%d&per_page=%d", page, perPage)
					body, _, callErr := ps.do(ctx, "GET", path, nil, true)
					if callErr != nil {
						errors = append(errors, map[string]string{
							"project_ref": p.Ref,
							"reason":      callErr.Error(),
						})
						break
					}
					var resp struct {
						Users []map[string]any `json:"users"`
					}
					if err := json.Unmarshal(body, &resp); err != nil {
						errors = append(errors, map[string]string{
							"project_ref": p.Ref,
							"reason":      "non-JSON response",
						})
						break
					}
					inWindowOnPage := 0
					for _, u := range resp.Users {
						createdStr, _ := u["created_at"].(string)
						if createdStr == "" {
							continue
						}
						t, perr := time.Parse(time.RFC3339, createdStr)
						if perr != nil {
							continue
						}
						if t.Before(cutoff) {
							// Newest-first ordering means once we see a user
							// older than cutoff, every subsequent user on this
							// and later pages is also older — bail out.
							stopProject = true
							continue
						}
						inWindowOnPage++
						id, _ := u["id"].(string)
						email, _ := u["email"].(string)
						results = append(results, recentUser{
							ID:         id,
							Email:      email,
							CreatedAt:  createdStr,
							ProjectRef: p.Ref,
						})
					}
					if len(resp.Users) < perPage {
						break
					}
					if page == maxPages && inWindowOnPage > 0 {
						errors = append(errors, map[string]string{
							"project_ref": p.Ref,
							"reason":      fmt.Sprintf("pagination capped at %d pages (%d users scanned); window may include older signups not reported", maxPages, maxPages*perPage),
						})
					}
				}
			}

			if flags.asJSON {
				return printJSONFiltered(out, map[string]any{
					"since":    since,
					"cutoff":   cutoff.Format(time.RFC3339),
					"count":    len(results),
					"users":    results,
					"errors":   errors,
					"projects": len(projects),
				}, flags)
			}
			fmt.Fprintf(out, "Recent signups (since %s) across %d project(s): %d user(s)\n\n", since, len(projects), len(results))
			fmt.Fprintf(out, "%-40s %-25s %s\n", "EMAIL", "PROJECT_REF", "CREATED_AT")
			fmt.Fprintf(out, "%-40s %-25s %s\n", "-----", "-----------", "----------")
			for _, r := range results {
				fmt.Fprintf(out, "%-40s %-25s %s\n", truncate(r.Email, 38), truncate(r.ProjectRef, 23), r.CreatedAt)
			}
			if len(errors) > 0 {
				fmt.Fprintf(out, "\n%d project(s) skipped (multi-project fan-out limit):\n", len(errors))
				for _, e := range errors {
					fmt.Fprintf(out, "  %s: %s\n", e["project_ref"], truncate(e["reason"], 80))
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&since, "since", "7d", "Time window (e.g., 24h, 7d, 30d)")
	cmd.Flags().IntVar(&perPage, "per-page", 100, "Auth Admin per-page page size (max 1000)")
	cmd.Flags().StringVar(&dbPath, "db", "", "Database path (default: ~/.local/share/supabase-pp-cli/data.db)")
	cmd.Flags().IntVar(&maxProjects, "max-projects", 50, "Maximum synced projects to scan")
	return cmd
}
