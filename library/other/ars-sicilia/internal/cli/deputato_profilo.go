// pp:data-source live
// pp:client-call
// Novel feature — profilo deputato cross-archive: aggrega tutti gli atti
// firmati (FIRMAT) o pronunciati (ORATOR) da un parlamentare.

package cli

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	icaro "github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/icaroclient"
	"github.com/spf13/cobra"
)

func newNovelDeputatoProfiloCmd(flags *rootFlags) *cobra.Command {
	var (
		flagLegisl int
		flagData   string
		flagLimit  int
	)
	cmd := &cobra.Command{
		Use:     "profilo <nome>",
		Short:   "Aggrega in un'unica vista tutti gli atti firmati o pronunciati da un deputato.",
		Example: "  ars-sicilia-pp-cli deputato profilo \"Rossi Mario\" --legisl 18 --data 2024-01-01:2024-12-31 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "nome=Abbate Ignazio;--legisl=18",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			name := strings.TrimSpace(strings.Join(args, " "))
			if name == "" {
				return fmt.Errorf("nome del deputato richiesto (es. \"Rossi Mario\")")
			}
			if dryRunOK(flags) {
				return emitDeputatoProfiloDryRun(cmd, name, flagLegisl, flagData)
			}
			return runDeputatoProfilo(cmd, flags, name, flagLegisl, flagData, flagLimit)
		},
	}
	cmd.Flags().IntVar(&flagLegisl, "legisl", 0, "Legislatura (es. 18). 0 = tutte le legislature.")
	cmd.Flags().StringVar(&flagData, "data", "", "Data di presentazione/seduta (YYYY-MM-DD; range con YYYY-MM-DD:YYYY-MM-DD). Filtra tutti i sotto-archivi su DATPRE/DATSED.")
	cmd.Flags().IntVar(&flagLimit, "limit", 30, "Max risultati per archivio.")
	return cmd
}

type profileItem struct {
	Tipo     string `json:"tipo"`
	Archivio string `json:"archivio"`
	// omitempty perché i record del backend /bd/ (resoconti) non hanno un
	// DocID Icaro: meglio assente che un fuorviante doc_id: 0 (vedi emitRecords).
	DocID  int    `json:"doc_id,omitempty"`
	Numero string `json:"numero,omitempty"`
	Data   string `json:"data,omitempty"`
	Titolo string `json:"titolo"`
	URL    string `json:"url,omitempty"`
}

type profileReport struct {
	Deputato  string         `json:"deputato"`
	Legisl    int            `json:"legisl,omitempty"`
	Data      string         `json:"data,omitempty"`
	Conteggio map[string]int `json:"conteggio"`
	// Troncato lists the archive slugs where Conteggio is a --limit cap, not
	// the true total: the portal had more matching records than were
	// fetched. Re-run with a higher --limit to see the rest.
	Troncato []string      `json:"troncato,omitempty"`
	Atti     []profileItem `json:"atti"`
}

// profiloSearchParams sono i parametri di una delle ricerche del profilo, in un
// posto solo: il nome viaggia come `firmatario` sugli archivi degli atti e come
// `testo` sui resoconti, dove l'oratore non e' un campo filtrabile. Anteprima e
// ricerca vera devono partire dagli stessi, o la prima smette di descrivere la
// seconda — e con sette archivi la deriva sarebbe anche difficile da vedere.
func profiloSearchParams(campo, name string, legisl int, data string) map[string]string {
	p := map[string]string{campo: name}
	if legisl > 0 {
		p["legisl"] = itoa(legisl)
	}
	if data != "" {
		p["data"] = data
	}
	return p
}

// profiloFirmaArchives sono gli archivi in cui il deputato compare come
// firmatario (campo FIRMAT). Condiviso con l'anteprima --dry-run, che deve
// elencare le stesse richieste che il comando poi fa.
var profiloFirmaArchives = []string{"ddl", "interrogazioni", "interpellanze", "mozioni", "odg", "risoluzioni"}

// emitDeputatoProfiloDryRun elenca le richieste del profilo invece di uscire in
// silenzio con exit 0. Il comando ne fa una per archivio, e l'anteprima serve
// proprio a vedere che il nome viaggia in due modi diversi: come firmatario
// (campo FIRMAT) sui sei archivi degli atti, e come testo libero sui resoconti,
// dove l'oratore non è un campo. È la differenza che spiega perché lo stesso
// nome renda su un archivio e non sull'altro.
func emitDeputatoProfiloDryRun(cmd *cobra.Command, name string, legisl int, data string) error {
	// runDeputatoProfilo passa da normalizeParams su ogni archivio: l'anteprima
	// fa lo stesso, altrimenti annuncia una --data in formato diverso da quello
	// che poi viaggia.
	target := func(slug string, p map[string]string) (map[string]any, error) {
		arc := icaro.BySlug(slug)
		if arc == nil {
			return nil, nil
		}
		return dryRunTargetBySlug(slug, normalizeParams(*arc, p))
	}
	requests := []map[string]any{}
	for _, slug := range profiloFirmaArchives {
		t, err := target(slug, profiloSearchParams("firmatario", name, legisl, data))
		if err != nil {
			return err
		}
		if t != nil {
			requests = append(requests, t)
		}
	}
	t, err := target("resoconti", profiloSearchParams("testo", name, legisl, data))
	if err != nil {
		return err
	}
	if t != nil {
		requests = append(requests, t)
	}
	return emitDryRunRequests(cmd, requests, "una richiesta per archivio: il nome va come firmatario (FIRMAT) sugli archivi degli atti e come testo libero sui resoconti, dove l'oratore non è un campo filtrabile.")
}

