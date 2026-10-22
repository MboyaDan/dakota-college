// Package mail sends transactional email. Locally the SMTP server is Mailpit
// (http://localhost:8025); production uses a provider with a verified domain
// (brief 12.3, DG-03).
package mail

import (
	"context"
	"fmt"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Message struct {
	To      string
	Subject string
	Text    string
}

type Mailer interface {
	Send(ctx context.Context, m Message) error
}

// SMTP sends plain-text mail. net/smtp upgrades to TLS with STARTTLS when the
// server offers it, and authenticates only when a username is set.
type SMTP struct {
	Host, Username, Password, From string
	Port                           int
}

// SenderName is the display name on every email.
const SenderName = "Dakota Training College"

func (s SMTP) Send(ctx context.Context, m Message) error {
	addr, err := mail.ParseAddress(s.From)
	if err != nil {
		return fmt.Errorf("SMTP_FROM: %w", err)
	}
	from := &mail.Address{Name: SenderName, Address: addr.Address}
	to, err := mail.ParseAddress(m.To)
	if err != nil {
		return fmt.Errorf("recipient: %w", err)
	}
	// Header injection guard: subjects come from code, but check anyway.
	if strings.ContainsAny(m.Subject, "\r\n") {
		return fmt.Errorf("subject contains a line break")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from.String())
	fmt.Fprintf(&b, "To: %s\r\n", to.String())
	fmt.Fprintf(&b, "Subject: %s\r\n", m.Subject)
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	b.WriteString("MIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n")
	b.WriteString(strings.ReplaceAll(m.Text, "\n", "\r\n"))

	var auth smtp.Auth
	if s.Username != "" {
		auth = smtp.PlainAuth("", s.Username, s.Password, s.Host)
	}
	server := net.JoinHostPort(s.Host, strconv.Itoa(s.Port))

	done := make(chan error, 1)
	go func() { done <- smtp.SendMail(server, auth, from.Address, []string{to.Address}, []byte(b.String())) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Recorder keeps messages in memory, for tests.
type Recorder struct {
	mu       sync.Mutex
	Messages []Message
}

func (r *Recorder) Send(_ context.Context, m Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Messages = append(r.Messages, m)
	return nil
}

func (r *Recorder) All() []Message {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Message(nil), r.Messages...)
}
