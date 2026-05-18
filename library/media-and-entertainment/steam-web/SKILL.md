---
name: pp-steam-web
description: "Every Steam Web API endpoint, plus a local SQLite store that turns friend playtimes, achievement progress, and library backlogs into single SQL queries no other tool can answer. Trigger phrases: `what should I play next on steam`, `find friends who own this game`, `easiest steam achievement to unlock`, `audit my steam backlog`, `use steam-web`."
author: "Trevin Chow"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - steam-web-pp-cli
    install:
      - kind: go
        bins: [steam-web-pp-cli]
        module: github.com/mvanhorn/printing-press-library/library/other/steam-web/cmd/steam-web-pp-cli
---

# Steam Web — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `steam-web-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install steam-web --cli-only
   ```
2. Verify: `steam-web-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails (no Node, offline, etc.), fall back to a direct Go install (requires Go 1.26.3 or newer):

```bash
go install github.com/mvanhorn/printing-press-library/library/other/steam-web/cmd/steam-web-pp-cli@latest
```

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Mirrors all 169 documented Steam Web API endpoints (and the undocumented store endpoints every wrapper picks one of) with rate-limit-aware throttling for the post-2025 25 req/s budget. Adds a local SQLite layer with FTS5 over apps, news, and achievements so cross-library queries — `next-achievement`, `friends compare`, `library audit`, `achievement-leaderboard` — run as one command instead of an N+1 fanout app rewrite. Ships an MCP server with both stdio and HTTP streamable transport plus a code-orchestration pair (`steam_web_search` + `steam_web_execute`) so the full surface is reachable without flooding your agent's tool catalog.

## When to Use This CLI

Pick this CLI when an agent is asked to query Steam profile, library, achievement, or news data and the question crosses entities — anything joining friends with owned games, achievements with global rarity, or news with a date filter. The local SQLite store turns those questions into one SQL query instead of an N+1 fanout. Every other Steam tool is single-shot read-once; this one compounds. For pure single-endpoint reads (one player's profile, one app's news), the per-endpoint MCP tools work too — but the novel commands are where this CLI earns its weight.

## Unique Capabilities

These capabilities aren't available in any other tool for this API.

### Local state that compounds
- **`library audit`** — Surface never-launched games, paid titles you bounced off in under 2 hours, and where your hours actually went by genre.

  _Use this when an agent is asked to plan what to play next, justify a refund, or summarize the user's gaming spend._

  ```bash
  steam-web-pp-cli library audit 76561197960287930 --bounce --json
  ```
- **`review-velocity`** — Reviews per day and voted-up share over a rolling window for one app — date-bucket aggregation over the cursor-paginated appreviews stream.

  _Use this when an agent is asked to track sentiment shifts after a patch, sale, or controversy._

  ```bash
  steam-web-pp-cli review-velocity 1245620 --window 30d --json
  ```
- **`news search`** — FTS5 search across the title and contents of every news post you've synced, optionally scoped by appid or date range.

  _Use this when an agent is asked to dig up patch notes, devlog mentions, or news around a specific topic across the user's library._

  ```bash
  steam-web-pp-cli news search 'patch notes' --since 2026-04-01 --json
  ```
- **`play-trend`** — Show concurrent-player count over time for one app as a sparkline plus min/max/last over a rolling window — value scales with how often you sample.

  _Use this when an agent needs to see whether a game's playerbase is growing, falling, or has spiked around an event — sample the count every few hours via cron and the window query becomes meaningful within a week._

  ```bash
  steam-web-pp-cli play-trend 1245620 --window 7d --json
  ```

### Friend-graph intelligence
- **`friends compare`** — Rank everyone in your friend list by hours spent in a specific game, with throttled fan-out so you don't trip Steam's 25 req/s budget.

  _Use this when an agent is asked 'who in my friends has the most hours in <game>' or 'who owns this and never played'._

  ```bash
  steam-web-pp-cli friends compare 1245620 --my-steamid 76561197960287930 --agent --select results.persona_name,results.playtime_hours
  ```
- **`library compare`** — Set operations across two libraries — what's mine-only, what's theirs-only, what's shared with playtime delta.

  _Use this when an agent is asked 'what games do my friend and I both own' or to plan a co-op session._

  ```bash
  steam-web-pp-cli library compare 76561197960287930 --my-steamid 76561197960287930 --shared --json
  ```
