// pp:client-call
// Hand-written core for the giustizia-amministrativa CLI. The generator's
// generic spec-driven HTML path cannot perform the Liferay session handshake
// (p_auth + cookies) nor parse the portlet result rows, so search/get and the
// novel features share the logic below and call internal/gaclient.
package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/giustizia-amministrativa/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/giustizia-amministrativa/internal/gaclient"
	"github.com/mvanhorn/printing-press-library/library/productivity/giustizia-amministrativa/internal/store"

	"github.com/spf13/cobra"
)

// gaSkip reports whether a live command should short-circuit: either the user
// asked for --dry-run, or we're under the verify harness (which must not hit the
// public institutional site nor contend on the shared SQLite store).
func gaSkip(flags *rootFlags) bool {
	return dryRunOK(flags) || cliutil.IsVerifyEnv()
}

// emitSkip chiude un comando uscito per --dry-run senza lasciare stdout vuoto.
//
// Con --json un'uscita muta non e' una risposta: chi legge riceve zero byte
// dove si aspetta un documento, e non puo' distinguere "non ho eseguito perche'
// me l'hai chiesto" da "sono andato in errore senza dirlo". Il verificatore la
// tratta infatti come JSON non valido. Senza --json il silenzio va bene: il
// comando non ha fatto nulla e non ha nulla da mostrare.
func emitSkip(cmd *cobra.Command, flags *rootFlags) error {
	if flags == nil || !flags.asJSON {
		return nil
	}
	_, err := fmt.Fprintln(cmd.OutOrStdout(), `{"dry_run":true,"eseguito":false}`)
	return err
}

// gaStorePath returns the local SQLite path for this CLI.
func gaStorePath() string {
	return defaultDBPath("giustizia-amministrativa-pp-cli")
}

// openGAStore opens (and migrates) the local store.
func openGAStore(ctx context.Context) (*store.Store, error) {
	return store.OpenWithContext(ctx, gaStorePath())
}

// provID is the stable store key for a provvedimento (ECLI, else idprovv).
func provID(p gaclient.Provvedimento) string {
	if p.Ecli != "" {
		return p.Ecli
	}
	return p.Idprovv
}

// persistProvvedimenti upserts rows into the local store, preserving any
// previously stored full_text and registry metadata when the incoming row
// doesn't carry them. A search result carries neither: without this, every
// search would erase the text and the metadata already fetched for the same
// provvedimento, and the next reader would go back to the portal for a
// document the store already held.
func persistProvvedimenti(st *store.Store, items []gaclient.Provvedimento) {
	for _, p := range items {
		id := provID(p)
		if id == "" {
			continue
		}
		if p.FullText == "" || p.Meta == nil {
			if existing, err := st.Get("provvedimenti", id); err == nil && len(existing) > 0 {
				var prev gaclient.Provvedimento
				if json.Unmarshal(existing, &prev) == nil {
					if p.FullText == "" && prev.FullText != "" {
						p.FullText = prev.FullText
					}
					if p.Meta == nil && prev.Meta != nil {
						p.Meta = prev.Meta
					}
				}
			}
		}
		data, err := json.Marshal(p)
		if err != nil {
			continue
		}
		_ = st.UpsertProvvedimenti(data)
	}
}

