// Copyright 2026 Luigi Pastore and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/normattiva/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/normattiva/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/normattiva/internal/norma"
	"github.com/mvanhorn/printing-press-library/library/productivity/normattiva/internal/store"

	"github.com/spf13/cobra"
)

type seguito struct {
	URN             string   `json:"urn"`
	Etichetta       string   `json:"etichetta"`
	Articoli        []string `json:"articoli"`
	AggiuntoIl      string   `json:"aggiunto_il"`
	UltimoControllo string   `json:"ultimo_controllo,omitempty"`
}

func loadSeguiti(ctx context.Context, db *store.Store) ([]seguito, error) {
	rows, err := db.DB().QueryContext(ctx, `SELECT urn, etichetta, articoli, aggiunto_il, ultimo_controllo FROM norma_segui ORDER BY urn`)
	if err != nil {
		return nil, err
	}
	out := make([]seguito, 0)
	for rows.Next() {
		var s seguito
		var arts string
		if err := rows.Scan(&s.URN, &s.Etichetta, &arts, &s.AggiuntoIl, &s.UltimoControllo); err != nil {
			_ = rows.Close()
			return nil, err
		}
		s.Articoli = splitArticoli(arts)
		out = append(out, s)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return nil, err
	}
	return out, rows.Close()
}

func splitArticoli(s string) []string {
	out := make([]string, 0)
	for _, a := range strings.Split(s, ",") {
		if a = norma.NormalizeArticle(a); a != "" {
			out = append(out, a)
		}
	}
	return out
}

type snapshot struct {
	hash, inizio, fine string
}

func loadSnapshot(ctx context.Context, db *store.Store, urn string) (snapshot, bool, error) {
	var s snapshot
	err := db.DB().QueryRowContext(ctx, `SELECT hash, inizio_vigenza, fine_vigenza FROM norma_snapshot WHERE urn = ?`, urn).Scan(&s.hash, &s.inizio, &s.fine)
	if errors.Is(err, sql.ErrNoRows) {
		return s, false, nil
	}
	return s, err == nil, err
}

func saveSnapshot(ctx context.Context, db *store.Store, actURN string, v articleView) error {
	_, err := db.DB().ExecContext(ctx, `INSERT INTO norma_snapshot (urn, atto_urn, hash, inizio_vigenza, fine_vigenza, testo, visto_il)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(urn) DO UPDATE SET hash=excluded.hash, inizio_vigenza=excluded.inizio_vigenza, fine_vigenza=excluded.fine_vigenza, testo=excluded.testo, visto_il=excluded.visto_il`,
		v.URN, actURN, v.Hash, v.InizioVigenza, v.FineVigenza, v.Testo, timeNow().UTC().Format(time.RFC3339))
	return err
}

func newSeguiCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "segui",
		Short: "Manage your local watchlist of acts (and articles) for 'novita'",
		Long: strings.Trim(`
Keep a local list of the acts you are responsible for. 'novita' then reports
which of them were amended since the last check and, for the articles you pin
with --articoli, exactly which articles changed.`, "\n"),
		Example: strings.Trim(`
  normattiva-pp-cli segui add "d.lgs. 231/2001" --articoli 5,6,24
  normattiva-pp-cli segui add gdpr
  normattiva-pp-cli segui list`, "\n"),
	}
	cmd.AddCommand(newSeguiAddCmd(flags), newSeguiLsCmd(flags), newSeguiRmCmd(flags))
	return cmd
}

