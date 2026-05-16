# User-Provided Research Report (Anthropic Agent)

(Verbatim from user attachment — comprehensive gap analysis of YouTube official APIs vs Studio UI features. Key Tier-1 priorities, gap workarounds via InnerTube/yt-dlp/Playwright. See chat for full text.)

## Key takeaways used in this CLI:
- Official APIs cover ~70-80% of Studio surface area
- Tier-1 build priorities: heldForReview moderation, analytics digest, bulk metadata, playlist hygiene, video backup (yt-dlp), PubSubHubbub upload pings
- Quota landmines: search.list=100 units, videos.insert=1600 units, default 10K/day
- Use ETags, prefer playlistItems.list over search.list
- API key gate: OAuth2 with refresh tokens; scopes vary by feature
