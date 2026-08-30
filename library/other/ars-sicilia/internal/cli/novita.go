// pp:client-call
// Novel feature — cosa è comparso negli archivi da una certa data in qua,
// tutti gli archivi in una chiamata, con accanto il ritardo della fonte.
//
// `ddl drift` risponde a «cosa si è **mosso**», e per farlo confronta lo stato
// dell'iter fra due sync: quello stato esiste solo sui ddl, quindi drift resta
// per forza un comando su un archivio solo. Ma «cosa è **nuovo**» non ha
// bisogno di nessun confronto — la data di presentazione sta dentro il dato — e
// dopo l'introduzione di data_iso è una chiave uniforme su tutti gli archivi.
// Questo comando fa quella domanda, che è la prima di chi monitora e che finora
// costava una ricerca per archivio più un filtro a mano.
//
// La seconda metà del comando è ciò che lo rende leggibile: un archivio vuoto
// non dice se non è successo niente o se la fonte non ha ancora pubblicato.
// Misurato il 14/08/2026: 9 giorni di ritardo sui resoconti, 45 sulle mozioni.
// Chiedere «gli ultimi 7 giorni» alle mozioni darà zero per un mese e mezzo, e
// senza il ritardo accanto quello zero si legge come «l'Assemblea non ha fatto
// niente». Perciò ogni archivio porta il suo `ritardo_fonte_giorni`, e quando
// la finestra chiesta cade tutta dentro il ritardo il comando lo dice.

package cli

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	icaro "github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/icaroclient"
	"github.com/spf13/cobra"
)

// novitaLimiteAnno è il tetto di righe lette per archivio. L'anno intero si
// scarica perché è l'unica finestra che tutti gli archivi datati accettano, e
// perché la stessa lettura serve a due cose: le righe nuove e il massimo, cioè
// il ritardo della fonte.
const novitaLimiteAnno = 1000

type novitaArchivio struct {
	Archivio string `json:"archivio"`
	// Nuovi sono i documenti con data dentro la finestra chiesta.
	Nuovi []map[string]string `json:"nuovi"`
	// Conteggio è quanti ne sono stati trovati, anche quando --limit ne mostra
	// meno: il numero non deve dipendere da quante righe si è scelto di leggere.
	Conteggio int `json:"conteggio"`
	// UltimoRecord e RitardoGiorni dicono fin dove arriva la fonte su questo
	// archivio: senza, uno zero non è interpretabile.
	UltimoRecord  string `json:"ultimo_record_fonte,omitempty"`
	RitardoGiorni *int   `json:"ritardo_fonte_giorni,omitempty"`
	Nota          string `json:"nota,omitempty"`
	Errore        string `json:"errore,omitempty"`
}

type novitaReport struct {
	Da        string           `json:"da"`
	A         string           `json:"a"`
	Archivi   []novitaArchivio `json:"archivi"`
	Totale    int              `json:"totale"`
	Nota      string           `json:"nota,omitempty"`
	GeneratoA string           `json:"generato_a"`
}

