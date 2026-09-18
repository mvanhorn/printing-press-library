// Copyright 2026 aborruso and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/openbdap/internal/cliutil"
)

// serieCatalogo raggruppa i dataset che condividono lo stesso titolo una volta
// tolti l'anno e la regione: e' l'unica forma in cui il catalogo espone le serie.
type serieCatalogo struct {
	Serie    string   `json:"serie"`
	Dataset  int      `json:"dataset"`
	Anni     []string `json:"anni,omitempty"`
	Periodi  []string `json:"periodi,omitempty"`
	Regioni  []string `json:"regioni,omitempty"`
	Aggiorn  string   `json:"ultimo_aggiornamento,omitempty"`
	Famiglia string   `json:"famiglia_mop,omitempty"`
}

// coperturaSerie e' una riga della matrice regione per anno.
type coperturaSerie struct {
	Serie   string `json:"serie"`
	Regione string `json:"regione"`
	Anno    string `json:"anno"`
	ID      string `json:"id"`
	Titolo  string `json:"titolo"`
}

func newNovelSerieCmd(flags *rootFlags) *cobra.Command {
	var copertura bool
	var limite int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "serie [testo]",
		Short: "Le annualita', le mensilita' e le regioni disponibili di una serie",
		Long: "Raggruppa i dataset per serie, ricavata dal titolo togliendo anno e regione, e mostra quali annualita' e quali regioni esistono.\n" +
			"Usa questo comando per elencare le annualita' di una serie. NON usarlo per cercare un dataset per parole chiave; usa 'cerca'.",
		Example: strings.Trim(`
  openbdap-pp-cli serie
  openbdap-pp-cli serie "Pagamenti Bilancio dello Stato"
  openbdap-pp-cli serie "Progetti Opere Pubbliche MOP" --copertura
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "testo=Opere", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "serie")
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("'serie' legge solo l'archivio locale: allinea il catalogo con 'openbdap-pp-cli allinea'"))
			}
			filtro := normalizza(strings.Join(args, " "))
			segnaOrigineLocale(flags)
			elenco, ok, err := datasetLocali(cmd, dbPath)
			if err != nil {
				return err
			}
			if !ok {
				segnaOrigineLocale(flags)
				return printJSONFiltered(cmd.OutOrStdout(), rispostaLocale{
					Richiesta: strings.Join(args, " "),
					Risultati: make([]serieCatalogo, 0),
					Nota:      notaSerieVuota,
				}, flags)
			}

			if copertura {
				righe := make([]coperturaSerie, 0)
				for _, d := range elenco {
					if filtro != "" && !strings.Contains(normalizza(d.Serie), filtro) {
						continue
					}
					righe = append(righe, coperturaSerie{Serie: d.Serie, Regione: d.Regione, Anno: d.Periodo, ID: d.ID, Titolo: d.Titolo})
				}
				sort.Slice(righe, func(i, j int) bool {
					if righe[i].Serie != righe[j].Serie {
						return righe[i].Serie < righe[j].Serie
					}
					if righe[i].Regione != righe[j].Regione {
						return righe[i].Regione < righe[j].Regione
					}
					return righe[i].Anno < righe[j].Anno
				})
				if limite > 0 && len(righe) > limite {
					righe = righe[:limite]
				}
				risposta := rispostaLocale{Richiesta: strings.Join(args, " "), Risultati: righe, Trovati: len(righe)}
				if len(righe) == 0 {
					risposta.Nota = notaSerieVuota
				}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), risposta, flags)
				}
				if len(righe) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), notaSerieVuota)
					return nil
				}
				tabella := make([]map[string]any, 0, len(righe))
				for _, r := range righe {
					tabella = append(tabella, map[string]any{"serie": r.Serie, "regione": r.Regione, "anno": r.Anno, "id": r.ID})
				}
				return printAutoTable(cmd.OutOrStdout(), tabella)
			}

			raggruppate := map[string]*serieCatalogo{}
			anni := map[string]map[string]bool{}
			periodi := map[string]map[string]bool{}
			regioni := map[string]map[string]bool{}
			for _, d := range elenco {
				if d.Serie == "" {
					continue
				}
				if filtro != "" && !strings.Contains(normalizza(d.Serie), filtro) {
					continue
				}
				s, ok := raggruppate[d.Serie]
				if !ok {
					s = &serieCatalogo{Serie: d.Serie, Famiglia: d.Famiglia}
					raggruppate[d.Serie] = s
					anni[d.Serie] = map[string]bool{}
					periodi[d.Serie] = map[string]bool{}
					regioni[d.Serie] = map[string]bool{}
				}
				s.Dataset++
				if d.Aggiorn > s.Aggiorn {
					s.Aggiorn = d.Aggiorn
				}
				if d.Anno != "" {
					anni[d.Serie][d.Anno] = true
				}
				if d.Periodo != "" && d.Periodo != d.Anno {
					periodi[d.Serie][d.Periodo] = true
				}
				if d.Regione != "" {
					regioni[d.Serie][d.Regione] = true
				}
			}
			risultati := make([]serieCatalogo, 0, len(raggruppate))
			for nome, s := range raggruppate {
				s.Anni = ordinato(anni[nome])
				s.Periodi = ordinato(periodi[nome])
				s.Regioni = ordinato(regioni[nome])
				risultati = append(risultati, *s)
			}
			sort.Slice(risultati, func(i, j int) bool {
				if risultati[i].Dataset != risultati[j].Dataset {
					return risultati[i].Dataset > risultati[j].Dataset
				}
				return risultati[i].Serie < risultati[j].Serie
			})
			if limite > 0 && len(risultati) > limite {
				risultati = risultati[:limite]
			}
			risposta := rispostaLocale{Richiesta: strings.Join(args, " "), Risultati: risultati, Trovati: len(risultati)}
			if len(risultati) == 0 {
				risposta.Nota = notaSerieVuota
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), risposta, flags)
			}
			if len(risultati) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), notaSerieVuota)
				return nil
			}
			tabella := make([]map[string]any, 0, len(risultati))
			for _, s := range risultati {
				tabella = append(tabella, map[string]any{
					"serie": s.Serie, "dataset": s.Dataset,
					"anni": strings.Join(s.Anni, " "), "regioni": len(s.Regioni),
				})
			}
			return printAutoTable(cmd.OutOrStdout(), tabella)
		},
	}
	cmd.Flags().BoolVar(&copertura, "copertura", false, "mostra la matrice regione per anno invece del riepilogo")
	cmd.Flags().IntVar(&limite, "limite", 50, "numero massimo di righe (0 = tutte)")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath("openbdap-pp-cli"), "percorso dell'archivio locale")
	return cmd
}

// dataAggiornamento legge la data di ultima modifica. Il portale usa la forma
// con microsecondi, ma non su tutti i dataset: senza le alternative le righe
// con un'altra precisione sparirebbero in silenzio.
func dataAggiornamento(valore string) (time.Time, bool) {
	valore = strings.TrimSpace(valore)
	if valore == "" {
		return time.Time{}, false
	}
	formati := []string{
		"2006-01-02T15:04:05.000000",
		"2006-01-02T15:04:05.999999999",
		"2006-01-02T15:04:05",
		time.RFC3339,
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, f := range formati {
		if t, err := time.Parse(f, valore); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func ordinato(m map[string]bool) []string {
	fuori := make([]string, 0, len(m))
	for k := range m {
		fuori = append(fuori, k)
	}
	sort.Strings(fuori)
	return fuori
}

// notaSerieVuota spiega come popolare l'archivio quando non ci sono serie.
const notaSerieVuota = "nessuna serie corrisponde: se l'archivio locale e' vuoto lancia 'openbdap-pp-cli allinea'"

// notaNovitaVuota spiega l'assenza di aggiornamenti recenti.
const notaNovitaVuota = "nessun dataset aggiornato nella finestra indicata: allarga --da oppure lancia 'openbdap-pp-cli allinea'"

// novitaDataset e' un dataset comparso o aggiornato di recente.
type novitaDataset struct {
	Titolo     string `json:"titolo"`
	ID         string `json:"id"`
	Aggiornato string `json:"aggiornato"`
	Serie      string `json:"serie,omitempty"`
	Anno       string `json:"anno,omitempty"`
	Regione    string `json:"regione,omitempty"`
}

func newNovelNovitaCmd(flags *rootFlags) *cobra.Command {
	var da string
	var limite int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "novita",
		Short: "I dataset aggiornati di recente nell'archivio locale",
		Long: "Elenca i dataset il cui metadato di ultima modifica cade nella finestra indicata. " +
			"Il portale non pubblica un flusso di attivita': il confronto e' possibile perche' l'archivio locale conserva le date.\n" +
			"Usa questo comando per vedere cosa e' cambiato. NON usarlo per riallineare i metadati; usa 'allinea'.",
		Example: strings.Trim(`
  openbdap-pp-cli novita
  openbdap-pp-cli novita --da 90d --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "novita")
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("'novita' legge solo l'archivio locale: allinea il catalogo con 'openbdap-pp-cli allinea'"))
			}
			durata, err := cliutil.ParseDurationLoose(da)
			if err != nil {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--da non valido: %w", err))
			}
			soglia := time.Now().Add(-durata)
			elenco, ok, err := datasetLocali(cmd, dbPath)
			if err != nil {
				return err
			}
			righe := make([]novitaDataset, 0)
			if ok {
				for _, d := range elenco {
					quando, ok := dataAggiornamento(d.Aggiorn)
					if !ok {
						continue
					}
					if quando.Before(soglia) {
						continue
					}
					righe = append(righe, novitaDataset{
						Titolo: d.Titolo, ID: d.ID, Aggiornato: d.Aggiorn, Serie: d.Serie, Anno: d.Periodo, Regione: d.Regione,
					})
				}
				sort.Slice(righe, func(i, j int) bool { return righe[i].Aggiornato > righe[j].Aggiornato })
				if limite > 0 && len(righe) > limite {
					righe = righe[:limite]
				}
			}
			risposta := rispostaLocale{Richiesta: da, Risultati: righe, Trovati: len(righe)}
			if len(righe) == 0 {
				risposta.Nota = notaNovitaVuota
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), risposta, flags)
			}
			if len(righe) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "Nessun dataset aggiornato negli ultimi %s.\n", da)
				return nil
			}
			tabella := make([]map[string]any, 0, len(righe))
			for _, r := range righe {
				tabella = append(tabella, map[string]any{"aggiornato": r.Aggiornato, "titolo": r.Titolo, "id": r.ID})
			}
			return printAutoTable(cmd.OutOrStdout(), tabella)
		},
	}
	cmd.Flags().StringVar(&da, "da", "30d", "finestra temporale, per esempio 7d, 4w, 24h")
	cmd.Flags().IntVar(&limite, "limite", 50, "numero massimo di dataset (0 = tutti)")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath("openbdap-pp-cli"), "percorso dell'archivio locale")
	return cmd
}
