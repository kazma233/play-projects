package repository

import (
	"database/sql"
	"fmt"
	"time"

	"picstash/internal/model"
)

type ImageRepositoryInterface interface {
	CreateImage(image *model.Image) (int64, error)
	AddImageTags(imageID int64, tagIDs []int) error
	DeleteImageTags(imageID int64) error
	GetImagesByCursor(cursor *model.ImageListCursor, limit int, tagID *int) ([]*model.Image, error)
	CountImages(tagID *int) (int, error)
	GetImageByID(id int64) (*model.Image, error)
	SoftDeleteImage(id int64) error
	UpdateImageTags(imageID int64, tagIDs []int) error
	GetTagsByImageID(imageID int64) ([]*model.Tag, error)
	GetAllImagesNotDeleted() ([]*model.Image, error)
	FindByPath(path string) (*model.Image, error)
	UpdateImageMeta(id int64, sha string, size *int64, width *int, height *int) error
	UpdateThumbnailMeta(id int64, thumbnailPath, thumbnailURL, thumbnailSHA string, thumbnailSize *int64, thumbnailWidth, thumbnailHeight *int) error
	UpdateWatermarkMeta(id int64, watermarkPath, watermarkURL, watermarkSHA string, watermarkSize *int64) error
	GetOrCreateSyncTag() (*model.Tag, error)
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

const tagSelectFields = `
		id, created_at, deleted_at, deleted, name, color
`

const tagSelectFieldsWithAlias = `
		t.id, t.created_at, t.deleted_at, t.deleted, t.name, t.color
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

func scanTag(scanner rowScanner, tag *model.Tag) error {
	return scanner.Scan(
		&tag.ID, &tag.CreatedAt, &tag.DeletedAt, &tag.Deleted, &tag.Name, &tag.Color,
	)
}

func (r *imageRepository) CreateImage(image *model.Image) (int64, error) {
	createdAt := image.CreatedAt
	if createdAt.IsZero() {
		if !image.UploadedAt.IsZero() {
			createdAt = image.UploadedAt
		} else {
			createdAt = time.Now()
		}
	}

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
		createdAt,
		image.Path, image.URL, image.SHA,
		image.ThumbnailPath, image.ThumbnailURL, image.ThumbnailSHA,
		image.WatermarkPath, image.WatermarkURL, image.WatermarkSHA, image.WatermarkSize,
		image.OriginalFilename, image.Filename, image.Size, image.ThumbnailSize,
		image.MimeType, image.Width, image.Height, image.HasThumbnail, image.HasWatermark,
		image.ThumbnailWidth, image.ThumbnailHeight)
	if err != nil {
		return 0, fmt.Errorf("创建图片失败: %w", err)
	}
	image.CreatedAt = createdAt
	image.Deleted = model.DeleteStateNotDeleted
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("获取图片ID失败: %w", err)
	}
	return id, nil
}

func (r *imageRepository) AddImageTags(imageID int64, tagIDs []int) error {
	for _, tagID := range tagIDs {
		_, err := r.tx.Exec(`INSERT INTO image_tags (image_id, tag_id) VALUES (?, ?)`, imageID, tagID)
		if err != nil {
			return fmt.Errorf("关联标签失败: %w", err)
		}
	}
	return nil
}

func (r *imageRepository) DeleteImageTags(imageID int64) error {
	_, err := r.tx.Exec(`UPDATE image_tags SET deleted = 1, deleted_at = ? WHERE image_id = ? AND deleted = 0`, time.Now(), imageID)
	if err != nil {
		return fmt.Errorf("删除标签关联失败: %w", err)
	}
	return nil
}

func (r *imageRepository) GetImagesByCursor(cursor *model.ImageListCursor, limit int, tagID *int) ([]*model.Image, error) {
	var args []interface{}
	query := `SELECT ` + imageSelectFields + ` FROM images WHERE deleted = 0`

	if tagID != nil {
		query += ` AND id IN (
			SELECT it.image_id
			FROM image_tags it
			INNER JOIN tags t ON t.id = it.tag_id
			WHERE it.tag_id = ? AND it.deleted = 0 AND t.deleted = 0
		)`
		args = append(args, *tagID)
	}

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
		err := scanImage(rows, img)
		if err != nil {
			return nil, fmt.Errorf("扫描图片数据失败: %w", err)
		}
		images = append(images, img)
	}

	if err := r.loadImageTags(images); err != nil {
		return nil, err
	}

	return images, nil
}

func (r *imageRepository) CountImages(tagID *int) (int, error) {
	var countQuery string
	var countArgs []interface{}
	if tagID != nil {
		countQuery = `
			SELECT COUNT(*)
			FROM images
			WHERE deleted = 0 AND id IN (
				SELECT it.image_id
				FROM image_tags it
				INNER JOIN tags t ON t.id = it.tag_id
				WHERE it.tag_id = ? AND it.deleted = 0 AND t.deleted = 0
			)
		`
		countArgs = append(countArgs, *tagID)
	} else {
		countQuery = `SELECT COUNT(*) FROM images WHERE deleted = 0`
	}

	var total int
	err := r.tx.QueryRow(countQuery, countArgs...).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("查询图片总数失败: %w", err)
	}

	return total, nil
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
			return nil, fmt.Errorf("图片不存在")
		}
		return nil, fmt.Errorf("查询图片失败: %w", err)
	}

	rows, err := r.tx.Query(`
		SELECT `+tagSelectFieldsWithAlias+`
		FROM tags t
		INNER JOIN image_tags it ON t.id = it.tag_id
		WHERE it.image_id = ? AND it.deleted = 0 AND t.deleted = 0
		ORDER BY it.created_at DESC
	`, id)
	if err != nil {
		return img, nil
	}
	defer rows.Close()

	for rows.Next() {
		tag := &model.Tag{}
		if err := scanTag(rows, tag); err != nil {
			continue
		}
		img.Tags = append(img.Tags, *tag)
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

func (r *imageRepository) UpdateImageTags(imageID int64, tagIDs []int) error {
	_, err := r.tx.Exec(`UPDATE image_tags SET deleted = 1, deleted_at = ? WHERE image_id = ? AND deleted = 0`, time.Now(), imageID)
	if err != nil {
		return fmt.Errorf("删除旧标签失败: %w", err)
	}

	for _, tagID := range tagIDs {
		_, err = r.tx.Exec(`INSERT INTO image_tags (image_id, tag_id) VALUES (?, ?)`, imageID, tagID)
		if err != nil {
			return fmt.Errorf("添加新标签失败: %w", err)
		}
	}
	return nil
}

func (r *imageRepository) GetTagsByImageID(imageID int64) ([]*model.Tag, error) {
	rows, err := r.tx.Query(`
		SELECT `+tagSelectFieldsWithAlias+`
		FROM tags t
		INNER JOIN image_tags it ON t.id = it.tag_id
		WHERE it.image_id = ? AND it.deleted = 0 AND t.deleted = 0
		ORDER BY it.created_at DESC
	`, imageID)
	if err != nil {
		return nil, fmt.Errorf("查询图片标签失败: %w", err)
	}
	defer rows.Close()

	var tags []*model.Tag
	for rows.Next() {
		tag := &model.Tag{}
		if err := scanTag(rows, tag); err != nil {
			continue
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

func placeholders(n int) string {
	if n <= 0 {
		return ""
	}
	result := "?"
	for i := 1; i < n; i++ {
		result += ", ?"
	}
	return result
}

func int64sToInterfaces(nums ...int64) []interface{} {
	interfaces := make([]interface{}, len(nums))
	for i, n := range nums {
		interfaces[i] = n
	}
	return interfaces
}

func (r *imageRepository) loadImageTags(images []*model.Image) error {
	if len(images) == 0 {
		return nil
	}

	imageIDs := make([]int64, len(images))
	for i, img := range images {
		imageIDs[i] = img.ID
	}

	tagRows, err := r.tx.Query(`
		SELECT it.image_id, `+tagSelectFieldsWithAlias+`
		FROM tags t
		INNER JOIN image_tags it ON t.id = it.tag_id
		WHERE it.image_id IN (`+placeholders(len(imageIDs))+`) AND it.deleted = 0 AND t.deleted = 0
	`, int64sToInterfaces(imageIDs...)...)
	if err != nil {
		return fmt.Errorf("查询图片标签失败: %w", err)
	}
	defer tagRows.Close()

	tagMap := make(map[int64][]model.Tag)
	for tagRows.Next() {
		var imageID int64
		var tag model.Tag
		if err := tagRows.Scan(&imageID, &tag.ID, &tag.CreatedAt, &tag.DeletedAt, &tag.Deleted, &tag.Name, &tag.Color); err != nil {
			continue
		}
		tagMap[imageID] = append(tagMap[imageID], tag)
	}

	for _, img := range images {
		img.Tags = tagMap[img.ID]
	}

	return nil
}

func (r *imageRepository) GetAllImagesNotDeleted() ([]*model.Image, error) {
	query := `SELECT ` + imageSelectFields + ` FROM images WHERE deleted = 0`
	rows, err := r.tx.Query(query)
	if err != nil {
		return nil, fmt.Errorf("查询所有图片失败: %w", err)
	}
	defer rows.Close()

	var images []*model.Image
	for rows.Next() {
		img := &model.Image{}
		err := scanImage(rows, img)
		if err != nil {
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

func (r *imageRepository) GetOrCreateSyncTag() (*model.Tag, error) {
	tag := &model.Tag{}
	err := r.tx.QueryRow(`SELECT `+tagSelectFields+` FROM tags WHERE name = '同步' AND deleted = 0`).Scan(
		&tag.ID, &tag.CreatedAt, &tag.DeletedAt, &tag.Deleted, &tag.Name, &tag.Color,
	)
	if err == nil {
		return tag, nil
	}
	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("查询同步标签失败: %w", err)
	}

	result, err := r.tx.Exec(`INSERT INTO tags (name, color) VALUES ('同步', '#9CA3AF')`)
	if err != nil {
		return nil, fmt.Errorf("创建同步标签失败: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("获取同步标签ID失败: %w", err)
	}

	tag.ID = id
	tag.Deleted = model.DeleteStateNotDeleted
	tag.Name = "同步"
	tag.Color = "#9CA3AF"
	tag.CreatedAt = time.Now()

	return tag, nil
}
