// pp:data-source live
// pp:client-call
// Novel feature — dossier 360° di una commissione: convocazioni, sommari,
// pareri al governo e DDL assegnati raggruppati per codice commissione.

package cli

import (
	"context"
	"fmt"
	"strings"

	icaro "github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/icaroclient"
	"github.com/spf13/cobra"
)

func newNovelCommissioneDossierCmd(flags *rootFlags) *cobra.Command {
	var (
		flagLegisl int
		flagLimit  int
	)
	cmd := &cobra.Command{
		Use:   "dossier <codcom-o-nome>",
		Short: "Vista completa di una commissione: convocazioni, sommari, pareri al Governo e DDL assegnati.",
		Long: "Vista completa di una commissione: convocazioni, sommari, pareri al Governo e DDL assegnati.\n\n" +
			"Accetta il codice 1-6, l'ordinale (PRIMA..SESTA) o un frammento della denominazione\n" +
			"d'archivio. Le commissioni speciali non hanno un codice: si raggiungono per\n" +
			"denominazione, che non coincide con l'etichetta d'uso corrente (l'Antimafia è\n" +
			"\"Commissione d'inchiesta e vigilanza sul fenomeno della mafia e della corruzione\n" +
			"in Sicilia\"). Un termine che non corrisponde a nulla non produce un dossier vuoto:\n" +
			"l'errore elenca le denominazioni disponibili per la legislatura.",
		Example: "  ars-sicilia-pp-cli commissione dossier 5 --legisl 18 --json\n" +
			"  ars-sicilia-pp-cli commissione dossier \"inchiesta e vigilanza\" --legisl 18 --json",
		Annotations: map[string]string{
			"mcp:read-only": "true",
			"pp:happy-args": "codcom-o-nome=SESTA;--legisl=18",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			arg := strings.TrimSpace(strings.Join(args, " "))
			if dryRunOK(flags) {
				return emitCommissioneDossierDryRun(cmd, arg, flagLegisl)
			}
			return runCommissioneDossier(cmd, flags, arg, flagLegisl, flagLimit)
		},
	}
	cmd.Flags().IntVar(&flagLegisl, "legisl", 0, "Legislatura (es. 18).")
	cmd.Flags().IntVar(&flagLimit, "limit", 30, "Max risultati per sezione.")
	return cmd
}

// resolveDossierCommissione traduce l'argomento del dossier nelle due forme che
// gli archivi capiscono: i parametri per il backend /bd/ (convocazioni, sommari)
// e il nome per l'ISIS dei pareri.
//
//	"6"                          -> /bd/ codcom=6            + pareri "SESTA"
//	"SESTA"                      -> /bd/ codcom=6            + pareri "SESTA"
//	"Servizi Sociali e Sanitari" -> /bd/ commissione=<arg>   + pareri <arg>
//
// Le prime due righe sono il fix: prima il termine grezzo andava a entrambi, e
// "SESTA" non è un frammento di "VI - Salute, Servizi Sociali e Sanitari" (zero
// convocazioni e sommari) mentre "6" non è un valore che l'ISIS pareri conosca
// (zero pareri). Un nome libero funziona già su entrambi e passa invariato: è
// anche l'unica via per le commissioni speciali, che non hanno un codice 1-6.
func resolveDossierCommissione(arg string) (bdParams map[string]string, isisName string) {
	if code := strings.TrimSpace(arg); commissioneOrdinale(code) != "" {
		return map[string]string{"codcom": code}, commissioneOrdinale(code)
	}
	if code := commissioneCodice(arg); code != "" {
		return map[string]string{"codcom": code}, strings.ToUpper(strings.TrimSpace(arg))
	}
	return map[string]string{"commissione": arg}, arg
}

// commissioneCodice è l'inversa di commissioneOrdinale: "SESTA" -> "6".
func commissioneCodice(name string) string {
	switch strings.ToUpper(strings.TrimSpace(name)) {
	case "PRIMA":
		return "1"
	case "SECONDA":
		return "2"
	case "TERZA":
		return "3"
	case "QUARTA":
		return "4"
	case "QUINTA":
		return "5"
	case "SESTA":
		return "6"
	}
	return ""
}

func copyParams(in map[string]string) map[string]string {
	out := make(map[string]string, len(in)+1)
	for k, v := range in {
		out[k] = v
	}
	return out
}

