package mailer_test

import (
	"context"
	"testing"

	"github.com/whinchman/jobhuntr/internal/mailer"
)

func TestNoopMailer_SendMailReturnsNil(t *testing.T) {
	n := &mailer.NoopMailer{}
	err := n.SendMail(context.Background(), "user@example.com", "Hello", "World")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestNoopMailer_SendMailEmptyInputs(t *testing.T) {
	n := &mailer.NoopMailer{}
	err := n.SendMail(context.Background(), "", "", "")
	if err != nil {
		t.Fatalf("expected nil error on empty inputs, got %v", err)
	}
}

func TestNoopMailer_ImplementsMailerInterface(t *testing.T) {
	var _ mailer.Mailer = &mailer.NoopMailer{}
}

func TestSMTPMailer_ImplementsMailerInterface(t *testing.T) {
	var _ mailer.Mailer = mailer.NewSMTPMailer("smtp.example.com", 587, "user", "pass", "from@example.com")
}

func TestNewSMTPMailer_ReturnsNonNil(t *testing.T) {
	m := mailer.NewSMTPMailer("smtp.example.com", 587, "user@example.com", "secret", "noreply@example.com")
	if m == nil {
		t.Fatal("expected non-nil *SMTPMailer")
	}
}

func TestNewSMTPMailer_Port465(t *testing.T) {
	m := mailer.NewSMTPMailer("smtp.example.com", 465, "user@example.com", "secret", "noreply@example.com")
	if m == nil {
		t.Fatal("expected non-nil *SMTPMailer for port 465")
	}
}

func TestNoopMailer_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled

	n := &mailer.NoopMailer{}
	// NoopMailer should still return nil regardless of context state
	err := n.SendMail(ctx, "user@example.com", "Subject", "Body")
	if err != nil {
		t.Fatalf("expected nil error even with cancelled context, got %v", err)
	}
}
