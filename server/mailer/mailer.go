package mailer

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"

	"upstream-x/server/config"
)

type Mailer struct {
	cfgProvider func() (config.SMTPConfig, error)
}

func New(provider func() (config.SMTPConfig, error)) *Mailer {
	return &Mailer{cfgProvider: provider}
}

func (m *Mailer) Enabled() bool {
	cfg, err := m.cfgProvider()
	if err != nil {
		return false
	}
	return cfg.Enabled && cfg.Host != "" && len(cfg.Recipients) > 0
}

func (m *Mailer) Send(subject, body string) error {
	cfg, err := m.cfgProvider()
	if err != nil {
		return err
	}
	if !cfg.Enabled {
		return nil
	}
	return sendMail(cfg, subject, body)
}

func (m *Mailer) SendTest(subject, body string) error {
	cfg, err := m.cfgProvider()
	if err != nil {
		return err
	}
	return sendMail(cfg, subject, body)
}

func sendMail(cfg config.SMTPConfig, subject, body string) error {
	if strings.TrimSpace(cfg.Host) == "" {
		return fmt.Errorf("smtp host is required")
	}
	if cfg.Port <= 0 {
		return fmt.Errorf("smtp port must be > 0")
	}
	if strings.TrimSpace(cfg.From) == "" {
		return fmt.Errorf("smtp from is required")
	}
	if len(cfg.Recipients) == 0 {
		return fmt.Errorf("at least one smtp recipient is required")
	}

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	msg := strings.Builder{}
	msg.WriteString("To: " + strings.Join(cfg.Recipients, ",") + "\r\n")
	msg.WriteString("From: " + cfg.From + "\r\n")
	msg.WriteString("Subject: " + subject + "\r\n")
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	msg.WriteString(body)

	if cfg.UseTLS {
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: cfg.Host})
		if err != nil {
			return fmt.Errorf("dial tls smtp: %w", err)
		}
		defer conn.Close()

		client, err := smtp.NewClient(conn, cfg.Host)
		if err != nil {
			return fmt.Errorf("new smtp client: %w", err)
		}
		defer client.Close()

		auth := smtp.PlainAuth("", cfg.UserName, cfg.Password, cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
		if err := client.Mail(cfg.From); err != nil {
			return fmt.Errorf("smtp mail from: %w", err)
		}
		for _, recipient := range cfg.Recipients {
			if err := client.Rcpt(recipient); err != nil {
				return fmt.Errorf("smtp rcpt %s: %w", recipient, err)
			}
		}
		writer, err := client.Data()
		if err != nil {
			return fmt.Errorf("smtp data: %w", err)
		}
		if _, err := writer.Write([]byte(msg.String())); err != nil {
			return fmt.Errorf("smtp write: %w", err)
		}
		if err := writer.Close(); err != nil {
			return fmt.Errorf("smtp close writer: %w", err)
		}
		return client.Quit()
	}

	auth := smtp.PlainAuth("", cfg.UserName, cfg.Password, cfg.Host)
	return smtp.SendMail(addr, auth, cfg.From, cfg.Recipients, []byte(msg.String()))
}
