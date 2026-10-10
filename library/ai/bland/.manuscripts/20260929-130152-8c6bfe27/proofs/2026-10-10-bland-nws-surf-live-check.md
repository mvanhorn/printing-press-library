# Bland CLI live call to the NWS Oʻahu surf forecast

On 2026-10-10, Greg authorized a task call to a recorded surf report. The [National Weather Service Honolulu automated phone recordings](https://www.weather.gov/hfo/office_products) list `+1 808-973-4380` and identify menu option 3 as the Oʻahu Surf Forecast. This call reached the recording; no NWS staff member was contacted.

The CLI invoked `bland-pp-cli calls task run --phone-number +18089734380 --task <select option 3 and report the recorded Oʻahu surf forecast> --agent`. Bland emitted keypad digit 3, received the forecast, and the CLI polled the call to completion. A subsequent read-only `calls get` returned the transcript and generated summary. `calls search-task "Oahu Surf Forecast" --agent` returned one local match: this call.

| Field | Observed value |
| --- | --- |
| Call ID | `c59ee20b-ef6e-4e05-b874-64e08d47b981` |
| Created | `2026-10-10T19:31:29.794Z` |
| Started | `2026-10-10T19:31:33.000Z` |
| Last updated | `2026-10-10T19:33:49.166Z` |
| API status / completed flag | `completed` / `true` |
| API call length | `2.08333333333333` |
| API ended by | `ASSISTANT` |
| API transcript turns | 49 |
| Local task-search match | 1, with the same call ID |

Selected transcript lines from `calls get`:

```text
user (NWS recording): Thank you for calling the National Weather Service.
agent-action: Pressed Button: 3
user (NWS recording): No SURF related advisories or warnings at this time.
user (NWS recording): Issued at eight fourteen AM.
user (NWS recording): The SURF forecast for Oahu.
user (NWS recording): North facing shores five to seven feet this morning. Then four to six feet this afternoon.
user (NWS recording): West facing shores, four to six feet this morning. Then three to five feet this afternoon.
user (NWS recording): South facing shores, five to seven feet this morning. Then four to six feet this afternoon.
user (NWS recording): East Facing Shores three to five feet today.
user (NWS recording): Two to four feet Sunday.
user (NWS recording): Oahu surf observations. No current reports available.
```

Bland's generated post-call summary reports north shores 5–7 feet in the morning and 4–6 in the afternoon; west shores 4–6 then 3–5; south shores 5–7 then 4–6; east shores 3–5 today and 2–4 Sunday. It also notes that no current surf observations were available. These values match the selected recording transcript. The agent pressed digit 3 several times while repeated advisory messages played before it reached the forecast.

This real call demonstrates outbound creation, automated-menu navigation, status polling, transcript and summary retrieval, and local task recall. It does not change the Printing Press `coverage_hollow` marker: that runner still adds `--dry-run` to mutating commands. The public proof includes only the NWS published number and this call's records; no API credential or personal call data is included.