- **`currently-playing`** — Show which friends are in-game right now and what they're playing — one batched API call across the friend list, no fanout.

  _Use this when an agent is asked 'who is online and playing what' or to power a status panel._

  ```bash
  steam-web-pp-cli currently-playing --my-steamid 76561197960287930 --json
  ```
- **`achievement-leaderboard`** — Rank your friends by achievement completion percentage for one app, throttled fan-out via the same limiter that powers friends compare.

  _Use this when an agent is asked 'who in my friends is closest to 100% in <game>' or to seed a competitive completion challenge._

  ```bash
  steam-web-pp-cli achievement-leaderboard 1245620 --my-steamid 76561197960287930 --json
  ```

### Achievement intelligence
- **`next-achievement`** — Across your entire library, surface the achievement with the highest global unlock percentage that you still don't have.

  _Use this when an agent is asked 'what should I go unlock next' or to recommend low-effort achievement progress for completionists._

  ```bash
  steam-web-pp-cli next-achievement --steamid 76561197960287930 --limit 10 --json
  ```
- **`achievement-hunt`** — Render the full achievement schema for one app side-by-side with your unlock state and the global rarity of each achievement in one table.

  _Use this when an agent needs the full unlock landscape for one game to plan a completion run._

  ```bash
  steam-web-pp-cli achievement-hunt 1245620 --steamid 76561197960287930 --locked --json
  ```
- **`rare-achievements`** — Surface your rarest achievement unlocks across all owned games — the inverse of the next-achievement query, sorted by ascending global percentage.

  _Use this when an agent is asked to summarize what the user is proud of, or to power a profile-flex panel._

  ```bash
  steam-web-pp-cli rare-achievements --steamid 76561197960287930 --limit 10 --json
  ```

## Command Reference

**iauthentication-service** — Manage iauthentication service

- `steam-web-pp-cli iauthentication-service begin-auth-session-via-credentials` — BeginAuthSessionViaCredentials operation of IAuthenticationService
- `steam-web-pp-cli iauthentication-service begin-auth-session-via-qr` — BeginAuthSessionViaQR operation of IAuthenticationService
- `steam-web-pp-cli iauthentication-service get-auth-session-info` — GetAuthSessionInfo operation of IAuthenticationService
- `steam-web-pp-cli iauthentication-service get-auth-session-risk-info` — GetAuthSessionRiskInfo operation of IAuthenticationService
- `steam-web-pp-cli iauthentication-service get-password-rsapublic-key` — GetPasswordRSAPublicKey operation of IAuthenticationService
- `steam-web-pp-cli iauthentication-service notify-risk-quiz-results` — NotifyRiskQuizResults operation of IAuthenticationService
- `steam-web-pp-cli iauthentication-service poll-auth-session-status` — PollAuthSessionStatus operation of IAuthenticationService
- `steam-web-pp-cli iauthentication-service update-auth-session-with-mobile-confirmation` — UpdateAuthSessionWithMobileConfirmation operation of IAuthenticationService
- `steam-web-pp-cli iauthentication-service update-auth-session-with-steam-guard-code` — UpdateAuthSessionWithSteamGuardCode operation of IAuthenticationService

**ibroadcast-service** — Manage ibroadcast service

- `steam-web-pp-cli ibroadcast-service` — PostGameDataFrameRTMP operation of IBroadcastService

**icheat-reporting-service** — Manage icheat reporting service

- `steam-web-pp-cli icheat-reporting-service` — ReportCheatData operation of ICheatReportingService

**iclient-stats-1046930** — Manage iclient stats 1046930

- `steam-web-pp-cli iclient-stats-1046930` — ReportEvent operation of IClientStats_1046930

**icontent-server-config-service** — Manage icontent server config service

- `steam-web-pp-cli icontent-server-config-service get-steam-cache-node-params` — GetSteamCacheNodeParams operation of IContentServerConfigService
- `steam-web-pp-cli icontent-server-config-service set-steam-cache-client-filters` — SetSteamCacheClientFilters operation of IContentServerConfigService
- `steam-web-pp-cli icontent-server-config-service set-steam-cache-performance-stats` — SetSteamCachePerformanceStats operation of IContentServerConfigService

**icontent-server-directory-service** — Manage icontent server directory service

