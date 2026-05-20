# Design — Table Reservation GOAT promo (Telegram chat ad)

The video imitates a real Telegram dark-mode chat on iPhone for ~22s, then closes on a brand card promoting the CLI and the underlying Printing Press library.

## Palette

### Telegram chat (scene 1)

| Token | Hex | Use |
|---|---|---|
| `chat-bg` | `#0E0E10` | Page/chat background (under doodle pattern) |
| `doodle-tint` | `#2A1F3A` | Faint purple doodle pattern overlay (low alpha ~0.18) |
| `header-bg` | `#1C1C1E` | Top bar with bot name |
| `bubble-bot-bg` | `#262629` | Charcoal bot bubble fill |
| `bubble-user-top` | `#8A56FF` | User bubble gradient — top (violet) |
| `bubble-user-bot` | `#4A7BFF` | User bubble gradient — bottom (blue) |
| `text-primary` | `#FFFFFF` | Primary text on bubbles + header |
| `text-secondary` | `#8E8E93` | Timestamps, sub-labels |
| `accent-success` | `#34C759` | Booked checkmark |
| `accent-tock` | `#E8C547` | Tock network badge |
| `accent-ot` | `#DA3743` | OpenTable network badge |
| `divider` | `#2C2C2E` | Hairlines, input-bar borders |

### Brand close (scene 2)

| Token | Hex | Use |
|---|---|---|
| `brand-bg` | `#0A0A0C` | Solid black-violet background |
| `brand-glow` | `#8A56FF` | Radial glow behind the wordmark (low alpha) |
| `brand-fg` | `#FFFFFF` | Wordmark, tagline |
| `brand-mute` | `#A8A8B3` | Tagline supporting text |
| `code-bg` | `#1A1A1F` | Install command pill background |
| `code-fg` | `#E8E8FF` | Install command text |
| `pp-accent` | `#FFB347` | Printing Press attribution warm accent |

## Typography

| Use | Family | Weight | Size |
|---|---|---|---|
| Chat header bot name | Inter | 600 | 44px |
| Chat header sub-label ("bot") | Inter | 400 | 28px |
| Chat bubble body | Inter | 400 | 42px |
| Chat bubble strong | Inter | 600 | 42px |
| Timestamps | Inter | 400 | 24px |
| Status bar | Inter | 600 | 32px |
| Brand wordmark | Inter | 800 | 110px (tracking -2%) |
| Brand tagline | Inter | 500 | 38px |
| Install command | JetBrains Mono | 500 | 38px |
| Printing Press attribution | Inter | 600 | 32px |

System fallback: `-apple-system, "SF Pro Text", system-ui, sans-serif` so the chat reads native on iOS preview.

## Motion language

- **Bubble entrance**: y: +24px, opacity 0 → 1, scale 0.96 → 1, `power3.out`, ~360ms.
- **Typing indicator**: 3 dots, staggered opacity 0.3 → 1.0 → 0.3 cycle, 0.4s per cycle (deterministic — calculate finite repeats from window duration).
- **Item reveal in long bubble** (the 3 venue list items): one-by-one fade + y, 220ms each, 180ms stagger.
- **Booked confirmation card**: scales from 0.94 with a slight y-up, `back.out(1.4)`, 480ms.
- **Phone-to-brand transition**: chat layer dims to 0.18 opacity + slight blur; brand card crossfades in with a subtle scale 0.96 → 1.0 over 0.6s using `power2.out`. Glow pulses once behind wordmark.
- **Install command reveal**: pill border-glow sweep + characters fade in left-to-right ~700ms.

## Corners & spacing

- Bubbles: `border-radius: 36px` (large, like Telegram).
- Bubble padding: `28px 36px`.
- Bubble max-width: `780px` (out of 1080 frame width).
- Chat side margins: `36px`.
- Header height: `170px` (status bar 80px + header 90px).
- Input bar height (decorative): `120px`, anchored bottom.
- Brand card: centered, `padding: 80px`, content width `940px`.

## Avoidance rules

- No linear full-frame gradients on dark BG (H.264 banding) — use radial glow + solid fills.
- No emoji-as-text where they fall outside Inter coverage at small sizes (use SF / native emoji at ≥36px).
- No extraneous chat ornament — the realism depends on restraint. One avatar, one header, doodle pattern, bubbles. That's it.
- No animated header (it stays static like real Telegram).
- No exit animations on chat bubbles before the brand transition — the dim/blur IS the exit.
