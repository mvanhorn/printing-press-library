# onboard live lifecycle (2026-11-05)
## preview
{
  "dry_run": false,
  "applied": false,
  "people": [
    {
      "email": "test-member@example.com",
      "action": "create",
      "spaces": [],
      "space_groups": [],
      "access_groups": [],
      "tags": [
        "pp-test-founding"
      ],
      "send_invitation": false
    }
  ],
  "tags_to_create": [
    "pp-test-founding"
  ]
}
## apply (--yes, skip_invitation true)
{
  "dry_run": false,
  "applied": true,
  "people": [
    {
      "email": "test-member@example.com",
      "action": "create",
      "spaces": [],
      "space_groups": [],
      "access_groups": [],
      "tags": [
        "pp-test-founding"
      ],
      "send_invitation": false
    }
  ],
  "tags_to_create": [
    "pp-test-founding"
  ],
  "failures": [
    {
      "email": "test-member@example.com",
      "step": "create tag pp-test-founding",
      "error": "POST /member_tags returned HTTP 422: {\"success\":false,\"message\":\"Unable to save member tag\",\"error_details\":{\"display_format\":[\"can't be blank\",\"must be one of icon, label\"]}}"
    }
  ]
}
Error: onboarding failed for one or more steps; see failures
## rerun (expect update-memberships)
{
  "dry_run": false,
  "applied": false,
  "people": [
    {
      "email": "test-member@example.com",
      "action": "create",
      "spaces": [],
      "space_groups": [],
      "access_groups": [],
      "tags": [],
      "send_invitation": false
    }
  ],
  "tags_to_create": []
}
## read-back
warning: 1/1 member-tags items returned but not cached locally (no extractable ID field; offline lookup against these rows will be incomplete; live queries unaffected)
{
  "meta": {
    "source": "live"
  },
  "results": {
    "count": 0,
    "has_next_page": false,
    "page": 1,
    "page_count": 0,
    "per_page": 10,
    "records": []
  }
}
## apply after display_format fix
{
  "dry_run": false,
  "applied": true,
  "people": [
    {
      "email": "test-member@example.com",
      "action": "create",
      "spaces": [],
      "space_groups": [],
      "access_groups": [],
      "tags": [
        "pp-test-founding"
      ],
      "send_invitation": false
    }
  ],
  "tags_to_create": [
    "pp-test-founding"
  ]
}
## rerun (expect update-memberships)
['create'] []
## read-back
{'id': None, 'email': None, 'accepted_invitation': None, 'member_tags': None}
[(400000, 'pp-test-founding')]
## rerun after status=all fix (expect update-memberships, no tags to create)
{
  "dry_run": false,
  "applied": false,
  "people": [
    {
      "email": "test-member@example.com",
      "action": "update-memberships",
      "spaces": [],
      "space_groups": [],
      "access_groups": [],
      "tags": [
        "pp-test-founding"
      ],
      "send_invitation": false
    }
  ],
  "tags_to_create": []
}
## apply existing member to pp-test-showtell space
{
  "dry_run": false,
  "applied": true,
  "people": [
    {
      "email": "test-member@example.com",
      "action": "update-memberships",
      "spaces": [
        1001070
      ],
      "space_groups": [],
      "access_groups": [],
      "tags": [],
      "send_invitation": false
    }
  ],
  "tags_to_create": []
}
{"id":1000000338,"user_id":70000000,"space_id":1001070,"notification_type":"never","posts_read_count":2,"in_app_notification_setting":"never","community_member_id":100000037,"moderator":false,"mobile_