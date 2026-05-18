# IndicePA CLI Brief

## API Identity
- **Domain:** Pubblica amministrazione italiana — anagrafica di tutti gli enti, aree organizzative, unità organizzative, servizi di fatturazione e domicili digitali registrati all'IPA
- **Users:** Sviluppatori di software gestionali, integratori di sistemi PA, chi deve verificare codici IPA per fatturazione elettronica, operatori PA, giornalisti/data journalists
- **Data profile:** ~20.000 enti PA, ~40.000 AOO, ~200.000 UO, endpoint PEC/domicilio digitale aggiornati real-time
- **Auth:** `AUTH_ID` parameter — gratuita, rilasciata immediatamente su registrazione. Env var: `IPA_auth_id`
- **Base URL:** `https://indicepa.gov.it/public-ws/WSxx_NAME.php` (REST/POST multipart form)
- **Response:** JSON `{result: {cod_err, desc_err, num_items}, data: [...]}`
- **Also available:** CKAN Open Data API `https://indicepa.gov.it/ipa-dati/api/3/action/` (no auth, bulk data)

## Reachability Risk
- **Low** — API confirmed reachable. WS02_AOO, WS05_AMM, WS16_DES_AMM, WS01_SFE_CF all tested successfully with real `AUTH_ID`. CKAN API confirmed reachable without auth.

## API Surface — 22 Web Service Endpoints

### Ricerca Enti
| WS | PHP | Param principale | Descrizione |
|----|-----|-----------------|-------------|
| WS05_AMM | WS05_AMM.php | COD_AMM | Dati anagrafici completi dell'ente per codice IPA |
| WS16_DES_AMM | WS16_DES_AMM.php | DESCR | Cerca enti per descrizione (nome) → lista cod_amm |
| WS07_EMAIL | WS07_EMAIL.php | EMAIL | Cerca entità per indirizzo email |
| WS23_DOM_DIG_CF | WS23_DOM_DIG_CF.php | CF | Cerca domicilio digitale per codice fiscale ente |

### AOO (Aree Organizzative Omogenee)
| WS | PHP | Param | Descrizione |
|----|-----|-------|-------------|
| WS02_AOO | WS02_AOO.php | COD_AMM | Lista AOO di un ente |
| WS08_AOOC | WS08_AOOC.php | COD_AMM | Lista AOO con filtri (cod_aoo opzionale) |
| WS18_AOO | WS18_AOO.php | DESCR_AOO | Cerca AOO per descrizione |
| WS19_AOO | WS19_AOO.php | COD_AMM + COD_UNI_AOO | Dati AOO specifica con data cessazione |
| WS09_DOM_DIG_AOO | WS09_DOM_DIG_AOO.php | COD_AOO | Domicilio digitale attivo di una AOO |
| WS11_DOM_DIG_STOR_AOO | WS11_DOM_DIG_STOR_AOO.php | COD_AOO | Storico domicili digitali AOO |

### UO (Unità Organizzative)
| WS | PHP | Param | Descrizione |
|----|-----|-------|-------------|
| WS03_OU | WS03_OU.php | COD_AMM | Lista UO di un ente |
| WS10_DOM_DIG_OU | WS10_DOM_DIG_OU.php | COD_UNI_OU | Domicilio digitale UO (codice univoco) |
| WS12_DOM_DIG_STOR_OU | WS12_DOM_DIG_STOR_OU.php | COD_UNI_OU | Storico domicili digitali UO |
| WS13_DOM_DIG | WS13_DOM_DIG.php | EMAIL | Cerca domicilio digitale per email |

### Fatturazione Elettronica (SFE)
| WS | PHP | Param | Descrizione |
|----|-----|-------|-------------|
| WS01_SFE_CF | WS01_SFE_CF.php | CF | Uffici destinatari FE per codice fiscale |
| WS04_SFE | WS04_SFE.php | COD_AMM | Servizi fatturazione ente (stato canale, cod_uni_ou) |

### NSO (Nodi Smistamento Ordini)
| WS | PHP | Param | Descrizione |
|----|-----|-------|-------------|
| WS14_NSO_CF | WS14_NSO_CF.php | CF | Nodi NSO per codice fiscale |
| WS15_NSO | WS15_NSO.php | COD_AMM | Nodi NSO di un ente |

