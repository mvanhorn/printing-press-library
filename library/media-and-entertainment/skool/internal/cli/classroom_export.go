// Copyright 2026 quoxientzero. Licensed under Apache-2.0. See LICENSE.
// Hand-written novel feature; not generated.

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// newClassroomExportCmd: exports a Skool course to a markdown bundle.
// Walks pageProps.course.modules[*].lessons[*] and writes one .md per lesson.
func newClassroomExportCmd(flags *rootFlags) *cobra.Command {
	var flagCommunity string
	var flagOut string

	cmd := &cobra.Command{
		Use:   "export <course-slug>",
		Short: "Export a course to a folder of markdown files (one per lesson)",
		Example: "  skool-pp-cli classroom export ai-foundations --community bewarethedefault --out ./course/",
		Annotations: map[string]string{"mcp:read-only": "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRunOK(flags) {
				return nil
			}
			if len(args) == 0 {
				return cmd.Help()
			}
			courseSlug := strings.TrimSpace(args[0])
			c, err := flags.newClient()
			if err != nil {
				return err
			}
			community := strings.TrimSpace(flagCommunity)
			if community == "" && c.Config != nil {
				community = c.Config.TemplateVars["community"]
			}
			if community == "" {
				return usageErr(fmt.Errorf("--community is required"))
			}
			outDir := flagOut
			if outDir == "" {
				outDir = "./" + courseSlug + "/"
			}
			if err := os.MkdirAll(outDir, 0o755); err != nil {
				return fmt.Errorf("creating out dir: %w", err)
			}

			path := "/_next/data/{buildId}/" + community + "/classroom/" + courseSlug + ".json"
			params := map[string]string{
				"g":  community,
				"md": courseSlug,
			}
			raw, err := c.Get(path, params)
			if err != nil {
				return classifyAPIError(err, flags)
			}

			var env struct {
				PageProps struct {
					Course struct {
						ID       string `json:"id"`
						Name     string `json:"name"`
						Modules []struct {
							ID      string `json:"id"`
							Name    string `json:"name"`
							Lessons []struct {
								ID       string         `json:"id"`
								Name     string         `json:"name"`
								Metadata map[string]any `json:"metadata"`
							} `json:"lessons"`
						} `json:"modules"`
					} `json:"course"`
				} `json:"pageProps"`
			}
			if err := json.Unmarshal(raw, &env); err != nil {
				return fmt.Errorf("parsing course response: %w", err)
			}
			course := env.PageProps.Course

			files := make([]string, 0, 32)
			indexLines := []string{"# " + course.Name, ""}
			for _, m := range course.Modules {
				modSlug := slugify(m.Name)
				modDir := filepath.Join(outDir, modSlug)
				if err := os.MkdirAll(modDir, 0o755); err != nil {
					return fmt.Errorf("creating module dir: %w", err)
				}
				indexLines = append(indexLines, "## "+m.Name)
				for _, l := range m.Lessons {
					lessonSlug := slugify(l.Name)
					filename := filepath.Join(modDir, lessonSlug+".md")
					body := lessonMarkdown(l.Name, l.ID, l.Metadata)
					if err := os.WriteFile(filename, []byte(body), 0o644); err != nil {
						return fmt.Errorf("writing %s: %w", filename, err)
					}
					files = append(files, filename)
					indexLines = append(indexLines, "- ["+l.Name+"]("+modSlug+"/"+lessonSlug+".md)")
				}
				indexLines = append(indexLines, "")
			}
			indexPath := filepath.Join(outDir, "index.md")
			if err := os.WriteFile(indexPath, []byte(strings.Join(indexLines, "\n")), 0o644); err != nil {
				return fmt.Errorf("writing index: %w", err)
			}
			files = append(files, indexPath)

			out := map[string]any{
				"course_slug": courseSlug,
				"course_name": course.Name,
				"out_dir":     outDir,
				"file_count":  len(files),
				"files":       files,
			}
			if flags.asJSON {
				return printJSONFiltered(cmd.OutOrStdout(), out, flags)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote %d files to %s\n", len(files), outDir)
			return nil
		},
	}
	cmd.Flags().StringVar(&flagCommunity, "community", "", "Community slug (defaults to template_vars.community)")
	cmd.Flags().StringVar(&flagOut, "out", "", "Output directory (default ./<course-slug>/)")
	return cmd
}

func lessonMarkdown(title, id string, metadata map[string]any) string {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "lesson_id: %s\n", id)
	fmt.Fprintf(&b, "title: %q\n", title)
	if v, ok := metadata["videoLink"]; ok {
		fmt.Fprintf(&b, "video_link: %q\n", fmt.Sprint(v))
	}
	if v, ok := metadata["videoLinkData"]; ok {
		if vm, ok := v.(map[string]any); ok {
			if mux, ok := vm["mux"]; ok {
				fmt.Fprintf(&b, "mux_url: %q\n", fmt.Sprint(mux))
			}
		}
	}
	if v, ok := metadata["attachments"]; ok {
		if s, ok := v.(string); ok && s != "" {
			fmt.Fprintf(&b, "attachments: %s\n", s)
		}
	}
	b.WriteString("---\n\n")
	b.WriteString("# " + title + "\n\n")
	if c, ok := metadata["content"].(string); ok && c != "" {
		b.WriteString(c)
		b.WriteString("\n")
	} else if d, ok := metadata["description"].(string); ok && d != "" {
		b.WriteString(d)
		b.WriteString("\n")
	} else {
		b.WriteString("_(no body content extracted; lesson body may be inside Mux video)_\n")
	}
	return b.String()
}

func slugify(name string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_' || r == '/':
			b.WriteByte('-')
		}
	}
	s := b.String()
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	s = strings.Trim(s, "-")
	if s == "" {
		s = "untitled"
	}
	return s
}
