// Copyright 2026 aborruso and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/other/openbdap/internal/client"
)

// colonna descrive un campo di un dataset. Il servizio pubblica un nome
// leggibile, un nome fisico e un identificativo offuscato: solo l'ultimo
// funziona nei filtri, ed e' quello che la CLI risolve per conto dell'utente.
type colonna struct {
	Indice      int      `json:"indice"`
	Nome        string   `json:"nome"`
	NomeFisico  string   `json:"nome_fisico"`
	ID          string   `json:"id"`
	Tipo        string   `json:"tipo"`
	Cardinalita int      `json:"cardinalita"`
	Sommabile   bool     `json:"sommabile"`
	Valori      []string `json:"valori,omitempty"`
}

const odataPrefisso = "/ODataProxy/MdData('"

func odataPath(odataID, coda string) string {
	return odataPrefisso + odataID + "@rgs')" + coda
}

// colonneDataset legge lo schema di un dataset dal servizio OData.
func colonneDataset(ctx context.Context, c *client.Client, odataID string) ([]colonna, error) {
	data, err := c.Get(ctx, odataPath(odataID, "/DataColumns"), nil)
	if err != nil {
		return nil, fmt.Errorf("colonne di %s: %w", odataID, err)
	}
	var risposta struct {
		D struct {
			Results []struct {
				ID            int    `json:"id"`
				PhysicalName  string `json:"physicalName"`
				LogicalName   string `json:"logicalName"`
				ColUniqueID   string `json:"colUniqueId"`
				DBType        string `json:"dbType"`
				Cardinality   int    `json:"cardinality"`
				CanSum        bool   `json:"canSum"`
				ValoriDistint string `json:"values"`
			} `json:"results"`
		} `json:"d"`
	}
	if err := json.Unmarshal(data, &risposta); err != nil {
		return nil, fmt.Errorf("lettura delle colonne di %s: %w", odataID, err)
	}
	fuori := make([]colonna, 0, len(risposta.D.Results))
	for _, r := range risposta.D.Results {
		col := colonna{
			Indice:      r.ID,
			Nome:        r.LogicalName,
			NomeFisico:  r.PhysicalName,
			ID:          r.ColUniqueID,
			Tipo:        r.DBType,
			Cardinalita: r.Cardinality,
			Sommabile:   r.CanSum,
		}
		if r.ValoriDistint != "" {
			var valori []string
			if err := json.Unmarshal([]byte(r.ValoriDistint), &valori); err == nil {
				col.Valori = valori
			}
		}
		fuori = append(fuori, col)
	}
	return fuori, nil
}

// risolviColonna accetta il nome leggibile, il nome fisico o l'identificativo
// gia' offuscato e restituisce l'identificativo da mettere nella query.
func risolviColonna(colonne []colonna, chiave string) (colonna, bool) {
	k := normalizza(chiave)
	// Senza questo controllo la ricerca parziale piu' sotto accetterebbe la
	// stringa vuota, che e' contenuta in qualsiasi nome: '--dove "=valore"'
	// finirebbe per filtrare sulla prima colonna del dataset.
	if k == "" {
		return colonna{}, false
	}
	for _, c := range colonne {
		if normalizza(c.ID) == k || normalizza(c.NomeFisico) == k || normalizza(c.Nome) == k {
			return c, true
		}
	}
	// La ricerca parziale accetta solo una corrispondenza: con piu' colonne
	// che contengono il testo, sceglierne una in silenzio filtrerebbe su un
	// campo diverso da quello inteso e restituirebbe righe sbagliate.
	var parziali []colonna
	for _, c := range colonne {
		if strings.Contains(normalizza(c.Nome), k) {
			parziali = append(parziali, c)
		}
	}
	if len(parziali) == 1 {
		return parziali[0], true
	}
	return colonna{}, false
}

