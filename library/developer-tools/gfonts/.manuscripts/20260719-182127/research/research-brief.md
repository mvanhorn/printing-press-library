# gfonts Research Brief

## API
Google Fonts public metadata endpoint: https://fonts.google.com/metadata/fonts
No authentication required. Returns JSON with all ~1,900 fonts.

## Key Endpoints
- Metadata: GET https://fonts.google.com/metadata/fonts
- CSS2: GET https://fonts.googleapis.com/css2?family=<Family>&wght@<weights>
- Downloads: fonts.gstatic.com/s/<family>/<variant>.ttf

## Design Decisions
- Zero auth: uses same endpoint as fonts.google.com website
- 24h metadata cache to avoid repeated fetches
- CSS2 API requires explicit weight enumeration for variable fonts
- Variant keys use compact form (400, 400i, not "regular", "italic")
