---
name: pp-ercot
description: "Printing Press CLI for Ercot. Client/Developer RESTFul web services documentation for ERCOT Market Information List (EMIL) products."
author: "synota-inc"
license: "Apache-2.0"
argument-hint: "<command> [args] | install cli|mcp"
allowed-tools: "Read Bash"
metadata:
  openclaw:
    requires:
      bins:
        - ercot-pp-cli
---

# Ercot — Printing Press CLI

## Prerequisites: Install the CLI

This skill drives the `ercot-pp-cli` binary. **You must verify the CLI is installed before invoking any command from this skill.** If it is missing, install it first:

1. Install via the Printing Press installer:
   ```bash
   npx -y @mvanhorn/printing-press install ercot --cli-only
   ```
2. Verify: `ercot-pp-cli --version`
3. Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is on `$PATH`.

If the `npx` install fails before this CLI has a public-library category, install Node or use the category-specific Go fallback after publish.

If `--version` reports "command not found" after install, the install step did not put the binary on `$PATH`. Do not proceed with skill commands until verification succeeds.

Client/Developer RESTFul web services documentation for ERCOT Market Information List (EMIL) products.

## When Not to Use This CLI

Do not activate this CLI for requests that require creating, updating, deleting, publishing, commenting, upvoting, inviting, ordering, sending messages, booking, purchasing, or changing remote state. This printed CLI exposes read-only commands for inspection, export, sync, and analysis.

## Command Reference

**archive** — Manage archive

- `ercot-pp-cli archive <emilId>` — Available report archives for a specified EMIL product.

**ercot-public-client-version** — Manage ercot public client version

- `ercot-pp-cli ercot-public-client-version` — Retrieve the current version for ERCOT Public API system.

**np3-233-cd** — <b>Hourly Resource Outage Capacity</b><br/><br/>This report includes all approved and accepted Planned, Forced and Maintenance Resource outages EXCEPT Resource outages for retirement of old equipment, seasonal mothballed (during the outaged season), and mothballed. The outage capacity in this report reflects aggregated ACTIVE resource outaged capacity by Load Zone sourced from the Outage Scheduler (OS) for the next 168 hours and is published every hour. Columns 'C\' - 'F' consists of the aggregated outaged capacity of ALL the ACTIVE resource outages by Load Zone in the OS for each hour excluding the outages stated above, IRR resource outages, and new equipment outages. Columns 'G' - 'J' consists of the aggregated outaged capacity of ALL the ACTIVE IRR resource outages by Load Zone in the OS for each hour excluding the outages stated above, outages in columns 'C' - 'F' and new equipment outages. Columns 'K' - 'N' consists of the aggregated outaged capacity of ALL the ACTIVE New Equipment Energization resource outages by Load Zone. Note that this report contains OS data only and does not look at telemetry. It includes both entire resource outage and de-rates.

- `ercot-pp-cli np3-233-cd` — Hourly Resource Outage Capacity

**np3-565-cd** — <b>Seven-Day Load Forecast by Model and Weather Zone</b><br/><br/>Hourly system-wide Mid-Term Load Forecasts (MTLFs) for all forecast models with an indicator for which forecast was in use by ERCOT at the time of publication for current day plus the next 7.

- `ercot-pp-cli np3-565-cd` — Seven-Day Load Forecast by Model and Weather Zone

**np3-566-cd** — <b>Seven-Day Load Forecast by Model and Study Area</b><br/><br/>Forecasted hourly demands by Study Area, for the current day plus the next seven days.

- `ercot-pp-cli np3-566-cd` — Seven-Day Load Forecast by Model and Study Area

**np3-910-er** — <b>2-Day Real Time Gen and Load Data Reports</b><br/><br/>This report will contain all 48 Hour disclosure data related to Real Time. The following individual files are included in the report:NP3-919-EX 48-hour Aggregate Output Schedules; NP3-920-EX 48-hour Aggregate Generation Summary; NP3-921-EX 48-hour Aggregate Load Summary; NP3-922-EX 48-hour Aggregate Dynamically Schedules Resources and Load; NP3-924-EX 48-hour Aggregate Load Summary by Disclosure Areas; NP3-931-EX 48-hour Aggregate Output Schedules by Disclosure Areas; NP3-935-EX 48-hour Aggregate Generation Summary by Disclosure Areas (previously 48 Hour Real Time Gen and Load Data Reports).

