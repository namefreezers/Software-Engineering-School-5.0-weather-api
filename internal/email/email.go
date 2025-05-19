package email

import (
	"crypto/tls"
	"fmt"
	"github.com/namefreezers/Software-Engineering-School-5.0-weather-api/internal/config"
	"net"
	"net/smtp"
	"strings"
	"time"

	"go.uber.org/zap"
)

// EmailMessage represents a single email to be sent.
type EmailMessage struct {
	To      []string // Recipient email addresses.
	Subject string   // Email subject.
	Body    string   // HTML or plain text email content.
}

// EmailSender defines an interface for sending batches of emails.
type EmailSender interface {
	// SendBatch sends multiple EmailMessage objects in a single SMTP session.
	SendBatch(messages []EmailMessage) error
}

// SMTPSender is a concrete implementation of EmailSender using SMTP.
type SMTPSender struct {
	host      string
	port      int
	from      string
	auth      smtp.Auth
	tlsConfig *tls.Config
	cfg       *config.Config
	logger    *zap.Logger
}

// NewSMTPSender reads SMTP configuration from environment variables and returns an SMTPSender.
// Required environment variables:
//
//	SMTP_HOST: e.g. smtp.example.com
//	SMTP_PORT: e.g. 587 or 465
//	SMTP_USER: username for SMTP auth
//	SMTP_PASS: password for SMTP auth
//	SMTP_FROM: optional; defaults to SMTP_USER if unset
func NewSMTPSender(cfg *config.Config, logger *zap.Logger) (*SMTPSender, error) {

	auth := smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost)
	tlsConfig := &tls.Config{ServerName: cfg.SMTPHost}

	return &SMTPSender{
		host:      cfg.SMTPHost,
		port:      cfg.SMTPPort,
		from:      cfg.SMTPUser,
		auth:      auth,
		tlsConfig: tlsConfig,
		logger:    logger,
	}, nil
}

// createClient encapsulates dialing and setting up an SMTP client connection.
// It handles both implicit TLS (port 465) and STARTTLS (other ports).
func (s *SMTPSender) createClient() (*smtp.Client, error) {
	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	var conn net.Conn
	var err error

	if s.port == 465 {
		// Implicit TLS
		conn, err = tls.Dial("tcp", addr, s.tlsConfig)
		if err != nil {
			s.logger.Error("failed to dial SMTPS", zap.String("addr", addr), zap.Error(err))
			return nil, fmt.Errorf("failed to dial SMTPS on %s: %w", addr, err)
		}
	} else {
		// Plain TCP, we'll upgrade via STARTTLS
		conn, err = net.Dial("tcp", addr)
		if err != nil {
			s.logger.Error("failed to dial SMTP", zap.String("addr", addr), zap.Error(err))
			return nil, fmt.Errorf("failed to dial SMTP on %s: %w", addr, err)
		}
	}

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		// close the underlying connection
		if cerr := conn.Close(); cerr != nil {
			s.logger.Warn("failed to close raw connection", zap.Error(cerr))
		}
		s.logger.Error("failed to create SMTP client", zap.Error(err))
		return nil, fmt.Errorf("failed to create SMTP client: %w", err)
	}

	// STARTTLS upgrade if not implicit TLS
	if s.port != 465 {
		if ok, _ := client.Extension("STARTTLS"); !ok {
			if cerr := client.Close(); cerr != nil {
				s.logger.Warn("failed to close SMTP client after missing STARTTLS", zap.Error(cerr))
			}
			err := fmt.Errorf("SMTP server does not support STARTTLS")
			s.logger.Error("STARTTLS not supported", zap.Error(err))
			return nil, err
		}
		if err := client.StartTLS(s.tlsConfig); err != nil {
			if cerr := client.Close(); cerr != nil {
				s.logger.Warn("failed to close SMTP client after STARTTLS failure", zap.Error(cerr))
			}
			s.logger.Error("failed to start TLS", zap.Error(err))
			return nil, fmt.Errorf("failed to start TLS: %w", err)
		}
	}

	return client, nil
}

// SendBatch opens a single SMTP session and sends all provided emails sequentially.
func (s *SMTPSender) SendBatch(messages []EmailMessage) (err error) {
	client, err := s.createClient()
	if err != nil {
		return err
	}
	// ensure QUIT is sent and connection closed
	defer func() {
		if quitErr := client.Quit(); quitErr != nil && err == nil {
			s.logger.Error("failed to close SMTP connection", zap.Error(quitErr))
			err = fmt.Errorf("failed to close SMTP connection: %w", quitErr)
		}
	}()

	// Authenticate once per session
	if err := client.Auth(s.auth); err != nil {
		s.logger.Error("SMTP authentication failed", zap.Error(err))
		return fmt.Errorf("failed to authenticate: %w", err)
	}

	// Send each message, resetting the envelope between them
	for _, msg := range messages {
		if err := client.Reset(); err != nil {
			s.logger.Error("failed to reset SMTP session", zap.Error(err))
			return fmt.Errorf("failed to reset SMTP session: %w", err)
		}
		if err := s.send(client, msg); err != nil {
			return err
		}
	}

	s.logger.Info("all messages sent successfully", zap.Int("count", len(messages)))
	return nil
}

// send sends a single EmailMessage using an existing SMTP client session.
func (s *SMTPSender) send(client *smtp.Client, m EmailMessage) error {
	// MAIL FROM
	if err := client.Mail(s.from); err != nil {
		s.logger.Error("MAIL FROM failed", zap.String("from", s.from), zap.Error(err))
		return fmt.Errorf("failed to set MAIL FROM: %w", err)
	}
	// RCPT TO
	for _, addr := range m.To {
		if err := client.Rcpt(addr); err != nil {
			s.logger.Error("RCPT TO failed", zap.String("to", addr), zap.Error(err))
			return fmt.Errorf("failed to add RCPT TO %q: %w", addr, err)
		}
	}
	// DATA
	wc, err := client.Data()
	if err != nil {
		s.logger.Error("DATA command failed", zap.Error(err))
		return fmt.Errorf("failed to start DATA command: %w", err)
	}

	// Build headers
	headers := []string{
		fmt.Sprintf("Date: %s", time.Now().Format(time.RFC1123Z)),
		fmt.Sprintf("From: %s", s.from),
		fmt.Sprintf("To: %s", strings.Join(m.To, ",")),
		fmt.Sprintf("Subject: %s", m.Subject),
		"MIME-Version: 1.0",
		`Content-Type: text/html; charset="utf-8"`,
	}
	fullMessage := strings.Join(headers, "\r\n") + "\r\n\r\n" + m.Body

	// Write body
	if _, writeErr := wc.Write([]byte(fullMessage)); writeErr != nil {
		// handle Close() error
		if cErr := wc.Close(); cErr != nil {
			s.logger.Warn("failed to close DATA writer after write error", zap.Error(cErr))
		}
		s.logger.Error("failed to write message body", zap.Error(writeErr))
		return fmt.Errorf("failed to write message body: %w", writeErr)
	}
	// Close DATA writer (and handle its error)
	if cErr := wc.Close(); cErr != nil {
		s.logger.Error("failed to close DATA writer", zap.Error(cErr))
		return fmt.Errorf("failed to close DATA writer: %w", cErr)
	}

	s.logger.Debug("email sent", zap.Strings("to", m.To), zap.String("subject", m.Subject))
	return nil
}