func newSeguiAddCmd(flags *rootFlags) *cobra.Command {
	var articoli, etichetta string
	cmd := &cobra.Command{
		Use:   "add [atto]",
		Short: "Follow an act; pin articles with --articoli to track their text",
		Example: strings.Trim(`
  normattiva-pp-cli segui add "d.lgs. 231/2001" --articoli 5,6,24
  normattiva-pp-cli segui add "codice privacy" --etichetta "privacy"`, "\n"),
		Annotations: map[string]string{"pp:data-source": "live", "pp:happy-args": "atto=costituzione;--articoli=1"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "segui add")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(errors.New("an act is required"))
			}
			u, cit, err := resolveReference(strings.Join(args, " "), "")
			if err != nil {
				return usageErr(err)
			}
			arts := splitArticoli(articoli)
			if cit.Articolo != "" {
				arts = append(arts, cit.Articolo)
			}
			// Adding articles to an act already followed extends its list.
			if db0, derr := normaStore(cmd.Context()); derr == nil {
				var prev string
				if db0.DB().QueryRowContext(cmd.Context(), `SELECT articoli FROM norma_segui WHERE urn = ?`, u.Act().String()).Scan(&prev) == nil {
					seen := map[string]bool{}
					merged := make([]string, 0)
					for _, a := range append(splitArticoli(prev), arts...) {
						if !seen[a] {
							seen[a] = true
							merged = append(merged, a)
						}
					}
					arts = merged
				}
				_ = db0.Close()
			}
			act := u.Act()
			if etichetta == "" {
				cit.Articolo = ""
				etichetta = cit.Label()
				for _, f := range norma.Fonti {
					if f.Key == cit.Fonte {
						etichetta = f.Nome
					}
				}
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			// Confirm the act exists and take baseline snapshots of pinned articles.
			base, err := fetchArticle(ctx, c, flags, act)
			if err != nil {
				return articleErr(cmd, err, flags)
			}
			views := make([]articleView, 0, len(arts))
			for _, a := range arts {
				v, ferr := fetchArticle(ctx, c, flags, act.WithArticle(a))
				if ferr != nil {
					return articleErr(cmd, fmt.Errorf("article %s: %w", norma.DisplayArticle(a), ferr), flags)
				}
				views = append(views, v)
			}
			db, err := normaStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			now := timeNow().UTC().Format(time.RFC3339)
			if _, err := db.DB().ExecContext(ctx, `INSERT INTO norma_segui (urn, etichetta, articoli, aggiunto_il, ultimo_controllo) VALUES (?, ?, ?, ?, ?)
				ON CONFLICT(urn) DO UPDATE SET etichetta=excluded.etichetta, articoli=excluded.articoli`,
				act.String(), etichetta, strings.Join(arts, ","), now, now); err != nil {
				return fmt.Errorf("saving watchlist: %w", err)
			}
			for _, v := range views {
				if err := saveSnapshot(ctx, db, act.String(), v); err != nil {
					return fmt.Errorf("saving snapshot: %w", err)
				}
			}
			s := seguito{URN: act.String(), Etichetta: etichetta, Articoli: arts, AggiuntoIl: now, UltimoControllo: now}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), s, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "following %s (%s)", etichetta, base.Titolo)
			if len(arts) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), ", articles %s", strings.Join(arts, ", "))
			}
			fmt.Fprintln(cmd.OutOrStdout())
			return nil
		},
	}
	cmd.Flags().StringVar(&articoli, "articoli", "", "Comma-separated articles to track word for word, e.g. 5,6,24-bis")
	cmd.Flags().StringVar(&etichetta, "etichetta", "", "Your own label for the act")
	return cmd
}

func newSeguiLsCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "list",
		Aliases:     []string{"ls"},
		Short:       "List the acts you follow",
		Example:     "  normattiva-pp-cli segui list --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "segui list")
			}
			if flags.dataSource == "live" {
				return usageErr(errors.New("segui list reads the local watchlist; there is no live equivalent"))
			}
			db, err := normaStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			list, err := loadSeguiti(cmd.Context(), db)
			if err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), list, flags)
			}
			if len(list) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Watchlist is empty. Add an act with: normattiva-pp-cli segui add \"d.lgs. 231/2001\"")
				return nil
			}
			rows := make([][]string, 0, len(list))
			for _, s := range list {
				rows = append(rows, []string{s.Etichetta, strings.Join(s.Articoli, ","), s.UltimoControllo, s.URN})
			}
			return flags.printTable(cmd, []string{"ATTO", "ARTICOLI", "ULTIMO CONTROLLO", "URN"}, rows)
		},
	}
}

func newSeguiRmCmd(flags *rootFlags) *cobra.Command {
	return &cobra.Command{
		Use:         "rm [atto]",
		Short:       "Stop following an act",
		Example:     `  normattiva-pp-cli segui rm "d.lgs. 231/2001"`,
		Annotations: map[string]string{"pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "segui rm")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(errors.New("an act is required"))
			}
			u, _, err := resolveReference(strings.Join(args, " "), "")
			if err != nil {
				return usageErr(err)
			}
			db, err := normaStore(cmd.Context())
			if err != nil {
				return err
			}
			defer db.Close()
			act := u.Act().String()
			res, err := db.DB().ExecContext(cmd.Context(), `DELETE FROM norma_segui WHERE urn = ?`, act)
			if err != nil {
				return err
			}
			if _, err := db.DB().ExecContext(cmd.Context(), `DELETE FROM norma_snapshot WHERE atto_urn = ?`, act); err != nil {
				return err
			}
			n, _ := res.RowsAffected()
			if n == 0 {
				return notFoundErr(fmt.Errorf("%s is not in the watchlist", act))
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"rimosso": act}, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "stopped following %s\n", act)
			return nil
		},
	}
}

