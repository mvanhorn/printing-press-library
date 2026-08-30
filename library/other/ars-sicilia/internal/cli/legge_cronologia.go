// pp:data-source live
// pp:client-call
// Novel feature — cronologia inversa di una legge regionale: dalla legge
// promulgata risale al DDL originario e ai passaggi parlamentari.

package cli

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	icaro "github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/icaroclient"
	"github.com/spf13/cobra"
)

func newNovelLeggeCronologiaCmd(flags *rootFlags) *cobra.Command {
	var flagAnno int
	cmd := &cobra.Command{
		Use:   "cronologia <legisl> <numero>",
		Short: "Inversa di `ddl iter`: dalla legge promulgata risale al DDL originario e ai passaggi parlamentari.",
		Long: `Usare questo comando solo per una legge GIA' promulgata (archivio 201).
Per un DDL ancora in iter usare ` + "`ars-sicilia ddl iter`" + `.

Il DDL d'origine viene risolto seguendo il collegamento "DDL ed Iter" del
portale (campi P010/P012), non indovinato dal titolo: numeri di legge
ripetuti negli anni non si confondono. Usare --anno quando lo stesso numero
esiste in piu' anni della stessa legislatura.`,
		Example: "  ars-sicilia-pp-cli legge cronologia 18 1 --anno 2024 --json",
		Args:    cobra.MaximumNArgs(2),
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "legisl=18;numero=26;--anno=2025",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				if dryRunOK(flags) || cliIsVerify() {
					return cmd.Help()
				}
				return usageErr(fmt.Errorf("richiesti 2 argomenti: <legisl> e <numero>"))
			}
			legisl, errL := atoiArg(args[0], "legisl")
			numero, errN := atoiArg(args[1], "numero")
			if errL != nil || errN != nil {
				// Sotto --dry-run (e sotto verify) gli argomenti possono essere
				// segnaposto: la matrice di collaudo sonda i comandi scritti a
				// mano con valori finti (`mock-value`), e prima di questa
				// anteprima il ramo dry-run usciva 0 senza guardarli. Fallire
				// qui trasformerebbe una sonda che passava in un errore, quindi
				// si ripiega sull'help come fa il ramo degli argomenti mancanti
				// qui sopra: un'uscita non muta, e con codice 0.
				if dryRunOK(flags) || cliIsVerify() {
					return cmd.Help()
				}
				if errL != nil {
					return errL
				}
				return errN
			}
			if dryRunOK(flags) {
				return emitLeggeCronologiaDryRun(cmd, legisl, numero, flagAnno)
			}
			return runLeggeCronologia(cmd, flags, legisl, numero, flagAnno)
		},
	}
	// Lo stesso numero di legge si ripete ogni anno (es. L.R. 3/2023 e
	// L.R. 3/2024): --anno disambigua sul campo LEGANN, come in `leggi get`.
	cmd.Flags().IntVar(&flagAnno, "anno", 0, "Anno della legge, per disambiguare numeri ripetuti tra anni diversi.")
	return cmd
}

// emitLeggeCronologiaDryRun mostra la prima richiesta — quella che aggancia la
// legge nell'archivio 201 — invece di uscire in silenzio con exit 0, che è
// quello che faceva: il flag era accettato e scartato, e chi diagnosticava con
// --dry-run (come `ddl iter`, che l'anteprima ce l'ha) leggeva l'uscita vuota
// come un guasto del comando.
//
// Le richieste successive non si possono anteprimare: il ddl d'origine si
// risolve dai campi P010/P012 della legge trovata, quindi la query dipende da
// una risposta che il dry-run non chiede. Si dichiara il passo invece di
// tacerlo o di inventarne la forma.
// leggeSearchParams sono i parametri con cui si aggancia la legge, in un posto
// solo: anteprima e ricerca vera devono partire dagli stessi.
func leggeSearchParams(legisl, numero, anno int) map[string]string {
	params := map[string]string{"legisl": itoa(legisl), "numero": itoa(numero)}
	if anno != 0 {
		params["anno"] = itoa(anno)
	}
	return params
}

func emitLeggeCronologiaDryRun(cmd *cobra.Command, legisl, numero, anno int) error {
	target, err := dryRunTargetBySlug("leggi", leggeSearchParams(legisl, numero, anno))
	if err != nil {
		return err
	}
	if target == nil {
		return fmt.Errorf("archivio leggi non disponibile")
	}
	nota := "aggancia la legge, poi risale al DDL d'origine seguendo i campi P010/P012 della scheda trovata e ne legge l'iter: quelle richieste dipendono da questa risposta e non sono anteprimabili."
	if anno == 0 {
		nota += " Senza --anno l'archivio restituisce una sola delle leggi con questo numero, e la cronologia può riferirsi all'atto sbagliato."
	}
	return emitDryRunRequests(cmd, []map[string]any{target}, nota)
}

