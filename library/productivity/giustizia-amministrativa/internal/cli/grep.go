// pp:client-call
// pp:data-source local
// Novel feature: regex search over the full texts already downloaded into the
// local store (not just the search snippets). Store-backed, fully offline.
package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/giustizia-amministrativa/internal/gaclient"
)

const (
	// maxEstratti caps how many excerpts a single provvedimento contributes:
	// enough to judge relevance, not so many that one document dominates.
	maxEstratti = 3
	// contextRunes is the amount of text kept on either side of a match.
	contextRunes = 160
)

// excerptsAround renders up to max excerpts of the text surrounding the given
// match positions, trimmed to rune boundaries and collapsed onto one line.
func excerptsAround(text string, locs [][]int, max, around int) []string {
	runes := []rune(text)
	// Byte offsets from the regexp need mapping onto rune positions.
	byteToRune := make(map[int]int, len(runes)+1)
	ri := 0
	for bi := range text {
		byteToRune[bi] = ri
		ri++
	}
	byteToRune[len(text)] = ri

	out := make([]string, 0, max)
	for _, loc := range locs {
		if len(out) >= max {
			break
		}
		start, ok1 := byteToRune[loc[0]]
		end, ok2 := byteToRune[loc[1]]
		if !ok1 || !ok2 {
			continue
		}
		from := start - around
		if from < 0 {
			from = 0
		}
		to := end + around
		if to > len(runes) {
			to = len(runes)
		}
		frag := strings.Join(strings.Fields(string(runes[from:to])), " ")
		if from > 0 {
			frag = "..." + frag
		}
		if to < len(runes) {
			frag += "..."
		}
		out = append(out, frag)
	}
	return out
}

func newNovelGrepCmd(flags *rootFlags) *cobra.Command {
	var ignoreCase bool
	var pattern string
	var limit int
	cmd := &cobra.Command{
		Use:   "grep [-e <regex>]",
		Short: "Cerca con regex nei testi integrali scaricati localmente (non solo negli snippet).",
		Long: "Esegue una ricerca con espressione regolare sul testo integrale dei provvedimenti\n" +
			"presenti nello store locale (scaricati con `get` o `corpus build`). Funziona offline.\n" +
			"Restituisce, per ogni provvedimento, il numero di occorrenze e gli estratti di testo\n" +
			"attorno alle prime corrispondenze, con ECLI e URL per leggere il resto: non l'intero\n" +
			"testo integrale, che su piu' risultati supererebbe quanto un client MCP puo' ricevere.",
		Example: strings.Trim(`
  giustizia-amministrativa-pp-cli grep -e "soccorso istruttorio"
  giustizia-amministrativa-pp-cli grep -i -e "clausola\\s+sociale" --json --select ecli,url
  giustizia-amministrativa-pp-cli grep -e "principio di proporzionalit" --limit 5`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if pattern == "" {
				return cmd.Help()
			}
			if gaSkip(flags) {
				return emitSkip(cmd, flags)
			}
			pat := pattern
			if ignoreCase {
				pat = "(?i)" + pat
			}
			re, err := regexp.Compile(pat)
			if err != nil {
				return fmt.Errorf("regex non valida %q: %w", pattern, err)
			}
			st, err := openGAStore(cmd.Context())
			if err != nil {
				return err
			}
			defer st.Close()
			rows, err := st.List("provvedimenti", 1000000)
			if err != nil {
				return err
			}
			// A grep returns where the pattern matched, not the documents it
			// matched in. Returning whole Provvedimento rows meant carrying the
			// entire full text of every hit — tens to hundreds of KB each — so a
			// handful of matches blew past what an MCP host will accept and the
			// caller got nothing at all. Emit the surrounding excerpt instead,
			// with the id and URL to read the rest.
			type occorrenza struct {
				Ecli     string   `json:"ecli"`
				Tipo     string   `json:"tipo"`
				Sede     string   `json:"sede"`
				Anno     int      `json:"anno"`
				Numero   int      `json:"numero"`
				URL      string   `json:"url"`
				Match    int      `json:"occorrenze"`
				Estratti []string `json:"estratti"`
			}
			matches := []occorrenza{}
			scanned, truncated := 0, 0
			for _, r := range rows {
				if limit > 0 && len(matches) >= limit {
					truncated++
					continue
				}
				var p gaclient.Provvedimento
				if json.Unmarshal(r, &p) != nil {
					continue
				}
				hay := p.FullText
				if hay == "" {
					hay = p.Snippet
				} else {
					scanned++
				}
				locs := re.FindAllStringIndex(hay, -1)
				if len(locs) == 0 {
					continue
				}
				matches = append(matches, occorrenza{
					Ecli: p.Ecli, Tipo: p.Tipo, Sede: p.Sede, Anno: p.Anno,
					Numero: p.Numero, URL: p.URL, Match: len(locs),
					Estratti: excerptsAround(hay, locs, maxEstratti, contextRunes),
				})
			}
			if scanned == 0 && wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintln(cmd.ErrOrStderr(), "Nota: nessun testo integrale nello store. Usa `get <id>` o `corpus build` per scaricarli, poi rilancia grep.")
			}
			if truncated > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "Attenzione: fermato a --limit %d provvedimenti; altri %d contengono il pattern e non sono elencati. Alza --limit o restringi la regex.\n", limit, truncated)
			}
			data, _ := json.Marshal(matches)
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVarP(&pattern, "pattern", "e", "", "Espressione regolare da cercare nei testi integrali (richiesto).")
	cmd.Flags().BoolVarP(&ignoreCase, "ignore-case", "i", false, "Ricerca case-insensitive.")
	cmd.Flags().IntVar(&limit, "limit", 20, "Max provvedimenti da elencare (0 = nessun limite).")
	return cmd
}
