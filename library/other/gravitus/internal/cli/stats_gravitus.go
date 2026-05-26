package cli

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/other/gravitus/internal/config"
	_ "modernc.org/sqlite"
)

func newStatsGravitusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Training statistics — volume trends, workout counts, and more",
		RunE:  parentNoSubcommandRunE(flags),
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newStatsVolumeCmd(flags))
	cmd.AddCommand(newStatsWorkoutsCmd(flags))
	return cmd
}

func newStatsVolumeCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var weeks int

	cmd := &cobra.Command{
		Use:   "volume",
		Short: "Weekly lifting volume (total lbs) over the last N weeks",
		Example: strings.Join([]string{
			`  gravitus-pp-cli stats volume --weeks 12`,
			`  gravitus-pp-cli stats volume --weeks 8 --agent`,
			`  gravitus-pp-cli stats volume --weeks 12 --json --select week,total_volume_lbs`,
		}, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("gravitus-pp-cli")
			}
			db, err := openLocalDB(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'gravitus-pp-cli gravitus-sync' first", err)
			}
			defer db.Close()

			cutoff := time.Now().AddDate(0, 0, -weeks*7)

			rows, err := db.Query(`
				SELECT date, total_volume_lbs FROM workouts
				WHERE date >= ?
				ORDER BY date ASC
			`, cutoff.UTC().Format("2006-01-02T15:04:05Z"))
			if err != nil {
				return fmt.Errorf("querying volume: %w", err)
			}
			defer rows.Close()

			// Aggregate by ISO week
			weekVolume := map[string]float64{}
			weekOrder := []string{}
			seen := map[string]bool{}

			for rows.Next() {
				var dateStr string
				var vol sql.NullFloat64
				if err := rows.Scan(&dateStr, &vol); err != nil {
					continue
				}
				t, _ := time.Parse("2006-01-02T15:04:05Z", dateStr)
				if t.IsZero() {
					t, _ = time.Parse("2006-01-02", dateStr[:10])
				}
				year, week := t.ISOWeek()
				wk := fmt.Sprintf("%d-W%02d", year, week)
				if !seen[wk] {
					weekOrder = append(weekOrder, wk)
					seen[wk] = true
				}
				weekVolume[wk] += vol.Float64
			}

			type VolumeRow struct {
				Week            string  `json:"week"`
				TotalVolumeLbs  float64 `json:"total_volume_lbs"`
				TotalVolumeTons float64 `json:"total_volume_tons"`
			}

			var result []VolumeRow
			for _, wk := range weekOrder {
				vol := weekVolume[wk]
				result = append(result, VolumeRow{
					Week:            wk,
					TotalVolumeLbs:  math2dp(vol),
					TotalVolumeTons: math2dp(vol / 2000),
				})
			}

			if len(result) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No data for the last %d weeks. Run 'gravitus-pp-cli gravitus-sync' first.\n", weeks)
				return nil
			}

			out, _ := json.Marshal(result)
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local CLI database path")
	cmd.Flags().IntVar(&weeks, "weeks", 12, "Number of weeks to include")
	return cmd
}

func newStatsWorkoutsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var weeks int

	cmd := &cobra.Command{
		Use:   "workouts",
		Short: "Workout count and frequency over the last N weeks",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("gravitus-pp-cli")
			}
			db, err := openLocalDB(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w", err)
			}
			defer db.Close()

			cutoff := time.Now().AddDate(0, 0, -weeks*7)

			var total int
			db.QueryRow(`SELECT COUNT(*) FROM workouts WHERE date >= ?`,
				cutoff.UTC().Format("2006-01-02T15:04:05Z")).Scan(&total)

			var allTime int
			db.QueryRow(`SELECT COUNT(*) FROM workouts`).Scan(&allTime)

			result := map[string]any{
				"period_weeks":     weeks,
				"workouts_in_period": total,
				"workouts_all_time": allTime,
				"avg_per_week":     math2dp(float64(total) / float64(weeks)),
			}
			out, _ := json.Marshal(result)
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local CLI database path")
	cmd.Flags().IntVar(&weeks, "weeks", 8, "Number of weeks to include")
	return cmd
}

func newExportCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var format string
	var output string

	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export Gravitus workout history to CSV or JSON",
		Long: `Export your complete synced Gravitus training history.

Writes every workout with full exercise and set details to CSV or JSON.
Gravitus has no native export feature — this is the only way to get your data out.`,
		Example: strings.Join([]string{
			`  gravitus-pp-cli export --format csv --output training_history.csv`,
			`  gravitus-pp-cli export --format json --output training_history.json`,
			`  gravitus-pp-cli export --format json`,
		}, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("gravitus-pp-cli")
			}
			db, err := openLocalDB(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'gravitus-pp-cli gravitus-sync' first", err)
			}
			defer db.Close()

			rows, err := db.Query(`
				SELECT w.id, w.title, w.gym, w.date, w.total_volume_lbs,
				       es.exercise_name, es.exercise_slug, es.set_number,
				       es.is_warmup, es.is_pr, es.weight_lbs, es.reps, es.duration_s
				FROM workouts w
				LEFT JOIN exercise_sets es ON es.workout_id = w.id
				ORDER BY w.date ASC, es.exercise_name, es.set_number
			`)
			if err != nil {
				return fmt.Errorf("querying workouts: %w", err)
			}
			defer rows.Close()

			type Row struct {
				WorkoutID      string  `json:"workout_id"`
				WorkoutTitle   string  `json:"workout_title"`
				Gym            string  `json:"gym"`
				Date           string  `json:"date"`
				TotalVolumeLbs float64 `json:"total_volume_lbs"`
				ExerciseName   string  `json:"exercise_name"`
				ExerciseSlug   string  `json:"exercise_slug"`
				SetNumber      int     `json:"set_number"`
				IsWarmup       bool    `json:"is_warmup"`
				IsPR           bool    `json:"is_pr"`
				WeightLbs      float64 `json:"weight_lbs"`
				Reps           int     `json:"reps"`
				DurationS      int     `json:"duration_s"`
			}

			var allRows []Row
			for rows.Next() {
				var r Row
				var gym, exerciseName, exerciseSlug sql.NullString
				var weight, totalVol sql.NullFloat64
				var reps, durationS, setNum, isWarmup, isPR sql.NullInt64
				if err := rows.Scan(
					&r.WorkoutID, &r.WorkoutTitle, &gym, &r.Date, &totalVol,
					&exerciseName, &exerciseSlug, &setNum,
					&isWarmup, &isPR, &weight, &reps, &durationS,
				); err != nil {
					continue
				}
				r.Gym = gym.String
				r.ExerciseName = exerciseName.String
				r.ExerciseSlug = exerciseSlug.String
				r.TotalVolumeLbs = totalVol.Float64
				r.SetNumber = int(setNum.Int64)
				r.IsWarmup = isWarmup.Int64 == 1
				r.IsPR = isPR.Int64 == 1
				r.WeightLbs = weight.Float64
				r.Reps = int(reps.Int64)
				r.DurationS = int(durationS.Int64)
				if len(r.Date) > 10 {
					r.Date = r.Date[:10]
				}
				allRows = append(allRows, r)
			}

			var dest *os.File
			if output != "" {
				dest, err = os.Create(output)
				if err != nil {
					return fmt.Errorf("creating output file: %w", err)
				}
				defer dest.Close()
			} else {
				dest = os.Stdout
			}

			switch strings.ToLower(format) {
			case "json":
				enc := json.NewEncoder(dest)
				enc.SetIndent("", "  ")
				if err := enc.Encode(allRows); err != nil {
					return err
				}
			case "csv":
				w := csv.NewWriter(dest)
				w.Write([]string{"workout_id", "date", "title", "gym", "total_volume_lbs",
					"exercise_name", "exercise_slug", "set_number", "is_warmup", "is_pr",
					"weight_lbs", "reps", "duration_s"})
				for _, r := range allRows {
					w.Write([]string{
						r.WorkoutID, r.Date, r.WorkoutTitle, r.Gym,
						fmt.Sprintf("%.2f", r.TotalVolumeLbs),
						r.ExerciseName, r.ExerciseSlug,
						fmt.Sprintf("%d", r.SetNumber),
						fmt.Sprintf("%v", r.IsWarmup),
						fmt.Sprintf("%v", r.IsPR),
						fmt.Sprintf("%.2f", r.WeightLbs),
						fmt.Sprintf("%d", r.Reps),
						fmt.Sprintf("%d", r.DurationS),
					})
				}
				w.Flush()
				if err := w.Error(); err != nil {
					return err
				}
			default:
				return fmt.Errorf("unknown format %q — use csv or json", format)
			}

			if output != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Exported %d rows to %s\n", len(allRows), output)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local CLI database path")
	cmd.Flags().StringVar(&format, "format", "json", "Output format: json or csv")
	cmd.Flags().StringVar(&output, "output", "", "Output file path (default: stdout)")
	return cmd
}