func runLeggeCronologia(cmd *cobra.Command, flags *rootFlags, legisl, numero, anno int) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	report := iterReport{Legisl: legisl, Numero: numero}

	c, err := icaro.New(nil)
	if err != nil {
		return fmt.Errorf("creazione client icaro: %w", err)
	}

	// 1. La legge (archivio 201). L'archivio tiene una riga per ARTICOLO, non
	// per legge: ne basta una — le altre ripetono la stessa promulgazione.
	arcLeggi := icaro.BySlug("leggi")
	if arcLeggi == nil {
		return fmt.Errorf("archivio leggi non disponibile")
	}
	recs, err := c.Search(ctx, *arcLeggi, icaro.SearchOptions{Params: leggeSearchParams(legisl, numero, anno), Limit: 1})
	if err != nil {
		return fmt.Errorf("ricerca legge: %w", err)
	}
	if len(recs) == 0 {
		return notFoundErr(fmt.Errorf("%s", leggeNonTrovataMsg(legisl, numero, anno)))
	}
	law := recs[0]
	warnAnnoNonPinnato(anno, numero, law.Fields["Data"])
	report.Titolo = law.Title
	// iterReport è condiviso con `ddl iter`: qui l'atto di cui si racconta la
	// storia è la legge, quindi la radice porta la sua scheda.
	report.URL = law.URL
	report.Eventi = append(report.Eventi, iterEvent{
		Fase:      "promulgazione",
		Data:      law.Fields["Data"],
		Sede:      "Legge regionale",
		Titolo:    law.Title,
		URL:       law.URL,
		ArchiveID: arcLeggi.ID,
		DocID:     law.DocID,
	})

	// The P010/P012 link below is keyed by the law's YEAR, so derive it from
	// the promulgation date when the caller didn't pin --anno.
	annoLegge := anno
	if annoLegge == 0 {
		if y := yearOf(law.Fields["Data"]); y != "" {
			annoLegge, _ = strconv.Atoi(y)
		}
	}

	// 2. Risali al DDL originario seguendo il collegamento del PORTALE, non
	// indovinandolo dal titolo. Ogni scheda-legge espone un link "DDL ed
	// Iter" che interroga l'archivio DDL sui campi P010/P012, dove il ddl
	// registra la legge in cui è confluito ("alr <anno> nlr <numero>"). È il
	// legame reale: la vecchia ricerca free-text sulle prime 4 parole del
	// titolo, non vincolata alla data, agganciava leggi omonime di altri anni
	// (la stabilità 2024 finiva collegata ai ddl di stabilità del 2026).
	arcDdl := icaro.BySlug("ddl")
	if arcDdl != nil && annoLegge > 0 {
		c2, cerr := icaro.New(nil)
		if cerr == nil {
			expr := fmt.Sprintf("alr adj %d.P010,P012 sfrase nlr adj %d.P010,P012", annoLegge, numero)
			ddls, derr := c2.Search(ctx, *arcDdl, icaro.SearchOptions{ISISRaw: expr, Limit: 5, MaxPages: 1})
			if derr == nil {
				for _, r := range ddls {
					if n, aerr := strconv.Atoi(strings.TrimSpace(r.Fields["Numero"])); aerr == nil {
						report.DdlOriginari = append(report.DdlOriginari, n)
					}
					report.Eventi = append(report.Eventi, iterEvent{
						Fase:      "ddl_originario",
						Data:      r.Fields["Data"],
						Sede:      "Disegno di legge n. " + r.Fields["Numero"],
						Titolo:    r.Title,
						URL:       r.URL,
						ArchiveID: arcDdl.ID,
						DocID:     r.DocID,
					})
					// 3. I passaggi parlamentari veri, letti dall'iter del ddl
					// d'origine — preferendo il blocco HTML etichettato "Iter"
					// (doc.Fields, via docIterEvents) al corpo del documento.
					// Prima venivano approssimati con una ricerca free-text
					// "legge <numero>" sui sommari, che intercettava qualunque
					// seduta citasse quelle due parole — sedute di altri anni
					// su tutt'altri disegni di legge. Anche dopo quel fix,
					// text-minare doc.Body restava fragile: il corpo concatena
					// anche i blocchi di coda (Firmatari, Gruppo Parlamentare,
					// Iniziativa) dopo l'articolato, e frammenti come "Seduta
					// n. 104" combaciano per caso con il pattern data
					// "<numero> <parola> <4 cifre>" quando il taglio di fine
					// iter non li esclude.
					if doc, gerr := c2.GetDoc(ctx, *arcDdl, r.DocID); gerr == nil {
						// Le stesse guardie di coerenza seduta↔data di `ddl
						// iter`: gli eventi sono letti dallo stesso iter e
						// portano le stesse contraddizioni della fonte. Senza,
						// la cronologia della L.R. 9/2020 dava il voto finale
						// al 2 maggio 2020 con la seduta 187 (che è del 28
						// aprile) senza alcun segnale, e chi incrociava per
						// data concludeva «resoconto mancante».
						evs := docIterEvents(doc)
						_, avviso := marcaEventiIncoerenti(cmd, legisl, evs)
						report.Note = uniscoNote(report.Note, avviso)
						for _, ev := range evs {
							ev.URL = r.URL
							ev.ArchiveID = arcDdl.ID
							ev.DocID = r.DocID
							report.Eventi = append(report.Eventi, ev)
						}
					}
				}
			}
		}
	}
	if len(report.Eventi) == 1 {
		report.Note = uniscoNote(report.Note, fmt.Sprintf("Nessun DDL d'origine collegato alla L.R. %d/%d nell'archivio: il portale non espone il legame (campi P010/P012) per questo atto.", numero, annoLegge))
	}

	sort.SliceStable(report.Eventi, func(i, j int) bool {
		return iterDateKey(report.Eventi[i].Data) < iterDateKey(report.Eventi[j].Data)
	})

	out := cmd.OutOrStdout()
	if flags.asJSON || !isTerminal(out) {
		return printJSONFiltered(out, report, flags)
	}
	fmt.Fprintf(out, "Legge %d/%d — %s\n", report.Legisl, report.Numero, report.Titolo)
	for _, e := range report.Eventi {
		fmt.Fprintf(out, "  [%s] %s — %s\n", e.Fase, e.Data, strings.TrimSpace(e.Sede+" "+e.Titolo))
	}
	return nil
}

