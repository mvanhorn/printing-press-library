package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/cliutil"
	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/vault"
)

func newNoteCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "note",
		Short: "Create, read, update, and delete notes in the vault.",
		Long: "Note CRUD with three-layer-protocol frontmatter enforcement. Every\n" +
			"write goes through the protocol validator; writes that would create\n" +
			"a non-conforming note are refused (use --force only when you really\n" +
			"need to).",
	}
	cmd.AddCommand(newNoteListCmd(flags))
	cmd.AddCommand(newNoteGetCmd(flags))
	cmd.AddCommand(newNoteNewCmd(flags))
	cmd.AddCommand(newNoteSetCmd(flags))
	cmd.AddCommand(newNoteRmCmd(flags))
	cmd.AddCommand(newNoteMvCmd(flags))
	cmd.AddCommand(newNoteOpenCmd(flags))
	return cmd
}

func newNoteListCmd(flags *rootFlags) *cobra.Command {
	var folder, typeFilter, statusFilter, tagFilter string
	var limit int
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List indexed notes (filterable by folder, type, status, tag).",
		Example: "  obsidian-pp-cli note list --folder People/ --json\n" +
			"  obsidian-pp-cli note list --type meeting --status active --limit 20",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}
			vc, err := openVaultAndStore(cmd.Context(), flags)
			if err != nil {
				return err
			}
			defer vc.Close()
			where := []string{}
			vals := []interface{}{}
			if folder != "" {
				where = append(where, "path LIKE ?")
				vals = append(vals, strings.TrimRight(folder, "/")+"/%")
			}
			if typeFilter != "" {
				where = append(where, "type = ?")
				vals = append(vals, typeFilter)
			}
			if statusFilter != "" {
				where = append(where, "status = ?")
				vals = append(vals, statusFilter)
			}
			q := "SELECT path, COALESCE(type,''), COALESCE(date,''), COALESCE(description,''), COALESCE(status,'') FROM notes"
			if tagFilter != "" {
				q = "SELECT n.path, COALESCE(n.type,''), COALESCE(n.date,''), COALESCE(n.description,''), COALESCE(n.status,'') FROM notes n JOIN tags t ON t.path = n.path WHERE t.tag = ?"
				vals = append([]interface{}{tagFilter}, vals...)
				if len(where) > 0 {
					q += " AND " + strings.Join(where, " AND ")
				}
				q += " GROUP BY n.path"
			} else if len(where) > 0 {
				q += " WHERE " + strings.Join(where, " AND ")
			}
			q += " ORDER BY path"
			if limit > 0 {
				q += " LIMIT ?"
				vals = append(vals, limit)
			}
			rows, err := vc.S.DB().QueryContext(cmd.Context(), q, vals...)
			if err != nil {
				return apiErr(err)
			}
			defer rows.Close()
			type entry struct {
				Path        string `json:"path"`
				Type        string `json:"type,omitempty"`
				Date        string `json:"date,omitempty"`
				Description string `json:"description,omitempty"`
				Status      string `json:"status,omitempty"`
			}
			var out []entry
			for rows.Next() {
				var e entry
				if err := rows.Scan(&e.Path, &e.Type, &e.Date, &e.Description, &e.Status); err != nil {
					return apiErr(err)
				}
				out = append(out, e)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(out)
			}
			for _, e := range out {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", e.Path, e.Type, e.Description)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&folder, "folder", "", "Filter to notes under this vault folder")
	cmd.Flags().StringVar(&typeFilter, "type", "", "Filter by frontmatter type")
	cmd.Flags().StringVar(&statusFilter, "status", "", "Filter by frontmatter status")
	cmd.Flags().StringVar(&tagFilter, "tag", "", "Filter by tag (frontmatter or inline)")
	cmd.Flags().IntVar(&limit, "limit", 0, "Max rows to return (0 = no limit)")
	return cmd
}

