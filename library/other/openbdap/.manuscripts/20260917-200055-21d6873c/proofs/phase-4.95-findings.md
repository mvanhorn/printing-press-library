# Fase 4.95 - revisione del codice

Percorso scelto: dispatch diretto di un sottoagente revisore (correttezza, sicurezza, manutenibilita') sui file scritti a mano.

Convergenza: rilievi chiusi al primo giro. 1 grave, 7 medi e 9 minori corretti in sessione; `go vet ./...` e `go test ./... -race -count=1` verdi prima e dopo.

## Correzioni principali
- **Grave**: un nome di colonna vuoto risolveva sulla prima colonna del dataset, perche' la ricerca parziale accetta la stringa vuota. `--dove "=valore"` filtrava quindi su una colonna a caso e restituiva zero come se fosse un fatto. Ora la chiave vuota non risolve e la condizione viene rifiutata; due test coprono il caso.
- Il conteggio non distingueva un filtro rifiutato da uno zero reale: il servizio risponde 200 con lista vuota nel primo caso e con `Tot: 0` nel secondo. Ora la lista vuota e' un errore esplicito.
- `--limite` oltre la pagina massima troncava in silenzio: ora impagina anche senza `--tutte`.
- `scarica` costruiva un client HTTP proprio e ignorava `--timeout`; ora ne eredita il valore come attesa delle intestazioni e verifica il tipo di contenuto, perche' il portale risponde 200 con HTML quando l'identificativo non esiste.
- L'uguaglianza OData era costruita in tre punti con comportamenti diversi: ora c'e' un solo `filtroUguale`.
- `opere` contava i dataset che non rispondevano senza conservare il perche': ora l'elenco degli errori resta nella risposta.
- Il dossier deduceva la regione dal nome del titolare senza dirlo: ora la risposta porta `regione_dedotta`.
- Le righe dell'archivio illeggibili sparivano in silenzio mentre il conteggio si diceva completo: ora vengono contate e segnalate.
- `allinea` usciva con 0 anche quando non allineava nulla: ora esce in errore, cosi' `allinea && cerca` non prosegue su un archivio vuoto.
- Rimosso codice morto (`risolviLocale`) e parametri mai letti negli helper dell'archivio.

## Candidati per il generatore, non corretti qui
- Il comando promosso per un endpoint con `response_format: csv` viene generato come lettore JSON e fallisce con "API returned a non-JSON response". La CLI usa una versione scritta a mano.
- `dead_code 1/5` nello scorecard deriva da cinque funzioni di supporto emesse dal generatore e mai usate.
- Il campione dal vivo dello scorecard non lancia il comando di bootstrap del CLI prima di campionare, quindi misura comandi locali su un archivio vuoto.
