// Copyright 2026 Luigi Pastore and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/normattiva/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/normattiva/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/normattiva/internal/norma"

	"github.com/spf13/cobra"
)

type rawAtto struct {
	CodiceRedazionale     string `json:"codiceRedazionale"`
	DataGU                string `json:"dataGU"`
	NumeroGU              string `json:"numeroGU"`
	DenominazioneAtto     string `json:"denominazioneAtto"`
	NumeroProvvedimento   string `json:"numeroProvvedimento"`
	AnnoProvvedimento     string `json:"annoProvvedimento"`
	MeseProvvedimento     string `json:"meseProvvedimento"`
	GiornoProvvedimento   string `json:"giornoProvvedimento"`
	DescrizioneAtto       string `json:"descrizioneAtto"`
	TitoloAtto            string `json:"titoloAtto"`
	DataEmanazione        string `json:"dataEmanazione"`
	DataUltimaModifica    string `json:"dataUltimaModifica"`
	UltimiAttiModificanti string `json:"ultimiAttiModificanti"`
}

type rawRicerca struct {
	ListaAtti         []rawAtto                  `json:"listaAtti"`
	FacetMap          map[string][]rawFacetValue `json:"facetMap"`
	NumeroPagine      int                        `json:"numeroPagine"`
	NumeroAttiTrovati int                        `json:"numeroAttiTrovati"`
	PaginaCorrente    int                        `json:"paginaCorrente"`
	Message           string                     `json:"message"`
	Code              any                        `json:"code"`
}

type rawFacetValue struct {
	Codice      string `json:"codice"`
	Valore      int    `json:"valore"`
	Descrizione string `json:"descrizione"`
}

// attoRow is one act in search or update results.
type attoRow struct {
	Descrizione        string   `json:"descrizione"`
	Titolo             string   `json:"titolo"`
	Tipo               string   `json:"tipo"`
	Numero             string   `json:"numero"`
	Anno               string   `json:"anno"`
	DataEmanazione     string   `json:"data_emanazione,omitempty"`
	CodiceRedazionale  string   `json:"codice_redazionale"`
	DataGU             string   `json:"data_gu"`
	NumeroGU           string   `json:"numero_gu,omitempty"`
	URN                string   `json:"urn,omitempty"`
	URL                string   `json:"url,omitempty"`
	DataUltimaModifica string   `json:"data_ultima_modifica,omitempty"`
	AttiModificanti    []string `json:"atti_modificanti,omitempty"`
}

func toAttoRow(a rawAtto) attoRow {
	r := attoRow{
		Descrizione:        strings.TrimSpace(a.DescrizioneAtto),
		Titolo:             cleanTitolo(a.TitoloAtto),
		Tipo:               a.DenominazioneAtto,
		Numero:             a.NumeroProvvedimento,
		Anno:               a.AnnoProvvedimento,
		CodiceRedazionale:  a.CodiceRedazionale,
		DataGU:             a.DataGU,
		NumeroGU:           a.NumeroGU,
		DataUltimaModifica: a.DataUltimaModifica,
	}
	if len(a.DataEmanazione) >= 10 {
		r.DataEmanazione = a.DataEmanazione[:10]
	} else if y, m, d := atoiOr0(a.AnnoProvvedimento), atoiOr0(a.MeseProvvedimento), atoiOr0(a.GiornoProvvedimento); y > 0 && m > 0 && d > 0 {
		// The amended-acts feed omits dataEmanazione but keeps its parts.
		r.DataEmanazione = fmt.Sprintf("%04d-%02d-%02d", y, m, d)
	}
	if a.UltimiAttiModificanti != "" {
		r.AttiModificanti = strings.Fields(a.UltimiAttiModificanti)
	}
	if tipo := norma.TipoFromLabel(a.DenominazioneAtto); tipo != "" && r.DataEmanazione != "" {
		u := norma.URN{Tipo: tipo, Data: r.DataEmanazione, Numero: strings.TrimLeft(a.NumeroProvvedimento, "0")}
		if tipo == "costituzione" {
			u.Numero = ""
		}
		r.URN, r.URL = u.String(), u.WebURL()
	}
	return r
}

// cleanTitolo strips the brackets, the trailing "(codice)" and the column
// padding Normattiva leaves in act titles.
func cleanTitolo(t string) string {
	t = strings.TrimSpace(t)
	t = strings.TrimPrefix(t, "[")
	t = strings.TrimSuffix(t, "]")
	t = strings.Join(strings.Fields(t), " ")
	if i := strings.LastIndex(t, " ("); i > 0 && strings.HasSuffix(t, ")") && len(t)-i <= 14 {
		t = strings.TrimSpace(t[:i])
	}
	return t
}

