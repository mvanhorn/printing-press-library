# Future Ideas

## Sincronizzazione locale e analisi aggregata

Aggiungere un comando `sync` che scarica tutti gli enti IPA in un database SQLite locale. Consentirebbe query offline istantanee senza rate limit e senza credenziali.

Una volta implementato `sync`, abilitare i comandi già scritti ma disabilitati con `//go:build ignore`:

- `stats --regione <R> [--categoria <C>]` — aggregati enti/AOO/UO per regione e percentuale canali SFE/NSO attivi (`internal/cli/stats.go`)
- `report sfe-mancante [--regione <R>]` — lista enti PA senza canale SFE attivo, per audit compliance fatturazione (`internal/cli/report.go`)

Per riattivare: rimuovere `//go:build ignore` da entrambi i file e registrare i comandi in `root.go`.
