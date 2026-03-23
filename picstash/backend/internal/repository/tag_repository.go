package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"picstash/internal/model"
)

var ErrTagNameExists = errors.New("标签名称已存在")

type TagRepositoryInterface interface {
	Create(name, color string) (*model.Tag, error)
	Update(id int64, name, color string) (*model.Tag, error)
	Delete(id int64) error
	GetByID(id int64) (*model.Tag, error)
	GetAll() ([]*model.Tag, error)
	GetByImageID(imageID int64) ([]*model.Tag, error)
}

type tagRepository struct {
	tx *sql.Tx
}

func NewTagRepository(tx *sql.Tx) TagRepositoryInterface {
	return &tagRepository{tx: tx}
}

func (r *tagRepository) Create(name, color string) (*model.Tag, error) {
	result, err := r.tx.Exec(`INSERT INTO tags (name, color) VALUES (?, ?)`, name, color)
	if err != nil {
		if isTagNameConflictError(err) {
			return nil, ErrTagNameExists
		}
		return nil, fmt.Errorf("创建标签失败: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("获取标签ID失败: %w", err)
	}

	tag := &model.Tag{
		BaseModel: model.BaseModel{
			ID:        id,
			CreatedAt: time.Now(),
			Deleted:   model.DeleteStateNotDeleted,
		},
		Name:  name,
		Color: color,
	}
	return tag, nil
}

func (r *tagRepository) Update(id int64, name, color string) (*model.Tag, error) {
	result, err := r.tx.Exec(`UPDATE tags SET name = ?, color = ? WHERE id = ? AND deleted = 0`, name, color, id)
	if err != nil {
		if isTagNameConflictError(err) {
			return nil, ErrTagNameExists
		}
		return nil, fmt.Errorf("更新标签失败: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return nil, fmt.Errorf("标签不存在")
	}

	tag, err := r.GetByID(id)
	if err != nil {
		return nil, err
	}
	return tag, nil
}

func (r *tagRepository) Delete(id int64) error {
	result, err := r.tx.Exec(`UPDATE tags SET deleted = 1, deleted_at = ? WHERE id = ? AND deleted = 0`, time.Now(), id)
	if err != nil {
		return fmt.Errorf("删除标签失败: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("标签不存在")
	}
	return nil
}

func (r *tagRepository) GetByID(id int64) (*model.Tag, error) {
	tag := &model.Tag{}

	err := r.tx.QueryRow(`
		SELECT id, name, color, created_at, deleted_at, deleted
		FROM tags
		WHERE id = ? AND deleted = 0
	`, id).Scan(&tag.ID, &tag.Name, &tag.Color, &tag.CreatedAt, &tag.DeletedAt, &tag.Deleted)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("标签不存在")
		}
		return nil, fmt.Errorf("查询标签失败: %w", err)
	}
	return tag, nil
}

func (r *tagRepository) GetAll() ([]*model.Tag, error) {
	rows, err := r.tx.Query(`
		SELECT id, name, color, created_at, deleted_at, deleted
		FROM tags
		WHERE deleted = 0
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("查询标签列表失败: %w", err)
	}
	defer rows.Close()

	var tags []*model.Tag
	for rows.Next() {
		tag := &model.Tag{}
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.Color, &tag.CreatedAt, &tag.DeletedAt, &tag.Deleted); err != nil {
			return nil, fmt.Errorf("扫描标签数据失败: %w", err)
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

func (r *tagRepository) GetByImageID(imageID int64) ([]*model.Tag, error) {
	rows, err := r.tx.Query(`
		SELECT t.id, t.name, t.color, t.created_at, t.deleted_at, t.deleted
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
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.Color, &tag.CreatedAt, &tag.DeletedAt, &tag.Deleted); err != nil {
			return nil, fmt.Errorf("扫描标签数据失败: %w", err)
		}
		tags = append(tags, tag)
	}
	return tags, nil
}

func isTagNameConflictError(err error) bool {
	if err == nil {
		return false
	}

	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "unique constraint failed: tags.name")
}
