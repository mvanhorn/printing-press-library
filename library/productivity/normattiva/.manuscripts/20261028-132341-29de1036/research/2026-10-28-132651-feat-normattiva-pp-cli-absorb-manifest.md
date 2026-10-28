# Normattiva CLI Absorb Manifest

Sources: normattiva-sdk (IreneBurresi), pipeworx mcp-it-normattiva, adellorto/normattiva-mcp, avvocati-e-mac/normattiva-mcp, SimonBerg255/gov-it-legal-mcp, ondata/normattiva_2_md, dej611/italian-laws-history.

### Absorbed (match or beat everything that exists)
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---------|-----------|-------------------|-------------|
| 1 | Full-text search (ricerca semplice) | normattiva-sdk `cerca`, adellorto search_acts | normattiva-pp-cli cerca | --json/--select, facets shown, result rows carry derived URN, cached to SQLite |
| 2 | Advanced search (type, title, dates, vigenza, class, year/number) | normattiva-sdk `avanzata`, pipeworx it_normattiva_search | (behavior in normattiva-pp-cli cerca) advanced flags --tipo --titolo --dal --al --pubblicato-dal --pubblicato-al --vigente-al --classe --anno --numero switch to /ricerca/avanzata | one command, human flags, Italian date parsing |
| 3 | Read article text by URN (with ~artN, @originale, !vig=) | normattiva-sdk `testo`, pipeworx get_article, norm `urn` | normattiva-pp-cli testo | HTML→clean text/markdown, vigenza window shown, cached |
| 4 | Read by Gazzetta coordinates (codice redazionale + data GU) | normattiva-sdk dettaglio_da_gazzetta, adellorto read_article | (behavior in normattiva-pp-cli testo) --codice/--data-gu | falls back automatically when no URN form |
| 5 | Citation aliases for well-known acts ("codice civile", "costituzione", "gdpr") | normattiva-sdk `noti`, norm `fonti` | normattiva-pp-cli fonti | offline list + fuzzy lookup, used by every command that takes an act |
| 6 | URN:NIR compose/parse offline | normattiva-sdk `identificatore` | normattiva-pp-cli urn | parses human citations "d.lgs. 196/2003 art. 13" too |
| 7 | Citation link builder (markdown citation + normattiva.it URL) | norm `link` | normattiva-pp-cli cita | markdown/plain/json, permalink with !vig |
| 8 | Article history (all versions of an article) | normattiva-sdk `cronologia` | normattiva-pp-cli cronologia | version windows table, cached per version |
| 9 | Acts updated in date range | normattiva-sdk `aggiornati`, adellorto recent_updates | normattiva-pp-cli aggiornati | --since 7d durations, auto-chunks >12 months windows |
| 10 | Read whole act (enumerate articles with budget) | adellorto read_act/list_articles | normattiva-pp-cli atto | streaming article-by-article, --max-chars budget, markdown with YAML front matter |
| 11 | List articles of an act | adellorto list_articles | (behavior in normattiva-pp-cli atto) --indice | TOC only |
| 12 | Dictionaries (denominazioni, classi, estensioni) | normattiva-sdk `dizionario`, adellorto resources | (generated endpoint) tipologiche estensioni/classe-provvedimento/denominazione-atto | cached |
| 13 | Predefined collections + predefined searches | normattiva-sdk `collezioni`, adellorto resources | (generated endpoint) collections collection-predefinite; ricerca predefinita | — |
| 14 | Download prebuilt collection ZIP (O/V/M, formats) | normattiva-sdk `scarica` | normattiva-pp-cli scarica | progress, output path, format validation |
| 15 | Async export (nuova-ricerca → conferma → check-status poll → download) | normattiva-sdk `esporta` | normattiva-pp-cli esporta | single command with polling and --wait |
| 16 | Health check | norm `doctor` | (behavior in normattiva-pp-cli doctor) probes GET /tipologiche/estensioni | — |
| 17 | Law → Markdown for LLMs (front matter, headings, ((modifiche)) markers) | ondata normattiva_2_md | (behavior in normattiva-pp-cli testo) --format markdown | no Python, no Exa key |
| 18 | MCP server exposure | all MCPs | (generated endpoint) MCP server binary normattiva-pp-mcp mirrors every command as a tool | single binary |

### Transcendence (only possible with our approach)
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---------|---------|--------------|------------------------|------------------|
| 1 | Citation checker (10/10) | verifica | hand-code | Regex citation extraction + alias table + N point-in-time lookups classified into resolved/missing/in force/ended, with non-zero exit on failure | Use this command to check citations that already appear in a piece of text (or a list) against Normattiva. Do NOT use it to build a citation or link for one known norm; use 'cita' instead. Do NOT use it to only parse or compose a URN offline; use 'urn' instead. |
| 2 | Article diff between dates (10/10) | diff | hand-code | Two !vig= / @originale fetches rendered to text and word-diffed, versions cached in SQLite | Use this command to see exactly what changed in an article between two dates, or between the original and the current text. Do NOT use it to list every version window of an article; use 'cronologia' instead. Do NOT use it to read one version; use 'testo' instead. |
| 3 | Watchlist change report (8/10) | novita | hand-code | Local watchlist (managed with segui add/list/rm) + article snapshots joined against the aggiornati feed since last check | Use this command to see which acts on your own watchlist changed since the last check. Do NOT use it to list every act in Italy updated in a date range; use 'aggiornati' instead. Do NOT use it to see the wording change of one article; use 'diff' instead. |
| 4 | Context pack / dossier (7/10) | fascicolo | hand-code | Many citations from different acts pinned to one reference date, composed into one LLM-ready markdown document | Use this command to assemble several articles from different acts into one document pinned to one date. Do NOT use it to read or export a whole single act; use 'atto' instead. Do NOT use it to read one article; use 'testo' instead. |

Stubs: none.
