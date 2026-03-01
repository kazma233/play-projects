package auth

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"html/template"
	"log/slog"
	"math/big"
	"net"
	"net/smtp"
	"path/filepath"
	"strconv"
	"strings"

	"picstash/internal/config"
)

type EmailService struct {
	host     string
	port     int
	username string
	password string
	from     string
	fromName string
	isDev    bool
}

type VerificationEmailData struct {
	Code      string
	ExpiresIn int
}

func NewEmailService(cfg *config.Config) *EmailService {
	port, _ := strconv.Atoi(cfg.SMTP.Port)
	return &EmailService{
		host:     cfg.SMTP.Host,
		port:     port,
		username: cfg.SMTP.Username,
		password: cfg.SMTP.Password,
		from:     cfg.SMTP.From,
		fromName: cfg.SMTP.FromName,
		isDev:    cfg.Server.Mode == "debug",
	}
}

func (e *EmailService) SendVerificationCode(to, code string, expiresIn int) error {
	if e.isDev {
		slog.Info("[开发模式] 验证码已记录", "to", to, "code", code, "expires_in", expiresIn)
		return nil
	}

	tmplPath, err := filepath.Abs("templates/emails/verification.html")
	if err != nil {
		return fmt.Errorf("获取模板路径失败: %w", err)
	}

	tmpl, err := template.ParseFiles(tmplPath)
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
	msg := e.buildHTMLMail(to, "验证码", htmlBody, code)

	return e.sendMail(to, []byte(msg))
}

// sendMail 通用的邮件发送方法，自动处理加密
func (e *EmailService) sendMail(to string, msg []byte) error {
	addr := fmt.Sprintf("%s:%d", e.host, e.port)

	// 465端口使用SMTPS（直接TLS连接）
	if e.port == 465 {
		return e.sendWithTLS(addr, to, msg, true)
	}

	// 其他端口（如587）尝试STARTTLS
	return e.sendWithTLS(addr, to, msg, false)
}

// sendWithTLS 统一的发送方法
// directTLS=true:  直接建立TLS连接（SMTPS，用于465端口）
// directTLS=false: 先建立TCP连接，然后尝试STARTTLS升级（用于587端口）
func (e *EmailService) sendWithTLS(addr, to string, msg []byte, directTLS bool) error {
	var client *smtp.Client

	if directTLS {
		// 直接TLS连接（SMTPS）
		tlsConfig := &tls.Config{ServerName: e.host}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("TLS连接失败: %w", err)
		}
		defer conn.Close()

		client, err = smtp.NewClient(conn, e.host)
		if err != nil {
			return fmt.Errorf("创建SMTP客户端失败: %w", err)
		}
	} else {
		// 普通TCP连接，然后尝试STARTTLS
		conn, err := net.Dial("tcp", addr)
		if err != nil {
			return fmt.Errorf("TCP连接失败: %w", err)
		}
		defer conn.Close()

		client, err = smtp.NewClient(conn, e.host)
		if err != nil {
			return fmt.Errorf("创建SMTP客户端失败: %w", err)
		}

		// 发送EHLO
		if err := client.Hello("localhost"); err != nil {
			client.Close()
			return fmt.Errorf("EHLO失败: %w", err)
		}

		// 如果服务器支持STARTTLS，则升级连接
		if ok, _ := client.Extension("STARTTLS"); ok {
			tlsConfig := &tls.Config{ServerName: e.host}
			if err := client.StartTLS(tlsConfig); err != nil {
				client.Close()
				return fmt.Errorf("STARTTLS升级失败: %w", err)
			}
		}
	}

	defer client.Close()

	// 执行认证和发送
	return e.authAndSend(client, to, msg)
}

// authAndSend 执行认证并发送邮件
func (e *EmailService) authAndSend(client *smtp.Client, to string, msg []byte) error {
	// 认证
	auth := smtp.PlainAuth("", e.username, e.password, e.host)
	if err := client.Auth(auth); err != nil {
		// 如果PLAIN认证失败，尝试LOGIN认证（某些邮件服务商需要）
		if !strings.Contains(err.Error(), "unsupport") {
			return fmt.Errorf("认证失败: %w", err)
		}
		if err := e.loginAuth(client); err != nil {
			return err
		}
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

// loginAuth LOGIN认证方式（用于部分不支持PLAIN认证的服务商）
func (e *EmailService) loginAuth(client *smtp.Client) error {
	conn := client.Text

	// 发送AUTH LOGIN
	id, err := conn.Cmd("%s", "AUTH LOGIN")
	if err != nil {
		return fmt.Errorf("发送AUTH LOGIN失败: %w", err)
	}
	conn.StartResponse(id)
	_, _, err = conn.ReadResponse(334)
	conn.EndResponse(id)
	if err != nil {
		return fmt.Errorf("等待用户名提示失败: %w", err)
	}

	// 发送用户名
	id, err = conn.Cmd("%s", base64.StdEncoding.EncodeToString([]byte(e.username)))
	if err != nil {
		return fmt.Errorf("发送用户名失败: %w", err)
	}
	conn.StartResponse(id)
	_, _, err = conn.ReadResponse(334)
	conn.EndResponse(id)
	if err != nil {
		return fmt.Errorf("等待密码提示失败: %w", err)
	}

	// 发送密码
	id, err = conn.Cmd("%s", base64.StdEncoding.EncodeToString([]byte(e.password)))
	if err != nil {
		return fmt.Errorf("发送密码失败: %w", err)
	}
	conn.StartResponse(id)
	_, _, err = conn.ReadResponse(235)
	conn.EndResponse(id)
	if err != nil {
		return fmt.Errorf("认证失败: %w", err)
	}

	return nil
}

func (e *EmailService) buildHTMLMail(to, subject, htmlBody, code string) string {
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "From: %s <%s>\r\n", e.fromName, e.from)
	fmt.Fprintf(&buf, "To: %s\r\n", to)
	fmt.Fprintf(&buf, "Subject: %s\r\n", subject)
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")

	boundary := "boundary_" + randString(16)
	fmt.Fprintf(&buf, "Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary)
	fmt.Fprintf(&buf, "\r\n")

	// 纯文本部分
	fmt.Fprintf(&buf, "--%s\r\n", boundary)
	fmt.Fprintf(&buf, "Content-Type: text/plain; charset=utf-8\r\n")
	fmt.Fprintf(&buf, "Content-Transfer-Encoding: base64\r\n\r\n")
	buf.WriteString(base64.StdEncoding.EncodeToString([]byte("您的验证码是："+code)) + "\r\n")

	// HTML部分
	fmt.Fprintf(&buf, "--%s\r\n", boundary)
	fmt.Fprintf(&buf, "Content-Type: text/html; charset=utf-8\r\n")
	fmt.Fprintf(&buf, "Content-Transfer-Encoding: base64\r\n\r\n")
	buf.WriteString(base64.StdEncoding.EncodeToString([]byte(htmlBody)) + "\r\n")

	fmt.Fprintf(&buf, "--%s--\r\n", boundary)

	return buf.String()
}

func randString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		nBig, _ := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		b[i] = letters[nBig.Int64()]
	}
	return string(b)
}

func GenerateCode() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	return fmt.Sprintf("%06d", n.Int64())
}
