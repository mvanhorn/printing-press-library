// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// pp:data-source local

package cli

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// newExportCmd writes the local library out. The mal-xml format is
// MyAnimeList's own list-import format, so a user can move a locally tracked
// library into their account without this CLI ever holding a token.
func newExportCmd(flags *rootFlags) *cobra.Command {
	var dbPath, kind, format, outPath string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export the local library as MyAnimeList import XML, JSON, or CSV",
		Long: "Use this command to move a locally tracked library into a MyAnimeList account.\n" +
			"The mal-xml format is MyAnimeList's own import schema; upload it at myanimelist.net/panel.php?go=import.",
		Example: "  myanimelist-pp-cli export --format mal-xml --out myanimelist.xml",
		Annotations: map[string]string{
			"mcp:read-only":       "true",
			"pp:data-source":      "local",
			"pp:happy-args":       "--kind=anime",
			"pp:typed-exit-codes": "0,3",
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "export")
			}
			switch format {
			case "mal-xml", "json", "csv":
			default:
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("--format must be mal-xml, json, or csv; got %q", format))
			}
			ctx, cancel := boundCtx(cmd.Context(), flags)
			defer cancel()
			dbPath = malDBPath(flags, dbPath)
			entries := make([]malLibraryEntry, 0)
			if malStoreExists(dbPath) {
				db, err := malOpenStore(ctx, dbPath)
				if err != nil {
					return err
				}
				defer db.Close()
				entries, err = malLoadLibrary(ctx, db, kind)
				if err != nil {
					return err
				}
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "no local mirror at %s; exporting an empty library\n", dbPath)
			}
			var payload []byte
			var err error
			switch format {
			case "mal-xml":
				payload, err = renderMALXML(entries, kind)
			case "json":
				payload, err = json.MarshalIndent(entries, "", "  ")
			case "csv":
				payload, err = renderCSV(entries)
			}
			if err != nil {
				return err
			}
			if outPath != "" && outPath != "-" {
				if werr := os.WriteFile(outPath, payload, 0o600); werr != nil {
					return fmt.Errorf("writing %s: %w", outPath, werr)
				}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{"written": outPath, "entries": len(entries), "format": format}, flags)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "wrote %d entries to %s (%s)\n", len(entries), outPath, format)
				return nil
			}
			if _, err := cmd.OutOrStdout().Write(payload); err != nil {
				return err
			}
			if len(payload) == 0 || payload[len(payload)-1] != '\n' {
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite database file path")
	cmd.Flags().StringVar(&kind, "kind", "", "Only export this kind (anime or manga)")
	cmd.Flags().StringVar(&format, "format", "mal-xml", "Output format: mal-xml, json, or csv")
	cmd.Flags().StringVar(&outPath, "out", "-", "Write to this file instead of stdout (use - for stdout)")
	return cmd
}

var malXMLStatus = map[string]string{
	"watching":      "Watching",
	"completed":     "Completed",
	"on-hold":       "On-Hold",
	"dropped":       "Dropped",
	"plan-to-watch": "Plan to Watch",
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&apos;")
	return r.Replace(s)
}