- `steam-web-pp-cli icontent-server-directory-service get-cdnfor-video` — GetCDNForVideo operation of IContentServerDirectoryService
- `steam-web-pp-cli icontent-server-directory-service get-client-update-hosts` — GetClientUpdateHosts operation of IContentServerDirectoryService
- `steam-web-pp-cli icontent-server-directory-service get-depot-patch-info` — GetDepotPatchInfo operation of IContentServerDirectoryService
- `steam-web-pp-cli icontent-server-directory-service get-servers-for-steam-pipe` — GetServersForSteamPipe operation of IContentServerDirectoryService
- `steam-web-pp-cli icontent-server-directory-service pick-single-content-server` — PickSingleContentServer operation of IContentServerDirectoryService

**icsgoplayers-730** — Manage icsgoplayers 730

- `steam-web-pp-cli icsgoplayers-730` — GetNextMatchSharingCode operation of ICSGOPlayers_730

**icsgoservers-730** — Manage icsgoservers 730

- `steam-web-pp-cli icsgoservers-730 get-game-maps-playtime` — GetGameMapsPlaytime operation of ICSGOServers_730
- `steam-web-pp-cli icsgoservers-730 get-game-servers-status` — GetGameServersStatus operation of ICSGOServers_730

**icsgotournaments-730** — Manage icsgotournaments 730

- `steam-web-pp-cli icsgotournaments-730 get-tournament-fantasy-lineup` — GetTournamentFantasyLineup operation of ICSGOTournaments_730
- `steam-web-pp-cli icsgotournaments-730 get-tournament-items` — GetTournamentItems operation of ICSGOTournaments_730
- `steam-web-pp-cli icsgotournaments-730 get-tournament-layout` — GetTournamentLayout operation of ICSGOTournaments_730
- `steam-web-pp-cli icsgotournaments-730 get-tournament-predictions` — GetTournamentPredictions operation of ICSGOTournaments_730
- `steam-web-pp-cli icsgotournaments-730 upload-tournament-fantasy-lineup` — UploadTournamentFantasyLineup operation of ICSGOTournaments_730
- `steam-web-pp-cli icsgotournaments-730 upload-tournament-predictions` — UploadTournamentPredictions operation of ICSGOTournaments_730

**idota2-match-570** — Manage idota2 match 570

- `steam-web-pp-cli idota2-match-570 get-live-league-games` — GetLiveLeagueGames operation of IDOTA2Match_570
- `steam-web-pp-cli idota2-match-570 get-match-details` — GetMatchDetails operation of IDOTA2Match_570
- `steam-web-pp-cli idota2-match-570 get-match-history` — GetMatchHistory operation of IDOTA2Match_570
- `steam-web-pp-cli idota2-match-570 get-match-history-by-sequence-num` — GetMatchHistoryBySequenceNum operation of IDOTA2Match_570
- `steam-web-pp-cli idota2-match-570 get-team-info-by-team-id` — GetTeamInfoByTeamID operation of IDOTA2Match_570
- `steam-web-pp-cli idota2-match-570 get-top-live-event-game` — GetTopLiveEventGame operation of IDOTA2Match_570
- `steam-web-pp-cli idota2-match-570 get-top-live-game` — GetTopLiveGame operation of IDOTA2Match_570
- `steam-web-pp-cli idota2-match-570 get-top-weekend-tourney-games` — GetTopWeekendTourneyGames operation of IDOTA2Match_570
- `steam-web-pp-cli idota2-match-570 get-tournament-player-stats` — GetTournamentPlayerStats operation of IDOTA2Match_570
- `steam-web-pp-cli idota2-match-570 get-tournament-player-stats-idota2match570` — GetTournamentPlayerStats operation of IDOTA2Match_570

**idota2-match-stats-570** — Manage idota2 match stats 570

- `steam-web-pp-cli idota2-match-stats-570` — GetRealtimeStats operation of IDOTA2MatchStats_570

**idota2-stream-system-570** — Manage idota2 stream system 570

- `steam-web-pp-cli idota2-stream-system-570` — GetBroadcasterInfo operation of IDOTA2StreamSystem_570

**idota2-ticket-570** — Manage idota2 ticket 570

- `steam-web-pp-cli idota2-ticket-570 get-steam-idfor-badge-id` — GetSteamIDForBadgeID operation of IDOTA2Ticket_570
- `steam-web-pp-cli idota2-ticket-570 set-steam-account-purchased` — SetSteamAccountPurchased operation of IDOTA2Ticket_570
- `steam-web-pp-cli idota2-ticket-570 steam-account-valid-for-badge-type` — SteamAccountValidForBadgeType operation of IDOTA2Ticket_570

