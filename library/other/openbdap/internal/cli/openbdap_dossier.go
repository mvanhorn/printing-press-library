// Copyright 2026 aborruso and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// dossierProgetto raccoglie il progetto e tutto cio' che vi ruota attorno.
type dossierProgetto struct {
	CUP      string                      `json:"cup"`
	Regione  string                      `json:"regione,omitempty"`
	Progetto []map[string]any            `json:"progetto"`
	Sezioni  map[string][]map[string]any `json:"sezioni"`
	Mancanti []string                    `json:"sezioni_non_disponibili,omitempty"`
	Nota     string                      `json:"nota,omitempty"`
	Dedotta  bool                        `json:"regione_dedotta,omitempty"`
}

func newNovelDossierCmd(flags *rootFlags) *cobra.Command {
	var regione, dbPath string
	var limite int

	cmd := &cobra.Command{
		Use:   "dossier [cup]",
		Short: "Il quadro completo di un'opera pubblica a partire dal CUP",
		Long: "Trova il progetto nel dataset nazionale, ne ricava la regione e raccoglie pagamenti, gare, partecipanti, " +
			"piano dei costi e soggetti titolari dai dataset regionali.\n" +
			"Usa questo comando per il quadro completo di un progetto. NON usarlo per la sola anagrafica; usa 'cup'.",
		Example: strings.Trim(`
  openbdap-pp-cli dossier I77H11000120009
  openbdap-pp-cli dossier I77H11000120009 --agent
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "live", "pp:happy-args": "cup=I77H11000120009"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "dossier")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("indica il CUP da approfondire"))
			}
			cup := strings.ToUpper(strings.TrimSpace(args[0]))
			if err := controllaCodice(cup, "cup"); err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}
			elenco, ok, err := datasetLocali(cmd, dbPath)
			if err != nil {
				return err
			}
			dossier := dossierProgetto{
				CUP:      cup,
				Progetto: make([]map[string]any, 0),
				Sezioni:  map[string][]map[string]any{},
			}
			if !ok {
				dossier.Nota = notaArchivioVuoto
				segnaOrigineLocale(flags)
				return printJSONFiltered(cmd.OutOrStdout(), dossier, flags)
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			ctx := cmd.Context()

			// Passo 1: il progetto. Il dataset nazionale copre tutte le regioni,
			// quindi basta una chiamata per sapere anche dove cercare il resto.
			progetti := datasetProgetti(elenco, regione)
			if len(progetti) == 0 {
				dossier.Nota = notaArchivioVuoto
				segnaOrigineLocale(flags)
				return printJSONFiltered(cmd.OutOrStdout(), dossier, flags)
			}
			for _, e := range cercaNeiMOP(ctx, c, progetti, "Codice CUP", cup, limite) {
				if e.Errore != "" {
					dossier.Mancanti = append(dossier.Mancanti, fmt.Sprintf("progetti/%s: %s", e.Regione, e.Errore))
					continue
				}
				dossier.Progetto = append(dossier.Progetto, e.Righe...)
				if len(e.Righe) > 0 && e.Regione != "" && !regioneAggregata(e.Regione) {
					dossier.Regione = e.Regione
				}
			}
			if len(dossier.Progetto) == 0 {
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), dossier, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Nessun progetto con CUP %s nei dataset interrogati.\n", cup)
				return nil
			}

			// Passo 2: la regione del titolare restringe il fan-out sulle altre
			// famiglie, che sono pubblicate solo per regione.
			regioneRicerca := regione
			if regioneRicerca == "" {
				regioneRicerca = dossier.Regione
			}
			if regioneRicerca == "" {
				// Il dataset nazionale non riporta la regione: si prova a
				// dedurla dal nome del titolare. E' un indizio, non un dato,
				// quindi la risposta lo dichiara.
				if dedotta := regioneDalTitolare(dossier.Progetto); dedotta != "" {
					regioneRicerca = dedotta
					dossier.Regione = dedotta
					dossier.Dedotta = true
				}
			}
			for _, fam := range famiglieMOPOrdinate {
				if fam == "progetti" {
					continue
				}
				bersagli := datasetMOP(elenco, fam, regioneRicerca)
				if len(bersagli) == 0 {
					dossier.Mancanti = append(dossier.Mancanti, fam+": nessun dataset nell'archivio locale")
					continue
				}
				righe := make([]map[string]any, 0)
				for _, e := range cercaNeiMOP(ctx, c, bersagli, "Codice CUP", cup, limite) {
					if e.Errore != "" {
						dossier.Mancanti = append(dossier.Mancanti, fmt.Sprintf("%s/%s: %s", fam, e.Regione, e.Errore))
						continue
					}
					righe = append(righe, e.Righe...)
				}
				dossier.Sezioni[fam] = righe
			}
			dossier.Progetto = compattaRighe(dossier.Progetto, flags.compact)
			for fam, righe := range dossier.Sezioni {
				dossier.Sezioni[fam] = compattaRighe(righe, flags.compact)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), dossier, flags)
			}
			// L'intestazione dice di che opera si tratta: il CUP e chi la
			// realizza. Senza, la scheda si apre con il primo campo che capita.
			fmt.Fprintf(cmd.OutOrStdout(), "CUP %s", cup)
			if titolare := testo(dossier.Progetto[0]["Descrizione Titolare"]); titolare != "" {
				fmt.Fprintf(cmd.OutOrStdout(), " - %s", titolare)
			}
			if stato := testo(dossier.Progetto[0]["Descrizione Stato CUP"]); stato != "" {
				fmt.Fprintf(cmd.OutOrStdout(), " - %s", stato)
			}
			if dossier.Regione != "" {
				fmt.Fprintf(cmd.OutOrStdout(), " (%s)", dossier.Regione)
			}
			fmt.Fprintln(cmd.OutOrStdout())
			if err := printAutoTable(cmd.OutOrStdout(), dossier.Progetto); err != nil {
				return err
			}
			for _, fam := range famiglieMOPOrdinate {
				righe := dossier.Sezioni[fam]
				if len(righe) == 0 {
					continue
				}
				fmt.Fprintf(cmd.OutOrStdout(), "\n%s (%d)\n", fam, len(righe))
				if err := printAutoTable(cmd.OutOrStdout(), righe); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&regione, "regione", "", "limita la ricerca a una regione")
	cmd.Flags().IntVar(&limite, "limite", 50, "numero massimo di righe per sezione")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath("openbdap-pp-cli"), "percorso dell'archivio locale")
	return cmd
}

// regioneDalTitolare prova a dedurre la regione dal nome del titolare quando
// il progetto arriva dal dataset nazionale, che la regione non la riporta.
// regioneAggregata riconosce le due "regioni" che in realta' sono aggregati
// nazionali: non indicano dove cercare i dataset regionali.
func regioneAggregata(r string) bool {
	return r == "Totale" || r == "Territorio Nazionale"
}

func regioneDalTitolare(progetto []map[string]any) string {
	for _, riga := range progetto {
		for _, campo := range []string{"Descrizione Titolare", "Descrizione Ente"} {
			valore := normalizza(testo(riga[campo]))
			if valore == "" {
				continue
			}
			for _, r := range regioniNote {
				if regioneAggregata(r) {
					continue
				}
				if strings.Contains(valore, normalizza(r)) {
					return r
				}
			}
		}
	}
	return ""
}
