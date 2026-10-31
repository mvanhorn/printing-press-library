// Copyright 2026 Luigi Pastore and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/mvanhorn/printing-press-library/library/productivity/normattiva/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/normattiva/internal/norma"

	"github.com/spf13/cobra"
)

var exportFormats = []string{"AKN", "XML", "JSON", "HTML", "PDF", "EPUB", "RTF", "URI"}

func checkFormat(f string) (string, error) {
	u := strings.ToUpper(strings.TrimSpace(f))
	for _, ok := range exportFormats {
		if u == ok {
			return u, nil
		}
	}
	return "", usageErr(fmt.Errorf("--formato must be one of %s (got %q)", strings.Join(exportFormats, ", "), f))
}

func checkModo(m string) (string, error) {
	switch strings.ToUpper(strings.TrimSpace(m)) {
	case "O", "ORIGINALE", "ORIGINARIO":
		return "O", nil
	case "V", "VIGENTE":
		return "V", nil
	case "M", "MULTIVIGENTE":
		return "M", nil
	}
	return "", usageErr(fmt.Errorf("--modo must be O (originale), V (vigente) or M (multivigente) (got %q)", m))
}

// rawHTTP performs a request against the Normattiva base URL without the
// generated client's JSON handling: the export endpoints answer with plain
// text tokens, 303 redirects and ZIP bodies.
type rawHTTP struct {
	base string
	hc   *http.Client
}

func newRawHTTP(flags *rootFlags, timeout time.Duration, followRedirects bool) (*rawHTTP, error) {
	c, err := flags.newClient()
	if err != nil {
		return nil, err
	}
	hc := &http.Client{Timeout: timeout}
	if !followRedirects {
		// Export polling reads the 303 + x-ipzs-location itself.
		hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	}
	return &rawHTTP{base: strings.TrimRight(c.RequestBaseURL(), "/"), hc: hc}, nil
}

func (r *rawHTTP) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		rd = bytes.NewReader(b)
	}
	target := path
	if !strings.HasPrefix(path, "http") {
		target = r.base + path
	}
	req, err := http.NewRequestWithContext(ctx, method, target, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "normattiva-pp-cli/"+version)
	req.Header.Set("Accept", "application/json, application/zip, text/plain, */*")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return r.hc.Do(req)
}

func downloadTo(resp *http.Response, path string) (int64, error) {
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return 0, fmt.Errorf("download failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*.part")
	if err != nil {
		return 0, err
	}
	tmp := f.Name()
	n, err := io.Copy(f, resp.Body)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		// #nosec G302 -- downloaded public law collection; normal document permissions.
		err = os.Chmod(tmp, 0o644)
	}
	if err != nil {
		_ = os.Remove(tmp)
		return 0, err
	}
	return n, os.Rename(tmp, path)
}

type collezioneRow struct {
	Nome        string `json:"nome"`
	Modo        string `json:"modo"`
	Descrizione string `json:"descrizione_modo"`
	Atti        int    `json:"atti"`
	Creata      string `json:"creata_il"`
}

type scaricaView struct {
	Collezione  string          `json:"collezione"`
	Formato     string          `json:"formato"`
	Modo        string          `json:"modo"`
	URL         string          `json:"url"`
	Disponibili []collezioneRow `json:"disponibili,omitempty"`
	File        string          `json:"file,omitempty"`
	Byte        int64           `json:"byte,omitempty"`
	Scaricato   bool            `json:"scaricato"`
}

