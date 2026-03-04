package storage

import "context"

// File 表示要上传的文件
type File struct {
	Path        string // 文件路径（相对路径）
	Content     []byte // 文件内容
	ContentType string // 文件内容类型
}

// UploadResult 表示上传结果
type UploadResult struct {
	Path string // 文件路径
	URL  string // 访问URL
	SHA  string // 文件哈希（GitHub/本地均为 SHA-256）
}

// RepositoryFile 表示仓库中的文件信息
// 用于 ListFiles 返回的文件列表
type RepositoryFile struct {
	Path        string // 文件路径
	SHA         string // 文件SHA或校验和
	Size        int64  // 文件大小（字节）
	Type        string // 文件类型："file" 或 "dir"
	DownloadURL string // 下载URL
}

type Storage interface {
	Upload(ctx context.Context, file *File) (*UploadResult, error)
	Delete(ctx context.Context, path, sha string) error
	GetURL(ctx context.Context, path string) string
	ListFiles(ctx context.Context, path string) ([]*RepositoryFile, error)
	GetRawFileContent(ctx context.Context, path string) ([]byte, error)
	GetPublicURL(path string) string
}
