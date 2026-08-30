# gfonts

A fast, zero-auth CLI for searching, browsing, and downloading fonts from [Google Fonts](https://fonts.google.com). No API key required — just install and go.

## Install

### Quick install (builds from source)

```bash
curl -fsSL https://raw.githubusercontent.com/neal-kyle/gfonts/main/install.sh | bash
```

Requires [Go](https://go.dev/dl/) installed. The script clones the repo, builds the binary, and installs it to `~/.local/bin`.

> **Private repo?** `curl` can't access private repos. Use `gh` instead:
> ```bash
> gh repo clone neal-kyle/gfonts && cd gfonts && make install
> ```

### Manual install

```bash
git clone https://github.com/neal-kyle/gfonts.git
cd gfonts
make install
```

Or build directly:

```bash
go build -o gfonts .
```

## Usage

### Search for fonts

```bash
# Search by name, category, or designer
gfonts search "serif"
gfonts search "playfair"
gfonts search "Sorkin"
```

### Browse and filter

```bash
# Top fonts by popularity
gfonts list

# Filter by category
gfonts list --category serif
gfonts list --category sans-serif --sort trending --limit 10

# Sort options: popularity (default), alpha, date, trending
gfonts list --sort alpha --limit 25
```

### Font details

```bash
gfonts info "Playfair Display"
gfonts info "inter"           # case-insensitive, fuzzy match
```

### Download fonts

```bash
# Preview available files without downloading
gfonts download "Inter" --show

# Download all variants
gfonts download "Cormorant Garamond"

# Download specific variant to a custom directory
gfonts download "Inter" --variant regular --output ./my-fonts
gfonts download "Playfair Display" --variant 700italic
```

### Discover

```bash
# Trending/popular fonts
gfonts trending
gfonts trending 25

# All categories with counts
gfonts categories

# Random font (great for inspiration)
gfonts random
gfonts random --category serif
gfonts random --category display
```

## Commands

| Command | Description |
|---|---|
| `search <query>` | Search fonts by name, category, or designer |
| `list` | Browse fonts with filters (`--category`, `--sort`, `--limit`) |
| `info <font>` | Show detailed font metadata |
| `download <font>` | Download font files (`--variant`, `--output`, `--show`) |
| `trending` | Show trending/popular fonts |
| `categories` | List all font categories with counts |
| `random` | Pick a random font (`--category`) |

## How it works

gfonts uses Google Fonts' public metadata endpoint — the same one that powers the [fonts.google.com](https://fonts.google.com) website. No API key, no OAuth, no Google Cloud project needed.

- **Metadata** is fetched from `fonts.google.com/metadata/fonts` and cached locally for 24 hours
- **Font files** are downloaded from Google's CDN via the CSS2 API
- All 1,900+ Google Fonts are available, with popularity rankings, trending data, and designer info

## Categories

| Category | Count |
|---|---|
| Sans Serif | 717 |
| Display | 467 |
| Handwriting | 358 |
| Serif | 349 |
| Monospace | 51 |

## Hermes Agent Skill

This repo includes an agent skill (`skills/gfonts/SKILL.md`) that teaches AI agents how to use gfonts. The install script auto-detects installed agents and offers to install the skill for:

- **Hermes** — `~/.hermes/skills/creative/gfonts/SKILL.md`
- **Claude Code** — `~/.claude/commands/gfonts.md`
- **Codex** — `~/.codex/instructions.md`
- **OpenCode** — `~/.opencode/gfonts.md`
- **Agents** (generic) — `~/.agents/skills/gfonts/SKILL.md`

You can also install manually by copying `skills/gfonts/SKILL.md` to your agent's skill directory.

## License

MIT