// newWorkoutsListCmd shows the synced workout list.
func newWorkoutsListCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List synced workouts from local database",
		Example: strings.Join([]string{
			`  gravitus-pp-cli workouts list`,
			`  gravitus-pp-cli workouts list --limit 5 --agent`,
		}, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("gravitus-pp-cli")
			}
			db, err := openLocalDB(dbPath)
			if err != nil {
				return fmt.Errorf("opening local database: %w\nRun 'gravitus-pp-cli gravitus-sync' first", err)
			}
			defer db.Close()

			rows, err := db.Query(`
				SELECT id, title, gym, date, total_volume_lbs FROM workouts
				ORDER BY date DESC LIMIT ?
			`, limit)
			if err != nil {
				return fmt.Errorf("querying workouts: %w", err)
			}
			defer rows.Close()

			type WorkoutRow struct {
				ID             string  `json:"id"`
				Title          string  `json:"title"`
				Gym            string  `json:"gym"`
				Date           string  `json:"date"`
				TotalVolumeLbs float64 `json:"total_volume_lbs"`
				URL            string  `json:"url"`
			}

			var workouts []WorkoutRow
			for rows.Next() {
				var r WorkoutRow
				var gym sql.NullString
				var vol sql.NullFloat64
				if err := rows.Scan(&r.ID, &r.Title, &gym, &r.Date, &vol); err != nil {
					continue
				}
				r.Gym = gym.String
				r.TotalVolumeLbs = math2dp(vol.Float64)
				r.URL = "https://gravitus.com/workouts/" + r.ID + "/"
				if len(r.Date) > 10 {
					r.Date = r.Date[:10]
				}
				workouts = append(workouts, r)
			}

			if len(workouts) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No workouts synced yet. Run 'gravitus-pp-cli gravitus-sync' first.")
				return nil
			}

			out, _ := json.Marshal(workouts)
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local CLI database path")
	cmd.Flags().IntVar(&limit, "limit", 25, "Maximum workouts to return")
	return cmd
}

// workoutsSyncStatus reports what's in the local store.
func newWorkoutsSyncStatusCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "sync-status",
		Short: "Show sync status — how many workouts are synced and last sync date",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dbPath == "" {
				dbPath = defaultDBPath("gravitus-pp-cli")
			}
			db, err := openLocalDB(dbPath)
			if err != nil {
				fmt.Fprintln(cmd.OutOrStdout(), "Local database not found. Run 'gravitus-pp-cli gravitus-sync' to sync.")
				return nil
			}
			defer db.Close()

			var count int
			var lastDate, lastSync sql.NullString
			db.QueryRow(`SELECT COUNT(*), MAX(date), MAX(synced_at) FROM workouts`).
				Scan(&count, &lastDate, &lastSync)

			result := map[string]any{
				"workouts_synced": count,
				"latest_workout":  lastDate.String,
				"last_synced_at":  lastSync.String,
			}
			out, _ := json.Marshal(result)
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local CLI database path")
	return cmd
}

// newSetDashboardCmd saves the dashboard DB path to config.
func newSetDashboardCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "set-dashboard <path>",
		Short:   "Configure the path to the training dashboard's dev.db",
		Example: `  gravitus-pp-cli set-dashboard ./prisma/dev.db`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			path := args[0]

			// Validate the database has the right schema
			if ok, err := dashboardTableExists(path); err != nil {
				return fmt.Errorf("cannot open %s: %w", path, err)
			} else if !ok {
				return fmt.Errorf("%s does not have a LiftingSession table — is this the correct dev.db?", path)
			}

			cfg, err := configLoad(flags)
			if err != nil {
				return configErr(err)
			}
			if err := cfg.SaveDashboardDB(path); err != nil {
				return configErr(fmt.Errorf("saving config: %w", err))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Dashboard DB path saved: %s\n", path)
			fmt.Fprintf(cmd.OutOrStdout(), "Future 'gravitus-pp-cli gravitus-sync' runs will write to this database automatically.\n")
			return nil
		},
	}
	return cmd
}

func dashboardTableExists(path string) (bool, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return false, err
	}
	defer db.Close()
	var name string
	err = db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name='LiftingSession'`,
	).Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func configLoad(flags *rootFlags) (*config.Config, error) {
	return config.Load(flags.configPath)
}
