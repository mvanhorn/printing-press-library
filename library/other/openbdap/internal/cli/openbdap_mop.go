// Copyright 2026 aborruso and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/openbdap/internal/client"
)

// rigaMOP e' una riga della mappa: per ogni regione e ruolo, il dataset da
// interrogare e il suo identificativo OData.
type rigaMOP struct {
	Regione  string `json:"regione"`
	Famiglia string `json:"famiglia"`
	Titolo   string `json:"titolo"`
	ID       string `json:"id"`
	ODataID  string `json:"odata_id"`
}

func newNovelMopCmd(flags *rootFlags) *cobra.Command {
	var regione, famiglia, dbPath string

	cmd := &cobra.Command{
		Use:   "mop",
		Short: "Quale dataset MOP interrogare per ogni regione e ruolo",
		Long: "La mappa dei dataset di monitoraggio delle opere pubbliche: per ogni regione e per ogni ruolo " +
			"(progetti, gare, partecipanti, pagamenti, piano dei costi, soggetti titolari) indica il dataset e il suo identificativo OData.\n" +
			"Usa questo comando per capire quale dataset MOP interrogare. NON usarlo per elencare i temi del catalogo; usa 'gruppi elenco'.",
		Example: strings.Trim(`
  openbdap-pp-cli mop
  openbdap-pp-cli mop --regione Sicilia
  openbdap-pp-cli mop --famiglia gare --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "mop")
			}
			if flags.dataSource == "live" {
				return usageErr(fmt.Errorf("'mop' legge solo l'archivio locale: allinea il catalogo con 'openbdap-pp-cli allinea'"))
			}
			segnaOrigineLocale(flags)
			elenco, ok, err := datasetLocali(cmd, dbPath)
			if err != nil {
				return err
			}
			righe := make([]rigaMOP, 0)
			if ok {
				for _, d := range datasetMOP(elenco, normalizza(famiglia), regione) {
					righe = append(righe, rigaMOP{
						Regione: d.Regione, Famiglia: d.Famiglia, Titolo: d.Titolo, ID: d.ID, ODataID: d.ODataID,
					})
				}
				sort.Slice(righe, func(i, j int) bool {
					if righe[i].Regione == righe[j].Regione {
						return righe[i].Famiglia < righe[j].Famiglia
					}
					return righe[i].Regione < righe[j].Regione
				})
			}
			risposta := rispostaLocale{Richiesta: strings.TrimSpace(regione + " " + famiglia), Risultati: righe, Trovati: len(righe)}
			if len(righe) == 0 {
				risposta.Nota = "nessun dataset MOP nell'archivio locale: lancia 'openbdap-pp-cli allinea'"
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), risposta, flags)
			}
			if len(righe) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "Nessun dataset MOP nell'archivio locale: lancia 'openbdap-pp-cli allinea'.")
				return nil
			}
			tabella := make([]map[string]any, 0, len(righe))
			for _, r := range righe {
				tabella = append(tabella, map[string]any{
					"regione": r.Regione, "famiglia": r.Famiglia, "odata_id": r.ODataID, "titolo": r.Titolo,
				})
			}
			return printAutoTable(cmd.OutOrStdout(), tabella)
		},
	}
	cmd.Flags().StringVar(&regione, "regione", "", "limita la mappa a una regione")
	cmd.Flags().StringVar(&famiglia, "famiglia", "", "limita la mappa a un ruolo: progetti, gare, partecipanti, pagamenti, piano-costi, soggetti-titolari")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath("openbdap-pp-cli"), "percorso dell'archivio locale")
	return cmd
}

// esitoFamiglia raccoglie le righe trovate in un dataset MOP, con l'errore
// eventuale: una regione che non risponde non deve sparire dal risultato.
type esitoFamiglia struct {
	Famiglia string           `json:"famiglia"`
	Regione  string           `json:"regione"`
	Dataset  string           `json:"dataset"`
	Righe    []map[string]any `json:"righe"`
	Errore   string           `json:"errore,omitempty"`
}

// cercaNeiMOP interroga in parallelo i dataset MOP indicati, filtrando su una
// colonna risolta per nome (per esempio "Codice CUP") in ciascun dataset.
func cercaNeiMOP(ctx context.Context, c *client.Client, datasets []dataset, nomeColonna, valore string, limite int) []esitoFamiglia {
	esiti := make([]esitoFamiglia, len(datasets))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 6)
	for i, d := range datasets {
		wg.Add(1)
		go func(i int, d dataset) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			e := esitoFamiglia{Famiglia: d.Famiglia, Regione: d.Regione, Dataset: d.ODataID, Righe: make([]map[string]any, 0)}
			colonne, err := colonneDataset(ctx, c, d.ODataID)
			if err != nil {
				e.Errore = err.Error()
				esiti[i] = e
				return
			}
			col, ok := risolviColonna(colonne, nomeColonna)
			if !ok {
				e.Errore = fmt.Sprintf("colonna %q assente in questo dataset", nomeColonna)
				esiti[i] = e
				return
			}
			filtro := filtroUguale(col.ID, valore)
			righe, err := righeDataset(ctx, c, d.ODataID, colonne, filtro, nil, limite, 0)
			if err != nil {
				e.Errore = err.Error()
				esiti[i] = e
				return
			}
			e.Righe = righe
			esiti[i] = e
		}(i, d)
	}
	wg.Wait()
	return esiti
}
