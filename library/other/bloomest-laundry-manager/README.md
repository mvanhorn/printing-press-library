# Bloomest Laundry Manager CLI

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

## Install

The recommended path installs both the `bloomest-laundry-manager-pp-cli` binary and the `pp-bloomest-laundry-manager` agent skill in one shot:

```bash
npx -y @mvanhorn/printing-press install bloomest-laundry-manager
```

For CLI only (no skill):

```bash
npx -y @mvanhorn/printing-press install bloomest-laundry-manager --cli-only
```


### Without Node

The generated install path is category-agnostic until this CLI is published. If `npx` is not available before publish, install Node or use the category-specific Go fallback from the public-library entry after publish.

### Pre-built binary

Download a pre-built binary for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/bloomest-laundry-manager-current). On macOS, clear the Gatekeeper quarantine: `xattr -d com.apple.quarantine <binary>`. On Unix, mark it executable: `chmod +x <binary>`.

<!-- pp-hermes-install-anchor -->
## Install for Hermes

From the Hermes CLI:

```bash
hermes skills install mvanhorn/printing-press-library/cli-skills/pp-bloomest-laundry-manager --force
```

Inside a Hermes chat session:

```bash
/skills install mvanhorn/printing-press-library/cli-skills/pp-bloomest-laundry-manager --force
```

## Install for OpenClaw

Tell your OpenClaw agent (copy this):

```
Install the pp-bloomest-laundry-manager skill from https://github.com/mvanhorn/printing-press-library/tree/main/cli-skills/pp-bloomest-laundry-manager. The skill defines how its required CLI can be installed.
```

## Quick Start

### 1. Install