- `ercot-pp-cli np3-910-er get-data-83` — 2 Day Aggregated Output Schedule West
- `ercot-pp-cli np3-910-er get-data-84` — 2 Day Aggregated Output Schedule South
- `ercot-pp-cli np3-910-er get-data-85` — 2 Day Aggregated Output Schedule North
- `ercot-pp-cli np3-910-er get-data-86` — 2 Day Aggregated Output Schedule Houston
- `ercot-pp-cli np3-910-er get-data-87` — 2 Day Aggregated Output Schedule
- `ercot-pp-cli np3-910-er get-data-88` — 2 Day Aggregated Load Summary West
- `ercot-pp-cli np3-910-er get-data-89` — 2 Day Aggregated Load Summary South
- `ercot-pp-cli np3-910-er get-data-90` — 2 Day Aggregated Load Summary North
- `ercot-pp-cli np3-910-er get-data-91` — 2 Day Aggregated Load Summary Houston
- `ercot-pp-cli np3-910-er get-data-92` — 2 Day Aggregated Load Summary
- `ercot-pp-cli np3-910-er get-data-93` — 2 Day Aggregated Generation Summary West
- `ercot-pp-cli np3-910-er get-data-94` — 2 Day Aggregated Generation Summary South
- `ercot-pp-cli np3-910-er get-data-95` — 2 Day Aggregated Generation Summary North
- `ercot-pp-cli np3-910-er get-data-96` — 2 Day Aggregated Generation Summary Houston
- `ercot-pp-cli np3-910-er get-data-97` — 2 Day Aggregated Generation Summary
- `ercot-pp-cli np3-910-er get-data-98` — 2 Day Aggregated DSR Loads

**np3-911-er** — <b>2-Day Ancillary Services Reports</b><br/><br/>This report will contain all 48 Hour disclosure data related to DAM. The following individual files are included in the report:NP3-959-EX 48-hour Aggregate AS Offers; NP3-960-EX 48-hour Self-Arranged AS; NP3-961-EX 48-hour Cleared DAM AS (previously named 48 Hour Ancillary Services Reports).

- `ercot-pp-cli np3-911-er get-data-57` — 2 Day Self Arranged Ancillary Service RRSUFR
- `ercot-pp-cli np3-911-er get-data-58` — 2 Day Self Arranged Ancillary Service RRSPFR
- `ercot-pp-cli np3-911-er get-data-59` — 2 Day Self Arranged Ancillary Service RRSFFR
- `ercot-pp-cli np3-911-er get-data-60` — 2 Day Self Arranged Ancillary Service REGUP
- `ercot-pp-cli np3-911-er get-data-61` — 2 Day Self Arranged Ancillary Service REGDN
- `ercot-pp-cli np3-911-er get-data-62` — 2 Day Self Arranged Ancillary Service NSPNM
- `ercot-pp-cli np3-911-er get-data-63` — 2 Day Self Arranged Ancillary Service NSPIN
- `ercot-pp-cli np3-911-er get-data-64` — 2 Day Self Arranged Ancillary Service ECRSS
- `ercot-pp-cli np3-911-er get-data-65` — 2 Day Self Arranged Ancillary Service ECRSM
- `ercot-pp-cli np3-911-er get-data-66` — 2 Day Cleared DAM Ancillary Service RRSUFR
- `ercot-pp-cli np3-911-er get-data-67` — 2 Day Cleared DAM Ancillary Service RRSPFR
- `ercot-pp-cli np3-911-er get-data-68` — 2 Day Cleared DAM Ancillary Service RRSFFR
- `ercot-pp-cli np3-911-er get-data-69` — 2 Day Cleared DAM Ancillary Service REGUP
- `ercot-pp-cli np3-911-er get-data-70` — 2 Day Cleared DAM Ancillary Service REGDN
- `ercot-pp-cli np3-911-er get-data-71` — 2 Day Cleared DAM Ancillary Service NSPIN
- `ercot-pp-cli np3-911-er get-data-72` — 2 Day Cleared DAM Ancillary Service ECRSS
- `ercot-pp-cli np3-911-er get-data-73` — 2 Day Cleared DAM Ancillary Service ECRSM
- `ercot-pp-cli np3-911-er get-data-74` — 2 Day Aggregated Ancillary Service Offers RRSUFR
- `ercot-pp-cli np3-911-er get-data-75` — 2 Day Aggregated Ancillary Service Offers RRSPFR
- `ercot-pp-cli np3-911-er get-data-76` — 2 Day Aggregated Ancillary Service Offers RRSFFR
- `ercot-pp-cli np3-911-er get-data-77` — 2 Day Aggregated Ancillary Service Offers REGUP
- `ercot-pp-cli np3-911-er get-data-78` — 2 Day Aggregated Ancillary Service Offers REGDN
- `ercot-pp-cli np3-911-er get-data-79` — 2 Day Aggregated Ancillary Service Offers ONNS
- `ercot-pp-cli np3-911-er get-data-80` — 2 Day Aggregated Ancillary Service Offers OFFNS
- `ercot-pp-cli np3-911-er get-data-81` — 2 Day Aggregated Ancillary Service Offers ECRSS
- `ercot-pp-cli np3-911-er get-data-82` — 2 Day Aggregated Ancillary Service Offers ECRSM