// colonneAmbigue elenca i nomi leggibili che contengono la chiave, per dire a
// chi sbaglia quali colonne stava per confondere.
func colonneAmbigue(colonne []colonna, chiave string) []string {
	k := normalizza(chiave)
	if k == "" {
		return nil
	}
	var nomi []string
	for _, c := range colonne {
		if strings.Contains(normalizza(c.Nome), k) {
			nomi = append(nomi, c.Nome)
		}
	}
	if len(nomi) < 2 {
		return nil
	}
	return nomi
}

// filtroUguale costruisce una condizione di uguaglianza OData. L'apostrofo va
// raddoppiato: altrimenti chiude la stringa e il servizio rifiuta la query.
func filtroUguale(idColonna, valore string) string {
	return fmt.Sprintf("%s eq '%s'", idColonna, strings.ReplaceAll(valore, "'", "''"))
}

// costruisciFiltro traduce le condizioni "campo=valore" in un filtro OData,
// risolvendo i nomi leggibili negli identificativi che il servizio pretende.
// Le condizioni con ~ diventano substringof.
func costruisciFiltro(colonne []colonna, condizioni []string) (string, error) {
	var parti []string
	for _, cond := range condizioni {
		cond = strings.TrimSpace(cond)
		if cond == "" {
			continue
		}
		sep := "="
		if i := strings.Index(cond, "~"); i >= 0 && (strings.Index(cond, "=") < 0 || i < strings.Index(cond, "=")) {
			sep = "~"
		}
		pezzi := strings.SplitN(cond, sep, 2)
		if len(pezzi) != 2 {
			return "", fmt.Errorf("condizione %q non valida: usare campo=valore oppure campo~testo", cond)
		}
		if strings.TrimSpace(pezzi[0]) == "" {
			return "", fmt.Errorf("condizione %q non valida: manca il nome della colonna", cond)
		}
		col, ok := risolviColonna(colonne, pezzi[0])
		if !ok {
			if ambigue := colonneAmbigue(colonne, pezzi[0]); len(ambigue) > 0 {
				return "", fmt.Errorf("colonna %q ambigua: corrisponde a %s. Indica il nome esatto", pezzi[0], strings.Join(ambigue, ", "))
			}
			return "", fmt.Errorf("colonna %q non trovata: usa 'colonne <dataset>' per vedere i nomi disponibili", pezzi[0])
		}
		valore := scartaVirgolette(strings.TrimSpace(pezzi[1]))
		if sep == "~" {
			parti = append(parti, fmt.Sprintf("substringof('%s',%s)", strings.ReplaceAll(valore, "'", "''"), col.ID))
			continue
		}
		parti = append(parti, filtroUguale(col.ID, valore))
	}
	return strings.Join(parti, " and "), nil
}

// scartaVirgolette toglie una sola coppia di virgolette o apici che racchiuda
// il valore. Trim toglierebbe anche l'apostrofo finale di un valore legittimo.
func scartaVirgolette(valore string) string {
	if len(valore) >= 2 {
		primo, ultimo := valore[0], valore[len(valore)-1]
		if (primo == '"' && ultimo == '"') || (primo == '\'' && ultimo == '\'') {
			return valore[1 : len(valore)-1]
		}
	}
	return valore
}

