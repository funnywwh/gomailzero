-- +goose Up
-- +goose StatementBegin
-- 添加 is_admin 字段到 users 表

ALTER TABLE users ADD COLUMN is_admin INTEGER DEFAULT 0;

-- 将第一个用户（初始化时创建的用户）设置为管理员
UPDATE users SET is_admin = 1 WHERE id = (SELECT MIN(id) FROM users);

-- +goose StatementEnd