func newScaricaCmd(flags *rootFlags) *cobra.Command {
	var nome, formato, modo, output string
	cmd := &cobra.Command{
		Use:   "scarica",
		Short: "Download a prebuilt Normattiva collection (e.g. Codici) as a ZIP in AKN, JSON, XML, PDF and more",
		Long: strings.Trim(`
Download one of Normattiva's prebuilt collections. Without --output the
command only shows which versions of the collection exist and the URL it
would download, so you can check size before fetching. Collection names
come from 'collezioni'.`, "\n"),
		Example: strings.Trim(`
  normattiva-pp-cli scarica --nome Codici --formato JSON --modo V
  normattiva-pp-cli scarica --nome Codici --formato AKN --modo V --output codici-akn.zip`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "mcp:write-flags": "output", "pp:data-source": "live", "pp:happy-args": "--nome=Codici;--formato=JSON;--modo=V"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "scarica")
			}
			if nome == "" {
				_ = cmd.Usage()
				return usageErr(errors.New("--nome is required (see 'collezioni')"))
			}
			f, err := checkFormat(formato)
			if err != nil {
				return err
			}
			m, err := checkModo(modo)
			if err != nil {
				return err
			}
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			q := url.Values{"nome": {nome}, "formato": {f}, "formatoRichiesta": {m}}
			view := scaricaView{Collezione: nome, Formato: f, Modo: m, URL: strings.TrimRight(c.RequestBaseURL(), "/") + "/collections/download/collection-preconfezionata?" + q.Encode()}
			if output == "" {
				ctx, cancel := boundCtx(cmd.Context(), flags)
				defer cancel()
				raw, err := c.Get(ctx, "/collections/collection-predefinite", nil)
				if err != nil {
					return classifyAPIError(cmd.OutOrStdout(), err, flags)
				}
				var all []struct {
					Nome   string `json:"nomeCollezione"`
					Modo   string `json:"formatoCollezione"`
					Desc   string `json:"descrizioneFormatoCollezione"`
					Atti   int    `json:"numeroAtti"`
					Creata string `json:"dataCreazione"`
				}
				if err := json.Unmarshal(raw, &all); err != nil {
					return fmt.Errorf("decoding collections: %w", err)
				}
				view.Disponibili = make([]collezioneRow, 0)
				for _, a := range all {
					if strings.EqualFold(a.Nome, nome) {
						view.Disponibili = append(view.Disponibili, collezioneRow{a.Nome, a.Modo, a.Desc, a.Atti, a.Creata})
					}
				}
				if len(view.Disponibili) == 0 {
					return notFoundErr(fmt.Errorf("no prebuilt collection named %q; run 'collezioni' for the names", nome))
				}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				for _, d := range view.Disponibili {
					fmt.Fprintf(cmd.OutOrStdout(), "%s  %s (%s)  %d atti  creata il %s\n", cliutil.ScrubTerminal(d.Nome), cliutil.ScrubTerminal(d.Modo), cliutil.ScrubTerminal(d.Descrizione), d.Atti, cliutil.ScrubTerminal(d.Creata))
				}
				fmt.Fprintf(cmd.OutOrStdout(), "\nadd --output <file.zip> to download %s\n", view.URL)
				return nil
			}
			if cliutil.IsAnyHarness() {
				return writeHarnessRefusal(cmd.OutOrStdout(), flags, "download collection")
			}
			rh, err := newRawHTTP(flags, 30*time.Minute, true)
			if err != nil {
				return err
			}
			resp, err := rh.do(cmd.Context(), http.MethodGet, view.URL, nil)
			if err != nil {
				return apiErr(err)
			}
			n, err := downloadTo(resp, output)
			if err != nil {
				return apiErr(err)
			}
			view.File, view.Byte, view.Scaricato = output, n, true
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "saved %s (%d bytes)\n", output, n)
			return nil
		},
	}
	cmd.Flags().StringVar(&nome, "nome", "", "Collection name, e.g. Codici (see 'collezioni')")
	cmd.Flags().StringVar(&formato, "formato", "AKN", "File format: AKN, XML, JSON, HTML, PDF, EPUB, RTF or URI")
	cmd.Flags().StringVar(&modo, "modo", "V", "Version: O originale, V vigente, M multivigente")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Write the ZIP to this file (without it, only show what is available)")
	return cmd
}

type esportaView struct {
	Token     string         `json:"token,omitempty"`
	Stato     int            `json:"stato"`
	Descr     string         `json:"descrizione_stato,omitempty"`
	Richiesta map[string]any `json:"richiesta"`
	File      string         `json:"file,omitempty"`
	Byte      int64          `json:"byte,omitempty"`
	Avviata   bool           `json:"avviata"`
	Nota      string         `json:"nota,omitempty"`
}

