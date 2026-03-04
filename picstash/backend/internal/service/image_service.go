package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"path"
	"path/filepath"
	"strings"
	"time"

	"picstash/internal/model"
	"picstash/internal/repository"
	"picstash/internal/storage"
	"picstash/pkg/imageutil"

	"github.com/google/uuid"
)

type ImageService struct {
	db         *sql.DB
	storage    storage.Storage
	pathPrefix string
}

type SyncResult struct {
	CreatedCount int
	UpdatedCount int
	DeletedCount int
	SkippedCount int
	ErrorCount   int
	ErrorMessage string
	LogID        int64
}

type SyncStartResult struct {
	LogID   int64
	Started bool
	Status  string
}

func NewImageService(db *sql.DB, storage storage.Storage, pathPrefix string) *ImageService {
	return &ImageService{
		db:         db,
		storage:    storage,
		pathPrefix: pathPrefix,
	}
}

func (s *ImageService) BatchUploadWithMapping(
	ctx context.Context,
	originalFiles map[string][]byte,
	watermarkFiles map[string][]byte,
	thumbnailFiles map[string][]byte,
	mapping []FileMapping,
	tagIDs []int,
) ([]*model.Image, error) {
	images := make([]*model.Image, 0, len(mapping))
	slog.Info("开始批量上传（带映射）", "mapping_count", len(mapping), "original_count", len(originalFiles), "watermark_count", len(watermarkFiles), "thumbnail_count", len(thumbnailFiles))

	for _, m := range mapping {
		var mainContent []byte
		var mainFilename string
		var originalName string

		if content, ok := originalFiles[m.Original]; ok {
			mainContent = content
			mainFilename = m.Original
			originalName = m.OriginalName
		} else {
			slog.Error("未找到原图", "original", m.Original)
			continue
		}

		var watermarkContent []byte
		if m.Watermark != nil {
			if content, ok := watermarkFiles[*m.Watermark]; ok && len(content) > 0 {
				watermarkContent = content
				slog.Info("找到水印图", "watermark", *m.Watermark)
			}
		}

		var thumbnailContent []byte
		if m.Thumbnail != nil {
			if thumb, ok := thumbnailFiles[*m.Thumbnail]; ok && len(thumb) > 0 {
				thumbnailContent = thumb
				slog.Info("使用传入的缩略图", "thumbnail", *m.Thumbnail)
			}
		}

		image, err := s.UploadWithContent(ctx, mainFilename, mainContent, originalName, watermarkContent, tagIDs, thumbnailContent)
		if err != nil {
			slog.Error("上传失败", "filename", mainFilename, "error", err)
			continue
		}

		images = append(images, image)
	}

	slog.Info("批量上传完成", "success_count", len(images))
	return images, nil
}

type FileMapping struct {
	Original     string  `json:"original"`
	OriginalName string  `json:"original_name"`
	Watermark    *string `json:"watermark"`
	Thumbnail    *string `json:"thumbnail"`
}

