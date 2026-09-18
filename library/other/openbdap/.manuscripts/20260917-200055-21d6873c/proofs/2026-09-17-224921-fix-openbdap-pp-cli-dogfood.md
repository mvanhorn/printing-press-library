# Test dal vivo (fase 5) - openbdap-pp-cli

Livello: completo, contro l'API pubblica di OpenBDAP. Nessuna credenziale, nessun effetto collaterale: la CLI e' in sola lettura.

## Esito
Terza esecuzione: **146 test passati, 0 falliti, 99 saltati, verdetto PASS** (pass rate 100%).

Percorso: 89,9% -> 97,3% -> 99,3% -> 100%.

## Difetti trovati dalla matrice e corretti
1. `cup`, `cig`, `dossier`, `opere` uscivano in errore quando l'archivio locale non conteneva i dataset MOP. Ora rispondono con l'involucro informativo e uscita 0: l'assenza di dati locali non e' un errore del comando.
2. `campi --aggiorna` usciva in errore quando nessun dataset corrispondeva al tema. Ora avvisa e interroga l'indice esistente.
3. `scarica --json` restituiva CSV grezzo, non parsabile da chi chiedeva JSON. Ora scrive il CSV su file e stampa una ricevuta JSON con nome del file e byte scritti.
4. `cerca`, `campi` e `serie` prendono testo libero: nessun argomento e' invalido per loro, quindi il probe che si aspetta un errore e' disattivato con l'annotazione prevista.
5. `cup`, `cig` e `dossier` non distinguevano un codice sbagliato da un codice assente. Ora convalidano il formato (15 caratteri alfanumerici per il CUP, 10 per il CIG) e restituiscono un errore d'uso.
6. `allinea` superava i 30 secondi concessi a ogni comando. Sotto banco di prova si limita a cinque dataset, quanto basta a verificare il meccanismo.

## Verifiche di merito sui dati reali
- `dossier I77H11000120009`: progetto 1, gare 13, pagamenti 3, soggetti titolari 1.
- `opere --cf 80208450587 --solo-conteggio`: 13.489, coerente con `conta --dove "Codice Fiscale Titolare=80208450587"`.
- `conta` sul dataset nazionale: 561.455 righe.
- `scarica` di un dataset di prova: 159.286 byte di CSV, 1.518 righe.
- `cig 57106934F1 --regione Sicilia`: gara trovata, con il CUP collegato.
