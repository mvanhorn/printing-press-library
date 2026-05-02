# Phase 4.85 Output Review Findings

Wave B policy: warnings only, no shipcheck blocking.

## Reviewer findings (2 warnings, both root-caused upstream)

### W1. scale: ingredient missing trailing token
- **Output sample:** `"4 teaspoons kosher"` (after scaling 2→4 servings on the Mom's Japanese Curry recipe)
- **Reviewer suggestion:** Audit scaling regex/tokenizer for non-numeric tail tokens.
- **Investigation:** The source JSON-LD on food52.com ships `"ingredientName": "teaspoons kosher"` — the word "salt" is missing in the Sanity CMS payload. The CLI's scaler doubles "2" to "4" and concatenates faithfully. **No CLI fix possible.**
- **Disposition:** **Pass-through warning.** Food52's own CMS has the data quality issue; correcting it would require either (a) maintaining a hand-edited override list, or (b) a heuristic that guesses missing words (high risk of getting other ingredients wrong). Neither is appropriate at the CLI layer. The site itself displays the same broken text.

### W2. scale: two ingredients merged into one line
- **Output sample:** `"4 small Yukon Gold potatoes (about 10 ounces), cut into 1 by ½-inch pieces 7 ounces cauliflower, cut into bite-size florets"`
- **Reviewer suggestion:** Inspect JSON-LD ingredient extractor for missing item separators.
- **Investigation:** The source JSON itself has these as a single `ingredientName` field with the value `"small Yukon Gold potatoes (about 10 ounces), cut into 1 by ½-inch pieces 7 ounces cauliflower, cut into bite-size florets"`. Two distinct ingredients were CMS-pasted together without a separator. The CLI extracts the field verbatim. **No CLI fix possible without heuristic re-splitting.**
- **Disposition:** **Pass-through warning.** Splitting on regex like `\s\d+\s+(ounces|grams|cups|...)\s` would correctly handle this case but introduce false positives elsewhere (parenthetical units, ranges). The honest behavior is to ship what Food52 ships.

## Verdict

Both findings are upstream data quality issues in Food52's Sanity CMS, not bugs in food52-pp-cli's extraction or scaling code. The CLI faithfully presents what the site itself shows. No code changes warranted.

**No blocker. Phase 4.85 PASS-with-pass-through warnings.**

User should be informed that scale output occasionally inherits CMS data weirdness; documented here so future runs know not to chase a fix in code.
