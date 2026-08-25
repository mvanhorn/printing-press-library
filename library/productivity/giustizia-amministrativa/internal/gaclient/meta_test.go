package gaclient

import "testing"

const sampleMetaXML = `<?xml version="1.0" encoding="UTF-8" standalone="no"?>` +
	`<GA xmlns:h="http://www.w3.org/HTML/1998/html4"><Provvedimento>` +
	`<meta id="20251593820260528123515630" descrizione="accesso procedimento di sorveglianza" tipo="2">` +
	`<descrittori><registro anno="2025" n="15938"/><urn>urn:nir:tar.lazio;sezione.3Q:sentenza:00000-0000</urn></descrittori>` +
	`<tipologia>Sentenza</tipologia>` +
	`<firmaPresidente><firma/><data>00:00:00</data></firmaPresidente>` +
	`<firmaEstensore><firma>Silvia Piemonte</firma><data>06/08/2026 12:43:45</data></firmaEstensore>` +
	`<dataPubblicazione>14/08/2026</dataPubblicazione><omissis>Falso</omissis></meta>` +
	`<epigrafe id="epi"><oggetto><h:div>per l'annullamento</h:div></oggetto></epigrafe>` +
	`</Provvedimento></GA>`

func TestParseMetaXML(t *testing.T) {
	m, ok := ParseMetaXML([]byte(sampleMetaXML))
	if !ok {
		t.Fatal("metadati non letti dall'XML del portale")
	}
	if m.Oggetto != "accesso procedimento di sorveglianza" {
		t.Errorf("oggetto = %q", m.Oggetto)
	}
	if m.DataPubblicazione != "14/08/2026" {
		t.Errorf("data pubblicazione = %q", m.DataPubblicazione)
	}
	if m.Estensore != "Silvia Piemonte" {
		t.Errorf("estensore = %q", m.Estensore)
	}
	// Firma del presidente vuota: si emette solo cio' che il documento porta.
	if m.Presidente != "" {
		t.Errorf("presidente = %q, atteso vuoto", m.Presidente)
	}
	if m.Urn != "urn:nir:tar.lazio;sezione.3Q:sentenza:00000-0000" {
		t.Errorf("urn = %q", m.Urn)
	}
	if m.Omissis {
		t.Error("omissis=Falso letto come vero")
	}
}

func TestParseMetaXMLIgnoraCioCheNonEXML(t *testing.T) {
	// L'endpoint e' polimorfo: per i provvedimenti pubblicati in PDF serve il
	// file, non l'XML. Nessun metadato e' la risposta giusta, non un errore.
	cases := map[string][]byte{
		"pdf":               []byte("%PDF-1.4\n1 0 obj\n"),
		"pagina errore":     []byte(`<!DOCTYPE html><html><head><title>404 - Pagina non trovata</title></head><body></body></html>`),
		"html renderizzato": []byte(`<html><body class="corpo">Pubblicato il 14/08/2026</body></html>`),
	}
	for name, body := range cases {
		if m, ok := ParseMetaXML(body); ok {
			t.Errorf("%s: metadati inventati: %+v", name, m)
		}
	}
}

func TestMetaXMLURL(t *testing.T) {
	got := MetaXMLURL("tar_rm", "202515938", "202614259_01.html")
	if want := "https://mdp.giustizia-amministrativa.it/visualizza/?"; got[:len(want)] != want {
		t.Errorf("url = %q", got)
	}
}
