// Package mailer sends transactional emails. When SMTP is not configured it
// logs the message (including any links) so flows still work in development.
package mailer

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/smtp"
	"strings"

	"github.com/guilherme/help-party/internal/config"
)

type Mailer struct {
	cfg config.Config
}

func New(cfg config.Config) *Mailer { return &Mailer{cfg: cfg} }

// Send delivers a plain-text email. If SMTP is unconfigured, it logs instead.
func (m *Mailer) Send(to, subject, body string) error {
	if m.cfg.SMTPHost == "" {
		log.Printf("[mailer:dev] (SMTP não configurado) email para %s\n  assunto: %s\n  %s",
			to, subject, strings.ReplaceAll(body, "\n", "\n  "))
		return nil
	}

	addr := m.cfg.SMTPHost + ":" + m.cfg.SMTPPort
	msg := buildMessage(m.cfg.SMTPFrom, to, subject, body)

	var auth smtp.Auth
	if m.cfg.SMTPUser != "" {
		auth = smtp.PlainAuth("", m.cfg.SMTPUser, m.cfg.SMTPPass, m.cfg.SMTPHost)
	}

	// Envelope sender must be the authenticated bare address (providers like
	// Hostinger reject a mismatched MAIL FROM); the header From may carry a name.
	envelopeFrom := m.cfg.SMTPUser
	if envelopeFrom == "" {
		envelopeFrom = m.cfg.SMTPFrom
	}

	// Port 465 uses implicit TLS (SMTPS); 587/25 use STARTTLS via smtp.SendMail.
	if m.cfg.SMTPPort == "465" {
		return m.sendImplicitTLS(addr, auth, envelopeFrom, to, msg)
	}
	if err := smtp.SendMail(addr, auth, envelopeFrom, []string{to}, msg); err != nil {
		return fmt.Errorf("send mail: %w", err)
	}
	return nil
}

// sendImplicitTLS delivers over a TLS-from-start connection (port 465).
func (m *Mailer) sendImplicitTLS(addr string, auth smtp.Auth, envelopeFrom, to string, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: m.cfg.SMTPHost})
	if err != nil {
		return fmt.Errorf("tls dial: %w", err)
	}
	c, err := smtp.NewClient(conn, m.cfg.SMTPHost)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer c.Close()
	if auth != nil {
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
	}
	if err := c.Mail(envelopeFrom); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	if err := c.Rcpt(to); err != nil {
		return fmt.Errorf("rcpt to: %w", err)
	}
	wc, err := c.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	if _, err := wc.Write(msg); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("close data: %w", err)
	}
	return c.Quit()
}

func buildMessage(from, to, subject, body string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(body)
	return []byte(b.String())
}
