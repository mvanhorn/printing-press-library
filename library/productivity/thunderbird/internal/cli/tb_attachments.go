// pp:data-source local

package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/store"
	"github.com/mvanhorn/printing-press-library/library/productivity/thunderbird/internal/tbprofile"
	"github.com/spf13/cobra"
)

func init() {
	registerNovelCommand(func(root *cobra.Command, flags *rootFlags) {
		addNovelCommandIfAbsent(root, newTBAttachmentsCmd(flags))
	})
}

// tbMessageAttachments returns the stored attachments of a message in index order.
func tbMessageAttachments(db *store.Store, messageID string) ([]tbAttachmentDoc, error) {
	docs, err := tbScanDocs[tbAttachmentDoc](db,
		`SELECT data FROM resources WHERE resource_type = 'attachments' AND id >= ? AND id < ?`, messageID+":", messageID+";")
	if err != nil {
		return nil, err
	}
	out := make([]tbAttachmentDoc, 0, len(docs))
	for _, d := range docs {
		if d.MessageID == messageID {
			out = append(out, d)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Index < out[j].Index })
	return out, nil
}

// tbAttInline falls back to a filename heuristic for docs synced before the inline field existed.
func tbAttInline(a tbAttachmentDoc) bool {
	if a.Inline != nil {
		return *a.Inline
	}
	return tbprofile.LikelyInline(a.Filename, a.ContentType)
}

// tbVisibleAttachments resolves inline on every doc and drops inline ones unless includeInline.
func tbVisibleAttachments(docs []tbAttachmentDoc, includeInline bool) []tbAttachmentDoc {
	out := make([]tbAttachmentDoc, 0, len(docs))
	for _, a := range docs {
		inline := tbAttInline(a)
		if inline && !includeInline {
			continue
		}
		a.Inline = &inline
		out = append(out, a)
	}
	return out
}

func newTBAttachmentsCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:         "attachments",
		Short:       "List and save the attachments of a message",
		Annotations: map[string]string{"mcp:read-only": "true", "pp:parent-group": "true", "pp:typed-exit-codes": "0,2", "pp:data-source": "local"},
		RunE:        parentNoSubcommandRunE(flags),
	}
	cmd.AddCommand(newTBAttachmentsListCmd(flags), newTBAttachmentsSaveCmd(flags))
	return cmd
}

func newTBAttachmentsListCmd(flags *rootFlags) *cobra.Command {
	var includeInline bool
	cmd := &cobra.Command{
		Use:   "list <message-id>",
		Short: "List the attachments of a message (index, filename, type, decoded size)",
		Long: `List the real attachments of a message. Inline parts (signature logos, images
embedded in the HTML body via cid:) are hidden unless --include-inline; indexes
are the original ones, so gaps mean hidden inline parts. Stores synced before
inline detection use a filename heuristic; sync --full recomputes it exactly.`,
		Example: strings.Trim(`
  thunderbird-pp-cli attachments list 3f9a1c2b7d4e
  thunderbird-pp-cli attachments list 3f9a1c2b7d4e --include-inline --json`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "true", "pp:data-source": "local", "pp:typed-exit-codes": "0,2,3", "pp:happy-args": "message-id=0123456789ab"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				return writeDryRun(cmd.OutOrStdout(), flags, "attachments list")
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("expected exactly one message id\nUsage: %s <message-id>", cmd.CommandPath()))
			}
			db, err := tbStoreFor(cmd, flags, "attachments")
			if err != nil || db == nil {
				return err
			}
			defer db.Close()
			d, err := tbGetMessage(db, args[0])
			if err != nil {
				return err
			}
			docs, err := tbMessageAttachments(db, d.ID)
			if err != nil {
				return err
			}
			rows := tbVisibleAttachments(docs, includeInline)
			if !wantsHumanTable(cmd.OutOrStdout(), flags) {
				return printJSONFiltered(cmd.OutOrStdout(), rows, flags)
			}
			tw := newTabWriter(tbHumanOut(cmd))
			fmt.Fprintln(tw, "INDEX\tFILENAME\tTYPE\tSIZE")
			for _, a := range rows {
				name := a.Filename
				if *a.Inline {
					name += " (inline)"
				}
				fmt.Fprintf(tw, "%d\t%s\t%s\t%s\n", a.Index, name, a.ContentType, tbHumanBytes(a.SizeBytes))
			}
			return tw.Flush()
		},
	}
	cmd.Flags().BoolVar(&includeInline, "include-inline", false, "Also list inline parts (signature logos, cid: images embedded in the body)")
	return cmd
}