**iecon-dota2-570** — Manage iecon dota2 570

- `steam-web-pp-cli iecon-dota2-570 get-event-stats-for-account` — GetEventStatsForAccount operation of IEconDOTA2_570
- `steam-web-pp-cli iecon-dota2-570 get-heroes` — GetHeroes operation of IEconDOTA2_570
- `steam-web-pp-cli iecon-dota2-570 get-item-creators` — GetItemCreators operation of IEconDOTA2_570
- `steam-web-pp-cli iecon-dota2-570 get-item-workshop-published-file-ids` — GetItemWorkshopPublishedFileIDs operation of IEconDOTA2_570
- `steam-web-pp-cli iecon-dota2-570 get-rarities` — GetRarities operation of IEconDOTA2_570
- `steam-web-pp-cli iecon-dota2-570 get-tournament-prize-pool` — GetTournamentPrizePool operation of IEconDOTA2_570

**iecon-items-1046930** — Manage iecon items 1046930

- `steam-web-pp-cli iecon-items-1046930` — GetPlayerItems operation of IEconItems_1046930

**iecon-items-1269260** — Manage iecon items 1269260

- `steam-web-pp-cli iecon-items-1269260` — GetEquippedPlayerItems operation of IEconItems_1269260

**iecon-items-440** — Manage iecon items 440

- `steam-web-pp-cli iecon-items-440 get-player-items` — GetPlayerItems operation of IEconItems_440
- `steam-web-pp-cli iecon-items-440 get-schema` — GetSchema operation of IEconItems_440
- `steam-web-pp-cli iecon-items-440 get-schema-items` — GetSchemaItems operation of IEconItems_440
- `steam-web-pp-cli iecon-items-440 get-schema-overview` — GetSchemaOverview operation of IEconItems_440
- `steam-web-pp-cli iecon-items-440 get-schema-url` — GetSchemaURL operation of IEconItems_440
- `steam-web-pp-cli iecon-items-440 get-store-meta-data` — GetStoreMetaData operation of IEconItems_440
- `steam-web-pp-cli iecon-items-440 get-store-status` — GetStoreStatus operation of IEconItems_440

**iecon-items-570** — Manage iecon items 570

- `steam-web-pp-cli iecon-items-570 get-player-items` — GetPlayerItems operation of IEconItems_570
- `steam-web-pp-cli iecon-items-570 get-store-meta-data` — GetStoreMetaData operation of IEconItems_570

**iecon-items-583950** — Manage iecon items 583950

- `steam-web-pp-cli iecon-items-583950` — GetEquippedPlayerItems operation of IEconItems_583950

**iecon-items-620** — Manage iecon items 620

- `steam-web-pp-cli iecon-items-620 get-player-items` — GetPlayerItems operation of IEconItems_620
- `steam-web-pp-cli iecon-items-620 get-schema` — GetSchema operation of IEconItems_620

**iecon-items-730** — Manage iecon items 730

- `steam-web-pp-cli iecon-items-730 get-player-items` — GetPlayerItems operation of IEconItems_730
- `steam-web-pp-cli iecon-items-730 get-schema` — GetSchema operation of IEconItems_730
- `steam-web-pp-cli iecon-items-730 get-schema-url` — GetSchemaURL operation of IEconItems_730
- `steam-web-pp-cli iecon-items-730 get-store-meta-data` — GetStoreMetaData operation of IEconItems_730

**iecon-service** — Manage iecon service

- `steam-web-pp-cli iecon-service get-trade-history` — GetTradeHistory operation of IEconService
- `steam-web-pp-cli iecon-service get-trade-hold-durations` — GetTradeHoldDurations operation of IEconService
- `steam-web-pp-cli iecon-service get-trade-offer` — GetTradeOffer operation of IEconService
- `steam-web-pp-cli iecon-service get-trade-offers` — GetTradeOffers operation of IEconService
- `steam-web-pp-cli iecon-service get-trade-offers-summary` — GetTradeOffersSummary operation of IEconService
- `steam-web-pp-cli iecon-service get-trade-status` — GetTradeStatus operation of IEconService

**igame-notifications-service** — Manage igame notifications service