func newNovelNovitaCmd(flags *rootFlags) *cobra.Command {
	var (
		flagSince  string
		flagDal    string
		flagArchvi []string
		flagLimit  int
	)
	cmd := &cobra.Command{
		Use:     "novita",
		Aliases: []string{"novità"},
		Args:    rejectPositionalArgs,
		Short:   "Cosa è comparso negli archivi da una certa data in qua, con accanto il ritardo della fonte.",
		Long: `novita elenca, archivio per archivio, i documenti comparsi dalla data chiesta in poi.

È la domanda di chi monitora: «cosa è successo dal mio ultimo controllo».
Diversa da ddl drift, che dice cosa si è *mosso* — cambio di stato dell'iter,
misurabile solo sui ddl e solo confrontando due deep sync. Qui la domanda è
cosa è *nuovo*, che si legge dalla data dell'atto e vale su ogni archivio.

Accanto a ogni archivio c'è il ritardo della fonte, perché senza quello uno
zero non si può leggere: se le mozioni sono pubblicate con 45 giorni di
ritardo, «gli ultimi 7 giorni» sarà vuoto a lungo, e non perché l'Assemblea
sia ferma. Quando la finestra chiesta cade tutta dentro il ritardo, il
comando lo dice invece di lasciare un elenco vuoto senza spiegazione.

Sull'archivio leggi la riga è per legge, non per articolo: il portale indicizza
un articolo per riga, quindi senza aggregazione una legge di sette articoli
conterebbe sette novità. Ogni riga porta articoli_trovati con quanti articoli
sono entrati nella finestra.

Gli archivi pareri e biblioteca non sono databili — il portale scrive le date
mangiando l'anno, o non ha proprio una colonna data — e vengono dichiarati
tali invece di essere riportati vuoti.`,
		Example: "  ars-sicilia-pp-cli novita --since 7d --agent\n" +
			"  ars-sicilia-pp-cli novita --since 30d --archivi ddl,interrogazioni,resoconti --agent\n" +
			"  ars-sicilia-pp-cli novita --dal 2026-07-01 --archivi resoconti --csv",
		Annotations: map[string]string{
			"mcp:read-only": "true",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) || cliIsVerify() {
				return nil
			}
			return runNovita(cmd, flags, flagSince, flagDal, flagArchvi, flagLimit)
		},
	}
	cmd.Flags().StringVar(&flagSince, "since", "7d", "Finestra all'indietro da oggi: 7d, 3w, 2m, 1y (m = mesi, non minuti), oppure 24h. Alternativa a --dal.")
	cmd.Flags().StringVar(&flagDal, "dal", "", "Data di inizio in forma YYYY-MM-DD. Vince su --since.")
	cmd.Flags().StringSliceVar(&flagArchvi, "archivi", nil, "Archivi da guardare (default: tutti quelli datati). Es: --archivi ddl,resoconti")
	cmd.Flags().IntVar(&flagLimit, "limit", 30, "Massimo di documenti mostrati per archivio; il conteggio resta quello vero. Sull'archivio leggi conta leggi, non righe-articolo.")
	return cmd
}

func runNovita(cmd *cobra.Command, flags *rootFlags, since, dal string, archivi []string, limit int) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	oggi := time.Now()
	da, err := inizioFinestra(since, dal, oggi)
	if err != nil {
		return usageErr(err)
	}
	if limit <= 0 {
		limit = 30
	}
	arcs, err := coverageArchivi(archivi)
	if err != nil {
		return usageErr(err)
	}
	report := novitaReport{
		Da:        da.Format("2006-01-02"),
		A:         oggi.Format("2006-01-02"),
		GeneratoA: oggi.UTC().Format(time.RFC3339),
	}
	for _, arc := range arcs {
		e := novitaDiArchivio(ctx, arc, da, oggi, limit)
		report.Totale += e.Conteggio
		report.Archivi = append(report.Archivi, e)
	}
	// In testa gli archivi che hanno qualcosa da dire; a parità, i più mossi.
	sort.SliceStable(report.Archivi, func(i, j int) bool {
		return report.Archivi[i].Conteggio > report.Archivi[j].Conteggio
	})
	if report.Totale == 0 {
		report.Nota = "nessun documento nella finestra chiesta. Guarda `ritardo_fonte_giorni` archivio per archivio prima di concludere che non sia successo niente: su questa fonte il ritardo di pubblicazione va dai 9 ai 45 giorni."
	}
	return emitNovita(cmd, flags, report)
}

// inizioFinestra risolve --dal o --since nella data di partenza.
func inizioFinestra(since, dal string, oggi time.Time) (time.Time, error) {
	if strings.TrimSpace(dal) != "" {
		t, err := time.Parse("2006-01-02", strings.TrimSpace(dal))
		if err != nil {
			return time.Time{}, fmt.Errorf("--dal vuole una data YYYY-MM-DD, non %q", dal)
		}
		if t.After(oggi) {
			return time.Time{}, fmt.Errorf("--dal è nel futuro (%s): la finestra sarebbe vuota per costruzione", dal)
		}
		return t, nil
	}
	d, err := durataIndietro(since)
	if err != nil {
		return time.Time{}, err
	}
	return oggi.Add(-d), nil
}

