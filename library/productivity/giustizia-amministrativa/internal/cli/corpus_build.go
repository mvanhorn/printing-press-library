// pp:client-call
// pp:data-source live
// Novel feature: assemble N provvedimenti on a theme into a folder of clean
// Markdown files plus a CSV manifest (ECLI, tipo, sede, data, url, file).
package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/giustizia-amministrativa/internal/gaclient"
)

var reUnsafe = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func newNovelCorpusBuildCmd(flags *rootFlags) *cobra.Command {
	var f searchFlags
	var out, ids string
	var skipExisting, plan, frontMatter, refresh bool
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Assembla N provvedimenti su un tema in Markdown + un CSV manifest.",
		Long: "Scrive una cartella-corpus: un .md per provvedimento più un manifest.csv con ECLI,\n" +
			"tipo, sede, sezione, anno, numero, NRG, data di deposito e URL pubblico di ciascuno.\n" +
			"Rispetto a salvare i testi a mano aggiunge proprio questo: nomi file deterministici,\n" +
			"la provenienza di ogni atto tracciata nel manifest (e nell'intestazione con\n" +
			"--front-matter), e i testi persistiti nello store locale per grep/massime/stats.\n" +
			"I provvedimenti si indicano in due modi alternativi: con i criteri di ricerca (--testo,\n" +
			"--all, --tipo, --sede, ...), oppure con --ids, passando la lista di ECLI/idprovv gia'\n" +
			"scelti — utile quando la selezione l'hai fatta tu leggendo i testi, e non vuoi che una\n" +
			"nuova ricerca la sostituisca con altri risultati.\n" +
			"I testi integrali gia' scaricati in precedenza (per esempio con `get`, mentre valutavi\n" +
			"quali provvedimenti tenere) NON vengono riscaricati: si riusano dallo store locale, quindi\n" +
			"su una selezione gia' letta il corpus si scrive senza altre richieste al portale. Usa\n" +
			"--refresh per forzare comunque il ri-download.\n" +
			"Con --skip-existing i provvedimenti il cui .md è già presente in --out non vengono riscaricati\n" +
			"(corpus incrementale), restando comunque elencati nel manifest. Con --plan non scarica nulla:\n" +
			"scrive solo il manifest delle candidate, per rivedere la query prima di impegnare N richieste.\n" +
			"Con --front-matter l'intestazione di ogni .md è il blocco YAML di `get --front-matter`\n" +
			"invece dell'intestazione Markdown predefinita.",
		Example: strings.Trim(`
  giustizia-amministrativa-pp-cli corpus build --testo "soccorso istruttorio" --tipo sentenza --limit 3 --out ./corpus
  giustizia-amministrativa-pp-cli corpus build --all "clausola sociale" --sede roma --limit 20 --out ./clausola-sociale
  giustizia-amministrativa-pp-cli corpus build --ids "ECLI:IT:CDS:2020:4665SENT,ECLI:IT:TARLAZ:2026:9344SENT" --out ./selezione
  giustizia-amministrativa-pp-cli corpus build --all "accesso generalizzato" --plan --out ./corpus
  giustizia-amministrativa-pp-cli corpus build --all "accesso generalizzato" --skip-existing --front-matter --out ./corpus`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "--ids=ECLI:IT:CDS:2020:4665SENT;--out=./corpus-dogfood"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if gaSkip(flags) {
				return emitSkip(cmd, flags)
			}
			if out == "" {
				return fmt.Errorf("specifica la cartella di destinazione con --out")
			}
			opts := f.opts("")
			idList := splitIDs(ids)
			switch {
			case len(idList) > 0 && hasAnySearchInput(opts):
				return fmt.Errorf("usa --ids oppure i criteri di ricerca, non entrambi: con --ids il corpus e' esattamente la lista che hai passato")
			case len(idList) == 0 && !hasAnySearchInput(opts):
				return fmt.Errorf("specifica --ids con la lista di ECLI/idprovv, oppure almeno un criterio di ricerca (--testo, --all, --tipo, --sede, ...)")
			}
			if opts.Limit == 0 {
				opts.Limit = 25
			}
			// Resolve --out to an absolute path before using or reporting it.
			// A relative path resolves against the process working directory,
			// which under an MCP host is wherever the host happened to launch
			// the server: the corpus lands somewhere the user will never find
			// while the command still reports success. Reporting the absolute
			// path is also the only way the answer to "where did you save the
			// files" can be checked.
			if abs, aerr := filepath.Abs(out); aerr == nil {
				out = abs
			}
			if err := os.MkdirAll(out, 0o755); err != nil {
				return fmt.Errorf("creazione della cartella di destinazione %s: %w", out, err)
			}
			c := gaclient.New()
			st, _ := openGAStore(cmd.Context())
			if st != nil {
				defer st.Close()
			}

			var items []gaclient.Provvedimento
			if len(idList) > 0 {
				// Curated list: resolve each id, reporting the ones that cannot be
				// found instead of quietly shrinking the corpus the caller asked for.
				for _, id := range idList {
					p, rerr := resolveProvvedimento(cmd.Context(), st, id)
					if rerr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "Attenzione: %s non risolto, escluso dal corpus: %v\n", id, rerr)
						continue
					}
					items = append(items, p)
				}
				if len(items) == 0 {
					return fmt.Errorf("nessuno dei %d id indicati e' stato risolto", len(idList))
				}
			} else {
				res, err := c.Search(cmd.Context(), opts)
				if err != nil {
					return classifyAPIError(err, flags)
				}
				for _, w := range res.Warnings {
					fmt.Fprintf(cmd.ErrOrStderr(), "Attenzione: %s\n", w)
				}
				items = res.Items
			}

			manifestPath := filepath.Join(out, "manifest.csv")
			mf, err := os.Create(manifestPath)
			if err != nil {
				return err
			}
			defer mf.Close()
			w := csv.NewWriter(mf)
			_ = w.Write([]string{"ecli", "tipo", "sede", "sezione", "anno", "numero", "nrg", "data_deposito", "url", "file"})

			type built struct {
				Ecli string `json:"ecli"`
				File string `json:"file"`
				URL  string `json:"url"`
			}
			manifestRow := func(p gaclient.Provvedimento, fname string) {
				_ = w.Write([]string{p.Ecli, p.Tipo, p.Sede, p.Sezione, strconv.Itoa(p.Anno), strconv.Itoa(p.Numero), p.Nrg, p.DataDeposito, p.URL, fname})
			}

			var summary []built
			var skipped, planned, reused, noText int
			for _, p := range items {
				fname := sanitizeFilename(provID(p)) + ".md"
				fpath := filepath.Join(out, fname)

				// --plan: list candidates only, no document fetch, no .md written.
				if plan {
					manifestRow(p, fname)
					summary = append(summary, built{Ecli: p.Ecli, File: fname, URL: p.URL})
					planned++
					continue
				}
				// --skip-existing: already-downloaded provvedimenti stay in the
				// manifest (from search metadata) but are not refetched.
				if skipExisting {
					if _, statErr := os.Stat(fpath); statErr == nil {
						manifestRow(p, fname)
						summary = append(summary, built{Ecli: p.Ecli, File: fname, URL: p.URL})
						skipped++
						continue
					}
				}

				// A provvedimento whose full text is already in the local store
				// (fetched by an earlier `get`, typically while deciding whether
				// it belonged in the corpus at all) is written from the store.
				// Re-fetching it would be a request to the portal for bytes we
				// already hold, and it is the whole difference between building
				// a curated corpus in one shot and paying for every document a
				// second time.
				var md string
				if p.FullText == "" {
					// The --ids path arrives with the stored text attached; the
					// search path carries metadata only, so look the text up.
					p.FullText = storedFullText(st, p)
				}
				if p.FullText != "" && !refresh {
					md = p.FullText
					reused++
					// The date is in the text ("Pubblicato il GG/MM/AAAA"); the
					// search results never carry it, so a corpus assembled from
					// reused text would leave the manifest column empty for the
					// very rows we already have in full.
					if p.DataDeposito == "" {
						p.DataDeposito = gaclient.ExtractDataDeposito(md)
					}
				} else {
					doc, ferr := c.Document(cmd.Context(), p)
					if ferr != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "salto %s: %v\n", provID(p), ferr)
						continue
					}
					if p.DataDeposito == "" {
						p.DataDeposito = gaclient.ExtractDataDeposito(doc.Raw)
					}
					md = doc.Raw
					if !doc.IsPDF {
						md = gaclient.HTMLToMarkdown(doc.Raw)
					}
					p.FullText = md
					if st != nil {
						persistProvvedimenti(st, []gaclient.Provvedimento{p})
					}
				}
				// The portal serves part of its rulings as PDF, which converts to
				// nothing. Writing the empty result would put a file in the corpus
				// that looks like a complete document saying nothing at all, and
				// the manifest would list it as successfully archived.
				if hasNoExtractableText(md) {
					fmt.Fprintf(cmd.ErrOrStderr(), "Attenzione: %s non ha testo estraibile (formato %s): scritta solo la scheda, l'originale e' su %s\n", noTextLabel(p), documentFormat(p), p.URL)
					md = noTextNotice(p)
					noText++
				}
				var header string
				if frontMatter {
					// I metadati di registro sono opt-in di `get --meta`: una
					// riga risolta dallo store puo' portarli, e senza questo il
					// corpus avrebbe schede diverse fra loro a seconda di quali
					// provvedimenti qualcuno aveva gia' letto con quel flag.
					// La riga nello store resta intatta: si azzera la copia.
					scheda := p
					scheda.Meta = nil
					header = gaclient.FrontMatter(scheda) + "\n"
				} else {
					header = fmt.Sprintf("# %s\n\n- Tipo: %s\n- Sede: %s %s\n- Data deposito: %s\n- NRG: %s\n- URL: %s\n\n---\n\n",
						provID(p), p.Tipo, p.Sede, p.Sezione, p.DataDeposito, p.Nrg, p.URL)
				}
				if werr := os.WriteFile(fpath, []byte(header+md+"\n"), 0o644); werr != nil {
					return werr
				}
				manifestRow(p, fname)
				summary = append(summary, built{Ecli: p.Ecli, File: fname, URL: p.URL})
			}
			w.Flush()
			if err := w.Error(); err != nil {
				return fmt.Errorf("scrittura manifest CSV: %w", err)
			}

			// A .md left in --out by an earlier, different selection is not
			// listed in the manifest just written, so nothing in the corpus
			// admits it exists: the response is coherent, the manifest is
			// coherent, only the folder is not. Someone citing from an orphan
			// cites a ruling this corpus deliberately excluded, and the
			// manifest — the corpus's only record of provenance — does not
			// mention it.
			//
			// Reported, never deleted: --skip-existing exists precisely to let
			// a corpus accumulate across runs, so removing unlisted files would
			// silently destroy what that flag is there to preserve.
			listed := make(map[string]bool, len(summary))
			for _, b := range summary {
				listed[b.File] = true
			}
			orphans := orphanFiles(out, listed)
			if len(orphans) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "Attenzione: in %s ci sono %d file .md non elencati nel manifest (residui di una selezione precedente): %s. Non sono stati toccati: rimuovili tu se non li vuoi nel corpus.\n",
					out, len(orphans), strings.Join(orphans, ", "))
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				switch {
				case plan:
					fmt.Fprintf(cmd.ErrOrStderr(), "Piano: %d candidate elencate in %s (nessun download).\n", planned, manifestPath)
				case skipped > 0:
					fmt.Fprintf(cmd.ErrOrStderr(), "Corpus aggiornato in %s: %d provvedimenti (%d già presenti, saltati), manifest %s.\n", out, len(summary), skipped, manifestPath)
				default:
					fmt.Fprintf(cmd.ErrOrStderr(), "Corpus creato in %s: %d provvedimenti (%d dal testo gia' in store, %d senza testo estraibile), manifest %s.\n", out, len(summary), reused, noText, manifestPath)
				}
			}
			result := map[string]any{
				"out": out, "manifest": manifestPath, "count": len(summary),
				"skipped": skipped, "reused": reused, "senza_testo": noText, "plan": plan,
				"orfani":       orphans,
				"generated_at": time.Now().UTC().Format(time.RFC3339), "items": summary,
			}
			data, _ := json.Marshal(result)
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	addSearchFlags(cmd, &f)
	cmd.Flags().StringVar(&out, "out", "", "Cartella di destinazione del corpus (richiesto).")
	cmd.Flags().StringVar(&ids, "ids", "", "Lista di ECLI/idprovv separati da virgola: il corpus e' esattamente questa selezione, senza rifare una ricerca. Alternativo ai criteri di ricerca.")
	cmd.Flags().BoolVar(&skipExisting, "skip-existing", false, "Non riscaricare i provvedimenti il cui .md è già presente in --out.")
	cmd.Flags().BoolVar(&plan, "plan", false, "Non scaricare: elenca solo le candidate nel manifest, per revisione.")
	cmd.Flags().BoolVar(&refresh, "refresh", false, "Riscarica dal portale anche i testi gia' presenti nello store locale (per default vengono riusati).")
	cmd.Flags().BoolVar(&frontMatter, "front-matter", false, "Usa il blocco YAML (come `get --front-matter`) come intestazione dei .md, invece dell'header Markdown predefinito.")
	return cmd
}

// orphanFiles returns the .md files present in dir that this run did not list.
func orphanFiles(dir string, listed map[string]bool) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || listed[e.Name()] {
			continue
		}
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

// splitIDs parses the comma-separated --ids list, tolerating spaces and
// newlines so a list pasted from a previous result works as-is.
func splitIDs(raw string) []string {
	var out []string
	for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' }) {
		if v := strings.TrimSpace(part); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func sanitizeFilename(s string) string {
	s = strings.ReplaceAll(s, ":", "_")
	s = reUnsafe.ReplaceAllString(s, "_")
	s = strings.Trim(s, "_")
	if s == "" {
		s = "provvedimento"
	}
	return s
}
