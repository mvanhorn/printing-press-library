// Copyright 2026 aborruso and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source live

package cli

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/other/openbdap/internal/cliutil"
)

// newScaricaCmd scarica il CSV integrale di un dataset.
//
// Il comando emesso dal generatore per questo endpoint non funziona: pretende
// una risposta JSON e sul CSV esce con "API returned a non-JSON response".
// Qui la risposta si scrive cosi' com'e', a flusso, perche' i dump arrivano
// anche a centinaia di megabyte.
func newScaricaCmd(flags *rootFlags) *cobra.Command {
	var destinazione string
	var dbPath string
	var grezzo bool

	cmd := &cobra.Command{
		Use:   "scarica [dataset]",
		Short: "Scarica l'intero dataset in CSV (separatore punto e virgola)",
		Long: "Scarica il dump CSV completo di un dataset, convertito in UTF-8: il portale lo serve in latin-1, " +
			"mentre il resto della CLI parla UTF-8. Con --raw i byte restano come arrivano.\n" +
			"Senza --output il contenuto va sullo standard output.\n" +
			"Usa questo comando per prendere tutto il dataset. NON usarlo per estrarre poche righe filtrate; usa 'righe'.",
		Example: strings.Trim(`
  openbdap-pp-cli scarica d032b3a2-2b70-4193-a0c8-cb7eb69f8710 --output conto-economico.csv
  openbdap-pp-cli scarica d032b3a2-2b70-4193-a0c8-cb7eb69f8710 | head -5
`, "\n"),
		Annotations: map[string]string{
			"mcp:read-only":  "true",
			"pp:data-source": "live",
			"pp:happy-args":  "dataset=d032b3a2-2b70-4193-a0c8-cb7eb69f8710",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "scarica")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("indica il dataset da scaricare"))
			}
			id := args[0]
			if d, ok, err := risolviLocaleMorbido(cmd, dbPath, args[0]); err != nil {
				return err
			} else if ok && d.ID != "" {
				id = d.ID
			}
			// L'indirizzo http pubblicato nei metadati non risponde: serve https.
			url := baseSito + pathDumpPrefix + id + ".csv"

			req, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, url, nil)
			if err != nil {
				return err
			}
			req.Header.Set("Accept", "text/csv")
			// Nessun timeout complessivo: i dump arrivano a centinaia di
			// megabyte e --timeout, se impostato, vale per l'attesa delle
			// intestazioni, non per l'intero trasferimento.
			trasporto := http.DefaultTransport.(*http.Transport).Clone()
			if flags.timeout > 0 {
				trasporto.ResponseHeaderTimeout = flags.timeout
			}
			client := &http.Client{Transport: trasporto}
			if cliutil.IsDogfoodEnv() {
				// Il banco di prova concede 120 secondi: il portale a volte
				// impiega decine di secondi solo per rispondere.
				client.Timeout = 90 * time.Second
			}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("scaricamento di %s: %w", id, err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("il portale ha risposto %s per il dataset %s", resp.Status, id)
			}
			// Il portale risponde 200 con una pagina HTML quando l'id non
			// esiste: senza questo controllo verrebbe scritta come se fosse CSV.
			if tipo := resp.Header.Get("Content-Type"); tipo != "" && !strings.Contains(strings.ToLower(tipo), "csv") {
				return fmt.Errorf("il portale ha risposto %s invece di CSV per il dataset %s: controlla l'identificativo", tipo, id)
			}

			// Con un formato macchina esplicito il CSV grezzo renderebbe
			// l'output non parsabile: si scrive su file e si stampa una
			// ricevuta. Con lo standard output in pipe, invece, il CSV deve
			// continuare a scorrere: 'scarica <id> | head -5' e' un uso normale.
			riceuta := wantsMachineOutput(flags)
			if destinazione == "" && riceuta {
				destinazione = id + ".csv"
			}
			destinatario := cmd.OutOrStdout()
			var file *os.File
			var temporaneo string
			if destinazione != "" {
				// Si scrive su un file temporaneo accanto alla destinazione e
				// si rinomina solo a scaricamento riuscito: un dump di
				// centinaia di megabyte interrotto a meta' non deve
				// distruggere una copia valida gia' presente.
				// La destinazione e' il percorso chiesto dall'operatore con
				// --output, oppure "<id>.csv" costruito qui sopra: scrivere
				// dove e' stato indicato e' il compito del comando.
				file, err = os.CreateTemp(filepath.Dir(destinazione), filepath.Base(destinazione)+".parziale-*") // #nosec G304 -- percorso di scrittura scelto da chi invoca la CLI
				if err != nil {
					return err
				}
				temporaneo = file.Name()
				defer func() {
					file.Close()
					if temporaneo != "" {
						os.Remove(temporaneo)
					}
				}()
				destinatario = file
			}
			// Il portale serve il dump in latin-1, mentre tutto il resto
			// della CLI parla UTF-8: due codifiche a seconda del comando sono
			// una trappola silenziosa, e chi legge il CSV senza accorgersene
			// si porta i comuni accentati corrotti fino in fondo.
			var sorgente io.Reader = resp.Body
			if !grezzo && !dichiaraUTF8(resp.Header.Get("Content-Type")) {
				sorgente = nuovoLettoreUTF8(resp.Body)
			}
			scritti, err := io.Copy(destinatario, sorgente)
			if err != nil {
				return fmt.Errorf("scrittura del CSV: %w", err)
			}
			if destinazione == "" {
				return nil
			}
			if err := file.Sync(); err != nil {
				return err
			}
			if err := file.Close(); err != nil {
				return err
			}
			if err := os.Rename(temporaneo, destinazione); err != nil {
				return fmt.Errorf("rinomina del file scaricato: %w", err)
			}
			temporaneo = ""
			if riceuta {
				return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
					"dataset": id,
					"file":    destinazione,
					"byte":    scritti,
				}, flags)
			}
			fmt.Fprintf(cmd.ErrOrStderr(), "scritti %d byte in %s\n", scritti, destinazione)
			return nil
		},
	}
	cmd.Flags().StringVar(&destinazione, "output", "", "scrivi il CSV in un file invece che sullo standard output")
	cmd.Flags().StringVar(&dbPath, "db", defaultDBPath("openbdap-pp-cli"), "percorso dell'archivio locale")
	cmd.Flags().BoolVar(&grezzo, "raw", false, "scrivi i byte come li serve il portale, senza convertirli in UTF-8")
	return cmd
}