// runGASearch performs a live search, persists results to the local store, and
// prints them honoring --json/--select/--csv/--compact. provenanceNote is shown
// to humans on stderr.
func runGASearch(cmd *cobra.Command, flags *rootFlags, opts gaclient.SearchOptions) error {
	if gaSkip(flags) {
		return emitSkip(cmd, flags)
	}
	c := gaclient.New()
	res, err := c.Search(cmd.Context(), opts)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	// Gli avvisi si raccolgono oltre che stamparsi: su stdout in JSON finiscono
	// nell'envelope, perche' un agente legge solo quello. Vedi emitRicerca.
	var avvisi []string
	for _, w := range res.Warnings {
		avvisi = append(avvisi, w)
		fmt.Fprintf(cmd.ErrOrStderr(), "Attenzione: %s\n", w)
	}
	// Best-effort persistence (offline search, watch, grep, stats build on this).
	if st, serr := openGAStore(cmd.Context()); serr == nil {
		persistProvvedimenti(st, res.Items)
		// Annotate match_count from cached full texts — free, no extra API
		// calls. Distinguishes "1 match in an obiter" from "12 matches in
		// the dispositivo". Absent when the text isn't cached yet.
		terms := searchTerms(opts)
		if len(terms) > 0 {
			for i := range res.Items {
				if ft := storedFullText(st, res.Items[i]); ft != "" {
					res.Items[i].MatchCount = countMatches(ft, terms)
				}
			}
		}
		_ = st.Close()
	}
	// A zero-result search is an answer in its own right — "there is no
	// Adunanza Plenaria on this since 2023" is exactly what a lawyer needs to
	// establish — but on its own it is unverifiable: it looks identical
	// whether the case law is absent or a filter was too tight. One extra
	// query, run only on zero, says which of the two it is.
	if res.Total == 0 && hasNarrowingFilter(opts) {
		bare := opts
		bare.Tipo, bare.Sede, bare.SedeSweep = "", "", false
		bare.Anno, bare.AnnoFrom, bare.AnnoTo = 0, 0, 0
		bare.Numero, bare.Nrg, bare.AnnoNrg = 0, 0, 0
		bare.Limit = 1
		if probe, perr := c.Search(cmd.Context(), bare); perr == nil {
			switch {
			case probe.Total == 0:
				nota := fmt.Sprintf("nessun risultato, e nemmeno senza i filtri (%s): il portale non trova nulla per questi termini di ricerca.", activeFilters(opts))
				avvisi = append(avvisi, nota)
				fmt.Fprintf(cmd.ErrOrStderr(), "Nota: %s\n", nota)
			default:
				nota := fmt.Sprintf("nessun risultato con i filtri applicati (%s), ma la stessa ricerca senza filtri ne dichiara %d. Il vuoto viene dai filtri, non dai termini di ricerca.", activeFilters(opts), probe.Total)
				avvisi = append(avvisi, nota)
				fmt.Fprintf(cmd.ErrOrStderr(), "Nota: %s\n", nota)
			}
		}
	}

	if res.Total > 0 {
		// Total is the match count declared by the portal (summed per year in a
		// sweep), not the number of rows returned: say so, or the two numbers
		// look contradictory whenever Total exceeds --limit.
		if wantsHumanTable(cmd.OutOrStdout(), flags) {
			fmt.Fprintf(cmd.ErrOrStderr(), "Trovati %d risultati sul portale (mostrati %d).\n", res.Total, len(res.Items))
		} else if res.Total > len(res.Items) {
			// Machine consumers get the array alone, whose length is the sample
			// size — so without this the denominator is simply absent, and a
			// request for "the most relevant N" cannot be calibrated by anyone:
			// N out of how many is unknowable. Prefixed as a note so the MCP
			// layer carries it into the result's `avvisi` field.
			nota := fmt.Sprintf("il portale dichiara %d risultati per questa query, questi sono i primi %d. Il numero di elementi restituiti e' la dimensione del campione, non il totale.", res.Total, len(res.Items))
			avvisi = append(avvisi, nota)
			fmt.Fprintf(cmd.ErrOrStderr(), "Nota: %s\n", nota)
		}
	}
	// Solo su stderr, non fra gli `avvisi` dell'envelope. Gli avvisi
	// incapsulati spiegano un risultato parziale — gemelli raggruppati,
	// campione piu' piccolo del totale — e sono l'eccezione. Questa nota
	// descrive una proprieta' costante dell'endpoint, vera su ogni ricerca:
	// metterla nell'envelope significherebbe incapsulare sempre, e l'array
	// nudo di `--json` smetterebbe di esistere per chiunque. Il layer MCP la
	// consegna comunque al client, perche' legge da stderr le righe con
	// prefisso "Nota: " (warningsFromStderr).
	if nota := notaDataDepositoAssente(res.Items); nota != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "Nota: %s\n", nota)
	}
	data, err := json.Marshal(res.Items)
	if err != nil {
		return err
	}
	return emitRicerca(cmd.OutOrStdout(), data, avvisi, flags)
}

