package store

const schema = `
CREATE TABLE IF NOT EXISTS snapshots (
  id TEXT PRIMARY KEY,
  created_at TEXT NOT NULL,
  source TEXT NOT NULL,
  sections_json TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS packets (
  id TEXT PRIMARY KEY,
  snapshot_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  opportunity TEXT NOT NULL,
  opportunity_id TEXT NOT NULL,
  status TEXT NOT NULL,
  risk_notes_json TEXT NOT NULL,
  changes_json TEXT NOT NULL,
  FOREIGN KEY(snapshot_id) REFERENCES snapshots(id)
);

CREATE TABLE IF NOT EXISTS apply_logs (
  id TEXT PRIMARY KEY,
  packet_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  privacy_status TEXT NOT NULL,
  sensitive_status TEXT NOT NULL,
  result TEXT NOT NULL,
  dry_run INTEGER NOT NULL,
  confirmation_source TEXT NOT NULL,
  FOREIGN KEY(packet_id) REFERENCES packets(id)
);

CREATE TABLE IF NOT EXISTS message_drafts (
  id TEXT PRIMARY KEY,
  created_at TEXT NOT NULL,
  recipient TEXT NOT NULL,
  body TEXT NOT NULL,
  status TEXT NOT NULL,
  source_note TEXT NOT NULL,
  sent_at TEXT,
  send_mode TEXT NOT NULL,
  confirm_text TEXT NOT NULL
);
`
