package deploy

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"deploygo/internal/config"
	"deploygo/internal/retry"

	"golang.org/x/crypto/ssh"
)

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
