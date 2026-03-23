-- 为图片表补充审计字段
ALTER TABLE images ADD COLUMN created_at DATETIME;
UPDATE images
SET created_at = COALESCE(created_at, uploaded_at, CURRENT_TIMESTAMP)
WHERE created_at IS NULL;

ALTER TABLE images ADD COLUMN deleted INTEGER NOT NULL DEFAULT 0;
UPDATE images
SET deleted = CASE WHEN deleted_at IS NULL THEN 0 ELSE 1 END
WHERE deleted_at IS NOT NULL OR deleted <> 0;

CREATE INDEX IF NOT EXISTS idx_images_deleted ON images(deleted);

-- 重建标签表，支持软删除和仅针对未删除数据的名称唯一性
CREATE TABLE tags_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    color TEXT DEFAULT '#3B82F6',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    deleted INTEGER NOT NULL DEFAULT 0
);

INSERT INTO tags_new (id, name, color, created_at, deleted_at, deleted)
SELECT id, name, color, created_at, NULL, 0
FROM tags;

DROP TABLE tags;
ALTER TABLE tags_new RENAME TO tags;

CREATE UNIQUE INDEX idx_tags_name_unique_active ON tags(name) WHERE deleted = 0;
CREATE INDEX IF NOT EXISTS idx_tags_deleted ON tags(deleted);

-- 重建图片标签关联表，补充ID和软删除字段
CREATE TABLE image_tags_new (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    image_id INTEGER NOT NULL,
    tag_id INTEGER NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    deleted_at DATETIME,
    deleted INTEGER NOT NULL DEFAULT 0
);

INSERT INTO image_tags_new (image_id, tag_id, created_at, deleted_at, deleted)
SELECT image_id, tag_id, created_at, NULL, 0
FROM image_tags;

DROP TABLE image_tags;
ALTER TABLE image_tags_new RENAME TO image_tags;

CREATE UNIQUE INDEX idx_image_tags_unique_active ON image_tags(image_id, tag_id) WHERE deleted = 0;
CREATE INDEX IF NOT EXISTS idx_image_tags_image_id ON image_tags(image_id);
CREATE INDEX IF NOT EXISTS idx_image_tags_tag_id ON image_tags(tag_id);
CREATE INDEX IF NOT EXISTS idx_image_tags_deleted ON image_tags(deleted);

-- 验证码表补充软删除字段
ALTER TABLE verification_codes ADD COLUMN deleted_at DATETIME;
ALTER TABLE verification_codes ADD COLUMN deleted INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_verification_codes_deleted ON verification_codes(deleted);

-- 同步日志表补充审计字段
ALTER TABLE sync_logs ADD COLUMN created_at DATETIME;
UPDATE sync_logs
SET created_at = COALESCE(created_at, started_at, CURRENT_TIMESTAMP)
WHERE created_at IS NULL;

ALTER TABLE sync_logs ADD COLUMN deleted_at DATETIME;
ALTER TABLE sync_logs ADD COLUMN deleted INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_sync_logs_deleted ON sync_logs(deleted);

-- 同步文件日志表补充软删除字段
ALTER TABLE sync_file_logs ADD COLUMN deleted_at DATETIME;
ALTER TABLE sync_file_logs ADD COLUMN deleted INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_sync_file_logs_deleted ON sync_file_logs(deleted);

-- 迁移记录表补充审计字段
ALTER TABLE migrations ADD COLUMN created_at DATETIME;
UPDATE migrations
SET created_at = COALESCE(created_at, executed_at, CURRENT_TIMESTAMP)
WHERE created_at IS NULL;

ALTER TABLE migrations ADD COLUMN deleted_at DATETIME;
ALTER TABLE migrations ADD COLUMN deleted INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_migrations_deleted ON migrations(deleted);
