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
// previously stored full_text when the incoming row doesn't carry one.
func persistProvvedimenti(st *store.Store, items []gaclient.Provvedimento) {
	for _, p := range items {
		id := provID(p)
		if id == "" {
			continue
		}
		if p.FullText == "" {
			if existing, err := st.Get("provvedimenti", id); err == nil && len(existing) > 0 {
				var prev gaclient.Provvedimento
				if json.Unmarshal(existing, &prev) == nil && prev.FullText != "" {
					p.FullText = prev.FullText
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
		return nil
	}
	c := gaclient.New()
	res, err := c.Search(cmd.Context(), opts)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	// Best-effort persistence (offline search, watch, grep, stats build on this).
	if st, serr := openGAStore(cmd.Context()); serr == nil {
		persistProvvedimenti(st, res.Items)
		_ = st.Close()
	}
	if wantsHumanTable(cmd.OutOrStdout(), flags) && res.Total > 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "Trovati %d risultati (mostrati %d).\n", res.Total, len(res.Items))
	}
	data, err := json.Marshal(res.Items)
	if err != nil {
		return err
	}
	return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
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
// requested format (md, text, html, json).
func runGAGet(cmd *cobra.Command, flags *rootFlags, id, format, sede, nrg, file string) error {
	if gaSkip(flags) {
		return nil
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

	docHTML, err := c.FullText(cmd.Context(), p)
	if err != nil {
		return classifyAPIError(err, flags)
	}
	if p.DataDeposito == "" {
		p.DataDeposito = gaclient.ExtractDataDeposito(docHTML)
	}
	markdown := gaclient.HTMLToMarkdown(docHTML)
	p.FullText = markdown

	// Persist the fetched full text for offline grep/corpus.
	if id != "" {
		if st, serr := openGAStore(cmd.Context()); serr == nil {
			persistProvvedimenti(st, []gaclient.Provvedimento{p})
			_ = st.Close()
		}
	}

	if flags.asJSON {
		data, _ := json.Marshal(p)
		return printOutputWithFlags(cmd.OutOrStdout(), data, flags)
	}
	switch strings.ToLower(format) {
	case "", "md", "markdown":
		fmt.Fprintln(cmd.OutOrStdout(), markdown)
	case "text", "txt":
		fmt.Fprintln(cmd.OutOrStdout(), gaclient.HTMLToText(docHTML))
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
