package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func questionsCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "questions", Short: "Question metadata and review notes"}
	cmd.AddCommand(questionsListCmd(), questionsShowCmd(), questionsNoteCmd())
	return cmd
}

func questionsListCmd() *cobra.Command {
	var limit int
	var attempt, qtype string
	var incorrect, flagged, unanswered bool
	var difficulty, minTime int
	c := &cobra.Command{Use: "list", RunE: func(cmd *cobra.Command, args []string) error {
		db, _, err := openDB()
		if err != nil {
			return err
		}
		defer db.Close()
		where := []string{"1=1"}
		vals := []any{}
		if attempt != "" {
			where = append(where, "attempt_id=?")
			vals = append(vals, attempt)
		}
		if qtype != "" {
			where = append(where, "question_type=?")
			vals = append(vals, qtype)
		}
		if incorrect {
			where = append(where, "is_correct=0")
		}
		if flagged {
			where = append(where, "flagged=1")
		}
		if unanswered {
			where = append(where, "answered=0")
		}
		if difficulty > 0 {
			where = append(where, "difficulty=?")
			vals = append(vals, difficulty)
		}
		if minTime > 0 {
			where = append(where, "time_spent_seconds>=?")
			vals = append(vals, minTime)
		}
		q := `SELECT id,attempt_id,section_id,section_index,question_number,question_type,chosen_answer,correct_answer,is_correct,time_spent_seconds,flagged,eliminated_count,review_note,source_url,answered,difficulty FROM questions WHERE ` + strings.Join(where, " AND ") + ` ORDER BY attempt_id, section_index, question_number`
		if limit > 0 {
			q += fmt.Sprintf(" LIMIT %d", limit)
		}
		rows, err := queryMaps(db, q, vals...)
		if err != nil {
			return err
		}
		return emit(rows)
	}}
	c.Flags().IntVar(&limit, "limit", 50, "limit rows")
	c.Flags().StringVar(&attempt, "attempt", "", "attempt id")
	c.Flags().StringVar(&qtype, "type", "", "question type")
	c.Flags().BoolVar(&incorrect, "incorrect", false, "only incorrect questions")
	c.Flags().BoolVar(&flagged, "flagged", false, "only flagged questions")
	c.Flags().BoolVar(&unanswered, "unanswered", false, "only unanswered questions")
	c.Flags().IntVar(&difficulty, "difficulty", 0, "difficulty level")
	c.Flags().IntVar(&minTime, "min-time", 0, "minimum time spent seconds")
	return c
}

func questionsShowCmd() *cobra.Command {
	return &cobra.Command{Use: "show <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		db, _, err := openDB()
		if err != nil {
			return err
		}
		defer db.Close()
		rows, err := queryMaps(db, `SELECT * FROM questions WHERE id=?`, args[0])
		if err != nil {
			return err
		}
		if len(rows) == 0 {
			return fmt.Errorf("question not found: %s", args[0])
		}
		return emit(rows[0])
	}}
}

func questionsNoteCmd() *cobra.Command {
	var note, whyPicked, whyCorrect, nextTime string
	c := &cobra.Command{Use: "note <id>", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		parts := []string{}
		if note != "" {
			parts = append(parts, note)
		}
		if whyPicked != "" {
			parts = append(parts, "Why picked: "+whyPicked)
		}
		if whyCorrect != "" {
			parts = append(parts, "Why correct: "+whyCorrect)
		}
		if nextTime != "" {
			parts = append(parts, "Next time: "+nextTime)
		}
		text := strings.Join(parts, "\n")
		if text == "" {
			return fmt.Errorf("provide --note or reflection fields")
		}
		db, _, err := openDB()
		if err != nil {
			return err
		}
		defer db.Close()
		res, err := db.Exec(`UPDATE questions SET review_note=? WHERE id=?`, text, args[0])
		if err != nil {
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return fmt.Errorf("question not found: %s", args[0])
		}
		return emit(map[string]any{"id": args[0], "updated": n, "review_note": text})
	}}
	c.Flags().StringVar(&note, "note", "", "freeform note")
	c.Flags().StringVar(&whyPicked, "why-picked", "", "why the chosen answer was attractive")
	c.Flags().StringVar(&whyCorrect, "why-correct", "", "why the correct answer works")
	c.Flags().StringVar(&nextTime, "next-time", "", "what to do differently next time")
	return c
}
