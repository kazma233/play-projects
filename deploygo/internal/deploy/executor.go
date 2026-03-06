package deploy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
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

type connectionConfig struct {
	Host     string
	User     string
	Port     int
	KeyPath  string
	Password string
	Timeout  time.Duration
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		if home != "" {
			return filepath.Join(home, path[2:])
		}
	}
	return path
}

func newConnectionConfig(server *config.ServerConfig) connectionConfig {
	return connectionConfig{
		Host:     server.Host,
		User:     server.User,
		Port:     server.Port,
		KeyPath:  server.KeyPath,
		Password: server.Password,
		Timeout:  30 * time.Second,
	}
}

func sshRetryPolicy() retry.Policy {
	// 这里只给连接建立类操作复用一套重试策略。
	// 真正的远程命令执行不自动重试，避免副作用命令被重复执行。
	return retry.NewPolicy(isRetryableSSHError)
}

func buildAuthMethods(cfg connectionConfig) ([]ssh.AuthMethod, error) {
	var authMethods []ssh.AuthMethod

	if cfg.Password != "" {
		authMethods = append(authMethods, ssh.Password(cfg.Password))
	}

	if cfg.KeyPath != "" {
		keyPath := expandHome(cfg.KeyPath)
		keyData, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read private key: %w", err)
		}

		signer, err := ssh.ParsePrivateKey(keyData)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}

		authMethods = append(authMethods, ssh.PublicKeys(signer))
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no authentication method provided")
	}

	return authMethods, nil
}

func dialSSHClient(cfg connectionConfig) (*ssh.Client, error) {
	authMethods, err := buildAuthMethods(cfg)
	if err != nil {
		return nil, err
	}

	host := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	client, err := ssh.Dial("tcp", host, &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         cfg.Timeout,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to dial SSH: %w", err)
	}

	return client, nil
}

func closeSSHClient(client **ssh.Client) {
	if *client == nil {
		return
	}
	(*client).Close()
	*client = nil
}

func ensureSSHClient(cfg connectionConfig, client **ssh.Client) error {
	if *client != nil {
		return nil
	}

	newClient, err := dialSSHClient(cfg)
	if err != nil {
		return err
	}

	*client = newClient
	return nil
}

func reconnectSSHClient(cfg connectionConfig, client **ssh.Client) error {
	closeSSHClient(client)
	return ensureSSHClient(cfg, client)
}

func isRetryableSSHError(err error) bool {
	if err == nil {
		return false
	}

	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	if errors.Is(err, io.EOF) ||
		errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.ETIMEDOUT) ||
		errors.Is(err, syscall.ENETUNREACH) ||
		errors.Is(err, syscall.EHOSTUNREACH) {
		return true
	}

	errText := strings.ToLower(err.Error())
	if strings.Contains(errText, "unable to authenticate") || strings.Contains(errText, "no supported methods remain") {
		return false
	}

	retryableKeywords := []string{
		"broken pipe",
		"connection reset",
		"connection refused",
		"connection timed out",
		"connection aborted",
		"connection closed",
		"handshake",
		"i/o timeout",
		"network is unreachable",
		"no route to host",
		"timeout",
		"use of closed network connection",
	}

	for _, keyword := range retryableKeywords {
		if strings.Contains(errText, keyword) {
			return true
		}
	}

	return false
}

type SSHExecutor struct {
	config connectionConfig
	client *ssh.Client
}

func NewSSHExecutor(server *config.ServerConfig) (*SSHExecutor, error) {
	return &SSHExecutor{
		config: newConnectionConfig(server),
	}, nil
}

func (s *SSHExecutor) Connect() error {
	return retry.Do(context.Background(), "SSH连接", sshRetryPolicy(), func() error {
		return reconnectSSHClient(s.config, &s.client)
	})
}

func (s *SSHExecutor) Close() {
	closeSSHClient(&s.client)
}

func (s *SSHExecutor) openSession() (*ssh.Session, error) {
	// 新建 session 本身是幂等的，适合放进 DoValue 中重试。
	// 一旦 session.Run 开始执行远程命令，就交给上层自行决定是否重试。
	return retry.DoValue(context.Background(), "SSH会话创建", sshRetryPolicy(), func() (*ssh.Session, error) {
		if err := ensureSSHClient(s.config, &s.client); err != nil {
			return nil, err
		}

		candidate, err := s.client.NewSession()
		if err != nil {
			closeSSHClient(&s.client)
			return nil, fmt.Errorf("failed to create SSH session: %w", err)
		}

		return candidate, nil
	})
}

func (s *SSHExecutor) Execute(command string) error {
	session, err := s.openSession()
	if err != nil {
		return err
	}
	defer session.Close()

	session.Stdout = os.Stdout
	session.Stderr = os.Stderr

	if err := session.Run(command); err != nil {
		return fmt.Errorf("ssh command failed: %w", err)
	}

	return nil
}

func (s *SSHExecutor) ExecuteBatch(commands []string) error {
	script := strings.Join(commands, " && ")
	return s.Execute(script)
}

type SFTPUploader struct {
	config connectionConfig
	client *ssh.Client
}

func NewSFTPUploader(server *config.ServerConfig) (*SFTPUploader, error) {
	return &SFTPUploader{
		config: newConnectionConfig(server),
	}, nil
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
