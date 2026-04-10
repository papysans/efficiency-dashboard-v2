package notify

import (
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/smtp"
)

// SendEmail 通过 SMTP 发送纯文本邮件
// smtpAddr 或 to 为空时静默跳过，返回 nil
// port 465 走 SSL 直连，其他端口（587/25）走 STARTTLS
func SendEmail(smtpAddr, username, password, from, to, subject, body string) error {
	if smtpAddr == "" || to == "" {
		return nil
	}

	host, port, err := net.SplitHostPort(smtpAddr)
	if err != nil {
		return fmt.Errorf("解析 SMTP 地址失败 %q: %w", smtpAddr, err)
	}

	msg := "From: " + from + "\r\n" +
		"To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n" +
		"\r\n" +
		body

	if port == "465" {
		tlsConn, err := tls.Dial("tcp", smtpAddr, &tls.Config{ServerName: host})
		if err != nil {
			return fmt.Errorf("SMTP SSL连接失败 %s: %w", smtpAddr, err)
		}

		client, err := smtp.NewClient(tlsConn, host)
		if err != nil {
			return err
		}

		if err = client.Auth(smtp.PlainAuth("", username, password, host)); err != nil {
			return err
		}

		if err = client.Mail(from); err != nil {
			return err
		}
		if err = client.Rcpt(to); err != nil {
			return err
		}

		var w io.WriteCloser
		w, err = client.Data()
		if err != nil {
			return err
		}
		fmt.Fprint(w, msg)
		w.Close()
		client.Quit()
		return nil
	}

	if err := smtp.SendMail(smtpAddr, smtp.PlainAuth("", username, password, host), from, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("SMTP发送失败: %w", err)
	}
	return nil
}