func (s *ImageService) UploadWithContent(
	ctx context.Context,
	filename string,
	content []byte,
	originalFilename string,
	watermarkContent []byte,
	tagIDs []int,
	thumbnailContent []byte,
) (*model.Image, error) {
	slog.Info("开始上传图片", "filename", filename, "size", len(content))

	now := time.Now()
	year, month, day := now.Date()
	uuidStr := strings.ReplaceAll(uuid.New().String(), "-", "")

	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".jpg"
	}
	newFilename := fmt.Sprintf("%04d/%02d/%02d/%s%s", year, month, day, uuidStr, ext)
	storagePath := path.Join(s.pathPrefix, newFilename)

	slog.Info("生成文件路径", "filename", filename, "path", storagePath)

	width, height, mimeType, err := imageutil.GetImageInfo(content)
	if err != nil {
		slog.Warn("获取图片信息失败，使用默认值", "error", err)
		width, height, mimeType = 0, 0, "image/jpeg"
	}

	slog.Info("图片信息", "width", width, "height", height, "mime_type", mimeType)

	if mimeType == "" {
		mimeType = imageutil.GetMimeType(filename)
		slog.Warn("无法检测MIME类型，使用扩展名判断", "filename", filename, "mime_type", mimeType)
	}

	uploadFile := &storage.File{
		Path:        storagePath,
		Content:     content,
		ContentType: mimeType,
	}

	slog.Info("开始上传到存储", "path", storagePath)
	uploadResult, err := s.storage.Upload(ctx, uploadFile)
	if err != nil {
		return nil, fmt.Errorf("上传图片失败: %w", err)
	}
	slog.Info("上传成功", "path", storagePath, "sha", uploadResult.SHA)

	hasWatermark := len(watermarkContent) > 0

	image := &model.Image{
		Path:             uploadResult.Path,
		URL:              uploadResult.URL,
		SHA:              uploadResult.SHA,
		OriginalFilename: originalFilename,
		Filename:         filename,
		Size:             int64Ptr(int64(len(content))),
		MimeType:         mimeType,
		Width:            intPtr(width),
		Height:           intPtr(height),
		HasThumbnail:     false,
		HasWatermark:     hasWatermark,
		UploadedAt:       time.Now(),
	}

	if hasWatermark {
		now := time.Now()
		year, month, day := now.Date()
		watermarkFilename := fmt.Sprintf("%04d/%02d/%02d/watermark_%s%s", year, month, day, uuidStr, ext)
		watermarkStoragePath := path.Join(s.pathPrefix, watermarkFilename)

		slog.Info("开始上传水印图", "path", watermarkStoragePath, "size", len(watermarkContent))
		watermarkFile := &storage.File{
			Path:        watermarkStoragePath,
			Content:     watermarkContent,
			ContentType: mimeType,
		}

		watermarkResult, watermarkUploadErr := s.storage.Upload(ctx, watermarkFile)
		if watermarkUploadErr != nil {
			slog.Warn("上传水印图失败", "error", watermarkUploadErr)
		} else {
			slog.Info("水印图上传成功", "path", watermarkStoragePath)
			image.WatermarkPath = watermarkResult.Path
			image.WatermarkURL = watermarkResult.URL
			image.WatermarkSHA = watermarkResult.SHA
			image.WatermarkSize = int64Ptr(int64(len(watermarkContent)))
		}
	}

	slog.Info("开始处理缩略图", "original_width", width, "original_height", height)

	if thumbnailContent != nil && len(thumbnailContent) > 0 {
		slog.Info("使用前端传入的缩略图", "size", len(thumbnailContent))
		thumbData := thumbnailContent
		thumbWidth, thumbHeight, _, thumbErr := imageutil.GetImageInfo(thumbData)
		if thumbErr != nil {
			slog.Warn("获取缩略图信息失败", "error", thumbErr)
		} else {
			now := time.Now()
			year, month, day := now.Date()
			thumbFilename := fmt.Sprintf("%04d/%02d/%02d/thumb_%s.jpg", year, month, day, uuidStr)
			thumbStoragePath := path.Join(s.pathPrefix, thumbFilename)

			slog.Info("开始上传缩略图", "path", thumbStoragePath, "size", len(thumbData))
			thumbFile := &storage.File{
				Path:        thumbStoragePath,
				Content:     thumbData,
				ContentType: "image/jpeg",
			}

			thumbResult, thumbUploadErr := s.storage.Upload(ctx, thumbFile)
			if thumbUploadErr != nil {
				slog.Warn("上传缩略图失败", "error", thumbUploadErr)
			} else {
				slog.Info("缩略图上传成功", "path", thumbStoragePath)
				image.ThumbnailPath = thumbResult.Path
				image.ThumbnailURL = thumbResult.URL
				image.ThumbnailSHA = thumbResult.SHA
				image.ThumbnailSize = int64Ptr(int64(len(thumbData)))
				image.ThumbnailWidth = intPtr(thumbWidth)
				image.ThumbnailHeight = intPtr(thumbHeight)
				image.HasThumbnail = true
			}
		}
	}

	slog.Info("开始保存图片记录到数据库", "filename", filename)
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	imageRepo := repository.NewImageRepository(tx)

	id, err := imageRepo.CreateImage(image)
	if err != nil {
		return nil, err
	}
	image.ID = id

	slog.Info("准备关联标签", "image_id", id, "tag_count", len(tagIDs), "tag_ids", tagIDs)
	if len(tagIDs) > 0 {
		err = imageRepo.AddImageTags(id, tagIDs)
		if err != nil {
			return nil, err
		}
		slog.Info("标签关联完成", "image_id", id, "count", len(tagIDs))
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("提交事务失败: %w", err)
	}

	slog.Info("图片上传成功", "id", id, "filename", filename, "has_thumbnail", image.HasThumbnail, "has_watermark", image.HasWatermark)

	return image, nil
}

