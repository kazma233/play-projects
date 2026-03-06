package deploy

import (
	"context"
	"fmt"
	"os"
	"strings"

	"deploygo/internal/config"
	"deploygo/internal/retry"

	"golang.org/x/crypto/ssh"
)

type SSHExecutor struct {
	config connectionConfig
	client *ssh.Client
}

func NewSSHExecutor(server *config.ServerConfig) *SSHExecutor {
	return &SSHExecutor{
		config: newConnectionConfig(server),
	}
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