func newNoteGetCmd(flags *rootFlags) *cobra.Command {
	var fmOnly, bodyOnly bool
	var layer string
	cmd := &cobra.Command{
		Use:         "get [path]",
		Short:       "Read a single note from disk.",
		Example:     "  obsidian-pp-cli note get 'People/Jeff Smith.md'\n  obsidian-pp-cli note get 'People/Jeff Smith.md' --layer description",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}
			v, _, err := openVaultOnly()
			if err != nil {
				return err
			}
			abs, err := v.ResolveAbs(args[0])
			if err != nil {
				return notFoundErr(err)
			}
			if _, err := os.Stat(abs); err != nil {
				return notFoundErr(fmt.Errorf("note not found: %s", args[0]))
			}
			n, err := v.LoadNote(abs)
			if err != nil {
				return apiErr(err)
			}
			if layer == "description" {
				if flags.asJSON {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]string{
						"path":        n.Path,
						"description": n.Frontmatter.Description,
					})
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", n.Path, n.Frontmatter.Description)
				return nil
			}
			if fmOnly {
				if flags.asJSON {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(n.Frontmatter)
				}
				enc, _ := n.Frontmatter.Encode()
				fmt.Fprint(cmd.OutOrStdout(), string(enc))
				return nil
			}
			if bodyOnly {
				fmt.Fprint(cmd.OutOrStdout(), n.Body)
				return nil
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
					"path":        n.Path,
					"frontmatter": n.Frontmatter,
					"body":        n.Body,
				})
			}
			data, _ := os.ReadFile(abs)
			fmt.Fprint(cmd.OutOrStdout(), string(data))
			return nil
		},
	}
	cmd.Flags().BoolVar(&fmOnly, "frontmatter-only", false, "Print only the frontmatter")
	cmd.Flags().BoolVar(&bodyOnly, "body-only", false, "Print only the body")
	cmd.Flags().StringVar(&layer, "layer", "", "Progressive disclosure layer (description = path + description only)")
	return cmd
}

var slugRe = regexp.MustCompile(`[^A-Za-z0-9\- _]+`)

func newNoteNewCmd(flags *rootFlags) *cobra.Command {
	var noteType, title, folder, descFlag, statusFlag, dateFlag, body string
	var bodyStdin bool
	var force bool
	cmd := &cobra.Command{
		Use:   "new",
		Short: "Create a new note with three-layer-protocol-compliant frontmatter.",
		Long: "Create a new note. Required: --type and --title. Description defaults\n" +
			"to the title if --description is not supplied; date defaults to today;\n" +
			"status defaults to 'active'. Refuses to write a note whose frontmatter\n" +
			"would fail the protocol lint (use --force to override at your own risk).",
		Example:     "  obsidian-pp-cli note new --type meeting --title '2026-05-15 Servosity sync'\n  echo 'Body...' | obsidian-pp-cli note new --type idea --title 'Latch on capture' --body-stdin",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if noteType == "" || title == "" {
				if cliutil.IsVerifyEnv() {
					return nil
				}
				if dryRunOK(flags) {
					return nil
				}
				return usageErr(fmt.Errorf("--type and --title are required"))
			}
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}
			v, cfg, err := openVaultOnly()
			if err != nil {
				return err
			}
			_ = cfg
			// Build relative path: <folder>/<safe-title>.md
			safe := slugify(title)
			rel := safe + ".md"
			if folder != "" {
				rel = filepath.Join(strings.TrimRight(folder, "/"), rel)
			}
			abs, err := v.ResolveAbs(rel)
			if err != nil {
				return usageErr(err)
			}
			if _, err := os.Stat(abs); err == nil {
				return usageErr(fmt.Errorf("note already exists at %s", rel))
			}
			fm := vault.Frontmatter{
				Type:        noteType,
				Date:        dateFlag,
				Description: descFlag,
				Status:      statusFlag,
				Extra:       map[string]interface{}{},
			}
			if fm.Date == "" {
				fm.Date = time.Now().Format("2006-01-02")
			}
			if fm.Status == "" {
				fm.Status = "active"
			}
			if fm.Description == "" {
				fm.Description = title
			}
			var bodyContent []byte
			if bodyStdin {
				bodyContent, _ = io.ReadAll(cmd.InOrStdin())
			} else {
				bodyContent = []byte(body)
			}
			if len(bodyContent) > 0 && !strings.HasPrefix(string(bodyContent), "\n") {
				bodyContent = append([]byte("\n"), bodyContent...)
			}
			// Validate before write.
			findings := vault.Validate(rel, fm, true)
			if hasErrors(findings) && !force {
				return usageErr(fmt.Errorf("validation failed for %s — fix or pass --force:\n%s", rel, formatFindings(findings)))
			}
			data, err := vault.AssembleNote(fm, bodyContent)
			if err != nil {
				return apiErr(err)
			}
			if err := vault.AtomicWrite(abs, data, 0o644); err != nil {
				return apiErr(err)
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]string{
					"path":   rel,
					"status": "created",
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "created: %s\n", rel)
			return nil
		},
	}
	cmd.Flags().StringVar(&noteType, "type", "", "Note type (person, company, meeting, journal, idea, ...)")
	cmd.Flags().StringVar(&title, "title", "", "Note title (becomes the filename stem)")
	cmd.Flags().StringVar(&folder, "folder", "", "Folder within the vault (e.g. People, Meetings)")
	cmd.Flags().StringVar(&descFlag, "description", "", "One-line description (defaults to title)")
	cmd.Flags().StringVar(&statusFlag, "status", "active", "Status: active, paused, completed, archived, superseded")
	cmd.Flags().StringVar(&dateFlag, "date", "", "Date in YYYY-MM-DD format (defaults to today)")
	cmd.Flags().StringVar(&body, "body", "", "Note body content")
	cmd.Flags().BoolVar(&bodyStdin, "body-stdin", false, "Read body from stdin")
	cmd.Flags().BoolVar(&force, "force", false, "Write even if protocol validation fails")
	return cmd
}

