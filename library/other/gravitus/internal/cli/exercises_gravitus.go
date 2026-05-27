package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/gravitus/internal/scraper"
	"github.com/spf13/cobra"
)

func newExercisesGravitusCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "exercises",
		Short:       "Analyze exercises — personal records, plateaus, and history",
		RunE:        parentNoSubcommandRunE(flags),
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	cmd.AddCommand(newExercisesPRsCmd(flags))
	cmd.AddCommand(newExercisesPlateauCmd(flags))
	cmd.AddCommand(newExercisesHistoryCmd(flags))
	return cmd
}

// newExercisesPRsCmd lists personal records across all synced exercises.
func newExercisesPRsCmd(flags *rootFlags) *cobra.Command {
	var dbPath string

	cmd := &cobra.Command{
		Use:   "prs",
		Short: "Show personal records across all synced exercises",
		Example: strings.Join([]string{
			`  gravitus-pp-cli exercises prs`,
			`  gravitus-pp-cli exercises prs --agent`,
			`  gravitus-pp-cli exercises prs --json --select exercise_name,weight_lbs,reps`,
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
				SELECT exercise_name, exercise_slug, weight_lbs, reps, w.date
				FROM exercise_sets es
				JOIN workouts w ON w.id = es.workout_id
				WHERE es.is_pr = 1
				ORDER BY exercise_name, w.date DESC
			`)
			if err != nil {
				return fmt.Errorf("querying PRs: %w", err)
			}
			defer rows.Close()

			type PRRow struct {
				ExerciseName string  `json:"exercise_name"`
				ExerciseSlug string  `json:"exercise_slug"`
				WeightLbs    float64 `json:"weight_lbs"`
				Reps         int     `json:"reps"`
				Date         string  `json:"date"`
				Est1RM       float64 `json:"estimated_1rm"`
			}

			var prs []PRRow
			for rows.Next() {
				var r PRRow
				var weightLbs sql.NullFloat64
				var reps sql.NullInt64
				var dateStr string
				if err := rows.Scan(&r.ExerciseName, &r.ExerciseSlug, &weightLbs, &reps, &dateStr); err != nil {
					continue
				}
				r.WeightLbs = weightLbs.Float64
				r.Reps = int(reps.Int64)
				r.Date = dateStr[:10] // YYYY-MM-DD
				r.Est1RM = math2dp(scraper.Estimated1RM(r.WeightLbs, r.Reps))
				prs = append(prs, r)
			}

			if len(prs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No personal records found. Run 'gravitus-pp-cli gravitus-sync' to sync your workouts.")
				return nil
			}

			out, _ := json.Marshal(prs)
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local CLI database path")
	return cmd
}

// newExercisesPlateauCmd detects exercises with no 1RM improvement over N weeks.
func newExercisesPlateauCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var weeks int

	cmd := &cobra.Command{
		Use:   "plateau",
		Short: "Find exercises where estimated 1RM hasn't improved in N weeks",
		Example: strings.Join([]string{
			`  gravitus-pp-cli exercises plateau --weeks 6`,
			`  gravitus-pp-cli exercises plateau --weeks 4 --agent`,
			`  gravitus-pp-cli exercises plateau --weeks 6 --json --select exercise_name,weeks_stalled,last_pr_date`,
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

			// Get the best 1RM per exercise per workout
			rows, err := db.Query(`
				SELECT es.exercise_name, es.exercise_slug,
				       es.weight_lbs, es.reps, w.date
				FROM exercise_sets es
				JOIN workouts w ON w.id = es.workout_id
				WHERE es.weight_lbs > 0 AND es.reps > 0 AND es.is_warmup = 0
				ORDER BY es.exercise_name, w.date ASC
			`)
			if err != nil {
				return fmt.Errorf("querying exercise data: %w", err)
			}
			defer rows.Close()

			type setRecord struct {
				weight float64
				reps   int
				date   time.Time
				est1rm float64
			}
			byExercise := map[string][]setRecord{}
			slugByName := map[string]string{}

			for rows.Next() {
				var name, slug string
				var weight sql.NullFloat64
				var reps sql.NullInt64
				var dateStr string
				if err := rows.Scan(&name, &slug, &weight, &reps, &dateStr); err != nil {
					continue
				}
				t, _ := time.Parse("2006-01-02T15:04:05Z", dateStr)
				if t.IsZero() {
					t, _ = time.Parse("2006-01-02", dateStr[:10])
				}
				slugByName[name] = slug
				byExercise[name] = append(byExercise[name], setRecord{
					weight: weight.Float64,
					reps:   int(reps.Int64),
					date:   t,
					est1rm: scraper.Estimated1RM(weight.Float64, int(reps.Int64)),
				})
			}

			type PlateauRow struct {
				ExerciseName string  `json:"exercise_name"`
				ExerciseSlug string  `json:"exercise_slug"`
				LastPRDate   string  `json:"last_pr_date"`
				WeeksStalled int     `json:"weeks_stalled"`
				BestEst1RM   float64 `json:"best_estimated_1rm"`
				RecentEst1RM float64 `json:"recent_estimated_1rm"`
			}

			var plateaus []PlateauRow

			for name, sets := range byExercise {
				if len(sets) < 2 {
					continue
				}

				// Find all-time best 1RM and when it occurred
				var bestEst1RM float64
				var bestDate time.Time
				for _, s := range sets {
					if s.est1rm > bestEst1RM {
						bestEst1RM = s.est1rm
						bestDate = s.date
					}
				}

				// Find best 1RM in the recent window
				var recentBest float64
				for _, s := range sets {
					if s.date.After(cutoff) && s.est1rm > recentBest {
						recentBest = s.est1rm
					}
				}

				// Plateau: all-time best was before the cutoff AND recent best hasn't matched it
				if bestDate.Before(cutoff) && recentBest < bestEst1RM*0.95 {
					weeksStalled := int(time.Since(bestDate).Hours() / 168)
					plateaus = append(plateaus, PlateauRow{
						ExerciseName: name,
						ExerciseSlug: slugByName[name],
						LastPRDate:   bestDate.Format("2006-01-02"),
						WeeksStalled: weeksStalled,
						BestEst1RM:   math2dp(bestEst1RM),
						RecentEst1RM: math2dp(recentBest),
					})
				}
			}

			if len(plateaus) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No plateaus detected in the last %d weeks. Keep lifting!\n", weeks)
				return nil
			}

			out, _ := json.Marshal(plateaus)
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local CLI database path")
	cmd.Flags().IntVar(&weeks, "weeks", 6, "Number of weeks to look back for plateau detection")
	return cmd
}

// newExercisesHistoryCmd shows set history for a specific exercise.
func newExercisesHistoryCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var limit int

	cmd := &cobra.Command{
		Use:   "history <exercise-slug>",
		Short: "Show set history for a specific exercise",
		Example: strings.Join([]string{
			`  gravitus-pp-cli exercises history bench-press`,
			`  gravitus-pp-cli exercises history db-step-ups --limit 20 --agent`,
		}, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
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

			slug := args[0]
			rows, err := db.Query(`
				SELECT es.exercise_name, es.weight_lbs, es.reps, es.duration_s,
				       es.is_warmup, es.is_pr, w.date, w.title
				FROM exercise_sets es
				JOIN workouts w ON w.id = es.workout_id
				WHERE es.exercise_slug = ? OR lower(es.exercise_name) LIKE ?
				ORDER BY w.date DESC, es.set_number ASC
				LIMIT ?
			`, slug, "%"+strings.ReplaceAll(slug, "-", " ")+"%", limit)
			if err != nil {
				return fmt.Errorf("querying exercise history: %w", err)
			}
			defer rows.Close()

			type HistoryRow struct {
				ExerciseName string  `json:"exercise_name"`
				WeightLbs    float64 `json:"weight_lbs"`
				Reps         int     `json:"reps"`
				DurationS    int     `json:"duration_s,omitempty"`
				IsWarmup     bool    `json:"is_warmup"`
				IsPR         bool    `json:"is_pr"`
				WorkoutDate  string  `json:"workout_date"`
				WorkoutTitle string  `json:"workout_title"`
				Est1RM       float64 `json:"estimated_1rm,omitempty"`
			}

			var history []HistoryRow
			for rows.Next() {
				var r HistoryRow
				var weight sql.NullFloat64
				var reps, durationS sql.NullInt64
				var isWarmup, isPR int
				var dateStr, title string
				if err := rows.Scan(&r.ExerciseName, &weight, &reps, &durationS,
					&isWarmup, &isPR, &dateStr, &title); err != nil {
					continue
				}
				r.WeightLbs = weight.Float64
				r.Reps = int(reps.Int64)
				r.DurationS = int(durationS.Int64)
				r.IsWarmup = isWarmup == 1
				r.IsPR = isPR == 1
				r.WorkoutDate = dateStr[:10]
				r.WorkoutTitle = title
				if r.WeightLbs > 0 && r.Reps > 0 {
					r.Est1RM = math2dp(scraper.Estimated1RM(r.WeightLbs, r.Reps))
				}
				history = append(history, r)
			}

			if len(history) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No history found for %q. Check the slug or run 'gravitus-pp-cli gravitus-sync' first.\n", slug)
				return nil
			}

			out, _ := json.Marshal(history)
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "Local CLI database path")
	cmd.Flags().IntVar(&limit, "limit", 100, "Maximum number of sets to return")
	return cmd
}

func math2dp(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}
