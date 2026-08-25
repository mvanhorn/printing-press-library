# ihatepdf.cv CLI Brief

## API Identity
- Domain: `https://www.ihatepdf.cv`, a Vercel-hosted React/Vite single-page PDF toolkit.
- Users: people processing confidential PDFs who prefer local/offline workflows over upload-based services.
- Data profile: local files; no public API or server-side document service was discovered.

## Reachability Risk
- Low — `probe-reachability https://www.ihatepdf.cv --json` returned HTTP 200 over stdlib and Surf.
- Browser architecture capture: Chrome DevTools MCP observed the SPA shell, lazy tool chunks, service worker, and zero document-processing API calls.

## Architecture Findings (Chrome DevTools MCP)
- React/Vite SPA with a lazy-loaded chunk per tool; the main bundle enumerates the tool routes.
- Processing is client-side: `pdf-lib`, PDF.js, jsPDF, Tesseract.js, Transformers/Whisper, JSZip, and SheetJS are loaded from public CDNs as needed.
- The service worker (`/sw.js`) precaches the app shell and caches static assets/runtime libraries; it explicitly bypasses `/api/` and `_next/data/` requests, but no such application API was observed.
- Browser state is small and local: IndexedDB is used for saved PDF blobs/history; localStorage stores workflow, recents, drafts, POS/GST state, integrity history, and an optional Gemini key. No cookies or login are required.
- Chrome network capture of `/`, `/merge-pdf`, and lazy chunks showed only HTML, JS/CSS, CDN libraries, analytics, and no PDF upload or processing endpoint.

## Tool Surface Found
- Page management: merge, compress, split, organize, rotate, crop/resize, PDF-to-ZIP.
- Edit/security: edit/sign, fill forms, redaction UI, watermark, page numbers, headers/footers, flatten, invert colors, encrypt, unlock/remove password, automatic redaction UI, privacy scanner, fingerprint.
- Conversion: text/Markdown/HTML/Word/images/Excel/PowerPoint/CSV/audio/eBook to PDF; PDF to Word/JPG/Excel/PowerPoint/text/HTML/audio/EPUB.
- AI/collaboration: OCR, chat/summarize, compare, repair, P2P share, whiteboard, workflow builder, camera scan.

## Top Workflows
1. Combine and normalize a packet: merge PDFs, rotate/reorder pages, then inspect or fingerprint the result.
2. Prepare a private submission: split/extract pages, scan for privacy risks, encrypt, and write output locally.
3. Batch file conversion: images or text/Markdown to PDF, with deterministic output paths suitable for scripts.
4. Audit a document before sharing: inspect metadata/page count, extract text, scan for PII, and compute hashes.

## Table Stakes
- Batch input handling and deterministic output naming.
- `--json`, `--dry-run`, bounded output, clear errors, and no interactive prompts.
- Merge/split/rotate/inspect, text extraction, encryption, image-to-PDF, privacy scanning, and hashing.
- Offline operation and explicit privacy guarantees.

## Data Layer
- Primary entities: local PDF jobs, input files, output artifacts, workflow steps, inspection records.
- Sync cursor: none; this is a stateless local-file tool, not a remote API client.
- FTS/search: no remote search; the CLI provides local extracted-text search backed by the SQLite catalog.

## Product Thesis
- Name: ihatepdf.cv CLI
- Why it should exist: it carries the site's strongest promise — private, no-upload document processing — into scripts, CI, agent workflows, and repeatable local pipelines. Unlike a browser-only UI, it gives agents stable commands, structured output, dry runs, and composable files.

## Explicit Boundary
- Browser-only features (P2P share, whiteboard, camera capture, interactive signature drawing, Gemini chat, and browser speech) are not represented as fake HTTP endpoints. The CLI focuses on deterministic local document operations and reports unsupported browser-only capabilities honestly.

## Build Priorities
1. Core PDF file operations: inspect, merge, split, rotate, extract text, fingerprint.
2. Privacy/security: encrypt and PII scanning with JSON output; the browser redaction UI is outside the CLI boundary.
3. Creation: images-to-PDF and text/Markdown-to-PDF.
4. Comprehensive dogfood using generated fixtures and negative/error paths.