**np3-965-er** — <b>60-Day SCED Disclosure Reports</b><br/><br/>This report will contain all 60 day disclosure data related to SCED. The following individual files are included in the report: NP3-967-EX 61-day QSE-specific Self-Arranged AS in SCED NP3-968-EX 61-day Generation Resource data in SCED NP3-969-EX 61-day Load Resource data in SCED NP3-970-EX 61-day Dynamically Scheduled Resource and Loads in SCED NP3-971-EX 61-day Inc/Dec Offer Curves in SCED NP3-973-EX 61-day Settlement Metered Net Energy for Generation Resources and NP6-585-ER 60 Day HDL and LDL Manual Override Summary

- `ercot-pp-cli np3-965-er get-data-51` — 60 Day SCED Settlement Metered Net Energy for Generation Resources
- `ercot-pp-cli np3-965-er get-data-52` — 60 Day QSE-specific Self-Arranged AS in SCED
- `ercot-pp-cli np3-965-er get-data-53` — 60 Day SCED Gen Resource Data
- `ercot-pp-cli np3-965-er get-data-54` — 60 Day SCED DSR Load Data
- `ercot-pp-cli np3-965-er get-data-55` — 60 Day Load Resource Data in SCED
- `ercot-pp-cli np3-965-er get-data-56` — 60 Day HDL and LDL Manual Override Summary

**np3-966-er** — <b>60-Day DAM Disclosure Reports</b><br/><br/>This report will contain all 60 day disclosure data related to DAM. The following individual files are included in the report: NP3-974-EX 61-day QSE-specific Self-Arranged AS in DAM NP3-975-EX 61-day Generation Resource data in DAM NP3-976-EX 61-day Generation Resource AS Offers in DAM NP3-977-EX 61-day Load Resource data in DAM NP3-978-EX 61-day Load Resource AS Offers in DAM NP3-979-EX 61-day Settlement Point Data in DAM- Energy Only Offers NP3-980-EX 61-day Settlement Point Data in DAM- Energy Only Offer Award NP3-981-EX 61-day Settlement Point Data in DAM- Energy Bids NP3-982-EX 61-day Settlement Point Data in DAM- Bids Award NP3-983-EX 61-day Settlement Point Data in DAM- CRR Offers NP3-984-EX 61-day Settlement Point Data in DAM- CRR Awards NP3-985-EX 61-day Settlement Point Data Point-to-Point Obligation Bids NP3-986-EX 61-day Settlement Point Data Point-to-Point Obligation Bid Awards

