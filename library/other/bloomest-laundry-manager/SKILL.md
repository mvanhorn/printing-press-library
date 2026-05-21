---
name: pp-bloomest-laundry-manager
description: "Printing Press CLI for Bloomest Laundry Manager. Mapping non-ufficiale dell'API consumata dal gestionale web `www.bloomestlaundry.app/bloomest/` (v1.22.0 in..."
author: "Andrea Tagliazucchi"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - bloomest-laundry-manager-pp-cli
---

# Bloomest Laundry Manager — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `bloomest-laundry-manager-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install bloomest-laundry-manager --cli-only
   ```
2. Verify: `bloomest-laundry-manager-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Mapping non-ufficiale dell'API consumata dal gestionale web
`www.bloomestlaundry.app/bloomest/` (v1.22.0 in produzione 2026-05-14).

**Fonte**: reverse engineering dei service.js + controller.js AngularJS
snapshot 2026-05-13 (v1.20.0) + 2026-05-14 (v1.22.0).
JS byte-identici tra v1.20 e v1.22 (versione bumped lato server, non lato bundle).
Ogni operationId include `x-source: file:linea` per tracciabilita'.

Questa spec NON e' fornita da Bloomest. E' costruita per:
- Documentare il contratto effettivo osservato lato webapp
- Servire da contract da implementare nel facade adapter
  Django -> Bloomest API
- Permettere validazione automatica delle integrazioni

## Command Reference

**accesses** — Utenti del gestionale web (login back-office). NON sono i clienti
finali (vedi tag `users`). Hanno userLevel 1-11.

- `bloomest-laundry-manager-pp-cli accesses access-delete` — Cancella utente gestionale
- `bloomest-laundry-manager-pp-cli accesses access-save` — Crea / aggiorna utente gestionale
- `bloomest-laundry-manager-pp-cli accesses list` — Lista utenti gestionale

**actions** — Coda azioni cassa (ricariche, sconti) per medias

- `bloomest-laundry-manager-pp-cli actions` — Query coda azioni cassa per medias + intervallo date

**alarms** — Allarmi e contatti di escalation

- `bloomest-laundry-manager-pp-cli alarms get` — Configurazione allarmi per tutte le lavanderie
- `bloomest-laundry-manager-pp-cli alarms save` — Salva configurazione allarmi

**automation** — Domotica / IoT (interruttori, sensori)

- `bloomest-laundry-manager-pp-cli automation get` — Configurazione domotica per LCS
- `bloomest-laundry-manager-pp-cli automation update` — Cambia stato automazione (isOn / isManual / reboot)

**bloomest-laundry-manager-profile** — Manage bloomest laundry manager profile

- `bloomest-laundry-manager-pp-cli bloomest-laundry-manager-profile` — Cambia password utente gestionale corrente

**bloomest-laundry-manager-version** — Manage bloomest laundry manager version

- `bloomest-laundry-manager-pp-cli bloomest-laundry-manager-version` — Versione applicazione

**cashbox-updates** — Manage cashbox updates

- `bloomest-laundry-manager-pp-cli cashbox-updates cancel` — Cancella aggiornamento firmware pianificato
- `bloomest-laundry-manager-pp-cli cashbox-updates get` — Lista firmware cassa + configurazioni pianificate
- `bloomest-laundry-manager-pp-cli cashbox-updates schedule` — Pianifica aggiornamento firmware cassa

**comments** — Community / commenti clienti

- `bloomest-laundry-manager-pp-cli comments delete` — Cancella commento
- `bloomest-laundry-manager-pp-cli comments list` — Lista commenti clienti

**contacts** — Rubrica contatti aziendali

- `bloomest-laundry-manager-pp-cli contacts list` — Lista contatti aziendali
- `bloomest-laundry-manager-pp-cli contacts save` — Salva intera lista contatti (CRUD bulk, filtra isRemoved client-side)

**dispenser** — Configurazione dispenser detersivi

- `bloomest-laundry-manager-pp-cli dispenser get` — Configurazione dispenser per lavanderia
- `bloomest-laundry-manager-pp-cli dispenser save` — Salva configurazione dispenser

**laundries** — Lavanderie e dispositivi (macchine)

- `bloomest-laundry-manager-pp-cli laundries delete` — Elimina device o lavanderia (discriminator type)
- `bloomest-laundry-manager-pp-cli laundries get` — Lista lavanderie con devices
- `bloomest-laundry-manager-pp-cli laundries laundry-prices-save` — Salva prezzi/configurazione device intero
- `bloomest-laundry-manager-pp-cli laundries save` — Crea o aggiorna lavanderia

**lockers** — Cassetti / locker

- `bloomest-laundry-manager-pp-cli lockers` — Lista locker/cassetti per LCS

**login** — Manage login

- `bloomest-laundry-manager-pp-cli login check` — Verifica stato login corrente
- `bloomest-laundry-manager-pp-cli login delete` — Logout (terminazione sessione)
- `bloomest-laundry-manager-pp-cli login post` — Autentica utente con credenziali SHA1

**loyalties** — Promozioni / fidelizzazione

- `bloomest-laundry-manager-pp-cli loyalties list` — Lista promozioni (attive con ?now=, storiche con ?start=)
- `bloomest-laundry-manager-pp-cli loyalties loyalty-create` — Crea / aggiorna promozione
- `bloomest-laundry-manager-pp-cli loyalties loyalty-delete` — Cancella promozione