type ricercaView struct {
	Totale    int                        `json:"totale"`
	Pagine    int                        `json:"pagine"`
	Pagina    int                        `json:"pagina"`
	Risultati []attoRow                  `json:"risultati"`
	Faccette  map[string][]rawFacetValue `json:"faccette,omitempty"`
	Modalita  string                     `json:"modalita"`
}

func istante(d string) string { return d + "T00:00:00Z" }

func newCercaCmd(flags *rootFlags) *cobra.Command {
	var titolo, tipo, emanatoDal, emanatoAl, pubDal, pubAl, vigenteAl, classe, ordine string
	var anno, numero, pagina, perPagina int
	var faccette bool
	cmd := &cobra.Command{
		Use:   "cerca [parole]",
		Short: "Search Italian State acts by keyword, title, type, year, number, dates and vigenza",
		Long: strings.Trim(`
Search Normattiva. Words alone run the portal's simple full-text search; any
filter flag switches to advanced search. Each result carries the act's
URN:NIR (when its type has a URN form) so you can read it with 'testo' or
'atto', and its Gazzetta coordinates for 'testo --codice/--data-gu'.`, "\n"),
		Example: strings.Trim(`
  normattiva-pp-cli cerca "intelligenza artificiale" --per-pagina 5
  normattiva-pp-cli cerca --tipo "DECRETO LEGISLATIVO" --anno 2003 --numero 196
  normattiva-pp-cli cerca appalti --pubblicato-dal 2026-01-01 --agent --select risultati.descrizione,risultati.urn`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "parole=privacy;--per-pagina=3", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "cerca")
			}
			if flags.dataSource == "local" {
				return usageErr(errors.New("cerca has no local data source; use 'search' to query cached results offline"))
			}
			if ordine != "recente" && ordine != "vecchio" {
				return usageErr(fmt.Errorf("--ordine must be recente or vecchio (got %q)", ordine))
			}
			if pagina < 1 || perPagina < 1 || perPagina > 100 {
				return usageErr(errors.New("--pagina starts from 1 and --per-pagina must be between 1 and 100"))
			}
			testo := strings.Join(args, " ")
			body := map[string]any{"orderType": ordine, "paginazione": map[string]int{"paginaCorrente": pagina, "numeroElementiPerPagina": perPagina}}
			now := timeNow()
			dateField := func(flag, val, key string) error {
				if val == "" {
					return nil
				}
				d, err := norma.ParseDate(val, now)
				if err != nil {
					return usageErr(fmt.Errorf("--%s: %w", flag, err))
				}
				if key == "vigenza" {
					body[key] = d
				} else {
					body[key] = istante(d)
				}
				return nil
			}
			for _, f := range []struct{ flag, val, key string }{
				{"emanato-dal", emanatoDal, "dataInizioEmanazione"}, {"emanato-al", emanatoAl, "dataFineEmanazione"},
				{"pubblicato-dal", pubDal, "dataInizioPubProvvedimento"}, {"pubblicato-al", pubAl, "dataFinePubProvvedimento"},
				{"vigente-al", vigenteAl, "vigenza"},
			} {
				if err := dateField(f.flag, f.val, f.key); err != nil {
					return err
				}
			}
			advanced := len(body) > 2 || titolo != "" || tipo != "" || classe != "" || anno > 0 || numero > 0
			path, modalita := "/ricerca/semplice", "semplice"
			if advanced {
				path, modalita = "/ricerca/avanzata", "avanzata"
				if testo != "" {
					body["testoRicerca"] = testo
				}
				if titolo != "" {
					body["titoloRicerca"] = titolo
				}
				if tipo != "" {
					body["denominazioneAtto"] = normalizeTipoLabel(tipo)
				}
				if classe != "" {
					body["classeProvvedimento"] = classe
				}
				if anno > 0 {
					body["annoProvvedimento"] = anno
				}
				if numero > 0 {
					body["numeroProvvedimento"] = numero
				}
			} else {
				if testo == "" {
					_ = cmd.Usage()
					return usageErr(errors.New("give search words or at least one filter flag"))
				}
				body["testoRicerca"] = testo
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			res, err := postRicerca(ctx, c, path, body)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			view := ricercaView{Totale: res.NumeroAttiTrovati, Pagine: res.NumeroPagine, Pagina: pagina, Risultati: make([]attoRow, 0, len(res.ListaAtti)), Modalita: modalita}
			for _, a := range res.ListaAtti {
				view.Risultati = append(view.Risultati, toAttoRow(a))
			}
			if faccette {
				view.Faccette = res.FacetMap
			}
			cacheAttoRows(ctx, view.Risultati)
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return printAttoTable(cmd, flags, view.Risultati, fmt.Sprintf("%d atti trovati (pagina %d di %d)", view.Totale, view.Pagina, max(view.Pagine, 1)))
		},
	}
	cmd.Flags().StringVar(&titolo, "titolo", "", "Words that must appear in the act title")
	cmd.Flags().StringVar(&tipo, "tipo", "", "Act type, e.g. LEGGE, \"DECRETO LEGISLATIVO\", DECRETO-LEGGE, d.lgs., dpr (see 'tipologiche denominazioni')")
	cmd.Flags().IntVar(&anno, "anno", 0, "Year the act was enacted")
	cmd.Flags().IntVar(&numero, "numero", 0, "Act number")
	cmd.Flags().StringVar(&emanatoDal, "emanato-dal", "", "Enacted on or after this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&emanatoAl, "emanato-al", "", "Enacted on or before this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&pubDal, "pubblicato-dal", "", "Published in Gazzetta Ufficiale on or after this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&pubAl, "pubblicato-al", "", "Published in Gazzetta Ufficiale on or before this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&vigenteAl, "vigente-al", "", "Only acts in force on this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&classe, "classe", "", "Vigenza class: 1 never amended, 2 amended, 3 repealed")
	cmd.Flags().StringVar(&ordine, "ordine", "recente", "Order: recente (newest first) or vecchio (oldest first)")
	cmd.Flags().IntVar(&pagina, "pagina", 1, "Result page (1-based)")
	cmd.Flags().IntVar(&perPagina, "per-pagina", 20, "Results per page (1-100)")
	cmd.Flags().BoolVar(&faccette, "faccette", false, "Include result counts by year, act type and issuing body")
	return cmd
}