- `steam-web-pp-cli igame-notifications-service user-create-session` — UserCreateSession operation of IGameNotificationsService
- `steam-web-pp-cli igame-notifications-service user-delete-session` — UserDeleteSession operation of IGameNotificationsService
- `steam-web-pp-cli igame-notifications-service user-update-session` — UserUpdateSession operation of IGameNotificationsService

**igame-servers-service** — Manage igame servers service

- `steam-web-pp-cli igame-servers-service create-account` — CreateAccount operation of IGameServersService
- `steam-web-pp-cli igame-servers-service delete-account` — DeleteAccount operation of IGameServersService
- `steam-web-pp-cli igame-servers-service get-account-list` — GetAccountList operation of IGameServersService
- `steam-web-pp-cli igame-servers-service get-account-public-info` — GetAccountPublicInfo operation of IGameServersService
- `steam-web-pp-cli igame-servers-service get-server-ips-by-steam-id` — GetServerIPsBySteamID operation of IGameServersService
- `steam-web-pp-cli igame-servers-service get-server-steam-ids-by-ip` — GetServerSteamIDsByIP operation of IGameServersService
- `steam-web-pp-cli igame-servers-service query-by-fake-ip` — QueryByFakeIP operation of IGameServersService
- `steam-web-pp-cli igame-servers-service query-login-token` — QueryLoginToken operation of IGameServersService
- `steam-web-pp-cli igame-servers-service reset-login-token` — ResetLoginToken operation of IGameServersService
- `steam-web-pp-cli igame-servers-service set-memo` — SetMemo operation of IGameServersService

**igcversion-1046930** — Manage igcversion 1046930

- `steam-web-pp-cli igcversion-1046930 get-client-version` — GetClientVersion operation of IGCVersion_1046930
- `steam-web-pp-cli igcversion-1046930 get-server-version` — GetServerVersion operation of IGCVersion_1046930

**igcversion-1269260** — Manage igcversion 1269260

- `steam-web-pp-cli igcversion-1269260 get-client-version` — GetClientVersion operation of IGCVersion_1269260
- `steam-web-pp-cli igcversion-1269260 get-server-version` — GetServerVersion operation of IGCVersion_1269260

**igcversion-1422450** — Manage igcversion 1422450

- `steam-web-pp-cli igcversion-1422450 get-client-version` — GetClientVersion operation of IGCVersion_1422450
- `steam-web-pp-cli igcversion-1422450 get-server-version` — GetServerVersion operation of IGCVersion_1422450

**igcversion-440** — Manage igcversion 440

- `steam-web-pp-cli igcversion-440 get-client-version` — GetClientVersion operation of IGCVersion_440
- `steam-web-pp-cli igcversion-440 get-server-version` — GetServerVersion operation of IGCVersion_440

**igcversion-570** — Manage igcversion 570

- `steam-web-pp-cli igcversion-570 get-client-version` — GetClientVersion operation of IGCVersion_570
- `steam-web-pp-cli igcversion-570 get-server-version` — GetServerVersion operation of IGCVersion_570

**igcversion-583950** — Manage igcversion 583950

- `steam-web-pp-cli igcversion-583950 get-client-version` — GetClientVersion operation of IGCVersion_583950
- `steam-web-pp-cli igcversion-583950 get-server-version` — GetServerVersion operation of IGCVersion_583950

**igcversion-730** — Manage igcversion 730

- `steam-web-pp-cli igcversion-730` — GetServerVersion operation of IGCVersion_730

**ihelp-request-logs-service** — Manage ihelp request logs service

- `steam-web-pp-cli ihelp-request-logs-service get-application-log-demand` — GetApplicationLogDemand operation of IHelpRequestLogsService
- `steam-web-pp-cli ihelp-request-logs-service upload-user-application-log` — UploadUserApplicationLog operation of IHelpRequestLogsService

**iinventory-service** — Manage iinventory service

- `steam-web-pp-cli iinventory-service combine-item-stacks` — CombineItemStacks operation of IInventoryService
- `steam-web-pp-cli iinventory-service get-price-sheet` — GetPriceSheet operation of IInventoryService
- `steam-web-pp-cli iinventory-service split-item-stack` — SplitItemStack operation of IInventoryService

**iplayer-service** — Manage iplayer service

