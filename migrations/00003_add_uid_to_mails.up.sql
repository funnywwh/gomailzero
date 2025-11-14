-- +goose Up
-- +goose StatementBegin
-- 添加 uid 字段到 mails 表（IMAP UID 支持）

-- 添加 uid 列（允许 NULL，因为现有邮件没有 UID）
ALTER TABLE mails ADD COLUMN uid INTEGER;

-- 为现有邮件分配 UID（按创建时间排序，从 1 开始）
-- 注意：这只是一个临时方案，理想情况下应该为每个邮箱独立分配 UID
UPDATE mails 
SET uid = (
    SELECT COUNT(*) + 1 
    FROM mails m2 
    WHERE m2.user_email = mails.user_email 
      AND m2.folder = mails.folder 
      AND (m2.created_at < mails.created_at OR (m2.created_at = mails.created_at AND m2.id < mails.id))
);

-- 将 uid 设置为 NOT NULL（新邮件必须有 UID）
-- 注意：SQLite 不支持直接修改列约束，需要重建表
-- 这里先保持允许 NULL，在应用层确保新邮件有 UID

-- 创建索引以提高查询性能
CREATE INDEX IF NOT EXISTS idx_mails_uid ON mails(user_email, folder, uid);

-- +goose StatementEnd

