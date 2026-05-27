# AllTrails Official-Library Preflight

Checked: 2026-05-26

## Result

AllTrails is clear for a new Printing Press package. No current upstream package, open PR, open issue, or live library tree entry was found for `alltrails` / `all-trails`.

## Evidence

- `printing-press-library search alltrails --json` returned `[]`.
  - Proof: `alltrails/proofs/pp-library-search-alltrails-2026-05-26.json`
- Open PR search for `alltrails OR all-trails` returned `[]`.
  - Proof: `alltrails/proofs/gh-pr-search-open-alltrails-2026-05-26.json`
- Closed PR search returned one unrelated maintenance PR, not an AllTrails package collision.
  - PR: `mvanhorn/printing-press-library#552`, `fix(skills): restore truncated SKILL.md descriptions across 9 CLIs`
  - Proof: `alltrails/proofs/gh-pr-search-closed-alltrails-2026-05-26.json`
- Open and closed issue searches returned `[]`.
  - Proofs:
    - `alltrails/proofs/gh-issue-search-open-alltrails-2026-05-26.json`
    - `alltrails/proofs/gh-issue-search-closed-alltrails-2026-05-26.json`
- Live travel category tree only showed `booking-com`, `flight-goat`, and `wanderlust-goat`.
  - Proof: `alltrails/proofs/github-library-travel-category-2026-05-26.json`
- GitHub code search inside `mvanhorn/printing-press-library` found no AllTrails package code.
  - Proof: `alltrails/proofs/gh-code-search-alltrails-library-2026-05-26.json`

## Working Boundary

AllTrails should start with a clear ToS/authorization note and dry-run/write barriers. The current local assumption is authorized/private-account usage per Zayd's 2026-05-26 note that he spoke with AllTrails and API CLI work is okay. This does not remove the need to document risk, avoid CAPTCHA or bot-protection bypass code, and keep mutations disabled by default.

## Next

Proceed in this order:

1. External/community API inventory.
2. Authenticated browser route capture from the dedicated CDP profile on port `9227`.
3. API-map draft with read/write/gated labels.
4. PP Go scaffold and personal TypeScript companion with write barriers from the first commit.