type articoloCambiato struct {
	Articolo  string `json:"articolo"`
	URN       string `json:"urn"`
	PrimaDal  string `json:"prima_vigente_dal,omitempty"`
	OraDal    string `json:"ora_vigente_dal,omitempty"`
	Abrogato  bool   `json:"abrogato"`
	PrimaHash string `json:"prima_hash,omitempty"`
	OraHash   string `json:"ora_hash"`
	Nuovo     bool   `json:"senza_istantanea_precedente"`
}

type attoCambiato struct {
	URN                string             `json:"urn"`
	Etichetta          string             `json:"etichetta"`
	DataUltimaModifica string             `json:"data_ultima_modifica,omitempty"`
	AttiModificanti    []string           `json:"atti_modificanti,omitempty"`
	ArticoliCambiati   []articoloCambiato `json:"articoli_cambiati"`
	ArticoliInvariati  []string           `json:"articoli_invariati"`
}

type novitaView struct {
	Dal       string         `json:"dal"`
	Al        string         `json:"al"`
	Seguiti   int            `json:"seguiti"`
	Cambiati  []attoCambiato `json:"cambiati"`
	Invariati []string       `json:"invariati"`
	Nota      string         `json:"nota,omitempty"`
	Errori    []string       `json:"fetch_failures,omitempty"`
}

// sameAct compares a watched act URN with a search-result URN by type,
// number and year (search results may carry a different date precision).
func sameAct(watched, found string) bool {
	a, err1 := norma.ParseURN(watched)
	b, err2 := norma.ParseURN(found)
	if err1 != nil || err2 != nil {
		return false
	}
	return a.Tipo == b.Tipo && a.Numero == b.Numero && len(a.Data) >= 4 && len(b.Data) >= 4 && a.Data[:4] == b.Data[:4]
}

func newNovelNovitaCmd(flags *rootFlags) *cobra.Command {
	var since string
	var tutti bool
	cmd := &cobra.Command{
		Use:   "novita",
		Short: "Follow the acts you care about and get a report of which ones and which articles changed since your last check.",
		Long: strings.Trim(`
Report what changed in the acts on your watchlist ('segui add'). The window
starts at your last check (or --since ago on the first run / when given),
the national amended-acts feed is joined against your watchlist, and the
pinned articles of every amended act are re-read and compared with their
stored snapshots. Snapshots and the last-check time are then updated.

Use this command to see which acts on your own watchlist changed since the
last check. Do NOT use it to list every act in Italy updated in a date range;
use 'aggiornati' instead. Do NOT use it to see the wording change of one
article; use 'diff' instead.`, "\n"),
		Example: strings.Trim(`
  normattiva-pp-cli novita
  normattiva-pp-cli novita --since 30d --agent
  normattiva-pp-cli novita --tutti`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--since=7d"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "novita")
			}
			if flags.dataSource == "local" {
				return usageErr(errors.New("novita needs the live amended-acts feed; there is no local-only mode"))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			db, err := normaStore(ctx)
			if err != nil {
				return err
			}
			defer db.Close()
			list, err := loadSeguiti(ctx, db)
			if err != nil {
				return err
			}
			now := timeNow().UTC()
			view := novitaView{Al: now.Format("2006-01-02"), Seguiti: len(list), Cambiati: make([]attoCambiato, 0), Invariati: make([]string, 0)}
			if len(list) == 0 {
				view.Nota = "watchlist is empty; add acts with: normattiva-pp-cli segui add \"d.lgs. 231/2001\" --articoli 5,6"
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), view.Nota)
				return nil
			}
			from := time.Time{}
			if !cmd.Flags().Changed("since") {
				for _, s := range list {
					if t, perr := time.Parse(time.RFC3339, s.UltimoControllo); perr == nil && (from.IsZero() || t.Before(from)) {
						from = t
					}
				}
			}
			if from.IsZero() {
				d, perr := cliutil.ParseDurationLoose(since)
				if perr != nil || d <= 0 {
					return usageErr(fmt.Errorf("--since must be a duration like 7d or 30d (got %q)", since))
				}
				from = now.Add(-d)
			}
			// Back off one day: the feed is dated by day, and an amendment
			// published on the day of the last check must not be missed.
			from = from.Add(-24 * time.Hour)
			view.Dal = from.Format("2006-01-02")
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			feed, err := fetchAggiornati(ctx, c, from, now)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			for _, s := range list {
				var hit *attoRow
				for i := range feed.Risultati {
					if sameAct(s.URN, feed.Risultati[i].URN) {
						hit = &feed.Risultati[i]
						break
					}
				}
				if hit == nil && !tutti {
					view.Invariati = append(view.Invariati, s.Etichetta)
					continue
				}
				changed, failures := compareArticles(ctx, c, flags, db, s)
				view.Errori = append(view.Errori, failures...)
				ac := attoCambiato{URN: s.URN, Etichetta: s.Etichetta, ArticoliCambiati: make([]articoloCambiato, 0), ArticoliInvariati: make([]string, 0)}
				if hit != nil {
					ac.DataUltimaModifica, ac.AttiModificanti = hit.DataUltimaModifica, hit.AttiModificanti
				}
				for _, a := range changed {
					if a.PrimaHash == a.OraHash && !a.Nuovo {
						ac.ArticoliInvariati = append(ac.ArticoliInvariati, a.Articolo)
					} else {
						ac.ArticoliCambiati = append(ac.ArticoliCambiati, a)
					}
				}
				if hit == nil && len(ac.ArticoliCambiati) == 0 {
					view.Invariati = append(view.Invariati, s.Etichetta)
					continue
				}
				view.Cambiati = append(view.Cambiati, ac)
			}
			sort.Strings(view.Invariati)
			if len(view.Errori) == 0 {
				if _, err := db.DB().ExecContext(ctx, `UPDATE norma_segui SET ultimo_controllo = ?`, now.Format(time.RFC3339)); err != nil {
					return fmt.Errorf("updating last check: %w", err)
				}
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "warning: %d article reads failed; last-check time not advanced so the next run retries them\n", len(view.Errori))
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			w := cmd.OutOrStdout()
			fmt.Fprintf(w, "Novità dal %s al %s su %d atti seguiti\n\n", view.Dal, view.Al, view.Seguiti)
			if len(view.Cambiati) == 0 {
				fmt.Fprintln(w, "No followed act changed.")
			}
			for _, a := range view.Cambiati {
				fmt.Fprintf(w, "• %s", cliutil.ScrubTerminal(a.Etichetta))
				if a.DataUltimaModifica != "" {
					fmt.Fprintf(w, " — modificato il %s", a.DataUltimaModifica)
				}
				if len(a.AttiModificanti) > 0 {
					fmt.Fprintf(w, " (atti modificanti: %s)", cliutil.ScrubTerminal(strings.Join(a.AttiModificanti, ", ")))
				}
				fmt.Fprintln(w)
				for _, art := range a.ArticoliCambiati {
					state := "testo cambiato"
					if art.Abrogato {
						state = "abrogato"
					} else if art.Nuovo {
						state = "prima istantanea"
					}
					fmt.Fprintf(w, "    art. %s: %s (vigente dal %s) → normattiva-pp-cli diff %q --articolo %s --dal %s\n", art.Articolo, state, art.OraDal, a.URN, art.Articolo, orToday(art.PrimaDal))
				}
				if len(a.ArticoliInvariati) > 0 {
					fmt.Fprintf(w, "    articoli invariati: %s\n", strings.Join(a.ArticoliInvariati, ", "))
				}
			}
			if len(view.Invariati) > 0 {
				fmt.Fprintf(w, "\nunchanged: %s\n", strings.Join(view.Invariati, "; "))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "Window on first run, or to override the last-check time (e.g. 7d, 30d)")
	cmd.Flags().BoolVar(&tutti, "tutti", false, "Re-read pinned articles of every followed act, even those not in the amended-acts feed")
	return cmd
}

