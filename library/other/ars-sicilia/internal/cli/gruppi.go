// pp:data-source live
// pp:client-call
// Novel feature — l'anagrafica dei gruppi parlamentari, che sta sul sito
// istituzionale (www.ars.sicilia.it) e non sul motore documentale.
//
// Fino a qui da un nome di gruppo trovato negli atti non si risaliva a
// niente: la CLI vive su dati.ars.sicilia.it e lì il gruppo compare solo
// come stringa accanto a una firma. Questi due comandi leggono le pagine
// Drupal del sito istituzionale (una richiesta per l'elenco, una per la
// composizione) e chiudono il buco dichiarato in which_assenti:
//
//	gruppi elenco [--legisl 16|17|18] [--deputato "<nome>"]
//	gruppi get <slug-o-nome>
//
// L'invariante che tiene insieme elenco e dettaglio: la somma dei
// componenti dei gruppi della XVIII fa 70, il numero dei deputati ARS
// (certificato dal test su fixture in internal/wwwclient).

package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/wwwclient"
	"github.com/spf13/cobra"
)

// legislatureGruppiValide è ciò che il selettore del sito offre
// (XVIII, XVII, XVI). Il numero va mappato, non calcolato: sulla pagina
// deputato XVIII è idLeg=71, sull'elenco gruppi 18 — qui si normalizza al
// numero arabo canonico.
var legislatureGruppiValide = []int{16, 17, 18}

func newNovelGruppiCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "gruppi",
		Short: "Gruppi parlamentari (www.ars.sicilia.it): elenco per legislatura e composizione con ruoli e collegio.",
		Long: `L'anagrafica dei gruppi parlamentari, dal sito istituzionale www.ars.sicilia.it.

` + "`gruppi elenco`" + ` elenca i gruppi di una legislatura (16, 17, 18; default 18) con lo slug
della pagina di ciascuno. ` + "`gruppi get`" + ` apre un gruppo — per slug (dall'elenco) o per
nome — e ne restituisce la composizione: presidente e cariche di gruppo, collegio di
elezione, email e scheda di ogni componente.

Con --deputato, elenco risponde alla domanda inversa: in quale gruppo sta un
parlamentare, e con quale ruolo. Costa una richiesta per gruppo (10 nella XVIII).

I nomi dei gruppi sono gli stessi che compaiono accanto alle firme sugli atti
(campo gruppo di ddl firmatari), quindi l'elenco è anche il vocabolario per
costruire quella join a valle.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newGruppiElencoCmd(flags))
	cmd.AddCommand(newGruppiGetCmd(flags))
	return cmd
}

// emitGruppiDryRun mostra la pagina che verrebbe letta invece di leggerla,
// come fa `ddl iter --dry-run` con la query ISIS: su una CLI che scrape HTML
// la fonte esatta è metà della diagnosi. Prima --dry-run era ignorato e i due
// comandi andavano in rete lo stesso.
func emitGruppiDryRun(cmd *cobra.Command, path string, params map[string]string) error {
	u := wwwclient.DefaultBaseURL + path
	sep := "?"
	for k, v := range params {
		u += sep + k + "=" + v
		sep = "&"
	}
	return writeJSON(cmd.OutOrStdout(), map[string]any{
		"source":      "www.ars.sicilia.it",
		"would_fetch": u,
		"note":        "reads the institutional site's HTML: no API, selectors certified by fixtures in internal/wwwclient/testdata",
		"dry_run":     true,
	})
}

func validateLegislaturaGruppi(legisl int) error {
	for _, l := range legislatureGruppiValide {
		if legisl == l {
			return nil
		}
	}
	return usageErr(fmt.Errorf("--legisl %d non valida per i gruppi: il selettore del sito offre %v", legisl, legislatureGruppiValide))
}

func newGruppiElencoCmd(flags *rootFlags) *cobra.Command {
	var (
		flagLegisl   int
		flagDeputato string
	)
	cmd := &cobra.Command{
		Use:   "elenco",
		Args:  rejectPositionalArgs,
		Short: "Elenca i gruppi parlamentari di una legislatura; con --deputato, dove sta un parlamentare.",
		Long: `Elenca i gruppi parlamentari di una legislatura (16, 17, 18; default 18), letti
da https://www.ars.sicilia.it/gruppi-parlamentari?idLeg=<N>. Una richiesta.

