# Bland CLI live call to NIST WWVH

On 2026-10-10, Greg authorized a real task call to obtain the time from a public telephone service in Hawaiʻi. [NIST lists WWVH's telephone time service](https://www.nist.gov/pml/time-and-frequency-division/time-distribution/radio-station-wwv/telephone-time-day-service) and its alternate Kauaʻi numbers. The first call to its primary number reached voicemail and yielded no time. The second call, to `+1 808-335-4744`, reached the broadcast. No other calls were made for this check.

The second call used the CLI's novel path, `bland-pp-cli calls task run --phone-number +18083354744 --task <listen for a complete WWVH UTC announcement, convert to HST, then end> --agent`. The CLI submitted one call and polled it to a terminal state. A subsequent read-only `calls get` returned the completed record, and `calls search-task WWVH --agent` found the same call in local task history.

| Field | Observed value |
| --- | --- |
| Call ID | `71df5b8e-7e60-4c18-8db2-ade1c24cfa8d` |
| Created | `2026-10-10T19:26:40.253Z` |
| Started | `2026-10-10T19:26:49.000Z` |
| Last updated | `2026-10-10T19:28:14.258Z` |
| API status | `completed` |
| API completed flag | `true` |
| API ended by | `ASSISTANT` |
| API transcript turns | 7 |
| Local `search-task WWVH` | 2 matches, including this call |

Relevant transcript turns, copied from `calls get`:

```text
assistant: I am calling the automated NIST WWVH telephone time-of-day service in Kauai, Hawaii to get the UTC time. I will stay on the line silently and listen for the complete announcement.
agent-action: [Waiting]
agent-action: [Waiting]
agent-action: [Waiting]
user (WWVH recording): At the tone, nineteen hours twenty eight minutes, coordinated universal time.
assistant: The exact announced UTC time is nineteen hours twenty eight minutes. Converting to Hawaii Standard Time by subtracting ten hours, it is zero zero hours twenty eight minutes.
agent-action: Ended call
```

The broadcast's announced time was **19:28 UTC**, which is **09:28 HST**. The Bland agent incorrectly said `00:28 HST`; Bland's generated summary repeats that incorrect conversion. This is disclosed rather than treated as a successful conversion. The call does demonstrate live creation, status polling, transcript retrieval, and local task recall. It does not clear `coverage_hollow` in the Printing Press dogfood marker because that runner dry runs mutating commands.

The source JSON, API credential, and any unrelated account call data are excluded from the public PR. The only dialed number shown here is NIST's published public time-service number.
