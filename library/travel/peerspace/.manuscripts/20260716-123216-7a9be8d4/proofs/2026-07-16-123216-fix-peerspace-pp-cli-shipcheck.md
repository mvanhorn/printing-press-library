# Peerspace shipcheck

## Results
- shipcheck umbrella: **PASS (7/7 legs)**
- scorecard: **85/100 Grade A**
- novel_features_check: 15/15 found
- live sample probe: 14/15 then shortlist similar fixed to default seed
- Live API: venues filters + venues list return real JSON via Surf (362 Paris meetup hits)
- Offline novel: scout budget/capacity, venues recommend, markets neighborhoods, shortlist similar work on local store

## Verdict
**ship**

## Known notes
- Cookie auth not configured until `auth login --chrome` (user has browser session available)
- Sync stores search envelope as one resource row; ExpandResourceData unpacks hits for novel commands
- No booking/checkout surface in capture