- `ercot-pp-cli np3-966-er get-data-38` — 60 Day DAM QSE Self Arranged AS
- `ercot-pp-cli np3-966-er get-data-39` — 60 Day DAM PTP Obligation Option Awards
- `ercot-pp-cli np3-966-er get-data-40` — 60 Day DAM PTP Obligation Option
- `ercot-pp-cli np3-966-er get-data-41` — 60 Day DAM PTP Obligation Bids
- `ercot-pp-cli np3-966-er get-data-42` — 60 Day DAM PTP Obligation Bid Awards
- `ercot-pp-cli np3-966-er get-data-43` — 60 Day DAM Load Resource Data
- `ercot-pp-cli np3-966-er get-data-44` — 60 Day DAM Load Resources AS Offers
- `ercot-pp-cli np3-966-er get-data-45` — 60 Day DAM Generation Resource Data
- `ercot-pp-cli np3-966-er get-data-46` — 60 Day DAM Generation Resources AS Offers
- `ercot-pp-cli np3-966-er get-data-47` — 60 Day DAM Energy Only Offers
- `ercot-pp-cli np3-966-er get-data-48` — 60 Day DAM Energy Offer Only Awards
- `ercot-pp-cli np3-966-er get-data-49` — 60 Day DAM Energy Bids
- `ercot-pp-cli np3-966-er get-data-50` — 60 Day DAM Energy Bid Awards

**np3-990-ex** — <b>60-Day SASM Disclosure Reports</b><br/><br/>This report will contain all 60 day disclosure data related to SASM for Generation and Load Resources. The following individual files are included in the report: 60d_SASM_Generation_Resource_AS_Offers-YY-MMM-DD.csv60d_SASM_Load_Resource_AS_Offers-YY-MMM-DD.csv60d_SASM_Generation_Resource_AS_Offer Awards-YY-MMM-DD.csv60d_SASM_Load_Resource_AS_Offer_Awards-YY-MMM-DD.csv

- `ercot-pp-cli np3-990-ex get-data-34` — 60 Day SASM Load Resource AS Offers
- `ercot-pp-cli np3-990-ex get-data-35` — 60 Day SASM Load Resource AS Offer Awards
- `ercot-pp-cli np3-990-ex get-data-36` — 60 Day SASM Generation Resource AS Offers
- `ercot-pp-cli np3-990-ex get-data-37` — 60 Day SASM Generation Resource AS Offer Awards

**np3-991-ex** — <b>60-Day COP All Updates</b><br/><br/>This report will contain all iterative Current Operating Plan (COP) submissions where a change has occurred for the operating day. Previously named 60-Day Current Operating Plan.

- `ercot-pp-cli np3-991-ex` — 60-Day COP All Updates

**np4-159-cd** — <b>Load Distribution Factors</b><br/><br/>Load forecast distribution factors from which Market Participants can calculate Load at the Electrical Bus level by hour for the next seven days.

- `ercot-pp-cli np4-159-cd` — Load Distribution Factors

**np4-179-cd** — <b>Total Ancillary Service Offers</b><br/><br/>The total quantity in MW of Offers per Ancillary Service per hour from the Day-Ahead Market for the last thirty days on a daily basis which includes the following AS types: REGDN, REGUP, RRSPFR, RRSFFR, RRSUFR, & NSPIN.

- `ercot-pp-cli np4-179-cd` — Total Ancillary Service Offers

**np4-183-cd** — <b>DAM Hourly LMPs</b><br/><br/>The Hourly Locational Marginal Prices per electrical bus from the Day-Ahead Market for the last thirty days on a daily basis.

- `ercot-pp-cli np4-183-cd` — DAM Hourly LMPs

**np4-188-cd** — <b>DAM Clearing Prices for Capacity</b><br/><br/>The Market Clearing Prices for Capacity for all Ancillary Services from the Day-Ahead Market for the last thirty days on a daily basis.

- `ercot-pp-cli np4-188-cd` — DAM Clearing Prices for Capacity

**np4-190-cd** — <b>DAM Settlement Point Prices</b><br/><br/>The Settlement Point Prices for all Resource Nodes, Load Zones, and Trading Hubs from the Day-Ahead Market for the last thirty days on a daily basis.

- `ercot-pp-cli np4-190-cd` — DAM Settlement Point Prices

**np4-191-cd** — <b>DAM Shadow Prices</b><br/><br/>The active and binding constraints as well as the associated shadow prices from the Day-Ahead Market for the last thirty days on a daily basis.

- `ercot-pp-cli np4-191-cd` — DAM Shadow Prices

**np4-196-m** — <b>DAM Price Corrections</b><br/><br/>Day-Ahead Market price corrections.

- `ercot-pp-cli np4-196-m get-data-24` — DAM Price Corrections for SPP
- `ercot-pp-cli np4-196-m get-data-25` — DAM Price Corrections for MCPC
- `ercot-pp-cli np4-196-m get-data-26` — DAM Price Corrections for EBLMP