func runDeputatoProfilo(cmd *cobra.Command, flags *rootFlags, name string, legisl int, data string, perArchive int) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	if perArchive <= 0 {
		perArchive = 30
	}
	report := profileReport{
		Deputato:  name,
		Legisl:    legisl,
		Data:      data,
		Conteggio: map[string]int{},
	}

	// Archivi contattati con successo (Search senza errore). Serve a distinguere
	// "nessun atto trovato" da "nessun archivio raggiungibile" (errori di rete).
	archivesContacted := 0

	// Archivi con FIRMAT.
	for _, slug := range profiloFirmaArchives {
		arc := icaro.BySlug(slug)
		if arc == nil {
			continue
		}
		c, err := icaro.New(nil)
		if err != nil {
			continue
		}
		params := profiloSearchParams("firmatario", name, legisl, data)
		var truncated bool
		recs, err := c.Search(ctx, *arc, icaro.SearchOptions{
			Params:    normalizeParams(*arc, params),
			Limit:     perArchive,
			MaxPages:  maxInt(1, (perArchive+9)/10),
			Truncated: &truncated,
		})
		if err != nil {
			// Un archivio che non risponde non deve far cadere il report, ma un
			// valore che l'utente ha scritto male non e' un archivio giu': se lo
			// si scarta come gli altri, il report finisce in «nessun atto trovato
			// … verifica il nome», che accusa la cosa sbagliata.
			if invalido := new(icaro.InvalidParamError); errors.As(err, &invalido) {
				return usageErr(err)
			}
			continue
		}
		archivesContacted++
		for _, r := range recs {
			report.Atti = append(report.Atti, profileItem{
				Tipo:     slug,
				Archivio: arc.ID,
				DocID:    r.DocID,
				Numero:   r.Fields["Numero"],
				Data:     r.Fields["Data"],
				Titolo:   r.Title,
				URL:      r.URL,
			})
			report.Conteggio[slug]++
		}
		if truncated {
			report.Troncato = append(report.Troncato, slug)
		}
	}

	// Resoconti d'aula con free-text match sul nome dell'oratore.
	if arc := icaro.BySlug("resoconti"); arc != nil {
		// Un errore di init non deve azzerare gli atti già raccolti sopra:
		// si salta solo questo archivio.
		if c, err := icaro.New(nil); err == nil {
			params := profiloSearchParams("testo", name, legisl, data)
			var truncated bool
			recs, err := c.Search(ctx, *arc, icaro.SearchOptions{
				Params:    normalizeParams(*arc, params),
				Limit:     perArchive,
				MaxPages:  maxInt(1, (perArchive+9)/10),
				Truncated: &truncated,
			})
			if invalido := new(icaro.InvalidParamError); errors.As(err, &invalido) {
				return usageErr(err)
			}
			if err == nil {
				archivesContacted++
				for _, r := range recs {
					report.Atti = append(report.Atti, profileItem{
						Tipo:     "resoconti",
						Archivio: arc.ID,
						DocID:    r.DocID,
						Numero:   r.Fields["Numero"],
						Data:     r.Fields["Data"],
						Titolo:   r.Title,
						URL:      r.URL,
					})
					report.Conteggio["resoconti"]++
				}
				if truncated {
					report.Troncato = append(report.Troncato, "resoconti")
				}
			}
		}
	}

	// Distinguo "deputato non trovato" da "archivi non raggiungibili": senza
	// alcun archivio contattato con successo, un report vuoto è quasi certamente
	// un problema di connettività, non un nome inesistente.
	if len(report.Atti) == 0 {
		if archivesContacted == 0 {
			return fmt.Errorf("impossibile contattare gli archivi ARS per il deputato %q (problema di rete o servizio non raggiungibile?)", name)
		}
		return notFoundErr(fmt.Errorf("nessun atto trovato per il deputato %q (verifica il nome e l'eventuale --legisl)", name))
	}

	// Sort by date (reverse chronological). La chiave passa da chiaveData, non
	// dalla stringa grezza: gli atti dei tre archivi serviti dal backend /bd/
	// scrivono la data come `05/08/2026` e nel confronto lessicografico
	// battevano le date già normalizzate ("28" > "20"), finendo in testa a atti
	// più recenti.
	sort.SliceStable(report.Atti, func(i, j int) bool {
		return chiaveData(report.Atti[i].Data) > chiaveData(report.Atti[j].Data)
	})

	out := cmd.OutOrStdout()
	if flags.asJSON || !isTerminal(out) {
		return printJSONFiltered(out, report, flags)
	}
	fmt.Fprintf(out, "Deputato: %s\n", report.Deputato)
	if report.Legisl > 0 {
		fmt.Fprintf(out, "Legislatura: %d\n", report.Legisl)
	}
	if report.Data != "" {
		fmt.Fprintf(out, "Data: %s\n", report.Data)
	}
	fmt.Fprintf(out, "\nConteggi per archivio:\n")
	troncato := map[string]bool{}
	for _, slug := range report.Troncato {
		troncato[slug] = true
	}
	for k, v := range report.Conteggio {
		suffix := ""
		if troncato[k] {
			suffix = " (troncato, aumenta --limit)"
		}
		fmt.Fprintf(out, "  %-15s %d%s\n", k, v, suffix)
	}
	fmt.Fprintf(out, "\nAtti (%d totali):\n", len(report.Atti))
	for _, a := range report.Atti {
		fmt.Fprintf(out, "  [%s] %s  %s\n      %s\n", a.Tipo, a.Data, a.Numero, a.Titolo)
	}
	return nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
