# ihatepdf.cv CLI Absorb Manifest

The source is a local-first website rather than an HTTP API. Browser-only capabilities are intentionally bounded instead of being invented as remote commands.

## Absorbed
| # | Feature | Best Source | Our Implementation | Added Value |
|---|---|---|---|---|
| 1 | Merge PDFs | ihatepdf Merge PDFs | ihatepdf-cv-pp-cli merge | Deterministic ordering, dry-run, JSON metadata |
| 2 | Split/extract page ranges | ihatepdf Split PDF | ihatepdf-cv-pp-cli split | Repeatable range syntax and safe output directory |
| 3 | Organize/rotate pages | ihatepdf Organize Pages / Rotate PDF | ihatepdf-cv-pp-cli rotate | Scriptable page selection and no UI drag/drop |
| 4 | Inspect PDF | ihatepdf page-management tools | ihatepdf-cv-pp-cli inspect | Machine-readable page count, metadata, size, hash |
| 5 | Extract text | ihatepdf Extract Text | ihatepdf-cv-pp-cli extract-text | Uses local PDF text extraction and stable stdout |
| 6 | Fingerprint | ihatepdf Fingerprint Generator | ihatepdf-cv-pp-cli fingerprint | SHA-256/SHA-1/MD5 for local integrity checks |
| 8 | Privacy risk scan | ihatepdf Privacy Risk Scanner | ihatepdf-cv-pp-cli privacy-scan | Finds emails, phone-like strings, card-like strings, and metadata |
| 9 | Encrypt PDF | ihatepdf Encrypt PDF | ihatepdf-cv-pp-cli encrypt | Password supplied through stdin/env, never printed |
| 10 | Images to PDF | ihatepdf Images to PDF | ihatepdf-cv-pp-cli images-to-pdf | Batch and deterministic ordering |
| 11 | Text/Markdown to PDF | ihatepdf Create PDF / Markdown to PDF | ihatepdf-cv-pp-cli text-to-pdf | Offline plain-text rendering for agent pipelines |
| 13 | Browser install/open | ihatepdf PWA | ihatepdf-cv-pp-cli web | Opens the original UI only when explicitly requested; no resident browser transport |

## Intentionally not shipped as fake API commands
- P2P File Share, Collab Whiteboard, camera scan, interactive signature capture, Gemini Chat, browser speech, and browser-only OCR/Whisper/Office fidelity. These require browser APIs, user gestures, or third-party model downloads and are disclosed in README/SKILL as boundaries.

## Transcendence
| # | Feature | Command | Buildability | Why Only We Can Do This | Long Description |
|---|---|---|---|---|---|
| 1 | Privacy audit before sharing | privacy-scan | hand-code | Correlates metadata and content-risk signals before an artifact leaves the machine | Use this before sharing a PDF; it reports risks without changing the source file. |
| 2 | Integrity trail | fingerprint | hand-code | Produces stable multi-hash records for inputs and outputs in one JSON envelope | none |
| 3 | Agent-native inspection | inspect | hand-code | Exposes stable structure, hashes, and metadata instead of requiring a browser preview | none |
