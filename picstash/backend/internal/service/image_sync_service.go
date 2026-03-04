package service

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"picstash/internal/model"
	"picstash/internal/repository"
	"picstash/internal/storage"
	"picstash/pkg/imageutil"
)

const (
	asyncSyncTimeout = 2 * time.Hour
	syncBatchSize    = 10
)

type originalBatchResult struct {
	CreatedCount int
	UpdatedCount int
	SkippedCount int
	ErrorCount   int
	NewImages    map[string]*model.Image
}

type deleteBatchResult struct {
	DeletedCount int
	ErrorCount   int
}

type ImageSyncService struct {
	db         *sql.DB
	storage    storage.Storage
	pathPrefix string
	syncMu     sync.Mutex
	runningLog int64
}

func NewImageSyncService(db *sql.DB, storage storage.Storage, pathPrefix string) *ImageSyncService {
	return &ImageSyncService{
		db:         db,
		storage:    storage,
		pathPrefix: pathPrefix,
	}
}

func (s *ImageSyncService) StartSyncFromStorageAsync(triggeredBy string) (*SyncStartResult, error) {
	s.syncMu.Lock()
	if s.runningLog != 0 {
		logID := s.runningLog
		s.syncMu.Unlock()
		return &SyncStartResult{LogID: logID, Started: false, Status: "running"}, nil
	}

	logID, err := s.createSyncLog(triggeredBy, time.Now())
	if err != nil {
		s.syncMu.Unlock()
		return nil, fmt.Errorf("创建同步日志失败: %w", err)
	}

	s.runningLog = logID
	s.syncMu.Unlock()

	go s.runSyncFromStorageJob(logID, triggeredBy)

	return &SyncStartResult{LogID: logID, Started: true, Status: "running"}, nil
}

func (s *ImageSyncService) runSyncFromStorageJob(logID int64, triggeredBy string) {
	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("同步任务异常: %v", r)
			slog.Error("异步同步任务异常", "log_id", logID, "panic", r)
			if err := s.markSyncLogFailed(logID, errMsg); err != nil {
				slog.Error("标记同步任务失败状态异常", "log_id", logID, "error", err)
			}
		}

		s.syncMu.Lock()
		if s.runningLog == logID {
			s.runningLog = 0
		}
		s.syncMu.Unlock()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), asyncSyncTimeout)
	defer cancel()

	if _, err := s.syncFromStorageWithLogID(ctx, triggeredBy, logID); err != nil {
		slog.Error("异步同步任务失败", "log_id", logID, "error", err)
	}
}

func (s *ImageSyncService) createSyncLog(triggeredBy string, startedAt time.Time) (int64, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	syncLogRepo := repository.NewSyncLogRepository(tx)
	logID, err := syncLogRepo.CreateSyncLog(triggeredBy, startedAt)
	if err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("提交事务失败: %w", err)
	}

	return logID, nil
}

func (s *ImageSyncService) markSyncLogFailed(logID int64, errMsg string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	syncLogRepo := repository.NewSyncLogRepository(tx)
	completedAt := time.Now()
	errorMessage := errMsg
	if err := syncLogRepo.UpdateSyncLog(logID, &completedAt, 0, 0, 1, &errorMessage, "failed"); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	return nil
}

func (s *ImageSyncService) SyncFromStorage(ctx context.Context, triggeredBy string) (*SyncResult, error) {
	logID, err := s.createSyncLog(triggeredBy, time.Now())
	if err != nil {
		return nil, fmt.Errorf("创建同步日志失败: %w", err)
	}

	return s.syncFromStorageWithLogID(ctx, triggeredBy, logID)
}