func normalizeTipoLabel(t string) string {
	if c, err := norma.ParseCitation(t+" 1/2000", ""); err == nil && c.Tipo != "" {
		if l, ok := norma.TipoLabel[c.Tipo]; ok {
			return l
		}
	}
	return strings.ToUpper(strings.TrimSpace(t))
}

func postRicerca(ctx context.Context, c *client.Client, path string, body map[string]any) (rawRicerca, error) {
	raw, _, err := c.PostQueryWithParams(ctx, path, nil, body)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 400 {
			var e struct {
				Message string `json:"message"`
				Code    string `json:"code"`
			}
			if json.Unmarshal([]byte(apiErr.Body), &e) == nil && e.Message != "" {
				return rawRicerca{}, usageErr(fmt.Errorf("Normattiva rejected the search: %s (code %s)", strings.TrimSpace(e.Message), e.Code))
			}
		}
		return rawRicerca{}, err
	}
	var res rawRicerca
	if err := json.Unmarshal(raw, &res); err != nil {
		return rawRicerca{}, fmt.Errorf("decoding search results: %w", err)
	}
	return res, nil
}

func cacheAttoRows(ctx context.Context, rows []attoRow) {
	if len(rows) == 0 {
		return
	}
	db, err := normaStore(ctx)
	if err != nil {
		return
	}
	defer db.Close()
	for _, r := range rows {
		if r.CodiceRedazionale == "" {
			continue
		}
		data, _ := json.Marshal(r)
		if err := db.Upsert("atti", r.CodiceRedazionale, data); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not cache search results: %v\n", err)
			return
		}
	}
}

func printAttoTable(cmd *cobra.Command, flags *rootFlags, rows []attoRow, header string) error {
	w := cmd.OutOrStdout()
	if len(rows) == 0 {
		fmt.Fprintln(w, "Nessun atto trovato.")
		return nil
	}
	fmt.Fprintln(w, header)
	table := make([][]string, 0, len(rows))
	for _, r := range rows {
		titolo := r.Titolo
		if len([]rune(titolo)) > 70 {
			titolo = string([]rune(titolo)[:67]) + "..."
		}
		extra := r.CodiceRedazionale
		if r.DataUltimaModifica != "" {
			extra = "mod. " + r.DataUltimaModifica
		}
		table = append(table, []string{cliutil.ScrubTerminal(r.Descrizione), cliutil.ScrubTerminal(titolo), extra})
	}
	return flags.printTable(cmd, []string{"ATTO", "TITOLO", "CODICE/MODIFICA"}, table)
}

type aggiornatiView struct {
	Dal       string    `json:"dal"`
	Al        string    `json:"al"`
	Totale    int       `json:"totale"`
	Finestre  int       `json:"finestre"`
	Risultati []attoRow `json:"risultati"`
}

