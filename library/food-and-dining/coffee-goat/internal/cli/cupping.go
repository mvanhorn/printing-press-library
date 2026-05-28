// Copyright 2026 Justin Fu and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/food-and-dining/coffee-goat/internal/store"
	"github.com/spf13/cobra"
)

// ensureCuppingSchema creates the 3 cupping tables if missing. Idempotent.
func ensureCuppingSchema(db *store.Store) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS cupping_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT,
			blind_mode INTEGER DEFAULT 0,
			state TEXT DEFAULT 'active',
			started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			finalized_at DATETIME,
			notes TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS cupping_session_beans (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id INTEGER NOT NULL,
			bean_ref TEXT NOT NULL,
			roaster_slug TEXT,
			product_slug TEXT,
			bean_id INTEGER,
			slot_label TEXT NOT NULL,
			display_label TEXT,
			FOREIGN KEY (session_id) REFERENCES cupping_sessions(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_cupping_session_beans_session ON cupping_session_beans(session_id)`,
		`CREATE TABLE IF NOT EXISTS cupping_scores (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id INTEGER NOT NULL,
			bean_slot TEXT NOT NULL,
			cupper_handle TEXT NOT NULL,
			fragrance_aroma REAL,
			flavor REAL,
			aftertaste REAL,
			acidity REAL,
			body REAL,
			balance REAL,
			uniformity REAL,
			clean_cup REAL,
			sweetness REAL,
			overall REAL,
			defects REAL,
			total REAL,
			notes TEXT,
			scored_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(session_id, bean_slot, cupper_handle),
			FOREIGN KEY (session_id) REFERENCES cupping_sessions(id) ON DELETE CASCADE
		)`,
		`CREATE INDEX IF NOT EXISTS idx_cupping_scores_session ON cupping_scores(session_id)`,
	}
	for _, s := range stmts {
		if _, err := db.DB().Exec(s); err != nil {
			return fmt.Errorf("cupping schema: %w", err)
		}
	}
	return nil
}

// cuppingSession is one session header row.
type cuppingSession struct {
	ID          int64                `json:"id"`
	Name        string               `json:"name"`
	BlindMode   bool                 `json:"blind_mode"`
	State       string               `json:"state"`
	StartedAt   string               `json:"started_at"`
	FinalizedAt string               `json:"finalized_at,omitempty"`
	Notes       string               `json:"notes,omitempty"`
	Beans       []cuppingSessionBean `json:"beans,omitempty"`
	Scores      []cuppingScoreRow    `json:"scores,omitempty"`
}

type cuppingSessionBean struct {
	ID           int64  `json:"id"`
	SessionID    int64  `json:"session_id"`
	BeanRef      string `json:"bean"`
	RoasterSlug  string `json:"roaster_slug,omitempty"`
	ProductSlug  string `json:"product_slug,omitempty"`
	BeanID       int64  `json:"bean_id,omitempty"`
	SlotLabel    string `json:"slot_label"`
	DisplayLabel string `json:"display_label,omitempty"`
}

type cuppingScoreRow struct {
	ID             int64   `json:"id"`
	SessionID      int64   `json:"session_id"`
	BeanSlot       string  `json:"bean_slot"`
	CupperHandle   string  `json:"cupper"`
	FragranceAroma float64 `json:"fragrance_aroma"`
	Flavor         float64 `json:"flavor"`
	Aftertaste     float64 `json:"aftertaste"`
	Acidity        float64 `json:"acidity"`
	Body           float64 `json:"body"`
	Balance        float64 `json:"balance"`
	Uniformity     float64 `json:"uniformity"`
	CleanCup       float64 `json:"clean_cup"`
	Sweetness      float64 `json:"sweetness"`
	Overall        float64 `json:"overall"`
	Defects        float64 `json:"defects"`
	Total          float64 `json:"total"`
	Notes          string  `json:"notes,omitempty"`
	ScoredAt       string  `json:"scored_at"`
}

func newCuppingCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cupping",
		Short: "SCA cupping sessions: full 10-attribute form, multi-cupper, blind or open, bridges scores to the brew log",
		Long: `Stateful cupping flow:
  start      Create a session, optionally --blind with slot labels A/B/C/...
  score      Add or update one cupper's scores for one bean in a session
  finalize   Lock the session, write brew rows for cupper=self scores,
             auto-create palate_profiles rows for new cuppers
  abandon    Delete an in-progress session
  show       Show one session with current scores
  list       List sessions (active and finalized)
  log        Atomic alternative: load a full session from a JSON file

Score scale: 6.00–10.00 in 0.25 increments per SCA attribute. Total is
auto-computed = sum(attributes) − defects.`,
		Example: `  coffee-goat-pp-cli cupping start --name "Sat AM" --bean sey/banko-gotiti --bean april/ethiopia-natural
  coffee-goat-pp-cli cupping score --session 1 --bean A --fragrance 7.5 --flavor 7.75 --aftertaste 7.25 ...
  coffee-goat-pp-cli cupping finalize --session 1`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newCuppingStartCmd(flags))
	cmd.AddCommand(newCuppingScoreCmd(flags))
	cmd.AddCommand(newCuppingFinalizeCmd(flags))
	cmd.AddCommand(newCuppingAbandonCmd(flags))
	cmd.AddCommand(newCuppingShowCmd(flags))
	cmd.AddCommand(newCuppingListCmd(flags))
	cmd.AddCommand(newCuppingLogCmd(flags))
	return cmd
}

func newCuppingStartCmd(flags *rootFlags) *cobra.Command {
	var (
		name  string
		beans []string
		blind bool
		notes string
	)
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Create a new cupping session with the given beans",
		Example: `  coffee-goat-pp-cli cupping start --name "Sat AM" --bean sey/banko-gotiti --bean april/ethiopia-natural
  coffee-goat-pp-cli cupping start --name "blind quad" --blind --bean sey/banko-gotiti --bean onyx/geisha --bean april/x --bean la-cabra/y`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(beans) == 0 {
				return usageErr(fmt.Errorf("at least one --bean is required"))
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			if err := ensureCuppingSchema(db); err != nil {
				return err
			}
			blindInt := 0
			if blind {
				blindInt = 1
			}
			now := time.Now().UTC().Format(time.RFC3339)
			res, err := db.DB().Exec(
				`INSERT INTO cupping_sessions (name, blind_mode, state, started_at, notes) VALUES (?, ?, 'active', ?, ?)`,
				nullableString(name), blindInt, now, nullableString(notes),
			)
			if err != nil {
				return fmt.Errorf("insert cupping_session: %w", err)
			}
			sessionID, _ := res.LastInsertId()
			session := cuppingSession{
				ID: sessionID, Name: name, BlindMode: blind,
				State: "active", StartedAt: now, Notes: notes,
			}
			for i, beanRef := range beans {
				slot := beanRef
				display := beanRef
				if blind {
					slot = slotLetter(i)
					display = "" // hidden until finalize
				}
				roaster, handle := splitRoasterHandle(beanRef)
				localID := lookupLocalBeanID(db, roaster, handle)
				_, err := db.DB().Exec(
					`INSERT INTO cupping_session_beans (session_id, bean_ref, roaster_slug, product_slug, bean_id, slot_label, display_label)
					 VALUES (?, ?, ?, ?, ?, ?, ?)`,
					sessionID, beanRef,
					nullableString(roaster), nullableString(handle),
					nullableInt64(localID),
					slot, nullableString(display),
				)
				if err != nil {
					return fmt.Errorf("insert session_bean: %w", err)
				}
				session.Beans = append(session.Beans, cuppingSessionBean{
					SessionID: sessionID, BeanRef: beanRef,
					RoasterSlug: roaster, ProductSlug: handle, BeanID: localID,
					SlotLabel: slot, DisplayLabel: display,
				})
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), session, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "started cupping session #%d (%s, %d beans)\n",
				sessionID, ifEmpty(name, "(unnamed)"), len(beans))
			for _, b := range session.Beans {
				if blind {
					fmt.Fprintf(cmd.OutOrStdout(), "  slot %s  (bean identity hidden until finalize)\n", b.SlotLabel)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "  slot %s  %s\n", b.SlotLabel, b.BeanRef)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "Session name (free text, e.g. \"Sat AM\")")
	cmd.Flags().StringSliceVar(&beans, "bean", nil, "Bean to include — repeat for multiple beans (e.g. --bean sey/banko-gotiti --bean april/x)")
	cmd.Flags().BoolVar(&blind, "blind", false, "Blind mode: slot labels A/B/C... replace bean identity until finalize")
	cmd.Flags().StringVar(&notes, "notes", "", "Session notes (free text)")
	return cmd
}

func newCuppingScoreCmd(flags *rootFlags) *cobra.Command {
	var (
		sessionID   int64
		bean        string
		cupper      string
		fragrance   float64
		flavor      float64
		aftertaste  float64
		acidity     float64
		body        float64
		balance     float64
		uniformity  float64
		cleanCup    float64
		sweetness   float64
		overall     float64
		defects     float64
		notes       string
		force       bool
		forceReason string
	)
	cmd := &cobra.Command{
		Use:     "score",
		Short:   "Add or update scores for one (session, bean, cupper) — upsert pre-finalize, --force after",
		Example: `  coffee-goat-pp-cli cupping score --session 1 --bean A --fragrance 7.75 --flavor 8.0 --aftertaste 7.5 --acidity 7.75 --body 7.5 --balance 7.5 --uniformity 10 --clean-cup 10 --sweetness 10 --overall 7.75`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if sessionID == 0 || bean == "" {
				return usageErr(fmt.Errorf("cupping score requires --session and --bean"))
			}
			if cupper == "" {
				cupper = "self"
			}
			cupper = strings.ToLower(strings.TrimSpace(cupper))
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			if err := ensureCuppingSchema(db); err != nil {
				return err
			}
			state, err := lookupSessionState(db, sessionID)
			if err != nil {
				return err
			}
			if state == "finalized" && !force {
				return usageErr(fmt.Errorf("session #%d is finalized; pass --force --reason \"<text>\" to amend", sessionID))
			}
			if state == "abandoned" {
				return usageErr(fmt.Errorf("session #%d is abandoned", sessionID))
			}
			// Auto-create palate_profile for new cupper.
			ensureCupperPalateProfile(db, cupper)
			// Validate SCA score scale (6.00–10.00) on each provided attribute.
			for label, v := range map[string]float64{
				"fragrance": fragrance, "flavor": flavor, "aftertaste": aftertaste,
				"acidity": acidity, "body": body, "balance": balance,
				"uniformity": uniformity, "clean-cup": cleanCup, "sweetness": sweetness,
				"overall": overall,
			} {
				if v != 0 && (v < 6 || v > 10) {
					return usageErr(fmt.Errorf("--%s must be 6.00..10.00 (got %.2f)", label, v))
				}
			}
			if defects < 0 {
				return usageErr(fmt.Errorf("--defects must be >= 0 (got %.2f)", defects))
			}
			total := fragrance + flavor + aftertaste + acidity + body + balance +
				uniformity + cleanCup + sweetness + overall - defects
			_, err = db.DB().Exec(
				`INSERT INTO cupping_scores (session_id, bean_slot, cupper_handle,
					fragrance_aroma, flavor, aftertaste, acidity, body, balance,
					uniformity, clean_cup, sweetness, overall, defects, total, notes, scored_at)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
				 ON CONFLICT(session_id, bean_slot, cupper_handle) DO UPDATE SET
					fragrance_aroma=excluded.fragrance_aroma, flavor=excluded.flavor,
					aftertaste=excluded.aftertaste, acidity=excluded.acidity,
					body=excluded.body, balance=excluded.balance,
					uniformity=excluded.uniformity, clean_cup=excluded.clean_cup,
					sweetness=excluded.sweetness, overall=excluded.overall,
					defects=excluded.defects, total=excluded.total,
					notes=excluded.notes, scored_at=excluded.scored_at`,
				sessionID, bean, cupper,
				fragrance, flavor, aftertaste, acidity, body, balance,
				uniformity, cleanCup, sweetness, overall, defects, total,
				nullableString(notes),
				time.Now().UTC().Format(time.RFC3339),
			)
			if err != nil {
				return fmt.Errorf("upsert cupping_score: %w", err)
			}
			out := cuppingScoreRow{
				SessionID: sessionID, BeanSlot: bean, CupperHandle: cupper,
				FragranceAroma: fragrance, Flavor: flavor, Aftertaste: aftertaste,
				Acidity: acidity, Body: body, Balance: balance,
				Uniformity: uniformity, CleanCup: cleanCup, Sweetness: sweetness,
				Overall: overall, Defects: defects, Total: round2(total),
				Notes: notes, ScoredAt: time.Now().UTC().Format(time.RFC3339),
			}
			if force {
				_, _ = db.DB().Exec(
					`INSERT INTO cupping_scores (session_id, bean_slot, cupper_handle, notes, scored_at)
					 VALUES (?, ?, ?, ?, ?)
					 ON CONFLICT(session_id, bean_slot, cupper_handle) DO NOTHING`,
					sessionID, "_amendment", cupper,
					fmt.Sprintf("post-finalize amendment: %s", forceReason),
					time.Now().UTC().Format(time.RFC3339),
				)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "scored session #%d slot %s cupper=%s  total=%.2f\n",
				sessionID, bean, cupper, total)
			return nil
		},
	}
	cmd.Flags().Int64Var(&sessionID, "session", 0, "Cupping session ID (from `cupping start`)")
	cmd.Flags().StringVar(&bean, "bean", "", "Bean slot (A/B/C in blind mode, or roaster/handle in open mode)")
	cmd.Flags().StringVar(&cupper, "cupper", "self", "Cupper handle (default: self; other handles auto-create palate_profiles rows)")
	cmd.Flags().Float64Var(&fragrance, "fragrance", 0, "Fragrance/Aroma 6.00–10.00")
	cmd.Flags().Float64Var(&flavor, "flavor", 0, "Flavor 6.00–10.00")
	cmd.Flags().Float64Var(&aftertaste, "aftertaste", 0, "Aftertaste 6.00–10.00")
	cmd.Flags().Float64Var(&acidity, "acidity", 0, "Acidity 6.00–10.00")
	cmd.Flags().Float64Var(&body, "body", 0, "Body 6.00–10.00")
	cmd.Flags().Float64Var(&balance, "balance", 0, "Balance 6.00–10.00")
	cmd.Flags().Float64Var(&uniformity, "uniformity", 0, "Uniformity 6.00–10.00 (or 10 for single-cup full marks)")
	cmd.Flags().Float64Var(&cleanCup, "clean-cup", 0, "Clean Cup 6.00–10.00 (or 10 for single-cup full marks)")
	cmd.Flags().Float64Var(&sweetness, "sweetness", 0, "Sweetness 6.00–10.00 (or 10 for single-cup full marks)")
	cmd.Flags().Float64Var(&overall, "overall", 0, "Overall 6.00–10.00")
	cmd.Flags().Float64Var(&defects, "defects", 0, "Defect penalty (taint=2/fault=4 × N defective cups)")
	cmd.Flags().StringVar(&notes, "notes", "", "Free-text tasting notes")
	cmd.Flags().BoolVar(&force, "force", false, "Required to amend a finalized session")
	cmd.Flags().StringVar(&forceReason, "reason", "", "Reason text for post-finalize amendment (with --force)")
	return cmd
}

func newCuppingFinalizeCmd(flags *rootFlags) *cobra.Command {
	var sessionID int64
	cmd := &cobra.Command{
		Use:     "finalize",
		Short:   "Lock a session: require ≥1 score per bean, reveal blind labels, bridge self-scores to brews",
		Example: `  coffee-goat-pp-cli cupping finalize --session 1`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if sessionID == 0 {
				return usageErr(fmt.Errorf("cupping finalize requires --session"))
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			if err := ensureCuppingSchema(db); err != nil {
				return err
			}
			state, err := lookupSessionState(db, sessionID)
			if err != nil {
				return err
			}
			if state == "finalized" {
				return usageErr(fmt.Errorf("session #%d is already finalized", sessionID))
			}
			if state == "abandoned" {
				return usageErr(fmt.Errorf("session #%d is abandoned", sessionID))
			}
			beans, err := loadSessionBeans(db, sessionID)
			if err != nil {
				return err
			}
			if len(beans) == 0 {
				return usageErr(fmt.Errorf("session #%d has no beans", sessionID))
			}
			// Coverage gate: ≥1 score per declared bean.
			for _, b := range beans {
				var count int
				_ = db.DB().QueryRow(
					`SELECT COUNT(*) FROM cupping_scores WHERE session_id=? AND bean_slot=?`,
					sessionID, b.SlotLabel,
				).Scan(&count)
				if count == 0 {
					return usageErr(fmt.Errorf("session #%d cannot finalize: bean %q has no scores", sessionID, b.SlotLabel))
				}
			}
			now := time.Now().UTC().Format(time.RFC3339)
			if _, err := db.DB().Exec(
				`UPDATE cupping_sessions SET state='finalized', finalized_at=? WHERE id=?`,
				now, sessionID,
			); err != nil {
				return fmt.Errorf("finalize session: %w", err)
			}
			// Reveal blind labels: copy slot_label from bean_ref into display_label.
			if _, err := db.DB().Exec(
				`UPDATE cupping_session_beans SET display_label = bean_ref WHERE session_id = ? AND (display_label IS NULL OR display_label = '')`,
				sessionID,
			); err != nil {
				return fmt.Errorf("reveal blind labels: %w", err)
			}
			// Bridge: for each self-scored bean, write a brew row.
			bridgedBrewIDs, err := bridgeSessionToBrews(db, sessionID, beans)
			if err != nil {
				return err
			}
			result := map[string]any{
				"session_id":       sessionID,
				"finalized_at":     now,
				"bridged_brew_ids": bridgedBrewIDs,
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "finalized session #%d (bridged %d brew rows for cupper=self)\n",
				sessionID, len(bridgedBrewIDs))
			return nil
		},
	}
	cmd.Flags().Int64Var(&sessionID, "session", 0, "Cupping session ID")
	return cmd
}

func newCuppingAbandonCmd(flags *rootFlags) *cobra.Command {
	var sessionID int64
	cmd := &cobra.Command{
		Use:     "abandon",
		Short:   "Mark a session abandoned (does not delete rows; just hides from active workflows)",
		Example: `  coffee-goat-pp-cli cupping abandon --session 1`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if sessionID == 0 {
				return usageErr(fmt.Errorf("cupping abandon requires --session"))
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			if err := ensureCuppingSchema(db); err != nil {
				return err
			}
			res, err := db.DB().Exec(
				`UPDATE cupping_sessions SET state='abandoned' WHERE id=? AND state='active'`,
				sessionID,
			)
			if err != nil {
				return fmt.Errorf("abandon: %w", err)
			}
			n, _ := res.RowsAffected()
			if n == 0 {
				return notFoundErr(fmt.Errorf("session #%d not active", sessionID))
			}
			fmt.Fprintf(cmd.OutOrStdout(), "abandoned session #%d\n", sessionID)
			return nil
		},
	}
	cmd.Flags().Int64Var(&sessionID, "session", 0, "Cupping session ID")
	return cmd
}

func newCuppingShowCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "show <session-id>",
		Short:       "Show one session: header, beans (with current blind/open state), and all scores",
		Example:     `  coffee-goat-pp-cli cupping show 1 --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			sessionID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return usageErr(fmt.Errorf("session id must be an integer (got %q)", args[0]))
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			if err := ensureCuppingSchema(db); err != nil {
				return err
			}
			session, err := loadFullSession(db, sessionID)
			if err != nil {
				return err
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), session, flags)
			}
			renderSession(cmd, session)
			return nil
		},
	}
	return cmd
}

