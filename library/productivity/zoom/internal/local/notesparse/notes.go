// Package notesparse extracts text + segments + action items from Zoom Notes
// exports (PDF or DOCX). Pure-Go: PDF via github.com/ledongthuc/pdf (already
// permissive licensed; tiny dep), DOCX via inline archive/zip + encoding/xml
// (no third-party dep). Returns the IngestedNote shape localstore expects.
package notesparse

import (
	"archive/zip"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	pdfreader "github.com/ledongthuc/pdf"
	"github.com/mvanhorn/printing-press-library/library/productivity/zoom/internal/local/localstore"
)

// Parse dispatches on file extension. Returns an IngestedNote whose
// SourceFile is the absolute path and FileFormat is "pdf" or "docx".
func Parse(path string) (localstore.IngestedNote, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	ext := strings.ToLower(filepath.Ext(abs))
	switch ext {
	case ".pdf":
		text, err := extractPDFText(abs)
		if err != nil {
			return localstore.IngestedNote{}, err
		}
		return buildNote(abs, "pdf", text), nil
	case ".docx":
		text, err := extractDocxText(abs)
		if err != nil {
			return localstore.IngestedNote{}, err
		}
		return buildNote(abs, "docx", text), nil
	case ".txt", ".md":
		// Allow .txt/.md so users can run the extractor against their own
		// pre-converted exports without dragging in another tool.
		data, err := os.ReadFile(abs)
		if err != nil {
			return localstore.IngestedNote{}, err
		}
		return buildNote(abs, ext[1:], string(data)), nil
	default:
		return localstore.IngestedNote{}, fmt.Errorf("notesparse: unsupported file type %s (need .pdf, .docx, .txt, or .md)", ext)
	}
}

// extractPDFText reads every page of a PDF and returns the concatenated text
// with page breaks preserved as double newlines.
func extractPDFText(path string) (string, error) {
	f, r, err := pdfreader.Open(path)
	if err != nil {
		return "", fmt.Errorf("notesparse: opening PDF: %w", err)
	}
	defer f.Close()
	var b strings.Builder
	for i := 1; i <= r.NumPage(); i++ {
		page := r.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		if i > 1 {
			b.WriteString("\n\n")
		}
		b.WriteString(text)
	}
	if b.Len() == 0 {
		return "", errors.New("notesparse: PDF had no extractable text (image-only PDF?)")
	}
	return b.String(), nil
}

// extractDocxText unzips a .docx and concatenates every <w:t> text run from
// word/document.xml. The XML namespace prefix varies; we match by local name.
func extractDocxText(path string) (string, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("notesparse: opening DOCX: %w", err)
	}
	defer zr.Close()
	var doc *zip.File
	for _, f := range zr.File {
		if f.Name == "word/document.xml" {
			doc = f
			break
		}
	}
	if doc == nil {
		return "", errors.New("notesparse: word/document.xml not found in DOCX")
	}
	r, err := doc.Open()
	if err != nil {
		return "", err
	}
	defer r.Close()

	dec := xml.NewDecoder(r)
	var b strings.Builder
	inPara := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				inPara = true
			case "t":
				var text string
				if err := dec.DecodeElement(&text, &t); err == nil {
					b.WriteString(text)
				}
			case "br", "tab":
				b.WriteByte(' ')
			}
		case xml.EndElement:
			if t.Name.Local == "p" && inPara {
				b.WriteByte('\n')
				inPara = false
			}
		}
	}
	if b.Len() == 0 {
		return "", errors.New("notesparse: DOCX had no extractable text runs")
	}
	return b.String(), nil
}

// buildNote turns raw text into segments + heuristically detected metadata.
func buildNote(path, format, text string) localstore.IngestedNote {
	n := localstore.IngestedNote{
		SourceFile: path,
		FileFormat: format,
	}

	// Heuristics on the first few lines:
	//   - first non-empty line is the meeting topic
	//   - look for date strings like "Mon, May 19, 2026" or ISO "2026-05-19"
	//   - look for "Meeting ID: 851 2345 6789"
	head := firstN(text, 1024)
	n.MeetingTopic = detectTopic(head)
	if t := detectDate(head); !t.IsZero() {
		n.StartTime = t
	}
	if id := detectMeetingID(head); id != "" {
		n.MeetingID = id
	}

	// Segments — split on blank lines.
	paras := splitParagraphs(text)
	ord := 0
	var heading string
	for _, p := range paras {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// Treat short ALL-CAPS or title-case-with-trailing-colon lines as
		// headings so search can return the section context.
		if isHeading(p) {
			heading = strings.TrimRight(p, ":")
			continue
		}
		ord++
		n.Segments = append(n.Segments, localstore.NoteSegment{Ord: ord, Heading: heading, Text: p})
	}
	if len(n.Segments) == 0 {
		// Fall back to one big segment if paragraph splitting yielded nothing.
		n.Segments = append(n.Segments, localstore.NoteSegment{Ord: 1, Text: strings.TrimSpace(text)})
	}

	// Action item extraction.
	n.Todos = extractTodos(text)

	return n
}

func firstN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