// dichiaraUTF8 riconosce un Content-Type che dichiara gia' UTF-8: in quel caso
// non c'e' niente da convertire.
func dichiaraUTF8(tipo string) bool {
	return strings.Contains(strings.ToLower(tipo), "utf-8")
}

const (
	codificaSconosciuta = iota
	codificaUTF8
	codificaLatin1
)

// lettoreUTF8 porta in UTF-8 un flusso servito in latin-1. La codifica si
// decide al primo byte non ASCII: se apre una sequenza UTF-8 valida il flusso
// passa intatto, altrimenti ogni byte alto diventa il carattere corrispondente.
// Finche' arrivano solo byte ASCII la conversione e' l'identita', quindi la
// decisione puo' aspettare senza rischi.
type lettoreUTF8 struct {
	src       *bufio.Reader
	codifica  int
	ingresso  []byte
	avanzo    []byte
	avanzoBuf [4]byte
}

func nuovoLettoreUTF8(r io.Reader) *lettoreUTF8 {
	return &lettoreUTF8{src: bufio.NewReaderSize(r, 64*1024)}
}

func (l *lettoreUTF8) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	if len(l.avanzo) > 0 {
		n := copy(p, l.avanzo)
		l.avanzo = l.avanzo[n:]
		return n, nil
	}
	soloASCII := -1
	if l.codifica == codificaSconosciuta {
		soloASCII = l.decidi()
	}
	if l.codifica == codificaUTF8 {
		return l.src.Read(p)
	}
	// Un byte alto diventa due byte: si legge al massimo meta' della
	// destinazione, cosi' il risultato ci sta sempre.
	massimo := len(p) / 2
	if massimo < 1 {
		massimo = 1
	}
	// Decisione rimandata: si consumano solo i byte ASCII gia' certi, sui
	// quali la conversione e' l'identita'. Alla lettura successiva il buffer
	// si e' riempito e la sequenza a cavallo del taglio si legge intera.
	if l.codifica == codificaSconosciuta && soloASCII >= 0 && soloASCII < massimo {
		massimo = soloASCII
		if massimo < 1 {
			massimo = 1
		}
	}
	if len(l.ingresso) < massimo {
		l.ingresso = make([]byte, massimo)
	}
	n, err := l.src.Read(l.ingresso[:massimo])
	scritti := 0
	for _, b := range l.ingresso[:n] {
		if b < 0x80 {
			p[scritti] = b
			scritti++
			continue
		}
		if scritti+1 < len(p) {
			p[scritti] = 0xC0 | b>>6
			p[scritti+1] = 0x80 | b&0x3F
			scritti += 2
			continue
		}
		// Succede solo con una destinazione da un byte: la coppia si tiene da
		// parte invece di spezzare il carattere.
		l.avanzoBuf[0] = 0xC0 | b>>6
		l.avanzoBuf[1] = 0x80 | b&0x3F
		l.avanzo = l.avanzoBuf[:2]
		break
	}
	if scritti > 0 {
		return scritti, nil
	}
	return 0, err
}

