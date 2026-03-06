package deploy

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"deploygo/internal/config"
	"deploygo/internal/retry"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

type FileUploader interface {
	UploadDir(source, dest string, excludes []string) error
	Upload(source, dest string, excludes []string) error
}

type SFTPUploader struct {
	config connectionConfig
	client *ssh.Client
}

func NewSFTPUploader(server *config.ServerConfig) *SFTPUploader {
	return &SFTPUploader{
		config: newConnectionConfig(server),
	}
}

func (s *SFTPUploader) Connect() error {
	return retry.Do(context.Background(), "SFTP连接", sshRetryPolicy(), func() error {
		return reconnectSSHClient(s.config, &s.client)
	})
}

func (s *SFTPUploader) Close() {
	closeSSHClient(&s.client)
}

func (s *SFTPUploader) openClient() (*sftp.Client, error) {
	// SFTP client 的创建仍然属于“连接准备阶段”，失败后可以安全重试。
	// 真正文件上传过程暂不自动重试，避免留下半截远程文件。
	return retry.DoValue(context.Background(), "SFTP客户端创建", sshRetryPolicy(), func() (*sftp.Client, error) {
		if err := ensureSSHClient(s.config, &s.client); err != nil {
			return nil, err
		}

		candidate, err := sftp.NewClient(s.client)
		if err != nil {
			closeSSHClient(&s.client)
			return nil, fmt.Errorf("failed to create SFTP client: %w", err)
		}

		return candidate, nil
	})
}

func (s *SFTPUploader) Upload(source, dest string, excludes []string) error {
	sftpClient, err := s.openClient()
	if err != nil {
		return err
	}
	defer sftpClient.Close()

	dest = filepath.ToSlash(dest)

	srcInfo, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("failed to stat source: %w", err)
	}

	if srcInfo.IsDir() {
		return s.uploadDir(sftpClient, source, dest, excludes)
	}

	return s.uploadFile(sftpClient, source, dest)
}

func remoteTempPath(dest string, now time.Time) string {
	dest = filepath.ToSlash(dest)
	dir := filepath.ToSlash(filepath.Dir(dest))
	base := filepath.Base(dest)
	tempName := fmt.Sprintf(".%s.deploygo-upload-%d.tmp", base, now.UnixNano())
	return filepath.ToSlash(filepath.Join(dir, tempName))
}

func promoteUploadedFile(client *sftp.Client, tempPath, dest string) error {
	if err := client.PosixRename(tempPath, dest); err == nil {
		return nil
	} else {
		log.Printf("SFTP POSIX rename failed, fallback to standard rename: %v", err)
		posixRenameErr := err

		if err := client.Rename(tempPath, dest); err == nil {
			return nil
		} else {
			standardRenameErr := err

			if _, statErr := client.Stat(dest); statErr == nil {
				log.Printf("SFTP standard rename hit existing target, removing destination before retry: %s", dest)
				if err := client.Remove(dest); err != nil {
					return fmt.Errorf("failed to remove existing destination file '%s': %w", dest, err)
				}

				if err := client.Rename(tempPath, dest); err == nil {
					return nil
				} else {
					standardRenameErr = errors.Join(standardRenameErr, err)
				}
			}

			return fmt.Errorf("failed to promote uploaded temp file '%s' to '%s': %w", tempPath, dest, errors.Join(posixRenameErr, standardRenameErr))
		}
	}
}

func (s *SFTPUploader) uploadFile(sftp *sftp.Client, source, dest string) error {
	log.Printf("SFTP upload file: %s -> %s", source, dest)

	srcFile, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// 确保目标路径使用 Unix 风格的斜杠（Linux 服务器）
	dest = filepath.ToSlash(dest)
	dstDir := filepath.Dir(dest)
	// 再次确保目录路径是 Unix 风格
	dstDir = filepath.ToSlash(dstDir)

	log.Printf("SFTP creating directory: %s", dstDir)
	if err := sftp.MkdirAll(dstDir); err != nil {
		return fmt.Errorf("failed to create destination directory '%s': %w", dstDir, err)
	}

	tempDest := remoteTempPath(dest, time.Now())
	log.Printf("SFTP creating temp file: %s", tempDest)
	dstFile, err := sftp.Create(tempDest)
	if err != nil {
		return fmt.Errorf("failed to create temp destination file '%s': %w", tempDest, err)
	}
	cleanupTemp := true
	defer func() {
		if dstFile != nil {
			dstFile.Close()
		}
		if cleanupTemp {
			if err := sftp.Remove(tempDest); err != nil {
				log.Printf("SFTP cleanup temp file failed: %s: %v", tempDest, err)
			}
		}
	}()

	_, err = dstFile.ReadFrom(srcFile)
	if err != nil {
		return fmt.Errorf("failed to upload temp file '%s': %w", tempDest, err)
	}

	if err := dstFile.Close(); err != nil {
		dstFile = nil
		return fmt.Errorf("failed to close temp file '%s': %w", tempDest, err)
	}
	dstFile = nil

	if err := promoteUploadedFile(sftp, tempDest, dest); err != nil {
		return err
	}
	cleanupTemp = false

	log.Printf("SFTP upload complete: %s -> %s", source, dest)
	return nil
}

func (s *SFTPUploader) uploadDir(sftp *sftp.Client, source, dest string, excludes []string) error {
	// 确保目标路径使用 Unix 风格的斜杠（Linux 服务器）
	dest = filepath.ToSlash(dest)

	return filepath.Walk(source, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 计算相对路径（本地格式）
		relPath, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}

		// 转换为 Unix 风格的斜杠用于远程路径
		relPathUnix := filepath.ToSlash(relPath)

		if relPathUnix == "." {
			return nil
		}

		// 检查排除规则（使用 Unix 风格路径）
		for _, exclude := range excludes {
			if strings.HasPrefix(relPathUnix, exclude) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// 构建目标路径（Unix 风格）
		dstPath := dest + "/" + relPathUnix
		log.Printf("DEBUG uploadDir dstPath=%s", dstPath)

		if info.IsDir() {
			return sftp.MkdirAll(dstPath)
		}

		return s.uploadFile(sftp, path, dstPath)
	})
}

func (s *SFTPUploader) UploadDir(source, dest string, excludes []string) error {
	return s.Upload(source+"/", dest, excludes)
}
