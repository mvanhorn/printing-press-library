package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/mvanhorn/printing-press-library/library/productivity/obsidian/internal/cliutil"
)

func newEntityCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "entity",
		Short: "Entity-centric queries against the synced vault.",
	}
	cmd.AddCommand(newEntityDossierCmd(flags))
	return cmd
}

func newEntityDossierCmd(flags *rootFlags) *cobra.Command {
	var layer string
	cmd := &cobra.Command{
		Use:   "dossier [name-or-path]",
		Short: "Pack a single entity's note, frontmatter, facts, backlinks, and tags into one agent-readable block.",
		Long: "Resolves the entity by stem (e.g. 'Jeff Smith' or '[[Jeff Smith]]') or\n" +
			"vault path, then joins notes + frontmatter + facts + backlinks + tags\n" +
			"in one SQL pass. --layer description returns only path + description\n" +
			"for token-efficient agent reads.",
		Example:     "  obsidian-pp-cli entity dossier '[[Jeff Smith]]' --json\n  obsidian-pp-cli entity dossier 'People/Jeff Smith.md' --layer description",
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
			vc, err := openVaultAndStore(cmd.Context(), flags)
			if err != nil {
				return err
			}
			defer vc.Close()
			needle := strings.TrimSpace(args[0])
			needle = strings.TrimPrefix(needle, "[[")
			needle = strings.TrimSuffix(needle, "]]")
			// Resolve to a path: exact match, stem match, or LIKE.
			path := ""
			_ = vc.S.DB().QueryRowContext(cmd.Context(),
				`SELECT path FROM notes WHERE path = ?`, needle).Scan(&path)
			if path == "" {
				stem := needle + ".md"
				_ = vc.S.DB().QueryRowContext(cmd.Context(),
					`SELECT path FROM notes WHERE path LIKE ? ORDER BY LENGTH(path) LIMIT 1`,
					"%/"+stem).Scan(&path)
			}
			if path == "" {
				_ = vc.S.DB().QueryRowContext(cmd.Context(),
					`SELECT path FROM notes WHERE path LIKE ? ORDER BY LENGTH(path) LIMIT 1`,
					"%"+needle+"%").Scan(&path)
			}
			if path == "" {
				return notFoundErr(fmt.Errorf("no note matches %s", needle))
			}
			type dossierShape struct {
				Path              string            `json:"path"`
				Type              string            `json:"type,omitempty"`
				Date              string            `json:"date,omitempty"`
				Description       string            `json:"description,omitempty"`
				Status            string            `json:"status,omitempty"`
				Layer             string            `json:"layer,omitempty"`
				Tags              []string          `json:"tags,omitempty"`
				FrontmatterFields map[string]string `json:"frontmatter_fields,omitempty"`
				Facts             []dossierFact     `json:"facts,omitempty"`
				Backlinks         []dossierLink     `json:"backlinks,omitempty"`
				Outgoing          []dossierLink     `json:"outgoing,omitempty"`
			}
			d := dossierShape{Path: path, FrontmatterFields: map[string]string{}}
			_ = vc.S.DB().QueryRowContext(cmd.Context(),
				`SELECT COALESCE(type,''), COALESCE(date,''), COALESCE(description,''), COALESCE(status,''), COALESCE(layer,'')
				 FROM notes WHERE path = ?`, path).Scan(&d.Type, &d.Date, &d.Description, &d.Status, &d.Layer)

			if layer == "description" {
				if flags.asJSON {
					return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]string{
						"path":        d.Path,
						"description": d.Description,
					})
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", d.Path, d.Description)
				return nil
			}

			// Tags
			tagRows, _ := vc.S.DB().QueryContext(cmd.Context(), `SELECT tag FROM tags WHERE path = ? ORDER BY tag`, path)
			if tagRows != nil {
				for tagRows.Next() {
					var t string
					if err := tagRows.Scan(&t); err == nil {
						d.Tags = append(d.Tags, t)
					}
				}
				tagRows.Close()
			}
			// Frontmatter extras
			fmRows, _ := vc.S.DB().QueryContext(cmd.Context(), `SELECT key, COALESCE(value,'') FROM frontmatter_fields WHERE path = ?`, path)
			if fmRows != nil {
				for fmRows.Next() {
					var k, v string
					if err := fmRows.Scan(&k, &v); err == nil {
						d.FrontmatterFields[k] = v
					}
				}
				fmRows.Close()
			}
			// Facts
			factRows, _ := vc.S.DB().QueryContext(cmd.Context(),
				`SELECT id, fact, COALESCE(category,''), COALESCE(timestamp,''), COALESCE(status,''), COALESCE(source,''), COALESCE(decision_trace_id,''), storage
				 FROM facts WHERE parent_note_path = ? ORDER BY timestamp DESC, id`, path)
			if factRows != nil {
				for factRows.Next() {
					var f dossierFact
					if err := factRows.Scan(&f.ID, &f.Fact, &f.Category, &f.Timestamp, &f.Status, &f.Source, &f.DecisionTraceID, &f.Storage); err == nil {
						d.Facts = append(d.Facts, f)
					}
				}
				factRows.Close()
			}
			// Backlinks
			blRows, _ := vc.S.DB().QueryContext(cmd.Context(),
				`SELECT from_path, to_target FROM links WHERE resolved_path = ? ORDER BY from_path`, path)
			if blRows != nil {
				for blRows.Next() {
					var l dossierLink
					if err := blRows.Scan(&l.Path, &l.Target); err == nil {
						d.Backlinks = append(d.Backlinks, l)
					}
				}
				blRows.Close()
			}
			// Outgoing
			outRows, _ := vc.S.DB().QueryContext(cmd.Context(),
				`SELECT to_target, COALESCE(resolved_path,'') FROM links WHERE from_path = ? ORDER BY to_target`, path)
			if outRows != nil {
				for outRows.Next() {
					var l dossierLink
					if err := outRows.Scan(&l.Target, &l.Path); err == nil {
						d.Outgoing = append(d.Outgoing, l)
					}
				}
				outRows.Close()
			}

			if flags.asJSON {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(d)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s\n  type: %s  date: %s  status: %s  layer: %s\n  description: %s\n",
				d.Path, d.Type, d.Date, d.Status, d.Layer, d.Description)
			if len(d.Tags) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  tags: %s\n", strings.Join(d.Tags, ", "))
			}
			if len(d.FrontmatterFields) > 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "  fields:")
				for k, v := range d.FrontmatterFields {
					fmt.Fprintf(cmd.OutOrStdout(), "    %s=%s\n", k, v)
				}
			}
			if len(d.Facts) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  facts (%d):\n", len(d.Facts))
				for _, f := range d.Facts {
					fmt.Fprintf(cmd.OutOrStdout(), "    [%s] %s: %s\n", f.Storage, f.ID, f.Fact)
				}
			}
			if len(d.Backlinks) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  backlinks (%d):\n", len(d.Backlinks))
				for _, l := range d.Backlinks {
					fmt.Fprintf(cmd.OutOrStdout(), "    %s\n", l.Path)
				}
			}
			if len(d.Outgoing) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  outgoing (%d):\n", len(d.Outgoing))
				for _, l := range d.Outgoing {
					fmt.Fprintf(cmd.OutOrStdout(), "    [[%s]] -> %s\n", l.Target, l.Path)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&layer, "layer", "", "Progressive disclosure layer (description = path + description only)")
	return cmd
}

type dossierFact struct {
	ID              string `json:"id"`
	Fact            string `json:"fact"`
	Category        string `json:"category,omitempty"`
	Timestamp       string `json:"timestamp,omitempty"`
	Status          string `json:"status,omitempty"`
	Source          string `json:"source,omitempty"`
	DecisionTraceID string `json:"decision_trace_id,omitempty"`
	Storage         string `json:"storage"`
}

type dossierLink struct {
	Path   string `json:"path,omitempty"`
	Target string `json:"target,omitempty"`
}