// emitRicerca scrive i risultati aggiungendo gli avvisi allo stdout JSON quando
// ce ne sono.
//
// Il motivo: gli avvisi che spiegano un risultato parziale - i ricorsi gemelli
// raggruppati, il totale dichiarato dal portale piu' alto del campione, i
// filtri che hanno svuotato la ricerca - viaggiano su stderr, e un agente che
// legge solo stdout non li vede. Chiede 100 righe, ne riceve 92, e non ha modo
// di sapere perche'. Il livello MCP li inserisce gia' nel campo `avvisi`
// (shellout.go); qui si fa lo stesso per chi chiama la CLI direttamente.
//
// Senza avvisi la forma resta l'array nudo di sempre, cosi' il caso ordinario
// non cambia contratto. --select e --compact si applicano agli elementi prima
// dell'incapsulamento, altrimenti filtrerebbero l'envelope invece dei
// provvedimenti. CSV, --quiet e la tabella umana restano fuori: un CSV di un
// envelope non vuole dire nulla, e a video gli avvisi sono gia' su stderr.
func emitRicerca(w io.Writer, data json.RawMessage, avvisi []string, flags *rootFlags) error {
	if len(avvisi) == 0 || flags == nil || !flags.asJSON || flags.csv || flags.quiet {
		return printOutputWithFlags(w, data, flags)
	}
	if flags.selectFields != "" {
		data = filterFields(data, flags.selectFields)
	} else if flags.compact {
		data = compactFields(data)
	}
	envelope, err := json.Marshal(map[string]any{"items": data, "avvisi": avvisi})
	if err != nil {
		return err
	}
	return printOutput(w, envelope, true)
}

// hasNarrowingFilter reports whether the query carries anything beyond the
// search terms, i.e. whether an empty result could be the filters' doing.
func hasNarrowingFilter(o gaclient.SearchOptions) bool {
	return o.Tipo != "" || o.Sede != "" || o.SedeSweep ||
		o.Anno != 0 || o.AnnoFrom != 0 || o.AnnoTo != 0 ||
		o.Numero != 0 || o.Nrg != 0 || o.AnnoNrg != 0
}

// activeFilters names the filters in play, for a message that tells the reader
// what to loosen rather than just that something was tight.
func activeFilters(o gaclient.SearchOptions) string {
	var f []string
	add := func(name, val string) {
		if val != "" {
			f = append(f, name+"="+val)
		}
	}
	addN := func(name string, val int) {
		if val != 0 {
			f = append(f, name+"="+strconv.Itoa(val))
		}
	}
	add("tipo", o.Tipo)
	add("sede", o.Sede)
	if o.SedeSweep {
		f = append(f, "sede-sweep")
	}
	addN("anno", o.Anno)
	addN("anno-from", o.AnnoFrom)
	addN("anno-to", o.AnnoTo)
	addN("numero", o.Numero)
	addN("nrg", o.Nrg)
	addN("anno-nrg", o.AnnoNrg)
	if len(f) == 0 {
		return "nessuno"
	}
	return strings.Join(f, ", ")
}

// storedFullText returns the Markdown already saved for p, or "" if the store
// has none. A search returns metadata only, so an item arrives here without
// its text even when the store holds it from an earlier get or corpus run —
// and the text of a published ruling cannot change, so refetching it is a
// request to the portal for bytes we already have. Commands that walk many
// results (massime, corpus build) pay that cost once per document otherwise.
func storedFullText(st *store.Store, p gaclient.Provvedimento) string {
	if st == nil {
		return ""
	}
	id := provID(p)
	if id == "" {
		return ""
	}
	raw, err := st.Get("provvedimenti", id)
	if err != nil || len(raw) == 0 {
		return ""
	}
	var prev gaclient.Provvedimento
	if json.Unmarshal(raw, &prev) != nil {
		return ""
	}
	return prev.FullText
}

