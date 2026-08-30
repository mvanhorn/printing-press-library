// Copyright 2026 Som Samantray and contributors. Licensed under Apache-2.0. See LICENSE.
// Hand-authored local document operations for ihatepdf.cv CLI.
package cli

import (
	"crypto/md5"  // #nosec G501 -- compatibility fingerprint; SHA-256 is also emitted.
	"crypto/sha1" // #nosec G505 -- compatibility fingerprint; SHA-256 is also emitted.
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-pdf/fpdf"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/spf13/cobra"
)

type fileHash struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	MD5    string `json:"md5"`
	SHA1   string `json:"sha1"`
	SHA256 string `json:"sha256"`
}
type inspectResult struct {
	Path          string   `json:"path"`
	Size          int64    `json:"size"`
	Pages         int      `json:"pages"`
	Valid         bool     `json:"valid"`
	MetadataHints []string `json:"metadata_hints,omitempty"`
	Hashes        fileHash `json:"hashes"`
}
type riskFinding struct {
	Kind     string   `json:"kind"`
	Count    int      `json:"count"`
	Examples []string `json:"examples,omitempty"`
}
type privacyResult struct {
	Path          string        `json:"path"`
	Findings      []riskFinding `json:"findings"`
	MetadataHints []string      `json:"metadata_hints,omitempty"`
	RiskCount     int           `json:"risk_count"`
}
type operationResult struct {
	Operation string   `json:"operation"`
	Inputs    []string `json:"inputs,omitempty"`
	Output    string   `json:"output,omitempty"`
	Bytes     int64    `json:"bytes,omitempty"`
	Pages     int      `json:"pages,omitempty"`
	Message   string   `json:"message,omitempty"`
}

func emitLocal(cmd *cobra.Command, flags *rootFlags, value any) error {
	if !wantsHumanTable(cmd.OutOrStdout(), flags) {
		return printJSONFiltered(cmd.OutOrStdout(), value, flags)
	}
	return printJSONFiltered(cmd.OutOrStdout(), value, flags)
}
func needInput(cmd *cobra.Command, args []string, n int) error {
	if len(args) < n {
		_ = cmd.Usage()
		return usageErr(fmt.Errorf("missing required input path"))
	}
	return nil
}
func refuseOverwrite(path string) error {
	if path == "" {
		return fmt.Errorf("--output is required")
	}
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("output %q already exists; choose another --output path", path)
	}
	return nil
}

func requireEmptyOutputDir(path string) error {
	if path == "" {
		return fmt.Errorf("--output-dir is required")
	}
	if st, err := os.Stat(path); err == nil {
		if !st.IsDir() {
			return fmt.Errorf("--output-dir %q exists but is not a directory", path)
		}
		entries, readErr := os.ReadDir(path)
		if readErr != nil {
			return fmt.Errorf("inspect output directory: %w", readErr)
		}
		if len(entries) > 0 {
			return fmt.Errorf("output directory %q is not empty; choose a new directory", path)
		}
	}
	return os.MkdirAll(path, 0750)
}
func readFile(path string) ([]byte, error) {
	// #nosec G304 -- the path is an explicit local CLI input, not a server-controlled filename.
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(b) == 0 {
		return nil, fmt.Errorf("input %s is empty", path)
	}
	return b, nil
}
func hashBytes(path string, b []byte) fileHash {
	// #nosec G401 -- MD5 is retained as a labeled compatibility fingerprint; SHA-256 is emitted alongside it.
	m := md5.Sum(b)
	// #nosec G401 -- SHA-1 is retained as a labeled compatibility fingerprint; SHA-256 is emitted alongside it.
	s1 := sha1.Sum(b)
	s256 := sha256.Sum256(b)
	return fileHash{Path: path, Size: int64(len(b)), MD5: hex.EncodeToString(m[:]), SHA1: hex.EncodeToString(s1[:]), SHA256: hex.EncodeToString(s256[:])}
}
func dryOrNeed(cmd *cobra.Command, flags *rootFlags, args []string, n int, action string) (bool, error) {
	if len(args) == 0 && cmd.Flags().NFlag() == 0 {
		return false, cmd.Help()
	}
	if dryRunOK(flags) {
		return true, writeDryRun(cmd.OutOrStdout(), flags, action)
	}
	if err := needInput(cmd, args, n); err != nil {
		return false, err
	}
	return false, nil
}

