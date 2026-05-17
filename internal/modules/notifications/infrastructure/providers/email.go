package providers

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"
	"time"

	entities "erg.ninja/internal/modules/notifications/domain/entity"
	"erg.ninja/pkg/logger"
)

// EmailProvider delivers notifications via SMTP email.
type EmailProvider struct {
	log       *logger.Logger
	host      string
	port      int
	username  string
	password  string
	from      string
	tls       bool
	rateLimit time.Duration
}

// EmailProviderOption configures the EmailProvider.
type EmailProviderOption func(*EmailProvider)

// WithEmailLogger sets the logger.
func WithEmailLogger(log *logger.Logger) EmailProviderOption {
	return func(p *EmailProvider) { p.log = log }
}

// WithSMTPCredentials configures SMTP authentication.
func WithSMTPCredentials(host string, port int, username, password string) EmailProviderOption {
	return func(p *EmailProvider) {
		p.host = host
		p.port = port
		p.username = username
		p.password = password
	}
}

// WithFromAddress sets the sender email address.
func WithFromAddress(from string) EmailProviderOption {
	return func(p *EmailProvider) { p.from = from }
}

// WithTLS enables TLS for the SMTP connection.
func WithTLS(enabled bool) EmailProviderOption {
	return func(p *EmailProvider) { p.tls = enabled }
}

// NewEmailProvider creates a new Email notification provider.
func NewEmailProvider(opts ...EmailProviderOption) *EmailProvider {
	p := &EmailProvider{
		log:       logger.NoOp(),
		port:      587,
		tls:       true,
		rateLimit: time.Minute / 100, // 100 emails/min default
	}
	for _, o := range opts {
		o(p)
	}
	return p
}

// Name returns the provider name.
func (p *EmailProvider) Name() string { return "smtp-email" }

// Supports reports whether this provider handles email channels.
func (p *EmailProvider) Supports(channel entities.ChannelType) bool {
	return channel == entities.ChannelEmail
}

// RateLimit returns a default rate limit for email.
func (p *EmailProvider) RateLimit() (int, time.Duration) { return 100, time.Minute }

// Send delivers a notification via SMTP.
func (p *EmailProvider) Send(ctx context.Context, msg *entities.Notification) error {
	if p.from == "" {
		return fmt.Errorf("email: sender address not configured")
	}
	if msg.Recipient == "" {
		return fmt.Errorf("email: recipient address is required")
	}
	if msg.Subject == "" {
		return fmt.Errorf("email: subject is required")
	}

	addr := net.JoinHostPort(p.host, fmt.Sprintf("%d", p.port))

	headers := make(map[string]string)
	headers["From"] = p.from
	headers["To"] = msg.Recipient
	headers["Subject"] = msg.Subject
	headers["MIME-Version"] = "1.0"
	headers["Content-Type"] = "text/html; charset=utf-8"
	headers["Date"] = time.Now().Format(time.RFC1123Z)

	// Build body.
	var body strings.Builder
	for k, v := range headers {
		body.WriteString(k)
		body.WriteString(": ")
		body.WriteString(v)
		body.WriteString("\r\n")
	}
	body.WriteString("\r\n")

	if msg.HTMLBody != "" {
		body.WriteString(msg.HTMLBody)
	} else {
		// Plain-text fallback: escape HTML.
		body.WriteString("<html><body><pre>")
		body.WriteString(htmlEscape(msg.Body))
		body.WriteString("</pre></body></html>")
	}

	var auth smtp.Auth
	if p.username != "" {
		auth = smtp.PlainAuth("", p.username, p.password, p.host)
	}

	err := p.sendMail(ctx, addr, auth, p.from, []string{msg.Recipient}, []byte(body.String()))
	if err != nil {
		return fmt.Errorf("email: send: %w", err)
	}

	return nil
}

// sendMail sends an email using the SMTP server.
// It supports STARTTLS and plain TLS connections.
func (p *EmailProvider) sendMail(ctx context.Context, addr string, auth smtp.Auth, from string, to []string, body []byte) error {
	if p.tls && p.port == 465 {
		return p.sendTLS(ctx, addr, auth, from, to, body)
	}
	return p.sendSTARTTLS(ctx, addr, auth, from, to, body)
}

func (p *EmailProvider) sendTLS(ctx context.Context, addr string, auth smtp.Auth, from string, to []string, body []byte) error {
	tlsCfg := &tls.Config{
		ServerName: p.host,
	}

	conn, err := tls.Dial("tcp", addr, tlsCfg)
	if err != nil {
		return fmt.Errorf("email: TLS dial: %w", err)
	}

	client, err := smtp.NewClient(conn, p.host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("email: new client: %w", err)
	}
	defer client.Close()

	return p.sendViaClient(ctx, client, auth, from, to, body)
}

func (p *EmailProvider) sendSTARTTLS(ctx context.Context, addr string, auth smtp.Auth, from string, to []string, body []byte) error {
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return fmt.Errorf("email: dial: %w", err)
	}

	client, err := smtp.NewClient(conn, p.host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("email: new client: %w", err)
	}
	defer client.Close()

	// Send EHLO.
	if err := client.Hello("erg-server"); err != nil {
		return fmt.Errorf("email: EHLO: %w", err)
	}

	// Start TLS if available.
	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsCfg := &tls.Config{ServerName: p.host}
		if err := client.StartTLS(tlsCfg); err != nil {
			return fmt.Errorf("email: STARTTLS: %w", err)
		}
	}

	// Authenticate if credentials provided.
	if auth != nil {
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("email: auth: %w", err)
		}
	}

	return p.sendViaClient(ctx, client, auth, from, to, body)
}

func (p *EmailProvider) sendViaClient(ctx context.Context, client *smtp.Client, auth smtp.Auth, from string, to []string, body []byte) error {
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("email: MAIL FROM: %w", err)
	}
	for _, addr := range to {
		if err := client.Rcpt(addr); err != nil {
			return fmt.Errorf("email: RCPT TO: %w", err)
		}
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("email: DATA: %w", err)
	}
	if _, err := w.Write(body); err != nil {
		return fmt.Errorf("email: write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("email: close body: %w", err)
	}
	return client.Quit()
}

// htmlEscape converts plain text to minimal HTML-escaped text.
func htmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\"", "&quot;")
	return s
}
