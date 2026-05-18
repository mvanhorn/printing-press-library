package cli

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/mvanhorn/printing-press-library/library/education/lawhub/internal/store"
	"github.com/spf13/cobra"
)

const app = "lawhub-pp-cli"
const lawhubURL = "https://app.lawhub.org/library/fulltests"

var cfg Config

type Config struct {
	DataDir   string
	ConfigDir string
	SecureDir string
	JSON      bool
	Compact   bool
	Agent     bool
	Select    string
	UserID    string
}

type Attempt struct {
	ID             string `json:"id"`
	TestID         any    `json:"test_id,omitempty"`
	TestName       any    `json:"test_name,omitempty"`
	Mode           any    `json:"mode,omitempty"`
	StartedAt      any    `json:"started_at,omitempty"`
	CompletedAt    any    `json:"completed_at,omitempty"`
	ScaledScore    any    `json:"scaled_score,omitempty"`
	RawScore       any    `json:"raw_score,omitempty"`
	TotalQuestions any    `json:"total_questions,omitempty"`
	CorrectCount   any    `json:"correct_count,omitempty"`
	SourceURL      any    `json:"source_url,omitempty"`
	SyncedAt       any    `json:"synced_at,omitempty"`
}

func homeJoin(parts ...string) string {
	h, _ := os.UserHomeDir()
	all := append([]string{h}, parts...)
	return filepath.Join(all...)
}

func Execute() {
	root := rootCmd()
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func rootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          app,
		Short:        "LawHub/LSAC practice-test analytics CLI",
		Long:         "Standalone local CLI for LawHub LSAT practice-test analytics. SQLite is the source of truth.",
		Version:      version,
		SilenceUsage: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if cfg.Agent {
				cfg.JSON = true
				cfg.Compact = true
			}
		},
	}
	cmd.PersistentFlags().StringVar(&cfg.DataDir, "data-dir", homeJoin(".local", "share", app), "data directory")
	cmd.PersistentFlags().StringVar(&cfg.ConfigDir, "config-dir", homeJoin(".config", app), "config directory")
	cmd.PersistentFlags().StringVar(&cfg.SecureDir, "secure-dir", homeJoin(".openclaw", "secure", "lawhub"), "secure session directory")
	cmd.PersistentFlags().BoolVar(&cfg.JSON, "json", false, "JSON output")
	cmd.PersistentFlags().BoolVar(&cfg.Compact, "compact", false, "compact JSON")
	cmd.PersistentFlags().BoolVar(&cfg.Agent, "agent", false, "agent mode: compact JSON, no prompts where possible")
	cmd.PersistentFlags().StringVar(&cfg.Select, "select", "", "comma-separated fields to include in JSON output")
	cmd.PersistentFlags().StringVar(&cfg.UserID, "user-id", "", "LawHub user id override (advanced; normally discovered at login)")
	cmd.SetVersionTemplate(app + " {{ .Version }}\n")

	cmd.AddCommand(doctorCmd(), summaryCmd(), attemptsCmd(), testsCmd(), questionsCmd(), weaknessCmd(), reviewCmd(), syncCmd(), loginCmd(), authCmd(), versionCmd())
	return cmd
}

func openDB() (*sql.DB, string, error) { return store.Open(cfg.DataDir) }
func fileExists(path string) bool      { _, err := os.Stat(path); return err == nil }

func emit(v any) error {
	if cfg.Select != "" {
		v = applySelect(v, cfg.Select)
	}
	if cfg.JSON || cfg.Agent {
		var b []byte
		var err error
		if cfg.Compact || cfg.Agent {
			b, err = json.Marshal(v)
		} else {
			b, err = json.MarshalIndent(v, "", "  ")
		}
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(b))
	return nil
}

func mustCounts(db *sql.DB) map[string]int {
	out := map[string]int{}
	for _, t := range []string{"tests", "attempts", "sections", "questions", "sync_log"} {
		c, _ := store.Count(db, t)
		out[t] = c
	}
	return out
}

func doctorCmd() *cobra.Command {
	var live bool
	c := &cobra.Command{Use: "doctor", Short: "Check local state", RunE: func(cmd *cobra.Command, args []string) error {
		db, dbPath, err := openDB()
		if err != nil {
			return err
		}
		defer db.Close()
		session := sessionPath()
		_, statErr := os.Stat(session)
		browserPath := resolveBrowserPath("")
		_, browserErr := os.Stat(browserPath)
		acct := readAccount()
		out := map[string]any{"app": app, "implementation": "go-cobra", "version": versionInfo(), "db": dbPath, "session_file": session, "session_exists": statErr == nil, "account_file": accountPath(), "user_id": nullIfEmpty(acct.UserID), "browser_path": browserPath, "browser_found": browserErr == nil, "counts": mustCounts(db), "lawhub_url": lawhubURL}
		if live {
			out["auth_live"] = authPing()
		}
		return emit(out)
	}}
	c.Flags().BoolVar(&live, "live", false, "perform live authenticated ping")
	return c
}

