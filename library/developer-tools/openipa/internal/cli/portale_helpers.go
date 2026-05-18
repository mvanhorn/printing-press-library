// Copyright 2026 aborruso. Licensed under Apache-2.0. See LICENSE.
// Hand-written addition: shared types for PortaleServices commands — preserve on regeneration.

package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/mvanhorn/printing-press-library/library/developer-tools/openipa/internal/client"
)

// paginazioneReq is the pagination block required by all PortaleServices endpoints.
type paginazioneReq struct {
	CampoOrdinamento  string `json:"campoOrdinamento"`
	TipoOrdinamento   string `json:"tipoOrdinamento"`
	PaginaRichiesta   int    `json:"paginaRichiesta"`
	NumTotalePagine   any    `json:"numTotalePagine"`
	NumeroRigheTotali any    `json:"numeroRigheTotali"`
	PaginaCorrente    any    `json:"paginaCorrente"`
	RighePerPagina    any    `json:"righePerPagina"`
}

func newPaginazione(sortField string, page int) paginazioneReq {
	return paginazioneReq{
		CampoOrdinamento: sortField,
		TipoOrdinamento:  "asc",
		PaginaRichiesta:  page,
	}
}

type portaleResp struct {
	Errore            bool             `json:"errore"`
	Risposta          *portaleRisposta `json:"risposta"`
	DescrizioneErrore *string          `json:"descrizioneErrore"`
}

type portaleRisposta struct {
	ListaResponse json.RawMessage `json:"listaResponse"`
	Paginazione   pagResp         `json:"paginazione"`
}

type pagResp struct {
	NumTotalePagine   int `json:"numTotalePagine"`
	NumeroRigheTotali int `json:"numeroRigheTotali"`
	PaginaCorrente    int `json:"paginaCorrente"`
	RighePerPagina    int `json:"righePerPagina"`
}

func parsePortaleResp(raw json.RawMessage) (*portaleResp, error) {
	var r portaleResp
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("parsing portale response: %w", err)
	}
	if r.Errore {
		msg := "portale error"
		if r.DescrizioneErrore != nil && *r.DescrizioneErrore != "" {
			msg = *r.DescrizioneErrore
		}
		return nil, fmt.Errorf("%s", msg)
	}
	return &r, nil
}

// fetchAllPages fetches every page for a PortaleServices endpoint and returns
// the concatenated listaResponse plus the final paginazione metadata.
// body["paginazione"] is overwritten on each iteration.
func fetchAllPages(c *client.Client, path string, body map[string]any, sortField string) (json.RawMessage, pagResp, int, error) {
	body["paginazione"] = newPaginazione(sortField, 1)
	raw, status, err := c.PostJSON(path, body)
	if err != nil {
		return nil, pagResp{}, status, err
	}
	r, err := parsePortaleResp(raw)
	if err != nil {
		return nil, pagResp{}, status, err
	}
	if r.Risposta == nil {
		return json.RawMessage(`[]`), pagResp{}, status, nil
	}

	var all []json.RawMessage
	var page1 []json.RawMessage
	if err := json.Unmarshal(r.Risposta.ListaResponse, &page1); err == nil {
		all = append(all, page1...)
	}

	for p := 2; p <= r.Risposta.Paginazione.NumTotalePagine; p++ {
		body["paginazione"] = newPaginazione(sortField, p)
		raw2, _, err := c.PostJSON(path, body)
		if err != nil {
			return nil, pagResp{}, 0, err
		}
		r2, err := parsePortaleResp(raw2)
		if err != nil {
			return nil, pagResp{}, 0, err
		}
		if r2.Risposta == nil {
			continue
		}
		var pageItems []json.RawMessage
		if err := json.Unmarshal(r2.Risposta.ListaResponse, &pageItems); err == nil {
			all = append(all, pageItems...)
		}
	}

	allJSON, err := json.Marshal(all)
	if err != nil {
		return nil, pagResp{}, 0, err
	}
	finalPag := r.Risposta.Paginazione
	finalPag.PaginaCorrente = finalPag.NumTotalePagine
	return json.RawMessage(allJSON), finalPag, status, nil
}

func newPortaleClient(f *rootFlags) *client.Client {
	c := client.NewPortale(f.timeout)
	c.DryRun = f.dryRun
	return c
}

// portaleOutput renders a PortaleServices list response with consistent envelope.
func portaleOutput(w io.Writer, flags *rootFlags, path, resource string, status int,
	items json.RawMessage, pag pagResp) error {

	if wantsHumanTable(w, flags) {
		var list []map[string]any
		if err := json.Unmarshal(items, &list); err == nil && len(list) > 0 {
			if err2 := printAutoTable(w, list); err2 == nil {
				return nil
			}
		}
	}

	filtered := items
	if flags.selectFields != "" {
		filtered = filterFields(filtered, flags.selectFields)
	} else if flags.compact {
		filtered = compactFields(filtered)
	}
	envelope := map[string]any{
		"action":   "post",
		"resource": resource,
		"path":     path,
		"status":   status,
		"success":  status >= 200 && status < 300,
		"meta": map[string]any{
			"total": pag.NumeroRigheTotali,
			"page":  pag.PaginaCorrente,
			"pages": pag.NumTotalePagine,
		},
	}
	var parsed any
	if err := json.Unmarshal(filtered, &parsed); err == nil {
		envelope["data"] = parsed
	}
	envelopeJSON, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	return printOutput(w, json.RawMessage(envelopeJSON), true)
}
