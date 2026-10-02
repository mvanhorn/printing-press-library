# gfonts

A fast, zero-auth CLI for searching, browsing, and downloading fonts from [Google Fonts](https://fonts.google.com). No API key required — just install and go.

Created by [@neal-kyle](https://github.com/neal-kyle) (neal-kyle).
Contributors: [@cathrynlavery](https://github.com/cathrynlavery) (Cathryn Lavery).

Metadata is cached privately under your user cache directory. Downloads now report failed files and exit with an error when any requested file cannot be saved.

## Install

### Printing Press installer

```bash
npx -y @mvanhorn/printing-press-library install gfonts --cli-only
gfonts-pp-cli --version
```

### Build from this catalog checkout

```bash
cd library/developer-tools/gfonts
go build -o gfonts-pp-cli ./cmd/gfonts-pp-cli
./gfonts-pp-cli --version
```

## Usage

### Search for fonts

```bash
# Search by name, category, or designer
gfonts-pp-cli search "serif"
gfonts-pp-cli search "playfair"
gfonts-pp-cli search "Sorkin"
```

### Browse and filter

```bash
# Top fonts by popularity
gfonts-pp-cli list

# Filter by category
gfonts-pp-cli list --category serif
gfonts-pp-cli list --category sans-serif --sort trending --limit 10

# Sort options: popularity (default), alpha, date, trending
gfonts-pp-cli list --sort alpha --limit 25
```

### Font details

```bash
gfonts-pp-cli info "Playfair Display"
gfonts-pp-cli info "inter"           # case-insensitive, fuzzy match
```

### Download fonts

```bash
# Preview available files without downloading
gfonts-pp-cli download "Inter" --show

# Download all variants
gfonts-pp-cli download "Cormorant Garamond"

# Download specific variant to a custom directory
gfonts-pp-cli download "Inter" --variant regular --output ./my-fonts
gfonts-pp-cli download "Playfair Display" --variant 700italic
```

### Discover

```bash
# Trending/popular fonts
gfonts-pp-cli trending
gfonts-pp-cli trending 25

# All categories with counts
gfonts-pp-cli categories

# Random font (great for inspiration)
gfonts-pp-cli random
gfonts-pp-cli random --category serif
gfonts-pp-cli random --category display
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

`gfonts-pp-cli` uses Google Fonts' public metadata endpoint — the same one that powers [fonts.google.com](https://fonts.google.com). No API key, OAuth flow, or Google Cloud project is needed.

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

## Agent skill

The catalog ships an agent skill for this CLI. Install it with:

```bash
npx skills add mvanhorn/printing-press-library -g -y --skill pp-gfonts
```

## License

MIT