- `steam-web-pp-cli iplayer-service get-badges` — GetBadges operation of IPlayerService
- `steam-web-pp-cli iplayer-service get-community-badge-progress` — GetCommunityBadgeProgress operation of IPlayerService
- `steam-web-pp-cli iplayer-service get-owned-games` — GetOwnedGames operation of IPlayerService
- `steam-web-pp-cli iplayer-service get-recently-played-games` — GetRecentlyPlayedGames operation of IPlayerService
- `steam-web-pp-cli iplayer-service get-steam-level` — GetSteamLevel operation of IPlayerService
- `steam-web-pp-cli iplayer-service is-playing-shared-game` — IsPlayingSharedGame operation of IPlayerService
- `steam-web-pp-cli iplayer-service record-offline-playtime` — RecordOfflinePlaytime operation of IPlayerService

**iportal2-leaderboards-620** — Manage iportal2 leaderboards 620

- `steam-web-pp-cli iportal2-leaderboards-620` — GetBucketizedData operation of IPortal2Leaderboards_620

**ipublished-file-service** — Manage ipublished file service

- `steam-web-pp-cli ipublished-file-service get-details` — GetDetails operation of IPublishedFileService
- `steam-web-pp-cli ipublished-file-service get-sub-section-data` — GetSubSectionData operation of IPublishedFileService
- `steam-web-pp-cli ipublished-file-service get-user-file-count` — GetUserFileCount operation of IPublishedFileService
- `steam-web-pp-cli ipublished-file-service get-user-files` — GetUserFiles operation of IPublishedFileService
- `steam-web-pp-cli ipublished-file-service get-user-vote-summary` — GetUserVoteSummary operation of IPublishedFileService
- `steam-web-pp-cli ipublished-file-service query-files` — QueryFiles operation of IPublishedFileService

**isteam-apps** — Manage isteam apps

- `steam-web-pp-cli isteam-apps get-sdrconfig` — GetSDRConfig operation of ISteamApps
- `steam-web-pp-cli isteam-apps get-servers-at-address` — GetServersAtAddress operation of ISteamApps
- `steam-web-pp-cli isteam-apps up-to-date-check` — UpToDateCheck operation of ISteamApps

**isteam-broadcast** — Manage isteam broadcast

- `steam-web-pp-cli isteam-broadcast player-stats` — PlayerStats operation of ISteamBroadcast
- `steam-web-pp-cli isteam-broadcast viewer-heartbeat` — ViewerHeartbeat operation of ISteamBroadcast

**isteam-cdn** — Manage isteam cdn

- `steam-web-pp-cli isteam-cdn set-client-filters` — SetClientFilters operation of ISteamCDN
- `steam-web-pp-cli isteam-cdn set-performance-stats` — SetPerformanceStats operation of ISteamCDN

**isteam-directory** — Manage isteam directory

- `steam-web-pp-cli isteam-directory get-cmlist` — GetCMList operation of ISteamDirectory
- `steam-web-pp-cli isteam-directory get-cmlist-for-connect` — GetCMListForConnect operation of ISteamDirectory
- `steam-web-pp-cli isteam-directory get-steam-pipe-domains` — GetSteamPipeDomains operation of ISteamDirectory

**isteam-economy** — Manage isteam economy

- `steam-web-pp-cli isteam-economy get-asset-class-info` — GetAssetClassInfo operation of ISteamEconomy
- `steam-web-pp-cli isteam-economy get-asset-prices` — GetAssetPrices operation of ISteamEconomy

**isteam-news** — Manage isteam news

- `steam-web-pp-cli isteam-news get-news-for-app` — GetNewsForApp operation of ISteamNews
- `steam-web-pp-cli isteam-news get-news-for-app-isteamnews` — GetNewsForApp operation of ISteamNews

**isteam-remote-storage** — Manage isteam remote storage

- `steam-web-pp-cli isteam-remote-storage get-collection-details` — GetCollectionDetails operation of ISteamRemoteStorage
- `steam-web-pp-cli isteam-remote-storage get-published-file-details` — GetPublishedFileDetails operation of ISteamRemoteStorage
- `steam-web-pp-cli isteam-remote-storage get-ugcfile-details` — GetUGCFileDetails operation of ISteamRemoteStorage

**isteam-user** — Manage isteam user

