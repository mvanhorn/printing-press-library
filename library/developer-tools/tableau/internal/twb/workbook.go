// Package twb parses and mutates Tableau workbook XML (.twb / .twbx).
package twb

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/beevik/etree"
)

// MaxTWBBytes caps decompressed .twb payload size (zip-bomb protection).
const MaxTWBBytes = 50 << 20 // 50 MiB

// Workbook is an in-memory Tableau workbook document.
type Workbook struct {
	doc  *etree.Document
	path string

	// fromTWBX is true when the workbook was opened from a packaged .twbx.
	// Writing only the XML as .twb would drop extracts/images/other package members.
	fromTWBX bool
	// packageExtraFiles counts non-.twb entries inside a source .twbx.
	packageExtraFiles int

	// AllowDropPackage permits Write when fromTWBX is true (explicit unsafe opt-in).
	AllowDropPackage bool
}

// Open loads a .twb or .twbx (zip containing a .twb) from path.
func Open(path string) (*Workbook, error) {
	data, fromTWBX, extras, err := readWorkbookBytes(path)
	if err != nil {
		return nil, err
	}
	doc := etree.NewDocument()
	doc.ReadSettings.CharsetReader = nil
	if err := doc.ReadFromBytes(data); err != nil {
		return nil, fmt.Errorf("parse workbook XML: %w", err)
	}
	root := doc.Root()
	if root == nil || root.Tag != "workbook" {
		return nil, fmt.Errorf("root element is not <workbook>")
	}
	return &Workbook{
		doc:               doc,
		path:              path,
		fromTWBX:          fromTWBX,
		packageExtraFiles: extras,
	}, nil
}

func readWorkbookBytes(path string) (data []byte, fromTWBX bool, extras int, err error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".twbx":
		data, extras, err = readTWBX(path)
		return data, true, extras, err
	case ".twb", "":
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, false, 0, fmt.Errorf("read %s: %w", path, err)
		}
		if int64(len(data)) > MaxTWBBytes {
			return nil, false, 0, fmt.Errorf("workbook XML exceeds %d byte limit", MaxTWBBytes)
		}
		return data, false, 0, nil
	default:
		data, err = os.ReadFile(path)
		if err != nil {
			return nil, false, 0, fmt.Errorf("read %s: %w", path, err)
		}
		if len(data) >= 2 && data[0] == 'P' && data[1] == 'K' {
			data, extras, err = readTWBX(path)
			return data, true, extras, err
		}
		if int64(len(data)) > MaxTWBBytes {
			return nil, false, 0, fmt.Errorf("workbook XML exceeds %d byte limit", MaxTWBBytes)
		}
		return data, false, 0, nil
	}
}

func readTWBX(path string) ([]byte, int, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, 0, fmt.Errorf("open twbx: %w", err)
	}
	defer zr.Close()

	var twbFile *zip.File
	extras := 0
	for _, f := range zr.File {
		name := f.Name
		if strings.HasPrefix(name, "__MACOSX") {
			continue
		}
		lower := strings.ToLower(name)
		if strings.HasSuffix(lower, ".twb") {
			if twbFile == nil {
				twbFile = f
			} else {
				// count additional twb-like entries as extras
				extras++
			}
			continue
		}
		if !f.FileInfo().IsDir() {
			extras++
		}
	}
	if twbFile == nil {
		return nil, 0, fmt.Errorf("no .twb entry found in %s", path)
	}
	if twbFile.UncompressedSize64 > MaxTWBBytes {
		return nil, 0, fmt.Errorf("twbx embedded .twb uncompressed size %d exceeds %d byte limit", twbFile.UncompressedSize64, MaxTWBBytes)
	}
	rc, err := twbFile.Open()
	if err != nil {
		return nil, 0, fmt.Errorf("open twb inside twbx: %w", err)
	}
	defer rc.Close()
	// Bound read even if header lies.
	limited := io.LimitReader(rc, int64(MaxTWBBytes)+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, 0, fmt.Errorf("read twb inside twbx: %w", err)
	}
	if int64(len(data)) > MaxTWBBytes {
		return nil, 0, fmt.Errorf("twbx embedded .twb exceeds %d byte limit after decompress", MaxTWBBytes)
	}
	return data, extras, nil
}

// Path returns the path the workbook was opened from.
func (w *Workbook) Path() string { return w.path }

// FromTWBX reports whether the workbook was opened from a packaged .twbx.
func (w *Workbook) FromTWBX() bool { return w.fromTWBX }

// PackageExtraFiles returns non-primary package member count for a source .twbx.
func (w *Workbook) PackageExtraFiles() int { return w.packageExtraFiles }

// Root returns the <workbook> element.
func (w *Workbook) Root() *etree.Element { return w.doc.Root() }

// Write serializes the workbook as .twb XML to path.
// .twbx packaging is not supported yet.
// If the source was a .twbx with packaged resources, Write fails unless AllowDropPackage is set
// so agents cannot silently detach extracts/images.
func (w *Workbook) Write(path string) error {
	if strings.EqualFold(filepath.Ext(path), ".twbx") {
		return fmt.Errorf("writing .twbx is not supported; write a .twb path instead")
	}
	if w.fromTWBX && !w.AllowDropPackage {
		return fmt.Errorf("refusing to write .twb from packaged .twbx source %q (%d extra package file(s) would be dropped). Re-export as .twb in Desktop, or pass --allow-drop-package if XML-only output is intentional", w.path, w.packageExtraFiles)
	}
	w.doc.Indent(2)
	var buf bytes.Buffer
	if _, err := w.doc.WriteTo(&buf); err != nil {
		return fmt.Errorf("serialize workbook: %w", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// Bytes returns the current workbook XML.
func (w *Workbook) Bytes() ([]byte, error) {
	w.doc.Indent(2)
	return w.doc.WriteToBytes()
}

// worksheetsParent returns the <worksheets> container, creating it if needed.
func (w *Workbook) worksheetsParent() *etree.Element {
	root := w.Root()
	ws := root.SelectElement("worksheets")
	if ws == nil {
		ws = root.CreateElement("worksheets")
	}
	return ws
}

// datasourcesParent returns the <datasources> container.
func (w *Workbook) datasourcesParent() *etree.Element {
	return w.Root().SelectElement("datasources")
}

// dashboardsParent returns the <dashboards> container.
func (w *Workbook) dashboardsParent() *etree.Element {
	return w.Root().SelectElement("dashboards")
}

// findWorksheet returns the worksheet element with the given name.
func (w *Workbook) findWorksheet(name string) *etree.Element {
	parent := w.Root().SelectElement("worksheets")
	if parent == nil {
		return nil
	}
	for _, el := range parent.SelectElements("worksheet") {
		if el.SelectAttrValue("name", "") == name {
			return el
		}
	}
	return nil
}

// cloneElement deep-copies an etree element (including attrs and children).
func cloneElement(src *etree.Element) *etree.Element {
	return src.Copy()
}
