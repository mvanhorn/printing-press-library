// Copyright 2026 Luigi Pastore and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/normattiva/internal/client"
	"github.com/mvanhorn/printing-press-library/library/productivity/normattiva/internal/norma"
	"github.com/mvanhorn/printing-press-library/library/productivity/normattiva/internal/store"

	"github.com/spf13/cobra"
)

const normaCLIName = "normattiva-pp-cli"

// articleResource is the local-store resource type for fetched article text,
// keyed by the exact URN that was requested.
const articleResource = "articoli"

// articleView is the normalized result of one dettaglio-atto read.
type articleView struct {
	URN           string `json:"urn"`
	URL           string `json:"url"`
	Titolo        string `json:"titolo"`
	Sottotitolo   string `json:"sottotitolo,omitempty"`
	Tipo          string `json:"tipo,omitempty"`
	Articolo      string `json:"articolo,omitempty"`
	Rubrica       string `json:"intestazione,omitempty"`
	InizioVigenza string `json:"inizio_vigenza,omitempty"`
	FineVigenza   string `json:"fine_vigenza,omitempty"`
	InVigore      bool   `json:"in_vigore"`
	Abrogato      bool   `json:"abrogato"`
	Testo         string `json:"testo"`
	HTML          string `json:"html,omitempty"`
	Hash          string `json:"hash"`
	Fonte         string `json:"fonte"` // "live" or "local"
	FetchedAt     string `json:"fetched_at,omitempty"`
}

type dettaglioEnvelope struct {
	Code    any    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Atto *struct {
			Titolo                       string `json:"titolo"`
			SottoTitolo                  string `json:"sottoTitolo"`
			ArticoloHTML                 string `json:"articoloHtml"`
			TipoProvvedimentoDescrizione string `json:"tipoProvvedimentoDescrizione"`
			ArticoloDataInizioVigenza    string `json:"articoloDataInizioVigenza"`
			ArticoloDataFineVigenza      string `json:"articoloDataFineVigenza"`
		} `json:"atto"`
	} `json:"data"`
}

// errNotFound marks a URN that Normattiva does not know (or an article that
// does not exist at that date).
var errNotFound = errors.New("not found on Normattiva")

func parseDettaglio(raw json.RawMessage, u norma.URN) (articleView, error) {
	var env dettaglioEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		return articleView{}, fmt.Errorf("decoding Normattiva response: %w", err)
	}
	if env.Data.Atto == nil {
		msg := env.Message
		if msg == "" {
			msg = "empty response"
		}
		return articleView{}, fmt.Errorf("%w: %s (%s)", errNotFound, u.String(), msg)
	}
	a := env.Data.Atto
	v := articleView{
		URN:           u.String(),
		URL:           u.WebURL(),
		Titolo:        strings.TrimSpace(a.Titolo),
		Sottotitolo:   strings.Join(strings.Fields(a.SottoTitolo), " "),
		Tipo:          a.TipoProvvedimentoDescrizione,
		Articolo:      norma.ArticleHeading(a.ArticoloHTML),
		InizioVigenza: norma.FormatVigenzaDate(a.ArticoloDataInizioVigenza),
		FineVigenza:   norma.FormatVigenzaDate(a.ArticoloDataFineVigenza),
		Abrogato:      norma.IsAbrogated(a.ArticoloHTML),
		HTML:          a.ArticoloHTML,
	}
	// in_vigore answers "was this text in force on the requested date?"
	// (today when the URN is not pinned to a date).
	_, open, _ := norma.ParseVigenzaDate(a.ArticoloDataFineVigenza)
	switch {
	case v.Abrogato:
		v.InVigore = false
	case u.Vigenza != "":
		v.InVigore = (v.InizioVigenza == "" || v.InizioVigenza <= u.Vigenza) && (open || v.FineVigenza == "" || v.FineVigenza >= u.Vigenza)
	default:
		v.InVigore = open || v.FineVigenza == "" || v.FineVigenza >= timeNow().Format("2006-01-02")
	}
	v.Testo = norma.RenderArticle(a.ArticoloHTML, norma.RenderOptions{})
	if m := strings.Index(v.Testo, "\n"); m > 0 {
		rest := strings.TrimSpace(v.Testo[m:])
		if strings.HasPrefix(rest, "(") {
			if e := strings.Index(rest, "\n"); e > 0 {
				v.Rubrica = norma.StripAmendmentMarkers(rest[:e])
			} else {
				v.Rubrica = norma.StripAmendmentMarkers(rest)
			}
		}
	}
	if v.Articolo == "" {
		// Code annexes carry the heading as plain text ("Art. 2043.").
		first := v.Testo
		if i := strings.Index(first, "\n"); i >= 0 {
			first = first[:i]
		}
		if strings.HasPrefix(first, "Art") && len(first) < 30 {
			v.Articolo = strings.TrimSuffix(strings.TrimSpace(first), ".")
		}
	}
	sum := sha256.Sum256([]byte(v.Testo))
	v.Hash = hex.EncodeToString(sum[:8])
	return v, nil
}

