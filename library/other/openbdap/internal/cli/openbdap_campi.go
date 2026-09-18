// Copyright 2026 aborruso and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source auto

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/openbdap/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/openbdap/internal/store"
)

const campiTipo = "campi_indice"

// notaCampiVuoti spiega che l'indice degli schemi si popola su richiesta:
// serve una chiamata al servizio per ogni dataset.
const notaCampiVuoti = "nessun campo corrisponde: popola l'indice con 'openbdap-pp-cli campi --aggiorna --tema 172_opere-pubbliche'"

// indiceCampi e' lo schema di un dataset conservato in locale: una chiamata
// OData per dataset e' costosa, quindi l'indice si popola su richiesta.
type indiceCampi struct {
	ODataID string    `json:"odata_id"`
	Titolo  string    `json:"titolo"`
	ID      string    `json:"id"`
	Colonne []colonna `json:"colonne"`
}

// campoTrovato e' una corrispondenza fra un campo cercato e un dataset.
type campoTrovato struct {
	Campo      string `json:"campo"`
	NomeFisico string `json:"nome_fisico"`
	IDFiltro   string `json:"id_filtro"`
	Tipo       string `json:"tipo"`
	Dataset    string `json:"dataset"`
	ODataID    string `json:"odata_id"`
	ID         string `json:"id"`
}

