package cli

import (
	"encoding/json"
	"testing"
)

func TestSelectNonPuoTogliereGliAvvisi(t *testing.T) {
	// La forma è quella di `deputato profilo … --select tipo,data`: `data`
	// risolve in radice, `tipo` no, e il ramo di recupero di filterFieldsRec
	// riammette solo gli array che rispondono ai nomi rimasti. `conteggio`
	// (oggetto) e `troncato` (array di stringhe nude) cadevano entrambi, e con
	// loro l'unico segnale che la risposta copriva 46 atti su 84.
	originale := json.RawMessage(`{"atti":[{"tipo":"interrogazioni","data":"3.08.26","titolo":"x"}],"conteggio":{"interrogazioni":30},"troncato":["interrogazioni"],"data":"2026-06-01:2026-08-14","deputato":"Tal dei Tali"}`)
	filtrato := filterFields(originale, "tipo,data")

	var prima map[string]any
	if err := json.Unmarshal(filtrato, &prima); err != nil {
		t.Fatal(err)
	}
	if _, c := prima["troncato"]; c {
		t.Fatal("premessa del test caduta: filterFields non toglie più troncato da sé")
	}

	var dopo map[string]any
	if err := json.Unmarshal(preservaAvvisi(originale, filtrato), &dopo); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"troncato", "conteggio"} {
		if _, c := dopo[k]; !c {
			t.Errorf("%s non è stato preservato: %v", k, dopo)
		}
	}
	// Ciò che il chiamante non ha chiesto e non è un avviso resta fuori.
	if _, c := dopo["deputato"]; c {
		t.Errorf("preservaAvvisi ha riportato dentro un campo che non è un avviso: %v", dopo)
	}
	// E i dati chiesti non si toccano.
	atti, _ := dopo["atti"].([]any)
	if len(atti) != 1 {
		t.Errorf("atti alterati: %v", dopo["atti"])
	}
}

func TestPreservaAvvisiNonDuplicaQuelliGiaPresenti(t *testing.T) {
	originale := json.RawMessage(`{"risultati":[{"numero":"1"}],"troncato":true,"hint":"quello vero"}`)
	filtrato := json.RawMessage(`{"risultati":[{"numero":"1"}],"troncato":true}`)
	var dopo map[string]any
	if err := json.Unmarshal(preservaAvvisi(originale, filtrato), &dopo); err != nil {
		t.Fatal(err)
	}
	if dopo["troncato"] != true {
		t.Errorf("troncato alterato: %v", dopo["troncato"])
	}
	if dopo["hint"] != "quello vero" {
		t.Errorf("hint non recuperato: %v", dopo["hint"])
	}
}

func TestPreservaAvvisiSoloInRadice(t *testing.T) {
	// Dentro le righe di un array `nota` è un campo come gli altri e --select
	// deve poterlo togliere: preservaAvvisi non scende.
	originale := json.RawMessage(`[{"numero":"1","nota":"dentro la riga"}]`)
	filtrato := filterFields(originale, "numero")
	var dopo []map[string]any
	if err := json.Unmarshal(preservaAvvisi(originale, filtrato), &dopo); err != nil {
		t.Fatal(err)
	}
	if _, c := dopo[0]["nota"]; c {
		t.Errorf("nota reintrodotta dentro una riga: %v", dopo[0])
	}
}