func newLocalOpsCmds() func(*cobra.Command, *rootFlags) {
	return func(root *cobra.Command, flags *rootFlags) {
		for _, c := range []*cobra.Command{newMergeCmd(flags), newSplitCmd(flags), newRotateCmd(flags), newExtractTextCmd(flags), newImagesToPDFCmd(flags), newTextToPDFCmd(flags), newEncryptCmd(flags)} {
			addNovelCommandIfAbsent(root, c)
		}
	}
}
func init() { registerNovelCommand(newLocalOpsCmds()) }

func newMergeCmd(flags *rootFlags) *cobra.Command {
	var output string
	var divider bool
	c := &cobra.Command{Use: "merge [files...]", Short: "Combine PDFs in explicit order without uploading them.", Example: "  ihatepdf-cv-pp-cli merge cover.pdf appendix.pdf --output packet.pdf --json", Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "first=testdata/fixture.pdf;second=testdata/fixture.pdf;--output=verify-merge.pdf"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "merge PDFs")
		}
		if len(args) < 2 {
			return usageErr(fmt.Errorf("provide at least two input PDFs"))
		}
		if err := refuseOverwrite(output); err != nil {
			return usageErr(err)
		}
		for _, p := range args {
			if _, err := os.Stat(p); err != nil {
				return fmt.Errorf("input %q is not readable: %w", p, err)
			}
		}
		if err := api.MergeCreateFile(args, output, divider, model.NewDefaultConfiguration()); err != nil {
			return fmt.Errorf("merge PDFs: %w", err)
		}
		st, _ := os.Stat(output)
		pages, _ := api.PageCountFile(output)
		return emitLocal(cmd, flags, operationResult{Operation: "merge", Inputs: args, Output: output, Bytes: st.Size(), Pages: pages, Message: "merged locally"})
	}}
	c.Flags().StringVar(&output, "output", "", "output PDF path (never overwrites an existing file)")
	c.Flags().BoolVar(&divider, "divider-page", false, "insert a divider page between input PDFs")
	return c
}

func newSplitCmd(flags *rootFlags) *cobra.Command {
	var outDir string
	var span int
	c := &cobra.Command{Use: "split [input.pdf]", Short: "Split a PDF into deterministic page-group files.", Example: "  ihatepdf-cv-pp-cli split report.pdf --span 1 --output-dir pages --json", Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "input=testdata/fixture.pdf;--output-dir=verify-split"}, RunE: func(cmd *cobra.Command, args []string) error {
		ok, err := dryOrNeed(cmd, flags, args, 1, "split PDF")
		if err != nil || ok {
			return err
		}
		if err := requireEmptyOutputDir(outDir); err != nil {
			return usageErr(err)
		}
		if span < 1 {
			return usageErr(fmt.Errorf("--span must be at least 1"))
		}
		if err := api.SplitFile(args[0], outDir, span, model.NewDefaultConfiguration()); err != nil {
			return fmt.Errorf("split PDF: %w", err)
		}
		entries, _ := os.ReadDir(outDir)
		return emitLocal(cmd, flags, operationResult{Operation: "split", Inputs: args, Output: outDir, Message: fmt.Sprintf("wrote %d page-group files", len(entries))})
	}}
	c.Flags().StringVar(&outDir, "output-dir", "", "directory for split PDFs")
	c.Flags().IntVar(&span, "span", 1, "pages per output PDF")
	return c
}

func newRotateCmd(flags *rootFlags) *cobra.Command {
	var output, pages string
	var degrees int
	c := &cobra.Command{Use: "rotate [input.pdf]", Short: "Rotate selected PDF pages locally.", Example: "  ihatepdf-cv-pp-cli rotate report.pdf --degrees 90 --output rotated.pdf --json", Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "input=testdata/fixture.pdf;--output=verify-rotate.pdf"}, RunE: func(cmd *cobra.Command, args []string) error {
		ok, err := dryOrNeed(cmd, flags, args, 1, "rotate PDF")
		if err != nil || ok {
			return err
		}
		if err := refuseOverwrite(output); err != nil {
			return usageErr(err)
		}
		if degrees%90 != 0 {
			return usageErr(fmt.Errorf("--degrees must be a multiple of 90"))
		}
		var sel []string
		if pages != "" {
			for _, p := range strings.Split(pages, ",") {
				sel = append(sel, strings.TrimSpace(p))
			}
		}
		if err := api.RotateFile(args[0], output, degrees, sel, model.NewDefaultConfiguration()); err != nil {
			return fmt.Errorf("rotate PDF: %w", err)
		}
		return emitLocal(cmd, flags, operationResult{Operation: "rotate", Inputs: args, Output: output, Message: fmt.Sprintf("rotated %d degrees", degrees)})
	}}
	c.Flags().StringVar(&output, "output", "", "output PDF path")
	c.Flags().IntVar(&degrees, "degrees", 90, "clockwise rotation: 90, 180, or 270")
	c.Flags().StringVar(&pages, "pages", "", "comma-separated 1-based page numbers; blank means all pages")
	return c
}