func orToday(d string) string {
	if d == "" {
		return "oggi"
	}
	return d
}

// compareArticles re-reads the pinned articles of s and compares them with
// the stored snapshots, then stores the new snapshots.
func compareArticles(ctx context.Context, c *client.Client, flags *rootFlags, db *store.Store, s seguito) ([]articoloCambiato, []string) {
	act, err := norma.ParseURN(s.URN)
	if err != nil {
		return nil, []string{s.URN + ": " + err.Error()}
	}
	// Preserve the annex of well-known codes.
	for _, f := range norma.Fonti {
		if f.URN == s.URN {
			act = f.Base()
		}
	}
	out := make([]articoloCambiato, 0, len(s.Articoli))
	var failures []string
	for _, a := range s.Articoli {
		u := act.WithArticle(a)
		prev, had, perr := loadSnapshot(ctx, db, u.String())
		if perr != nil {
			failures = append(failures, u.String()+": "+perr.Error())
			continue
		}
		live := *flags
		live.dataSource = "live"
		v, ferr := fetchArticle(ctx, c, &live, u)
		if ferr != nil {
			failures = append(failures, u.String()+": "+ferr.Error())
			continue
		}
		out = append(out, articoloCambiato{Articolo: norma.DisplayArticle(a), URN: u.String(), PrimaDal: prev.inizio, OraDal: v.InizioVigenza, Abrogato: v.Abrogato, PrimaHash: prev.hash, OraHash: v.Hash, Nuovo: !had})
		if err := saveSnapshot(ctx, db, s.URN, v); err != nil {
			failures = append(failures, u.String()+": "+err.Error())
		}
	}
	return out, failures
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newSeguiCmd(flags))
	})
}