**message** — Messaggi push a clienti (broadcast)

- `bloomest-laundry-manager-pp-cli message get` — Lista messaggi push pianificati
- `bloomest-laundry-manager-pp-cli message send` — Crea messaggio push (broadcast su laundries)

**messages** — Messaggi push a clienti (broadcast)

- `bloomest-laundry-manager-pp-cli messages` — Invia email a clienti selezionati

**notifications** — Notifiche calendar event-based

- `bloomest-laundry-manager-pp-cli notifications delete` — Cancella notifica
- `bloomest-laundry-manager-pp-cli notifications list` — Lista notifiche pianificate
- `bloomest-laundry-manager-pp-cli notifications save` — Crea / modifica notifica calendar event

**payments** — Configurazione gateway pagamento (Satispay, Argentea)

- `bloomest-laundry-manager-pp-cli payments get` — Configurazione gateway pagamento
- `bloomest-laundry-manager-pp-cli payments save` — Salva configurazione gateway pagamento

**policy** — Manage policy

- `bloomest-laundry-manager-pp-cli policy accept` — Registra accettazione privacy + terms
- `bloomest-laundry-manager-pp-cli policy get` — Versione corrente policy (privacy/terms)

**reboot** — Manage reboot

- `bloomest-laundry-manager-pp-cli reboot` — Reboot remoto cassa POS

**registration-bonus** — Bonus di benvenuto nuova registrazione

- `bloomest-laundry-manager-pp-cli registration-bonus get` — Configurazione bonus di registrazione
- `bloomest-laundry-manager-pp-cli registration-bonus save` — Salva configurazione bonus di registrazione

**research** — Manage research

- `bloomest-laundry-manager-pp-cli research` — Log query di ricerca training

**reservations** — Prenotazioni macchine

- `bloomest-laundry-manager-pp-cli reservations cancel` — Cancella prenotazione device
- `bloomest-laundry-manager-pp-cli reservations list` — Lista prenotazioni per lavanderia

**reservationsconf** — Manage reservationsconf

- `bloomest-laundry-manager-pp-cli reservationsconf reservations-config-get` — Configurazione prenotazioni
- `bloomest-laundry-manager-pp-cli reservationsconf reservations-config-save` — Salva configurazione prenotazioni

**statistics** — Statistiche aggregate (allarmi, corrispettivo Agenzia Entrate,
card-report, movimenti, cicli, summaries, sales by device).
9 sub-tipi via PUT discriminator `?type=X`.

- `bloomest-laundry-manager-pp-cli statistics` — Query statistiche aggregate (discriminator ?type=X)

**tickets** — Ticket assistenza clienti

- `bloomest-laundry-manager-pp-cli tickets` — Lista ticket assistenza clienti per lavanderia

**training** — Materiale formativo / knowledge base

- `bloomest-laundry-manager-pp-cli training list` — Lista sezioni training per lingua
- `bloomest-laundry-manager-pp-cli training post-multiplex` — Crea sezione o carica file (discriminator ?type=)
- `bloomest-laundry-manager-pp-cli training put-multiplex` — Mutazioni training (modify / delete / deleteFiles / download)

**updates** — Manage updates

- `bloomest-laundry-manager-pp-cli updates get` — Lista device aggiornati per LCS (eventualmente filtrato per device)
- `bloomest-laundry-manager-pp-cli updates put` — Aggiorna firmware o configurazione device

**users** — Utenti finali (clienti) + gruppi + card

- `bloomest-laundry-manager-pp-cli users add-group` — Crea nuovo gruppo utenti
- `bloomest-laundry-manager-pp-cli users delete` — Cancella utente
- `bloomest-laundry-manager-pp-cli users get-list` — Lista clienti (default) o gruppi (type=groups)
- `bloomest-laundry-manager-pp-cli users put-multiplex` — Mutazioni utente — discriminator query ?type=X


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
bloomest-laundry-manager-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup
Run `bloomest-laundry-manager-pp-cli auth setup` to print the URL and steps for getting a key (add `--launch` to open the URL). Then set:

```bash
export BLOOMEST_LAUNDRY_MANAGER_COOKIE_AUTH="<your-key>"
```

Or persist it in `~/.config/bloomest-laundry-manager-pp-cli/config.toml`.

Run `bloomest-laundry-manager-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  bloomest-laundry-manager-pp-cli accesses list --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success, and `--ignore-missing` only when a missing delete target should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
bloomest-laundry-manager-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
bloomest-laundry-manager-pp-cli feedback --stdin < notes.txt
bloomest-laundry-manager-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.bloomest-laundry-manager-pp-cli/feedback.jsonl`. They are never POSTed unless `BLOOMEST_LAUNDRY_MANAGER_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `BLOOMEST_LAUNDRY_MANAGER_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
bloomest-laundry-manager-pp-cli profile save briefing --json
bloomest-laundry-manager-pp-cli --profile briefing accesses list
bloomest-laundry-manager-pp-cli profile list --json
bloomest-laundry-manager-pp-cli profile show briefing
bloomest-laundry-manager-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `bloomest-laundry-manager-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add bloomest-laundry-manager-pp-mcp -- bloomest-laundry-manager-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which bloomest-laundry-manager-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   bloomest-laundry-manager-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `bloomest-laundry-manager-pp-cli <command> --help`.
