// pp:data-source local
package cli

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/dropbox/internal/store"
	"github.com/spf13/cobra"
)

type treeNode struct {
	PathDisplay  string     `json:"path_display"`
	IndexMissing bool       `json:"index_missing,omitempty"`
	Note         string     `json:"note,omitempty"`
	Files        int        `json:"files"`
	Bytes        int64      `json:"bytes"`
	Children     []treeNode `json:"children"`
	More         int        `json:"more,omitempty"`
}

type treeTotals struct {
	files int
	bytes int64
}

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) { addNovelCommandIfAbsent(root, newTreeCmd(flags)) })
}
func newTreeCmd(flags *rootFlags) *cobra.Command {
	var dbPath string
	var depth, limit int
	cmd := &cobra.Command{
		Use: "tree [path]", Short: "Show indexed folder sizes as a tree",
		Long: "Inspect recursive file counts and bytes in a folder and its children using the local index.",
		Example: strings.Trim(`
  dropbox-pp-cli tree "/Camera Uploads" --depth 2 --agent
  dropbox-pp-cli tree "/Documents/Taxes" --agent`, "\n"),
		Args: cobra.MaximumNArgs(1), Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:happy-args": "path=/"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "tree")
			}
			if err := localSourceOnly(flags); err != nil {
				return err
			}
			if depth < 0 || limit < 0 {
				return usageErr(fmt.Errorf("--depth and --limit must be nonnegative"))
			}
			requested := "/"
			if len(args) > 0 && args[0] != "" {
				requested = args[0]
			}
			if !strings.HasPrefix(requested, "/") {
				return usageErr(fmt.Errorf("unknown path %q", requested))
			}
			db, found, err := openIndex(cmd, flags, dbPath)
			if err != nil {
				return err
			}
			result := treeNode{PathDisplay: requested, Children: make([]treeNode, 0)}
			if !found {
				result.IndexMissing = true
				result.Note = missingIndexNote
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			defer db.Close()
			result, ok, err := buildTreeSQL(cmd.Context(), db, requested, depth, limit)
			if err != nil {
				return err
			}
			if !ok {
				return usageErr(fmt.Errorf("unknown path %q", requested))
			}
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), result, flags)
			}
			printTree(cmd, result, 0)
			return nil
		},
	}
	cmd.Flags().StringVar(&dbPath, "db", "", "SQLite index path")
	cmd.Flags().IntVar(&depth, "depth", 2, "Folder levels to include")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum children per folder")
	return cmd
}

func buildTreeSQL(ctx context.Context, db *store.Store, requested string, depth, limit int) (treeNode, bool, error) {
	q := db.DB()
	key := strings.ToLower(strings.TrimSuffix(requested, "/"))
	if requested == "/" {
		key = ""
	}
	root := treeNode{PathDisplay: "/", Children: make([]treeNode, 0)}
	tag := "folder"
	if key != "" {
		err := q.QueryRowContext(ctx, `SELECT tag,COALESCE(NULLIF(path_display,''),path_lower) FROM dbx_files WHERE path_lower=?`, key).Scan(&tag, &root.PathDisplay)
		if err == sql.ErrNoRows {
			return treeNode{}, false, nil
		}
		if err != nil {
			return treeNode{}, false, err
		}
	}
	scope := ""
	args := []any{}
	if key != "" {
		lo, hi := descendantRange(key)
		scope = ` AND (path_lower=? OR (path_lower>=? AND path_lower<?))`
		args = []any{key, lo, hi}
	}
	if depth == 0 || tag == "file" || limit == 0 {
		if err := q.QueryRowContext(ctx, `SELECT COUNT(*),COALESCE(SUM(size),0) FROM dbx_files INDEXED BY dbx_files_file_path_size WHERE tag='file'`+scope, args...).Scan(&root.Files, &root.Bytes); err != nil {
			return treeNode{}, false, err
		}
		if limit == 0 && depth > 0 && tag != "file" {
			if err := q.QueryRowContext(ctx, `SELECT COUNT(*) FROM dbx_files WHERE parent_lower=?`, key).Scan(&root.More); err != nil {
				return treeNode{}, false, err
			}
		}
		return root, true, nil
	}
	// Read each file once and tally its ancestors up to the requested depth.
	// Repeating a grouped descendant query for every visible folder becomes
	// expensive when a single folder contains most of the index.
	rows, err := q.QueryContext(ctx, `SELECT path_lower,COALESCE(size,0) FROM dbx_files INDEXED BY dbx_files_file_path_size WHERE tag='file'`+scope, args...)
	if err != nil {
		return treeNode{}, false, err
	}
	totals := make(map[string]treeTotals)
	for rows.Next() {
		var path string
		var size int64
		if err := rows.Scan(&path, &size); err != nil {
			_ = rows.Close()
			return treeNode{}, false, err
		}
		root.Files++
		root.Bytes += size
		if path == key {
			continue
		}
		segmentStart := len(key) + 1
		for level := 0; level < depth && segmentStart < len(path); level++ {
			slash := strings.IndexByte(path[segmentStart:], '/')
			childPath := path
			if slash >= 0 {
				childPath = path[:segmentStart+slash]
			}
			total := totals[childPath]
			total.files++
			total.bytes += size
			totals[childPath] = total
			if slash < 0 {
				break
			}
			segmentStart += slash + 1
		}
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return treeNode{}, false, err
	}
	if err := fillTreeChildren(ctx, db, key, tag, depth, limit, totals, &root); err != nil {
		return treeNode{}, false, err
	}
	return root, true, nil
}

func fillTreeChildren(ctx context.Context, db *store.Store, parent, tag string, depth, limit int, totals map[string]treeTotals, node *treeNode) error {
	if depth == 0 || tag == "file" {
		return nil
	}
	q := db.DB()
	rows, err := q.QueryContext(ctx, `SELECT path_lower,COALESCE(NULLIF(path_display,''),path_lower),tag FROM dbx_files WHERE parent_lower=?`, parent)
	if err != nil {
		return err
	}
	type childRow struct {
		key, tag string
		node     treeNode
	}
	children := make([]childRow, 0)
	for rows.Next() {
		var c childRow
		if err := rows.Scan(&c.key, &c.node.PathDisplay, &c.tag); err != nil {
			_ = rows.Close()
			return err
		}
		total := totals[c.key]
		c.node.Files, c.node.Bytes = total.files, total.bytes
		c.node.Children = make([]treeNode, 0)
		children = append(children, c)
	}
	err = rows.Err()
	_ = rows.Close()
	if err != nil {
		return err
	}
	sort.Slice(children, func(i, j int) bool {
		if children[i].node.Bytes != children[j].node.Bytes {
			return children[i].node.Bytes > children[j].node.Bytes
		}
		return children[i].key < children[j].key
	})
	shown := len(children)
	if shown > limit {
		shown = limit
	}
	node.More = len(children) - shown
	for _, c := range children[:shown] {
		if err := fillTreeChildren(ctx, db, c.key, c.tag, depth-1, limit, totals, &c.node); err != nil {
			return err
		}
		node.Children = append(node.Children, c.node)
	}
	return nil
}
func printTree(cmd *cobra.Command, n treeNode, indent int) {
	fmt.Fprintf(cmd.OutOrStdout(), "%s%s (%d files, %d bytes)\n", strings.Repeat("  ", indent), n.PathDisplay, n.Files, n.Bytes)
	for _, child := range n.Children {
		printTree(cmd, child, indent+1)
	}
	if n.More > 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "%s... %d more\n", strings.Repeat("  ", indent+1), n.More)
	}
}
