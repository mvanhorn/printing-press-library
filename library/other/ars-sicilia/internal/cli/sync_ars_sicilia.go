// Implementazione del sync per i 12 archivi ARS Sicilia.
// Usa icaroclient.Search (query "all") e salva i record nel DB locale via store.Upsert.
// analytics e ddl drift leggono dalla tabella generica `resources`, quindi basta
// store.Upsert (nessun dispatch per-tipo necessario).

package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	icaro "github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/icaroclient"
	"github.com/mvanhorn/printing-press-library/library/other/ars-sicilia/internal/store"
	"github.com/spf13/cobra"
)

// runSyncAll sincronizza gli archivi selezionati nel DB SQLite locale.
func runSyncAll(cmd *cobra.Command, flags *rootFlags, dbPath string, maxPages int, resourcesFilter []string, legisl string) error {
	if dbPath == "" {
		dbPath = defaultDBPath("ars-sicilia-pp-cli")
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	db, err := store.OpenWithContext(ctx, dbPath)
	if err != nil {
		return fmt.Errorf("aprendo il database: %w", err)
	}
	defer db.Close()

	archives := filterArchives(icaro.All, resourcesFilter)
	if len(archives) == 0 {
		return fmt.Errorf("nessun archivio trovato per il filtro %v", resourcesFilter)
	}

	// 0 = tutte le pagine: mappiamo a un valore alto; il client si ferma a totalPages.
	clientMaxPages := maxPages
	if clientMaxPages == 0 {
		clientMaxPages = 9999
	}

	out := cmd.OutOrStdout()

	type archiveResult struct {
		Archivio string `json:"archivio"`
		Records  int    `json:"records"`
		Errore   string `json:"errore,omitempty"`
	}
	var results []archiveResult

	for _, arc := range archives {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		params := map[string]string{}
		if legisl != "" {
			params["legisl"] = legisl
		}

		if !flags.asJSON {
			fmt.Fprintf(out, "→ %s (%s)…\n", arc.Slug, arc.Description)
		}

		c, err := icaro.New(nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  errore client %s: %v\n", arc.Slug, err)
			results = append(results, archiveResult{Archivio: arc.Slug, Errore: err.Error()})
			continue
		}

		recs, err := c.Search(ctx, arc, icaro.SearchOptions{
			Params:   params,
			MaxPages: clientMaxPages,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "  errore ricerca %s: %v\n", arc.Slug, err)
			results = append(results, archiveResult{Archivio: arc.Slug, Errore: err.Error()})
			continue
		}

		count := 0
		for _, r := range recs {
			id, flat := flattenRecord(arc, r)
			if id == "" || id == "-" {
				continue
			}
			data, merr := json.Marshal(flat)
			if merr != nil {
				continue
			}
			if uerr := db.Upsert(arc.Slug, id, json.RawMessage(data)); uerr != nil {
				fmt.Fprintf(os.Stderr, "  upsert %s/%s: %v\n", arc.Slug, id, uerr)
				continue
			}
			// Snapshot into resources_history so ddl_drift can compare across syncs.
			if arc.Slug == "ddl" {
				_, _ = db.DB().ExecContext(cmd.Context(),
					`INSERT INTO resources_history (resource_type, resource_id, data, captured_at)
					 VALUES (?, ?, ?, ?)`,
					arc.Slug, id, json.RawMessage(data), time.Now().UTC().Format(time.RFC3339),
				)
			}
			count++
		}

		if serr := db.SaveSyncState(arc.Slug, "", count); serr != nil {
			fmt.Fprintf(os.Stderr, "  sync_state %s: %v\n", arc.Slug, serr)
		}

		if !flags.asJSON {
			fmt.Fprintf(out, "  %d record\n", count)
		}
		results = append(results, archiveResult{Archivio: arc.Slug, Records: count})

		// Pausa cortese tra archivi per non sovraccaricare il portale.
		if len(archives) > 1 {
			time.Sleep(500 * time.Millisecond)
		}
	}

	if flags.asJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(results)
	}
	return nil
}

// flattenRecord produce la mappa flat (stessa forma di emitRecords) e l'ID stabile.
func flattenRecord(arc icaro.Archive, r icaro.Record) (id string, flat map[string]any) {
	flat = map[string]any{
		"doc_id":  r.DocID,
		"title":   r.Title,
		"excerpt": r.Excerpt,
		"url":     r.URL,
	}
	for k, v := range r.Fields {
		flat[strings.ToLower(strings.TrimSuffix(k, "."))] = v
	}
	id = deriveSyncID(arc.Slug, flat)
	// Inietta "id" nel JSON così è presente per eventuali query future.
	flat["id"] = id
	return
}

// deriveSyncID costruisce una chiave stabile per ogni archivio.
func deriveSyncID(slug string, flat map[string]any) string {
	get := func(k string) string {
		if v, ok := flat[k]; ok {
			s := fmt.Sprint(v)
			return strings.TrimSpace(s)
		}
		return ""
	}
	switch slug {
	case "leggi":
		// Columns: Legisl., Atto, Docum., Data, Titolo
		return fmt.Sprintf("%s-%s", get("legisl"), get("atto"))
	case "convocazioni":
		// Columns: Legisl., Commissione, Data, ODG — nessun numero univoco
		return fmt.Sprintf("%s-%s-%s", get("legisl"), get("commissione"), get("data"))
	case "sommari":
		// Columns: Legisl., Commissione, Data, Numero, Argomenti
		return fmt.Sprintf("%s-%s-%s", get("legisl"), get("commissione"), get("numero"))
	case "biblioteca":
		// Columns: Autore, Titolo, Anno
		return fmt.Sprintf("%s-%s", get("autore"), get("titolo"))
	default:
		// ddl, resoconti, pareri, interrogazioni, interpellanze, mozioni, odg, risoluzioni
		// Hanno tutte Legisl. + Numero
		return fmt.Sprintf("%s-%s", get("legisl"), get("numero"))
	}
}

// filterArchives restituisce gli archivi il cui slug compare nel filtro (case-insensitive).
// Se il filtro è vuoto, restituisce tutti.
func filterArchives(all []icaro.Archive, filter []string) []icaro.Archive {
	if len(filter) == 0 {
		return all
	}
	set := make(map[string]struct{}, len(filter))
	for _, f := range filter {
		set[strings.ToLower(strings.TrimSpace(f))] = struct{}{}
	}
	var result []icaro.Archive
	for _, arc := range all {
		if _, ok := set[arc.Slug]; ok {
			result = append(result, arc)
		}
	}
	return result
}