**np4-197-m** — <b>RTM Price Corrections</b><br/><br/>Real-Time Market price corrections.

- `ercot-pp-cli np4-197-m get-data-18` — RTM Price Corrections for SPP
- `ercot-pp-cli np4-197-m get-data-19` — RTM Price Corrections SP LMP
- `ercot-pp-cli np4-197-m get-data-20` — RTM Price Corrections for SOG Price
- `ercot-pp-cli np4-197-m get-data-21` — RTM Price Corrections for SOG LMP
- `ercot-pp-cli np4-197-m get-data-22` — RTM Price Corrections for Shadow Prices
- `ercot-pp-cli np4-197-m get-data-23` — RTM Price Corrections for EB LMP

**np4-33-cd** — <b>DAM Ancillary Service Plan</b><br/><br/>Ancillary Service requirements by type and quantity for each hour of the current day plus the next 6 days.

- `ercot-pp-cli np4-33-cd` — DAM Ancillary Service Plan

**np4-523-cd** — <b>DAM System Lambda</b><br/><br/>System lambda of each successful DAM.

- `ercot-pp-cli np4-523-cd` — DAM System Lambda

**np4-732-cd** — <b>Wind Power Production - Hourly Averaged Actual and Forecasted Values</b><br/><br/>This report is posted every hour and includes System-wide and Regional actual hourly averaged wind power production, STWPF, WGRPP and COP HSLs for On-Line WGRs for a rolling historical 48-hour period as well as the System-wide and Regional STWPF, WGRPP and COP HSLs for On-Line WGRs for the rolling future 168-hour period. Our forecasts attempt to predict HSL, which is uncurtailed power generation potential. Actual system-wide generation, which is included in this report as "ACTUAL_SYSTEM_WIDE" or "SYSTEM_WIDE" is impacted by curtailments. Because of this, the data in this report should not be used to evaluate forecast performance. Steps will be taken to include Actual System-wide HSL in this report in the future.

- `ercot-pp-cli np4-732-cd` — Wind Power Production - Hourly Averaged Actual and Forecasted Values

**np4-733-cd** — <b>Wind Power Production - Actual 5-Minute Averaged Values</b><br/><br/>This report is posted every 5 minutes and includes System-wide and Regional actual 5-min averaged wind power production for a rolling historical 60-minute period.

- `ercot-pp-cli np4-733-cd` — Wind Power Production - Actual 5-Minute Averaged Values

**np4-737-cd** — <b>Solar Power Production - Hourly Averaged Actual and Forecasted Values</b><br/><br/>This report includes System-wide actual hourly averaged solar power production, STPPF, PVGRPP, and COP HSLs for On-Line PVGRs for a rolling historical 48-hour period as well as the System-wide STPPF, PVGRPP and COP HSLs for On-Line PVGRs for the rolling future 168-hour period. Our forecasts attempt to predict HSL, which is uncurtailed power generation potential. Actual system-wide generation, which is included in this report as "ACTUAL_SYSTEM_WIDE" or "SYSTEM_WIDE" is impacted by curtailments. Because of this, the data in this report should not be used to evaluate forecast performance. Steps will be taken to include Actual System-wide HSL in this report in the future.

- `ercot-pp-cli np4-737-cd` — Solar Power Production - Hourly Averaged Actual and Forecasted Values

**np4-738-cd** — <b>Solar Power Production - Actual 5-Minute Averaged Values</b><br/><br/>This report is posted every 5 minutes and includes System-wide actual 5-minute averaged solar power production for On-Line PVGRs for a rolling historical 60-minute period.

- `ercot-pp-cli np4-738-cd` — Solar Power Production - Actual 5-Minute Averaged Values

**np4-742-cd** — <b>Wind Power Production - Hourly Averaged Actual and Forecasted Values by Geographical Region</b><br/><br/>This report is posted every hour and includes System-wide and Geographic Regional actual hourly averaged wind power production, STWPF, WGRPP and COP HSLs for On-Line WGRs for a rolling historical 48-hour period as well as the System-wide and Regional STWPF, WGRPP and COP HSLs for On-Line WGRs for the rolling future 168-hour period. Our forecasts attempt to predict HSL, which is uncurtailed power generation potential. Actual system-wide generation, which is included in this report as "ACTUAL_SYSTEM_WIDE" or "SYSTEM_WIDE" is impacted by curtailments. Because of this, the data in this report should not be used to evaluate forecast performance. Steps will be taken to include Actual System-wide HSL in this report in the future.

