# spaces apply live lifecycle (2026-11-05)

## apply
True {'create': 1, 'noop': 1, 'update': 0} [('space_group', 'noop', 'Spaces', 1999918), ('space', 'create', 'pp-test-apply-space', 1001059)]
## rerun (expect noop)
{'create': 0, 'noop': 2, 'update': 0} []
## change one field (is_post_disabled)
## rerun after update (expect noop)
{'create': 0, 'noop': 1, 'update': 1}
## change one field after id fix (is_post_disabled: true)
True {'create': 0, 'noop': 1, 'update': 1} [('pp-test-apply-space', 'update', {'is_post_disabled': {'from': None, 'to': True}})]
## rerun after update (expect noop)
{'create': 0, 'noop': 2, 'update': 0}
## read-back
{
  "meta": {
    "source": "live"
  },
  "results": {
    "community_id": "redacted",
    "cover_image_display_style": "normal",
    "cover_image_url": null,
    "cover_image_visible": false,
    "custom_emoji_dark_url": null,
    "custom_emoji_url": null,
    "default_comment_sort": null,
    "default_in_app_notification_setting": "never",
    "default_member_sort": null,
    "default_mention_in_app_notification_setting": "never",
    "default_mention_mobile_notification_setting": "never",
    "default_mobile_notification_setting": "never",
    "default_notification_setting": "never",
    "default_sort": null,
    "default_tab": "posts",
    "disable_member_post_covers": null,
    "display_view": "posts",
    "emoji": "#",
    "event_auto_rsvp_enabled": false,
    "hide_from_featured_areas": false,
    "hide_from_sidebar": false,
    "hide_members_count": null,
    "hide_post_settings": null,
    "hide_right_sidebar": false,
    "hide_sorting": null,
    "host": null,
    "id": 1001059,
    "is_draft": false,
    "is_hidden": true,
    "is_hidden_from_non_members": true,
    "is_post_disabled": true,
    "is_private": true,
    "lock_screen_blocks": null,
    "locked_button_label": null,
    "locked_button_url": null,
    "locked_page_description": null,
    "locked_page_heading": null,
    "name": "pp-test-apply-space",
    "pinned_posts_label": null,
    "post_ids": [],
    "prevent_members_from_adding_others": true,
    "require_topic_selection": false,
    "show_lock_icon_for_non_members": true,
    "show_next_event": true,
    "show_tab_bar": true,
    "slug": "pp-test-apply-space",
    "space_group": {
      "id": 1999918,
      "name": "Spaces"
    },
    "space_type": "basic",
    "thumbnail_image_url": null,
    "topics": [],
    "url": "https://example.circle.so/c/pp-test-apply-space",
    "visible_tabs": {
      "posts": true,
      "members": true
    }
  }
}
{'id': 1001059, 'name': 'pp-test-apply-space', 'is_post_disabled': True, 'is_private': True, 'is_hidden': True, 'is_hidden_from_non_members': True}