### PEC (Posta Elettronica Certificata)
| WS | PHP | Param | Descrizione |
|----|-----|-------|-------------|
| WS20_PEC | WS20_PEC.php | COD_AMM | PEC attive di un ente |
| WS21_PEC_ENTE_STOR | WS21_PEC_ENTE_STOR.php | COD_AMM | Storico PEC ente |
| WS22_PEC_STOR | WS22_PEC_STOR.php | PEC | Storico per indirizzo PEC |

## Top Workflows
1. **Verifica codice IPA per fattura elettronica**: CF → WS01_SFE_CF → codice destinatario IPA (cod_uni_ou) per campo "Ufficio destinatario" SDI
2. **Trova PEC di un ente**: nome ente → WS16_DES_AMM → cod_amm → WS20_PEC → indirizzo PEC
3. **Lookup completo ente**: cod_amm → WS05_AMM (info base) + WS02_AOO (struttura) + WS03_OU (uffici)
4. **Cerca entità per email**: email → WS07_EMAIL → tipo entità (AMM/AOO/UO) + cod_amm
5. **Verifica domicilio digitale NSO**: CF → WS14_NSO_CF → stato NSO per acquisti PA

## Table Stakes (competitor features)
Competitor principale: **FatturaElettronica.IndicePA** (C#/.NET, 7 endpoint su 22, nessuna CLI)
- Codice fiscale → uffici fatturazione ✓
- AOO per ente ✓
- UO per ente ✓
- SFE per ente ✓
- Dati AMM per ente ✓
- Email → entità IPA ✓

**Gap competitor**: nessuna gestione PEC, NSO, domicili digitali storici, nessuna ricerca per nome, no CLI.

## Data Layer
- **Primary entities:** enti (AMM), aoo, uo, pec, domicili_digitali, sfe_channels, nso_channels
- **Sync cursor:** snapshot periodico da CKAN bulk download + real-time WS per singoli lookup
- **FTS/search:** full-text su des_amm, des_aoo, des_ou per lookup offline
- **Local SQLite:** store per cache + ricerca offline su grandi insiemi

## Codebase Intelligence
- Source: endpoint REST/POST `/public-ws/WSxx.php`
- Auth: `AUTH_ID` form param (env: `IPA_auth_id`)
- Error codes: `cod_err=0` (OK), `cod_err=51` (param mancante), `cod_err=902` (AUTH_ID errato)
- Rate limiting: non documentato, probabilmente nessuno per uso gratuito
- Architecture: ogni WS è indipendente, stateless, risposta JSON paginata via num_items

## Product Thesis
- **Name:** `openipa` (binary: `openipa-pp-cli`)
- **Why it should exist:** Primo CLI per IPA in assoluto. Permette ai developer di fare lookup da terminale, scriptare verifiche CF→codice destinatario per fatturazione, costruire pipeline di validazione PA. L'alternativa oggi è aprire il browser su indicepa.gov.it.

## Build Priorities
1. **Ricerca ente** — `openipa ente cerca <nome>` → lista COD_AMM + WS16_DES_AMM
2. **Lookup CF per fatturazione** — `openipa fatturazione cf <CF>` → WS01_SFE_CF, output cod_uni_ou
3. **PEC lookup** — `openipa pec ente <cod_amm>` → WS20_PEC
4. **Lookup completo ente** — `openipa ente get <cod_amm>` → WS05_AMM (full info)
5. **AOO di un ente** — `openipa aoo list <cod_amm>` → WS02_AOO
6. **UO di un ente** — `openipa uo list <cod_amm>` → WS03_OU
7. **Email → entità** — `openipa cerca email <email>` → WS07_EMAIL
8. **NSO per CF** — `openipa nso cf <CF>` → WS14_NSO_CF
9. **Domicili digitali** — `openipa domicilio aoo <cod_aoo>` → WS09_DOM_DIG_AOO
10. **Sync bulk** — `openipa sync` → scarica dataset CKAN in SQLite locale per ricerca offline