// giorniPerUnita traduce le unità con cui si scrive una finestra di
// monitoraggio. `m` vale **mesi**, non minuti: è il contrario di quel che
// intende time.ParseDuration, e la differenza non è teorica — `--since 2m`
// passato a ParseDuration dà due minuti, quindi zero risultati su ogni
// archivio, cioè esattamente lo zero silenzioso che questo comando esiste per
// evitare. Nessuno chiede all'Assemblea cosa è successo negli ultimi due
// minuti; molti chiedono cosa è successo negli ultimi due mesi.
var giorniPerUnita = map[string]int{"d": 1, "w": 7, "m": 30, "y": 365}

// durataIndietro legge 7d, 3w, 2m, 1y e, per chi la scrive così, 24h.
func durataIndietro(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" {
		return 7 * 24 * time.Hour, nil
	}
	unita := s[len(s)-1:]
	if f, ok := giorniPerUnita[unita]; ok {
		n, err := strconv.Atoi(s[:len(s)-1])
		if err != nil || n <= 0 {
			return 0, fmt.Errorf("--since vuole un numero positivo prima dell'unità, non %q", s)
		}
		return time.Duration(n*f) * 24 * time.Hour, nil
	}
	if d, err := time.ParseDuration(s); err == nil && d > 0 {
		return d, nil
	}
	return 0, fmt.Errorf("--since vuole una finestra come 7d, 3w, 2m, 1y o 24h, non %q", s)
}

// novitaDiArchivio legge un archivio e ne ricava insieme le righe nuove e il
// ritardo della fonte: sono due letture della stessa scaricata, non due giri.
func novitaDiArchivio(ctx context.Context, arc icaro.Archive, da, oggi time.Time, limit int) novitaArchivio {
	e := novitaArchivio{Archivio: arc.Slug, Nuovi: []map[string]string{}}
	if finestraAnno(arc, oggi.Year()) == nil || !haColonnaData(arc) {
		e.Nota = "archivio non databile: il portale non espone qui una data interrogabile, quindi «cosa è nuovo» non si può chiedere. Non è un archivio vuoto."
		return e
	}

	soglia := da.Format("2006-01-02")
	righe := []icaro.Record{}
	tettoRaggiunto := false
	// Si legge l'anno della soglia e quello corrente: una finestra a cavallo di
	// capodanno altrimenti perderebbe la metà vecchia.
	for anno := da.Year(); anno <= oggi.Year(); anno++ {
		params := finestraAnno(arc, anno)
		if params == nil {
			continue
		}
		r, err := righeFinoAllaSoglia(ctx, arc, params, soglia)
		if err != nil {
			e.Errore = erroreLeggibile(arc, err)
			return e
		}
		if len(r) >= novitaLimiteAnno {
			tettoRaggiunto = true
		}
		righe = append(righe, r...)
	}

	var date []string
	nuove := []icaro.Record{}
	for _, r := range righe {
		iso := dataISO(r.Fields["Data"])
		if iso == "" {
			continue
		}
		date = append(date, iso)
		if iso >= soglia {
			nuove = append(nuove, r)
			e.Nuovi = append(e.Nuovi, rigaNovita(arc, r, iso))
		}
	}
	// L'archivio leggi è indicizzato per ARTICOLO: senza aggregazione la stessa
	// legge occupa una riga per articolo e `conteggio` conta articoli. Misurato
	// il 21/08/2026: la finestra a 30 giorni dava 7, cioè i sette articoli
	// della sola L.R. 14/2026, e chi legge «7» capisce sette leggi. Vedi
	// leggi_collapse.go, che fa la stessa cosa per `leggi cerca`. Il ramo resta
	// legato allo slug perché i campi che aggrega (Atto, Docum.) sono
	// dell'archivio 201.
	if arc.Slug == "leggi" {
		e.Nuovi = righeNovitaLeggi(arc, nuove)
		if tettoRaggiunto {
			e.Nota = uniscoNote(e.Nota, fmt.Sprintf("lette %d righe-articolo, il tetto per archivio: il conteggio delle leggi può essere in difetto. Restringi la finestra per averlo completo.", novitaLimiteAnno))
		}
	}
	e.Conteggio = len(e.Nuovi)
	// Il più recente in cima: sugli archivi in ordine crescente (leggi) la
	// scaricata arriva dal più vecchio.
	sort.SliceStable(e.Nuovi, func(i, j int) bool {
		return e.Nuovi[i]["data_iso"] > e.Nuovi[j]["data_iso"]
	})
	if len(e.Nuovi) > limit {
		e.Nuovi = e.Nuovi[:limit]
	}
	if len(date) == 0 {
		e.Nota = "nessuna riga con una data leggibile: su questo archivio la finestra non è verificabile."
		return e
	}

	max := massimo(date)
	e.UltimoRecord = max
	if t, err := time.Parse("2006-01-02", max); err == nil {
		g := int(oggi.Truncate(24*time.Hour).Sub(t.Truncate(24*time.Hour)).Hours() / 24)
		e.RitardoGiorni = &g
		// Il caso che questo comando esiste per rendere leggibile.
		if e.Conteggio == 0 && max < soglia {
			e.Nota = fmt.Sprintf("nessuna novità, ma la fonte su questo archivio è ferma al %s: la finestra chiesta parte dal %s, cioè tutta dentro il ritardo di pubblicazione. Questo zero è latenza, non assenza.", max, soglia)
		}
	}
	return e
}