func newNoteSetCmd(flags *rootFlags) *cobra.Command {
	var body string
	var bodyStdin, force bool
	cmd := &cobra.Command{
		Use:         "set [path]",
		Short:       "Replace the body of an existing note, preserving frontmatter.",
		Example:     "  cat draft.md | obsidian-pp-cli note set 'Ideas/draft.md' --body-stdin",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}
			v, _, err := openVaultOnly()
			if err != nil {
				return err
			}
			abs, err := v.ResolveAbs(args[0])
			if err != nil {
				return notFoundErr(err)
			}
			n, err := v.LoadNote(abs)
			if err != nil {
				return notFoundErr(err)
			}
			var newBody []byte
			if bodyStdin {
				newBody, _ = io.ReadAll(cmd.InOrStdin())
			} else {
				newBody = []byte(body)
			}
			if !n.HasFM {
				if !force {
					return usageErr(fmt.Errorf("note has no frontmatter — refusing to overwrite; pass --force to replace the whole file"))
				}
				if err := vault.AtomicWrite(abs, newBody, 0o644); err != nil {
					return apiErr(err)
				}
			} else {
				findings := vault.Validate(n.Path, n.Frontmatter, true)
				if hasErrors(findings) && !force {
					return usageErr(fmt.Errorf("existing frontmatter has errors — fix first or pass --force:\n%s", formatFindings(findings)))
				}
				assembled, err := vault.AssembleNote(n.Frontmatter, newBody)
				if err != nil {
					return apiErr(err)
				}
				if err := vault.AtomicWrite(abs, assembled, 0o644); err != nil {
					return apiErr(err)
				}
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]string{"path": n.Path, "status": "updated"})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "updated: %s\n", n.Path)
			return nil
		},
	}
	cmd.Flags().StringVar(&body, "body", "", "Body content (use --body-stdin to read from stdin)")
	cmd.Flags().BoolVar(&bodyStdin, "body-stdin", false, "Read body from stdin")
	cmd.Flags().BoolVar(&force, "force", false, "Override safety checks (replace bodyless file, bypass validation)")
	return cmd
}

func newNoteRmCmd(flags *rootFlags) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:         "rm [path]",
		Short:       "Delete a note from the vault.",
		Example:     "  obsidian-pp-cli note rm 'Drafts/old.md' --dry-run",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}
			vc, err := openVaultAndStore(cmd.Context(), flags)
			if err != nil {
				return err
			}
			defer vc.Close()
			abs, err := vc.V.ResolveAbs(args[0])
			if err != nil {
				return notFoundErr(err)
			}
			if _, err := os.Stat(abs); err != nil {
				return notFoundErr(fmt.Errorf("note not found: %s", args[0]))
			}
			// Refuse if there are incoming links unless --force.
			var n int
			_ = vc.S.DB().QueryRowContext(cmd.Context(),
				`SELECT COUNT(*) FROM links WHERE resolved_path = ?`,
				args[0]).Scan(&n)
			if n > 0 && !force {
				return usageErr(fmt.Errorf("refusing to delete: %d notes link to %s (pass --force to delete anyway)", n, args[0]))
			}
			if err := os.Remove(abs); err != nil {
				return apiErr(err)
			}
			_ = vc.S.DeletePath(cmd.Context(), args[0])
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]string{"path": args[0], "status": "deleted"})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "deleted: %s\n", args[0])
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Delete even if other notes link to this one")
	return cmd
}