// leggeNonTrovataMsg spiega perché la ricerca è tornata vuota, e la spiegazione
// cambia a seconda che --anno sia stato dato o no.
//
// Senza --anno il consiglio giusto è darlo: lo stesso numero si ripete in anni
// diversi della stessa legislatura. Ma con --anno già fornito quel consiglio
// descrive un caso che non è quello in corso, e l'uscita si legge come «legge
// inesistente» con un rimedio inapplicabile. Misurato sulla L.R. 21/2026,
// promulgata il 4 agosto: l'archivio leggi arrivava al 22 luglio, quindi la
// legge esisteva ed era solo troppo fresca per essere indicizzata.
//
// Il fronte della fonte NON si calcola qui. L'archivio leggi consegna dal più
// vecchio (vedi novita.go), quindi una sonda da una riga stamperebbe la prima
// legge dell'anno come «l'archivio arriva al…»: un numero sbagliato detto con
// sicurezza, peggio di nessun numero. Leggerlo davvero vorrebbe dire scaricare
// l'anno intero su un percorso d'errore. Si nomina invece il comando che quel
// numero ce l'ha già.
func leggeNonTrovataMsg(legisl, numero, anno int) string {
	if anno == 0 {
		return fmt.Sprintf("nessuna legge trovata per legisl=%d numero=%d (aggiungi --anno per disambiguare numeri ripetuti tra anni diversi)", legisl, numero)
	}
	return fmt.Sprintf(
		"nessuna legge trovata per legisl=%d numero=%d anno=%d. Due cause possibili: la legge è stata promulgata da poco e l'archivio delle leggi non l'ha ancora indicizzata (`ars-sicilia-pp-cli novita --archivi leggi` dice fin dove arriva la fonte), oppure quella coppia numero-anno non esiste in questa legislatura (`ars-sicilia-pp-cli leggi cerca --legisl %d --anno %d` le elenca). Se la legge è recente, l'iter parlamentare si legge comunque dal lato ddl con `ars-sicilia-pp-cli ddl cerca` e `ddl iter`.",
		legisl, numero, anno, legisl, anno)
}

// warnAnnoNonPinnato avverte su stderr quando il chiamante non ha fissato
// --anno: l'archivio tiene lo stesso numero di legge in anni diversi della
// stessa legislatura (nella XVIII ci sono due L.R. 26, ottobre 2024 e giugno
// 2025) e la ricerca ne restituisce una sola, la prima. Senza avviso si ottiene
// una cronologia perfettamente coerente e riferita all'atto sbagliato, senza
// alcun segnale che ne esistesse un altro. Non sondiamo l'archivio per contare
// gli anni davvero presenti: costerebbe pagine di richieste e resterebbe
// inaffidabile, perché l'archivio tiene una riga per articolo e una legge lunga
// riempirebbe da sola la finestra. Dire quale legge si è presa è più economico
// e altrettanto utile: chi legge la data riconosce subito l'atto sbagliato.
func warnAnnoNonPinnato(anno, numero int, data string) {
	if msg := annoNonPinnatoHint(anno, numero, data); msg != "" {
		fmt.Fprintln(os.Stderr, msg)
	}
}

// annoNonPinnatoHint torna il testo dell'avviso, o "" quando non c'è nulla da
// dire. Separato da warnAnnoNonPinnato per poterlo verificare senza catturare
// stderr, come truncatedHint.
func annoNonPinnatoHint(anno, numero int, data string) string {
	if anno != 0 || strings.TrimSpace(data) == "" {
		return ""
	}
	return fmt.Sprintf(
		"hint: --anno non indicato: uso la L.R. %d promulgata il %s. Lo stesso numero si ripete in anni diversi della stessa legislatura e l'archivio ne restituisce una sola: se non è questa, ripeti con --anno.",
		numero, data)
}