func extractLiteralText(b []byte) string {
	re := regexp.MustCompile(`(?s)\(([^()]*)\)\s*Tj`)
	var out []string
	for _, m := range re.FindAllSubmatch(b, -1) {
		s := strings.ReplaceAll(string(m[1]), `\\n`, "\n")
		s = strings.ReplaceAll(s, `\\r`, "\n")
		s = strings.ReplaceAll(s, `\\(`, "(")
		s = strings.ReplaceAll(s, `\\)`, ")")
		out = append(out, s)
	}
	return strings.Join(out, "\n")
}
func newExtractTextCmd(flags *rootFlags) *cobra.Command {
	var output string
	var stdin bool
	c := &cobra.Command{Use: "extract-text [input.pdf]", Short: "Extract readable PDF text to stdout or a file.", Example: "  ihatepdf-cv-pp-cli extract-text report.pdf --json\n  cat report.pdf | ihatepdf-cv-pp-cli extract-text --stdin --agent", Annotations: map[string]string{"mcp:read-only": "true", "pp:happy-args": "input=testdata/fixture.pdf"}, RunE: func(cmd *cobra.Command, args []string) error {
		if !stdin {
			ok, err := dryOrNeed(cmd, flags, args, 1, "extract PDF text")
			if err != nil || ok {
				return err
			}
		}
		var b []byte
		var err error
		if stdin {
			b, err = io.ReadAll(os.Stdin)
		} else {
			b, err = readFile(args[0])
		}
		if err != nil {
			return err
		}
		text := extractLiteralText(b)
		if output != "" {
			if err := refuseOverwrite(output); err != nil {
				return usageErr(err)
			}
			if err := os.WriteFile(output, []byte(text+"\n"), 0600); err != nil {
				return err
			}
		}
		path := "<stdin>"
		if len(args) > 0 {
			path = args[0]
		}
		return emitLocal(cmd, flags, map[string]any{"path": path, "text": text, "output": output, "characters": len(text)})
	}}
	c.Flags().StringVar(&output, "output", "", "optional text output path")
	c.Flags().BoolVar(&stdin, "stdin", false, "read PDF bytes from stdin")
	return c
}

func imageTypeForPath(path string) (string, error) {
	ext := filepath.Ext(path)
	if ext == "" {
		return "", usageErr(fmt.Errorf("image path %s has no file extension", path))
	}
	return strings.ToUpper(ext[1:]), nil
}

func newImagesToPDFCmd(flags *rootFlags) *cobra.Command {
	var output string
	c := &cobra.Command{Use: "images-to-pdf [images...]", Short: "Convert JPG and PNG images to a local PDF.", Example: "  ihatepdf-cv-pp-cli images-to-pdf scan-1.png scan-2.jpg --output scans.pdf --json", Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "image=testdata/fixture.png;--output=verify-images.pdf"}, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			return cmd.Help()
		}
		if len(args) == 0 {
			return usageErr(fmt.Errorf("provide at least one image"))
		}
		if dryRunOK(flags) {
			return writeDryRun(cmd.OutOrStdout(), flags, "convert images to PDF")
		}
		if err := refuseOverwrite(output); err != nil {
			return usageErr(err)
		}
		pdf := fpdf.New("P", "mm", "A4", "")
		pdf.SetCompression(false)
		for _, path := range args {
			// #nosec G304 -- image paths are explicit local CLI inputs.
			f, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("open image %s: %w", path, err)
			}
			cfg, _, err := image.DecodeConfig(f)
			if closeErr := f.Close(); closeErr != nil && err == nil {
				return fmt.Errorf("close image %s: %w", path, closeErr)
			}
			if err != nil {
				return fmt.Errorf("decode image %s: %w", path, err)
			}
			imageType, err := imageTypeForPath(path)
			if err != nil {
				return err
			}
			pdf.AddPage()
			pdf.ImageOptions(path, 10, 10, 190, 0, false, fpdf.ImageOptions{ImageType: imageType, ReadDpi: true}, 0, "")
			_ = cfg
		}
		if err := pdf.OutputFileAndClose(output); err != nil {
			return fmt.Errorf("write image PDF: %w", err)
		}
		st, _ := os.Stat(output)
		return emitLocal(cmd, flags, operationResult{Operation: "images-to-pdf", Inputs: args, Output: output, Bytes: st.Size(), Pages: len(args)})
	}}
	c.Flags().StringVar(&output, "output", "", "output PDF path")
	return c
}