- `steam-web-pp-cli isteam-user get-friend-list` — GetFriendList operation of ISteamUser
- `steam-web-pp-cli isteam-user get-player-bans` — GetPlayerBans operation of ISteamUser
- `steam-web-pp-cli isteam-user get-player-summaries` — GetPlayerSummaries operation of ISteamUser
- `steam-web-pp-cli isteam-user get-player-summaries-isteamuser` — GetPlayerSummaries operation of ISteamUser
- `steam-web-pp-cli isteam-user get-user-group-list` — GetUserGroupList operation of ISteamUser
- `steam-web-pp-cli isteam-user resolve-vanity-url` — ResolveVanityURL operation of ISteamUser

**isteam-user-auth** — Manage isteam user auth

- `steam-web-pp-cli isteam-user-auth` — AuthenticateUserTicket operation of ISteamUserAuth

**isteam-user-oauth** — Manage isteam user oauth

- `steam-web-pp-cli isteam-user-oauth` — GetTokenDetails operation of ISteamUserOAuth

**isteam-user-stats** — Manage isteam user stats

- `steam-web-pp-cli isteam-user-stats get-global-achievement-percentages-for-app` — GetGlobalAchievementPercentagesForApp operation of ISteamUserStats
- `steam-web-pp-cli isteam-user-stats get-global-achievement-percentages-for-app-isteamuserstats` — GetGlobalAchievementPercentagesForApp operation of ISteamUserStats
- `steam-web-pp-cli isteam-user-stats get-global-stats-for-game` — GetGlobalStatsForGame operation of ISteamUserStats
- `steam-web-pp-cli isteam-user-stats get-number-of-current-players` — GetNumberOfCurrentPlayers operation of ISteamUserStats
- `steam-web-pp-cli isteam-user-stats get-player-achievements` — GetPlayerAchievements operation of ISteamUserStats
- `steam-web-pp-cli isteam-user-stats get-schema-for-game` — GetSchemaForGame operation of ISteamUserStats
- `steam-web-pp-cli isteam-user-stats get-schema-for-game-isteamuserstats` — GetSchemaForGame operation of ISteamUserStats
- `steam-web-pp-cli isteam-user-stats get-user-stats-for-game` — GetUserStatsForGame operation of ISteamUserStats
- `steam-web-pp-cli isteam-user-stats get-user-stats-for-game-isteamuserstats` — GetUserStatsForGame operation of ISteamUserStats

**isteam-web-apiutil** — Manage isteam web apiutil

- `steam-web-pp-cli isteam-web-apiutil get-server-info` — GetServerInfo operation of ISteamWebAPIUtil
- `steam-web-pp-cli isteam-web-apiutil get-supported-apilist` — GetSupportedAPIList operation of ISteamWebAPIUtil

**istore-service** — Manage istore service

- `steam-web-pp-cli istore-service get-app-list` — Gets a list of all apps available on the Steam Store
- `steam-web-pp-cli istore-service get-games-followed` — GetGamesFollowed operation of IStoreService
- `steam-web-pp-cli istore-service get-games-followed-count` — GetGamesFollowedCount operation of IStoreService
- `steam-web-pp-cli istore-service get-recommended-tags-for-user` — GetRecommendedTagsForUser operation of IStoreService

**itfitems-440** — Manage itfitems 440

- `steam-web-pp-cli itfitems-440 get-golden-wrenches` — GetGoldenWrenches operation of ITFItems_440
- `steam-web-pp-cli itfitems-440 get-golden-wrenches-itfitems440` — GetGoldenWrenches operation of ITFItems_440

**itfpromos-440** — Manage itfpromos 440

- `steam-web-pp-cli itfpromos-440 get-item-id` — GetItemID operation of ITFPromos_440
- `steam-web-pp-cli itfpromos-440 grant-item` — GrantItem operation of ITFPromos_440

**itfpromos-620** — Manage itfpromos 620

- `steam-web-pp-cli itfpromos-620 get-item-id` — GetItemID operation of ITFPromos_620
- `steam-web-pp-cli itfpromos-620 grant-item` — GrantItem operation of ITFPromos_620

**itfsystem-440** — Manage itfsystem 440

- `steam-web-pp-cli itfsystem-440` — GetWorldStatus operation of ITFSystem_440

**iwishlist-service** — Manage iwishlist service

- `steam-web-pp-cli iwishlist-service get-wishlist` — GetWishlist operation of IWishlistService
- `steam-web-pp-cli iwishlist-service get-wishlist-item-count` — GetWishlistItemCount operation of IWishlistService
- `steam-web-pp-cli iwishlist-service get-wishlist-sorted-filtered` — GetWishlistSortedFiltered operation of IWishlistService


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
steam-web-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Recipes


