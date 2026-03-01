-- 为图片表添加缩略图尺寸字段
ALTER TABLE images ADD COLUMN thumbnail_width INTEGER;
ALTER TABLE images ADD COLUMN thumbnail_height INTEGER;