// righeFinoAllaSoglia scarica quanto basta, non l'anno intero.
//
// Quasi tutti gli archivi consegnano dal più recente: se le date scendono
// davvero — verificato sulle righe lette, non dato per scontato, perché `leggi`
// esce dal più vecchio — allora le novità stanno tutte in cima e si può
// smettere appena si scende sotto la soglia. Misurato: leggere l'anno intero su
// quattro archivi costava 2 minuti e 11 secondi, e su una finestra di sette
// giorni quasi tutte quelle righe erano scaricate per essere buttate.
//
// **Ogni ricerca vuole una sessione sua.** Ripetere la stessa query sullo
// stesso client non riparte da capo: la seconda risposta comincia più in basso,
// e i documenti più recenti spariscono senza che niente lo segnali. Misurato
// sulle interrogazioni del 2026, tre chiamate identiche a 30 righe: con lo
// stesso client la prima parte dal 3 agosto e le successive dal 28 luglio; con
// un client nuovo ogni volta partono tutte dal 3 agosto. È lo stesso motivo per
// cui `deputato profilo` costruisce un client dentro il ciclo sugli archivi.
//
// Dove l'ordine non scende si legge l'anno: lì il record più recente può stare
// in fondo, e fermarsi prima darebbe un elenco corto scambiato per completo.
func righeFinoAllaSoglia(ctx context.Context, arc icaro.Archive, params map[string]string, soglia string) ([]icaro.Record, error) {
	prime, _, err := sondaNuova().cerca(ctx, arc, params, 20, 1)
	if err != nil {
		return nil, err
	}
	if len(prime) == 0 {
		return prime, nil
	}
	if !decrescente(dateInOrdine(prime)) {
		tutte, _, err := sondaNuova().cerca(ctx, arc, params, novitaLimiteAnno, 0)
		if err != nil {
			return nil, err
		}
		return tutte, nil
	}
	righe := prime
	// L'ordine scende: si allarga la finestra finché l'ultima riga letta è
	// ancora dentro la soglia, cioè finché può esserci altro da prendere.
	for limite := 60; ultimaDentro(righe, soglia) && limite <= novitaLimiteAnno; limite *= 3 {
		piu, troncato, err := sondaNuova().cerca(ctx, arc, params, limite, 0)
		if err != nil {
			// Quel che si è già letto vale: è la parte più recente, cioè
			// esattamente quella che interessa qui.
			return righe, nil
		}
		if len(piu) <= len(righe) && !troncato {
			return piu, nil // l'archivio è finito prima della soglia
		}
		righe = piu
	}
	return righe, nil
}

// ultimaDentro dice se l'ultima riga letta è ancora dentro la finestra, cioè
// se conviene chiederne altre.
func ultimaDentro(righe []icaro.Record, soglia string) bool {
	for i := len(righe) - 1; i >= 0; i-- {
		if iso := dataISO(righe[i].Fields["Data"]); iso != "" {
			return iso >= soglia
		}
	}
	return false
}

