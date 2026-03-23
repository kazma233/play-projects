package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"picstash/internal/model"
)

var ErrImageNotFound = errors.New("图片不存在")

type ImageRepositoryInterface interface {
	CreateImage(image *model.Image) (int64, error)
	GetImagesByCursor(cursor *model.ImageListCursor, limit int) ([]*model.Image, error)
	CountImages() (int, error)
	CountImagesByIDs(ids []int64) (int, error)
	GetImagesByIDs(ids []int64) ([]*model.Image, error)
	GetImageByID(id int64) (*model.Image, error)
	SoftDeleteImage(id int64) error
	GetAllImagesNotDeleted() ([]*model.Image, error)
	FindByPath(path string) (*model.Image, error)
	UpdateImageMeta(id int64, sha string, size *int64, width *int, height *int) error
	UpdateThumbnailMeta(id int64, thumbnailPath, thumbnailURL, thumbnailSHA string, thumbnailSize *int64, thumbnailWidth, thumbnailHeight *int) error
	UpdateWatermarkMeta(id int64, watermarkPath, watermarkURL, watermarkSHA string, watermarkSize *int64) error
}

type imageRepository struct {
	tx *sql.Tx
}

type rowScanner interface {
	Scan(dest ...any) error
}

const imageSelectFields = `
		id, created_at, deleted_at, deleted,
		path, url, sha,
		thumbnail_path, thumbnail_url, thumbnail_sha,
		watermark_path, watermark_url, watermark_sha, watermark_size,
		original_filename, filename, size, thumbnail_size,
		mime_type, width, height, has_thumbnail, has_watermark,
		uploaded_at, thumbnail_width, thumbnail_height
`

func NewImageRepository(tx *sql.Tx) ImageRepositoryInterface {
	return &imageRepository{tx: tx}
}

func scanImage(scanner rowScanner, img *model.Image) error {
	return scanner.Scan(
		&img.ID, &img.CreatedAt, &img.DeletedAt, &img.Deleted,
		&img.Path, &img.URL, &img.SHA,
		&img.ThumbnailPath, &img.ThumbnailURL, &img.ThumbnailSHA,
		&img.WatermarkPath, &img.WatermarkURL, &img.WatermarkSHA, &img.WatermarkSize,
		&img.OriginalFilename, &img.Filename, &img.Size, &img.ThumbnailSize,
		&img.MimeType, &img.Width, &img.Height, &img.HasThumbnail, &img.HasWatermark,
		&img.UploadedAt, &img.ThumbnailWidth, &img.ThumbnailHeight,
	)
}

