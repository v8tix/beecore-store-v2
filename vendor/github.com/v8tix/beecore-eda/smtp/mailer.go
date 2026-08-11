// Package smtp provides email sending functionality
package smtp

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/wneessen/go-mail"
)

var (
	// ErrMailClientCreationFailed indicates that creating mail client failed.
	ErrMailClientCreationFailed = errors.New("mail client creation failed")
	// ErrRecipientSetFailed indicates that setting recipient failed.
	ErrRecipientSetFailed = errors.New("recipient set failed")
	// ErrSenderSetFailed indicates that setting sender failed.
	ErrSenderSetFailed = errors.New("sender set failed")
	// ErrEmailSendFailed indicates that sending email failed.
	ErrEmailSendFailed = errors.New("email send failed")
)

const defaultTimeout = 10 * time.Second

// Mailer provides email sending capabilities using SMTP.
type Mailer struct {
	client *mail.Client
	from   string
	mu     sync.Mutex
}

// NewMailer creates a new SMTP mailer with the given configuration.
func NewMailer(host, username, password, sender string, port int) (*Mailer, error) {
	client, err := mail.NewClient(
		host,
		mail.WithTimeout(defaultTimeout),
		mail.WithSMTPAuth(mail.SMTPAuthLogin),
		mail.WithPort(port),
		mail.WithUsername(username),
		mail.WithPassword(password),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMailClientCreationFailed, err)
	}

	mailer := &Mailer{
		client: client,
		from:   sender,
	}

	return mailer, nil
}

// Send sends an email with both plain text and HTML body.
func (m *Mailer) Send(recipient, subject, plainBody, htmlBody string) error {
	msg := mail.NewMsg()

	if err := msg.To(recipient); err != nil {
		return fmt.Errorf("%w: %v", ErrRecipientSetFailed, err)
	}

	if err := msg.From(m.from); err != nil {
		return fmt.Errorf("%w: %v", ErrSenderSetFailed, err)
	}

	msg.Subject(subject)
	msg.SetBodyString(mail.TypeTextPlain, plainBody)
	msg.AddAlternativeString(mail.TypeTextHTML, htmlBody)

	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.client.DialAndSend(msg); err != nil {
		return fmt.Errorf("%w: %v", ErrEmailSendFailed, err)
	}

	return nil
}
