package mailer

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
)

// Mailer sends transactional emails.
type Mailer interface {
	SendMail(ctx context.Context, to, subject, body string) error
}

// SMTPMailer implements Mailer using authenticated SMTP.
type SMTPMailer struct {
	host     string // e.g. "smtp.mailgun.org"
	port     int    // 587 = STARTTLS, 465 = TLS
	username string
	password string
	from     string // e.g. "noreply@jobhuntr.example.com"
}

// NewSMTPMailer creates a new SMTPMailer with the given SMTP credentials.
func NewSMTPMailer(host string, port int, username, password, from string) *SMTPMailer {
	return &SMTPMailer{
		host:     host,
		port:     port,
		username: username,
		password: password,
		from:     from,
	}
}

// SendMail sends an email via authenticated SMTP.
// Port 587 uses STARTTLS; port 465 uses direct TLS.
func (m *SMTPMailer) SendMail(ctx context.Context, to, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", m.host, m.port)

	var client *smtp.Client
	var err error

	if m.port == 465 {
		// Direct TLS connection
		tlsConn, dialErr := tls.Dial("tcp", addr, &tls.Config{ServerName: m.host})
		if dialErr != nil {
			return fmt.Errorf("mailer: tls dial %s: %w", addr, dialErr)
		}
		client, err = smtp.NewClient(tlsConn, m.host)
		if err != nil {
			return fmt.Errorf("mailer: smtp client (TLS): %w", err)
		}
	} else {
		// STARTTLS connection (default: port 587)
		conn, dialErr := net.Dial("tcp", addr)
		if dialErr != nil {
			return fmt.Errorf("mailer: dial %s: %w", addr, dialErr)
		}
		client, err = smtp.NewClient(conn, m.host)
		if err != nil {
			return fmt.Errorf("mailer: smtp client (STARTTLS): %w", err)
		}
		if err = client.StartTLS(&tls.Config{ServerName: m.host}); err != nil {
			return fmt.Errorf("mailer: starttls: %w", err)
		}
	}
	defer client.Quit() //nolint:errcheck

	if err = client.Auth(smtp.PlainAuth("", m.username, m.password, m.host)); err != nil {
		return fmt.Errorf("mailer: auth: %w", err)
	}

	if err = client.Mail(m.from); err != nil {
		return fmt.Errorf("mailer: MAIL FROM: %w", err)
	}
	if err = client.Rcpt(to); err != nil {
		return fmt.Errorf("mailer: RCPT TO: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("mailer: DATA: %w", err)
	}
	defer w.Close() //nolint:errcheck

	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		m.from, to, subject, body,
	)
	if _, err = fmt.Fprint(w, msg); err != nil {
		return fmt.Errorf("mailer: write message: %w", err)
	}

	return nil
}

// NoopMailer drops all emails — used in tests and when SMTP is not configured.
type NoopMailer struct{}

// SendMail does nothing and returns nil.
func (n *NoopMailer) SendMail(_ context.Context, _, _, _ string) error {
	return nil
}