func queryAttempts(db *sql.DB, limit int) ([]Attempt, error) {
	q := `SELECT id,test_id,test_name,mode,started_at,completed_at,scaled_score,raw_score,total_questions,correct_count,source_url,synced_at FROM attempts ORDER BY COALESCE(completed_at, started_at, synced_at) DESC`
	if limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", limit)
	}
	rows, err := db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Attempt
	for rows.Next() {
		var a Attempt
		var testID, testName, mode, started, completed, source, synced sql.NullString
		var scaled, raw, total, correct sql.NullInt64
		if err := rows.Scan(&a.ID, &testID, &testName, &mode, &started, &completed, &scaled, &raw, &total, &correct, &source, &synced); err != nil {
			return nil, err
		}
		a.TestID = store.NullString(testID)
		a.TestName = store.NullString(testName)
		a.Mode = store.NullString(mode)
		a.StartedAt = store.NullString(started)
		a.CompletedAt = store.NullString(completed)
		a.ScaledScore = store.NullInt(scaled)
		a.RawScore = store.NullInt(raw)
		a.TotalQuestions = store.NullInt(total)
		a.CorrectCount = store.NullInt(correct)
		a.SourceURL = store.NullString(source)
		a.SyncedAt = store.NullString(synced)
		out = append(out, a)
	}
	return out, rows.Err()
}

func summaryCmd() *cobra.Command {
	var limit, minCount int
	cmd := &cobra.Command{Use: "summary", Short: "CLI-native LSAT analytics summary", RunE: func(cmd *cobra.Command, args []string) error {
		db, _, err := openDB()
		if err != nil {
			return err
		}
		defer db.Close()
		attempts, err := queryAttempts(db, limit)
		if err != nil {
			return err
		}
		allAttempts, _ := queryAttempts(db, 0)
		var latest, best *Attempt
		var sum float64
		var n int
		for i := range allAttempts {
			a := &allAttempts[i]
			if a.ScaledScore == nil {
				continue
			}
			if latest == nil {
				latest = a
			}
			if best == nil || asInt(a.ScaledScore) > asInt(best.ScaledScore) {
				best = a
			}
			if n < 5 {
				sum += float64(asInt(a.ScaledScore))
				n++
			}
		}
		var avg any
		if n > 0 {
			avg = float64(int((sum/float64(n))*10+0.5)) / 10
		}
		sections, _ := sectionWeakness(db)
		qtypes, _ := questionTypeWeakness(db, minCount)
		if limit > 0 && len(sections) > limit {
			sections = sections[:limit]
		}
		if limit > 0 && len(qtypes) > limit {
			qtypes = qtypes[:limit]
		}
		return emit(map[string]any{"counts": mustCounts(db), "latest": latest, "best": best, "recent_average": avg, "recent_attempts": attempts, "weakest_sections": sections, "weakest_question_types": qtypes})
	}}
	cmd.Flags().IntVar(&limit, "limit", 5, "limit rows")
	cmd.Flags().IntVar(&minCount, "min-count", 1, "minimum question count")
	return cmd
}

func asInt(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case int:
		return int64(x)
	case float64:
		return int64(x)
	}
	return 0
}

func attemptsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "attempts", Short: "Manage attempts"}
	var limit int
	list := &cobra.Command{Use: "list", Short: "List attempts", RunE: func(cmd *cobra.Command, args []string) error {
		db, _, err := openDB()
		if err != nil {
			return err
		}
		defer db.Close()
		rows, err := queryAttempts(db, limit)
		if err != nil {
			return err
		}
		return emit(rows)
	}}
	list.Flags().IntVar(&limit, "limit", 20, "limit rows")
	show := &cobra.Command{Use: "show <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error { return showAttempt(args[0]) }}
	cmd.AddCommand(list, show)
	return cmd
}

func showAttempt(id string) error {
	db, _, err := openDB()
	if err != nil {
		return err
	}
	defer db.Close()
	attempts, err := queryAttemptsByID(db, id)
	if err != nil {
		return err
	}
	if len(attempts) == 0 {
		return fmt.Errorf("attempt not found: %s", id)
	}
	sections, _ := queryMaps(db, `SELECT * FROM sections WHERE attempt_id=? ORDER BY section_index`, id)
	questions, _ := queryMaps(db, `SELECT id,section_index,question_number,question_type,chosen_answer,correct_answer,is_correct,time_spent_seconds,flagged,difficulty,source_url FROM questions WHERE attempt_id=? ORDER BY section_index, question_number`, id)
	return emit(map[string]any{"attempt": attempts[0], "sections": sections, "questions": questions})
}

