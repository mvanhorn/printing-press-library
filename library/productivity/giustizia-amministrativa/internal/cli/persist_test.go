package cli

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mvanhorn/printing-press-library/library/productivity/giustizia-amministrativa/internal/gaclient"
	"github.com/mvanhorn/printing-press-library/library/productivity/giustizia-amministrativa/internal/store"
)

// TestPersistProvvedimentiConservaTestoEMetadati fissa la regola che la review
// ha fatto emergere due volte: una riga in arrivo che non porta il testo o i
// metadati non deve cancellare quelli gia' nello store. Una riga di ricerca non
// porta ne' l'uno ne' gli altri, quindi senza questo ogni ricerca costringerebbe
// il lettore successivo a riscaricare dal portale cio' che era gia' in locale.
func TestPersistProvvedimentiConservaTestoEMetadati(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	completo := gaclient.Provvedimento{
		Ecli:     "ECLI:IT:TARLAZ:2026:14259SENT",
		Idprovv:  "abc",
		FullText: "testo integrale del provvedimento",
		Meta:     &gaclient.Meta{Urn: "urn:nir:tar.lazio;sezione.3Q:sentenza:00000-0000"},
	}
	persistProvvedimenti(st, []gaclient.Provvedimento{completo})

	// La stessa riga come torna da una ricerca: nessun testo, nessun metadato.
	daRicerca := gaclient.Provvedimento{Ecli: completo.Ecli, Idprovv: completo.Idprovv, Snippet: "..."}
	persistProvvedimenti(st, []gaclient.Provvedimento{daRicerca})

	raw, err := st.Get("provvedimenti", completo.Ecli)
	if err != nil || len(raw) == 0 {
		t.Fatalf("riga assente dallo store: %v", err)
	}
	var dopo gaclient.Provvedimento
	if err := json.Unmarshal(raw, &dopo); err != nil {
		t.Fatal(err)
	}
	if dopo.FullText != completo.FullText {
		t.Errorf("testo perso: %q", dopo.FullText)
	}
	if dopo.Meta == nil || dopo.Meta.Urn != completo.Meta.Urn {
		t.Errorf("metadati persi: %+v", dopo.Meta)
	}
}