func (s *ImageService) Delete(ctx context.Context, id int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	imageRepo := repository.NewImageRepository(tx)

	image, err := imageRepo.GetImageByID(id)
	if err != nil {
		return err
	}

	if image.Path != "" && image.SHA != "" {
		if err := s.storage.Delete(ctx, image.Path, image.SHA); err != nil {
			return fmt.Errorf("(STEP_1)删除原图失败: %w", err)
		}
	}

	if image.ThumbnailPath != "" && image.ThumbnailSHA != "" {
		if err := s.storage.Delete(ctx, image.ThumbnailPath, image.ThumbnailSHA); err != nil {
			return fmt.Errorf("(STEP_2)删除缩略图失败: %w", err)
		}
	}

	if image.WatermarkPath != "" && image.WatermarkSHA != "" {
		if err := s.storage.Delete(ctx, image.WatermarkPath, image.WatermarkSHA); err != nil {
			return fmt.Errorf("(STEP_3)删除水印图失败: %w", err)
		}
	}

	err = imageRepo.SoftDeleteImage(id)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	slog.Info("图片删除成功", "id", id, "path", image.Path, "thumbnail_path", image.ThumbnailPath, "watermark_path", image.WatermarkPath)

	return nil
}

func (s *ImageService) GetList(page, limit int, tagID *int) ([]*model.Image, int, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, 0, fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	imageRepo := repository.NewImageRepository(tx)

	images, total, err := imageRepo.GetImages(page, limit, tagID)
	if err != nil {
		return nil, 0, err
	}

	// 填充完整URL
	for _, image := range images {
		s.fillImageURLs(image)
	}

	return images, total, nil
}

func (s *ImageService) GetByID(id int64) (*model.Image, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	imageRepo := repository.NewImageRepository(tx)

	image, err := imageRepo.GetImageByID(id)
	if err != nil {
		return nil, err
	}

	// 填充完整URL
	s.fillImageURLs(image)

	return image, nil
}

func (s *ImageService) UpdateTags(id int64, tagIDs []int) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	imageRepo := repository.NewImageRepository(tx)

	err = imageRepo.UpdateImageTags(id, tagIDs)
	if err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	slog.Info("更新图片标签成功", "image_id", id, "tag_count", len(tagIDs))

	return nil
}

func intPtr(i int) *int {
	return &i
}

func int64Ptr(i int64) *int64 {
	return &i
}

// fillImageURLs 填充图片的完整URL
// 使用 storage.GetPublicURL 获取前端访问地址
func (s *ImageService) fillImageURLs(image *model.Image) {
	if image == nil {
		return
	}
	if image.Path != "" {
		image.URL = s.storage.GetPublicURL(image.Path)
	}
	if image.ThumbnailPath != "" {
		image.ThumbnailURL = s.storage.GetPublicURL(image.ThumbnailPath)
	}
	if image.WatermarkPath != "" {
		image.WatermarkURL = s.storage.GetPublicURL(image.WatermarkPath)
	}
}