var (
	dateISO        = regexp.MustCompile(`\b(\d{4}-\d{2}-\d{2})\b`)
	dateUS         = regexp.MustCompile(`\b(?:Mon|Tue|Wed|Thu|Fri|Sat|Sun)?,?\s*(?:January|February|March|April|May|June|July|August|September|October|November|December)\s+\d{1,2},?\s+\d{4}\b`)
	meetingIDLine  = regexp.MustCompile(`Meeting\s*ID\s*:?\s*([\d\s-]{9,15})`)
	headingLikeRow = regexp.MustCompile(`^[\p{Lu}\d][\p{Lu}\s\p{Pd}']{1,60}:?$`)
)

func detectTopic(head string) string {
	for _, line := range strings.Split(head, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Skip obvious metadata lines.
		if strings.HasPrefix(strings.ToLower(line), "meeting id") ||
			strings.HasPrefix(strings.ToLower(line), "date") ||
			strings.HasPrefix(strings.ToLower(line), "time") {
			continue
		}
		if len(line) > 0 && len(line) <= 200 {
			return line
		}
	}
	return ""
}

func detectDate(head string) time.Time {
	if m := dateISO.FindString(head); m != "" {
		if t, err := time.Parse("2006-01-02", m); err == nil {
			return t
		}
	}
	if m := dateUS.FindString(head); m != "" {
		// Strip leading weekday + comma.
		m = regexp.MustCompile(`^(?:Mon|Tue|Wed|Thu|Fri|Sat|Sun),?\s*`).ReplaceAllString(m, "")
		for _, layout := range []string{"January 2, 2006", "January 2 2006", "Jan 2, 2006"} {
			if t, err := time.Parse(layout, m); err == nil {
				return t
			}
		}
	}
	return time.Time{}
}

func detectMeetingID(head string) string {
	if m := meetingIDLine.FindStringSubmatch(head); m != nil {
		s := strings.TrimSpace(m[1])
		// Strip all whitespace and dashes inside the captured group.
		var b strings.Builder
		for _, r := range s {
			if r >= '0' && r <= '9' {
				b.WriteRune(r)
			}
		}
		return b.String()
	}
	return ""
}

func splitParagraphs(text string) []string {
	// Normalise line endings, collapse runs of 3+ newlines to two.
	text = strings.ReplaceAll(text, "\r\n", "\n")
	for strings.Contains(text, "\n\n\n") {
		text = strings.ReplaceAll(text, "\n\n\n", "\n\n")
	}
	return strings.Split(text, "\n\n")
}

func isHeading(line string) bool {
	if len(line) > 80 {
		return false
	}
	if strings.HasSuffix(line, ":") && !strings.Contains(line, " ") {
		return true
	}
	if headingLikeRow.MatchString(line) && strings.Contains(line, " ") {
		// Title-case-ish all-words capitalised.
		return countUpperWords(line) >= 2
	}
	return false
}

func countUpperWords(line string) int {
	n := 0
	for _, w := range strings.Fields(line) {
		if len(w) > 0 && (w[0] >= 'A' && w[0] <= 'Z') {
			n++
		}
	}
	return n
}

// Action item patterns. Order matters — first match wins.
var todoPatterns = []struct {
	name string
	re   *regexp.Regexp
}{
	{"checkbox_done", regexp.MustCompile(`(?m)^\s*[-*]?\s*\[(x|X)\]\s+(.+)$`)},
	{"checkbox", regexp.MustCompile(`(?m)^\s*[-*]?\s*\[\s?\]\s+(.+)$`)},
	{"todo_colon", regexp.MustCompile(`(?im)^\s*TODO\s*:\s*(.+)$`)},
	{"action_item", regexp.MustCompile(`(?im)^\s*Action\s*Item\s*:?\s*(.+)$`)},
	{"action_colon", regexp.MustCompile(`(?im)^\s*Action\s*:\s*(.+)$`)},
	{"follow_up", regexp.MustCompile(`(?im)^\s*Follow\s*[\s\-]*up\s*:?\s*(.+)$`)},
	{"next_steps", regexp.MustCompile(`(?im)^\s*Next\s*(?:Step)?s?\s*:\s*(.+)$`)},
	{"owner_colon", regexp.MustCompile(`(?im)^\s*Owner\s*:\s*(.+)$`)},
}

var ownerSuffix = regexp.MustCompile(`\s*\(\s*Owner\s*:\s*([^)]+)\)\s*$`)

// extractTodos scans the full text for any of the supported patterns.
func extractTodos(text string) []localstore.NoteTodo {
	var todos []localstore.NoteTodo
	ord := 0
	seen := map[string]bool{}
	for _, p := range todoPatterns {
		for _, m := range p.re.FindAllStringSubmatch(text, -1) {
			body := strings.TrimSpace(m[len(m)-1])
			if body == "" {
				continue
			}
			key := p.name + "::" + body
			if seen[key] {
				continue
			}
			seen[key] = true
			ord++
			owner := ""
			if mm := ownerSuffix.FindStringSubmatch(body); mm != nil {
				owner = strings.TrimSpace(mm[1])
				body = strings.TrimSpace(ownerSuffix.ReplaceAllString(body, ""))
			}
			todos = append(todos, localstore.NoteTodo{
				Ord:     ord,
				Pattern: p.name,
				Text:    body,
				Owner:   owner,
				Checked: p.name == "checkbox_done",
			})
		}
	}
	return todos
}
