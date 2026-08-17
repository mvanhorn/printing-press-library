package icaroclient

import "testing"

// Fixture fedele al markup reale di /edem/channel.jsp: <li class="intestazione">
// da saltare, poi righe con due colonne — la prima ha il link "+" (class="goto",
// da ignorare) e il link showDDL col nome, la seconda la label simobile "N° DDL"
// col conteggio dopo </strong>.
const edemFixture = `<ul class="tabella">
  <li class="intestazione">
    <div class="intesta intesta_75"><p>Titolo</p></div>
    <div class="intesta intesta_25"><p>Numero totale DDL</p></div>
  </li>
  <li >
    <div class="intesta intesta_75"><p>
      <a href="javascript: setItem(0)" class="goto">+</a> &nbsp;
      <strong><span class="simobile">Titolo</span></strong>
      <a href="javascript: showDDL(1, 0)" >Figuccia Vincenzo</a>
    </p></div>
    <div class="intesta intesta_25"><p>
      <strong><span class="simobile">N° DDL</span></strong> 114
    </p></div>
  </li>
  <li >
    <div class="intesta intesta_75"><p>
      <a href="javascript: setItem(1)" class="goto">+</a> &nbsp;
      <strong><span class="simobile">Titolo</span></strong>
      <a href="javascript: showDDL(1, 1)" >Abbate Ignazio</a>
    </p></div>
    <div class="intesta intesta_25"><p>
      <strong><span class="simobile">N° DDL</span></strong> 16
    </p></div>
  </li>
</ul>`

func TestParseEdemRanking(t *testing.T) {
	rows, err := parseEdemRanking(edemFixture)
	if err != nil {
		t.Fatalf("parseEdemRanking: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("attese 2 righe, ottenute %d: %+v", len(rows), rows)
	}
	// L'intestazione è saltata e il link "+" (goto) non deve diventare il nome.
	if rows[0].Name != "Figuccia Vincenzo" || rows[0].Count != 114 {
		t.Errorf("riga 0 = %+v, atteso {Figuccia Vincenzo 114}", rows[0])
	}
	if rows[1].Name != "Abbate Ignazio" || rows[1].Count != 16 {
		t.Errorf("riga 1 = %+v, atteso {Abbate Ignazio 16}", rows[1])
	}
}

func TestParseEdemRanking_Empty(t *testing.T) {
	rows, err := parseEdemRanking(`<html><body><p>nessuna tabella</p></body></html>`)
	if err != nil {
		t.Fatalf("parseEdemRanking: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("attese 0 righe senza ul.tabella, ottenute %d", len(rows))
	}
}
