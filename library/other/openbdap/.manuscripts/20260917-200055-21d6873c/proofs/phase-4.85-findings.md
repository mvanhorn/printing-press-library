# Fase 4.85 - revisione dell'output

Esito: WARN, quattro segnalazioni, tutte corrette in sessione.

1. `meta.source` riportava l'annotazione statica del comando invece dell'origine reale: `dossier` dichiarava `live` anche quando rispondeva senza toccare la rete. Corretto: i percorsi che rispondono dall'archivio locale, o dalla sua assenza, dichiarano `local`.
2. `allinea` non scriveva nulla per minuti. Corretto: avanzamento ogni 250 dataset e riepilogo finale su standard error, durata anche nell'output JSON.
3. Il campione dal vivo passava su un archivio vuoto, quindi misurava l'eco della richiesta e non il recupero dei dati. Corretto di fatto popolando l'archivio predefinito; resta una nota per il generatore, che non lancia il comando di bootstrap prima del campionamento.
4. `cig` cambiava forma dell'output fra archivio assente e nessun risultato. Corretto: involucro identico nei due casi. L'esempio nell'aiuto usa ora un CIG reale del catalogo (57106934F1) invece di un codice inventato.