Con --deputato "<nome>" legge anche le pagine dei singoli gruppi (una richiesta
per gruppo) e restituisce i gruppi in cui il parlamentare compare, con ruolo e
collegio di elezione: è la domanda inversa, «in quale gruppo sta X».`,
		Example: "  ars-sicilia-pp-cli gruppi elenco --legisl 18 --json\n" +
			"  ars-sicilia-pp-cli gruppi elenco --deputato \"Cracolici\" --legisl 18 --json",
		// Niente `pp:endpoint`: quell'annotazione dice «questo comando ha già
		// un tool MCP tipizzato», e cobratree salta i comandi che la portano
		// (classify → commandEndpoint). Qui il tool tipizzato non esiste — i
		// 22 sono gli archivi documentali — quindi annotarla toglieva il
		// comando dall'MCP senza metterlo da nessun'altra parte.
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "--legisl=18"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateLegislaturaGruppi(flagLegisl); err != nil {
				return err
			}
			if dryRunOK(flags) {
				path, params := wwwclient.GruppiElencoReq(flagLegisl)
				return emitGruppiDryRun(cmd, path, params)
			}
			ctx := cmd.Context()
			www := wwwclient.New(flags.timeout, flags.rateLimit)
			www.NoCache = flags.noCache
			gruppi, err := www.GruppiElenco(ctx, flagLegisl)
			if err != nil {
				var nf *wwwclient.NotFoundError
				if errors.As(err, &nf) {
					return notFoundErr(err)
				}
				return err
			}
			if flagDeputato != "" {
				return runGruppiElencoDeputato(cmd, flags, www, gruppi, flagDeputato)
			}
			return emitGruppiElenco(cmd, flags, gruppi)
		},
	}
	cmd.Flags().IntVar(&flagLegisl, "legisl", 18, "Legislatura (16, 17, 18).")
	cmd.Flags().StringVar(&flagDeputato, "deputato", "", "Nome (o cognome) di un parlamentare: restituisce i gruppi cui è iscritto, con ruolo e collegio. Costa una richiesta per gruppo.")
	return cmd
}

// gruppoElencoRow è la riga dell'elenco semplice.
type gruppoElencoRow struct {
	Legisl int    `json:"legislatura"`
	Gruppo string `json:"gruppo"`
	Slug   string `json:"slug"`
	URL    string `json:"url"`
}

func emitGruppiElenco(cmd *cobra.Command, flags *rootFlags, gruppi []wwwclient.Gruppo) error {
	rows := make([]gruppoElencoRow, 0, len(gruppi))
	for _, g := range gruppi {
		rows = append(rows, gruppoElencoRow{Legisl: g.Legisl, Gruppo: g.Nome, Slug: g.Slug, URL: g.URL})
	}
	out := cmd.OutOrStdout()
	if flags.asJSON || flags.csv || flags.plain || flags.quiet || !isTerminal(out) {
		return printJSONFiltered(out, rows, flags)
	}
	header := []string{"legislatura", "gruppo", "slug"}
	tab := make([][]string, 0, len(rows))
	for _, r := range rows {
		tab = append(tab, []string{itoa(r.Legisl), r.Gruppo, r.Slug})
	}
	return flags.printTable(cmd, header, tab)
}

// gruppoDeputatoRow è la riga del mode --deputato: il gruppo (con URL, per
// risalire alla fonte) e i dati del parlamentare dentro quel gruppo.
type gruppoDeputatoRow struct {
	Legisl   int    `json:"legislatura"`
	Gruppo   string `json:"gruppo"`
	URL      string `json:"url"`
	Deputato string `json:"deputato"`
	Ruolo    string `json:"ruolo,omitempty"`
	Collegio string `json:"collegio,omitempty"`
	Email    string `json:"email,omitempty"`
	Scheda   string `json:"scheda"`
}

// nomeDeputatoMatch fa il matching del nome chiesto contro il nome della
// pagina («On.Cognome Nome»): case-insensitive, substring, anche senza il
// prefisso «On.». Chi cerca «Cracolici» deve trovare «On.Cracolici Antonino».
func nomeDeputatoMatch(nomePagina, cercato string) bool {
	q := strings.ToLower(strings.TrimSpace(cercato))
	if q == "" {
		return false
	}
	if strings.Contains(strings.ToLower(nomePagina), q) {
		return true
	}
	// senza il prefisso titolo, per non dipendere dalla forma della pagina
	return strings.Contains(strings.ToLower(strings.TrimPrefix(nomePagina, "On.")), q)
}

func runGruppiElencoDeputato(cmd *cobra.Command, flags *rootFlags, www *wwwclient.Client, gruppi []wwwclient.Gruppo, cercato string) error {
	ctx := cmd.Context()
	rows := []gruppoDeputatoRow{}
	letti := 0
	campione := []string{}
	// I gruppi che non hanno risposto vanno detti per nome: un deputato può
	// stare proprio lì, e un «non trovato» costruito su una copertura parziale
	// è indistinguibile da un'assenza vera. È la stessa regola che vale per
	// `analytics --group-by cofirme` e `--group-by oratore`, dove i deputati
	// senza risposta sono nominati su stderr e mai contati come zero.
	saltati := []string{}
	for _, g := range gruppi {
		det, err := www.GruppoDettaglio(ctx, g.Slug)
		if err != nil {
			saltati = append(saltati, g.Slug)
			continue
		}
		letti++
		for _, c := range det.Componenti {
			if len(campione) < 10 {
				campione = append(campione, c.Deputato)
			}
			if nomeDeputatoMatch(c.Deputato, cercato) {
				rows = append(rows, gruppoDeputatoRow{
					Legisl:   g.Legisl,
					Gruppo:   g.Nome,
					URL:      g.URL,
					Deputato: c.Deputato,
					Ruolo:    c.Ruolo,
					Collegio: c.Collegio,
					Email:    c.Email,
					Scheda:   c.Scheda,
				})
			}
		}
	}
	if len(saltati) > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(),
			"hint: %d gruppi su %d non hanno risposto (%s) e non sono stati guardati: la risposta copre gli altri %d. Riprova, o aprili uno per uno con `gruppi get <slug>`.\n",
			len(saltati), len(gruppi), strings.Join(saltati, ", "), letti)
	}
	if len(rows) == 0 {
		if letti == 0 {
			return fmt.Errorf("impossibile leggere le pagine dei gruppi per cercare %q (problema di rete o sito non raggiungibile?)", cercato)
		}
		msg := fmt.Sprintf("nessun componente %q nei %d gruppi letti. Nomi letti (campione): %s", cercato, letti, strings.Join(campione, ", "))
		if len(saltati) > 0 {
			// Con una copertura parziale l'assenza non è dimostrata, e va detto
			// nel messaggio e non solo su stderr: chi legge solo l'errore deve
			// vedere che restano gruppi non guardati.
			msg += fmt.Sprintf(". Restano non guardati %d gruppi (%s): l'assenza non è dimostrata", len(saltati), strings.Join(saltati, ", "))
		}
		return notFoundErr(errors.New(msg))
	}
	out := cmd.OutOrStdout()
	if flags.asJSON || flags.csv || flags.plain || flags.quiet || !isTerminal(out) {
		return printJSONFiltered(out, rows, flags)
	}
	header := []string{"legislatura", "gruppo", "deputato", "ruolo", "collegio"}
	tab := make([][]string, 0, len(rows))
	for _, r := range rows {
		tab = append(tab, []string{itoa(r.Legisl), r.Gruppo, r.Deputato, r.Ruolo, r.Collegio})
	}
	return flags.printTable(cmd, header, tab)
}

func newGruppiGetCmd(flags *rootFlags) *cobra.Command {
	var flagLegisl int
	cmd := &cobra.Command{
		Use: "get <gruppo>",
		// MaximumNArgs e non ExactArgs: senza argomenti il verify della press e
		// il --dry-run devono vedere l'aiuto, non un errore di aritmetica degli
		// argomenti. Stesso schema di `ddl iter`.
		Args:  cobra.MaximumNArgs(1),
		Short: "La composizione di un gruppo: cariche, collegio di elezione, email e scheda di ogni componente.",
		Long: `Apre la pagina di un gruppo su www.ars.sicilia.it e ne restituisce la
