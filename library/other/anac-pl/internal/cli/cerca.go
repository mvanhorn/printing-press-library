package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// fetchFullText scorre /avvisi-full-text con la paginazione a TOKEN, l'unica
// che il servizio onori. Il parametro `page` viene accettato e ignorato:
// misurato il 18/09/2026 con la stessa query, page 0, 1, 2, 10, 50 e 100
// restituiscono gli stessi idAvviso. Le pagine successive si chiedono con
// direzionePaginazione=AVANTI + tokenPaginazione=<lastPaginationToken>, in
// modalita' estesa come in esatta. Gli avvisi tornano deduplicati per idAvviso,
// insieme al `count` dichiarato dal servizio.
type fullTextGetter interface {
	GetWithHeaders(context.Context, string, map[string]string, map[string]string) (json.RawMessage, error)
}

func fetchFullText(ctx context.Context, c fullTextGetter, base map[string]string, pages int) ([]json.RawMessage, int64, int, error) {
	return fetchFullTextWithHeaders(ctx, c, base, pages, nil, paginatedGetMaxPages)
}

// pages == 0 follows all continuation tokens for avvisi search --all.
func fetchFullTextWithHeaders(ctx context.Context, c fullTextGetter, base map[string]string, pages int, headers map[string]string, maxPages int) ([]json.RawMessage, int64, int, error) {
	size, _ := strconv.Atoi(base["size"])
	var out []json.RawMessage
	var total int64
	fetched := 0
	seen := map[string]bool{}
	token := ""
	seenTokens := map[string]bool{}
	for p := 0; pages == 0 || p < pages; p++ {
		if pages == 0 && p >= maxPages {
			return out, total, fetched, fmt.Errorf("ricerca incompleta: raggiunto il limite di %d pagine; riprova con --max-pages maggiore", maxPages)
		}
		params := map[string]string{}
		for k, v := range base {
			params[k] = v
		}
		if token != "" {
			params["direzionePaginazione"] = "AVANTI"
			params["tokenPaginazione"] = token
		}
		data, err := c.GetWithHeaders(ctx, "/avvisi-full-text", params, headers)
		if err != nil {
			return out, total, fetched, err
		}
		var env struct {
			Content             []json.RawMessage `json:"content"`
			Count               float64           `json:"count"`
			LastPaginationToken string            `json:"lastPaginationToken"`
		}
		if err := json.Unmarshal(data, &env); err != nil {
			return out, total, fetched, fmt.Errorf("risposta di /avvisi-full-text non decodificabile: %w", err)
		}
		if pages == 0 && env.Content == nil {
			return out, total, fetched, fmt.Errorf("ricerca incompleta: risposta di /avvisi-full-text senza array content")
		}
		if len(env.Content) == 0 {
			if pages == 0 && env.LastPaginationToken != "" {
				return out, total, fetched, fmt.Errorf("ricerca incompleta: pagina vuota con token di continuazione")
			}
			break
		}
		fetched++
		if env.Count > 0 {
			total = int64(env.Count)
		}
		added := 0
		for _, raw := range env.Content {
			var idOnly struct {
				IDAvviso string `json:"idAvviso"`
			}
			decodeErr := json.Unmarshal(raw, &idOnly)
			if pages == 0 && (decodeErr != nil || idOnly.IDAvviso == "") {
				return out, total, fetched, fmt.Errorf("ricerca incompleta: avviso senza idAvviso valido nella pagina %d", p+1)
			}
			if idOnly.IDAvviso == "" || seen[idOnly.IDAvviso] {
				continue
			}
			seen[idOnly.IDAvviso] = true
			out = append(out, raw)
			added++
		}
		if env.LastPaginationToken == "" || (pages > 0 && (added == 0 || (size > 0 && len(env.Content) < size))) {
			break
		}
		if pages == 0 && seenTokens[env.LastPaginationToken] {
			return out, total, fetched, fmt.Errorf("ricerca incompleta: la paginazione ANAC non avanza")
		}
		token = env.LastPaginationToken
		seenTokens[token] = true
	}
	return out, total, fetched, nil
}

