package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var ddlMaterie = []string{
	"Abrogazione di norme",
	"Acquedotti, irrigazioni e dighe",
	"Agricoltura",
	"Agrumicoltura",
	"Ambiente",
	"Amministrazione regionale",
	"Anziani",
	"Aree interne",
	"Artigianato",
	"Assistenza sanitaria",
	"Assistenza sociale",
	"Associazionismo, volontariato",
	"Attivita all estero e connessa con la normativa UE",
	"Attivita produttive",
	"Banche",
	"Beni culturali",
	"Beni sequestrati e confiscati",
	"Bilancio e contabilita regionale",
	"Caccia",
	"Calamita naturali",
	"Case da gioco",
	"Celebrazioni e manifestazioni pubbliche",
	"Centri storici",
	"Commercio",
	"Commissioni speciali, indagine, inchiesta",
	"Comparti agricoli",
	"Comuni",
	"Consorzi di Comuni",
	"Consorzi di bonifica",
	"Controlli attivita pubblica amministrazione",
	"Cooperazione",
	"Credito",
	"Cultura",
	"Demanio",
	"Deputati regionali",
	"Difensore civico",
	"Edilizia",
	"Edilizia scolastica",
	"Elezioni",
	"Emigrazione",
	"Energia",
	"Enti locali",
	"Enti regionali",
	"Enti, istituti e fondazioni",
	"Esattorie",
	"Famiglia",
	"Farmacie",
	"Forestazione",
	"Formazione professionale",
	"Gemellaggi tra i comuni",
	"Handicap, soggetti portatori di",
	"IACP",
	"Idrocarburi",
	"Igiene e profilassi",
	"Immigrazione",
	"Impresa",
	"Industria",
	"Informatica",
	"Informazione radiotelevisiva",
	"Interventi per lo sviluppo dei comparti agricoli",
	"Isole minori",
	"Lavori pubblici",
	"Lavoro",
	"Leggi voto",
	"Lotta alla mafia",
	"Metanizzazione",
	"Miniere",
	"Nomine",
	"Obiezione di coscienza",
	"Occupazione giovanile",
	"Onorificenze",
	"Ordinamento costituzionale della Regione",
	"Ordinamento delle autonomie locali",
	"Osservatorio regionale del lavoro",
	"Parchi e riserve naturali",
	"Pareri Parlamentari",
	"Parita di diritti",
	"Partecipazioni regionali",
	"Personale ospedaliero",
	"Personale regionale",
	"Personale statale",
	"Pesca",
	"Piccola e media impresa",
	"Politiche di sicurezza",
	"Politiche giovanili",
	"Polizia municipale",
	"Porti",
	"Premi e borse di studio",
	"Previdenza sociale",
	"Procedure concorsuali",
	"Programmazione",
	"Protezione animali",
	"Protezione civile",
	"Pubblica amministrazione",
	"Pubblica istruzione",
	"Pubblico impiego",
	"Ricerca scientifica",
	"Salute",
	"Sanita",
	"Serricoltura",
	"Servizio civile",
	"Solidarieta civile",
	"Spettacolo",
	"Sport",
	"Stampa",
	"Teatri",
	"Telecomunicazioni",
	"Territorio e ambiente",
	"Tossicodipendenza",
	"Trasporti",
	"Tributi",
	"Turismo",
	"Tutela consumatori",
	"Tutela diritti",
	"Uffici stampa",
	"Unione Europea",
	"Unita sanitarie locali",
	"Universita",
	"Urbanistica",
	"Usi civici",
	"Viabilita",
	"Vitivinicoltura",
	"Zootecnia",
}

func newDdlMaterieCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "materie",
		Short: "Elenca le materie (settori) disponibili per --materia in 'ddl cerca'.",
		Long: `Elenca il vocabolario controllato del campo SETTOR (materia) dei DDL.
I valori sono estratti dal portale ARS e passati verbatim a --materia.
Nota: i valori non hanno accenti (es. Sanita, Parita).`,
		Example: strings.Trim(`
  ars-sicilia-pp-cli ddl materie
  ars-sicilia-pp-cli ddl materie --json
  ars-sicilia-pp-cli ddl materie --json | jq '.[] | select(test("ambiente|energia";"i"))'`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if flags.asJSON || !isTerminal(cmd.OutOrStdout()) {
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(ddlMaterie)
			}
			for _, m := range ddlMaterie {
				fmt.Fprintln(cmd.OutOrStdout(), m)
			}
			return nil
		},
	}
	return cmd
}
