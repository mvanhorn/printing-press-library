# show-tell live lifecycle (2026-11-05), hidden event space 1001070, draft template 30000788
## schedule preview
{
  "applied": false,
  "template_id": "30000788",
  "every": "2w",
  "until": "2026-12-31",
  "occurrences": [
    {
      "date": "2026-12-03",
      "starts_at": "2026-12-03T18:00:00Z",
      "action": "create",
      "event_id": 0
    },
    {
      "date": "2026-12-17",
      "starts_at": "2026-12-17T18:00:00Z",
      "action": "create",
      "event_id": 0
    },
    {
      "date": "2026-12-31",
      "starts_at": "2026-12-31T18:00:00Z",
      "action": "create",
      "event_id": 0
    }
  ]
}
## schedule --yes (until 2026-12-17: 2 occurrences)
## schedule --yes after space_id fix (until 2026-12-17)
{
  "applied": true,
  "template_id": "30000788",
  "every": "2w",
  "until": "2026-12-17",
  "occurrences": [
    {
      "date": "2026-12-03",
      "starts_at": "2026-12-03T18:00:00Z",
      "action": "create",
      "event_id": 30000897
    },
    {
      "date": "2026-12-17",
      "starts_at": "2026-12-17T18:00:00Z",
      "action": "create",
      "event_id": 30000898
    }
  ]
}
## read-back occurrences
{'id': 30000897, 'name': 'pp-test Show and Tell', 'starts_at': '2026-12-03T18:00:00.000Z', 'ends_at': '2026-12-03T19:00:00.000Z', 'send_email_reminder': False, 'send_email_confirmation': False, 'send_in_app_notification_reminder': False}
{'id': 30000898, 'name': 'pp-test Show and Tell', 'starts_at': '2026-12-17T18:00:00.000Z', 'ends_at': '2026-12-17T19:00:00.000Z', 'send_email_reminder': False, 'send_email_confirmation': False, 'send_in_app_notification_reminder': False}
## rerun schedule preview (expect exists)
{
  "applied": false,
  "template_id": "30000788",
  "every": "2w",
  "until": "2026-12-17",
  "occurrences": [
    {
      "date": "2026-12-03",
      "starts_at": "2026-12-03T18:00:00Z",
      "action": "create",
      "event_id": 0
    },
    {
      "date": "2026-12-17",
      "starts_at": "2026-12-17T18:00:00Z",
      "action": "create",
      "event_id": 0
    }
  ]
}
## recap (no live rooms in community)
{
  "event": {
    "id": 30000788,
    "name": "pp-test Show and Tell",
    "starts_at": "2026-11-19T18:00:00Z"
  },
  "attendees": [
    {
      "id": 900000000,
      "member_name": "Example Member"
    }
  ],
  "room_id": null,
  "transcript_found": false,
  "markdown": "# pp-test Show and Tell recap\n\nDate: 2026-11-19T18:00:00Z\n\nDuration: 1h0m0s\n\nAttendees: 1\n- Example Member\n\n## Transcript\n\nNo live room recording found\n",
  "post": null
}
## recap --post preview
{
  "event": {
    "id": 30000788,
    "name": "pp-test Show and Tell",
    "starts_at": "2026-11-19T18:00:00Z"
  },
  "attendees": [
    {
      "id": 900000000,
      "member_name": "Example Member"
    }
  ],
  "room_id": null,
  "transcript_found": false,
  "markdown": "# pp-test Show and Tell recap\n\nDate: 2026-11-19T18:00:00Z\n\nDuration: 1h0m0s\n\nAttendees: 1\n- Example Member\n\n## Transcript\n\nNo live room recording found\n",
  "post": {
    "applied": false,
    "id": 0,
    "space_id": 1001059,
    "name": "pp-test Show and Tell recap",
    "status": "draft",
    "skip_notifications": true
  }
}
## recap --post --yes (draft, skip_notifications, hidden basic test space)
Error: draft post response has no valid ID; inspect posts before retrying: expected a positive numeric ID
## seeded ledger with the 2 pre-ledger occurrences; preview until 2027-01-14
{
  "applied": false,
  "template_id": "30000788",
  "every": "2w",
  "until": "2027-01-14",
  "occurrences": [
    {
      "date": "2026-12-03",
      "starts_at": "2026-12-03T18:00:00Z",
      "action": "exists",
      "event_id": 30000897
    },
    {
      "date": "2026-12-17",
      "starts_at": "2026-12-17T18:00:00Z",
      "action": "exists",
      "event_id": 30000898
    },
    {
      "date": "2026-12-31",
      "starts_at": "2026-12-31T18:00:00Z",
      "action": "create",
      "event_id": 0
    },
    {
      "date": "2027-01-14",
      "starts_at": "2027-01-14T18:00:00Z",
      "action": "create",
      "event_id": 0
    }
  ]
}
## --yes until 2027-01-14
{
  "applied": true,
  "template_id": "30000788",
  "every": "2w",
  "until": "2027-01-14",
  "occurrences": [
    {
      "date": "2026-12-03",
      "starts_at": "2026-12-03T18:00:00Z",
      "action": "exists",
      "event_id": 30000897
    },
    {
      "date": "2026-12-17",
      "starts_at": "2026-12-17T18:00:00Z",
      "action": "exists",
      "event_id": 30000898
    },
    {
      "date": "2026-12-31",
      "starts_at": "2026-12-31T18:00:00Z",
      "action": "create",
      "event_id": 30001066
    },
    {
      "date": "2027-01-14",
      "starts_at": "2027-01-14T18:00:00Z",
      "action": "create",
      "event_id": 30001068
    }
  ]
}
## rerun (expect all exists)
{
  "applied": false,
  "template_id": "30000788",
  "every": "2w",
  "until": "2027-01-14",
  "occurrences": [
    {
      "date": "2026-12-03",
      "starts_at": "2026-12-03T18:00:00Z",
      "action": "exists",
      "event_id": 30000897
    },
    {
      "date": "2026-12-17",
      "starts_at": "2026-12-17T18:00:00Z",
      "action": "exists",
      "event_id": 30000898
    },
    {
      "date": "2026-12-31",
      "starts_at": "2026-12-31T18:00:00Z",
      "action": "exists",
      "event_id": 30001066
    },
    {
      "date": "2027-01-14",
      "starts_at": "2027-01-14T18:00:00Z",
      "action": "exists",
      "event_id": 30001068
    }
  ]
}
## recap --post --yes after post-id fix
{
  "event": {
    "id": 30000788,
    "name": "pp-test Show and Tell",
    "starts_at": "2026-11-19T18:00:00Z"
  },
  "attendees": [
    {
      "id": 900000000,
      "member_name": "Example Member"
    }
  ],
  "room_id": null,
  "transcript_found": false,
  "markdown": "# pp-test Show and Tell recap\n\nDate: 2026-11-19T18:00:00Z\n\nDuration: 1h0m0s\n\nAttendees: 1\n- Example Member\n\n## Transcript\n\nNo live room recording found\n",
  "post": {
    "applied": true,
    "id": 30001602,
    "space_id": 1001059,
    "name": "pp-test Show and Tell recap",
    "status": "draft",
    "skip_notifications": true
  }
}