func newTextToPDFCmd(flags *rootFlags) *cobra.Command {
	var output string
	var title string
	var stdin bool
	c := &cobra.Command{Use: "text-to-pdf [input.txt]", Short: "Render plain text or Markdown into a local PDF.", Example: "  ihatepdf-cv-pp-cli text-to-pdf notes.md --output notes.pdf --json\n  echo '# Notes' | ihatepdf-cv-pp-cli text-to-pdf --stdin --output notes.pdf --agent", Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "input=testdata/fixture.txt;--output=verify-text.pdf"}, RunE: func(cmd *cobra.Command, args []string) error {
		if !stdin {
			ok, err := dryOrNeed(cmd, flags, args, 1, "convert text to PDF")
			if err != nil || ok {
				return err
			}
		}
		if err := refuseOverwrite(output); err != nil {
			return usageErr(err)
		}
		var b []byte
		var err error
		if stdin {
			b, err = io.ReadAll(os.Stdin)
		} else {
			b, err = readFile(args[0])
		}
		if err != nil {
			return err
		}
		pdf := fpdf.New("P", "mm", "A4", "")
		pdf.SetCompression(false)
		pdf.SetTitle(title, false)
		pdf.SetFont("Arial", "", 11)
		pdf.AddPage()
		for _, line := range strings.Split(strings.ReplaceAll(string(b), "\r\n", "\n"), "\n") {
			pdf.MultiCell(190, 6, line, "", "L", false)
		}
		if err := pdf.OutputFileAndClose(output); err != nil {
			return err
		}
		return emitLocal(cmd, flags, operationResult{Operation: "text-to-pdf", Inputs: args, Output: output, Message: "rendered locally"})
	}}
	c.Flags().StringVar(&output, "output", "", "output PDF path")
	c.Flags().StringVar(&title, "title", "", "PDF title metadata")
	c.Flags().BoolVar(&stdin, "stdin", false, "read text from stdin")
	return c
}

func newEncryptCmd(flags *rootFlags) *cobra.Command {
	var output, password string
	c := &cobra.Command{Use: "encrypt [input.pdf]", Short: "Password-protect a PDF locally with AES encryption.", Example: "  IHATEPDF_CV_PASSWORD=secret ihatepdf-cv-pp-cli encrypt report.pdf --output locked.pdf --json", Annotations: map[string]string{"mcp:read-only": "false", "pp:happy-args": "input=testdata/fixture.pdf;--output=verify-encrypted.pdf;--password=test-password"}, RunE: func(cmd *cobra.Command, args []string) error {
		ok, err := dryOrNeed(cmd, flags, args, 1, "encrypt PDF")
		if err != nil || ok {
			return err
		}
		if err := refuseOverwrite(output); err != nil {
			return usageErr(err)
		}
		if password == "" {
			password = os.Getenv("IHATEPDF_CV_PASSWORD")
		}
		if password == "" {
			return usageErr(fmt.Errorf("set --password or IHATEPDF_CV_PASSWORD; the password is never printed"))
		}
		cfg := model.NewAESConfiguration(password, password, 256)
		if err := api.EncryptFile(args[0], output, cfg); err != nil {
			return fmt.Errorf("encrypt PDF: %w", err)
		}
		return emitLocal(cmd, flags, operationResult{Operation: "encrypt", Inputs: args, Output: output, Message: "encrypted locally; password omitted"})
	}}
	c.Flags().StringVar(&output, "output", "", "output PDF path")
	c.Flags().StringVar(&password, "password", "", "encryption password (prefer IHATEPDF_CV_PASSWORD)")
	return c
}
