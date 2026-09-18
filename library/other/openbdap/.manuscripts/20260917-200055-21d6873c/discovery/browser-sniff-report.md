# OpenBDAP browser-sniff report

## User Goal Flow
- Goal: sfogliare il catalogo e ottenere le righe di un dataset.
- Steps: (1) home; (2) pagina dataset "Progetti Opere Pubbliche MOP - Totale"; (3) tab Tabella (anteprima righe); (4) tab Scarica; (5) lista dataset per tema; (6) pagina API.
- Coverage: 6/6.

## Pages & Interactions
1. `/` home: 13 temi con link "Consulta dataset".
2. `/content/progetti-opere-pubbliche-mop-totale`: tab Descrizione/Tabella/Scarica, link a dataset correlati (regionali).
3. Click "Tabella": XHR OData.
4. Click "Scarica": iframe `/metadata_download_page/<nid>/csv/<nid2>/<odata-id>@rgs`.
5. Click "DATASET" → `/tema/<slug>`: lista SSR Drupal con faccette Tema/Banca dati/Parola chiave/Licenza, ordinamento (rilevanza, titolo, ultimo aggiornamento, visualizzazioni, ricerche, download).
6. `/content/api`: documenta CKAN api/1, api/2 rest e api/3 action (dataset, group, tag, license).

## Browser-Sniff Configuration
- Backend: agent-browser 0.31.1, headless, anonymous; HAR 437 requests.
- Pacing: manual interactions, ~1 req/s on API calls; no 429.
- Proxy pattern: not detected.
- `cli-printing-press browser-sniff` failed: "at least one resource is required" (XHR URLs use `//ODataProxy` double slash; rest is Drupal HTML). Spec will be hand-authored from verified direct calls.

## Endpoints Discovered
| Method | Path | Status | Content-Type |
|---|---|---|---|
| GET | /ODataProxy/MdData('{id}@rgs') | 200 | application/json (with Accept) |
| GET | /ODataProxy/MdData('{id}@rgs')/DataColumns | 200 | application/json |
| GET | /ODataProxy/MdData('{id}@rgs')/DataRows?$select=…&$top=50&$skip=0 | 200 | application/json |
| GET | /ODataProxy/MdData('{id}@rgs')/DataRows?$select=…&count=true | 200 | application/json → `{"Tot": N}` |
| GET | /viewer/multidimensional/{id}%40rgs/{nid} | 200 | text/html (grid bootstrap) |
| GET | /sites/all/modules/spodata/metadata/blocks/download.php?_url=… | 200 | text/html (iframe wrapper) |
| GET | /SpodCkanApi/api/2/rest/dataset, /group/{name}, api/3/action/license_list | 200 | application/json |

## Traffic Analysis
- Protocols: rest_json (OData v2 verbose JSON `d` envelope), ssr html (Drupal).
- Auth signals: none. Session cookie `SPODPRODUZIONE` set but not required.
- Protection: none.
- New vs brief: **`count=true` query param on DataRows returns real total row count (`Tot`), honoring `$filter`** (e.g. 561455 total, 13489 for CF 80208450587). Standard `$inlinecount` is broken, so this is the only count path. api/2 rest `dataset/{name}` returns "Non trovato" (names not resolvable either).
- Candidate commands: conta righe; landing URL `/content/<slug>` for dataset link.

## Coverage Analysis
- Covered: catalog facets (via CKAN store), dataset detail, rows preview, count, download.
- Not needed: Drupal SSR listing (CKAN package_list + package_show gives full catalog).

## Response Samples
- DataRows count: `{"d":{"results":[{"__metadata":{…},"Tot":561455}],"__count":"0"}}`
- DataRows row: `{"row_id":0,"Cccodice_cup_1267962549":"I77H11000120009", …}`

## Rate Limiting Events
- None.

## Authentication Context
- No authenticated session used.
