package model

import "time"

const (
	DeleteStateNotDeleted = 0
	DeleteStateDeleted    = 1
)

type BaseModel struct {
	ID        int64      `json:"id"`
	CreatedAt time.Time  `json:"created_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
	Deleted   int        `json:"deleted"`
}

type Image struct {
	BaseModel
	Path             string    `json:"path"`
	URL              string    `json:"url"`
	SHA              string    `json:"sha,omitempty"`
	ThumbnailPath    string    `json:"thumbnail_path,omitempty"`
	ThumbnailURL     string    `json:"thumbnail_url,omitempty"`
	ThumbnailSHA     string    `json:"thumbnail_sha,omitempty"`
	ThumbnailSize    *int64    `json:"thumbnail_size,omitempty"`
	ThumbnailWidth   *int      `json:"thumbnail_width,omitempty"`
	ThumbnailHeight  *int      `json:"thumbnail_height,omitempty"`
	WatermarkPath    string    `json:"watermark_path,omitempty"`
	WatermarkURL     string    `json:"watermark_url,omitempty"`
	WatermarkSHA     string    `json:"watermark_sha,omitempty"`
	WatermarkSize    *int64    `json:"watermark_size,omitempty"`
	OriginalFilename string    `json:"original_filename"`
	Filename         string    `json:"filename"`
	Size             *int64    `json:"size"`
	MimeType         string    `json:"mime_type"`
	Width            *int      `json:"width,omitempty"`
	Height           *int      `json:"height,omitempty"`
	HasThumbnail     bool      `json:"has_thumbnail"`
	HasWatermark     bool      `json:"has_watermark"`
	UploadedAt       time.Time `json:"uploaded_at"`
	Tags             []Tag     `json:"tags,omitempty"`
}

type ImageListCursor struct {
	ID int64 `json:"id"`
}

type Tag struct {
	BaseModel
	Name  string `json:"name"`
	Color string `json:"color"`
}

type ImageTag struct {
	BaseModel
	ImageID int64 `json:"image_id"`
	TagID   int64 `json:"tag_id"`
}

type VerificationCode struct {
	BaseModel
	Email     string     `json:"email"`
	Code      string     `json:"code"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	IPAddress string     `json:"ip_address,omitempty"`
}

type SyncLog struct {
	BaseModel
	TriggeredBy    string     `json:"triggered_by"`
	StartedAt      time.Time  `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	Status         string     `json:"status"`
	TotalFiles     int        `json:"total_files"`
	ProcessedFiles int        `json:"processed_files"`
	ErrorCount     int        `json:"error_count"`
	ErrorMessage   *string    `json:"error_message,omitempty"`
}

type SyncFileLog struct {
	BaseModel
	SyncLogID    int64   `json:"sync_log_id"`
	Path         string  `json:"path"`
	Action       string  `json:"action"`
	Status       string  `json:"status"`
	SHA          *string `json:"sha,omitempty"`
	OldSHA       *string `json:"old_sha,omitempty"`
	Size         *int64  `json:"size,omitempty"`
	OldSize      *int64  `json:"old_size,omitempty"`
	ErrorMessage *string `json:"error_message,omitempty"`
}
