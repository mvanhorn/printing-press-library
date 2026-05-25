# WEB SERVICE WS00_INDICE_DEI_SERVIZI Indice e specifiche tecniche dei web-services

### Documentazione Tecnica V.2.0

### 31/03/2023

![](image_0.png)

# INDICE

- Prefazione

................................................................................................................................

......3

- 1\. Scopo del Documento

................................................................................................

............3

- 2\. Storia del Documento

.............3

- Indice dei servizi

............................4

- Specifiche tecniche

........................7

- 1\. Endpoint

### .7

- 2\. Request

### ...7

- 3\. Response

### 7

- 4\. Codici di errore

......................8

![](image_1.png)

## 1\. Prefazione

## 1.1. Scopo del Documento

Scopo del presente è la documentazione utile alla fruizione del servizio web per la ricerca di informazioni sull'IPA.

## 1.2. Storia del Documento

**Versione Data Descrizione della modifica**

- 0

02/02/2015 Nascita del documento

- 1

13/03/2015 Aggiornamento Codici Errore

- 2

19/09/2017 Aggiornamento elenco WS

- 3

26/06/2018 Aggiornamento protocollo comunicazione

- 4

26/02/2019 Aggiornamento elenco WS

- 5

12/04/2019 Aggiornamento elenco WS

- 6

13/02/2020 Aggiornamento elenco WS

- 7

28/03/2021 Nuovi endpoint

- 8

23/04/2021 Revisione stile documento

- 9

15/12/2021 Aggiornamento elenco WS

- 0

28/03/2023 Aggiornamento elenco WS

## 2\. Indice dei servizi

Nella tabella seguente sono elencati tutti i web service disponibili per estrarre informazioni dall'Indice dei domicili digitali delle Pubbliche Amministrazioni e dei gestori di pubblici servizi, IPA.

Nella tabella sono utilizzati i seguenti termini:

**Codice IPA codice che identifica un ente nell'ambito di IPA AOO Area Organizzativa Omogenea Codice AOO codice che identifica un'Area Organizzativa Omogenea nell'ambito di un ente Codice univoco AOO codice che identifica un'Area Organizzativa Omogenea nell'ambito dell'intero IPA UO Unità Organizzativa Codice univoco UO codice che identifica un'Unità Organizzativa nell'ambito dell'intero IPA NSO Nodo di Smistamento Ordini SFE Servizio di Fatturazione Elettronica** Per ciascun web service è indicato il documento di riferimento che descrive il servizio nel dettaglio.

**Documento ENDPOINT INPUT OUTPUT**

**WS01_SFE_CF** public-ws/WS01_SFE_CF.php oppure ws/WS01SFECFServices/api/WS01_SFE_CF Codice fiscale Lista UO con SFEassociato al codice fiscale indicato **WS02_AOO** public-ws/WS02_AOO.php oppure ws/WS02AOOServices/api/WS02_AOO Codice IPA Codice AOO (opzionale) Lista AOO dell'ente o dati singola AOO se specificato il relativo codice **WS03_OU** public-ws/WS03_OU.php oppure ws/WS03OUServices/api/WS03_OU Codice IPA Lista UO dell'ente **WS04_SFE** public-ws/WS04_SFE.php oppure ws/WS04SFEServices/api/WS04_SFE Codice IPA Lista UO dell'ente con SFE

### Documento

**ENDPOINT INPUT OUTPUT**

**WS05_AMM** public-ws/WS05_AMM.php oppure ws/WS05AMMServices/api/WS05_AMM Codice IPA Dati dell'Ente **WS06_OU_CODUNI** public-ws/WS06_OU_CODUNI.php oppure ws/WS06OUCODUNIServices/api/WS06_OU_COD_UNI Codice univoco UO Dati UOcomprensivi delloSFE e del NSO, se presenti **WS07_EMAIL** public-ws/WS07_EMAIL.php oppure ws/WS07EMAILServices/api/WS07_EMAIL Indirizzo di posta elettronica Lista Enti, AOO e UO a cui è associato l'indirizzo di posta elettronica **WS08_AOOC** public-ws/WS08_AOOC.php oppure ws/WS08AOOCServices/api/WS08_AOOC Codice IPA Codice AOO (opzionale) Lista AOO cessate dell'ente o dati singola AOO se specificato il relativo codice **WS09_DOM_DIG_AOO** public-ws/WS09_DOM_DIG_AOO.php oppure ws/WS09DOMDIGAOOServices/api/WS09_DOMDIGAOO Codice IPA Codice AOO (opzionale) Lista domicili digitali di un ente o di una singola AOO se specificato il relativo codice **WS10_DOM_DIG_OU** public-ws/WS10_DOM_DIG_OU.php oppure ws/WS10DOMDIGOUServices/api/WS10_DOM_DIG_OU Codice univoco UO Lista domicili digitali di una UO **WS11_DOM_DIG_STOR_AOO** public-ws/WS11_DOM_DIG_STOR_AOO.php oppure ws/WS11DOMDIGSTORAOOServices/api/WS11_DOM_DIG_STOR_AOO Codice IPA Codice AOO (opzionale) Lista domicili digitali di un ente o di una singola AOO se specificato il relativo codice, comprensiva dei domicili digitali cessati **WS12_DOM_DIG_STOR_OU** public-ws/WS12_DOM_DIG_STOR_OU.php oppure ws/WS12DOMDIGSTOROUServices/api/WS12_DOM_DIG_STOR_OU Codice univoco UO Lista domicili digitali di una UO,comprensiva dei domicili digitali cessati

![](image_2.png)

### Documento

**ENDPOINT INPUT OUTPUT**