type tbSavedAttachment struct {
	MessageID   string `json:"message_id"`
	Index       int    `json:"index"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	Path        string `json:"path"`
	Exists      bool   `json:"exists,omitempty"`
	Written     bool   `json:"written"`
}

type tbSavePlan struct {
	DryRun bool                `json:"dry_run"`
	Files  []tbSavedAttachment `json:"files"`
}

func newTBAttachmentsSaveCmd(flags *rootFlags) *cobra.Command {
	var outDir string
	var index int
	var force, includeInline bool
	cmd := &cobra.Command{
		Use:   "save <message-id>",
		Short: "Decode and save a message's attachments into a directory",
		Long: `Decode attachments from the original mbox bytes and write them into --output.
File names are sanitized (no path separators or reserved names); an existing
file is never overwritten unless --force is given. --dry-run prints the files
that would be written without touching the disk. Without --index only real
attachments are saved (inline signature logos and cid: images are skipped
unless --include-inline); --index N saves any part by its original index.`,
		Example: strings.Trim(`
  thunderbird-pp-cli attachments save 3f9a1c2b7d4e --output ./attachments
  thunderbird-pp-cli attachments save 3f9a1c2b7d4e --index 0 -o ./attachments --dry-run`, "\n"),
		Annotations: map[string]string{"mcp:read-only": "false", "pp:data-source": "local", "pp:typed-exit-codes": "0,2,3", "pp:happy-args": "message-id=0123456789ab"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && cmd.Flags().NFlag() == 0 {
				return cmd.Help()
			}
			if dryRunOK(flags) {
				if plan, err := tbPlanAttachmentSave(cmd, args, outDir, index, includeInline); err == nil {
					return tbPrintSave(cmd, flags, tbSavePlan{DryRun: true, Files: plan})
				}
				return writeDryRun(cmd.OutOrStdout(), flags, "attachments save")
			}
			if len(args) != 1 {
				return usageErr(fmt.Errorf("expected exactly one message id\nUsage: %s <message-id> --output <dir>", cmd.CommandPath()))
			}
			if strings.TrimSpace(outDir) == "" {
				return usageErr(fmt.Errorf("--output <dir> is required"))
			}
			db, err := tbStoreFor(cmd, flags, "attachments")
			if err != nil || db == nil {
				return err
			}
			d, err := tbGetMessage(db, args[0])
			_ = db.Close()
			if err != nil {
				return err
			}
			raw, err := tbReadRaw(d)
			if err != nil {
				return err
			}
			files, payloads, err := tbExtractForSave(d, raw, outDir, index, includeInline)
			if err != nil {
				return err
			}
			var clash []string
			for _, f := range files {
				if f.Exists && !force {
					clash = append(clash, f.Path)
				}
			}
			if len(clash) > 0 {
				return fmt.Errorf("refusing to overwrite existing file(s): %s (use --force)", strings.Join(clash, ", "))
			}
			if err := os.MkdirAll(outDir, 0o700); err != nil {
				return err
			}
			for i := range files {
				if err := tbWriteNewFile(files[i].Path, payloads[i], force); err != nil {
					return err
				}
				files[i].Written = true
			}
			return tbPrintSave(cmd, flags, tbSavePlan{Files: files})
		},
	}
	tbOutputDirFlag(cmd, &outDir, "Directory to write the attachments into (created if missing)")
	cmd.Flags().IntVar(&index, "index", -1, "Save only the attachment with this index (from attachments list)")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing files")
	cmd.Flags().BoolVar(&includeInline, "include-inline", false, "Without --index, also save inline parts (signature logos, cid: images)")
	return cmd
}

// tbPlanAttachmentSave is the read-only part of save for --dry-run.
func tbPlanAttachmentSave(cmd *cobra.Command, args []string, outDir string, index int, includeInline bool) ([]tbSavedAttachment, error) {
	if len(args) != 1 || strings.TrimSpace(outDir) == "" {
		return nil, errors.New("incomplete")
	}
	db, err := tbOpenStoreQuiet(cmd)
	if err != nil || db == nil {
		return nil, errors.New("no store")
	}
	d, err := tbGetMessage(db, args[0])
	_ = db.Close()
	if err != nil {
		return nil, err
	}
	raw, err := tbReadRaw(d)
	if err != nil {
		return nil, err
	}
	files, _, err := tbExtractForSave(d, raw, outDir, index, includeInline)
	return files, err
}

func tbExtractForSave(d *tbMessageDoc, raw []byte, outDir string, index int, includeInline bool) ([]tbSavedAttachment, [][]byte, error) {
	atts := tbprofile.ParseMessage(raw).Attachments
	count := len(atts)
	if count == 0 {
		return nil, nil, notFoundErr(fmt.Errorf("message %s has no attachments", d.ID))
	}
	var indexes []int
	if index >= 0 {
		if index >= count {
			return nil, nil, notFoundErr(fmt.Errorf("attachment index %d not found (message %s has %d attachments)", index, d.ID, count))
		}
		indexes = []int{index}
	} else {
		for _, a := range atts {
			if includeInline || !a.Inline {
				indexes = append(indexes, a.Index)
			}
		}
		if len(indexes) == 0 {
			return nil, nil, notFoundErr(fmt.Errorf("message %s has only inline parts (%d); use --include-inline or --index", d.ID, count))
		}
	}
	used := map[string]bool{}
	files := make([]tbSavedAttachment, 0, len(indexes))
	payloads := make([][]byte, 0, len(indexes))
	for _, i := range indexes {
		meta, data, err := tbprofile.ExtractAttachment(raw, i)
		if err != nil {
			return nil, nil, err
		}
		name := tbSafeFilename(meta.Filename, i)
		ext := filepath.Ext(name)
		stem := strings.TrimSuffix(name, ext)
		for n := 0; used[strings.ToLower(name)]; n++ {
			suffix := "-" + strconv.Itoa(i)
			if n > 0 {
				suffix += "-" + strconv.Itoa(n)
			}
			name = stem + suffix + ext
		}
		used[strings.ToLower(name)] = true
		p := filepath.Join(outDir, name)
		_, statErr := os.Lstat(p)
		files = append(files, tbSavedAttachment{
			MessageID: d.ID, Index: i, Filename: meta.Filename, ContentType: meta.ContentType,
			SizeBytes: int64(len(data)), Path: p, Exists: statErr == nil,
		})
		payloads = append(payloads, data)
	}
	return files, payloads, nil
}

var tbReservedNames = map[string]bool{"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true, "COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true, "LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true}

// tbSafeFilename keeps only the base name and drops characters that are
// invalid or dangerous on any supported OS.
func tbSafeFilename(name string, index int) string {
	name = path.Base(strings.ReplaceAll(name, `\`, "/"))
	name = strings.Map(func(r rune) rune {
		if r < 32 || r >= 0x7f && r <= 0x9f || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, name)
	name = strings.Trim(name, " .")
	if name == "" {
		return "attachment-" + strconv.Itoa(index)
	}
	stem := name
	if i := strings.IndexByte(stem, '.'); i >= 0 {
		stem = stem[:i]
	}
	if tbReservedNames[strings.ToUpper(strings.TrimSpace(stem))] {
		name = "_" + name
	}
	if len(name) > 200 {
		ext := filepath.Ext(name)
		if len(ext) > 20 {
			ext = ""
		}
		cut := 200 - len(ext)
		for cut > 0 && !isRuneStart(name[cut]) {
			cut--
		}
		name = name[:cut] + ext
	}
	return name
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }

// tbWriteNewFile creates path exclusively, so an existing file or symlink is never followed or truncated; force replaces a file or link, never a directory.
func tbWriteNewFile(p string, data []byte, force bool) error {
	p = filepath.Clean(p)
	if force {
		if fi, err := os.Lstat(p); err == nil {
			if fi.IsDir() {
				return fmt.Errorf("refusing to overwrite directory %s", p)
			}
			return tbReplaceFile(p, data)
		}
	}
	f, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("refusing to overwrite existing file(s): %s (use --force)", p)
	}
	if err != nil {
		return err
	}
	if err := tbWriteData(f, data); err != nil {
		_ = f.Close()
		_ = os.Remove(p)
		return err
	}
	return f.Close()
}

var tbRename = os.Rename

var tbWriteData = func(f *os.File, data []byte) error {
	_, err := f.Write(data)
	return err
}

// tbReplaceFile writes beside the target and renames over it, so a failed write keeps the original and a symlink is replaced, not followed.
func tbReplaceFile(p string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(p), "."+filepath.Base(p)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if err := tbWriteData(tmp, data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := tbRename(tmpName, p); err != nil {
		// Windows refuses to rename over a read-only file; os.Remove used to clear that bit implicitly.
		if fi, lerr := os.Lstat(p); lerr == nil && fi.Mode().IsRegular() && fi.Mode().Perm()&0o200 == 0 && os.Chmod(p, 0o600) == nil {
			if err = tbRename(tmpName, p); err != nil {
				_ = os.Chmod(p, fi.Mode().Perm())
			}
		}
		if err != nil {
			_ = os.Remove(tmpName)
			return err
		}
	}
	return nil
}

// tbOutputDirFlag uses --output/-o because the MCP shell-out blocks that name; --out stays as a deprecated alias.
func tbOutputDirFlag(cmd *cobra.Command, dir *string, usage string) {
	cmd.Flags().StringVarP(dir, "output", "o", "", usage)
	cmd.Flags().StringVar(dir, "out", "", usage)
	_ = cmd.Flags().MarkDeprecated("out", "use --output")
}

func tbPrintSave(cmd *cobra.Command, flags *rootFlags, plan tbSavePlan) error {
	if plan.Files == nil {
		plan.Files = []tbSavedAttachment{}
	}
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		if plan.DryRun {
			return printJSONFiltered(cmd.OutOrStdout(), plan, flags)
		}
		return printJSONFiltered(cmd.OutOrStdout(), plan.Files, flags)
	}
	w := tbHumanOut(cmd)
	for _, f := range plan.Files {
		switch {
		case plan.DryRun && f.Exists:
			fmt.Fprintf(w, "dry-run: would overwrite %s (%s; needs --force)\n", f.Path, tbHumanBytes(f.SizeBytes))
		case plan.DryRun:
			fmt.Fprintf(w, "dry-run: would write %s (%s)\n", f.Path, tbHumanBytes(f.SizeBytes))
		default:
			fmt.Fprintf(w, "saved %s (%s)\n", f.Path, tbHumanBytes(f.SizeBytes))
		}
	}
	return nil
}
