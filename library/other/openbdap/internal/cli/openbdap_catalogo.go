// Copyright 2026 aborruso and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mvanhorn/printing-press-library/library/other/openbdap/internal/client"
	"github.com/mvanhorn/printing-press-library/library/other/openbdap/internal/store"
)

const (
	catalogoTipo    = "catalogo"
	pathPackageList = "/SpodCkanApi/api/3/action/package_list"
	pathPackageShow = "/SpodCkanApi/api/3/action/package_show"
	pathGroupShow   = "/SpodCkanApi/api/3/action/group_show"
	pathDumpPrefix  = "/SpodCkanApi/api/3/datastore/dump/"
	baseSito        = "https://bdap-opendata.rgs.mef.gov.it"
)

// dataset e' la vista locale di un dataset del catalogo: i campi CKAN utili
// piu' i campi derivati dal titolo e dalle risorse, che l'API non espone.
type dataset struct {
	ID       string `json:"id"`
	Nome     string `json:"nome"`
	Titolo   string `json:"titolo"`
	Note     string `json:"note,omitempty"`
	Tema     string `json:"tema,omitempty"`
	Tag      string `json:"tag,omitempty"`
	Licenza  string `json:"licenza,omitempty"`
	Aggiorn  string `json:"aggiornato,omitempty"`
	ODataID  string `json:"odata_id,omitempty"`
	CSVURL   string `json:"csv_url,omitempty"`
	Pagina   string `json:"pagina,omitempty"`
	Anno     string `json:"anno,omitempty"`
	Periodo  string `json:"periodo,omitempty"`
	Regione  string `json:"regione,omitempty"`
	Serie    string `json:"serie,omitempty"`
	Famiglia string `json:"famiglia_mop,omitempty"`
}

var (
	reAnnoMese = regexp.MustCompile(`^(\d{4})/(\d{2})\s*-\s*`)
	reAnno     = regexp.MustCompile(`^(\d{4})\s*-\s*`)
	reMopNome  = regexp.MustCompile(`^spd_mop_([a-z]{3})_`)
)

// regioni elencate come compaiono nei titoli dei dataset MOP.
var regioniNote = []string{
	"Abruzzo", "Basilicata", "Calabria", "Campania", "Emilia-Romagna",
	"Friuli-Venezia Giulia", "Lazio", "Liguria", "Lombardia", "Marche",
	"Molise", "Piemonte", "Puglia", "Sardegna", "Sicilia", "Toscana",
	"Trentino-Alto Adige", "Umbria", "Valle d'Aosta", "Veneto",
	"Territorio Nazionale", "Totale",
}

// famiglieMOP mappa il codice presente nel nome del dataset al ruolo leggibile.
var famiglieMOP = map[string]string{
	"prg": "progetti",
	"gar": "gare",
	"pga": "partecipanti",
	"sal": "pagamenti",
	"pdc": "piano-costi",
	"sog": "soggetti-titolari",
	"loc": "localizzazione",
}

// famiglieMOPOrdinate e' l'ordine in cui il dossier interroga le famiglie.
var famiglieMOPOrdinate = []string{"progetti", "pagamenti", "gare", "partecipanti", "piano-costi", "soggetti-titolari"}