// decidi guarda avanti nel flusso senza consumarlo e restituisce quanti byte
// iniziali sono ASCII, cioe' quanti se ne possono scrivere comunque mentre la
// decisione resta aperta. Restituisce -1 quando la codifica e' decisa.
//
// La decisione guarda tutto il blocco, non il primo carattere accentato: un
// dump latin-1 contiene quasi sempre una sequenza che in UTF-8 non sta in
// piedi, mentre un singolo byte alto puo' somigliare per caso a una sequenza
// valida. Una sequenza tagliata in fondo al buffer non e' una sequenza
// invalida, quindi non decide: si consuma l'ASCII che la precede e si torna a
// guardare con il buffer riempito.
func (l *lettoreUTF8) decidi() int {
	prefisso, err := l.src.Peek(l.src.Size())
	primoAlto := -1
	for i, b := range prefisso {
		if b >= 0x80 {
			primoAlto = i
			break
		}
	}
	if primoAlto < 0 {
		// Finora solo ASCII: la conversione e' l'identita', si va avanti.
		return len(prefisso)
	}
	fine := len(prefisso)
	if err == nil {
		// Il buffer e' pieno: il flusso continua oltre, quindi l'ultima
		// sequenza puo' essere tagliata a meta'.
		fine = tagliaRuneIncompleta(prefisso)
		if fine <= primoAlto {
			return primoAlto
		}
	}
	if utf8.Valid(prefisso[:fine]) {
		l.codifica = codificaUTF8
	} else {
		l.codifica = codificaLatin1
	}
	return -1
}

// tagliaRuneIncompleta restituisce la lunghezza del blocco togliendo una
// eventuale sequenza UTF-8 iniziata e non conclusa in coda.
func tagliaRuneIncompleta(blocco []byte) int {
	for i := len(blocco) - 1; i >= 0 && i >= len(blocco)-3; i-- {
		b := blocco[i]
		if b&0xC0 == 0x80 {
			// Byte di continuazione: si risale all'inizio della sequenza.
			continue
		}
		if b < 0x80 {
			// Carattere ASCII: la coda e' completa.
			return len(blocco)
		}
		var attesa int
		switch {
		case b&0xE0 == 0xC0:
			attesa = 2
		case b&0xF0 == 0xE0:
			attesa = 3
		case b&0xF8 == 0xF0:
			attesa = 4
		default:
			// Byte di partenza non valido: non e' una sequenza tagliata.
			return len(blocco)
		}
		if len(blocco)-i < attesa {
			return i
		}
		return len(blocco)
	}
	return len(blocco)
}