**WS13_DOM_DIG** public-ws/WS13_DOM_DIG.php oppure ws/WS13DOMDIGServices/api/WS13_DOM_DIG Indirizzo di postaelettronica certificata Lista dei periodi in cui l'indirizzo è o è stato domicilio digitale di un ente **WS14_NSO_CF** public-ws/WS14_NSO_CF.php oppure ws/WS14NSOCFServices/api/WS14_NSO_CF Codice fiscale Lista delle UO con NSO associato al codice fiscale indicato **WS15_NSO** public-ws/WS15_NSO.php oppure ws/WS15NSOServices/api/WS15_NSO Codice IPA Lista delle UOdell'ente con NSO **WS16_DES_AMM** public-ws/WS16_DES_AMM.php oppure ws/WS16DESAMMServices/api/WS16_DES_AMM Stringa Lista enti con denominazione o acronimo contenente la stringa in input **WS18_AOO** ws/WS18AOOServices/api/WS18_AOO Codice univoco AOO Dati Aoo **WS19_AOO** ws/WS19AOOServices/api/WS19_AOO Codice IPA Codice univoco AOO (opzionale) Storico (opzionale) Lista AOO dell'ente, cessate o non cessate,o dati singola AOO se specificato il relativo codice **WS20_PEC** ws/WS20PECServices/api/WS20_PEC Codice IPA Lista delle PEC di un Ente ricercate per Codice IPA **WS21_PEC_ENTE_STOR** ws/WS21PECENTESTORServices/api/WS21_PEC_ENTE_STOR Codice IPA Lista dello storico delle PEC di un Ente ricercate per Codice IPA **WS22_PEC_STOR** ws/WS22PECSTORServices/api/ WS22_PEC_STOR PEC Storia di una PEC nell'IPA ricercato per indirizzo PEC

![](image_3.png)

**Documento ENDPOINT INPUT OUTPUT**

**WS23_DOM_DIG_CF** ws/WS23DOMDIGCFServices/api/ WS23_DOM_DIG_CF Codice Fiscale Lista dei domicili digitali di un ente a partire dal suo Codice Fiscale o da un Codice Fiscale appartenente ad un suo servizio di fatturazione elettronica.

## 3\. Specifiche tecniche

## 3.1. Endpoint

I servizi web sono disponibili su INTERNET all'indirizzo del portale IPA www.indicepa.gov.it port 443 protocollo HTTPS.

L'URL di ogni servizio si ottiene anteponendo all' ENDPOINT definito nella tabella precedente il suffisso: www.indicepa.gov.it/ Esempio per il web service WS01_SFE_CF: https://www.indicepa.gov.it:443/public-ws/WS01_SFE_CF.php oppure https://www.indicepa.gov.it:443/ws/WS01SFECFServices/api/WS01_SFE_CF

## 3.2. Request

Il protocollo da utilizzare per la Request è REST/POST.

Ogni Request deve includere il parametro AUTH_ID il cui valore ottiene attraverso il sito www.indicepa.gov.it nell'area Utente Pubblico / Web Services Pubblici previa registrazione dell'utente e di una propria casella email, la quale potrà essere utilizzata per eventuali comunicazioni da parte del Gestore iPA.

Oltre al parametro AUTH_ID ogni request dovrà contenere gli altri parametri previsti dal web-service specifico, per i dettagli si vedano i documenti presenti sul sito nella stessa area.

## 3.3. Response

La response utilizza il protocollo per il trasporto dati JSON. Per ogni web-service è scaricabile dal portale www.indicepa.gov.it nell'area Utente Pubblico / Web Services Pubblici il file contenente lo schema JSON.

Ogni Response è costituita da un oggetto e due proprietà: result e data.

La proprietà data è specifica per ogni web-service mentre la proprietà result è comune a tutti i web-service e contiene la seguente informazione.

**ATTRIBUTO TIPO CONTENUTO cod_err** number 0 in caso di nessun errore.Per informazione sugli altri codici vedere la tabella Codici di errore **desc_err** string Descrizione dell'errore **num_items** number Numero di elementi trovati sull' IPA della entità di output del web-service specifico. Esempio: per il web-service WS01_SFE_CF l'output è costituito dalle UO con SFE, quindi l'attributo conterrà il numero di UO con SFE contenuti nella Response.

## 3.4. Codici di errore

**CODICE DESCRIZIONE 0** Nessun errore **1** Parametro CF mancante **2** Parametro CF non valorizzato **3** Parametro CF valorizzato erroneamente **10** Parametro EMAIL mancante **11** Parametro EMAIL non valorizzato **12** Parametro EMAIL valorizzato erroneamente **20** Parametro COD_AMM mancante **21** Parametro COD_AMM non valorizzato **22** Parametro COD_AMM valorizzato erroneamente **23** Valore COD_AMM non presente in archivio **30** Parametro COD_UNI_OU mancante **31** Parametro COD_UNI_OU non valorizzato **32** Parametro COD_UNI_OU valorizzato erroneamente **40** Parametro COD_AOO mancante **41** Parametro COD_AOO non valorizzato **42** Parametro COD_AOO valorizzato erroneamente

**CODICE DESCRIZIONE 50** Parametro DESCR mancante **51** Parametro DESCR non valorizzato **52** Parametro DESCR valorizzato erroneamente **60** Parametro DOM_DIG mancante **61** Parametro DOM_DIG non valorizzato **62** Parametro DOM_DIG valorizzato erroneamente **70** Parametro COD_UNI_AOO mancante **71** Parametro COD_UNI_AOO non valorizzato **72** Parametro COD_UNI_AOO valorizzato erroneamente **900** Parametro AUTH_ID mancante **901** Parametro AUTH_ID non valorizzato **902** Parametro AUTH_ID valorizzato erroneamente
