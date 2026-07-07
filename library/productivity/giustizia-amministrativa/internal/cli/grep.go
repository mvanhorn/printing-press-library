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

func newNovelGrepCmd(flags *rootFlags) *cobra.Command {
	var ignoreCase bool
	var pattern string
	cmd := &cobra.Command{
		Use:   "grep [-e <regex>]",
		Short: "Cerca con regex nei testi integrali scaricati localmente (non solo negli snippet).",
		Long: "Esegue una ricerca con espressione regolare sul testo integrale dei provvedimenti\n" +
			"presenti nello store locale (scaricati con `get` o `corpus build`). Funziona offline.",
		Example: strings.Trim(`
  giustizia-amministrativa-pp-cli grep -e "soccorso istruttorio"
  giustizia-amministrativa-pp-cli grep -i -e "clausola\\s+sociale" --json --select ecli,url`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true"},
		Args:        cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if pattern == "" {
				return cmd.Help()
			}
			if gaSkip(flags) {
				return nil
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
			matches := []gaclient.Provvedimento{}
			scanned := 0
			for _, r := range rows {
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
				if re.MatchString(hay) {
					matches = append(matches, p)
				}
			}
			if scanned == 0 && wantsHumanTable(cmd.OutOrStdout(), flags) {
				fmt.Fprintln(cmd.ErrOrStderr(), "Nota: nessun testo integrale nello store. Usa `get <id>` o `corpus build` per scaricarli, poi rilancia grep.")
			}
			data, _ := json.Marshal(matches)
			return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
		},
	}
	cmd.Flags().StringVarP(&pattern, "pattern", "e", "", "Espressione regolare da cercare nei testi integrali (richiesto).")
	cmd.Flags().BoolVarP(&ignoreCase, "ignore-case", "i", false, "Ricerca case-insensitive.")
	return cmd
}
