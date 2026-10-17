package store

// schema 建表与建索引语句（幂等，按顺序执行）
var schema = []string{
	// users：账户
	`CREATE TABLE IF NOT EXISTS users (
			id              TEXT PRIMARY KEY,
			nickname        TEXT NOT NULL,
			avatar          TEXT NOT NULL DEFAULT '',
			avatar_preset   INTEGER NOT NULL DEFAULT 0,
			gender          INTEGER NOT NULL DEFAULT 0,
			age             INTEGER NOT NULL DEFAULT 0,
			password_hash   TEXT NOT NULL,
			password_salt   TEXT NOT NULL,
			created_at      INTEGER NOT NULL,
			last_login_at   INTEGER NOT NULL,
			token_version   INTEGER NOT NULL DEFAULT 0
		)`,
	`CREATE INDEX IF NOT EXISTS idx_users_last_login ON users(last_login_at)`,

	// bottles：瓶子
	`CREATE TABLE IF NOT EXISTS bottles (
		id              INTEGER PRIMARY KEY AUTOINCREMENT,
		owner_id        TEXT NOT NULL REFERENCES users(id),
		holder_id       TEXT REFERENCES users(id),
		content         TEXT NOT NULL,
		target_gender   INTEGER NOT NULL DEFAULT 0,
		target_age_min  INTEGER NOT NULL,
		target_age_max  INTEGER NOT NULL,
		state           INTEGER NOT NULL,
		created_at      INTEGER NOT NULL,
		last_message_at INTEGER
	)`,
	`CREATE INDEX IF NOT EXISTS idx_bottles_state ON bottles(state)`,
	`CREATE INDEX IF NOT EXISTS idx_bottles_owner ON bottles(owner_id)`,
	`CREATE INDEX IF NOT EXISTS idx_bottles_holder ON bottles(holder_id)`,

	// messages：会话消息
	`CREATE TABLE IF NOT EXISTS messages (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		bottle_id  INTEGER NOT NULL REFERENCES bottles(id) ON DELETE CASCADE,
		sender_id  TEXT NOT NULL,
		content    TEXT NOT NULL,
		created_at INTEGER NOT NULL,
		expire_at  INTEGER NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_messages_bottle ON messages(bottle_id, created_at)`,
	`CREATE INDEX IF NOT EXISTS idx_messages_expire ON messages(expire_at)`,

	// attachments：投放内容/聊天消息的媒体附件（每条内容或消息至多一个）
	// bottle_id 与 message_id 同时为 NULL 表示"已上传待绑定"
	`CREATE TABLE IF NOT EXISTS attachments (
		id         INTEGER PRIMARY KEY AUTOINCREMENT,
		owner_id   TEXT NOT NULL,
		bottle_id  INTEGER,
		message_id INTEGER,
		kind       TEXT NOT NULL,
		mime       TEXT NOT NULL,
		size       INTEGER NOT NULL,
		orig_name  TEXT NOT NULL,
		path       TEXT NOT NULL,
		created_at INTEGER NOT NULL
	)`,
	`CREATE INDEX IF NOT EXISTS idx_attachments_bottle ON attachments(bottle_id)`,
	`CREATE INDEX IF NOT EXISTS idx_attachments_message ON attachments(message_id)`,
	`CREATE INDEX IF NOT EXISTS idx_attachments_owner ON attachments(owner_id, created_at)`,

	// bottle_reads：每个用户对每个瓶子的最近已读时间
	`CREATE TABLE IF NOT EXISTS bottle_reads (
		bottle_id    INTEGER NOT NULL REFERENCES bottles(id) ON DELETE CASCADE,
		user_id      TEXT NOT NULL,
		last_read_at INTEGER NOT NULL,
		PRIMARY KEY (bottle_id, user_id)
	)`,

	// daily_usage：每日配额用量
	`CREATE TABLE IF NOT EXISTS daily_usage (
		user_id     TEXT NOT NULL,
		usage_date  TEXT NOT NULL,
		throw_count INTEGER NOT NULL DEFAULT 0,
		pick_count  INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (user_id, usage_date)
	)`,
}
