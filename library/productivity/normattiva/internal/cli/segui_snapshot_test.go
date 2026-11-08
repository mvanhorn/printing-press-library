// Copyright 2026 Luigi Pastore and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/normattiva/internal/cliutil/testenv"
)

func TestSeguiExtendKeepsExistingSnapshots(t *testing.T) {
	testenv.Isolate(t)
	art5 := "testo originale articolo cinque"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			URN string `json:"urn"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		html := "<div>Atto.</div>"
		switch {
		case strings.Contains(body.URN, "~art5"):
			html = "<div>" + art5 + "</div>"
		case strings.Contains(body.URN, "~art6"):
			html = "<div>articolo sei</div>"
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code":    200,
			"message": "ok",
			"data": map[string]any{
				"atto": map[string]any{
					"titolo":                    "Legge 241",
					"articoloHtml":              html,
					"articoloDataInizioVigenza": "19900807",
					"articoloDataFineVigenza":   "99999999",
				},
			},
		})
	}))
	defer srv.Close()
	t.Setenv("NORMATTIVA_BASE_URL", srv.URL)

	const act = "urn:nir:stato:legge:1990-08-07;241"
	run := func(args ...string) {
		t.Helper()
		cmd := RootCmd()
		cmd.SetArgs(append([]string{
			"--no-learn", "--no-cache", "--rate-limit", "0", "--data-source", "live", "--json",
		}, args...))
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("segui %v: %v\n%s", args, err, out.String())
		}
	}

	run("segui", "add", act, "--articoli", "5")
	before, err := snapshotHash(t, act+"~art5")
	if err != nil {
		t.Fatal(err)
	}
	art5 = "testo modificato articolo cinque"
	run("segui", "add", act, "--articoli", "6")
	after, err := snapshotHash(t, act+"~art5")
	if err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("article 5 snapshot changed from %s to %s when article 6 was added", before, after)
	}
	added, err := snapshotHash(t, act+"~art6")
	if err != nil {
		t.Fatal(err)
	}
	if added == "" {
		t.Fatal("article 6 has no baseline snapshot")
	}
	run("segui", "add", act, "--etichetta", "legge 241")
	renamed, err := snapshotHash(t, act+"~art5")
	if err != nil {
		t.Fatal(err)
	}
	if renamed != before {
		t.Fatalf("article 5 snapshot changed from %s to %s when the label changed", before, renamed)
	}
	db, err := normaStore(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var label string
	if err := db.DB().QueryRowContext(t.Context(), `SELECT etichetta FROM norma_segui WHERE urn = ?`, act).Scan(&label); err != nil {
		t.Fatal(err)
	}
	if label != "legge 241" {
		t.Fatalf("etichetta = %q, want legge 241", label)
	}
}

func snapshotHash(t *testing.T, urn string) (string, error) {
	t.Helper()
	db, err := normaStore(t.Context())
	if err != nil {
		return "", err
	}
	defer db.Close()
	var hash string
	err = db.DB().QueryRowContext(t.Context(), `SELECT hash FROM norma_snapshot WHERE urn = ?`, urn).Scan(&hash)
	return hash, err
}