### Find friends who own a game but never played it

```bash
steam-web-pp-cli friends compare 1245620 --my-steamid 76561197960287930 --filter owns-zero-hours --agent --select results.persona_name,results.steamid
```

Throttled fan-out to identify dormant owners — the people most likely to actually play if invited.

### Plan a completion run on one game

```bash
steam-web-pp-cli achievement-hunt 1245620 --steamid 76561197960287930 --locked --agent --select achievements.display_name,achievements.global_pct
```

Surfaces every locked achievement plus its global rarity so you can rank effort vs. reward.

### Find low-hanging fruit across your whole library

```bash
steam-web-pp-cli next-achievement --steamid 76561197960287930 --limit 10 --json
```

Cross-library SQL — the ten achievements with the highest global unlock rate that you still don't have.

### Audit your backlog after a sale

```bash
steam-web-pp-cli library audit 76561197960287930 --never-launched --json
```

Reveals what you bought and didn't open; pairs well with `library audit --bounce` for the under-2h paid set.

### Search your synced news feed for a topic

```bash
steam-web-pp-cli news search 'mod support' --since 2026-04-01 --json
```

FTS5 search over every news post you've synced — useful for tracking when devs ship features you care about across all your owned games.

## Auth Setup

Standard Steam Web API key auth: get one at https://steamcommunity.com/dev/apikey and set STEAM_WEB_API_KEY in your environment. The key is sent as a `?key=` query parameter on every request. Some endpoints (server time, app list, news) require no auth at all and work without a key. The IAuthenticationService endpoints in the spec are NOT for Web API auth — they implement Steam's interactive QR-code login flow. They remain reachable via the CLI subcommand and the MCP code-orchestration pair for completeness but should not be used as part of normal Web API workflows.

Run `steam-web-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  steam-web-pp-cli iauthentication-service begin-auth-session-via-credentials --account-name example-resource --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Explicit retries** — use `--idempotent` only when an already-existing create should count as success

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal — piped/agent consumers get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
steam-web-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
steam-web-pp-cli feedback --stdin < notes.txt
steam-web-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.steam-web-pp-cli/feedback.jsonl`. They are never POSTed unless `STEAM_WEB_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `STEAM_WEB_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

Write what *surprised* you, not a bug report. Short, specific, one line: that is the part that compounds.

## Output Delivery

Every command accepts `--deliver <sink>`. The output goes to the named sink in addition to (or instead of) stdout, so agents can route command results without hand-piping. Three sinks are supported:

| Sink | Effect |
|------|--------|
| `stdout` | Default; write to stdout only |
| `file:<path>` | Atomically write output to `<path>` (tmp + rename) |
| `webhook:<url>` | POST the output body to the URL (`application/json` or `application/x-ndjson` when `--compact`) |

Unknown schemes are refused with a structured error naming the supported set. Webhook failures return non-zero and log the URL + HTTP status on stderr.

## Named Profiles

A profile is a saved set of flag values, reused across invocations. Use it when a scheduled agent calls the same command every run with the same configuration - HeyGen's "Beacon" pattern.

```
steam-web-pp-cli profile save briefing --json
steam-web-pp-cli --profile briefing iauthentication-service begin-auth-session-via-credentials --account-name example-resource
steam-web-pp-cli profile list --json
steam-web-pp-cli profile show briefing
steam-web-pp-cli profile delete briefing --yes
```

Explicit flags always win over profile values; profile values win over defaults. `agent-context` lists all available profiles under `available_profiles` so introspecting agents discover them at runtime.

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error (wrong arguments) |
| 3 | Resource not found |
| 4 | Authentication required |
| 5 | API error (upstream issue) |
| 7 | Rate limited (wait and retry) |
| 10 | Config error |

## Argument Parsing

Parse `$ARGUMENTS`:

1. **Empty, `help`, or `--help`** → show `steam-web-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

1. Install the MCP server:
   ```bash
   go install github.com/mvanhorn/printing-press-library/library/other/steam-web/cmd/steam-web-pp-mcp@latest
   ```
2. Register with Claude Code:
   ```bash
   claude mcp add steam-web-pp-mcp -- steam-web-pp-mcp
   ```
3. Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which steam-web-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   steam-web-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `steam-web-pp-cli <command> --help`.
