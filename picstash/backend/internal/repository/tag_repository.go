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
var ErrTagNotFound = errors.New("标签不存在")

type TagRepositoryInterface interface {
	Create(name, color string) (*model.Tag, error)
	Update(id int64, name, color string) (*model.Tag, error)
	Delete(id int64) error
	GetByID(id int64) (*model.Tag, error)
	GetByName(name string) (*model.Tag, error)
	GetByIDs(ids []int64) ([]*model.Tag, error)
	GetAll() ([]*model.Tag, error)
}

type tagRepository struct {
	tx *sql.Tx
}

const tagSelectFields = `
		id, created_at, deleted_at, deleted, name, color
`

func NewTagRepository(tx *sql.Tx) TagRepositoryInterface {
	return &tagRepository{tx: tx}
}

func scanTag(scanner rowScanner, tag *model.Tag) error {
	return scanner.Scan(
		&tag.ID,
		&tag.CreatedAt,
		&tag.DeletedAt,
		&tag.Deleted,
		&tag.Name,
		&tag.Color,
	)
}

func (r *tagRepository) Create(name, color string) (*model.Tag, error) {
	createdAt := time.Now()

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

	return &model.Tag{
		BaseModel: model.BaseModel{
			ID:        id,
			CreatedAt: createdAt,
			Deleted:   model.DeleteStateNotDeleted,
		},
		Name:  name,
		Color: color,
	}, nil
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
		return nil, ErrTagNotFound
	}

	return r.GetByID(id)
}

func (r *tagRepository) Delete(id int64) error {
	result, err := r.tx.Exec(`UPDATE tags SET deleted = 1, deleted_at = ? WHERE id = ? AND deleted = 0`, time.Now(), id)
	if err != nil {
		return fmt.Errorf("删除标签失败: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return ErrTagNotFound
	}

	return nil
}

func (r *tagRepository) GetByID(id int64) (*model.Tag, error) {
	tag := &model.Tag{}
	err := scanTag(r.tx.QueryRow(`
		SELECT `+tagSelectFields+`
		FROM tags
		WHERE id = ? AND deleted = 0
	`, id), tag)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrTagNotFound
		}
		return nil, fmt.Errorf("查询标签失败: %w", err)
	}

	return tag, nil
}

func (r *tagRepository) GetByName(name string) (*model.Tag, error) {
	tag := &model.Tag{}
	err := scanTag(r.tx.QueryRow(`
		SELECT `+tagSelectFields+`
		FROM tags
		WHERE name = ? AND deleted = 0
	`, name), tag)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrTagNotFound
		}
		return nil, fmt.Errorf("按名称查询标签失败: %w", err)
	}

	return tag, nil
}

func (r *tagRepository) GetByIDs(ids []int64) ([]*model.Tag, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	rows, err := r.tx.Query(`
		SELECT `+tagSelectFields+`
		FROM tags
		WHERE deleted = 0 AND id IN (`+placeholders(len(ids))+`)
	`, int64sToInterfaces(ids)...)
	if err != nil {
		return nil, fmt.Errorf("按ID查询标签失败: %w", err)
	}
	defer rows.Close()

	var tags []*model.Tag
	for rows.Next() {
		tag := &model.Tag{}
		if err := scanTag(rows, tag); err != nil {
			return nil, fmt.Errorf("扫描标签数据失败: %w", err)
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

func (r *tagRepository) GetAll() ([]*model.Tag, error) {
	rows, err := r.tx.Query(`
		SELECT ` + tagSelectFields + `
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
		if err := scanTag(rows, tag); err != nil {
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
