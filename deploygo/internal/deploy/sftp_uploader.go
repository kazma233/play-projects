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
	"sync"
	"time"

	"deploygo/internal/config"
	"deploygo/internal/fileutil"
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

const sftpUploadWorkers = 4

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

	dest = fileutil.RemoteClean(dest)

	srcInfo, err := os.Stat(source)
	if err != nil {
		return fmt.Errorf("failed to stat source: %w", err)
	}

	if srcInfo.IsDir() {
		return s.uploadDir(sftpClient, source, dest, excludes)
	}

	return s.uploadFile(sftpClient, source, dest)
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

	dstDir := fileutil.RemoteDir(dest)

	log.Printf("SFTP creating directory: %s", dstDir)
	if err := sftp.MkdirAll(dstDir); err != nil {
		return fmt.Errorf("failed to create destination directory '%s': %w", dstDir, err)
	}

	tempDest := fileutil.RemoteTempPath(dest, time.Now())
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
	type uploadJob struct {
		source string
		dest   string
	}

	var (
		errMu    sync.Mutex
		firstErr error
	)
	setErr := func(err error) {
		if err == nil {
			return
		}
		errMu.Lock()
		if firstErr == nil {
			firstErr = err
		}
		errMu.Unlock()
	}
	getErr := func() error {
		errMu.Lock()
		defer errMu.Unlock()
		return firstErr
	}

	jobs := make(chan uploadJob, sftpUploadWorkers*2)
	var wg sync.WaitGroup
	for range sftpUploadWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				if getErr() != nil {
					continue
				}
				if err := s.uploadFile(sftp, job.source, job.dest); err != nil {
					setErr(err)
				}
			}
		}()
	}

	errStopWalk := errors.New("stop walking upload tree")
	stopWalk := func(err error) error {
		setErr(err)
		return errStopWalk
	}

	walkErr := filepath.Walk(source, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return stopWalk(err)
		}
		if getErr() != nil {
			return errStopWalk
		}

		relPath, err := filepath.Rel(source, path)
		if err != nil {
			return stopWalk(err)
		}

		relPathUnix := filepath.ToSlash(relPath)

		if relPathUnix == "." {
			return nil
		}

		for _, exclude := range excludes {
			if strings.HasPrefix(relPathUnix, exclude) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		dstPath := fileutil.RemoteJoin(dest, relPathUnix)
		log.Printf("DEBUG uploadDir dstPath=%s", dstPath)

		if info.IsDir() {
			if err := sftp.MkdirAll(dstPath); err != nil {
				return stopWalk(fmt.Errorf("failed to create destination directory '%s': %w", dstPath, err))
			}
			return nil
		}

		if getErr() != nil {
			return errStopWalk
		}
		jobs <- uploadJob{source: path, dest: dstPath}
		return nil
	})
	close(jobs)
	wg.Wait()

	if walkErr != nil && !errors.Is(walkErr, errStopWalk) {
		setErr(walkErr)
	}
	return getErr()
}

func (s *SFTPUploader) UploadDir(source, dest string, excludes []string) error {
	return s.Upload(source, dest, excludes)
}
