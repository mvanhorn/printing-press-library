package gaclient

import (
	"fmt"
	"strings"
)

// FrontMatter renders a YAML front-matter block (delimited by ---) describing a
// provvedimento, emitting only the non-empty mechanical fields. It is meant to
// be prepended to the Markdown/text full text so the output is directly
// archivable with structured, source-traceable metadata.
//
// String values are always double-quoted (sede/sezione/tipo may contain spaces
// and ecli/url contain ':' '/' '&' '=' which are unsafe as bare YAML scalars).
func FrontMatter(p Provvedimento) string {
	var b strings.Builder
	b.WriteString("---\n")
	addStr := func(k, v string) {
		if v != "" {
			fmt.Fprintf(&b, "%s: %s\n", k, yamlQuote(v))
		}
	}
	addInt := func(k string, v int) {
		if v != 0 {
			fmt.Fprintf(&b, "%s: %d\n", k, v)
		}
	}
	addStr("ecli", p.Ecli)
	addStr("tipo", p.Tipo)
	addStr("sede", p.Sede)
	addStr("sezione", p.Sezione)
	addInt("numero", p.Numero)
	addInt("anno", p.Anno)
	addStr("nrg", p.Nrg)
	addStr("data_deposito", p.DataDeposito)
	addStr("formato", p.Formato)
	addStr("url", p.URL)
	if p.Meta != nil {
		addStr("data_pubblicazione", p.Meta.DataPubblicazione)
		addStr("oggetto", p.Meta.Oggetto)
		addStr("presidente", p.Meta.Presidente)
		addStr("estensore", p.Meta.Estensore)
		addStr("urn", p.Meta.Urn)
		if p.Meta.Omissis {
			b.WriteString("omissis: true\n")
		}
	}
	b.WriteString("---\n")
	return b.String()
}

// yamlQuote returns s as a YAML double-quoted scalar. Besides backslash and
// double quote, it escapes the control characters that are invalid inside a
// double-quoted scalar (newline, carriage return, tab) so a stray CR/LF from
// CRLF-encoded HTML can never produce syntactically invalid YAML.
func yamlQuote(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}
