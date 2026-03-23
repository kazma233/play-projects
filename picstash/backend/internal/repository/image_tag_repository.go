package repository

import (
	"database/sql"
	"fmt"
	"time"

	"picstash/internal/model"
)

type ImageTagRepositoryInterface interface {
	AddImageTags(imageID int64, tagIDs []int) error
	SoftDeleteByImageID(imageID int64) error
	ListByImageIDs(imageIDs []int64) ([]*model.ImageTag, error)
	ListImageIDsByTagID(tagID int64, cursorID *int64, limit int) ([]int64, error)
}

type imageTagRepository struct {
	tx *sql.Tx
}

func NewImageTagRepository(tx *sql.Tx) ImageTagRepositoryInterface {
	return &imageTagRepository{tx: tx}
}

func (r *imageTagRepository) AddImageTags(imageID int64, tagIDs []int) error {
	for _, tagID := range tagIDs {
		_, err := r.tx.Exec(`INSERT INTO image_tags (image_id, tag_id) VALUES (?, ?)`, imageID, tagID)
		if err != nil {
			return fmt.Errorf("关联标签失败: %w", err)
		}
	}

	return nil
}

func (r *imageTagRepository) SoftDeleteByImageID(imageID int64) error {
	_, err := r.tx.Exec(
		`UPDATE image_tags SET deleted = 1, deleted_at = ? WHERE image_id = ? AND deleted = 0`,
		time.Now(),
		imageID,
	)
	if err != nil {
		return fmt.Errorf("删除标签关联失败: %w", err)
	}

	return nil
}

func (r *imageTagRepository) ListByImageIDs(imageIDs []int64) ([]*model.ImageTag, error) {
	if len(imageIDs) == 0 {
		return nil, nil
	}

	rows, err := r.tx.Query(`
		SELECT id, created_at, deleted_at, deleted, image_id, tag_id
		FROM image_tags
		WHERE image_id IN (`+placeholders(len(imageIDs))+`) AND deleted = 0
		ORDER BY image_id ASC, created_at DESC
	`, int64sToInterfaces(imageIDs)...)
	if err != nil {
		return nil, fmt.Errorf("查询图片标签关联失败: %w", err)
	}
	defer rows.Close()

	var relations []*model.ImageTag
	for rows.Next() {
		relation := &model.ImageTag{}
		if err := rows.Scan(
			&relation.ID,
			&relation.CreatedAt,
			&relation.DeletedAt,
			&relation.Deleted,
			&relation.ImageID,
			&relation.TagID,
		); err != nil {
			return nil, fmt.Errorf("扫描图片标签关联失败: %w", err)
		}
		relations = append(relations, relation)
	}

	return relations, nil
}

func (r *imageTagRepository) ListImageIDsByTagID(tagID int64, cursorID *int64, limit int) ([]int64, error) {
	args := []interface{}{tagID}
	query := `
		SELECT image_id
		FROM image_tags
		WHERE tag_id = ? AND deleted = 0
	`

	if cursorID != nil {
		query += ` AND image_id < ?`
		args = append(args, *cursorID)
	}

	query += ` ORDER BY image_id DESC`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := r.tx.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("按标签查询图片关联失败: %w", err)
	}
	defer rows.Close()

	imageIDs := make([]int64, 0)
	for rows.Next() {
		var imageID int64
		if err := rows.Scan(&imageID); err != nil {
			return nil, fmt.Errorf("扫描图片关联失败: %w", err)
		}
		imageIDs = append(imageIDs, imageID)
	}

	return imageIDs, nil
}
