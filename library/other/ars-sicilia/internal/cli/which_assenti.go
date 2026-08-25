// pp:helper
// Le domande a cui la risposta è «il portale non lo pubblica»: dirlo vale più
// di tacere, e molto più di un comando plausibile.

package cli

import "strings"

// capacitaAssente è una domanda ricorrente a cui questa CLI non può
// rispondere, con il motivo e — dove esiste — la cosa più vicina che si può
// fare davvero.
type capacitaAssente struct {
	Capacita string   `json:"capacita"`
	Perche   string   `json:"perche"`
	Invece   string   `json:"invece,omitempty"`
	chiavi   []string // token che identificano la domanda
}

// whichAssenti sono i limiti della fonte, non della CLI: nessun comando li
// coprirà, e sono esattamente le domande che un giornalista o un ricercatore
// pone per prime.
//
// Perché stanno qui invece che in un documento: `which` è il punto in cui
// qualcuno sta cercando il comando per fare X, ed è l'unico momento in cui la
// risposta «X non si può» arriva quando serve. Prima di questo, «come ha
// votato un deputato» riceveva `deputato profilo` presentato con la stessa
// confidenza dei match veri, poi — corretto il confronto — riceveva silenzio.
// Nessuna delle due è la risposta giusta, che è: il portale non pubblica i
// voti nominali.
var whichAssenti = []capacitaAssente{
	{
		Capacita: "come ha votato un singolo deputato",
		Perche:   "il portale ARS non pubblica i voti nominali in forma di dato: nessuno dei 12 archivi ha un campo voto, e non esiste una sezione votazioni.",
		Invece:   "l'esito delle votazioni è raccontato nella prosa dei resoconti d'aula: `resoconti cerca --testo \"votazione per appello nominale\"` trova le sedute in cui si è votato così, e il testo integrale sta nel PDF (`resoconti get`, campo pdf_url).",
		chiavi:   []string{"votato", "votazione", "voti", "voto", "votano", "votare"},
	},
	{
		Capacita: "presenze e assenze in aula",
		Perche:   "il portale non pubblica i tabellini di presenza: non c'è archivio né campo che dica chi era in aula.",
		Invece:   "`analytics --type resoconti --group-by oratore` dice chi è intervenuto e quante volte, che è cosa diversa dalla presenza ma è l'unico segnale di partecipazione che il dato consente.",
		chiavi:   []string{"presenza", "presenze", "assente", "assenti", "assenza", "assenteismo", "presente"},
	},
	{
		Capacita: "gli emendamenti a un atto",
		Perche:   "non esiste un archivio degli emendamenti: il portale non li pubblica come atti a sé.",
		// Indicava `legge cronologia`, che gli emendamenti non li ha mai
		// raccolti: chi cercava un ripiego finiva su un comando che non
		// risponde a quella domanda. La cosa più vicina che esiste davvero è
		// la prosa dei resoconti d'aula, dove gli emendamenti sono citati uno
		// per uno ma solo come testo.
		Invece: "gli emendamenti sono citati nella prosa dei resoconti d'aula: prendi la seduta con `ddl iter` o `legge cronologia` e leggine il testo con `resoconti get <legisl> <seduta>` (sulle sedute recenti il testo sta nel PDF indicato da `pdf_url`). Sono citazioni dentro un discorso, non un elenco interrogabile.",
		chiavi: []string{"emendamento", "emendamenti", "subemendamenti"},
	},
	{
		Capacita: "spese, bilancio e costi dell'Assemblea",
		Perche:   "questa CLI copre gli atti parlamentari; i dati di spesa e i costi di funzionamento dell'ARS stanno nella sezione amministrazione trasparente del sito istituzionale, che non è coperta.",
		Invece:   "`leggi cerca --testo \"bilancio\"` trova le leggi di bilancio e finanziarie, cioè le norme sulla spesa regionale, non le spese dell'Assemblea.",
		chiavi:   []string{"spesa", "spese", "spende", "costi", "costo", "stipendio", "stipendi", "vitalizio", "vitalizi", "indennita", "indennità", "rimborsi"},
	},
	{
		// Quel che resta scoperto dopo l'arrivo di `gruppi`: l'anagrafica dei
		// gruppi c'è, lo schieramento no. La voce era più larga e copriva
		// anche «gruppo»/«partito»; ristretta e non tolta, perché toglierla
		// del tutto lasciava «maggioranza» e «opposizione» senza match e
		// senza spiegazione — cioè col silenzio, che è quello che questo
		// elenco esiste per evitare.
		Capacita: "l'appartenenza a maggioranza o opposizione",
		Perche:   "nessuna fonte del portale dichiara lo schieramento: i gruppi parlamentari sono un'anagrafica, non una coalizione, e il sostegno al Governo regionale non è un campo pubblicato da nessuna parte.",
		Invece:   "`gruppi elenco --legisl 18` dà i gruppi e `gruppi get <gruppo>` la loro composizione: da lì lo schieramento si ricostruisce a mano, sapendo quali gruppi sostengono il Governo — un'informazione che va portata da fuori.",
		chiavi:   []string{"maggioranza", "opposizione", "schieramento", "coalizione", "coalizioni"},
	},
}

// assentiPerQuery torna le capacità assenti che la domanda tocca. Il
// confronto è lo stesso del ranker — parola intera sotto le quattro lettere,
// prefisso sopra — così «votazioni» aggancia «votazione» e «chi» non aggancia
// niente.
func assentiPerQuery(query string) []capacitaAssente {
	var out []capacitaAssente
	for _, a := range whichAssenti {
		for _, k := range a.chiavi {
			if contienePrefissoDiParola(strings.ToLower(query), k) {
				out = append(out, a)
				break
			}
		}
	}
	return out
}
