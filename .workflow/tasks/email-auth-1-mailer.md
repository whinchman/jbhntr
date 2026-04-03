# Task: email-auth-1-mailer

- **Type**: coder
- **Status**: pending
- **Repo**: .
- **Parallel Group**: 1
- **Branch**: feature/email-auth-1-mailer
- **Source Item**: email-auth (email/password authentication)
- **Dependencies**: none

## Description

Create the `internal/mailer` package: a thin SMTP wrapper with a `Mailer` interface and a `NoopMailer` for dev/test. This package has no dependencies on anything else in the email-auth feature — it can be implemented in parallel with the migration and config tasks.

Two files: `internal/mailer/mailer.go` and `internal/mailer/mailer_test.go`.

## Acceptance Criteria

- [ ] `internal/mailer/mailer.go` exists with `Mailer` interface, `SMTPMailer`, and `NoopMailer`
- [ ] `NewSMTPMailer` constructor exists with the correct signature
- [ ] `SMTPMailer.SendMail` dials STARTTLS (port 587) or direct TLS (port 465), authenticates with AUTH PLAIN, and sends a valid RFC 2822 message
- [ ] `NoopMailer.SendMail` returns nil without sending anything
- [ ] Unit tests in `internal/mailer/mailer_test.go` cover `NoopMailer` and basic struct construction
- [ ] `go test ./internal/mailer/...` passes

## Interface Contracts

No cross-package dependencies within the email-auth feature. The `EmailSender` interface used by the `Server` struct (defined in `internal/web/server.go` by the Group 2 task) will simply mirror the `Mailer` interface — same single method `SendMail(ctx, to, subject, body) error`. The `internal/mailer` package is consumed by `main.go`; the handler code uses the `EmailSender` interface from `internal/web/server.go` (which `*SMTPMailer` and `*NoopMailer` will satisfy automatically since it has the same method signature).

## Context

### Package layout

```
internal/mailer/
  mailer.go
  mailer_test.go
```

### Types and signatures

```go
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
    host     string  // e.g. "smtp.mailgun.org"
    port     int     // 587 = STARTTLS, 465 = TLS
    username string
    password string
    from     string  // e.g. "noreply@jobhuntr.example.com"
}

func NewSMTPMailer(host string, port int, username, password, from string) *SMTPMailer

func (m *SMTPMailer) SendMail(ctx context.Context, to, subject, body string) error
// Port 587: net.Dial → smtp.NewClient → STARTTLS → AUTH PLAIN → send
// Port 465: tls.Dial → smtp.NewClient → AUTH PLAIN → send
// Message format: minimal RFC 2822 headers (From, To, Subject, MIME-Version, Content-Type, body)

// NoopMailer drops all emails — used in tests and when SMTP is not configured.
type NoopMailer struct{}

func (n *NoopMailer) SendMail(ctx context.Context, to, subject, body string) error {
    return nil
}
```

### SendMail implementation notes

For port 587 (STARTTLS):
1. `net.Dial("tcp", host+":587")`
2. `smtp.NewClient(conn, host)`
3. `client.StartTLS(&tls.Config{ServerName: host})`
4. `client.Auth(smtp.PlainAuth("", username, password, host))`
5. `client.Mail(from)`, `client.Rcpt(to)`, `client.Data()` — write headers + body

For port 465 (TLS):
1. `tls.Dial("tcp", host+":465", &tls.Config{ServerName: host})`
2. `smtp.NewClient(tlsConn, host)`
3. `client.Auth(smtp.PlainAuth("", username, password, host))`
4. Same send sequence as above

Message body format (write to `w` returned by `client.Data()`):
```
From: <from>
To: <to>
Subject: <subject>
MIME-Version: 1.0
Content-Type: text/plain; charset=UTF-8

<body>
```

### Tests

Unit tests should:
- Test that `NoopMailer.SendMail` returns nil for any input
- Test that `NewSMTPMailer` stores the constructor arguments (accessible via a simple Send call to a mock or by testing the struct fields if exported in test)
- Do NOT attempt a live SMTP connection in unit tests (that's an integration concern)

Use `mailer_test.go` in package `mailer_test` (external test package) for black-box testing.

## Notes

<!-- implementing agent fills this in -->