func normalizza(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// derivaDataset costruisce la vista locale a partire dal payload CKAN.
func derivaDataset(raw map[string]any) dataset {
	d := dataset{
		ID:      testo(raw["id"]),
		Nome:    testo(raw["name"]),
		Titolo:  testo(raw["title"]),
		Note:    strings.TrimSpace(testo(raw["notes"])),
		Licenza: testo(raw["license_id"]),
		Aggiorn: testo(raw["metadata_modified"]),
	}
	if gruppi, ok := raw["groups"].([]any); ok && len(gruppi) > 0 {
		var nomi []string
		for _, g := range gruppi {
			if m, ok := g.(map[string]any); ok {
				nomi = append(nomi, testo(m["name"]))
			}
		}
		d.Tema = strings.Join(nomi, ",")
	}
	if tag, ok := raw["tags"].([]any); ok && len(tag) > 0 {
		var nomi []string
		for _, t := range tag {
			if m, ok := t.(map[string]any); ok {
				nomi = append(nomi, testo(m["name"]))
			}
		}
		d.Tag = strings.Join(nomi, ",")
	}
	if risorse, ok := raw["resources"].([]any); ok {
		for _, r := range risorse {
			m, ok := r.(map[string]any)
			if !ok {
				continue
			}
			if strings.EqualFold(testo(m["resource_type"]), "OData") {
				d.ODataID = testo(m["id"])
				continue
			}
			u := testo(m["url"])
			if strings.Contains(u, "/datastore/dump/") && strings.HasSuffix(strings.ToLower(u), ".csv") {
				d.CSVURL = baseSito + pathDumpPrefix + d.ID + ".csv"
			}
		}
	}
	if slug := slugTitolo(d.Titolo); slug != "" {
		d.Pagina = baseSito + "/content/" + slug
	}
	d.Anno, d.Periodo, d.Regione, d.Serie = derivaSerie(d.Titolo)
	if m := reMopNome.FindStringSubmatch(d.Nome); m != nil {
		d.Famiglia = famiglieMOP[m[1]]
	}
	return d
}

// derivaSerie estrae anno, periodo, regione e nome di serie dal titolo.
// I titoli seguono le forme "AAAA - Testo", "AAAA/MM - Testo" e "Testo - Regione".
func derivaSerie(titolo string) (anno, periodo, regione, serie string) {
	resto := titolo
	if m := reAnnoMese.FindStringSubmatch(titolo); m != nil {
		anno, periodo = m[1], m[1]+"/"+m[2]
		resto = titolo[len(m[0]):]
	} else if m := reAnno.FindStringSubmatch(titolo); m != nil {
		anno, periodo = m[1], m[1]
		resto = titolo[len(m[0]):]
	}
	// La regione compare in coda nei dataset MOP ("... - Sicilia") e subito
	// dopo l'anno nelle serie SIOPE ("2024 - Sicilia - SIOPE ..."): vanno
	// riconosciute entrambe, altrimenti il filtro --regione perde meta' del
	// catalogo.
	// Il confronto ignora le maiuscole perche' il portale scrive la stessa
	// regione in modi diversi ("Valle d'Aosta" e "Valle D'Aosta"); la forma
	// restituita e' sempre quella canonica, cosi' i raggruppamenti non si
	// spezzano in due.
	bassoResto := strings.ToLower(resto)
	for _, r := range regioniNote {
		if suffisso := " - " + strings.ToLower(r); strings.HasSuffix(bassoResto, suffisso) {
			regione = r
			resto = resto[:len(resto)-len(suffisso)]
			break
		}
		if prefisso := strings.ToLower(r) + " - "; strings.HasPrefix(bassoResto, prefisso) {
			regione = r
			resto = resto[len(prefisso):]
			break
		}
	}
	serie = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(resto), "."))
	return anno, periodo, regione, serie
}

// slugTitolo ricostruisce lo slug della pagina del portale dal titolo:
// il portale pubblica /content/<titolo-normalizzato>, non /content/<nome>.
func slugTitolo(titolo string) string {
	var b strings.Builder
	precedenteTrattino := false
	for _, r := range strings.ToLower(titolo) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			precedenteTrattino = false
		default:
			if !precedenteTrattino && b.Len() > 0 {
				b.WriteByte('-')
				precedenteTrattino = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func testo(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case nil:
		return ""
	default:
		return fmt.Sprint(t)
	}
}

// elencoDataset chiede al portale gli identificativi di tutti i dataset.
func elencoDataset(ctx context.Context, c *client.Client) ([]string, error) {
	data, err := c.Get(ctx, pathPackageList, nil)
	if err != nil {
		return nil, fmt.Errorf("elenco dei dataset: %w", err)
	}
	var risposta struct {
		Result []string `json:"result"`
	}
	if err := json.Unmarshal(data, &risposta); err != nil {
		return nil, fmt.Errorf("lettura dell'elenco dei dataset: %w", err)
	}
	return risposta.Result, nil
}

// dettaglioDataset legge i metadati di un dataset. Il portale li risolve solo
// per UUID: i nomi restituiscono una pagina di aiuto, non un errore.
func dettaglioDataset(ctx context.Context, c *client.Client, id string) (map[string]any, error) {
	data, err := c.Get(ctx, pathPackageShow, map[string]string{"id": id})
	if err != nil {
		return nil, err
	}
	var risposta struct {
		Success bool           `json:"success"`
		Result  map[string]any `json:"result"`
	}
	if err := json.Unmarshal(data, &risposta); err != nil {
		return nil, fmt.Errorf("lettura dei metadati di %s: %w", id, err)
	}
	if risposta.Result == nil {
		return nil, fmt.Errorf("nessun metadato per %s", id)
	}
	return risposta.Result, nil
}

// datasetDelGruppo restituisce gli UUID dei dataset di un tema.
func datasetDelGruppo(ctx context.Context, c *client.Client, gruppo string) ([]string, error) {
	data, err := c.Get(ctx, pathGroupShow, map[string]string{"id": gruppo})
	if err != nil {
		return nil, err
	}
	var risposta struct {
		Result struct {
			Packages []string `json:"packages"`
		} `json:"result"`
	}
	if err := json.Unmarshal(data, &risposta); err != nil {
		return nil, fmt.Errorf("lettura del tema %s: %w", gruppo, err)
	}
	return risposta.Result.Packages, nil
}

// scaricaDataset legge in parallelo i metadati degli id indicati e passa ogni
// dataset alla funzione di raccolta. Gli errori viaggiano con il risultato:
// un dataset che non risponde non deve svuotare l'allineamento.
func scaricaDataset(ctx context.Context, c *client.Client, ids []string, paralleli int, raccogli func(dataset) error) (int, []error) {
	if paralleli < 1 {
		paralleli = 1
	}
	type esito struct {
		d   dataset
		err error
	}
	lavori := make(chan string)
	esiti := make(chan esito)
	var wg sync.WaitGroup
	for i := 0; i < paralleli; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range lavori {
				raw, err := dettaglioDataset(ctx, c, id)
				if err != nil && ctx.Err() == nil {
					// Il portale va in timeout a intermittenza su richieste
					// isolate: un secondo tentativo recupera la gran parte
					// dei dataset che il primo passaggio perde.
					select {
					case <-ctx.Done():
					case <-time.After(time.Second):
					}
					if ctx.Err() == nil {
						raw, err = dettaglioDataset(ctx, c, id)
					}
				}
				if err != nil {
					esiti <- esito{err: fmt.Errorf("dataset %s: %w", id, err)}
					continue
				}
				esiti <- esito{d: derivaDataset(raw)}
			}
		}()
	}
	go func() {
		defer close(lavori)
		for _, id := range ids {
			select {
			case <-ctx.Done():
				return
			case lavori <- id:
			}
		}
	}()
	go func() {
		wg.Wait()
		close(esiti)
	}()

	var errori []error
	contati := 0
	for e := range esiti {
		if e.err != nil {
			errori = append(errori, e.err)
			continue
		}
		if err := raccogli(e.d); err != nil {
			errori = append(errori, err)
			continue
		}
		contati++
	}
	return contati, errori
}

