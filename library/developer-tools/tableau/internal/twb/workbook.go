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

// Workbook is an in-memory Tableau workbook document.
type Workbook struct {
	doc  *etree.Document
	path string
}

// Open loads a .twb or .twbx (zip containing a .twb) from path.
func Open(path string) (*Workbook, error) {
	data, err := readWorkbookBytes(path)
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
	return &Workbook{doc: doc, path: path}, nil
}

func readWorkbookBytes(path string) ([]byte, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".twbx":
		return readTWBX(path)
	case ".twb", "":
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		return data, nil
	default:
		// Try as plain XML first; fall back to zip if magic matches.
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		if len(data) >= 2 && data[0] == 'P' && data[1] == 'K' {
			return readTWBX(path)
		}
		return data, nil
	}
}

func readTWBX(path string) ([]byte, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open twbx: %w", err)
	}
	defer zr.Close()

	var twbFile *zip.File
	for _, f := range zr.File {
		if strings.HasSuffix(strings.ToLower(f.Name), ".twb") && !strings.HasPrefix(f.Name, "__MACOSX") {
			twbFile = f
			break
		}
	}
	if twbFile == nil {
		return nil, fmt.Errorf("no .twb entry found in %s", path)
	}
	rc, err := twbFile.Open()
	if err != nil {
		return nil, fmt.Errorf("open twb inside twbx: %w", err)
	}
	defer rc.Close()
	data, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("read twb inside twbx: %w", err)
	}
	return data, nil
}

// Path returns the path the workbook was opened from.
func (w *Workbook) Path() string { return w.path }

// Root returns the <workbook> element.
func (w *Workbook) Root() *etree.Element { return w.doc.Root() }

// Write serializes the workbook as .twb XML to path.
// .twbx packaging is not supported yet.
func (w *Workbook) Write(path string) error {
	if strings.EqualFold(filepath.Ext(path), ".twbx") {
		return fmt.Errorf("writing .twbx is not supported; write a .twb path instead")
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
