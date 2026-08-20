#!/bin/bash
# Re-applies v0-pp-cli generated-file patches after a force regen wipes them.
# Patches are also recorded under .printing-press-patches/ for provenance.
set -euo pipefail
cd "${CLI_WORK_DIR:-$(pwd)}"

python3 - <<'PYEOF'
# --- chats_files.go --tree ---
p = 'internal/cli/chats_files.go'
src = open(p).read()
if '"tree"' not in src:
    src = src.replace(
        'func newChatsFilesCmd(flags *rootFlags) *cobra.Command {\n',
        'func newChatsFilesCmd(flags *rootFlags) *cobra.Command {\n\tvar tree bool\n',
    )
    src = src.replace(
        '''			outputData := data''',
        '''			if flags.dryRun && tree {
				return writeDryRun(cmd.OutOrStdout(), flags, "chats files --tree")
			}
			if tree {
				var items []struct {
					Path     string `json:"path"`
					Encoding string `json:"encoding"`
				}
				if uerr := json.Unmarshal(data, &items); uerr != nil {
					return fmt.Errorf("parsing files response: %w", uerr)
				}
				if !wantsHumanTable(cmd.OutOrStdout(), flags) {
					rows := make([]map[string]any, 0, len(items))
					for _, it := range items {
						rows = append(rows, map[string]any{"path": it.Path, "encoding": it.Encoding})
					}
					return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
				}
				if len(items) == 0 {
					fmt.Fprintln(cmd.OutOrStdout(), "No files in this chat.")
					return nil
				}
				printV0FileTree(cmd.OutOrStdout(), items)
				return nil
			}
			outputData := data''',
    )
    src = src.replace(
        '''		},
	}

	return cmd
}
''',
        '''		},
	}
	cmd.Flags().BoolVar(&tree, "tree", false, "Render the file paths as an indented directory tree")
	return cmd
}

// printV0FileTree renders file paths as an indented directory tree.
func printV0FileTree(w io.Writer, files []struct {
	Path     string `json:"path"`
	Encoding string `json:"encoding"`
}) {
	root := map[string]*v0TreeNode{}
	for _, f := range files {
		parts := splitV0Path(f.Path)
		node := root
		for _, part := range parts {
			if node[part] == nil {
				node[part] = &v0TreeNode{children: map[string]*v0TreeNode{}}
			}
			node = node[part].children
		}
	}
	var walk func(map[string]*v0TreeNode, string)
	walk = func(n map[string]*v0TreeNode, prefix string) {
		names := make([]string, 0, len(n))
		for name := range n {
			names = append(names, name)
		}
		sort.Strings(names)
		for i, name := range names {
			last := i == len(names)-1
			branch := "├── "
			if last {
				branch = "└── "
			}
			fmt.Fprintf(w, "%s%s%s\\n", prefix, branch, name)
			childPrefix := prefix
			if last {
				childPrefix += "    "
			} else {
				childPrefix += "│   "
			}
			walk(n[name].children, childPrefix)
		}
	}
	walk(root, "")
}

type v0TreeNode struct {
	children map[string]*v0TreeNode
}

func splitV0Path(p string) []string {
	var parts []string
	cur := ""
	for _, r := range p {
		if r == '/' {
			if cur != "" {
				parts = append(parts, cur)
				cur = ""
			}
			continue
		}
		cur += string(r)
	}
	if cur != "" {
		parts = append(parts, cur)
	}
	return parts
}
''',
    )
    if '"io"' not in src:
        src = src.replace('import (\n\t"encoding/json"\n\t"fmt"\n\t"os"\n', 'import (\n\t"encoding/json"\n\t"fmt"\n\t"io"\n\t"os"\n\t"sort"\n')
    open(p, 'w').write(src)
    print("--tree applied")