// noTextNotice is the body written for a provvedimento whose document yields
// no extractable text. The portal publishes part of its rulings as PDF, which
// the HTML-to-Markdown conversion cannot render: the result is an empty
// document that looks complete and simply appears to say nothing. Say what
// happened and where the original is, so an empty page is never mistaken for
// a ruling with no content.
func noTextNotice(p gaclient.Provvedimento) string {
	return fmt.Sprintf("> **Testo non disponibile in formato testuale.**\n"+
		"> Il portale pubblica questo provvedimento come %s, che non è convertibile in Markdown.\n"+
		"> Scarica l'originale: %s\n", documentFormat(p), p.URL)
}

// documentFormat names the document's format for a message. The search
// metadata carries it, but a direct --sede/--nrg/--file fetch has none, so
// fall back to the file extension before giving up on naming it.
func documentFormat(p gaclient.Provvedimento) string {
	if f := strings.ToUpper(strings.TrimSpace(p.Formato)); f != "" {
		return f
	}
	if ext := strings.TrimPrefix(strings.ToUpper(filepath.Ext(p.NomeFile)), "."); ext != "" {
		return ext
	}
	return "formato non testuale"
}

// noTextLabel identifies the provvedimento in a warning, falling back to the
// document coordinates when a direct fetch left the registry fields empty.
func noTextLabel(p gaclient.Provvedimento) string {
	if id := provID(p); id != "" {
		return id
	}
	if p.NomeFile != "" {
		return p.NomeFile
	}
	return "il provvedimento richiesto"
}

// hasNoExtractableText reports whether the converted document came out empty.
func hasNoExtractableText(markdown string) bool {
	return strings.TrimSpace(markdown) == ""
}

// resolveProvvedimento finds a provvedimento by id (ECLI or idprovv). It looks
// in the local store first; if absent it returns an error guiding the user to
// run a search first.
func resolveProvvedimento(ctx context.Context, st *store.Store, id string) (gaclient.Provvedimento, error) {
	var p gaclient.Provvedimento
	raw, err := st.Get("provvedimenti", id)
	if err == nil && len(raw) > 0 {
		if json.Unmarshal(raw, &p) == nil {
			return p, nil
		}
	}
	// Try matching by idprovv across stored rows.
	rows, lerr := st.List("provvedimenti", 100000)
	if lerr == nil {
		for _, r := range rows {
			var cand gaclient.Provvedimento
			if json.Unmarshal(r, &cand) == nil && (cand.Idprovv == id || cand.Ecli == id) {
				return cand, nil
			}
		}
	}
	return p, fmt.Errorf("provvedimento %q non trovato nello store locale: esegui prima una ricerca (es. `giustizia-amministrativa-pp-cli search \"<termine>\"`) oppure passa --sede/--nrg/--file", id)
}

