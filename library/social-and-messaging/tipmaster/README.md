# tipmaster-pp-cli

CLI Printing Press generated CLI for [TipMaster](https://tipmaster.onrender.com) — zero-custody Farcaster RLUSD tip bot.

## Install

```bash
go install github.com/mvanhorn/printing-press-library/library/social-and-messaging/tipmaster@latest
```

## Quick Start

```bash
tipmaster resolve dwr                  # Farcaster username → XRPL wallet
tipmaster leaderboard                  # top 10 tippers this week
tipmaster leaderboard --period alltime
tipmaster user 3                       # look up by FID
tipmaster status
```