func newNoteMvCmd(flags *rootFlags) *cobra.Command {
	var skipLinks bool
	cmd := &cobra.Command{
		Use:         "mv [from] [to]",
		Short:       "Move or rename a note and rewrite all [[wikilinks]] pointing to it.",
		Example:     "  obsidian-pp-cli note mv 'People/Jeff.md' 'People/Jeff Smith.md'",
		Annotations: map[string]string{"mcp:read-only": "false"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return cmd.Help()
			}
			if cliutil.IsVerifyEnv() {
				return nil
			}
			if dryRunOK(flags) {
				return nil
			}
			vc, err := openVaultAndStore(cmd.Context(), flags)
			if err != nil {
				return err
			}
			defer vc.Close()
			fromAbs, err := vc.V.ResolveAbs(args[0])
			if err != nil {
				return notFoundErr(err)
			}
			toAbs, err := vc.V.ResolveAbs(args[1])
			if err != nil {
				return usageErr(err)
			}
			if _, err := os.Stat(toAbs); err == nil {
				return usageErr(fmt.Errorf("destination already exists: %s", args[1]))
			}
			if err := os.MkdirAll(filepath.Dir(toAbs), 0o755); err != nil {
				return apiErr(err)
			}
			if err := os.Rename(fromAbs, toAbs); err != nil {
				return apiErr(err)
			}
			rewritten := 0
			if !skipLinks {
				oldStem := strings.TrimSuffix(filepath.Base(args[0]), ".md")
				newStem := strings.TrimSuffix(filepath.Base(args[1]), ".md")
				rewritten, err = rewriteWikilinks(vc.V, oldStem, newStem)
				if err != nil {
					return apiErr(err)
				}
			}
			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]interface{}{
					"from":                args[0],
					"to":                  args[1],
					"wikilinks_rewritten": rewritten,
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "moved: %s -> %s (rewrote %d wikilinks)\n", args[0], args[1], rewritten)
			return nil
		},
	}
	cmd.Flags().BoolVar(&skipLinks, "skip-links", false, "Don't rewrite [[wikilinks]] pointing to the renamed note")
	return cmd
}

func newNoteOpenCmd(flags *rootFlags) *cobra.Command {
	var launch bool
	cmd := &cobra.Command{
		Use:         "open [path]",
		Short:       "Print the obsidian:// URI for a note (or launch it with --launch).",
		Example:     "  obsidian-pp-cli note open 'People/Jeff Smith.md'\n  obsidian-pp-cli note open 'People/Jeff Smith.md' --launch",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return nil
			}
			_, cfg, err := openVaultOnly()
			if err != nil {
				return err
			}
			vaultName := filepath.Base(cfg.VaultPath)
			notePath := strings.TrimSuffix(args[0], ".md")
			uri := fmt.Sprintf("obsidian://open?vault=%s&file=%s",
				escapeURI(vaultName), escapeURI(notePath))
			if cliutil.IsVerifyEnv() {
				fmt.Fprintln(cmd.OutOrStdout(), "would launch:", uri)
				return nil
			}
			if !launch {
				fmt.Fprintln(cmd.OutOrStdout(), uri)
				return nil
			}
			// macOS: `open <uri>`; Linux: xdg-open; Windows: start. Best-effort.
			return runOpen(uri)
		},
	}
	cmd.Flags().BoolVar(&launch, "launch", false, "Actually open the URI (default: print only)")
	return cmd
}

// escapeURI does minimal URI escaping for vault/file query params (spaces -> %20, & -> %26).
func escapeURI(s string) string {
	s = strings.ReplaceAll(s, "%", "%25")
	s = strings.ReplaceAll(s, "&", "%26")
	s = strings.ReplaceAll(s, " ", "%20")
	s = strings.ReplaceAll(s, "#", "%23")
	return s
}

func slugify(title string) string {
	t := strings.TrimSpace(title)
	t = slugRe.ReplaceAllString(t, "")
	return t
}

func hasErrors(findings []vault.Finding) bool {
	for _, f := range findings {
		if f.Severity == vault.SeverityError {
			return true
		}
	}
	return false
}

func formatFindings(findings []vault.Finding) string {
	var b strings.Builder
	for _, f := range findings {
		b.WriteString(fmt.Sprintf("  [%s] %s: %s\n", f.Severity, f.Rule, f.Message))
	}
	return b.String()
}

// rewriteWikilinks walks every .md file in the vault and rewrites [[oldStem]],
// [[oldStem|alias]], [[oldStem#section]] to use newStem. Returns the count of
// files modified.
func rewriteWikilinks(v *vault.Vault, oldStem, newStem string) (int, error) {
	count := 0
	pattern := regexp.MustCompile(`\[\[` + regexp.QuoteMeta(oldStem) + `((?:#[^\[\]\|]+)?(?:\|[^\[\]]+)?)\]\]`)
	err := v.Walk(func(n *vault.Note) error {
		data, err := os.ReadFile(n.AbsPath)
		if err != nil {
			return err
		}
		replaced := pattern.ReplaceAllString(string(data), "[["+newStem+"$1]]")
		if replaced != string(data) {
			if err := vault.AtomicWrite(n.AbsPath, []byte(replaced), 0o644); err != nil {
				return err
			}
			count++
		}
		return nil
	})
	return count, err
}
