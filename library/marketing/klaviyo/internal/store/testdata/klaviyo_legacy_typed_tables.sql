-- Structure-only statements from a store created by the published Printing Press 3.5.0 CLI.


CREATE TABLE campaign_messages (
			id TEXT PRIMARY KEY,
			campaigns_id TEXT NOT NULL,
			data JSON NOT NULL,
			synced_at DATETIME DEFAULT CURRENT_TIMESTAMP
		, "created" DATETIME, "updated" DATETIME);
CREATE INDEX idx_campaign_messages_campaigns_id ON campaign_messages(campaigns_id);

CREATE TABLE lists (
			id TEXT PRIMARY KEY,
			profiles_id TEXT NOT NULL,
			data JSON NOT NULL,
			synced_at DATETIME DEFAULT CURRENT_TIMESTAMP
		, "created" DATETIME, "name" TEXT, "opt_in_process" TEXT, "updated" DATETIME);
CREATE INDEX idx_lists_profiles_id ON lists(profiles_id);

CREATE TABLE metrics (
			id TEXT PRIMARY KEY,
			custom_metrics_id TEXT NOT NULL,
			data JSON NOT NULL,
			synced_at DATETIME DEFAULT CURRENT_TIMESTAMP
		, "created" TEXT, "name" TEXT, "updated" TEXT);
CREATE INDEX idx_metrics_custom_metrics_id ON metrics(custom_metrics_id);

CREATE TABLE profiles (
			id TEXT PRIMARY KEY,
			segments_id TEXT NOT NULL,
			data JSON NOT NULL,
			synced_at DATETIME DEFAULT CURRENT_TIMESTAMP
		, "type" TEXT);
CREATE INDEX idx_profiles_segments_id ON profiles(segments_id);

CREATE TABLE tags (
			id TEXT PRIMARY KEY,
			segments_id TEXT NOT NULL,
			data JSON NOT NULL,
			synced_at DATETIME DEFAULT CURRENT_TIMESTAMP
		, "name" TEXT);
CREATE INDEX idx_tags_segments_id ON tags(segments_id);