// normaStore opens the local store, creating it when needed. Callers close it.
func normaStore(ctx context.Context) (*store.Store, error) {
	db, err := store.OpenWithContext(ctx, defaultDBPath(normaCLIName))
	if err != nil {
		return nil, err
	}
	if err := ensureNormaTables(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func cacheArticle(ctx context.Context, v articleView) {
	db, err := normaStore(ctx)
	if err != nil {
		return
	}
	defer db.Close()
	v.Fonte = ""
	data, err := json.Marshal(v)
	if err != nil {
		return
	}
	if err := db.Upsert(articleResource, v.URN, data); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not cache %s: %v\n", v.URN, err)
	}
}

func cachedArticle(ctx context.Context, urn string) (articleView, bool) {
	path := defaultDBPath(normaCLIName)
	if _, err := os.Stat(path); err != nil {
		return articleView{}, false
	}
	db, err := store.OpenWithContext(ctx, path)
	if err != nil {
		return articleView{}, false
	}
	defer db.Close()
	raw, err := db.Get(articleResource, urn)
	if err != nil || len(raw) == 0 {
		return articleView{}, false
	}
	var v articleView
	if json.Unmarshal(raw, &v) != nil || v.URN == "" {
		return articleView{}, false
	}
	v.Fonte = "local"
	return v, true
}

// fetchArticle reads one URN, honoring --data-source: "local" reads only the
// cache, "live" only the API, "auto" goes live and falls back to the cache on
// network failure. Pinned versions (!vig= / @originale) never change, so auto
// serves them from the cache when present.
func fetchArticle(ctx context.Context, c *client.Client, flags *rootFlags, u norma.URN) (articleView, error) {
	key := u.String()
	pinned := u.Vigenza != "" || u.Originale
	switch flags.dataSource {
	case "local":
		if v, ok := cachedArticle(ctx, key); ok {
			return v, nil
		}
		return articleView{}, fmt.Errorf("%w in the local cache: %s (read it once without --data-source local)", errNotFound, key)
	case "", "auto":
		if pinned {
			if v, ok := cachedArticle(ctx, key); ok {
				return v, nil
			}
		}
	}
	raw, _, err := c.PostQueryWithParams(ctx, "/atto/dettaglio-atto-urn", nil, map[string]any{"urn": key})
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return articleView{}, fmt.Errorf("%w: %s", errNotFound, key)
		}
		if flags.dataSource != "live" && isNetworkError(err) {
			if v, ok := cachedArticle(ctx, key); ok {
				fmt.Fprintf(os.Stderr, "warning: Normattiva unreachable, using cached copy of %s\n", key)
				return v, nil
			}
		}
		return articleView{}, err
	}
	v, err := parseDettaglio(raw, u)
	if err != nil {
		return articleView{}, err
	}
	v.Fonte = "live"
	v.FetchedAt = time.Now().UTC().Format(time.RFC3339)
	cacheArticle(ctx, v)
	return v, nil
}