func (s *ImageSyncService) syncFromStorageWithLogID(ctx context.Context, triggeredBy string, logID int64) (*SyncResult, error) {
	slog.Info("开始从存储同步", "triggered_by", triggeredBy, "log_id", logID)

	slog.Info("开始扫描存储文件", "log_id", logID)
	storageFiles, err := s.storage.ListFiles(ctx, s.pathPrefix)
	if err != nil {
		errMsg := fmt.Sprintf("扫描仓库文件失败: %v", err)
		if updateErr := s.markSyncLogFailed(logID, errMsg); updateErr != nil {
			slog.Error("更新同步日志失败", "log_id", logID, "error", updateErr)
		}
		return nil, fmt.Errorf("%s", errMsg)
	}
	slog.Info("扫描完成", "log_id", logID, "total_files", len(storageFiles))

	storageFileMap := make(map[string]*storage.RepositoryFile, len(storageFiles))
	imageExtensions := map[string]bool{
		".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
	}
	originalFiles := make([]*storage.RepositoryFile, 0, len(storageFiles))
	for _, file := range storageFiles {
		storageFileMap[file.Path] = file

		if file.Type != "file" {
			continue
		}

		baseName := filepath.Base(file.Path)
		if strings.HasPrefix(baseName, "thumb_") || strings.HasPrefix(baseName, "watermark_") {
			continue
		}

		ext := ""
		if idx := strings.LastIndex(file.Path, "."); idx > 0 {
			ext = strings.ToLower(file.Path[idx:])
		}
		if imageExtensions[ext] {
			originalFiles = append(originalFiles, file)
		}
	}

	slog.Info("找到的原图文件", "log_id", logID, "count", len(originalFiles))

	syncTag, dbImages, err := s.loadSyncContext(logID)
	if err != nil {
		errMsg := fmt.Sprintf("初始化同步上下文失败: %v", err)
		if updateErr := s.markSyncLogFailed(logID, errMsg); updateErr != nil {
			slog.Error("更新同步日志失败", "log_id", logID, "error", updateErr)
		}
		return nil, fmt.Errorf("%s", errMsg)
	}

	dbImageMap := make(map[string]*model.Image)
	for _, img := range dbImages {
		dbImageMap[img.Path] = img
	}

	result := &SyncResult{LogID: logID}

	totalFiles := len(originalFiles)
	processedFiles := 0
	errorCount := 0

	if err := s.updateSyncLogStatus(logID, nil, totalFiles, processedFiles, errorCount, nil, "running"); err != nil {
		slog.Error("初始化同步进度失败", "log_id", logID, "error", err)
	}

	for batchStart := 0; batchStart < len(originalFiles); batchStart += syncBatchSize {
		batchEnd := batchStart + syncBatchSize
		if batchEnd > len(originalFiles) {
			batchEnd = len(originalFiles)
		}

		batch := originalFiles[batchStart:batchEnd]
		slog.Info("处理原图批次", "log_id", logID, "batch_start", batchStart+1, "batch_end", batchEnd, "batch_size", len(batch))

		batchResult, batchErr := s.processOriginalBatch(ctx, logID, batch, storageFileMap, dbImageMap, syncTag.ID)
		processedFiles += len(batch)

		if batchErr != nil {
			errorCount += len(batch)
			slog.Error("原图批次事务失败，已跳过该批", "log_id", logID, "batch_start", batchStart+1, "batch_end", batchEnd, "error", batchErr)
		} else {
			result.CreatedCount += batchResult.CreatedCount
			result.UpdatedCount += batchResult.UpdatedCount
			result.SkippedCount += batchResult.SkippedCount
			errorCount += batchResult.ErrorCount
			for path, image := range batchResult.NewImages {
				dbImageMap[path] = image
			}
		}

		if err := s.updateSyncLogStatus(logID, nil, totalFiles, processedFiles, errorCount, nil, "running"); err != nil {
			slog.Error("更新同步日志进度失败", "log_id", logID, "error", err)
		}
	}

	slog.Info("检查数据库中已删除的文件", "log_id", logID)
	missingImages := make([]*model.Image, 0)
	for _, dbImage := range dbImages {
		if storageFileMap[dbImage.Path] == nil {
			missingImages = append(missingImages, dbImage)
		}
	}

	for batchStart := 0; batchStart < len(missingImages); batchStart += syncBatchSize {
		batchEnd := batchStart + syncBatchSize
		if batchEnd > len(missingImages) {
			batchEnd = len(missingImages)
		}

		batch := missingImages[batchStart:batchEnd]
		slog.Info("处理删除批次", "log_id", logID, "batch_start", batchStart+1, "batch_end", batchEnd, "batch_size", len(batch))

		batchResult, batchErr := s.processDeleteBatch(logID, batch)
		if batchErr != nil {
			errorCount += len(batch)
			slog.Error("删除批次事务失败，已跳过该批", "log_id", logID, "batch_start", batchStart+1, "batch_end", batchEnd, "error", batchErr)
		} else {
			result.DeletedCount += batchResult.DeletedCount
			errorCount += batchResult.ErrorCount
		}

		if err := s.updateSyncLogStatus(logID, nil, totalFiles, processedFiles, errorCount, nil, "running"); err != nil {
			slog.Error("更新同步日志进度失败", "log_id", logID, "error", err)
		}
	}

	result.ErrorCount = errorCount

	completedAt := time.Now()
	status := "completed"
	errorMessage := (*string)(nil)
	if errorCount > 0 {
		status = "completed_with_errors"
	}

	if err := s.updateSyncLogStatus(logID, &completedAt, totalFiles, processedFiles, errorCount, errorMessage, status); err != nil {
		slog.Error("更新同步日志失败", "log_id", logID, "error", err)
	}

	slog.Info("同步完成", "log_id", logID, "created", result.CreatedCount, "updated", result.UpdatedCount, "deleted", result.DeletedCount, "skipped", result.SkippedCount, "errors", result.ErrorCount)

	return result, nil
}

