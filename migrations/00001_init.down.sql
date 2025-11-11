-- 回滚初始数据库结构

DROP INDEX IF EXISTS idx_aliases_domain;
DROP INDEX IF EXISTS idx_aliases_from;
DROP INDEX IF EXISTS idx_mails_received_at;
DROP INDEX IF EXISTS idx_mails_user_folder;

DROP TABLE IF EXISTS totp_secrets;
DROP TABLE IF EXISTS mails;
DROP TABLE IF EXISTS aliases;
DROP TABLE IF EXISTS domains;
DROP TABLE IF EXISTS users;

