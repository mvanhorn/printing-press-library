# Research Notes

LawHub exposes authenticated browser application flows for library/history/score-report review. This CLI uses a saved browser session and stores only user-owned performance metadata.

Useful current endpoints/routes:

- Library: `https://app.lawhub.org/library/fulltests`
- History endpoint shape: `/api/request/v2/api/user/<user-id>/history/<module-id>?PageNumber=1&SortOrder=desc&SortField=startDate&PageSize=25`
- Score report route: `https://app.lawhub.org/scoreReport/<testInstanceId>`
- Official review route: `https://app.lawhub.org/question/<testInstanceId>/Section%20<N>?question=<Q>`

The score-report table currently provides correctness, chosen/correct answer letters, type, difficulty, timing, and flag state without storing official question content.
