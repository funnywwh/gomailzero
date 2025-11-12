-- +goose Down
-- +goose StatementBegin
-- 移除 is_admin 字段

-- SQLite 不支持直接删除列，需要重建表
CREATE TABLE users_new (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	email TEXT UNIQUE NOT NULL,
	password_hash TEXT NOT NULL,
	quota INTEGER DEFAULT 0,
	active INTEGER DEFAULT 1,
	created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO users_new (id, email, password_hash, quota, active, created_at, updated_at)
SELECT id, email, password_hash, quota, active, created_at, updated_at
FROM users;

DROP TABLE users;
ALTER TABLE users_new RENAME TO users;

-- +goose StatementEnd