// renderMALXML emits MyAnimeList's own list-import schema.
func renderMALXML(entries []malLibraryEntry, kind string) ([]byte, error) {
	anime := make([]malLibraryEntry, 0)
	manga := make([]malLibraryEntry, 0)
	for _, e := range entries {
		if e.Kind == "manga" {
			manga = append(manga, e)
			continue
		}
		anime = append(anime, e)
	}
	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<myanimelist>\n")
	b.WriteString("  <myinfo>\n    <user_id>0</user_id>\n    <user_name>local</user_name>\n")
	b.WriteString("    <user_export_type>" + strconv.Itoa(exportType(kind)) + "</user_export_type>\n")
	b.WriteString("    <user_total_anime>" + strconv.Itoa(len(anime)) + "</user_total_anime>\n")
	b.WriteString("    <user_total_watching>" + strconv.Itoa(countStatus(anime, "watching")) + "</user_total_watching>\n")
	b.WriteString("    <user_total_completed>" + strconv.Itoa(countStatus(anime, "completed")) + "</user_total_completed>\n")
	b.WriteString("    <user_total_onhold>" + strconv.Itoa(countStatus(anime, "on-hold")) + "</user_total_onhold>\n")
	b.WriteString("    <user_total_dropped>" + strconv.Itoa(countStatus(anime, "dropped")) + "</user_total_dropped>\n")
	b.WriteString("    <user_total_plantowatch>" + strconv.Itoa(countStatus(anime, "plan-to-watch")) + "</user_total_plantowatch>\n")
	b.WriteString("  </myinfo>\n")
	for _, e := range anime {
		b.WriteString("  <anime>\n")
		fmt.Fprintf(&b, "    <series_animedb_id>%d</series_animedb_id>\n", e.ID)
		fmt.Fprintf(&b, "    <series_title><![CDATA[%s]]></series_title>\n", strings.ReplaceAll(e.Title, "]]>", "]]&gt;"))
		fmt.Fprintf(&b, "    <series_episodes>%d</series_episodes>\n", e.Total)
		fmt.Fprintf(&b, "    <my_watched_episodes>%d</my_watched_episodes>\n", e.Progress)
		b.WriteString("    <my_start_date>0000-00-00</my_start_date>\n    <my_finish_date>0000-00-00</my_finish_date>\n")
		fmt.Fprintf(&b, "    <my_rated_score>%d</my_rated_score>\n", e.Score)
		fmt.Fprintf(&b, "    <my_status>%s</my_status>\n", malXMLStatus[e.Status])
		fmt.Fprintf(&b, "    <my_comments><![CDATA[%s]]></my_comments>\n", strings.ReplaceAll(e.Notes, "]]>", "]]&gt;"))
		b.WriteString("    <update_on_import>1</update_on_import>\n  </anime>\n")
	}
	for _, e := range manga {
		b.WriteString("  <manga>\n")
		fmt.Fprintf(&b, "    <manga_mangadb_id>%d</manga_mangadb_id>\n", e.ID)
		fmt.Fprintf(&b, "    <manga_title><![CDATA[%s]]></manga_title>\n", strings.ReplaceAll(e.Title, "]]>", "]]&gt;"))
		fmt.Fprintf(&b, "    <manga_chapters>%d</manga_chapters>\n", e.Total)
		fmt.Fprintf(&b, "    <my_read_chapters>%d</my_read_chapters>\n", e.Progress)
		b.WriteString("    <my_start_date>0000-00-00</my_start_date>\n    <my_finish_date>0000-00-00</my_finish_date>\n")
		fmt.Fprintf(&b, "    <my_rated_score>%d</my_rated_score>\n", e.Score)
		fmt.Fprintf(&b, "    <my_status>%s</my_status>\n", xmlEscape(malXMLStatus[e.Status]))
		fmt.Fprintf(&b, "    <my_comments><![CDATA[%s]]></my_comments>\n", strings.ReplaceAll(e.Notes, "]]>", "]]&gt;"))
		b.WriteString("    <update_on_import>1</update_on_import>\n  </manga>\n")
	}
	b.WriteString("</myanimelist>\n")
	return []byte(b.String()), nil
}

func exportType(kind string) int {
	if kind == "manga" {
		return 2
	}
	return 1
}

func countStatus(entries []malLibraryEntry, status string) int {
	n := 0
	for _, e := range entries {
		if e.Status == status {
			n++
		}
	}
	return n
}

func renderCSV(entries []malLibraryEntry) ([]byte, error) {
	var b strings.Builder
	w := csv.NewWriter(&b)
	if err := w.Write([]string{"kind", "id", "title", "status", "progress", "total", "score", "notes", "updated_at"}); err != nil {
		return nil, err
	}
	for _, e := range entries {
		row := []string{e.Kind, strconv.Itoa(e.ID), e.Title, e.Status, strconv.Itoa(e.Progress), strconv.Itoa(e.Total), strconv.Itoa(e.Score), e.Notes, e.UpdatedAt}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}
