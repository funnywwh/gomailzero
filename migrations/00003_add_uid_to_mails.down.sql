-- +goose Down
-- +goose StatementBegin
-- 移除 uid 字段

DROP INDEX IF EXISTS idx_mails_uid;
ALTER TABLE mails DROP COLUMN uid;

-- +goose StatementEnd