See [Install](#install) above.

### 2. Set Up Credentials

Get your API key from your API provider's developer portal. The key typically looks like a long alphanumeric string.

```bash
export BLOOMEST_LAUNDRY_MANAGER_COOKIE_AUTH="<paste-your-key>"
```

You can also persist this in your config file at `~/.config/bloomest-laundry-manager-pp-cli/config.toml`.

### 3. Verify Setup

```bash
bloomest-laundry-manager-pp-cli doctor
```

This checks your configuration and credentials.

### 4. Try Your First Command

```bash
bloomest-laundry-manager-pp-cli accesses list
```

## Usage

Run `bloomest-laundry-manager-pp-cli --help` for the full command reference and flag list.

## Commands

### accesses

Utenti del gestionale web (login back-office). NON sono i clienti
finali (vedi tag `users`). Hanno userLevel 1-11.

- **`bloomest-laundry-manager-pp-cli accesses access-delete`** - Cancella utente gestionale
- **`bloomest-laundry-manager-pp-cli accesses access-save`** - Crea / aggiorna utente gestionale
- **`bloomest-laundry-manager-pp-cli accesses list`** - Lista utenti gestionale

### actions

Coda azioni cassa (ricariche, sconti) per medias

- **`bloomest-laundry-manager-pp-cli actions search`** - Query coda azioni cassa per medias + intervallo date

### alarms

Allarmi e contatti di escalation

- **`bloomest-laundry-manager-pp-cli alarms get`** - Configurazione allarmi per tutte le lavanderie
- **`bloomest-laundry-manager-pp-cli alarms save`** - Salva configurazione allarmi

### automation

Domotica / IoT (interruttori, sensori)

- **`bloomest-laundry-manager-pp-cli automation get`** - Configurazione domotica per LCS
- **`bloomest-laundry-manager-pp-cli automation update`** - Cambia stato automazione (isOn / isManual / reboot)

### bloomest-laundry-manager-profile

Manage bloomest laundry manager profile

- **`bloomest-laundry-manager-pp-cli bloomest-laundry-manager-profile change-password`** - Cambia password utente gestionale corrente

### bloomest-laundry-manager-version

Manage bloomest laundry manager version

- **`bloomest-laundry-manager-pp-cli bloomest-laundry-manager-version get`** - Versione applicazione

### cashbox-updates

Manage cashbox updates

- **`bloomest-laundry-manager-pp-cli cashbox-updates cancel`** - Cancella aggiornamento firmware pianificato
- **`bloomest-laundry-manager-pp-cli cashbox-updates get`** - Lista firmware cassa + configurazioni pianificate
- **`bloomest-laundry-manager-pp-cli cashbox-updates schedule`** - Pianifica aggiornamento firmware cassa

### comments

Community / commenti clienti

- **`bloomest-laundry-manager-pp-cli comments delete`** - Cancella commento
- **`bloomest-laundry-manager-pp-cli comments list`** - Lista commenti clienti

### contacts

Rubrica contatti aziendali

- **`bloomest-laundry-manager-pp-cli contacts list`** - Lista contatti aziendali
- **`bloomest-laundry-manager-pp-cli contacts save`** - Salva intera lista contatti (CRUD bulk, filtra isRemoved client-side)

### dispenser

Configurazione dispenser detersivi

- **`bloomest-laundry-manager-pp-cli dispenser get`** - Configurazione dispenser per lavanderia
- **`bloomest-laundry-manager-pp-cli dispenser save`** - Salva configurazione dispenser

### laundries

Lavanderie e dispositivi (macchine)

- **`bloomest-laundry-manager-pp-cli laundries delete`** - Elimina device o lavanderia (discriminator type)
- **`bloomest-laundry-manager-pp-cli laundries get`** - Lista lavanderie con devices
- **`bloomest-laundry-manager-pp-cli laundries laundry-prices-save`** - Salva prezzi/configurazione device intero
- **`bloomest-laundry-manager-pp-cli laundries save`** - Crea o aggiorna lavanderia

### lockers

Cassetti / locker

- **`bloomest-laundry-manager-pp-cli lockers get`** - Lista locker/cassetti per LCS

### login

Manage login

- **`bloomest-laundry-manager-pp-cli login check`** - Verifica stato login corrente
- **`bloomest-laundry-manager-pp-cli login delete`** - Logout (terminazione sessione)
- **`bloomest-laundry-manager-pp-cli login post`** - Autentica utente con credenziali SHA1

### loyalties

Promozioni / fidelizzazione

- **`bloomest-laundry-manager-pp-cli loyalties list`** - Lista promozioni (attive con ?now=, storiche con ?start=)
- **`bloomest-laundry-manager-pp-cli loyalties loyalty-create`** - Crea / aggiorna promozione
- **`bloomest-laundry-manager-pp-cli loyalties loyalty-delete`** - Cancella promozione

### message

Messaggi push a clienti (broadcast)

- **`bloomest-laundry-manager-pp-cli message get`** - Lista messaggi push pianificati
- **`bloomest-laundry-manager-pp-cli message send`** - Crea messaggio push (broadcast su laundries)

### messages

Messaggi push a clienti (broadcast)

- **`bloomest-laundry-manager-pp-cli messages users-send-email`** - Invia email a clienti selezionati

### notifications

Notifiche calendar event-based

- **`bloomest-laundry-manager-pp-cli notifications delete`** - Cancella notifica
- **`bloomest-laundry-manager-pp-cli notifications list`** - Lista notifiche pianificate
- **`bloomest-laundry-manager-pp-cli notifications save`** - Crea / modifica notifica calendar event

### payments

Configurazione gateway pagamento (Satispay, Argentea)

- **`bloomest-laundry-manager-pp-cli payments get`** - Configurazione gateway pagamento
- **`bloomest-laundry-manager-pp-cli payments save`** - Salva configurazione gateway pagamento

### policy

Manage policy

- **`bloomest-laundry-manager-pp-cli policy accept`** - Registra accettazione privacy + terms
- **`bloomest-laundry-manager-pp-cli policy get`** - Versione corrente policy (privacy/terms)

### reboot

Manage reboot

- **`bloomest-laundry-manager-pp-cli reboot cashbox`** - Reboot remoto cassa POS

### registration-bonus

Bonus di benvenuto nuova registrazione

- **`bloomest-laundry-manager-pp-cli registration-bonus get`** - Configurazione bonus di registrazione
- **`bloomest-laundry-manager-pp-cli registration-bonus save`** - Salva configurazione bonus di registrazione

### research

Manage research

- **`bloomest-laundry-manager-pp-cli research log`** - Log query di ricerca training

### reservations

Prenotazioni macchine

- **`bloomest-laundry-manager-pp-cli reservations cancel`** - Cancella prenotazione device
- **`bloomest-laundry-manager-pp-cli reservations list`** - Lista prenotazioni per lavanderia

### reservationsconf

Manage reservationsconf

- **`bloomest-laundry-manager-pp-cli reservationsconf reservations-config-get`** - Configurazione prenotazioni
- **`bloomest-laundry-manager-pp-cli reservationsconf reservations-config-save`** - Salva configurazione prenotazioni

### statistics

Statistiche aggregate (allarmi, corrispettivo Agenzia Entrate,
card-report, movimenti, cicli, summaries, sales by device).
9 sub-tipi via PUT discriminator `?type=X`.

- **`bloomest-laundry-manager-pp-cli statistics query`** - Query statistiche aggregate (discriminator ?type=X)

### tickets

Ticket assistenza clienti

- **`bloomest-laundry-manager-pp-cli tickets list`** - Lista ticket assistenza clienti per lavanderia

### training

Materiale formativo / knowledge base

- **`bloomest-laundry-manager-pp-cli training list`** - Lista sezioni training per lingua
- **`bloomest-laundry-manager-pp-cli training post-multiplex`** - Crea sezione o carica file (discriminator ?type=)
- **`bloomest-laundry-manager-pp-cli training put-multiplex`** - Mutazioni training (modify / delete / deleteFiles / download)

### updates

Manage updates

- **`bloomest-laundry-manager-pp-cli updates get`** - Lista device aggiornati per LCS (eventualmente filtrato per device)
- **`bloomest-laundry-manager-pp-cli updates put`** - Aggiorna firmware o configurazione device

### users

Utenti finali (clienti) + gruppi + card

- **`bloomest-laundry-manager-pp-cli users add-group`** - Crea nuovo gruppo utenti
- **`bloomest-laundry-manager-pp-cli users delete`** - Cancella utente
- **`bloomest-laundry-manager-pp-cli users get-list`** - Lista clienti (default) o gruppi (type=groups)
- **`bloomest-laundry-manager-pp-cli users put-multiplex`** - Mutazioni utente — discriminator query ?type=X


## Output Formats

```bash
# Human-readable table (default in terminal, JSON when piped)
bloomest-laundry-manager-pp-cli accesses list

# JSON for scripting and agents
bloomest-laundry-manager-pp-cli accesses list --json

# Filter to specific fields
bloomest-laundry-manager-pp-cli accesses list --json --select id,name,status

# Dry run — show the request without sending
bloomest-laundry-manager-pp-cli accesses list --dry-run

# Agent mode — JSON + compact + no prompts in one flag
bloomest-laundry-manager-pp-cli accesses list --agent
```

## Agent Usage

This CLI is designed for AI agent consumption:

- **Non-interactive** - never prompts, every input is a flag
- **Pipeable** - `--json` output to stdout, errors to stderr
- **Filterable** - `--select id,name` returns only fields you need
- **Previewable** - `--dry-run` shows the request without sending
- **Explicit retries** - add `--idempotent` to create retries and `--ignore-missing` to delete retries when a no-op success is acceptable
- **Confirmable** - `--yes` for explicit confirmation of destructive actions
- **Piped input** - write commands can accept structured input when their help lists `--stdin`
- **Offline-friendly** - sync/search commands can use the local SQLite store when available
- **Agent-safe by default** - no colors or formatting unless `--human-friendly` is set

Exit codes: `0` success, `2` usage error, `3` not found, `4` auth error, `5` API error, `7` rate limited, `10` config error.

## Use with Claude Code

Install the focused skill — it auto-installs the CLI on first invocation:

```bash
npx skills add mvanhorn/printing-press-library/cli-skills/pp-bloomest-laundry-manager -g
```

Then invoke `/pp-bloomest-laundry-manager <query>` in Claude Code. The skill is the most efficient path — Claude Code drives the CLI directly without an MCP server in the middle.

<details>
<summary>Use as an MCP server in Claude Code (advanced)</summary>

If you'd rather register this CLI as an MCP server in Claude Code, install the MCP binary first:


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Then register it:

```bash
claude mcp add bloomest-laundry-manager bloomest-laundry-manager-pp-mcp -e BLOOMEST_LAUNDRY_MANAGER_COOKIE_AUTH=<your-key>
```

</details>

## Use with Claude Desktop

This CLI ships an [MCPB](https://github.com/modelcontextprotocol/mcpb) bundle — Claude Desktop's standard format for one-click MCP extension installs (no JSON config required).

To install:

1. Download the `.mcpb` for your platform from the [latest release](https://github.com/mvanhorn/printing-press-library/releases/tag/bloomest-laundry-manager-current).
2. Double-click the `.mcpb` file. Claude Desktop opens and walks you through the install.
3. Fill in `BLOOMEST_LAUNDRY_MANAGER_COOKIE_AUTH` when Claude Desktop prompts you.

Requires Claude Desktop 1.0.0 or later. Pre-built bundles ship for macOS Apple Silicon (`darwin-arm64`) and Windows (`amd64`, `arm64`); for other platforms, use the manual config below.

<details>
<summary>Manual JSON config (advanced)</summary>

If you can't use the MCPB bundle (older Claude Desktop, unsupported platform), install the MCP binary and configure it manually.


Install the MCP binary from this CLI's published public-library entry or pre-built release.

Add to your Claude Desktop config (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "bloomest-laundry-manager": {
      "command": "bloomest-laundry-manager-pp-mcp",
      "env": {
        "BLOOMEST_LAUNDRY_MANAGER_COOKIE_AUTH": "<your-key>"
      }
    }
  }
}
```

</details>

## Health Check

```bash
bloomest-laundry-manager-pp-cli doctor
```

Verifies configuration, credentials, and connectivity to the API.

## Configuration

Config file: `~/.config/bloomest-laundry-manager-pp-cli/config.toml`

Static request headers can be configured under `headers`; per-command header overrides take precedence.

Environment variables:

| Name | Kind | Required | Description |
| --- | --- | --- | --- |
| `BLOOMEST_LAUNDRY_MANAGER_COOKIE_AUTH` | harvested | Yes | Populated automatically by auth login. |

## Troubleshooting
**Authentication errors (exit code 4)**
- Run `bloomest-laundry-manager-pp-cli doctor` to check credentials
- Verify the environment variable is set: `echo $BLOOMEST_LAUNDRY_MANAGER_COOKIE_AUTH`
**Not found errors (exit code 3)**
- Check the resource ID is correct
- Run the `list` command to see available items

---

Generated by [CLI Printing Press](https://github.com/mvanhorn/cli-printing-press)
