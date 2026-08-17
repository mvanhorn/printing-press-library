// pp:client-call
// pp:data-source live
// Novel feature: save a query under a name and, on each run, show only the
// provvedimenti that are new since the last run. The web form is stateless and
// cannot do this; we keep the seen-set in the local store.
package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/giustizia-amministrativa/internal/gaclient"
)

// watchState persists a saved query and the set of already-seen provvedimenti.
type watchState struct {
	Name     string                 `json:"name"`
	Opts     gaclient.SearchOptions `json:"opts"`
	Seen     []string               `json:"seen"`
	LastRun  string                 `json:"last_run"`
	LastNew  int                    `json:"last_new"`
	RunCount int                    `json:"run_count"`
}

func newNovelWatchRunCmd(flags *rootFlags) *cobra.Command {
	var f searchFlags
	cmd := &cobra.Command{
		Use:   "run <nome>",
		Short: "Salva una ricerca con un nome e mostra solo i provvedimenti nuovi dall'ultima esecuzione.",
		Long: "Alla prima esecuzione salva la query (dai flag) sotto <nome> e registra i risultati come 'visti'.\n" +
			"Alle esecuzioni successive ripete la query e mostra solo i provvedimenti comparsi nel frattempo.",
		Example: strings.Trim(`
  giustizia-amministrativa-pp-cli watch run appalti-lazio --testo appalto --sede roma --tipo sentenza --limit 50
  giustizia-amministrativa-pp-cli watch run appalti-lazio --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if gaSkip(flags) {
				return nil
			}
			name := args[0]
			st, err := openGAStore(cmd.Context())
			if err != nil {
				return err
			}
			defer st.Close()

			var ws watchState
			if raw, gerr := st.Get("watches", name); gerr == nil && len(raw) > 0 {
				_ = json.Unmarshal(raw, &ws)
			}
			// First creation requires explicit search criteria; later runs reuse the
			// saved query unless new criteria are provided.
			newOpts := f.opts("")
			if ws.RunCount == 0 && !hasAnySearchInput(newOpts) {
				return fmt.Errorf("per creare la watch %q servono criteri di ricerca (es. --testo appalto --sede roma)", name)
			}
			ws.Name = name
			if hasAnySearchInput(newOpts) {
				// --limit defaults to 0 (unset). When the user re-runs a watch
				// with new criteria but omits --limit, keep the watch's stored
				// limit instead of silently resetting it; only fall back to 50
				// for a brand-new watch with no stored limit.
				if newOpts.Limit == 0 {
					if ws.Opts.Limit > 0 {
						newOpts.Limit = ws.Opts.Limit
					} else {
						newOpts.Limit = 50
					}
				}
				ws.Opts = newOpts
			}

			c := gaclient.New()
			res, err := c.Search(cmd.Context(), ws.Opts)
			if err != nil {
				return classifyAPIError(err, flags)
			}
			persistProvvedimenti(st, res.Items)

			seen := map[string]bool{}
			for _, id := range ws.Seen {
				seen[id] = true
			}
			fresh := []gaclient.Provvedimento{}
			for _, p := range res.Items {
				id := provID(p)
				if !seen[id] {
					fresh = append(fresh, p)
					seen[id] = true
					ws.Seen = append(ws.Seen, id)
				}
			}
			// Bound the seen-set so a broad query re-run for months doesn't grow
			// the persisted state without limit. Keep the most recent IDs.
			const maxSeen = 5000
			if len(ws.Seen) > maxSeen {
				ws.Seen = ws.Seen[len(ws.Seen)-maxSeen:]
			}
			ws.LastRun = time.Now().UTC().Format(time.RFC3339)
			ws.LastNew = len(fresh)
			ws.RunCount++
			if data, merr := json.Marshal(ws); merr == nil {
				_ = st.Upsert("watches", name, data)
			}

			if wantsHumanTable(cmd.OutOrStdout(), flags) {
				if ws.RunCount == 1 {
					fmt.Fprintf(cmd.ErrOrStderr(), "Watch %q creata: %d provvedimenti registrati come baseline.\n", name, len(fresh))
				} else {
					fmt.Fprintf(cmd.ErrOrStderr(), "Watch %q: %d nuovi provvedimenti dall'ultima esecuzione.\n", name, len(fresh))
				}
			}
			out, _ := json.Marshal(fresh)
			return printOutputWithFlags(cmd.OutOrStdout(), out, flags)
		},
	}
	addSearchFlags(cmd, &f)
	return cmd
}

func hasAnySearchInput(o gaclient.SearchOptions) bool {
	return o.Testo != "" || o.All != "" || o.Any != "" || o.Not != "" || o.Phrase != "" ||
		o.Tipo != "" || o.Sede != "" || o.Anno != 0 || o.Numero != 0 || o.Nrg != 0
}
