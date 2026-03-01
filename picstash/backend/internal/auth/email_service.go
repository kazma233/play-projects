package auth

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"html/template"
	"log/slog"
	"net"
	"net/smtp"
	"os"
	"strconv"
	"time"

	_ "embed"

	"picstash/internal/config"
)

//go:embed templates/emails/verification.html
var verificationHTML string

// EmailService 提供邮件发送功能
type EmailService struct {
	host          string
	port          int
	username      string
	password      string
	from          string
	fromName      string
	isDev         bool
	timeout       time.Duration
	skipTLSVerify bool
}

// VerificationEmailData 验证码邮件模板数据
type VerificationEmailData struct {
	Code      string
	ExpiresIn int
}

// NewEmailService 创建邮件服务实例
func NewEmailService(cfg *config.Config) *EmailService {
	return &EmailService{
		host:          cfg.SMTP.Host,
		port:          cfg.SMTP.Port,
		username:      cfg.SMTP.Username,
		password:      cfg.SMTP.Password,
		from:          cfg.SMTP.From,
		fromName:      cfg.SMTP.FromName,
		isDev:         cfg.Server.Mode == "debug",
		timeout:       10 * time.Second,
		skipTLSVerify: cfg.SMTP.SkipTLSVerify,
	}
}

// SendVerificationCode 发送验证码邮件
func (e *EmailService) SendVerificationCode(to, code string, expiresIn int) error {
	if e.isDev {
		slog.Info("[开发模式] 验证码已记录", "to", to, "code", code, "expires_in", expiresIn)
		return nil
	}

	tmpl, err := template.New("verification").Parse(verificationHTML)
	if err != nil {
		return fmt.Errorf("解析邮件模板失败: %w", err)
	}

	data := VerificationEmailData{
		Code:      code,
		ExpiresIn: expiresIn,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("执行邮件模板失败: %w", err)
	}

	htmlBody := buf.String()
	plainText := fmt.Sprintf("您的验证码是：%s，有效期%d分钟。如非本人操作，请忽略。", code, expiresIn)
	msg := e.buildHTMLMail(to, "验证码", htmlBody, plainText)

	return e.send(to, msg)
}

// send 自动选择发送方式
func (e *EmailService) send(to string, msg []byte) error {
	addr := net.JoinHostPort(e.host, strconv.Itoa(e.port))

	slog.Info("尝试连接SMTP服务器", "addr", addr, "host", e.host)

	dialer := &net.Dialer{Timeout: e.timeout}

	conn, err := dialer.Dial("tcp", addr)
	if err != nil {
		return fmt.Errorf("TCP连接失败: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, e.host)
	if err != nil {
		return fmt.Errorf("创建SMTP客户端失败: %w", err)
	}
	defer client.Close()

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "localhost"
	}
	if err := client.Hello(hostname); err != nil {
		return fmt.Errorf("EHLO失败: %w", err)
	}

	slog.Info("检查服务器STARTTLS支持")

	if ok, _ := client.Extension("STARTTLS"); ok {
		slog.Info("服务器支持STARTTLS，使用STARTTLS方式")
		return e.sendWithSTARTTLSUpgrade(client, to, msg)
	}

	if e.port == 465 || e.port == 994 {
		slog.Info("服务器不支持STARTTLS，尝试直接TLS连接")
		return e.sendWithDirectTLS(to, msg)
	}

	return fmt.Errorf("服务器不支持STARTTLS，且端口非SMTPS端口(465/994)")
}

// sendWithSTARTTLSUpgrade 使用已建立的连接升级TLS
func (e *EmailService) sendWithSTARTTLSUpgrade(client *smtp.Client, to string, msg []byte) error {
	tlsConfig := &tls.Config{
		ServerName:         e.host,
		InsecureSkipVerify: e.skipTLSVerify,
		MinVersion:         tls.VersionTLS12,
	}
	if err := client.StartTLS(tlsConfig); err != nil {
		return fmt.Errorf("STARTTLS升级失败: %w", err)
	}

	slog.Info("TLS已升级，开始认证")
	return e.doAuthAndSend(client, to, msg)
}

// sendWithDirectTLS 关闭明文连接，重新建立直接TLS连接
func (e *EmailService) sendWithDirectTLS(to string, msg []byte) error {
	addr := net.JoinHostPort(e.host, strconv.Itoa(e.port))

	slog.Info("建立直接TLS连接", "addr", addr)

	dialer := &net.Dialer{Timeout: e.timeout}

	tlsConfig := &tls.Config{
		ServerName:         e.host,
		InsecureSkipVerify: e.skipTLSVerify,
		MinVersion:         tls.VersionTLS12,
	}

	conn, err := tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
	if err != nil {
		return fmt.Errorf("TLS连接失败: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, e.host)
	if err != nil {
		return fmt.Errorf("创建SMTP客户端失败: %w", err)
	}
	defer client.Close()

	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "localhost"
	}
	if err := client.Hello(hostname); err != nil {
		return fmt.Errorf("EHLO失败: %w", err)
	}

	slog.Info("SMTP客户端已创建，开始认证")
	return e.doAuthAndSend(client, to, msg)
}

// doAuthAndSend 执行认证并发送邮件
func (e *EmailService) doAuthAndSend(client *smtp.Client, to string, msg []byte) error {
	// PLAIN认证
	auth := smtp.PlainAuth("", e.username, e.password, e.host)
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("认证失败: %w", err)
	}

	// 设置发件人
	if err := client.Mail(e.from); err != nil {
		return fmt.Errorf("设置发件人失败: %w", err)
	}

	// 设置收件人
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("设置收件人失败: %w", err)
	}

	// 写入邮件内容
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("获取数据写入器失败: %w", err)
	}

	if _, err := w.Write(msg); err != nil {
		return fmt.Errorf("写入邮件内容失败: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("关闭数据写入器失败: %w", err)
	}

	return client.Quit()
}

// buildHTMLMail 构建多部分MIME邮件
func (e *EmailService) buildHTMLMail(to, subject, htmlBody, plainText string) []byte {
	var buf bytes.Buffer

	buf.WriteString(fmt.Sprintf("From: %s <%s>\r\n", e.fromName, e.from))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", to))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	buf.WriteString("MIME-Version: 1.0\r\n")

	boundary := generateBoundary()
	buf.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary))
	buf.WriteString("\r\n")

	buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	buf.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	buf.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
	buf.WriteString(base64.StdEncoding.EncodeToString([]byte(plainText)))
	buf.WriteString("\r\n")

	buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	buf.WriteString("Content-Type: text/html; charset=utf-8\r\n")
	buf.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
	buf.WriteString(base64.StdEncoding.EncodeToString([]byte(htmlBody)))
	buf.WriteString("\r\n")

	buf.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	return buf.Bytes()
}

func generateBoundary() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("boundary_%x", b)
}