// runGAGet fetches the full text of a provvedimento and renders it in the
// requested format (md, text, html, json). When frontMatter is set and the
// format is md/text, a YAML front-matter block with the provvedimento metadata
// is prepended (no-op for json/html, which already carry the fields).
func runGAGet(cmd *cobra.Command, flags *rootFlags, id, format, sede, nrg, file string, frontMatter, meta bool) error {
	if gaSkip(flags) {
		return emitSkip(cmd, flags)
	}
	c := gaclient.New()
	var p gaclient.Provvedimento

	if sede != "" && nrg != "" && file != "" {
		// Direct fetch without a prior search.
		p = gaclient.Provvedimento{Schema: sede, Nrg: nrg, NomeFile: file, URL: gaclient.DocURL(sede, nrg, file)}
	} else {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("specifica un id (ECLI o idprovv) oppure --sede --nrg --file")
		}
		st, err := openGAStore(cmd.Context())
		if err != nil {
			return err
		}
		p, err = resolveProvvedimento(cmd.Context(), st, id)
		if err != nil {
			_ = st.Close()
			return err
		}
		_ = st.Close()
	}

	doc, err := c.Document(cmd.Context(), p)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	docHTML := doc.Raw
	if p.DataDeposito == "" {
		p.DataDeposito = gaclient.ExtractDataDeposito(docHTML)
	}
	if meta {
		// Not fatal: the document is already in hand, and the metadata is an
		// addition to it. Say what is missing and carry on.
		m, merr := c.Meta(cmd.Context(), p)
		switch {
		case merr != nil:
			fmt.Fprintf(cmd.ErrOrStderr(), "Attenzione: metadati non recuperati per %s: %v\n", noTextLabel(p), merr)
		case m.Empty():
			fmt.Fprintf(cmd.ErrOrStderr(), "Attenzione: %s non ha una forma XML con i metadati di registro (formato %s)\n", noTextLabel(p), documentFormat(p))
		default:
			p.Meta = &m
			// Same date under two names: the portal prints it as "Pubblicato
			// il" in the document, and that wording is one of the two that
			// ExtractDataDeposito already reads into data_deposito. The XML
			// value fills the field only where the document states none —
			// measured on a sample of 10, one parere of the Consiglio di
			// Stato where the date is in the registry and not in the text.
			if p.DataDeposito == "" {
				p.DataDeposito = m.DataPubblicazione
			}
		}
	}
	// A PDF's text layer is already plain text; running the HTML converter
	// over it would mangle what it does not need to touch.
	markdown := docHTML
	if !doc.IsPDF {
		markdown = gaclient.HTMLToMarkdown(docHTML)
	}
	p.FullText = markdown

	// The no-text check belongs here, before any format branch: JSON is what
	// the MCP tool actually returns, and there the empty conversion used to
	// surface as a record with full_text simply missing — same schema, same
	// keys, no error. A caller reading such a record cannot tell it apart from
	// a valid one, and a corpus built on it is silently full of holes.
	noText := hasNoExtractableText(markdown)

	// Persist before substituting the notice: storing it as full_text would
	// leave the store holding an explanation dressed as a document, which
	// every reader of the store (corpus build, massime, grep) would then
	// serve as the ruling's text — and would keep serving even after the
	// extraction starts working.
	if id != "" {
		if st, serr := openGAStore(cmd.Context()); serr == nil {
			persistProvvedimenti(st, []gaclient.Provvedimento{p})
			_ = st.Close()
		}
	}

	// The store keeps whatever metadata a previous `--meta` run fetched, and
	// resolveProvvedimento loads it back. Emitting it here would make the
	// output of a plain `get` depend on whether someone once asked for the
	// metadata: opt-in has to mean opt-in on every call. This runs after the
	// write above, so a plain get does not erase what the store already has:
	// dropping it there would send the next --meta back to the portal for a
	// document it already holds.
	if !meta {
		p.Meta = nil
	}

	if noText {
		fmt.Fprintf(cmd.ErrOrStderr(), "Attenzione: %s non ha testo estraibile (formato %s): scaricare l'originale da %s\n", noTextLabel(p), documentFormat(p), p.URL)
		markdown = noTextNotice(p)
		p.FullText = markdown
	}

	if flags.asJSON {
		data, err := json.Marshal(p)
		if err != nil {
			return err
		}
		if noText {
			// Marshal into a map to add the flag without changing the
			// Provvedimento type, which is shared with the search output.
			var obj map[string]any
			if json.Unmarshal(data, &obj) == nil {
				obj["testo_non_estraibile"] = true
				obj["formato_documento"] = documentFormat(p)
				if b, merr := json.Marshal(obj); merr == nil {
					data = b
				}
			}
		}
		return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
	}
	switch strings.ToLower(format) {
	case "", "md", "markdown":
		if frontMatter {
			fmt.Fprintln(cmd.OutOrStdout(), gaclient.FrontMatter(p))
		}
		fmt.Fprintln(cmd.OutOrStdout(), markdown)
	case "text", "txt":
		if frontMatter {
			fmt.Fprintln(cmd.OutOrStdout(), gaclient.FrontMatter(p))
		}
		plain := docHTML
		if !doc.IsPDF {
			plain = gaclient.HTMLToText(docHTML)
		}
		if noText {
			plain = noTextNotice(p)
		}
		fmt.Fprintln(cmd.OutOrStdout(), plain)
	case "html":
		fmt.Fprintln(cmd.OutOrStdout(), docHTML)
	case "json":
		data, _ := json.Marshal(p)
		return printOutput(cmd.OutOrStdout(), data, true)
	default:
		return fmt.Errorf("formato non valido: %q (usa md, text, html o json)", format)
	}
	return nil
}