composizione completa: email del gruppo e, per ogni componente, nome, eventuale
carica (Presidente, Vice-Presidente, Segretario, Tesoriere), collegio di
elezione, email istituzionale e URL della scheda.

L'argomento è lo slug della pagina (XVIII-misto, dall'elenco) oppure il nome
del gruppo («misto», «Partito Democratico»), cercato con matching su substring
nella legislatura --legisl (default 18). Con più corrispondenze elenca i
candidati e esce.`,
		Example: "  ars-sicilia-pp-cli gruppi get XVIII-misto --json\n" +
			"  ars-sicilia-pp-cli gruppi get \"Partito Democratico\" --legisl 18 --json",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "gruppo=XVIII-misto"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateLegislaturaGruppi(flagLegisl); err != nil {
				return err
			}
			if len(args) < 1 {
				if dryRunOK(flags) || cliIsVerify() {
					return cmd.Help()
				}
				return usageErr(fmt.Errorf("richiesto 1 argomento: <slug-o-nome> del gruppo (vedi `ars-sicilia-pp-cli gruppi elenco`)"))
			}
			if dryRunOK(flags) {
				path, params := wwwclient.GruppoDettaglioReq(strings.TrimSpace(args[0]))
				return emitGruppiDryRun(cmd, path, params)
			}
			ctx := cmd.Context()
			www := wwwclient.New(flags.timeout, flags.rateLimit)
			www.NoCache = flags.noCache
			slug, err := risolviSlugGruppo(ctx, www, args[0], flagLegisl)
			if err != nil {
				return err
			}
			det, err := www.GruppoDettaglio(ctx, slug)
			if err != nil {
				var nf *wwwclient.NotFoundError
				if errors.As(err, &nf) {
					return notFoundErr(fmt.Errorf("gruppo %q non trovato su www.ars.sicilia.it", slug))
				}
				return err
			}
			return emitGruppoDettaglio(cmd, flags, det)
		},
	}
	cmd.Flags().IntVar(&flagLegisl, "legisl", 18, "Legislatura in cui cercare il gruppo per nome (ignorata se si passa lo slug).")
	return cmd
}

// risolviSlugGruppo accetta lo slug (XVIII-misto) o il nome («misto»).
// Lo slug si riconosce dal prefisso romano; il nome si risolve
// sull'elenco della legislatura, e l'ambiguità è un errore con i candidati.
func risolviSlugGruppo(ctx context.Context, www *wwwclient.Client, arg string, legisl int) (string, error) {
	arg = strings.TrimSpace(arg)
	for _, pref := range []string{"XVIII-", "XVII-", "XVI-"} {
		if strings.HasPrefix(arg, pref) {
			return arg, nil
		}
	}
	gruppi, err := www.GruppiElenco(ctx, legisl)
	if err != nil {
		return "", err
	}
	q := strings.ToLower(arg)
	candidati := []string{}
	for _, g := range gruppi {
		if strings.Contains(strings.ToLower(g.Nome), q) {
			candidati = append(candidati, g.Slug)
		}
	}
	switch len(candidati) {
	case 1:
		return candidati[0], nil
	case 0:
		return "", notFoundErr(fmt.Errorf("nessun gruppo %q nella legislatura %d: usa `gruppi elenco --legisl %d` per vedere i nomi", arg, legisl, legisl))
	default:
		return "", usageErr(fmt.Errorf("%q è ambiguo (%d gruppi contengono quella stringa): usa lo slug — %s", arg, len(candidati), strings.Join(candidati, ", ")))
	}
}

// gruppoDettaglioOut specchia wwwclient.DettaglioGruppo aggiungendo il conteggio:
// chi consuma il JSON non deve contare a mano per sapere quanti sono.
type gruppoDettaglioOut struct {
	Legisl     int                    `json:"legislatura"`
	Gruppo     string                 `json:"gruppo"`
	Slug       string                 `json:"slug"`
	URL        string                 `json:"url"`
	Email      string                 `json:"email,omitempty"`
	Componenti []wwwclient.Componente `json:"componenti"`
	Conteggio  int                    `json:"conteggio"`
}

func emitGruppoDettaglio(cmd *cobra.Command, flags *rootFlags, det *wwwclient.DettaglioGruppo) error {
	out := gruppoDettaglioOut{
		Legisl:     det.Legisl,
		Gruppo:     det.Gruppo,
		Slug:       det.Slug,
		URL:        det.URL,
		Email:      det.Email,
		Componenti: det.Componenti,
		Conteggio:  len(det.Componenti),
	}
	w := cmd.OutOrStdout()
	if flags.asJSON || flags.csv || flags.plain || flags.quiet || !isTerminal(w) {
		return printJSONFiltered(w, out, flags)
	}
	fmt.Fprintf(w, "Gruppo: %s (legislatura %d)\n", out.Gruppo, out.Legisl)
	if out.Email != "" {
		fmt.Fprintf(w, "Email:  %s\n", out.Email)
	}
	fmt.Fprintf(w, "URL:    %s\n", out.URL)
	fmt.Fprintf(w, "\nComponenti (%d):\n", out.Conteggio)
	for _, c := range out.Componenti {
		ruolo := ""
		if c.Ruolo != "" {
			ruolo = " — " + c.Ruolo
		}
		collegio := ""
		if c.Collegio != "" {
			collegio = " — collegio " + c.Collegio
		}
		fmt.Fprintf(w, "  %s%s%s\n    %s\n", c.Deputato, ruolo, collegio, c.Scheda)
	}
	return nil
}
