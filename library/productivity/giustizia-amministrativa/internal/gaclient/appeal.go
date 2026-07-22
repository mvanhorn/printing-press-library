package gaclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// appealResourceURL is the Liferay resource-phase endpoint that reports whether
// a first-instance (TAR) ruling was appealed. It returns clean JSON (a Spring
// Data Page) and does not require the p_auth handshake.
const appealResourceURL = BaseURL + formPath +
	"?p_p_id=" + portletID +
	"&p_p_lifecycle=2&p_p_state=normal&p_p_mode=view" +
	"&p_p_resource_id=%2Fverifica_appello&p_p_cacheability=cacheLevelPage" +
	"&" + portletPrefix + "_cmd=verifica-appello"

// appealPage is the envelope returned by /verifica_appello.
type appealPage struct {
	Content       []json.RawMessage `json:"content"`
	TotalElements int               `json:"totalElements"`
}

// TipoProvvFromNomeFile extracts the part suffix (e.g. "01") that the portal
// uses as tipoProvvedimento, derived from a nome_file like "202611307_01.html".
func TipoProvvFromNomeFile(nomeFile string) string {
	base := nomeFile
	if i := strings.LastIndex(base, "."); i >= 0 {
		base = base[:i]
	}
	if i := strings.LastIndex(base, "_"); i >= 0 {
		return base[i+1:]
	}
	return "01"
}

// VerificaAppello queries whether a provvedimento was appealed, returning the
// raw appeal entries (may be empty) and the total count.
func (c *Client) VerificaAppello(ctx context.Context, p Provvedimento) ([]json.RawMessage, int, error) {
	if p.Schema == "" || p.Nrg == "" {
		return nil, 0, fmt.Errorf("dati insufficienti per la verifica appello (servono schema e nrg)")
	}
	numProvv := fmt.Sprintf("%d%05d", p.Anno, p.Numero)
	if p.Anno == 0 {
		numProvv = fmt.Sprintf("%d", p.Numero)
	}
	v := url.Values{}
	v.Set(portletPrefix+"_va_nrg", p.Nrg)
	v.Set(portletPrefix+"_va_numProvvedimento", numProvv)
	v.Set(portletPrefix+"_va_tipoProvvedimento", TipoProvvFromNomeFile(p.NomeFile))
	v.Set(portletPrefix+"_va_schema", p.Schema)

	body, status, err := c.get(ctx, appealResourceURL+"&"+v.Encode())
	if err != nil {
		return nil, 0, err
	}
	if status != http.StatusOK {
		return nil, 0, fmt.Errorf("verifica appello: HTTP %d", status)
	}
	var page appealPage
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, 0, fmt.Errorf("verifica appello: risposta non valida: %w", err)
	}
	return page.Content, page.TotalElements, nil
}
