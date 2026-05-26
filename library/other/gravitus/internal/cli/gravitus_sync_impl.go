package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/gravitus/internal/config"
	"github.com/mvanhorn/printing-press-library/library/other/gravitus/internal/dashboard"
	"github.com/mvanhorn/printing-press-library/library/other/gravitus/internal/scraper"
	_ "modernc.org/sqlite"
)

func newGravitusSyncCmd(flags *rootFlags) *cobra.Command {
	var userID string
	var dashboardDB string
	var fullSync bool
	var incremental bool
	var dbPath string
	var maxPages int

	cmd := &cobra.Command{
		Use:   "gravitus-sync",
		Short: "Sync Gravitus workouts to local store and dashboard database",
		Long: `Scrapes your Gravitus workout history and writes it to:
  1. The local CLI SQLite store (for analytics commands)
  2. Your training dashboard's dev.db (as LiftingSession records)

Requires authentication — run 'auth login-password' first.

The sync paginates through all workout pages (/users/{id}/?page=N), fetches
each workout detail page, parses exercises/sets/weights, and upserts records.`,
		Example: strings.Join([]string{
			`  # Full sync into dashboard`,
			`  gravitus-pp-cli gravitus-sync --dashboard-db ./prisma/dev.db`,
			``,
			`  # Incremental sync (only new workouts not already in dev.db)`,
			`  gravitus-pp-cli gravitus-sync --incremental --dashboard-db ./prisma/dev.db`,
			``,
			`  # Specify your Gravitus user ID explicitly`,
			`  gravitus-pp-cli gravitus-sync --user-id YOUR_USER_ID --dashboard-db ./prisma/dev.db`,
		}, "\n"),
		Annotations: map[string]string{},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}

			cfg, err := config.Load(flags.configPath)
			if err != nil {
				return configErr(err)
			}

			// Resolve user ID: flag > config > env
			if userID == "" {
				userID = cfg.UserID()
			}
			if userID == "" {
				return usageErr(fmt.Errorf("--user-id is required (or set via GRAVITUS_USER_ID env var, or run 'auth login-password' to auto-discover)"))
			}

			// Resolve dashboard DB path: flag > config > env
			if dashboardDB == "" {
				dashboardDB = cfg.DashboardDBPath()
			}

			// Open dashboard DB once — shared for validation, incremental checks,
			// and upserts to avoid a sql.Open/Close round-trip per workout.
			var dashDB *sql.DB
			if dashboardDB != "" {
				var dbErr error
				dashDB, dbErr = dashboard.OpenDB(dashboardDB)
				if dbErr != nil {
					return fmt.Errorf("cannot open dashboard db %s: %w", dashboardDB, dbErr)
				}
				defer dashDB.Close()
				if ok, dbErr := dashboard.TableExists(dashDB); dbErr != nil {
					return fmt.Errorf("reading dashboard db %s: %w", dashboardDB, dbErr)
				} else if !ok {
					return fmt.Errorf("dashboard db %s does not have a LiftingSession table — is this the right file? Expected prisma/dev.db", dashboardDB)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Dashboard DB: %s\n", dashboardDB)
			}

			// Build authenticated HTTP client
			sessionCookie := cfg.AuthHeader()
			if sessionCookie == "" {
				return authErr(fmt.Errorf("not authenticated — run 'gravitus-pp-cli auth login-password' first"))
			}

			httpClient, err := buildAuthenticatedClient(sessionCookie)
			if err != nil {
				return fmt.Errorf("building HTTP client: %w", err)
			}

			// Open local CLI store
			if dbPath == "" {
				dbPath = defaultDBPath("gravitus-pp-cli")
			}
			localDB, err := openLocalDB(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer localDB.Close()

			baseURL := strings.TrimRight(cfg.BaseURL, "/")
			w := cmd.OutOrStdout()

			// Phase 1: fetch all workout summaries from profile pages
			fmt.Fprintf(w, "Fetching workout list for user %s...\n", userID)
			var allSummaries []scraper.WorkoutSummary
			page := 1
			cappedByLimit := false
			for {
				if maxPages > 0 && page > maxPages {
					cappedByLimit = true
					break
				}
				pageURL := fmt.Sprintf("%s/users/%s/?page=%d", baseURL, userID, page)
				htmlStr, err := fetchHTML(httpClient, pageURL)
				if err != nil {
					return fmt.Errorf("fetching profile page %d: %w", page, err)
				}

				summaries, hasNext, err := scraper.ParseProfilePage(htmlStr)
				if err != nil {
					return fmt.Errorf("parsing profile page %d: %w", page, err)
				}

				allSummaries = append(allSummaries, summaries...)
				fmt.Fprintf(w, "  Page %d: %d workouts found\n", page, len(summaries))

				if !hasNext || len(summaries) == 0 {
					break
				}
				page++
				time.Sleep(200 * time.Millisecond) // polite rate limiting
			}

			if cappedByLimit {
				fmt.Fprintf(w, "Warning: stopped at page limit (%d pages). Use --max-pages=0 for an unlimited sync.\n", maxPages)
			}
			fmt.Fprintf(w, "Total workouts found: %d\n", len(allSummaries))

			// Phase 2: fetch and parse each workout
			added, updated, skipped := 0, 0, 0
			var errs []string

			for i, s := range allSummaries {
				// Incremental: skip if already in dashboard DB
				if incremental && dashDB != nil {
					exists, err := dashboard.ExistsOnDate(dashDB, s.Date, "gravitus")
					if err == nil && exists {
						skipped++
						continue
					}
				}

				// Check local store once — drives both the skip gate and the add/updated counter below.
				existsLocally, _ := localWorkoutExists(localDB, s.ID)

				// Skip if already in local store (unless --full)
				if !fullSync && existsLocally {
					skipped++
					continue
				}

				fmt.Fprintf(w, "  [%d/%d] Syncing workout %s: %s (%s)...\n",
					i+1, len(allSummaries), s.ID, s.Title, s.Date.Format("2006-01-02"))

				workoutURL := fmt.Sprintf("%s/workouts/%s/", baseURL, s.ID)
				htmlStr, err := fetchHTML(httpClient, workoutURL)
				if err != nil {
					errs = append(errs, fmt.Sprintf("workout %s: %v", s.ID, err))
					continue
				}

				wo, err := scraper.ParseWorkoutPage(htmlStr, s.ID)
				if err != nil {
					errs = append(errs, fmt.Sprintf("parsing workout %s: %v", s.ID, err))
					continue
				}
				wo.Date = s.Date
				if wo.Title == "" {
					wo.Title = s.Title
				}

				// Write to local store
				if err := upsertLocalWorkout(localDB, wo); err != nil {
					errs = append(errs, fmt.Sprintf("storing workout %s: %v", s.ID, err))
					continue
				}

				// Write to dashboard DB
				if dashDB != nil {
					var exEntries []dashboard.ExerciseEntry
					for _, ex := range wo.Exercises {
						entry := dashboard.ExerciseEntry{Name: ex.Name}
						for _, set := range ex.Sets {
							if set.Reps > 0 && set.WeightLbs > 0 {
								entry.Sets = append(entry.Sets, dashboard.ExerciseSet{
									Reps:      set.Reps,
									WeightLbs: set.WeightLbs,
								})
							}
						}
						exEntries = append(exEntries, entry)
					}
					sess := dashboard.LiftingSession{
						Date:      wo.Date,
						Title:     wo.Title,
						Exercises: exEntries,
						Source:    "gravitus",
					}
					if err := dashboard.Upsert(dashDB, sess); err != nil {
						errs = append(errs, fmt.Sprintf("dashboard write for workout %s: %v", s.ID, err))
						continue
					}
				}

				if existsLocally {
					updated++
				} else {
					added++
				}
				time.Sleep(300 * time.Millisecond) // polite rate limiting
			}

			// Summary
			fmt.Fprintf(w, "\nSync complete:\n")
			fmt.Fprintf(w, "  Added:   %d\n", added)
			fmt.Fprintf(w, "  Updated: %d\n", updated)
			fmt.Fprintf(w, "  Skipped: %d (already synced)\n", skipped)
			if len(errs) > 0 {
				fmt.Fprintf(w, "  Errors:  %d\n", len(errs))
				for _, e := range errs {
					fmt.Fprintf(os.Stderr, "    error: %s\n", e)
				}
			}

			if flags.asJSON {
				return printOutputWithFlags(w, mustJSON(map[string]any{
					"added":   added,
					"updated": updated,
					"skipped": skipped,
					"errors":  errs,
				}), flags)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&userID, "user-id", "", "Gravitus user ID (find it in your profile URL)")
	cmd.Flags().StringVar(&dashboardDB, "dashboard-db", "", "Path to training dashboard's dev.db")
	cmd.Flags().BoolVar(&fullSync, "full", false, "Re-sync all workouts (ignore previous sync state)")
	cmd.Flags().BoolVar(&incremental, "incremental", false, "Only fetch workouts not already in the dashboard DB")
	cmd.Flags().StringVar(&dbPath, "db", "", "Local CLI database path")
	cmd.Flags().IntVar(&maxPages, "max-pages", 50, "Maximum profile pages to fetch (0 = unlimited)")
	return cmd
}

// buildAuthenticatedClient creates an HTTP client with the session cookie set.
func buildAuthenticatedClient(sessionCookie string) (htmlFetcher, error) {
	inner := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
	return &authenticatedClient{Client: inner, cookieHeader: sessionCookie}, nil
}

// authenticatedClient wraps http.Client to inject the session cookie on every request.
type authenticatedClient struct {
	*http.Client
	cookieHeader string
}

func (c *authenticatedClient) Get(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Cookie", c.cookieHeader)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")
	return c.Client.Do(req)
}

type htmlFetcher interface {
	Get(url string) (*http.Response, error)
}

func fetchHTML(client htmlFetcher, url string) (string, error) {
	resp, err := client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 302 || resp.StatusCode == 301 {
		// Redirect to login — session expired
		return "", fmt.Errorf("redirected to login page — session may have expired, run 'auth login-password' again")
	}
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("HTTP %d for %s", resp.StatusCode, url)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Check if we got redirected to login (some Django setups return 200 with login form)
	if strings.Contains(string(body), "csrfmiddlewaretoken") && strings.Contains(string(body), "sign_in") &&
		!strings.Contains(url, "sign_in") {
		return "", fmt.Errorf("session expired — run 'auth login-password' to re-authenticate")
	}

	return string(body), nil
}

// --- Local SQLite store helpers for workout persistence ---

func openLocalDB(path string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("%s: %w", pragma, err)
		}
	}
	if err := initLocalSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func initLocalSchema(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS workouts (
			id TEXT PRIMARY KEY,
			title TEXT NOT NULL,
			gym TEXT,
			date TEXT NOT NULL,
			exercises_json TEXT NOT NULL,
			total_volume_lbs REAL DEFAULT 0,
			synced_at TEXT NOT NULL
		);
		CREATE TABLE IF NOT EXISTS exercise_sets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			workout_id TEXT NOT NULL,
			exercise_name TEXT NOT NULL,
			exercise_slug TEXT,
			set_number INTEGER,
			is_warmup INTEGER DEFAULT 0,
			is_pr INTEGER DEFAULT 0,
			weight_lbs REAL,
			reps INTEGER,
			duration_s INTEGER,
			FOREIGN KEY(workout_id) REFERENCES workouts(id)
		);
		CREATE INDEX IF NOT EXISTS idx_exercise_sets_slug ON exercise_sets(exercise_slug);
		CREATE INDEX IF NOT EXISTS idx_exercise_sets_pr ON exercise_sets(is_pr) WHERE is_pr = 1;
		CREATE INDEX IF NOT EXISTS idx_workouts_date ON workouts(date);
	`)
	return err
}

func localWorkoutExists(db *sql.DB, workoutID string) (bool, error) {
	var id string
	err := db.QueryRow(`SELECT id FROM workouts WHERE id = ?`, workoutID).Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func upsertLocalWorkout(db *sql.DB, wo *scraper.Workout) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	_, err = tx.Exec(
		`INSERT OR REPLACE INTO workouts (id, title, gym, date, exercises_json, total_volume_lbs, synced_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		wo.ID,
		wo.Title,
		wo.Gym,
		wo.Date.UTC().Format("2006-01-02T15:04:05Z"),
		wo.ExercisesJSON(),
		wo.TotalVolumeLbs(),
		time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	)
	if err != nil {
		return err
	}

	// Delete existing sets for this workout (to handle re-sync)
	if _, err := tx.Exec(`DELETE FROM exercise_sets WHERE workout_id = ?`, wo.ID); err != nil {
		return err
	}

	// Insert all sets
	for _, ex := range wo.Exercises {
		for _, s := range ex.Sets {
			var weightLbs, reps, durationS interface{}
			if s.WeightLbs > 0 {
				weightLbs = s.WeightLbs
			}
			if s.Reps > 0 {
				reps = s.Reps
			}
			if s.DurationS > 0 {
				durationS = s.DurationS
			}

			_, err = tx.Exec(
				`INSERT INTO exercise_sets (workout_id, exercise_name, exercise_slug, set_number, is_warmup, is_pr, weight_lbs, reps, duration_s)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				wo.ID, ex.Name, ex.Slug,
				s.Number,
				boolInt(s.IsWarmup),
				boolInt(s.IsPR),
				weightLbs, reps, durationS,
			)
			if err != nil {
				return err
			}
		}
	}

	return tx.Commit()
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