// newCercaCmd is a hand-authored, human-friendly front end to the
// /avvisi-full-text endpoint. It improves on the raw `avvisi search` by:
//   - accepting a tipologia by name/slug (e.g. "bandi", "esiti") or template id
//   - accepting a free importo range via --amount-min/--amount-max (the web form
//     only offers 4 fixed bands, but the API accepts any min,max)
//   - exposing the search "modalità" (estesa | esatta | archivio) as one flag
//     instead of the two derived params atlasFuzzySearchEnabled / ricercaArchivio
func newCercaCmd(flags *rootFlags) *cobra.Command {
	var query, tipologia, cpv, from, to, mode, sortField, sortDir string
	var amountMin, amountMax string
	var page, pages, size int

	cmd := &cobra.Command{
		Use:   "cerca",
		Short: "Ricerca avvisi con filtri semplici (tipologia per nome, importo min/max, modalità)",
		Long: strings.Trim(`
Ricerca full-text degli avvisi ANAC con filtri facili da usare.

Tipologia: passa un nome o slug (bandi, esiti, indagini-sopra-soglia, ...) oppure
il numero template (4, 7, 5a, ...). Vedi 'tipologie list'.

Importo: --amount-min/--amount-max accettano qualsiasi soglia in euro (non solo
le 4 fasce fisse del sito). Esempi: --amount-min 200000 --amount-max 500000.

Pagine: --pages N scarica N pagine da --size risultati ciascuna e le unisce
(deduplicate per idAvviso). Il servizio pagina a token, non per numero di
pagina: --page non esiste più, perché veniva accettato e ignorato.

Modalità (--mode):
  estesa   (default) ricerca estesa/fuzzy
  esatta   frase esatta: parole adiacenti nell'ordine dato; richiede --tipologia
  archivio cerca nell'archivio storico (usa un intervallo date < 6 mesi)
`, "\n"),
		Example: strings.Trim(`
  anac-pl-pp-cli cerca --query microsoft --size 5
  anac-pl-pp-cli cerca --query "ufficio stampa" --size 50 --pages 4 --json
  anac-pl-pp-cli cerca --tipologia esiti --query "servizi informatici" --json
  anac-pl-pp-cli cerca --tipologia bandi --amount-min 1000000 --published-from 01/01/2025
  anac-pl-pp-cli cerca --cpv 72000000 --tipologia 7 --agent --select content.idAvviso,content.codiceScheda
`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			// --page c'era e non serviva a nulla: il servizio lo accetta e
			// restituisce sempre la prima pagina. Meglio un errore che una
			// paginazione immaginaria.
			if cmd.Flags().Changed("page") {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--page non è supportato: ANAC pagina a token e ignora il numero di pagina. Usa --pages %d per scaricare le prime %d pagine", page+1, page+1))
			}
			if pages <= 0 {
				pages = 1
			}
			if size <= 0 {
				size = 10
			}
			if dryRunOK(flags) {
				fmt.Fprintln(cmd.OutOrStdout(), "would search avvisi (live)")
				return nil
			}

			params := map[string]string{
				"size": strconv.Itoa(size),
			}
			if query != "" {
				params["keywords"] = query
			}
			if cpv != "" {
				params["cpv"] = cpv
				// Il campo cpv di /avvisi-full-text è un match testuale sulle
				// descrizioni, non un filtro sul codice del lotto: restituisce
				// anche avvisi di CPV diversi. Vedi docs/note-per-anac.md.
				fmt.Fprintln(cmd.ErrOrStderr(), "avviso: il filtro CPV di 'cerca' non è selettivo (match testuale). Per filtrare davvero per codice usa 'cerca-avanzata --cpv'")
			}
			if from != "" {
				params["dataPubblicazioneStart"] = from
			}
			if to != "" {
				params["dataPubblicazioneEnd"] = to
			}
			if sortField != "" {
				params["sortField"] = sortField
			}
			if sortDir != "" {
				d := strings.ToUpper(sortDir)
				if d != "ASC" && d != "DESC" {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--sort-dir deve essere ASC o DESC"))
				}
				params["sortDirection"] = d
			}
			warnOrdinamentoIgnorato(cmd.ErrOrStderr(), query, sortField, sortDir)
			warnCIGNonValido(cmd.ErrOrStderr(), query)

			// tipologia -> codiceScheda (template id)
			if tipologia != "" {
				tpl, ok := resolveTipologia(tipologia)
				if !ok {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("tipologia %q non riconosciuta; vedi 'tipologie list'", tipologia))
				}
				params["codiceScheda"] = tpl
			}

			// importo range -> importoLotto = "min,max" (max 0 = aperto verso l'alto)
			if amountMin != "" || amountMax != "" {
				lo := amountMin
				if lo == "" {
					lo = "0"
				}
				hi := amountMax
				if hi == "" {
					hi = "0"
				}
				if !isAmount(lo) || !isAmount(hi) {
					_ = cmd.Usage()
					return usageErr(fmt.Errorf("--amount-min/--amount-max devono essere numeri interi (euro)"))
				}
				params["importoLotto"] = lo + "," + hi
			}

			// modalità ricerca
			switch strings.ToLower(strings.TrimSpace(mode)) {
			case "", "estesa", "atlas":
				params["atlasFuzzySearchEnabled"] = "true"
			case "esatta", "base":
				params["atlasFuzzySearchEnabled"] = "false"
			case "archivio", "archive":
				params["atlasFuzzySearchEnabled"] = "true"
				params["ricercaArchivio"] = "true"
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--mode deve essere uno tra: estesa, esatta, archivio"))
			}
			if err := verificaRicercaEsatta(params["atlasFuzzySearchEnabled"] != "false", params["codiceScheda"]); err != nil {
				_ = cmd.Usage()
				return usageErr(err)
			}

			c, err := flags.newClient()
			if err != nil {
				return err
			}
			items, total, fetched, err := fetchFullText(cmd.Context(), c, params, pages)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			if flags.asJSON || flags.agent || (!isTerminal(cmd.OutOrStdout()) && !flags.csv && !flags.quiet && !flags.plain) {
				env := map[string]any{"count": total, "content": items}
				return printJSONFiltered(cmd.OutOrStdout(), env, flags)
			}

			// human: table of the content array
			fmt.Fprintf(cmd.ErrOrStderr(), "risultati totali: %d (scaricati %d, %d pagine da %d)\n", total, len(items), fetched, size)
			if len(items) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "nessun risultato")
				return nil
			}
			rows := make([]map[string]any, 0, len(items))
			for _, raw := range items {
				var m map[string]any
				if json.Unmarshal(raw, &m) != nil {
					continue
				}
				rows = append(rows, m)
			}
			return printAutoTable(cmd.OutOrStdout(), rows)
		},
	}

	f := cmd.Flags()
	f.StringVarP(&query, "query", "q", "", "Testo libero: parola chiave, CIG, CUP, stazione appaltante, oggetto")
	f.StringVarP(&tipologia, "tipologia", "t", "", "Tipologia avviso: nome/slug (bandi, esiti, ...) o template (4, 7, 5a). Vedi 'tipologie list'")
	f.StringVar(&cpv, "cpv", "", "Codice CPV (vedi 'cpv search')")
	f.StringVar(&amountMin, "amount-min", "", "Importo lotto minimo in euro (range libero)")
	f.StringVar(&amountMax, "amount-max", "", "Importo lotto massimo in euro (range libero; vuoto/0 = nessun limite)")
	f.StringVar(&from, "published-from", "", "Data pubblicazione minima, formato GG/MM/AAAA")
	f.StringVar(&to, "published-to", "", "Data pubblicazione massima, formato GG/MM/AAAA")
	f.StringVar(&mode, "mode", "estesa", "Modalità ricerca: estesa | esatta | archivio")
	f.StringVar(&sortField, "sort-field", "", "Campo di ordinamento (es. dataPubblicazione). Il servizio lo onora solo senza --query: con testo libero ordina per rilevanza")
	f.StringVar(&sortDir, "sort-dir", "", "Direzione ordinamento: ASC o DESC")
	f.IntVar(&page, "page", 0, "Non supportato: ANAC ignora il numero di pagina, usa --pages")
	_ = f.MarkHidden("page")
	f.IntVar(&pages, "pages", 1, "Numero di pagine da scaricare (paginazione a token)")
	f.IntVar(&size, "size", 10, "Risultati per pagina")
	return cmd
}

func isAmount(s string) bool {
	if s == "" {
		return false
	}
	if _, err := strconv.ParseInt(s, 10, 64); err != nil {
		return false
	}
	return true
}
