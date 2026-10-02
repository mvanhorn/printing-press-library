# Public source fixtures

These deterministic parser inputs were captured from Yamato first-party public service pages and date calculators on 2026-10-02. They contain no request headers, cookie jars, credentials or private shipment/account data. Quotes use public Tokyo/Kyoto postal examples and Narita airport identifiers.

- Standard policy: https://www.kuronekoyamato.co.jp/ytc/en/send/services/takkyubin/
- Airport policy: https://www.kuronekoyamato.co.jp/ytc/en/send/services/airport/
- Roundtrip policy: https://www.kuronekoyamato.co.jp/ytc/en/send/services/bothways/
- Same-day source: https://www.kuronekoyamato.co.jp/ytc/en/send/services/same-day-delivery/
- Calculator fixtures use https://date.kuronekoyamato.co.jp/date/ (TakkyubinSmp, LeisureTakkyubinSmp and KuukouTakkyubinSmp). These snapshots are test inputs; runtime quotes always read current upstream responses.

The full same-day PDF is used only to verify fingerprint guarding; structured runtime transcriptions remain restricted to the nine approved Narita/Haneda airport-to-hotel rows.

Same-day PDF source: https://www.kuronekoyamato.co.jp/ytc/en/send/services/baggage-branch-list/pdf/same-day_delivery.pdf