// righeNovitaLeggi rende una riga per legge invece di una per articolo.
//
// Il conteggio degli articoli agganciati resta nella riga (`articoli_trovati`),
// così chi vuole le righe grezze sa che ci sono. La legge si nomina in due
// modi, entrambi presenti: `atto` è l'etichetta con cui si cita ("L.R. 14"),
// `numero` è il numero nudo ("14") che si passa a --numero. Nell'archivio 201
// il campo Numero è vuoto, ed è il motivo per cui `--select numero` non
// trovava nulla e la resa a schermo lasciava la colonna in bianco.
func righeNovitaLeggi(arc icaro.Archive, recs []icaro.Record) []map[string]string {
	out := []map[string]string{}
	for _, l := range collapseLeggi(recs) {
		riga := map[string]string{
			"archivio":         arc.Slug,
			"data":             l.Data,
			"data_iso":         dataISO(l.Data),
			"titolo":           l.Titolo,
			"articoli_trovati": itoa(l.ArticoliTrovati),
		}
		// Due nomi per la stessa cosa, di proposito: `atto` è quello con cui
		// la legge si cita ed è il nome che usa `leggi cerca`, `numero` è
		// quello che si passa a --numero ed è la chiave che tutti gli altri
		// archivi di questo comando usano. Chi arriva da una delle due strade
		// trova il campo dove se lo aspetta.
		if l.Atto != "" {
			riga["atto"] = l.Atto
		}
		if l.Numero != "" {
			riga["numero"] = l.Numero
		}
		if l.URL != "" {
			riga["url"] = l.URL
		}
		out = append(out, riga)
	}
	return out
}

// rigaNovita tiene i campi che servono a riconoscere un atto e a raggiungerlo.
// Il testo integrale non entra: qui si guarda cosa è comparso, non lo si legge.
func rigaNovita(arc icaro.Archive, r icaro.Record, iso string) map[string]string {
	riga := map[string]string{
		"archivio": arc.Slug,
		"data":     r.Fields["Data"],
		"data_iso": iso,
		"titolo":   titoloAlias(r),
	}
	if n := r.Fields["Numero"]; n != "" {
		riga["numero"] = n
	}
	if r.URL != "" {
		riga["url"] = r.URL
	}
	return riga
}

func emitNovita(cmd *cobra.Command, flags *rootFlags, report novitaReport) error {
	out := cmd.OutOrStdout()
	if flags.asJSON || flags.csv || !isTerminal(out) {
		return printJSONFiltered(out, report, flags)
	}
	fmt.Fprintf(out, "Novità dal %s al %s — %d documenti\n\n", report.Da, report.A, report.Totale)
	for _, a := range report.Archivi {
		fmt.Fprintf(out, "%-16s %d", a.Archivio, a.Conteggio)
		if a.RitardoGiorni != nil {
			fmt.Fprintf(out, "  (fonte ferma a %d giorni fa)", *a.RitardoGiorni)
		}
		fmt.Fprintln(out)
		for _, n := range a.Nuovi {
			// Sulle leggi l'atto ("L.R. 14") si legge, il numero nudo ("14")
			// no: a schermo vince il primo, nel JSON restano entrambi.
			etichetta := n["numero"]
			if n["atto"] != "" {
				etichetta = n["atto"]
			}
			fmt.Fprintf(out, "    %s  %s  %s\n", n["data_iso"], etichetta, troncaTitolo(n["titolo"]))
		}
		if a.Nota != "" {
			fmt.Fprintf(out, "    → %s\n", a.Nota)
		}
		if a.Errore != "" {
			fmt.Fprintf(out, "    ! %s\n", a.Errore)
		}
	}
	if report.Nota != "" {
		fmt.Fprintf(out, "\n%s\n", report.Nota)
	}
	return nil
}

func troncaTitolo(s string) string {
	r := []rune(s)
	if len(r) <= 90 {
		return s
	}
	return string(r[:89]) + "…"
}