func (r *imageRepository) CreateImage(image *model.Image) (int64, error) {
	result, err := r.tx.Exec(`
		INSERT INTO images (
			created_at,
			path, url, sha,
			thumbnail_path, thumbnail_url, thumbnail_sha,
			watermark_path, watermark_url, watermark_sha, watermark_size,
			original_filename, filename, size, thumbnail_size,
			mime_type, width, height, has_thumbnail, has_watermark,
			thumbnail_width, thumbnail_height
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		image.CreatedAt,
		image.Path, image.URL, image.SHA,
		image.ThumbnailPath, image.ThumbnailURL, image.ThumbnailSHA,
		image.WatermarkPath, image.WatermarkURL, image.WatermarkSHA, image.WatermarkSize,
		image.OriginalFilename, image.Filename, image.Size, image.ThumbnailSize,
		image.MimeType, image.Width, image.Height, image.HasThumbnail, image.HasWatermark,
		image.ThumbnailWidth, image.ThumbnailHeight,
	)
	if err != nil {
		return 0, fmt.Errorf("创建图片失败: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("获取图片ID失败: %w", err)
	}

	return id, nil
}

func (r *imageRepository) GetImagesByCursor(cursor *model.ImageListCursor, limit int) ([]*model.Image, error) {
	var args []interface{}
	query := `SELECT ` + imageSelectFields + ` FROM images WHERE deleted = 0`

	if cursor != nil {
		query += ` AND id < ?`
		args = append(args, cursor.ID)
	}

	query += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := r.tx.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("查询图片列表失败: %w", err)
	}
	defer rows.Close()

	var images []*model.Image
	for rows.Next() {
		img := &model.Image{}
		if err := scanImage(rows, img); err != nil {
			return nil, fmt.Errorf("扫描图片数据失败: %w", err)
		}
		images = append(images, img)
	}

	return images, nil
}

func (r *imageRepository) CountImages() (int, error) {
	var total int
	err := r.tx.QueryRow(`SELECT COUNT(*) FROM images WHERE deleted = 0`).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("查询图片总数失败: %w", err)
	}

	return total, nil
}

func (r *imageRepository) CountImagesByIDs(ids []int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	var total int
	err := r.tx.QueryRow(
		`SELECT COUNT(*) FROM images WHERE deleted = 0 AND id IN (`+placeholders(len(ids))+`)`,
		int64sToInterfaces(ids)...,
	).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("按ID统计图片数量失败: %w", err)
	}

	return total, nil
}

func (r *imageRepository) GetImagesByIDs(ids []int64) ([]*model.Image, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	rows, err := r.tx.Query(`
		SELECT `+imageSelectFields+`
		FROM images
		WHERE deleted = 0 AND id IN (`+placeholders(len(ids))+`)
	`, int64sToInterfaces(ids)...)
	if err != nil {
		return nil, fmt.Errorf("按ID查询图片失败: %w", err)
	}
	defer rows.Close()

	var images []*model.Image
	for rows.Next() {
		img := &model.Image{}
		if err := scanImage(rows, img); err != nil {
			return nil, fmt.Errorf("扫描图片数据失败: %w", err)
		}
		images = append(images, img)
	}

	return images, nil
}

func (r *imageRepository) GetImageByID(id int64) (*model.Image, error) {
	img := &model.Image{}
	err := scanImage(r.tx.QueryRow(`
		SELECT `+imageSelectFields+`
		FROM images
		WHERE id = ? AND deleted = 0
	`, id), img)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrImageNotFound
		}
		return nil, fmt.Errorf("查询图片失败: %w", err)
	}

	return img, nil
}

func (r *imageRepository) SoftDeleteImage(id int64) error {
	_, err := r.tx.Exec(`UPDATE images SET deleted = 1, deleted_at = ? WHERE id = ? AND deleted = 0`, time.Now(), id)
	if err != nil {
		return fmt.Errorf("删除图片失败: %w", err)
	}

	return nil
}

func (r *imageRepository) GetAllImagesNotDeleted() ([]*model.Image, error) {
	rows, err := r.tx.Query(`SELECT ` + imageSelectFields + ` FROM images WHERE deleted = 0`)
	if err != nil {
		return nil, fmt.Errorf("查询所有图片失败: %w", err)
	}
	defer rows.Close()

	var images []*model.Image
	for rows.Next() {
		img := &model.Image{}
		if err := scanImage(rows, img); err != nil {
			return nil, fmt.Errorf("扫描图片数据失败: %w", err)
		}
		images = append(images, img)
	}

	return images, nil
}

func (r *imageRepository) FindByPath(path string) (*model.Image, error) {
	img := &model.Image{}
	err := scanImage(r.tx.QueryRow(`
		SELECT `+imageSelectFields+`
		FROM images
		WHERE path = ? AND deleted = 0
	`, path), img)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("查询图片失败: %w", err)
	}

	return img, nil
}

func (r *imageRepository) UpdateImageMeta(id int64, sha string, size *int64, width *int, height *int) error {
	_, err := r.tx.Exec(`UPDATE images SET sha = ?, size = ?, width = ?, height = ? WHERE id = ? AND deleted = 0`, sha, size, width, height, id)
	if err != nil {
		return fmt.Errorf("更新图片元数据失败: %w", err)
	}

	return nil
}

func (r *imageRepository) UpdateThumbnailMeta(id int64, thumbnailPath, thumbnailURL, thumbnailSHA string, thumbnailSize *int64, thumbnailWidth, thumbnailHeight *int) error {
	_, err := r.tx.Exec(`
		UPDATE images
		SET thumbnail_path = ?, thumbnail_url = ?, thumbnail_sha = ?, thumbnail_size = ?, has_thumbnail = 1, thumbnail_width = ?, thumbnail_height = ?
		WHERE id = ? AND deleted = 0
	`, thumbnailPath, thumbnailURL, thumbnailSHA, thumbnailSize, thumbnailWidth, thumbnailHeight, id)
	if err != nil {
		return fmt.Errorf("更新缩略图元数据失败: %w", err)
	}

	return nil
}

func (r *imageRepository) UpdateWatermarkMeta(id int64, watermarkPath, watermarkURL, watermarkSHA string, watermarkSize *int64) error {
	_, err := r.tx.Exec(`
		UPDATE images
		SET watermark_path = ?, watermark_url = ?, watermark_sha = ?, watermark_size = ?, has_watermark = 1
		WHERE id = ? AND deleted = 0
	`, watermarkPath, watermarkURL, watermarkSHA, watermarkSize, id)
	if err != nil {
		return fmt.Errorf("更新水印图元数据失败: %w", err)
	}

	return nil
}
