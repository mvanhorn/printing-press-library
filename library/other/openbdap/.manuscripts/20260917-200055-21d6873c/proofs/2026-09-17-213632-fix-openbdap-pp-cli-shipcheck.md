# Shipcheck openbdap-pp-cli

## Esito finale
`shipcheck --dir <work> --spec <spec> --research-dir <run>` esce 0 con 7 legs su 7 in PASS:
verify, validate-narrative, dogfood, workflow-verify, apify-audit, verify-skill, scorecard.

- Scorecard: **92/100, grado A** (prima iterazione: 91/100).
- Sample Output Probe dal vivo: **7/7** funzioni originali (prima iterazione: 5/7).
- dogfood `novel_features_check`: planned 7, found 7, missing nessuno.
- `go test ./...` verde, compreso il nuovo pacchetto di test sulla logica pura.

## Blocchi trovati e correzioni applicate
1. **`cig` invisibile al controllo delle funzioni originali.** Il rilevatore cerca un `Use:` letterale nel sorgente e il comando veniva costruito da una funzione condivisa con nome a runtime. Corretto: `cup` e `cig` dichiarano il proprio `Use` letterale e condividono la logica tramite `configuraRicercaMOP`. Da 6/7 a 7/7.
2. **`serie` e `campi` restituivano output vuoto al campione dal vivo.** Erano corretti ma leggevano un archivio locale vuoto. Corretto su due fronti: l'output vuoto dei comandi locali ora e' un involucro che riporta la richiesta, il numero di risultati e una nota che indica il comando da lanciare (`allinea`, oppure `campi --aggiorna`); e l'archivio predefinito e' stato popolato. Da 5/7 a 7/7.
3. **Codici delle famiglie MOP sbagliati.** Pagamenti e partecipanti erano stati dedotti come `pag` e `par`, mentre nel catalogo sono `sal` e `pga`: il dossier restava senza due sezioni su cinque. Corretto contro i dati reali.
4. **Allineamento troncato dal timeout globale.** I comandi lunghi non legano piu' l'intero contesto a `--timeout`, che il client generato applica gia' a ogni richiesta.
5. **Timeout intermittenti del portale.** Il primo allineamento completo perdeva 132 dataset su 3857. Aggiunto un secondo tentativo per dataset: al passaggio successivo gli allineati salgono a 3820 su 3857.
6. **Date di aggiornamento lette con un solo formato.** `novita` scartava in silenzio le righe con precisione diversa. Ora prova piu' formati, RFC3339 compreso; verificato che tutti i 128 dataset del tema opere pubbliche producono una data valida.

## Correttezza dal vivo delle funzioni approvate
- `dossier I77H11000120009`: progetto 1, gare 13, pagamenti 3, soggetti titolari 1, nessuna sezione mancante.
- `opere --cf 80208450587 --solo-conteggio`: 13.489 opere, conteggio reale da `count=true`.
- `conta "Progetti Opere Pubbliche MOP - Totale" --dove "Codice Fiscale Titolare=80208450587"`: 13.489, coerente con il precedente.
- `righe ... --dove "Codice CUP=I77H11000120009" --campi "Codice CUP,Descrizione Titolare"`: una riga, colonne con nomi leggibili.
- `cup`, `cig`, `serie`, `mop`, `campi`, `novita`, `cerca`: verificati sul campione dal vivo di scorecard.

## Prima e dopo
- verify: PASS in entrambe le iterazioni.
- scorecard: 91 -> 92.
- campione dal vivo: 5/7 -> 7/7.

## Gap noti, non bloccanti
- `dead_code 1/5`: cinque funzioni di supporto emesse dal generatore e non usate (`handleBinaryResponseDelivery`, `readSecretFromStdin`, `retainCLIQueryParams`, `retainExplicitQueryParams`, `successfulNoop`). Sono in file templati: rimuoverle significherebbe divergere dal generatore.
- L'indice dei campi copre i dataset per cui e' stato richiesto (`campi --aggiorna`), non tutti e 3857: servirebbe una chiamata al servizio per ciascuno.
- La versione "Totale" esiste solo per i progetti: per le altre famiglie MOP la ricerca resta un fan-out regionale.

## Verdetto
**ship**