func newEsportaCmd(flags *rootFlags) *cobra.Command {
	var testo, titolo, tipo, formato, modo, vigenteAl, emanatoDal, emanatoAl, output, token string
	var anno, numero int
	var avvia, attendi bool
	var pollEveryS, waitMaxS string
	cmd := &cobra.Command{
		Use:   "esporta",
		Short: "Export every act matching a search as one ZIP (asynchronous Normattiva export)",
		Long: strings.Trim(`
Run Normattiva's asynchronous export: submit the search, confirm it, poll
until the archive is ready, then download it. Exports are queued server-side
work, so the command only prints the request unless you pass --avvia.
Resume a running export with --token.`, "\n"),
		Example: strings.Trim(`
  normattiva-pp-cli esporta --tipo "DECRETO LEGISLATIVO" --anno 2023 --formato JSON
  normattiva-pp-cli esporta --tipo LEGGE --anno 1990 --numero 241 --formato AKN --avvia --attendi --output l241.zip
  normattiva-pp-cli esporta --token f1a2e1f8-95bf-434b-9575-af6bba94212d --attendi --output export.zip`, "\n"),
		Annotations: map[string]string{"pp:data-source": "live", "mcp:write-flags": "output"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "esporta")
			}
			f, err := checkFormat(formato)
			if err != nil {
				return err
			}
			m, err := checkModo(modo)
			if err != nil {
				return err
			}
			params := map[string]any{"limitaAnniVigenza": false}
			set := func(k string, v any) {
				switch x := v.(type) {
				case string:
					if x != "" {
						params[k] = x
					}
				case int:
					if x > 0 {
						params[k] = x
					}
				}
			}
			set("testoRicerca", testo)
			set("titoloRicerca", titolo)
			if tipo != "" {
				set("denominazioneAtto", normalizeTipoLabel(tipo))
			}
			set("annoProvvedimento", anno)
			set("numeroProvvedimento", numero)
			for _, d := range []struct{ flag, val, key string }{{"vigente-al", vigenteAl, "dataVigenza"}, {"emanato-dal", emanatoDal, "dataInizioEmanazione"}, {"emanato-al", emanatoAl, "dataFineEmanazione"}} {
				if d.val == "" {
					continue
				}
				day, perr := norma.ParseDate(d.val, timeNow())
				if perr != nil {
					return usageErr(fmt.Errorf("--%s: %w", d.flag, perr))
				}
				if d.key == "dataVigenza" {
					params[d.key] = day
				} else {
					params[d.key] = istante(day)
				}
			}
			pollEvery, perr := cliutil.ParseDurationLoose(pollEveryS)
			if perr != nil || pollEvery < time.Second {
				return usageErr(fmt.Errorf("--intervallo must be a duration of at least 1s (got %q)", pollEveryS))
			}
			waitMax, perr := cliutil.ParseDurationLoose(waitMaxS)
			if perr != nil {
				return usageErr(fmt.Errorf("--attesa-max must be a duration like 15m (got %q)", waitMaxS))
			}
			body := map[string]any{"formato": f, "tipoRicerca": "A", "richiestaExport": m, "modalita": "C", "parametriRicerca": params}
			view := esportaView{Token: token, Richiesta: body}
			if token == "" && len(params) == 1 {
				return usageErr(errors.New("give at least one search criterion (--testo, --titolo, --tipo, --anno, --numero, dates) or --token"))
			}
			if token == "" && !avvia {
				view.Nota = "not submitted: add --avvia to queue this export on Normattiva"
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), view, flags)
				}
				b, _ := json.MarshalIndent(body, "", "  ")
				fmt.Fprintf(cmd.OutOrStdout(), "would submit:\n%s\n%s\n", b, view.Nota)
				return nil
			}
			if cliutil.IsAnyHarness() {
				return writeHarnessRefusal(cmd.OutOrStdout(), flags, "submit export")
			}
			rh, err := newRawHTTP(flags, 2*time.Minute, false)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			if token == "" {
				resp, err := rh.do(ctx, http.MethodPost, "/ricerca-asincrona/nuova-ricerca", body)
				if err != nil {
					return apiErr(err)
				}
				raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
				_ = resp.Body.Close()
				if resp.StatusCode != 200 && resp.StatusCode != 202 {
					return apiErr(fmt.Errorf("export request rejected: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw))))
				}
				token = parseExportToken(raw)
				if token == "" {
					return apiErr(fmt.Errorf("export accepted but no token found in the response: %s", strings.TrimSpace(string(raw))))
				}
				resp, err = rh.do(ctx, http.MethodPut, "/ricerca-asincrona/conferma-ricerca", map[string]string{"token": token})
				if err != nil {
					return apiErr(err)
				}
				_ = resp.Body.Close()
				if resp.StatusCode >= 300 {
					return apiErr(fmt.Errorf("export confirmation failed: HTTP %d (token %s)", resp.StatusCode, token))
				}
				view.Token, view.Avviata = token, true
			}
			location := ""
			deadline := time.Now().Add(waitMax)
			for {
				resp, err := rh.do(ctx, http.MethodGet, "/ricerca-asincrona/check-status/"+url.PathEscape(token), nil)
				if err != nil {
					return apiErr(err)
				}
				raw, _ := io.ReadAll(io.LimitReader(resp.Body, 8192))
				_ = resp.Body.Close()
				var st struct {
					Stato  *int   `json:"stato"`
					Descr  string `json:"descrizioneStato"`
					Errore string `json:"descrizioneErrore"`
				}
				_ = json.Unmarshal(raw, &st)
				if st.Stato != nil {
					view.Stato = *st.Stato
				}
				view.Descr = st.Descr
				if resp.StatusCode == http.StatusSeeOther {
					location = resp.Header.Get("x-ipzs-location")
					view.Stato = 3
				}
				if resp.StatusCode == 404 {
					return notFoundErr(fmt.Errorf("unknown export token %s", token))
				}
				if resp.StatusCode >= 400 {
					return apiErr(fmt.Errorf("export status check failed: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw))))
				}
				if view.Stato == 4 || view.Stato == 5 {
					return apiErr(fmt.Errorf("export %s failed: %s %s", token, st.Descr, st.Errore))
				}
				if view.Stato == 3 || !attendi || time.Now().After(deadline) {
					break
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(pollEvery):
				}
			}
			if view.Stato == 3 && output != "" {
				if location == "" {
					location = "/collections/download/collection-asincrona/" + url.PathEscape(token)
				}
				dl, err := newRawHTTP(flags, 30*time.Minute, true)
				if err != nil {
					return err
				}
				resp, err := dl.do(ctx, http.MethodGet, location, nil)
				if err != nil {
					return apiErr(err)
				}
				if view.Byte, err = downloadTo(resp, output); err != nil {
					return apiErr(err)
				}
				view.File = output
			} else if view.Stato != 3 {
				view.Nota = fmt.Sprintf("still processing; resume with: normattiva-pp-cli esporta --token %s --attendi --output <file.zip>", token)
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), view, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "token %s — stato %d %s\n", view.Token, view.Stato, view.Descr)
			if view.File != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "saved %s (%d bytes)\n", view.File, view.Byte)
			}
			if view.Nota != "" {
				fmt.Fprintln(cmd.OutOrStdout(), view.Nota)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&testo, "testo", "", "Words to search in the act text")
	cmd.Flags().StringVar(&titolo, "titolo", "", "Words to search in the act title")
	cmd.Flags().StringVar(&tipo, "tipo", "", "Act type, e.g. LEGGE, \"DECRETO LEGISLATIVO\", d.lgs.")
	cmd.Flags().IntVar(&anno, "anno", 0, "Year the act was enacted")
	cmd.Flags().IntVar(&numero, "numero", 0, "Act number")
	cmd.Flags().StringVar(&vigenteAl, "vigente-al", "", "Only acts in force on this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&emanatoDal, "emanato-dal", "", "Enacted on or after this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&emanatoAl, "emanato-al", "", "Enacted on or before this date (YYYY-MM-DD)")
	cmd.Flags().StringVar(&formato, "formato", "JSON", "Archive format: AKN, XML, JSON, HTML, PDF, EPUB, RTF or URI")
	cmd.Flags().StringVar(&modo, "modo", "V", "Version: O originale, V vigente, M multivigente")
	cmd.Flags().BoolVar(&avvia, "avvia", false, "Actually submit the export to Normattiva (otherwise only print the request)")
	cmd.Flags().BoolVar(&attendi, "attendi", false, "Poll until the archive is ready (bounded by --attesa-max)")
	cmd.Flags().StringVar(&pollEveryS, "intervallo", "10s", "Polling interval while waiting, e.g. 10s, 1m")
	cmd.Flags().StringVar(&waitMaxS, "attesa-max", "15m", "Give up waiting after this long and print the resume command, e.g. 15m, 1h")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Write the finished ZIP to this file")
	cmd.Flags().StringVar(&token, "token", "", "Resume an export already submitted, by its token")
	return cmd
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newScaricaCmd(flags))
		addNovelCommandIfAbsent(root, newEsportaCmd(flags))
	})
}

var exportTokenRe = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

// parseExportToken pulls the UUID token out of a plain-text or JSON reply.
func parseExportToken(raw []byte) string {
	return exportTokenRe.FindString(string(raw))
}
