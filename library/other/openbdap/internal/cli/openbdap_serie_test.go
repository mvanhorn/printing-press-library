// Copyright 2026 aborruso and contributors. Licensed under Apache-2.0. See LICENSE.

package cli

import "testing"

func TestDataAggiornamento(t *testing.T) {
	validi := []string{
		"2026-09-04T12:20:22.000000",
		"2026-09-04T12:20:22",
		"2026-09-04T12:20:22Z",
		"2026-09-04 12:20:22",
		"2026-09-04",
	}
	for _, v := range validi {
		if _, ok := dataAggiornamento(v); !ok {
			t.Errorf("dataAggiornamento(%q) non riconosciuta", v)
		}
	}
	for _, v := range []string{"", "   ", "non una data"} {
		if _, ok := dataAggiornamento(v); ok {
			t.Errorf("dataAggiornamento(%q) non doveva essere riconosciuta", v)
		}
	}
	// Le due forme equivalenti devono dare lo stesso istante: altrimenti la
	// finestra di 'novita' cambierebbe a seconda della precisione pubblicata.
	a, _ := dataAggiornamento("2026-09-04T12:20:22.000000")
	b, _ := dataAggiornamento("2026-09-04T12:20:22")
	if !a.Equal(b) {
		t.Errorf("%v e %v dovrebbero coincidere", a, b)
	}
}

func TestOrdinato(t *testing.T) {
	got := ordinato(map[string]bool{"2024": true, "2022": true, "2023": true})
	atteso := []string{"2022", "2023", "2024"}
	if len(got) != len(atteso) {
		t.Fatalf("ordinato = %v", got)
	}
	for i := range atteso {
		if got[i] != atteso[i] {
			t.Fatalf("ordinato = %v, atteso %v", got, atteso)
		}
	}
	if len(ordinato(nil)) != 0 {
		t.Error("una mappa vuota deve dare un elenco vuoto")
	}
}