// fetchAggiornati calls /ricerca/aggiornati, splitting windows longer than
// the service's 12-month limit.
func fetchAggiornati(ctx context.Context, c *client.Client, from, to time.Time) (aggiornatiView, error) {
	view := aggiornatiView{Dal: from.Format("2006-01-02"), Al: to.Format("2006-01-02"), Risultati: make([]attoRow, 0)}
	seen := map[string]bool{}
	for start := from; start.Before(to); {
		end := start.AddDate(0, 11, 28)
		if end.After(to) {
			end = to
		}
		body := map[string]any{"dataInizioAggiornamento": start.UTC().Format(time.RFC3339), "dataFineAggiornamento": end.UTC().Format(time.RFC3339)}
		res, err := postRicerca(ctx, c, "/ricerca/aggiornati", body)
		if err != nil {
			return view, err
		}
		view.Finestre++
		for _, a := range res.ListaAtti {
			r := toAttoRow(a)
			key := r.CodiceRedazionale + "|" + r.DataUltimaModifica
			if seen[key] {
				continue
			}
			seen[key] = true
			view.Risultati = append(view.Risultati, r)
		}
		start = end
	}
	view.Totale = len(view.Risultati)
	return view, nil
}

func parseWindow(since, dal, al string) (time.Time, time.Time, error) {
	now := timeNow().UTC()
	to := now
	if al != "" {
		d, err := norma.ParseDate(al, now)
		if err != nil {
			return time.Time{}, time.Time{}, usageErr(fmt.Errorf("--al: %w", err))
		}
		to, _ = time.Parse("2006-01-02", d)
		to = to.Add(24*time.Hour - time.Second)
	}
	if dal != "" {
		d, err := norma.ParseDate(dal, now)
		if err != nil {
			return time.Time{}, time.Time{}, usageErr(fmt.Errorf("--dal: %w", err))
		}
		from, _ := time.Parse("2006-01-02", d)
		if !from.Before(to) {
			return time.Time{}, time.Time{}, usageErr(errors.New("--dal must be before --al"))
		}
		return from, to, nil
	}
	dur, err := cliutil.ParseDurationLoose(since)
	if err != nil || dur <= 0 {
		return time.Time{}, time.Time{}, usageErr(fmt.Errorf("--since must be a duration like 7d, 2w or 72h (got %q)", since))
	}
	return to.Add(-dur), to, nil
}

func newAggiornatiCmd(flags *rootFlags) *cobra.Command {
	var since, dal, al, apiDal, apiAl string
	cmd := &cobra.Command{
		Use:   "aggiornati",
		Short: "List acts whose consolidated text was amended in a date window",
		Long: strings.Trim(`
List every act whose consolidated (vigente) text changed in a window, with the
date of the change and the amending acts. Windows longer than Normattiva's
12-month limit are split automatically.

Use this command for the national feed of amended acts. Do NOT use it to see
changes to the acts you follow; use 'novita' instead. Newly published acts
are found with 'cerca --pubblicato-dal'.`, "\n"),
		Example: strings.Trim(`
  normattiva-pp-cli aggiornati --since 7d
  normattiva-pp-cli aggiornati --dal 2026-09-01 --al 2026-09-30 --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "--since=3d"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "aggiornati")
			}
			if flags.dataSource == "local" {
				return usageErr(errors.New("aggiornati has no local data source"))
			}
			if dal == "" {
				dal = apiDay(apiDal)
			}
			if al == "" {
				al = apiDay(apiAl)
			}
			from, to, err := parseWindow(since, dal, al)
			if err != nil {
				return err
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			view, err := fetchAggiornati(ctx, c, from, to)
			if err != nil {
				return classifyAPIError(cmd.OutOrStdout(), err, flags)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			return printAttoTable(cmd, flags, view.Risultati, fmt.Sprintf("%d atti aggiornati dal %s al %s", view.Totale, view.Dal, view.Al))
		},
	}
	cmd.Flags().StringVar(&since, "since", "7d", "Window length ending now, e.g. 7d, 2w, 90d")
	cmd.Flags().StringVar(&dal, "dal", "", "Window start date (YYYY-MM-DD); overrides --since")
	cmd.Flags().StringVar(&al, "al", "", "Window end date (YYYY-MM-DD, default today)")
	// Accept the API's own parameter names (as used by 'ricerca aggiornati')
	// so callers who learned them there can reuse them here.
	cmd.Flags().StringVar(&apiDal, "data-inizio-aggiornamento", "", "Alias of --dal using the API parameter name (date or RFC 3339 timestamp)")
	cmd.Flags().StringVar(&apiAl, "data-fine-aggiornamento", "", "Alias of --al using the API parameter name (date or RFC 3339 timestamp)")
	_ = cmd.Flags().MarkHidden("data-inizio-aggiornamento")
	_ = cmd.Flags().MarkHidden("data-fine-aggiornamento")
	return cmd
}

// apiDay reduces an API-style timestamp (2026-10-01T00:00:00Z) to its
// YYYY-MM-DD day so it can feed the --dal/--al window parser.
func apiDay(v string) string {
	v = strings.TrimSpace(v)
	if len(v) > 10 && v[10] == 'T' {
		return v[:10]
	}
	return v
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newCercaCmd(flags))
		addNovelCommandIfAbsent(root, newAggiornatiCmd(flags))
	})
}

func atoiOr0(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}