// fetchByGazzetta reads the current text of an act from its Gazzetta
// coordinates. This path never supports historical versions.
func fetchByGazzetta(ctx context.Context, c *client.Client, codice, dataGU string, articolo int) (articleView, error) {
	body := map[string]any{"codiceRedazionale": codice, "dataGU": dataGU}
	if articolo > 0 {
		body["idArticolo"] = articolo
	}
	raw, _, err := c.PostQueryWithParams(ctx, "/atto/dettaglio-atto", nil, body)
	if err != nil {
		var apiErr *client.APIError
		if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
			return articleView{}, fmt.Errorf("%w: codice redazionale %s del %s", errNotFound, codice, dataGU)
		}
		return articleView{}, err
	}
	v, err := parseDettaglio(raw, norma.URN{})
	if err != nil {
		return articleView{}, err
	}
	v.URN = ""
	v.URL = "https://www.normattiva.it/atto/caricaDettaglioAtto?" + url.Values{"atto.dataPubblicazioneGazzetta": {dataGU}, "atto.codiceRedazionale": {codice}}.Encode()
	v.Fonte = "live"
	return v, nil
}

// resolveReference turns a user act reference plus optional --articolo into a URN.
func resolveReference(ref, articolo string) (norma.URN, norma.Citation, error) {
	cit, err := norma.ParseCitation(ref, articolo)
	if err != nil {
		return norma.URN{}, norma.Citation{}, err
	}
	u := cit.URN()
	if norma.IsURN(ref) {
		if parsed, perr := norma.ParseURN(ref); perr == nil {
			// Keep version pins and annex typed in the URN itself.
			u.Vigenza, u.Originale = parsed.Vigenza, parsed.Originale
			if parsed.Allegato != "" {
				u.Allegato = parsed.Allegato
			}
		}
	}
	return u, cit, nil
}

// applyVersion pins u to --al/--vigenza or --originale.
func applyVersion(u norma.URN, al string, originale bool) (norma.URN, error) {
	if originale && al != "" {
		return u, usageErr(fmt.Errorf("use either --originale or --al, not both"))
	}
	if originale {
		return u.WithOriginale(), nil
	}
	if al != "" {
		d, err := norma.ParseDate(al, time.Now())
		if err != nil {
			return u, usageErr(err)
		}
		return u.WithVigenza(d), nil
	}
	return u, nil
}

// articleErr maps fetch errors onto typed exit codes.
func articleErr(cmd *cobra.Command, err error, flags *rootFlags) error {
	if errors.Is(err, errNotFound) {
		return notFoundErr(err)
	}
	var ce *cliError
	if errors.As(err, &ce) {
		return err
	}
	return classifyAPIError(cmd.OutOrStdout(), err, flags)
}

func renderOpts(format string) norma.RenderOptions {
	return norma.RenderOptions{Markdown: format == "markdown" || format == "md", Links: format == "markdown" || format == "md"}
}

func validateFormat(format string) error {
	switch format {
	case "text", "markdown", "md", "html":
		return nil
	}
	return usageErr(fmt.Errorf("--formato must be text, markdown or html (got %q)", format))
}

// articleBody returns the article in the requested human format.
func articleBody(v articleView, format string) string {
	if format == "html" {
		return v.HTML
	}
	if format == "text" {
		return v.Testo
	}
	return norma.RenderArticle(v.HTML, renderOpts(format))
}

func vigenzaLabel(v articleView) string {
	from := v.InizioVigenza
	if from == "" {
		from = "?"
	}
	to := v.FineVigenza
	if to == "" {
		to = "oggi"
	}
	state := "in vigore"
	switch {
	case v.Abrogato:
		state = "abrogato"
	case !v.InVigore:
		state = "non più in vigore"
	}
	return fmt.Sprintf("vigenza %s → %s (%s)", from, to, state)
}

var timeNow = time.Now

func parseDay(d string) (time.Time, error) { return time.Parse("2006-01-02", d) }

// maxInputBytes caps text files read by verifica and fascicolo.
const maxInputBytes = 10 << 20

func readLimited(path string) ([]byte, error) {
	// #nosec G304 -- the path is a file the user named on the command line to read.
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxInputBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxInputBytes {
		return nil, fmt.Errorf("%s is larger than %d MB", path, maxInputBytes>>20)
	}
	return data, nil
}

// writeFileAtomic writes through a temp file in the same directory and
// renames it into place, so a failed write never leaves a truncated file.
func writeFileAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.part")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(name)
		return err
	}
	// #nosec G302 -- exported public law text the user asked for; normal document permissions.
	if err := os.Chmod(name, 0o644); err != nil {
		_ = os.Remove(name)
		return err
	}
	return os.Rename(name, path)
}