func (s *ImageSyncService) loadSyncContext(logID int64) (*model.Tag, []*model.Image, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, nil, fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	imageRepo := repository.NewImageRepository(tx)

	syncTag, err := imageRepo.GetOrCreateSyncTag()
	if err != nil {
		return nil, nil, fmt.Errorf("获取或创建同步标签失败: %w", err)
	}
	slog.Info("同步标签", "log_id", logID, "tag_id", syncTag.ID, "tag_name", syncTag.Name)

	slog.Info("查询数据库所有图片", "log_id", logID)
	dbImages, err := imageRepo.GetAllImagesNotDeleted()
	if err != nil {
		return nil, nil, fmt.Errorf("查询数据库图片失败: %w", err)
	}
	slog.Info("数据库图片", "log_id", logID, "total", len(dbImages))

	if err := tx.Commit(); err != nil {
		return nil, nil, fmt.Errorf("提交事务失败: %w", err)
	}

	return syncTag, dbImages, nil
}

func (s *ImageSyncService) updateSyncLogStatus(logID int64, completedAt *time.Time, totalFiles, processedFiles, errorCount int, errorMessage *string, status string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	syncLogRepo := repository.NewSyncLogRepository(tx)
	if err := syncLogRepo.UpdateSyncLog(logID, completedAt, totalFiles, processedFiles, errorCount, errorMessage, status); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("提交事务失败: %w", err)
	}

	return nil
}

func (s *ImageSyncService) processOriginalBatch(
	ctx context.Context,
	logID int64,
	batch []*storage.RepositoryFile,
	storageFileMap map[string]*storage.RepositoryFile,
	dbImageMap map[string]*model.Image,
	syncTagID int64,
) (*originalBatchResult, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	imageRepo := repository.NewImageRepository(tx)
	syncLogRepo := repository.NewSyncLogRepository(tx)

	result := &originalBatchResult{
		NewImages: make(map[string]*model.Image),
	}

	for _, storageFile := range batch {
		dbImage := dbImageMap[storageFile.Path]
		if createdImage, ok := result.NewImages[storageFile.Path]; ok {
			dbImage = createdImage
		}

		if dbImage == nil {
			slog.Info("文件不在数据库中，创建新记录", "log_id", logID, "path", storageFile.Path)
			image, err := s.createImageFromStorage(ctx, storageFile, syncTagID, imageRepo)
			if err != nil {
				errMsg := fmt.Sprintf("创建图片记录失败: %v", err)
				slog.Error(errMsg, "log_id", logID, "path", storageFile.Path)
				_ = syncLogRepo.CreateSyncFileLog(logID, storageFile.Path, "created", "failed", &storageFile.SHA, nil, &storageFile.Size, nil, &errMsg)
				result.ErrorCount++
				continue
			}
			slog.Info("创建成功", "log_id", logID, "path", storageFile.Path, "image_id", image.ID)
			_ = syncLogRepo.CreateSyncFileLog(logID, storageFile.Path, "created", "success", &storageFile.SHA, nil, &storageFile.Size, nil, nil)
			result.CreatedCount++
			result.NewImages[storageFile.Path] = image
			s.updateDerivedImageMeta(ctx, image.ID, storageFile.Path, storageFileMap, imageRepo, syncLogRepo, logID, &result.ErrorCount)
			continue
		}

		if dbImage.SHA != storageFile.SHA {
			slog.Info("文件SHA不一致，更新元数据", "log_id", logID, "path", storageFile.Path, "old_sha", dbImage.SHA, "new_sha", storageFile.SHA)
			oldSHA := dbImage.SHA
			oldSize := dbImage.Size

			width, height := 0, 0
			content, err := s.storage.GetRawFileContent(ctx, storageFile.Path)
			if err != nil {
				slog.Warn("获取文件内容失败，无法解析分辨率", "log_id", logID, "path", storageFile.Path, "error", err)
			} else {
				detectedWidth, detectedHeight, _, imgErr := imageutil.GetImageInfo(content)
				if imgErr != nil {
					slog.Warn("解析图片分辨率失败", "log_id", logID, "path", storageFile.Path, "error", imgErr)
				} else {
					width = detectedWidth
					height = detectedHeight
				}
			}

			err = imageRepo.UpdateImageMeta(dbImage.ID, storageFile.SHA, &storageFile.Size, &width, &height)
			if err != nil {
				errMsg := fmt.Sprintf("更新图片元数据失败: %v", err)
				slog.Error(errMsg, "log_id", logID, "path", storageFile.Path)
				_ = syncLogRepo.CreateSyncFileLog(logID, storageFile.Path, "updated", "failed", &storageFile.SHA, &oldSHA, &storageFile.Size, oldSize, &errMsg)
				result.ErrorCount++
				continue
			}
			slog.Info("更新成功", "log_id", logID, "path", storageFile.Path)
			_ = syncLogRepo.CreateSyncFileLog(logID, storageFile.Path, "updated", "success", &storageFile.SHA, &oldSHA, &storageFile.Size, oldSize, nil)
			result.UpdatedCount++
			s.updateDerivedImageMeta(ctx, dbImage.ID, storageFile.Path, storageFileMap, imageRepo, syncLogRepo, logID, &result.ErrorCount)
			continue
		}

		slog.Info("SHA一致，跳过", "log_id", logID, "path", storageFile.Path)
		_ = syncLogRepo.CreateSyncFileLog(logID, storageFile.Path, "skipped", "success", &storageFile.SHA, &storageFile.SHA, &storageFile.Size, &storageFile.Size, nil)
		result.SkippedCount++
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("提交事务失败: %w", err)
	}

	return result, nil
}

