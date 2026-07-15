# AirHint CLI Brief

## API Identity
- Domain: Flight price prediction & timing — "buy now or wait" recommendations for airline tickets
- Users: Budget travelers (especially on low-cost carriers), travel agencies, OTAs, price-alert-seekers
- Data profile: Routes (origin/destination), dates, airline, cabin class → prediction (Buy/Wait), confidence score, price-drop probability %, price range, textual explanation

## Reachability Risk
- **Medium** — The B2B API is private (email-access only, no public docs). The consumer website has undocumented API calls that could be browser-sniffed.
- No npm/PyPI wrappers exist. No community-reverse-engineered endpoints found.
- `api.airhint.com` returns "OK" with no docs.
- The consumer site at `airhint.com/ryanair` etc. is JS-rendered with XHR calls to an undiscovered backend.
- No GitHub issues reporting 403s or "broken" (no public repos targeting this API).

## Top Workflows
1. **Flight price prediction**: Enter route + dates → get Buy/Wait recommendation with confidence score
2. **Price alert tracking**: Set up alerts for a specific route/date, get notified when price drops
3. **Multi-date price comparison**: Compare prices across departure dates to find the cheapest window
4. **Airline-specific predictions**: 40+ airlines supported with airline-specific ML models
5. **Trip planning**: Check predictions for multiple routes as part of planning a trip

## Table Stakes (from competitors)
- Google Flights Price Insights: "Low / Typical / High" + price graph
- KAYAK Price Forecast: Buy/Wait with ~85% accuracy
- Hopper: Price prediction with calendar view, push notifications
- Skyscanner: Price alerts, calendar fare view
- Flyr: AI-driven airline revenue intelligence
- Going (formerly Scott's Cheap Flights): Deal alerts

## Data Layer
- Primary entities: predictions (route+date+airline → result), alerts (user-defined route/date watch), price_history (local SQLite log of past predictions for trend analysis)
- Sync cursor: date-based (predictions are point-in-time; alerts need polling)
- FTS/search: Route search by IATA code or city name

## Codebase Intelligence
- No public GitHub repos with AirHint API source code
- iOS client: `akuzminskyi/AirHint-Predictor` (packages only, no discoverable source)
- Mobile app: com.airhint.app (Android), id6754238703 (iOS)
- API endpoint patterns (inferred from business page + consumer UI):
  - Likely: GET /predict?origin=LHR&destination=BCN&date=2026-08-15&airline=ryanair
  - Response shape: { recommendation: "buy"|"wait", confidence: 0-1, price_drop_probability: 0-1, price_range: {min, max}, insights: [string] }

## Product Thesis
- Name: airhint-pp-cli
- Why it should exist: AirHint has no public CLI or API for developers/agents. Power users running multiple flight searches, automating travel booking decisions, or integrating predictions into their workflows have no scriptable interface. A CLI enables batch predictions, local caching of price history, scheduled alerts via cron, and agent-native structured output.

## Build Priorities
1. **Predict command**: core Buy/Wait prediction for a route+date (requires browser-sniff to discover endpoint)
2. **Alert management**: Set up and track price alerts locally (SQLite-backed)
3. **Multi-date sweep**: Predict across a date range for a route (transcendence)
4. **Price history**: Cache predictions over time to spot long-term trends (transcendence)
5. **Route comparison**: Compare predictions across multiple routes simultaneously (transcendence)
