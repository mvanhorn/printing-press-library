# TOYOTA Rent a Car CLI Brief

## API Identity
- Domain: https://rent.toyota.co.jp/eng/; anonymous first-party ASP.NET website, no public SDK/spec found.
- Users: Japan travelers planning self-drive routes, families choosing seats and waivers, agents comparing shops and class prices before human booking.
- Data: public shop names/Japanese names/stable company:branch identifiers, dated class estimates and inventory, option/insurance and driving-document policies, independent one-way fee simulation.

## Reachability Risk
- Public GETs and anonymous cookiejar POSTs succeed with HTTP200. Native Chrome reached genuine dated C1/C2 availability and blank customer-information page without auth or challenges.
- Sandbox DNS000 is the local tool restriction; escalated stdlib HTTP works.
- Native Oct20–21 C1 lists11,990JPY and initial customer total11,990JPY, tax1,090JPY. Anonymous date replay reaches index02 with echoed dates/classes. Anonymous class selection currently returns Toyota network-error page; do not claim a complete booking quote or expose this endpoint as available.
- Source initial customer total does not update when ETC radio changes. It is not a final inclusive quote. Fees selected after the class page are separate estimates; full total stays unknown until the human confirms on Toyota.

## Top Workflows
1. Resolve a keyword to distinct pickup/dropoff shops, including opening hours, closure notes, one-way return restrictions and source IDs.
2. Search real pickup/dropoff datetimes (JST,30-minute source increments) and list selectable vs fully-booked classes with source prices, capacity and representative models.
3. Select AT/MT,4WD/winter tires and child seats as supported source options; explain ETC card/device, waiver/NOC and child-seat fees without inventing inclusions.
4. Run source one-way fee simulation for a shop pair and vehicle family; date-independent surcharge is separate from dated inventory.
5. Review first-party license/document guidance and hand off to canonical pickup shop booking page with a reusable date/options checklist.

## Table Stakes and Sources
- Toyota website is the canonical incumbent: keyword shop search, classes/representative models, tax-inclusive source estimates, insurance/ETC/child-seat options, one-way simulation.
- Nippon Rent-A-Car https://www.nipponrentacar.co.jp/en/car_price/joyo.html: class capacity, luggage and child-seat limits, GPS/ETC device info and optional waiver. Times Car Rental is another supplier; cross-supplier comparison is outside this focused CLI.
- Targeted GitHub/npm/PyPI/MCP/skills/automation searches on2026-10-02 found no credible Toyota Japan wrapper or SDK to absorb. Unrelated vehicle-rental templates and travel itineraries were not API evidence. No wrapper issue/reachability audit applies.
- Pain points: ambiguous similarly named station shops; class is not a promised model; list price does not safely imply insurance, ETC,tolls,fuel or one-way inclusion; eligibility depends on real documents.

## Data Layer
- Primary entities: Shop(company_code:branch_code), RentalPeriod(JST), ClassOffer(class_code+period+shops), SourceOption, OneWaySimulation, EligibilityGuidance.
- No persistent anonymous cookies or personal records. Session/form state stays in memory and is discarded. No speculative offline inventory cache; every live output carries fetched_at and upstream metrics.
- Local store only the Press framework's optional saved public output. Framework sync is not presented as fresh inventory.

## User Vision
Build a focused read-only Go Toyota planning CLI autonomously. Native browser first; exactly one fresh-context gpt-6.1-sol max reviewer. Routine gate choices and local promotion are already authorized. No bookings,payments,account mutations or personal data.

## Product Thesis
- Name: TOYOTA Rent a Car CLI; binary toyota-rentacar-pp-cli.
- Purpose: reproducible bounded source-grounded rental planning, explicit unknowns and a safe canonical human booking handoff.

## Build Priorities
1. Live shop search/detail with strict composite IDs and Japanese names.
2. Anonymous stateful dated class search, robust successful form controls and client-populated shop/date fields; validate echoed context before returning offers.
3. Options and insurance policies with units, tax and stock caveats; never aggregate an unknown-inclusive class estimate into a final total.
4. Source one-way simulator plus shop return restrictions; forbidden island crossings and buses remain source rules, not made-up fees.
5. Eligibility source guidance and canonical booking handoff that truthfully requires re-entering dates/options.
6. Compact JSON, field selection,dry-run,typed errors,request budgets and deterministic consequential tests + real live matrix.

## Workflow Exceptions
The explicit batch instruction limits each builder to exactly one reviewer and forbids extra coordination/ideation agents. It overrides Press phase08's extra novel-features subagent and phase14/16 extra review agents. Own those analyses directly, record results; use one fresh-context MAX reviewer for the full scope in phase17. Routine scope/manifest choices are preauthorized by the batch brief.
