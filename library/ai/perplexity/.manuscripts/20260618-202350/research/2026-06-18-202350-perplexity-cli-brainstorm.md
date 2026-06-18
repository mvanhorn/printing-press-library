# Perplexity CLI Brainstorm

## Novel Features Worth Keeping

| Feature | Command | Why It Matters |
|---------|---------|----------------|
| Trace export | `history export <thread>` | Turns a Perplexity conversation into durable local evidence. |
| Transcript read | `history read <uuid-or-slug>` | Makes an old thread recoverable without hunting through the UI. |
| Recent threads | `history recent` | Gives agents a quick index over the user's active research trail. |
| Browser-session auth | `auth login --chrome` | Avoids a paid API dependency when the browser session already has the needed context. |

## Use Cases
- Recover an answer thread that was useful but got lost in the UI.
- Save research output into the monorepo for later reuse.
- Let agents inspect recent research without opening the Perplexity app.

## Design Bias
- Optimize for trace preservation first.
- Keep the CLI usable without the API when browser cookies are available.
- Prefer explicit export and read commands over one giant implicit sync.
