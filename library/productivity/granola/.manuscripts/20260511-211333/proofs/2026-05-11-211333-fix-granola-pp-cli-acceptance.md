{
  "schema_version": 1,
  "api_name": "granola",
  "run_id": "20260511-211333",
  "status": "fail",
  "level": "full",
  "matrix_size": 78,
  "tests_passed": 36,
  "tests_failed": 42,
  "auth_context": {
    "type": "none",
    "api_key_available": false,
    "browser_session_available": false
  },
  "failure_summary": {
    "missing_examples_section": 36,
    "auth_exit_4": 4,
    "missing_runnable_example": 1,
    "invalid_json": 1
  },
  "note": "Behavioral assertions PASSED (sync, meetings list, talktime --by participant, extract three streams all verified against real cache). Failures are structural: most commands lack an Examples: block in their Cobra help. Polish (Phase 5.5) will batch-fix these."
}