func newNovelCampiCmd(flags *rootFlags) *cobra.Command {
	var aggiorna bool
	var tema string
	var limite, paralleli int
	var dbPath string

	cmd := &cobra.Command{
		Use:   "campi [testo]",
		Short: "In quali dataset esiste un campo, con l'identificativo per i filtri",
		Long: "Cerca un nome di colonna nell'indice locale degli schemi e restituisce i dataset che lo contengono, " +
			"con l'identificativo gia' pronto per il filtro di 'righe'.\n" +
			"Usa questo comando per trovare dove vive un campo. NON usarlo per elencare i campi di un dataset che gia' conosci; usa 'colonne'.",
		Example: strings.Trim(`
  openbdap-pp-cli campi --aggiorna --tema 172_opere-pubbliche
  openbdap-pp-cli campi "codice fiscale"
  openbdap-pp-cli campi cup --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "auto", "pp:happy-args": "testo=cup", "pp:no-error-path-probe": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "campi")
			}
			segnaOrigineLocale(flags)
			db, ok, err := apriStore(cmd, dbPath)
			if err != nil {
				return err
			}
			if !ok {
				segnaOrigineLocale(flags)
				return printJSONFiltered(cmd.OutOrStdout(), rispostaLocale{
					Richiesta: strings.Join(args, " "),
					Risultati: make([]campoTrovato, 0),
					Nota:      notaCampiVuoti,
				}, flags)
			}
			defer db.Close()

			if aggiorna {
				elenco, err := leggiDataset(db)
				if err != nil {
					return err
				}
				var bersagli []dataset
				for _, d := range elenco {
					if d.ODataID == "" {
						continue
					}
					if tema != "" && !strings.Contains(normalizza(d.Tema), normalizza(tema)) {
						continue
					}
					bersagli = append(bersagli, d)
				}
				if len(bersagli) == 0 {
					// Niente da indicizzare: si risponde con l'indice
					// esistente e una nota, non con un errore.
					fmt.Fprintln(cmd.ErrOrStderr(), "nessun dataset con risorsa OData corrisponde: allinea il catalogo o cambia --tema")
					aggiorna = false
				}
				c, err := flags.newClient()
				if err != nil {
					return err
				}
				indicizzati, falliti := aggiornaIndiceCampi(cmd.Context(), c, db, bersagli, paralleli)
				fmt.Fprintf(cmd.ErrOrStderr(), "indicizzati %d dataset su %d\n", indicizzati, len(bersagli))
				if falliti > 0 {
					fmt.Fprintf(cmd.ErrOrStderr(), "attenzione: %d dataset non hanno risposto\n", falliti)
				}
			}

			cercato := normalizza(strings.Join(args, " "))
			righe, err := cercaCampi(db, cercato, limite)
			if err != nil {
				return err
			}
			risposta := rispostaLocale{Richiesta: strings.Join(args, " "), Risultati: righe, Trovati: len(righe)}
			if len(righe) == 0 {
				risposta.Nota = notaCampiVuoti
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), risposta, flags)
			}
			if len(righe) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), notaCampiVuoti)
				return nil
			}
			tabella := make([]map[string]any, 0, len(righe))
			for _, r := range righe {
				tabella = append(tabella, map[string]any{
					"campo": r.Campo, "id_filtro": r.IDFiltro, "tipo": r.Tipo, "dataset": r.Dataset,
				})
			}
			return printAutoTable(cmd.OutOrStdout(), tabella)
		},
	}
	cmd.Flags().BoolVar(&aggiorna, "aggiorna", false, "rileggi gli schemi dal servizio prima di cercare")
	cmd.Flags().StringVar(&tema, "tema", "", "limita l'aggiornamento dell'indice a un tema")
	cmd.Flags().IntVar(&limite, "limite", 50, "numero massimo di corrispondenze (0 = tutte)")
	cmd.Flags().IntVar(&paralleli, "paralleli", 4, "richieste in parallelo durante l'aggiornamento")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath("openbdap-pp-cli"), "percorso dell'archivio locale")
	return cmd
}

// aggiornaIndiceCampi rilegge gli schemi dei dataset indicati e li conserva.
func aggiornaIndiceCampi(ctx context.Context, c *client.Client, db *store.Store, bersagli []dataset, paralleli int) (int, int) {
	if paralleli < 1 {
		paralleli = 1
	}
	type esito struct {
		idx indiceCampi
		err error
	}
	lavori := make(chan dataset)
	esiti := make(chan esito)
	var wg sync.WaitGroup
	for i := 0; i < paralleli; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for d := range lavori {
				colonne, err := colonneDataset(ctx, c, d.ODataID)
				if err != nil {
					esiti <- esito{err: err}
					continue
				}
				for i := range colonne {
					colonne[i].Valori = nil
				}
				esiti <- esito{idx: indiceCampi{ODataID: d.ODataID, Titolo: d.Titolo, ID: d.ID, Colonne: colonne}}
			}
		}()
	}
	go func() {
		defer close(lavori)
		for _, d := range bersagli {
			select {
			case <-ctx.Done():
				return
			case lavori <- d:
			}
		}
	}()
	go func() {
		wg.Wait()
		close(esiti)
	}()

	indicizzati, falliti := 0, 0
	for e := range esiti {
		if e.err != nil {
			falliti++
			continue
		}
		blob, err := json.Marshal(e.idx)
		if err != nil {
			falliti++
			continue
		}
		if err := db.Upsert(campiTipo, e.idx.ODataID, blob); err != nil {
			falliti++
			continue
		}
		indicizzati++
	}
	return indicizzati, falliti
}

// cercaCampi interroga l'indice locale degli schemi.
func cercaCampi(db *store.Store, cercato string, limite int) ([]campoTrovato, error) {
	righe, err := db.List(campiTipo, 0)
	if err != nil {
		return nil, err
	}
	fuori := make([]campoTrovato, 0)
	for _, riga := range righe {
		var idx indiceCampi
		if err := json.Unmarshal(riga, &idx); err != nil {
			continue
		}
		for _, col := range idx.Colonne {
			if cercato != "" &&
				!strings.Contains(normalizza(col.Nome), cercato) &&
				!strings.Contains(normalizza(col.NomeFisico), cercato) &&
				!strings.Contains(normalizza(col.ID), cercato) {
				continue
			}
			fuori = append(fuori, campoTrovato{
				Campo: col.Nome, NomeFisico: col.NomeFisico, IDFiltro: col.ID, Tipo: col.Tipo,
				Dataset: idx.Titolo, ODataID: idx.ODataID, ID: idx.ID,
			})
		}
	}
	sort.Slice(fuori, func(i, j int) bool {
		if fuori[i].Campo != fuori[j].Campo {
			return fuori[i].Campo < fuori[j].Campo
		}
		return fuori[i].Dataset < fuori[j].Dataset
	})
	if limite > 0 && len(fuori) > limite {
		fuori = fuori[:limite]
	}
	return fuori, nil
}
