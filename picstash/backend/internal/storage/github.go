package storage

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/google/go-github/v58/github"
)

type githubStorage struct {
	client    *github.Client
	repoOwner string
	repoName  string
	branch    string
	baseURL   string
}

func NewGitHubStorage(token, owner, repo, branch string) Storage {
	client := github.NewTokenClient(context.Background(), token)
	return &githubStorage{
		client:    client,
		repoOwner: owner,
		repoName:  repo,
		branch:    branch,
		baseURL:   fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/", owner, repo, branch),
	}
}

func (s *githubStorage) Upload(ctx context.Context, file *File) (*UploadResult, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}

		opts := &github.RepositoryContentFileOptions{
			Message: github.String("Upload " + file.Path),
			Content: file.Content,
			Branch:  &s.branch,
		}

		fileContent, resp, err := s.client.Repositories.CreateFile(ctx, s.repoOwner, s.repoName, file.Path, opts)
		if err != nil {
			lastErr = err
			slog.Warn("GitHub上传失败，重试中", "path", file.Path, "attempt", attempt+1, "error", err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			lastErr = fmt.Errorf("上传失败，状态码: %d", resp.StatusCode)
			slog.Warn("GitHub上传失败，重试中", "path", file.Path, "attempt", attempt+1, "status", resp.StatusCode)
			continue
		}

		return &UploadResult{
			Path: file.Path,
			URL:  s.baseURL + file.Path,
			SHA:  fileContent.Content.GetSHA(),
		}, nil
	}

	slog.Error("GitHub上传失败", "path", file.Path, "error", lastErr)
	return nil, fmt.Errorf("上传文件到GitHub失败: %w", lastErr)
}

func (s *githubStorage) BatchUpload(ctx context.Context, files []*File) ([]*UploadResult, error) {
	results := make([]*UploadResult, 0, len(files))

	for _, file := range files {
		result, err := s.Upload(ctx, file)
		if err != nil {
			slog.Error("批量上传失败", "path", file.Path, "error", err)
			return results, err
		}
		results = append(results, result)
	}

	return results, nil
}

func (s *githubStorage) Delete(ctx context.Context, path, sha string) error {
	if sha == "" {
		return fmt.Errorf("文件SHA不能为空")
	}

	opts := &github.RepositoryContentFileOptions{
		SHA:     github.String(sha),
		Message: github.String("Delete " + path),
		Branch:  &s.branch,
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}

		_, resp, err := s.client.Repositories.DeleteFile(ctx, s.repoOwner, s.repoName, path, opts)
		if err != nil {
			lastErr = err
			if resp != nil && resp.StatusCode == http.StatusNotFound {
				slog.Warn("文件不存在，跳过删除", "path", path)
				return nil
			}
			slog.Warn("GitHub删除失败，重试中", "path", path, "attempt", attempt+1, "error", err)
			continue
		}

		if resp != nil {
			resp.Body.Close()
		}

		return nil
	}

	slog.Error("GitHub删除失败", "path", path, "error", lastErr)
	return fmt.Errorf("从GitHub删除文件失败: %w", lastErr)
}

func (s *githubStorage) GetURL(ctx context.Context, path string) string {
	return s.baseURL + path
}

// GetPublicURL 获取前端访问的完整URL
// 返回GitHub原始文件地址: https://raw.githubusercontent.com/owner/repo/branch/path
func (s *githubStorage) GetPublicURL(path string) string {
	return s.baseURL + path
}

func (s *githubStorage) Exists(ctx context.Context, path string) (bool, error) {
	_, _, resp, err := s.client.Repositories.GetContents(ctx, s.repoOwner, s.repoName, path, nil)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *githubStorage) ListFiles(ctx context.Context, path string) ([]*RepositoryFile, error) {
	var allFiles []*RepositoryFile

	_, dirContent, _, err := s.client.Repositories.GetContents(ctx, s.repoOwner, s.repoName, path, nil)
	if err != nil {
		return nil, fmt.Errorf("获取目录内容失败: %w", err)
	}

	for _, item := range dirContent {
		if item.GetType() == "file" {
			allFiles = append(allFiles, &RepositoryFile{
				Path:        item.GetPath(),
				SHA:         item.GetSHA(),
				Size:        int64(item.GetSize()),
				Type:        item.GetType(),
				DownloadURL: item.GetDownloadURL(),
			})
		} else if item.GetType() == "dir" {
			subFiles, err := s.ListFiles(ctx, item.GetPath())
			if err != nil {
				slog.Warn("递归获取子目录失败", "path", item.GetPath(), "error", err)
				continue
			}
			allFiles = append(allFiles, subFiles...)
		}
	}

	return allFiles, nil
}

func (s *githubStorage) GetRawFileContent(ctx context.Context, path string) ([]byte, error) {
	readCloser, _, err := s.client.Repositories.DownloadContents(ctx, s.repoOwner, s.repoName, path, nil)
	if err != nil {
		return nil, fmt.Errorf("下载文件内容失败: %w", err)
	}
	defer readCloser.Close()

	data, err := io.ReadAll(readCloser)
	if err != nil {
		return nil, fmt.Errorf("读取文件内容失败: %w", err)
	}

	return data, nil
}