// dossierNoMatchError distingue i due modi in cui un dossier resta vuoto: una
// commissione che esiste ma non ha attività (report vuoto legittimo, nessun
// errore) e un termine che non corrisponde a nessuna commissione. Nel secondo
// caso elenca le denominazioni della legislatura, perché sono l'informazione
// che manca per riprovare: le etichette d'uso corrente ("Antimafia") non
// compaiono nelle denominazioni d'archivio.
func dossierNoMatchError(ctx context.Context, arg string, legisl int) error {
	c, err := icaro.New(nil)
	if err != nil {
		return fmt.Errorf("nessun risultato per la commissione %q", arg)
	}
	legislStr := ""
	if legisl > 0 {
		legislStr = itoa(legisl)
	}
	nomi, err := c.CommissioniDisponibili(ctx, legislStr)
	if err != nil || len(nomi) == 0 {
		return fmt.Errorf("nessun risultato per la commissione %q", arg)
	}
	needle := strings.ToLower(strings.TrimSpace(arg))
	for _, n := range nomi {
		if strings.Contains(strings.ToLower(n), needle) {
			// La commissione esiste: il vuoto è un dato, non un errore d'input.
			return nil
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "nessuna commissione corrisponde a %q", arg)
	if legisl > 0 {
		fmt.Fprintf(&b, " nella legislatura %d", legisl)
	}
	b.WriteString(". Denominazioni disponibili:")
	for _, n := range nomi {
		b.WriteString("\n  - " + n)
	}
	b.WriteString("\nBasta un frammento della denominazione (es. \"inchiesta e vigilanza\"), oppure il codice 1-6 per le permanenti.")
	return fmt.Errorf("%s", b.String())
}

type dossierSection struct {
	Tipo      string           `json:"tipo"`
	Archivio  string           `json:"archivio"`
	Risultati []map[string]any `json:"risultati"`
}

type dossierReport struct {
	Commissione string         `json:"commissione"`
	Legisl      int            `json:"legisl,omitempty"`
	Conteggio   map[string]int `json:"conteggio"`
	// Troncato lists the section labels where Conteggio is a --limit cap,
	// not the true total: the portal had more matching records than were
	// fetched. Re-run with a higher --limit to see the rest.
	Troncato []string         `json:"troncato,omitempty"`
	Sezioni  []dossierSection `json:"sezioni"`
}

// emitCommissioneDossierDryRun elenca le quattro richieste del dossier invece
// di uscire in silenzio con exit 0. È l'anteprima che serve di più delle tre,
// perché mostra la traduzione fatta da resolveDossierCommissione: lo stesso
// argomento parte come `codcom` verso /bd/ e come ordinale a lettere verso
// l'ISIS dei pareri. Chi vede mezze sezioni vuote legge qui, senza indovinare,
// quale delle due forme non ha agganciato.
func emitCommissioneDossierDryRun(cmd *cobra.Command, arg string, legisl int) error {
	bdParams, isisName := resolveDossierCommissione(arg)
	sezione := func(slug string, params map[string]string) (map[string]any, error) {
		if legisl > 0 {
			params["legisl"] = itoa(legisl)
		}
		return dryRunTargetBySlug(slug, params)
	}
	requests := []map[string]any{}
	for _, sez := range dossierSezioni(bdParams, isisName) {
		r, err := sezione(sez.slug, sez.params)
		if err != nil {
			return err
		}
		if r != nil {
			requests = append(requests, r)
		}
	}
	return emitDryRunRequests(cmd, requests, fmt.Sprintf("l'argomento %q viaggia in due forme: %v verso il backend /bd/ (convocazioni, sommari) e %q verso l'ISIS di pareri e ddl. La sezione ddl è una ricerca testuale, non l'elenco dei ddl assegnati.", arg, bdParams, isisName))
}

// dossierSezione descrive una delle quattro ricerche del dossier.
type dossierSezione struct {
	slug   string
	label  string
	params map[string]string
}

// dossierSezioni elenca le quattro ricerche in un posto solo: l'anteprima e il
// dossier vero devono partire dalle stesse, o la prima smette di descrivere la
// seconda. È anche il punto in cui si vede la traduzione dell'argomento — i
// parametri /bd/ da una parte, l'ordinale a lettere dall'altra — che è la
// ragione per cui metà sezioni possono tornare vuote.
func dossierSezioni(bdParams map[string]string, isisName string) []dossierSezione {
	return []dossierSezione{
		{"convocazioni", "convocazioni", copyParams(bdParams)},
		{"sommari", "sommari", copyParams(bdParams)},
		{"pareri", "pareri", map[string]string{"commissione": isisName}},
		// La sezione ddl è una ricerca testuale, non l'elenco degli assegnati:
		// l'archivio 221 non espone l'assegnazione come campo filtrabile.
		{"ddl", "ddl_assegnati", map[string]string{"testo": isisName}},
	}
}

func runCommissioneDossier(cmd *cobra.Command, flags *rootFlags, arg string, legisl, perSection int) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	if perSection <= 0 {
		perSection = 30
	}
	report := dossierReport{
		Commissione: arg,
		Legisl:      legisl,
		Conteggio:   map[string]int{},
	}

	// Gli archivi non parlano la stessa lingua: /bd/ (convocazioni, sommari)
	// risolve la commissione per codice o per frammento della denominazione
	// ("VI - Salute, Servizi Sociali e Sanitari"), l'ISIS dei pareri vuole
	// l'ordinale a lettere ("SESTA"). Passare lo stesso termine a entrambi
	// lascia sempre metà sezioni vuote, quindi si traduce prima.
	bdParams, isisName := resolveDossierCommissione(arg)

	section := func(slug, label string, params map[string]string) {
		arc := icaro.BySlug(slug)
		if arc == nil {
			return
		}
		c, err := icaro.New(nil)
		if err != nil {
			return
		}
		if legisl > 0 {
			params["legisl"] = itoa(legisl)
		}
		var truncated bool
		recs, err := c.Search(ctx, *arc, icaro.SearchOptions{
			Params:    params,
			Limit:     perSection,
			MaxPages:  maxInt(1, (perSection+9)/10),
			Truncated: &truncated,
		})
		if err != nil {
			return
		}
		s := dossierSection{Tipo: label, Archivio: arc.ID}
		for _, r := range recs {
			row := map[string]any{
				"data":    r.Fields["Data"],
				"numero":  r.Fields["Numero"],
				"titolo":  r.Title,
				"excerpt": r.Excerpt,
				"url":     r.URL,
			}
			// convocazioni e sommari arrivano dal backend /bd/, che non
			// espone un DocID Icaro: la chiave resta fuori invece di
			// riportare uno zero buono per nulla (vedi emitRecords).
			if r.DocID > 0 {
				row["doc_id"] = r.DocID
			}
			s.Risultati = append(s.Risultati, row)
		}
		report.Sezioni = append(report.Sezioni, s)
		report.Conteggio[label] = len(s.Risultati)
		if truncated {
			report.Troncato = append(report.Troncato, label)
		}
	}

	sezioni := dossierSezioni(bdParams, isisName)
	for _, s := range sezioni[:3] {
		section(s.slug, s.label, s.params)
	}
	// La sezione ddl è una ricerca testuale sul termine, non l'elenco dei ddl
	// assegnati: l'archivio 221 non espone l'assegnazione come campo filtrabile.
	// Resta perché utile, ma non concorre a decidere se la commissione esiste.
	// Il termine cercato è isisName, non l'argomento grezzo: con "6" si
	// cercherebbe la cifra 6 in tutti i ddl, mentre "SESTA" è la parola che
	// compare davvero nell'iter ("Assegnato per esame Commissione SESTA").
	section(sezioni[3].slug, sezioni[3].label, sezioni[3].params)

	// Solo le sezioni che filtrano davvero per commissione dicono se il termine
	// ha agganciato qualcosa: la sezione ddl produce righe anche per un nome
	// inesistente ed è ciò che nascondeva l'assenza di riscontro.
	matched := 0
	for _, label := range []string{"convocazioni", "sommari", "pareri"} {
		matched += report.Conteggio[label]
	}
	if matched == 0 {
		if err := dossierNoMatchError(ctx, arg, legisl); err != nil {
			return err
		}
		// Commissione esistente ma senza attività: si prosegue e si stampa il
		// report vuoto, che è la risposta corretta alla domanda posta.
	}

	out := cmd.OutOrStdout()
	if flags.asJSON || !isTerminal(out) {
		return printJSONFiltered(out, report, flags)
	}
	fmt.Fprintf(out, "Commissione: %s\n", report.Commissione)
	if report.Legisl > 0 {
		fmt.Fprintf(out, "Legislatura: %d\n\n", report.Legisl)
	}
	troncato := map[string]bool{}
	for _, label := range report.Troncato {
		troncato[label] = true
	}
	for _, s := range report.Sezioni {
		suffix := ""
		if troncato[s.Tipo] {
			suffix = " (troncato, aumenta --limit)"
		}
		fmt.Fprintf(out, "[%s] %d risultati%s\n", s.Tipo, len(s.Risultati), suffix)
		for _, r := range s.Risultati {
			if id, ok := r["doc_id"]; ok {
				fmt.Fprintf(out, "  #%v  %v  %v\n", id, r["data"], r["titolo"])
				continue
			}
			fmt.Fprintf(out, "  %v  %v\n", r["data"], r["titolo"])
		}
		fmt.Fprintln(out)
	}
	return nil
}