- `ercot-pp-cli np4-742-cd` — Wind Power Production - Hourly Averaged Actual and Forecasted Values by Geographical Region

**np4-743-cd** — <b>Wind Power Production - Actual 5-Minute Averaged Values by Geographical Region</b><br/><br/>This report is posted every 5 minutes and includes System-wide and Geographic Regional actual 5-minute averaged wind power production for a rolling historical 60-minute period.

- `ercot-pp-cli np4-743-cd` — Wind Power Production - Actual 5-Minute Averaged Values by Geographical Region

**np4-745-cd** — <b>Solar Power Production - Hourly Averaged Actual and Forecasted Values by Geographical Region</b><br/><br/>This report is posted every hour and includes System-wide and geographic regional hourly averaged solar power production, STPPF, PVGRPP, and COP HSL for On-Line PVGRs for a rolling historical 48-hour period as well as the system-wide and regional STPPF, PVGRPP, and COP HSL for On-Line PVGRs for the rolling future 168-hour period. System-wide and regional generation, are included in this report under column labels with "GEN_" prefixes. ERCOT's forecasts attempt to predict HSL, which is uncurtailed power generation potential. Since generation is impacted by curtailments, the data in this report should not be used to evaluate forecast performance. Steps will be taken to include HSL in this report in the future.

- `ercot-pp-cli np4-745-cd` — Solar Power Production - Hourly Averaged Actual and Forecasted Values by Geographical Region

**np4-746-cd** — <b>Solar Power Production - Actual 5-Minute Averaged Values by Geographical Region</b><br/><br/>This report is posted every 5 minutes and includes system-wide and geographic regional 5-minute averaged solar power production for a rolling historical 60-minute period.

- `ercot-pp-cli np4-746-cd` — Solar Power Production - Actual 5-Minute Averaged Values by Geographical Region

**np6-322-cd** — <b>SCED System Lambda</b><br/><br/>System lambda of each successful SCED.

- `ercot-pp-cli np6-322-cd` — SCED System Lambda

**np6-345-cd** — <b>Actual System Load by Weather Zone</b><br/><br/>Report of Actual hourly load data by weather zone and ERCOT total.

- `ercot-pp-cli np6-345-cd` — Actual System Load by Weather Zone

**np6-346-cd** — <b>Actual System Load by Forecast Zone</b><br/><br/>A daily report of Actual System Load by Forecast Zone for each hour of the previous operating day.

- `ercot-pp-cli np6-346-cd` — Actual System Load by Forecast Zone

**np6-787-cd** — <b>LMPs by Electrical Bus</b><br/><br/>The Locational Marginal Price for each Electrical Bus, normally produced by SCED every five minutes.

- `ercot-pp-cli np6-787-cd` — LMP by Electrical Bus

**np6-788-cd** — <b>LMPs by Resource Nodes, Load Zones and Trading Hubs</b><br/><br/>The Locational Marginal Price for each Settlement Point, normally produced by SCED every five minutes.

- `ercot-pp-cli np6-788-cd` — LMPs by Resource Nodes, Load Zones and Trading Hubs

**np6-86-cd** — <b>SCED Shadow Prices and Binding Transmission Constraints</b><br/><br/>The report for Shadow Prices of binding/violated constraints in SCED. The report shows the contingency name, overloaded element details (element name, from/to station name and kV level), Shadow Price ( price for resolving one MW of the constraint), Penalty for violating the constraint (MaxShadow Price), overloaded element limit and flow (value)pairs that caused such constraint.

- `ercot-pp-cli np6-86-cd` — SCED Shadow Prices and Binding Transmission Constraints

**np6-905-cd** — <b>Settlement Point Prices at Resource Nodes, Hubs and Load Zones</b><br/><br/>Settlement Point Price for each Settlement Point, produced from SCED LMPs every 15 minutes.