// salvaDataset scrive un dataset nella tabella locale del catalogo.
func salvaDataset(db *store.Store, d dataset) error {
	blob, err := json.Marshal(d)
	if err != nil {
		return err
	}
	return db.Upsert(catalogoTipo, d.ID, blob)
}

// leggiDataset rilegge tutti i dataset dallo store locale.
func leggiDataset(db *store.Store) ([]dataset, error) {
	righe, err := db.List(catalogoTipo, 0)
	if err != nil {
		return nil, err
	}
	fuori := make([]dataset, 0, len(righe))
	illeggibili := 0
	for _, riga := range righe {
		var d dataset
		if err := json.Unmarshal(riga, &d); err != nil {
			// Una riga illeggibile farebbe sparire un dataset da ogni
			// comando locale mentre il conteggio continua a dirsi completo.
			illeggibili++
			continue
		}
		// I campi derivati si ricalcolano a ogni lettura: sono ricavati dal
		// titolo e dal nome, quindi costano poco, e un archivio allineato
		// prima di un miglioramento del riconoscimento resta valido senza
		// doverlo riscaricare.
		d.Anno, d.Periodo, d.Regione, d.Serie = derivaSerie(d.Titolo)
		if m := reMopNome.FindStringSubmatch(d.Nome); m != nil {
			d.Famiglia = famiglieMOP[m[1]]
		}
		fuori = append(fuori, d)
	}
	if illeggibili > 0 {
		return fuori, fmt.Errorf("%d righe dell'archivio locale non sono leggibili: rilancia 'openbdap-pp-cli allinea'", illeggibili)
	}
	sort.Slice(fuori, func(i, j int) bool { return fuori[i].Titolo < fuori[j].Titolo })
	return fuori, nil
}

// trovaDataset risolve un dataset locale per UUID, nome, identificativo OData
// o titolo esatto. Restituisce false quando non c'e' corrispondenza.
func trovaDataset(elenco []dataset, chiave string) (dataset, bool) {
	k := normalizza(chiave)
	for _, d := range elenco {
		if normalizza(d.ID) == k || normalizza(d.Nome) == k || normalizza(d.ODataID) == k || normalizza(d.Titolo) == k {
			return d, true
		}
	}
	return dataset{}, false
}

// datasetMOP filtra i dataset di una famiglia MOP, eventualmente per regione.
func datasetMOP(elenco []dataset, famiglia, regione string) []dataset {
	var fuori []dataset
	for _, d := range elenco {
		if d.Famiglia == "" || d.ODataID == "" {
			continue
		}
		if famiglia != "" && d.Famiglia != famiglia {
			continue
		}
		if regione != "" && normalizza(d.Regione) != normalizza(regione) {
			continue
		}
		fuori = append(fuori, d)
	}
	sort.Slice(fuori, func(i, j int) bool { return fuori[i].Regione < fuori[j].Regione })
	return fuori
}