func queryAttemptsByID(db *sql.DB, id string) ([]Attempt, error) {
	rows, err := db.Query(`SELECT id,test_id,test_name,mode,started_at,completed_at,scaled_score,raw_score,total_questions,correct_count,source_url,synced_at FROM attempts WHERE id=?`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Attempt
	for rows.Next() {
		var a Attempt
		var testID, testName, mode, started, completed, source, synced sql.NullString
		var scaled, raw, total, correct sql.NullInt64
		if err := rows.Scan(&a.ID, &testID, &testName, &mode, &started, &completed, &scaled, &raw, &total, &correct, &source, &synced); err != nil {
			return nil, err
		}
		a.TestID = store.NullString(testID)
		a.TestName = store.NullString(testName)
		a.Mode = store.NullString(mode)
		a.StartedAt = store.NullString(started)
		a.CompletedAt = store.NullString(completed)
		a.ScaledScore = store.NullInt(scaled)
		a.RawScore = store.NullInt(raw)
		a.TotalQuestions = store.NullInt(total)
		a.CorrectCount = store.NullInt(correct)
		a.SourceURL = store.NullString(source)
		a.SyncedAt = store.NullString(synced)
		out = append(out, a)
	}
	return out, rows.Err()
}

func queryMaps(db *sql.DB, q string, args ...any) ([]map[string]any, error) {
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		m := map[string]any{}
		for i, c := range cols {
			switch v := vals[i].(type) {
			case []byte:
				m[c] = string(v)
			default:
				m[c] = v
			}
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func testsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "tests", Short: "Manage tests"}
	cmd.AddCommand(&cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		db, _, err := openDB()
		if err != nil {
			return err
		}
		defer db.Close()
		rows, err := queryMaps(db, `SELECT id,name,type,available,source_url,synced_at FROM tests ORDER BY name`)
		if err != nil {
			return err
		}
		return emit(rows)
	}})
	return cmd
}

func weaknessCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "weakness", Short: "Weakness analytics"}
	var min int
	r := &cobra.Command{Use: "report", RunE: func(cmd *cobra.Command, args []string) error {
		db, _, err := openDB()
		if err != nil {
			return err
		}
		defer db.Close()
		s, _ := sectionWeakness(db)
		q, _ := questionTypeWeakness(db, min)
		return emit(map[string]any{"by_section": s, "by_question_type": q})
	}}
	r.Flags().IntVar(&min, "min-count", 1, "minimum count")
	cmd.AddCommand(r)
	return cmd
}

type WeaknessRow struct {
	SectionType  string  `json:"section_type,omitempty"`
	QuestionType string  `json:"question_type,omitempty"`
	Correct      int64   `json:"correct"`
	Total        int64   `json:"total"`
	AccuracyPct  float64 `json:"accuracy_pct"`
}

func sectionWeakness(db *sql.DB) ([]WeaknessRow, error) {
	rows, err := db.Query(`SELECT section_type, COALESCE(SUM(correct_count),0), COALESCE(SUM(total_questions),0), COALESCE(ROUND(100.0*SUM(correct_count)/NULLIF(SUM(total_questions),0),1),0) FROM sections WHERE section_type IS NOT NULL GROUP BY section_type ORDER BY 4 ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WeaknessRow
	for rows.Next() {
		var r WeaknessRow
		if err := rows.Scan(&r.SectionType, &r.Correct, &r.Total, &r.AccuracyPct); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
func questionTypeWeakness(db *sql.DB, min int) ([]WeaknessRow, error) {
	rows, err := db.Query(`SELECT question_type, COALESCE(SUM(CASE WHEN is_correct=1 THEN 1 ELSE 0 END),0), COUNT(*), COALESCE(ROUND(100.0*SUM(CASE WHEN is_correct=1 THEN 1 ELSE 0 END)/NULLIF(COUNT(*),0),1),0) FROM questions WHERE question_type IS NOT NULL GROUP BY question_type HAVING COUNT(*) >= ? ORDER BY 4 ASC`, min)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WeaknessRow
	for rows.Next() {
		var r WeaknessRow
		if err := rows.Scan(&r.QuestionType, &r.Correct, &r.Total, &r.AccuracyPct); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func reviewCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "review", Short: "Open LawHub review pages"}
	var section, question int
	var printURL bool
	open := &cobra.Command{Use: "open [attempt-id]", Args: cobra.MaximumNArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		attempt := ""
		if len(args) > 0 {
			attempt = args[0]
		} else {
			db, _, err := openDB()
			if err != nil {
				return err
			}
			defer db.Close()
			var id string
			if err := db.QueryRow(`SELECT id FROM attempts ORDER BY COALESCE(completed_at, started_at, synced_at) DESC LIMIT 1`).Scan(&id); err != nil {
				return errors.New("no attempts found")
			}
			attempt = id
		}
		url := fmt.Sprintf("https://app.lawhub.org/question/%s/Section%%20%d", attempt, section)
		if question > 0 {
			url += fmt.Sprintf("?question=%d", question)
		}
		if printURL || cfg.Agent || cfg.JSON {
			return emit(map[string]any{"url": url, "attempt_id": attempt, "section": section, "question": question})
		}
		return openURL(url)
	}}
	open.Flags().IntVar(&section, "section", 0, "section number")
	_ = open.MarkFlagRequired("section")
	open.Flags().IntVar(&question, "question", 0, "question number")
	open.Flags().BoolVar(&printURL, "print-url", false, "print URL instead of opening")
	cmd.AddCommand(open)
	return cmd
}

func openURL(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

func syncCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "sync", Short: "Sync LawHub data"}
	cmd.AddCommand(syncBrowserCmd(), syncHistoryCmd(), syncReportMetadataCmd())
	return cmd
}