- `ercot-pp-cli np6-905-cd` — Settlement Point Prices at Resource Nodes, Hubs and Load Zones

**np6-970-cd** — <b>RTD Indicative LMPs by Resource Nodes, Load Zones and Hubs</b><br/><br/>This report is posted after every Look Ahead RTD run and includes indicative LMPs at Resource Nodes, Hub LMPs and Load Zone for each interval in the Look Ahead SCED-RTD Study Period.

- `ercot-pp-cli np6-970-cd` — RTD Indicative LMPs by Resource Nodes, Load Zones and Hubs


### Finding the right command

When you know what you want to do but not which command does it, ask the CLI directly:

```bash
ercot-pp-cli which "<capability in your own words>"
```

`which` resolves a natural-language capability query to the best matching command from this CLI's curated feature index. Exit code `0` means at least one match; exit code `2` means no confident match — fall back to `--help` or use a narrower query.

## Auth Setup

Run `ercot-pp-cli auth setup` for the URL and steps to obtain a token (add `--launch` to open the URL). Then store it:

```bash
ercot-pp-cli auth set-token YOUR_TOKEN_HERE
```

Or set `ERCOT_OAUTH2_ROPC` as an environment variable.

Run `ercot-pp-cli doctor` to verify setup.

## Agent Mode

Add `--agent` to any command. Expands to: `--json --compact --no-input --no-color --yes`.

- **Pipeable** — JSON on stdout, errors on stderr
- **Filterable** — `--select` keeps a subset of fields. Dotted paths descend into nested structures; arrays traverse element-wise. Critical for keeping context small on verbose APIs:

  ```bash
  ercot-pp-cli ercot-public-client-version --agent --select id,name,status
  ```
- **Previewable** — `--dry-run` shows the request without sending
- **Offline-friendly** — sync/search commands can use the local SQLite store when available
- **Non-interactive** — never prompts, every input is a flag
- **Read-only** — do not use this CLI for create, update, delete, publish, comment, upvote, invite, order, send, or other mutating requests

### Response envelope

Commands that read from the local store or the API wrap output in a provenance envelope:

```json
{
  "meta": {"source": "live" | "local", "synced_at": "...", "reason": "..."},
  "results": <data>
}
```

Parse `.results` for data and `.meta.source` to know whether it's live or local. A human-readable `N results (live)` summary is printed to stderr only when stdout is a terminal AND no machine-format flag (`--json`, `--csv`, `--compact`, `--quiet`, `--plain`, `--select`) is set — piped/agent consumers and explicit-format runs get pure JSON on stdout.

## Agent Feedback

When you (or the agent) notice something off about this CLI, record it:

```
ercot-pp-cli feedback "the --since flag is inclusive but docs say exclusive"
ercot-pp-cli feedback --stdin < notes.txt
ercot-pp-cli feedback list --json --limit 10
```

Entries are stored locally at `~/.ercot-pp-cli/feedback.jsonl`. They are never POSTed unless `ERCOT_FEEDBACK_ENDPOINT` is set AND either `--send` is passed or `ERCOT_FEEDBACK_AUTO_SEND=true`. Default behavior is local-only.

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
ercot-pp-cli profile save briefing --json
ercot-pp-cli --profile briefing ercot-public-client-version
ercot-pp-cli profile list --json
ercot-pp-cli profile show briefing
ercot-pp-cli profile delete briefing --yes
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

1. **Empty, `help`, or `--help`** → show `ercot-pp-cli --help` output
2. **Starts with `install`** → ends with `mcp` → MCP installation; otherwise → see Prerequisites above
3. **Anything else** → Direct Use (execute as CLI command with `--agent`)

## MCP Server Installation

Install the MCP binary from this CLI's published public-library entry or pre-built release, then register it:

```bash
claude mcp add ercot-pp-mcp -- ercot-pp-mcp
```

Verify: `claude mcp list`

## Direct Use

1. Check if installed: `which ercot-pp-cli`
   If not found, offer to install (see Prerequisites at the top of this skill).
2. Match the user query to the best command from the Unique Capabilities and Command Reference above.
3. Execute with the `--agent` flag:
   ```bash
   ercot-pp-cli <command> [subcommand] [args] --agent
   ```
4. If ambiguous, drill into subcommand help: `ercot-pp-cli <command> --help`.