// righeDataset legge una pagina di righe e le restituisce con i nomi di colonna
// leggibili al posto degli identificativi offuscati.
func righeDataset(ctx context.Context, c *client.Client, odataID string, colonne []colonna, filtro string, campi []string, limite, salta int) ([]map[string]any, error) {
	params := map[string]string{
		"$top":  strconv.Itoa(limite),
		"$skip": strconv.Itoa(salta),
	}
	if filtro != "" {
		params["$filter"] = filtro
	}
	if len(campi) > 0 {
		var ids []string
		for _, campo := range campi {
			if strings.TrimSpace(campo) == "" {
				return nil, fmt.Errorf("elenco dei campi non valido: c'e' un nome vuoto")
			}
			col, ok := risolviColonna(colonne, campo)
			if !ok {
				return nil, fmt.Errorf("colonna %q non trovata: usa 'colonne <dataset>' per vedere i nomi disponibili", campo)
			}
			ids = append(ids, col.ID)
		}
		params["$select"] = strings.Join(ids, ",")
	}
	data, err := c.Get(ctx, odataPath(odataID, "/DataRows"), params)
	if err != nil {
		return nil, fmt.Errorf("righe di %s: %w", odataID, err)
	}
	var risposta struct {
		D struct {
			Results []map[string]any `json:"results"`
		} `json:"d"`
	}
	if err := json.Unmarshal(data, &risposta); err != nil {
		return nil, fmt.Errorf("lettura delle righe di %s: %w", odataID, err)
	}
	etichette := make(map[string]string, len(colonne))
	for _, col := range colonne {
		etichette[col.ID] = col.Nome
	}
	fuori := make([]map[string]any, 0, len(risposta.D.Results))
	for _, riga := range risposta.D.Results {
		pulita := make(map[string]any, len(riga))
		for k, v := range riga {
			// row_id e' l'indice della riga dentro la pagina OData: riparte da
			// zero a ogni chiamata e non dice nulla sul dato. __metadata e'
			// l'involucro del protocollo.
			if k == "__metadata" || k == "row_id" {
				continue
			}
			if etichetta, ok := etichette[k]; ok && etichetta != "" {
				pulita[etichetta] = v
				continue
			}
			pulita[k] = v
		}
		fuori = append(fuori, pulita)
	}
	return fuori, nil
}

// compattaRighe toglie dai risultati i campi senza contenuto. Su questi
// dataset sono spesso meta' della riga, e --compact altrimenti non avrebbe
// effetto, perche' cerca nomi convenzionali (id, name, status) che qui non
// esistono: le colonne hanno i nomi leggibili del dataset.
func compattaRighe(righe []map[string]any, compatta bool) []map[string]any {
	if !compatta {
		return righe
	}
	fuori := make([]map[string]any, 0, len(righe))
	for _, riga := range righe {
		pulita := make(map[string]any, len(riga))
		for k, v := range riga {
			if campoVuoto(v) {
				continue
			}
			pulita[k] = v
		}
		fuori = append(fuori, pulita)
	}
	return fuori
}

// campoVuoto riconosce i valori che il servizio usa per "nessun dato": la
// stringa vuota e il valore assente. Uno zero non e' un campo mancante: su
// questi dataset "Costo Lavori Effettivo: 0.00" e' un importo registrato, e
// toglierlo lo renderebbe indistinguibile da un dato che non c'e'.
func campoVuoto(v any) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(t) == ""
	default:
		return false
	}
}

// contaRighe usa il parametro non standard count=true, che e' l'unico
// conteggio affidabile del servizio: $inlinecount restituisce sempre zero.
func contaRighe(ctx context.Context, c *client.Client, odataID, filtro string) (int, error) {
	params := map[string]string{"count": "true"}
	if filtro != "" {
		params["$filter"] = filtro
	}
	data, err := c.Get(ctx, odataPath(odataID, "/DataRows"), params)
	if err != nil {
		return 0, fmt.Errorf("conteggio di %s: %w", odataID, err)
	}
	var risposta struct {
		D struct {
			Results []struct {
				Tot json.Number `json:"Tot"`
			} `json:"results"`
		} `json:"d"`
	}
	if err := json.Unmarshal(data, &risposta); err != nil {
		return 0, fmt.Errorf("lettura del conteggio di %s: %w", odataID, err)
	}
	if len(risposta.D.Results) == 0 {
		// Una query valida restituisce sempre una riga con Tot, anche quando
		// il conteggio e' zero: nessuna riga significa filtro rifiutato.
		return 0, fmt.Errorf("il servizio non ha restituito un conteggio per %s: il filtro e' stato rifiutato", odataID)
	}
	n, err := risposta.D.Results[0].Tot.Int64()
	if err != nil {
		return 0, fmt.Errorf("conteggio non numerico per %s", odataID)
	}
	return int(n), nil
}