func newCuppingListCmd(flags *rootFlags) *cobra.Command {
	var state string
	cmd := &cobra.Command{
		Use:         "list",
		Short:       "List cupping sessions (active by default; --state finalized|abandoned|all to widen)",
		Example:     `  coffee-goat-pp-cli cupping list --state all --agent`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			if err := ensureCuppingSchema(db); err != nil {
				return err
			}
			q := `SELECT id, COALESCE(name,''), blind_mode, state, started_at, COALESCE(finalized_at,''), COALESCE(notes,'')
			      FROM cupping_sessions`
			switch strings.ToLower(state) {
			case "", "active":
				q += ` WHERE state='active'`
			case "finalized":
				q += ` WHERE state='finalized'`
			case "abandoned":
				q += ` WHERE state='abandoned'`
			case "all":
				// no filter
			default:
				return usageErr(fmt.Errorf("--state must be active, finalized, abandoned, or all (got %q)", state))
			}
			q += ` ORDER BY started_at DESC`
			rows, err := db.DB().Query(q)
			if err != nil {
				return err
			}
			defer rows.Close()
			var out []cuppingSession
			for rows.Next() {
				var s cuppingSession
				var blindInt int
				if err := rows.Scan(&s.ID, &s.Name, &blindInt, &s.State, &s.StartedAt, &s.FinalizedAt, &s.Notes); err != nil {
					return err
				}
				s.BlindMode = blindInt == 1
				out = append(out, s)
			}
			if err := rows.Err(); err != nil {
				return fmt.Errorf("iterate cupping_sessions rows: %w", err)
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			if len(out) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no sessions")
				return nil
			}
			for _, s := range out {
				blindMark := ""
				if s.BlindMode {
					blindMark = " (blind)"
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  #%d  %s  %s%s  %s\n",
					s.ID, s.State, ifEmpty(s.Name, "(unnamed)"), blindMark, s.StartedAt)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&state, "state", "active", "Filter: active, finalized, abandoned, all")
	return cmd
}

func newCuppingLogCmd(flags *rootFlags) *cobra.Command {
	var path string
	cmd := &cobra.Command{
		Use:     "log",
		Short:   "Atomic alternative to start/score/finalize: load a full session from a JSON file",
		Example: `  coffee-goat-pp-cli cupping log --file session.json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if path == "" {
				return usageErr(fmt.Errorf("cupping log requires --file"))
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read %s: %w", path, err)
			}
			var spec cuppingLogSpec
			if err := json.Unmarshal(data, &spec); err != nil {
				return usageErr(fmt.Errorf("parse %s: %w", path, err))
			}
			db, err := store.OpenWithContext(cmd.Context(), defaultDBPath("coffee-goat-pp-cli"))
			if err != nil {
				return err
			}
			defer db.Close()
			if err := ensureCuppingSchema(db); err != nil {
				return err
			}
			sessionID, err := applyCuppingLogSpec(db, spec)
			if err != nil {
				return err
			}
			result := map[string]any{"session_id": sessionID, "loaded_from": path}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "loaded session #%d from %s\n", sessionID, path)
			return nil
		},
	}
	cmd.Flags().StringVar(&path, "file", "", "Path to JSON session spec (required)")
	return cmd
}

// cuppingLogSpec is the atomic file shape for `cupping log --file`.
type cuppingLogSpec struct {
	Name     string                 `json:"name"`
	Blind    bool                   `json:"blind"`
	Notes    string                 `json:"notes,omitempty"`
	Beans    []string               `json:"beans"`
	Scores   []cuppingLogScoreEntry `json:"scores"`
	Finalize bool                   `json:"finalize"`
}

type cuppingLogScoreEntry struct {
	Bean           string  `json:"bean"`
	Cupper         string  `json:"cupper,omitempty"`
	FragranceAroma float64 `json:"fragrance_aroma"`
	Flavor         float64 `json:"flavor"`
	Aftertaste     float64 `json:"aftertaste"`
	Acidity        float64 `json:"acidity"`
	Body           float64 `json:"body"`
	Balance        float64 `json:"balance"`
	Uniformity     float64 `json:"uniformity"`
	CleanCup       float64 `json:"clean_cup"`
	Sweetness      float64 `json:"sweetness"`
	Overall        float64 `json:"overall"`
	Defects        float64 `json:"defects"`
	Notes          string  `json:"notes,omitempty"`
}

// applyCuppingLogSpec performs start → score* → optional finalize as
// one atomic transaction (best-effort: SQLite is one connection).
func applyCuppingLogSpec(db *store.Store, spec cuppingLogSpec) (int64, error) {
	if len(spec.Beans) == 0 {
		return 0, usageErr(fmt.Errorf("log spec must declare beans"))
	}
	blindInt := 0
	if spec.Blind {
		blindInt = 1
	}
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := db.DB().Exec(
		`INSERT INTO cupping_sessions (name, blind_mode, state, started_at, notes) VALUES (?, ?, 'active', ?, ?)`,
		nullableString(spec.Name), blindInt, now, nullableString(spec.Notes),
	)
	if err != nil {
		return 0, err
	}
	sessionID, _ := res.LastInsertId()
	slotByBean := map[string]string{}
	for i, beanRef := range spec.Beans {
		slot := beanRef
		display := beanRef
		if spec.Blind {
			slot = slotLetter(i)
			display = ""
		}
		slotByBean[beanRef] = slot
		roaster, handle := splitRoasterHandle(beanRef)
		localID := lookupLocalBeanID(db, roaster, handle)
		if _, err := db.DB().Exec(
			`INSERT INTO cupping_session_beans (session_id, bean_ref, roaster_slug, product_slug, bean_id, slot_label, display_label)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`,
			sessionID, beanRef,
			nullableString(roaster), nullableString(handle),
			nullableInt64(localID),
			slot, nullableString(display),
		); err != nil {
			return sessionID, err
		}
	}
	for _, sc := range spec.Scores {
		cupper := strings.ToLower(strings.TrimSpace(sc.Cupper))
		if cupper == "" {
			cupper = "self"
		}
		ensureCupperPalateProfile(db, cupper)
		slot, ok := slotByBean[sc.Bean]
		if !ok {
			slot = sc.Bean // assume user passed slot label directly
		}
		total := sc.FragranceAroma + sc.Flavor + sc.Aftertaste + sc.Acidity + sc.Body +
			sc.Balance + sc.Uniformity + sc.CleanCup + sc.Sweetness + sc.Overall - sc.Defects
		if _, err := db.DB().Exec(
			`INSERT INTO cupping_scores (session_id, bean_slot, cupper_handle,
				fragrance_aroma, flavor, aftertaste, acidity, body, balance,
				uniformity, clean_cup, sweetness, overall, defects, total, notes, scored_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
			 ON CONFLICT(session_id, bean_slot, cupper_handle) DO UPDATE SET
				fragrance_aroma=excluded.fragrance_aroma, flavor=excluded.flavor,
				aftertaste=excluded.aftertaste, acidity=excluded.acidity,
				body=excluded.body, balance=excluded.balance,
				uniformity=excluded.uniformity, clean_cup=excluded.clean_cup,
				sweetness=excluded.sweetness, overall=excluded.overall,
				defects=excluded.defects, total=excluded.total`,
			sessionID, slot, cupper,
			sc.FragranceAroma, sc.Flavor, sc.Aftertaste, sc.Acidity, sc.Body, sc.Balance,
			sc.Uniformity, sc.CleanCup, sc.Sweetness, sc.Overall, sc.Defects, total,
			nullableString(sc.Notes),
			time.Now().UTC().Format(time.RFC3339),
		); err != nil {
			return sessionID, err
		}
	}
	if spec.Finalize {
		beans, _ := loadSessionBeans(db, sessionID)
		// Coverage gate
		for _, b := range beans {
			var n int
			_ = db.DB().QueryRow(
				`SELECT COUNT(*) FROM cupping_scores WHERE session_id=? AND bean_slot=?`,
				sessionID, b.SlotLabel,
			).Scan(&n)
			if n == 0 {
				return sessionID, usageErr(fmt.Errorf("cannot finalize: bean %q has no scores", b.SlotLabel))
			}
		}
		_, _ = db.DB().Exec(
			`UPDATE cupping_sessions SET state='finalized', finalized_at=? WHERE id=?`,
			now, sessionID,
		)
		_, _ = db.DB().Exec(
			`UPDATE cupping_session_beans SET display_label = bean_ref WHERE session_id = ? AND (display_label IS NULL OR display_label = '')`,
			sessionID,
		)
		_, _ = bridgeSessionToBrews(db, sessionID, beans)
	}
	return sessionID, nil
}

func loadSessionBeans(db *store.Store, sessionID int64) ([]cuppingSessionBean, error) {
	rows, err := db.DB().Query(
		`SELECT id, session_id, bean_ref, COALESCE(roaster_slug,''), COALESCE(product_slug,''),
		        COALESCE(bean_id,0), slot_label, COALESCE(display_label,'')
		 FROM cupping_session_beans WHERE session_id=?`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []cuppingSessionBean
	for rows.Next() {
		var b cuppingSessionBean
		if err := rows.Scan(&b.ID, &b.SessionID, &b.BeanRef, &b.RoasterSlug, &b.ProductSlug,
			&b.BeanID, &b.SlotLabel, &b.DisplayLabel); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func loadFullSession(db *store.Store, sessionID int64) (cuppingSession, error) {
	var s cuppingSession
	var blindInt int
	err := db.DB().QueryRow(
		`SELECT id, COALESCE(name,''), blind_mode, state, started_at, COALESCE(finalized_at,''), COALESCE(notes,'')
		 FROM cupping_sessions WHERE id=?`, sessionID,
	).Scan(&s.ID, &s.Name, &blindInt, &s.State, &s.StartedAt, &s.FinalizedAt, &s.Notes)
	if err == sql.ErrNoRows {
		return s, notFoundErr(fmt.Errorf("cupping session #%d not found", sessionID))
	}
	if err != nil {
		return s, err
	}
	s.BlindMode = blindInt == 1
	s.Beans, err = loadSessionBeans(db, sessionID)
	if err != nil {
		return s, err
	}
	rows, err := db.DB().Query(
		`SELECT id, session_id, bean_slot, cupper_handle,
		        COALESCE(fragrance_aroma,0), COALESCE(flavor,0), COALESCE(aftertaste,0),
		        COALESCE(acidity,0), COALESCE(body,0), COALESCE(balance,0),
		        COALESCE(uniformity,0), COALESCE(clean_cup,0), COALESCE(sweetness,0),
		        COALESCE(overall,0), COALESCE(defects,0), COALESCE(total,0),
		        COALESCE(notes,''), COALESCE(scored_at,'')
		 FROM cupping_scores WHERE session_id=? ORDER BY bean_slot, cupper_handle`,
		sessionID,
	)
	if err != nil {
		return s, err
	}
	defer rows.Close()
	for rows.Next() {
		var r cuppingScoreRow
		if err := rows.Scan(&r.ID, &r.SessionID, &r.BeanSlot, &r.CupperHandle,
			&r.FragranceAroma, &r.Flavor, &r.Aftertaste, &r.Acidity, &r.Body, &r.Balance,
			&r.Uniformity, &r.CleanCup, &r.Sweetness, &r.Overall, &r.Defects, &r.Total,
			&r.Notes, &r.ScoredAt); err != nil {
			return s, err
		}
		s.Scores = append(s.Scores, r)
	}
	return s, rows.Err()
}

func lookupSessionState(db *store.Store, sessionID int64) (string, error) {
	var state string
	err := db.DB().QueryRow(`SELECT state FROM cupping_sessions WHERE id=?`, sessionID).Scan(&state)
	if err == sql.ErrNoRows {
		return "", notFoundErr(fmt.Errorf("cupping session #%d not found", sessionID))
	}
	return state, err
}

func lookupLocalBeanID(db *store.Store, roaster, handle string) int64 {
	if roaster == "" || handle == "" {
		return 0
	}
	var id int64
	_ = db.DB().QueryRow(
		`SELECT id FROM beans WHERE roaster_slug=? AND product_slug=? ORDER BY added_at DESC LIMIT 1`,
		roaster, handle,
	).Scan(&id)
	return id
}

// ensureCupperPalateProfile creates an empty palate_profiles row if the
// cupper handle isn't already present. Future cupping scores enrich
// the profile's descriptor signature.
func ensureCupperPalateProfile(db *store.Store, cupperHandle string) {
	if cupperHandle == "" {
		return
	}
	var existing string
	err := db.DB().QueryRow(`SELECT name FROM palate_profiles WHERE name=?`, cupperHandle).Scan(&existing)
	if err == sql.ErrNoRows {
		_, _ = db.DB().Exec(
			`INSERT INTO palate_profiles (name, signature_json, source) VALUES (?, '{}', 'cupping')`,
			cupperHandle,
		)
	}
}

// bridgeSessionToBrews iterates self-scored beans and writes one brew
// row each with method='cupping', rating=round(total/10), notes='cupping-session:<id>'.
func bridgeSessionToBrews(db *store.Store, sessionID int64, beans []cuppingSessionBean) ([]int64, error) {
	var bridged []int64
	for _, b := range beans {
		var total float64
		err := db.DB().QueryRow(
			`SELECT COALESCE(total,0) FROM cupping_scores WHERE session_id=? AND bean_slot=? AND cupper_handle='self'`,
			sessionID, b.SlotLabel,
		).Scan(&total)
		if err == sql.ErrNoRows || total <= 0 {
			continue
		}
		if err != nil && err != sql.ErrNoRows {
			return bridged, err
		}
		rating := int(math.Round(total / 10.0))
		if rating > 10 {
			rating = 10
		}
		if rating < 0 {
			rating = 0
		}
		notes := fmt.Sprintf("cupping-session:%d", sessionID)
		res, err := db.DB().Exec(
			`INSERT INTO brews (bean_id, method, rating, notes, brewed_at) VALUES (?, 'cupping', ?, ?, ?)`,
			nullableInt64(b.BeanID), rating, notes,
			time.Now().UTC().Format(time.RFC3339),
		)
		if err != nil {
			return bridged, fmt.Errorf("bridge to brews: %w", err)
		}
		id, _ := res.LastInsertId()
		bridged = append(bridged, id)
	}
	return bridged, nil
}

func slotLetter(idx int) string {
	if idx < 26 {
		return string('A' + byte(idx))
	}
	return fmt.Sprintf("AA%d", idx-26)
}

func ifEmpty(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func renderSession(cmd *cobra.Command, s cuppingSession) {
	w := cmd.OutOrStdout()
	blindMark := ""
	if s.BlindMode && s.State != "finalized" {
		blindMark = " [blind]"
	}
	fmt.Fprintf(w, "session #%d  %s%s\n", s.ID, ifEmpty(s.Name, "(unnamed)"), blindMark)
	fmt.Fprintf(w, "  state: %s  started: %s", s.State, s.StartedAt)
	if s.FinalizedAt != "" {
		fmt.Fprintf(w, "  finalized: %s", s.FinalizedAt)
	}
	fmt.Fprintln(w)
	if len(s.Beans) > 0 {
		fmt.Fprintln(w, "  beans:")
		for _, b := range s.Beans {
			ident := b.DisplayLabel
			if ident == "" && s.State != "finalized" && s.BlindMode {
				ident = "(hidden)"
			}
			if ident == "" {
				ident = b.BeanRef
			}
			fmt.Fprintf(w, "    %s  %s\n", b.SlotLabel, ident)
		}
	}
	if len(s.Scores) > 0 {
		fmt.Fprintln(w, "  scores:")
		// Group by bean slot.
		bySlot := map[string][]cuppingScoreRow{}
		for _, sc := range s.Scores {
			bySlot[sc.BeanSlot] = append(bySlot[sc.BeanSlot], sc)
		}
		slots := make([]string, 0, len(bySlot))
		for k := range bySlot {
			slots = append(slots, k)
		}
		sort.Strings(slots)
		for _, slot := range slots {
			fmt.Fprintf(w, "    slot %s:\n", slot)
			for _, sc := range bySlot[slot] {
				fmt.Fprintf(w, "      cupper=%s  total=%.2f  (fra=%.2f flav=%.2f after=%.2f acid=%.2f body=%.2f bal=%.2f uni=%.2f clean=%.2f sweet=%.2f over=%.2f def=%.2f)\n",
					sc.CupperHandle, sc.Total,
					sc.FragranceAroma, sc.Flavor, sc.Aftertaste, sc.Acidity,
					sc.Body, sc.Balance, sc.Uniformity, sc.CleanCup,
					sc.Sweetness, sc.Overall, sc.Defects)
			}
		}
	}
}