# --- chats_preview.go --url ---
p = 'internal/cli/chats_preview.go'
src = open(p).read()
if '"url"' not in src:
    src = src.replace(
        'func newChatsPreviewCmd(flags *rootFlags) *cobra.Command {\n',
        'func newChatsPreviewCmd(flags *rootFlags) *cobra.Command {\n\tvar urlOnly bool\n',
    )
    src = src.replace(
        '''			outputData := data''',
        '''			if flags.dryRun && urlOnly {
				return writeDryRun(cmd.OutOrStdout(), flags, "chats preview --url")
			}
			if urlOnly {
				var pv struct {
					URL string `json:"url"`
				}
				if json.Unmarshal(data, &pv) != nil || pv.URL == "" {
					return fmt.Errorf("preview response did not include a url")
				}
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]string{"url": pv.URL}, flags)
				}
				fmt.Fprintln(cmd.OutOrStdout(), pv.URL)
				return nil
			}
			outputData := data''',
    )
    src = src.replace(
        '''		},
	}

	return cmd
}
''',
        '''		},
	}
	cmd.Flags().BoolVar(&urlOnly, "url", false, "Print only the preview URL")
	return cmd
}
''',
    )
    open(p, 'w').write(src)
    print("--url applied")

# --- stream dry-run guards + binary json envelopes ---
for f in ['internal/cli/chats_createStream.go', 'internal/cli/messages_sendStream.go',
          'internal/cli/chats_resumeStream.go', 'internal/cli/messages_resolveTaskStream.go']:
    src = open(f).read()
    label = f.split('/')[-1].replace('.go','').replace('_',' ')
    if 'writeDryRun' not in src:
        old = '''			_ = json.Valid
			_ = os.Stderr
			_ = statusCode'''
        new = '''			if flags.dryRun {
				return writeDryRun(cmd.OutOrStdout(), flags, "%s")
			}
			_ = json.Valid
			_ = os.Stderr
			_ = statusCode''' % label
        if old in src:
            src = src.replace(old, new, 1)
    if 'binary response cannot be rendered as structured output",' not in src:
        old2 = '''			if flags.asJSON || flags.csv || flags.compact || flags.plain || flags.selectFields != "" {
				return fmt.Errorf("binary response cannot be rendered as structured output; redirect stdout or use --deliver file:<path>")
			}
			_, err = cmd.OutOrStdout().Write(data)
			return err'''
        new2 = '''			if flags.asJSON || flags.csv || flags.compact || flags.plain || flags.selectFields != "" {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"error": "binary response cannot be rendered as structured output",
						"hint":  "this is a streaming/binary endpoint; omit --json to stream raw",
						"bytes": len(data),
					}, flags)
				}
				return fmt.Errorf("binary response cannot be rendered as structured output; redirect stdout or use --deliver file:<path>")
			}
			_, err = cmd.OutOrStdout().Write(data)
			return err'''
        if old2 in src:
            src = src.replace(old2, new2, 1)
    open(f, 'w').write(src)
    print(f, "patched")

p = 'internal/cli/chats_downloadFiles.go'
src = open(p).read()
if 'writeDryRun' not in src:
    old = '''			_ = json.Valid
			_ = os.Stderr
			_ = prov'''
    new = '''			if flags.dryRun {
				return writeDryRun(cmd.OutOrStdout(), flags, "chats download-files")
			}
			_ = json.Valid
			_ = os.Stderr
			_ = prov'''
    if old in src:
        src = src.replace(old, new, 1)
if 'binary response cannot be rendered as structured output",' not in src:
    old2 = '''			if flags.asJSON || flags.csv || flags.compact || flags.plain || flags.selectFields != "" {
				return fmt.Errorf("binary response cannot be rendered as structured output; redirect stdout or use --deliver file:<path>")
			}
			_, err = cmd.OutOrStdout().Write(data)
			return err'''
    new2 = '''			if flags.asJSON || flags.csv || flags.compact || flags.plain || flags.selectFields != "" {
				if flags.asJSON {
					return printJSONFiltered(cmd.OutOrStdout(), map[string]any{
						"error":      "binary response cannot be rendered as structured output",
						"hint":       "redirect stdout to a file or use --deliver file:<path>",
						"bytes":      len(data),
					}, flags)
				}
				return fmt.Errorf("binary response cannot be rendered as structured output; redirect stdout or use --deliver file:<path>")
			}
			_, err = cmd.OutOrStdout().Write(data)
			return err'''
    if old2 in src:
        src = src.replace(old2, new2, 1)
open(p, 'w').write(src)
print("download-files patched")
PYEOF
go build -o build/stage/bin/v0-pp-cli ./cmd/v0-pp-cli && echo "built OK"
