package cli

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/mvanhorn/printing-press-library/library/media-and-entertainment/magnific/internal/store"
)

func newMagnificStockCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stock",
		Short: "Local index of Magnific stock-content downloads (icons, videos, resources)",
		Long: `stock provides offline search over the icons/videos/resources you
have downloaded locally. The Magnific stock catalog is 250M+ assets so we
do not pre-sync it; instead, point 'stock library index' at the directory
where your downloads live and the FTS5 index lets you grep your own
working set without an API call.`,
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	sub := &cobra.Command{
		Use:         "library",
		Short:       "Manage the local stock-library FTS index",
		Annotations: map[string]string{"mcp:read-only": "true"},
	}
	sub.AddCommand(newMagnificStockIndexCmd(flags))
	sub.AddCommand(newMagnificStockSearchCmd(flags))
	cmd.AddCommand(sub)
	return cmd
}

func newMagnificStockIndexCmd(flags *rootFlags) *cobra.Command {
	var dir string
	var kindHint string
	cmd := &cobra.Command{
		Use:     "index",
		Short:   "Walk a directory of downloaded stock files into the local FTS index",
		Example: "  magnific-pp-cli stock library index --dir ~/Downloads/magnific --kind icon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if dir == "" {
				dir = filepath.Join(os.Getenv("HOME"), "Downloads")
			}
			ctx := cmd.Context()
			db, err := store.OpenWithContext(ctx, defaultDBPath("magnific-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := store.EnsureMagnificTables(ctx, db.DB()); err != nil {
				return fmt.Errorf("initializing magnific tables: %w", err)
			}
			indexed := 0
			err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
				if err != nil {
					return nil // skip unreadable
				}
				if info.IsDir() {
					return nil
				}
				name := info.Name()
				ext := strings.ToLower(filepath.Ext(name))
				kind := kindHint
				if kind == "" {
					switch ext {
					case ".svg":
						kind = "icon"
					case ".png", ".jpg", ".jpeg", ".webp":
						kind = "image"
					case ".mp4", ".mov", ".webm":
						kind = "video"
					case ".mp3", ".wav", ".flac":
						kind = "audio"
					default:
						return nil
					}
				}
				id := strings.TrimSuffix(name, ext)
				title := strings.ReplaceAll(strings.TrimSuffix(name, ext), "-", " ")
				_, err = db.DB().ExecContext(ctx, `
					INSERT INTO magnific_stock_library(id, kind, title, local_path, tags)
					VALUES (?, ?, ?, ?, ?)
					ON CONFLICT(id) DO UPDATE SET local_path=excluded.local_path, title=excluded.title, kind=excluded.kind`,
					id, kind, title, path, "")
				if err == nil {
					indexed++
				}
				return nil
			})
			if err != nil {
				return fmt.Errorf("walking %s: %w", dir, err)
			}
			return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
				"dir":     dir,
				"indexed": indexed,
			}, flags)
		},
	}
	cmd.Flags().StringVar(&dir, "dir", "", "Directory to index (default: ~/Downloads)")
	cmd.Flags().StringVar(&kindHint, "kind", "", "Force kind for every indexed file (icon|image|video|audio); auto-detect by extension when empty")
	return cmd
}

func newMagnificStockSearchCmd(flags *rootFlags) *cobra.Command {
	var limit int
	var kind string
	cmd := &cobra.Command{
		Use:         "search [query]",
		Short:       "FTS5 search the local stock-library index",
		Example:     "  magnific-pp-cli stock library search \"rocket\" --json",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			q := strings.TrimSpace(strings.Join(args, " "))
			if q == "" {
				return cmd.Help()
			}
			ctx := cmd.Context()
			db, err := store.OpenWithContext(ctx, defaultDBPath("magnific-pp-cli"))
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer db.Close()
			if err := store.EnsureMagnificTables(ctx, db.DB()); err != nil {
				return fmt.Errorf("initializing magnific tables: %w", err)
			}
			where := []string{"magnific_stock_library_fts MATCH ?"}
			args2 := []any{q}
			if kind != "" {
				where = append(where, "s.kind = ?")
				args2 = append(args2, kind)
			}
			args2 = append(args2, limit)
			query := fmt.Sprintf(`
				SELECT s.id, s.kind, COALESCE(s.title,''), COALESCE(s.local_path,''), COALESCE(s.indexed_at,'')
				FROM magnific_stock_library s
				JOIN magnific_stock_library_fts fts ON fts.rowid = s.rowid
				WHERE %s
				ORDER BY s.indexed_at DESC
				LIMIT ?`, strings.Join(where, " AND "))
			rows, err := db.DB().QueryContext(ctx, query, args2...)
			if err != nil {
				return fmt.Errorf("query: %w", err)
			}
			defer rows.Close()
			type row struct {
				ID        string `json:"id"`
				Kind      string `json:"kind"`
				Title     string `json:"title"`
				LocalPath string `json:"local_path"`
				IndexedAt string `json:"indexed_at"`
			}
			out := []row{}
			for rows.Next() {
				var r row
				var i, k, t, lp, ix sql.NullString
				if err := rows.Scan(&i, &k, &t, &lp, &ix); err != nil {
					continue
				}
				r.ID = i.String
				r.Kind = k.String
				r.Title = t.String
				r.LocalPath = lp.String
				r.IndexedAt = ix.String
				out = append(out, r)
			}
			return printJSONFiltered(cmd.OutOrStdout(), out, flags)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "Max results")
	cmd.Flags().StringVar(&kind, "kind", "", "Filter by kind (icon|image|video|audio)")
	return cmd
}