func (s *ImageSyncService) processDeleteBatch(logID int64, batch []*model.Image) (*deleteBatchResult, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("开始事务失败: %w", err)
	}
	defer tx.Rollback()

	imageRepo := repository.NewImageRepository(tx)
	syncLogRepo := repository.NewSyncLogRepository(tx)

	result := &deleteBatchResult{}
	for _, dbImage := range batch {
		slog.Info("文件在存储中不存在，软删除", "log_id", logID, "path", dbImage.Path)
		err := imageRepo.SoftDeleteImage(dbImage.ID)
		if err != nil {
			errMsg := fmt.Sprintf("软删除图片失败: %v", err)
			slog.Error(errMsg, "log_id", logID, "path", dbImage.Path)
			_ = syncLogRepo.CreateSyncFileLog(logID, dbImage.Path, "deleted", "failed", nil, &dbImage.SHA, nil, dbImage.Size, &errMsg)
			result.ErrorCount++
			continue
		}

		slog.Info("软删除成功", "log_id", logID, "path", dbImage.Path)
		_ = syncLogRepo.CreateSyncFileLog(logID, dbImage.Path, "deleted", "success", nil, &dbImage.SHA, nil, dbImage.Size, nil)
		result.DeletedCount++
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("提交事务失败: %w", err)
	}

	return result, nil
}

func (s *ImageSyncService) deriveThumbnailPath(originalPath string) string {
	dir := filepath.Dir(originalPath)
	base := filepath.Base(originalPath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	return filepath.Join(dir, "thumb_"+name+".jpg")
}

func (s *ImageSyncService) deriveWatermarkPath(originalPath string) string {
	dir := filepath.Dir(originalPath)
	base := filepath.Base(originalPath)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)
	return filepath.Join(dir, "watermark_"+name+ext)
}

