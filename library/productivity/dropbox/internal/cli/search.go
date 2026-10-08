// pp:data-source local
package cli

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/dropbox"
	"github.com/spf13/cobra"
)

type searchEntry struct {
	Path           string `json:"path"`
	Tag            string `json:"tag"`
	Size           int64  `json:"size"`
	ClientModified string `json:"client_modified"`
}
type searchResult struct {
	Entries      []searchEntry `json:"entries"`
	IndexMissing bool          `json:"index_missing,omitempty"`
	Note         string        `json:"note,omitempty"`
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) { addNovelCommandIfAbsent(root, newSearchCmd(flags)) })
}
func newSearchCmd(flags *rootFlags) *cobra.Command {
	var dbPath, kind, under string
	var limit int
	cmd := &cobra.Command{
		Use: "search <query>", Short: "Search indexed Dropbox names and paths offline",
		Long: "Use this command for instant offline name and path search over the local index. Do NOT use it for searching inside file contents; use 'files search' instead.",
		Example: strings.Trim(`
  dropbox-pp-cli search invoice --agent
  dropbox-pp-cli search taxes --under "/Documents/Taxes" --type file --agent`, "\n"),
		Args: cobra.MaximumNArgs(1), Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "query=invoice"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "search")
			}
			if len(args) == 0 {
				_ = cmd.Usage()
				return usageErr(fmt.Errorf("search requires a query"))
			}
			if err := localSourceOnly(flags); err != nil {
				return err
			}
			if kind != "" && kind != "file" && kind != "folder" {
				return usageErr(fmt.Errorf("--type must be file or folder"))
			}
			if limit < 0 {
				return usageErr(fmt.Errorf("--limit must be nonnegative"))
			}
			match := ftsPrefixQuery(args[0])
			if match == "" {
				return usageErr(fmt.Errorf("query has no searchable tokens"))
			}
			db, found, err := openIndex(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			result := searchResult{Entries: make([]searchEntry, 0)}
			if !found {
				result.IndexMissing = true
				result.Note = missingIndexNote
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			defer db.Close()
			query := `SELECT COALESCE(f.path_display,f.path_lower),f.tag,COALESCE(f.size,0),COALESCE(f.client_modified,'') FROM dbx_files_fts JOIN dbx_files f ON f.rowid=dbx_files_fts.rowid WHERE dbx_files_fts MATCH ? AND (?='' OR f.tag=?) AND (?='' OR f.path_lower=? OR f.path_lower LIKE ? ESCAPE '\') ORDER BY f.path_lower LIMIT ?`
			scope := strings.ToLower(strings.TrimSuffix(under, "/"))
			if scope == "/" {
				scope = ""
			}
			rows, err := db.DB().QueryContext(cmd.Context(), query, match, kind, kind, scope, scope, dropbox.EscapeLike(scope)+"/%", limit)
			if err != nil {
				return err
			}
			for rows.Next() {
				var e searchEntry
				if err := rows.Scan(&e.Path, &e.Tag, &e.Size, &e.ClientModified); err != nil {
					_ = rows.Close()
					return err
				}
				result.Entries = append(result.Entries, e)
			}
			err = rows.Err()
			_ = rows.Close()
			if err != nil {
				return err
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			for _, e := range result.Entries {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%d bytes\n", e.Path, e.Tag, e.Size)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite index path")
	cmd.Flags().StringVar(&kind, "type", "", "Filter by file or folder")
	cmd.Flags().StringVar(&under, "under", "", "Restrict search to a Dropbox path")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum results")
	return cmd
}

func ftsPrefixQuery(input string) string {
	parts := strings.FieldsFunc(input, func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) })
	terms := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			terms = append(terms, `"`+part+`"*`)
		}
	}
	return strings.Join(terms, " AND ")
}
