// Copyright 2026 Arun Gopalakrishnan and contributors. Licensed under Apache-2.0. See LICENSE.

package ticompliance

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Statement is one of TI's signed blanket compliance statement PDFs. These are
// anonymous downloads and apply company-wide (all parts or defined subsets) —
// the manufacturer-CoC evidence pattern.
type Statement struct {
	Doc   string `json:"doc"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

// Statements is the catalog of blanket statements relevant to RoHS/REACH
// evidence, keyed by TI literature number.
var Statements = []Statement{
	{Doc: "szzq088", Title: "TI RoHS Statement", URL: "https://www.ti.com/lit/pdf/szzq088"},
	{Doc: "szzq119", Title: "TI RoHS Exempt and Expiration Statement", URL: "https://www.ti.com/lit/pdf/szzq119"},
	{Doc: "szzq087", Title: "TI REACH Statement", URL: "https://www.ti.com/lit/pdf/szzq087"},
	{Doc: "szzq077", Title: "TI Low Halogen (Green) Statement", URL: "https://www.ti.com/lit/pdf/szzq077"},
	{Doc: "szzq195", Title: "TI IEC 62474 Database Statement", URL: "https://www.ti.com/lit/pdf/szzq195"},
}

// DownloadedStatement records where one statement PDF landed on disk.
type DownloadedStatement struct {
	Statement
	Path  string `json:"path,omitempty"`
	Error string `json:"error,omitempty"`
}

// DownloadStatements fetches the catalog (or the subset named in docs) into
// dir. Per-document failures are recorded, not fatal; the call errors only
// when every download failed.
func DownloadStatements(ctx context.Context, dir string, docs []string) ([]DownloadedStatement, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	want := map[string]bool{}
	for _, d := range docs {
		want[strings.ToLower(strings.TrimSpace(d))] = true
	}
	client := &http.Client{Timeout: 60 * time.Second}
	var out []DownloadedStatement
	okCount := 0
	for _, s := range Statements {
		if len(want) > 0 && !want[s.Doc] {
			continue
		}
		d := DownloadedStatement{Statement: s}
		path := filepath.Join(dir, "TI_"+s.Doc+".pdf")
		if err := downloadPDF(ctx, client, s.URL, path); err != nil {
			d.Error = err.Error()
		} else {
			d.Path = path
			okCount++
		}
		out = append(out, d)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no statements matched %v", docs)
	}
	if okCount == 0 {
		return out, fmt.Errorf("all %d statement downloads failed", len(out))
	}
	return out, nil
}

func downloadPDF(ctx context.Context, client *http.Client, url, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", productUserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.Contains(ct, "pdf") {
		return fmt.Errorf("unexpected content type %q (expected PDF)", ct)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	n, err := io.Copy(f, resp.Body)
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("empty PDF body")
	}
	return nil
}