func (s *ImageSyncService) updateDerivedImageMeta(
	ctx context.Context,
	imageID int64,
	originalPath string,
	storageFileMap map[string]*storage.RepositoryFile,
	imageRepo repository.ImageRepositoryInterface,
	syncLogRepo repository.SyncLogRepositoryInterface,
	logID int64,
	errorCount *int,
) {
	thumbPath := s.deriveThumbnailPath(originalPath)
	if thumbFile, ok := storageFileMap[thumbPath]; ok {
		thumbWidth, thumbHeight := 0, 0
		thumbContent, err := s.storage.GetRawFileContent(ctx, thumbFile.Path)
		if err != nil {
			slog.Warn("获取缩略图内容失败，无法解析分辨率", "path", thumbFile.Path, "error", err)
		} else {
			detectedWidth, detectedHeight, _, imgErr := imageutil.GetImageInfo(thumbContent)
			if imgErr != nil {
				slog.Warn("解析缩略图分辨率失败", "path", thumbFile.Path, "error", imgErr)
			} else {
				thumbWidth = detectedWidth
				thumbHeight = detectedHeight
			}
		}

		if err := imageRepo.UpdateThumbnailMeta(imageID, thumbFile.Path, s.storage.GetURL(ctx, thumbFile.Path), thumbFile.SHA, &thumbFile.Size, &thumbWidth, &thumbHeight); err != nil {
			errMsg := fmt.Sprintf("更新缩略图元数据失败: %v", err)
			slog.Error(errMsg, "path", thumbPath, "image_id", imageID)
			_ = syncLogRepo.CreateSyncFileLog(logID, thumbPath, "updated", "failed", &thumbFile.SHA, nil, &thumbFile.Size, nil, &errMsg)
			*errorCount++
		} else {
			slog.Info("缩略图元数据更新成功", "path", thumbPath, "image_id", imageID)
			_ = syncLogRepo.CreateSyncFileLog(logID, thumbPath, "updated", "success", &thumbFile.SHA, nil, &thumbFile.Size, nil, nil)
		}
	}

	watermarkPath := s.deriveWatermarkPath(originalPath)
	if watermarkFile, ok := storageFileMap[watermarkPath]; ok {
		if err := imageRepo.UpdateWatermarkMeta(imageID, watermarkFile.Path, s.storage.GetURL(ctx, watermarkFile.Path), watermarkFile.SHA, &watermarkFile.Size); err != nil {
			errMsg := fmt.Sprintf("更新水印图元数据失败: %v", err)
			slog.Error(errMsg, "path", watermarkPath, "image_id", imageID)
			_ = syncLogRepo.CreateSyncFileLog(logID, watermarkPath, "updated", "failed", &watermarkFile.SHA, nil, &watermarkFile.Size, nil, &errMsg)
			*errorCount++
		} else {
			slog.Info("水印图元数据更新成功", "path", watermarkPath, "image_id", imageID)
			_ = syncLogRepo.CreateSyncFileLog(logID, watermarkPath, "updated", "success", &watermarkFile.SHA, nil, &watermarkFile.Size, nil, nil)
		}
	}
}

func (s *ImageSyncService) createImageFromStorage(ctx context.Context, storageFile *storage.RepositoryFile, syncTagID int64, imageRepo repository.ImageRepositoryInterface) (*model.Image, error) {
	mimeType := "image/jpeg"
	if strings.LastIndex(storageFile.Path, ".") > 0 {
		mimeType = imageutil.GetMimeType(storageFile.Path)
	}

	width, height := 0, 0
	content, err := s.storage.GetRawFileContent(ctx, storageFile.Path)
	if err != nil {
		slog.Warn("获取文件内容失败，无法解析分辨率", "path", storageFile.Path, "error", err)
	} else {
		detectedWidth, detectedHeight, detectedMimeType, imgErr := imageutil.GetImageInfo(content)
		if imgErr != nil {
			slog.Warn("解析图片分辨率失败", "path", storageFile.Path, "error", imgErr)
		} else {
			width = detectedWidth
			height = detectedHeight
			if mimeType == "image/jpeg" && detectedMimeType != "" {
				mimeType = detectedMimeType
			}
		}
	}

	image := &model.Image{
		Path:             storageFile.Path,
		URL:              s.storage.GetURL(ctx, storageFile.Path),
		SHA:              storageFile.SHA,
		OriginalFilename: filepath.Base(storageFile.Path),
		Filename:         filepath.Base(storageFile.Path),
		Size:             &storageFile.Size,
		MimeType:         mimeType,
		Width:            &width,
		Height:           &height,
		HasThumbnail:     false,
		HasWatermark:     false,
		UploadedAt:       time.Now(),
	}

	id, err := imageRepo.CreateImage(image)
	if err != nil {
		return nil, fmt.Errorf("创建图片记录失败: %w", err)
	}
	image.ID = id

	err = imageRepo.AddImageTags(id, []int{int(syncTagID)})
	if err != nil {
		return nil, fmt.Errorf("关联同步标签失败: %w", err)
	}

	return image, nil
}
