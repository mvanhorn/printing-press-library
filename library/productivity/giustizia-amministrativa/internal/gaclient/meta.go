package gaclient

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"strings"
)

// The portal publishes each provvedimento twice from the same source: the
// /visualizzah2/ endpoint renders it as an HTML page (what this CLI converts
// to Markdown), while /visualizza/ serves the XML the rendering is made from.
// The rendering drops what has no place on a printed page — the URN NIR, the
// anonymisation flag, the registry's own description of the subject — and
// leaves the rest unlabelled: the names of the president and of the judge who
// wrote the ruling are there, but only as two lines among the others.
//
// Meta carries those fields. It is additive: the text still comes from the
// HTML rendering, and fetching it costs a second request, so it is opt-in.
type Meta struct {
	// Oggetto is the subject as recorded in the registry ("accesso
	// procedimento di sorveglianza"), not a sentence extracted from the text.
	Oggetto           string `json:"oggetto,omitempty"`
	DataPubblicazione string `json:"data_pubblicazione,omitempty"`
	Presidente        string `json:"presidente,omitempty"`
	Estensore         string `json:"estensore,omitempty"`
	// Urn is the NIR identifier as the registry writes it. It names the
	// organ, the section and the kind of provvedimento, but the portal leaves
	// its number and date at "00000-0000" (measured on 10 provvedimenti from
	// 10 different sedi, 2010-2026): it identifies the sezione, not the single
	// ruling, and is not a citation key on its own.
	Urn string `json:"urn,omitempty"`
	// Omissis reports that the published text is anonymised, so names missing
	// from it are missing by law and not by a fault of the extraction.
	Omissis bool `json:"omissis,omitempty"`
}

// Empty reports whether nothing was parsed.
func (m Meta) Empty() bool {
	return m == Meta{}
}

// gaXML mirrors the <GA> document served by /visualizza/, limited to the
// registry metadata: the body is read from the HTML rendering instead.
type gaXML struct {
	XMLName       xml.Name `xml:"GA"`
	Provvedimento struct {
		Meta struct {
			Descrizione string `xml:"descrizione,attr"`
			Descrittori struct {
				Urn string `xml:"urn"`
			} `xml:"descrittori"`
			FirmaPresidente struct {
				Firma string `xml:"firma"`
			} `xml:"firmaPresidente"`
			FirmaEstensore struct {
				Firma string `xml:"firma"`
			} `xml:"firmaEstensore"`
			DataPubblicazione string `xml:"dataPubblicazione"`
			Omissis           string `xml:"omissis"`
		} `xml:"meta"`
	} `xml:"Provvedimento"`
}

// ParseMetaXML reads the registry metadata from the XML form of a
// provvedimento. It reports false for anything that is not that XML — the
// endpoint is polymorphic and answers with the file itself for the rulings
// published as PDF.
func ParseMetaXML(body []byte) (Meta, bool) {
	trimmed := bytes.TrimLeft(body, " \t\r\n")
	if !bytes.HasPrefix(trimmed, []byte("<?xml")) && !bytes.HasPrefix(trimmed, []byte("<GA")) {
		return Meta{}, false
	}
	var doc gaXML
	if err := xml.Unmarshal(trimmed, &doc); err != nil {
		return Meta{}, false
	}
	m := doc.Provvedimento.Meta
	out := Meta{
		Oggetto:           strings.TrimSpace(m.Descrizione),
		DataPubblicazione: strings.TrimSpace(m.DataPubblicazione),
		Presidente:        strings.TrimSpace(m.FirmaPresidente.Firma),
		Estensore:         strings.TrimSpace(m.FirmaEstensore.Firma),
		Urn:               strings.TrimSpace(m.Descrittori.Urn),
		Omissis:           strings.EqualFold(strings.TrimSpace(m.Omissis), "vero"),
	}
	return out, !out.Empty()
}

// MetaXMLURL is the URL of the XML form of a provvedimento. It is the
// /visualizza/ endpoint, which serves the source document: XML for the rulings
// published as HTML, the file itself for those published as PDF.
func MetaXMLURL(schema, nrg, nomeFile string) string {
	u := DocURL(schema, nrg, nomeFile)
	return strings.Replace(u, "/visualizzah2/?", "/visualizza/?", 1)
}

// Meta fetches the registry metadata of a provvedimento. A provvedimento
// published as PDF has no XML form: it returns an empty Meta and no error,
// because there is nothing missing to report — that is how the portal
// publishes it.
func (c *Client) Meta(ctx context.Context, p Provvedimento) (Meta, error) {
	if p.Schema == "" || p.Nrg == "" || p.NomeFile == "" {
		return Meta{}, fmt.Errorf("dati insufficienti per i metadati (servono schema, nrg, nome_file)")
	}
	if isPDFPath(p.NomeFile) {
		return Meta{}, nil
	}
	body, status, err := c.get(ctx, MetaXMLURL(p.Schema, p.Nrg, p.NomeFile))
	if err != nil {
		return Meta{}, err
	}
	if status != http.StatusOK {
		return Meta{}, fmt.Errorf("metadati: HTTP %d", status)
	}
	if isErrorPage(body) {
		return Meta{}, fmt.Errorf("metadati non disponibili: il portale ha risposto con la propria pagina di errore")
	}
	m, ok := ParseMetaXML(body)
	if !ok {
		return Meta{}, nil
	}
	return m, nil
}
