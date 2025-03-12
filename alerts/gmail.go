package alerts

import (
	"fmt"
	"log"

	"gopkg.in/gomail.v2"
)

const gmailSMTP = "smtp.gmail.com"
const gmailSMTPPort = 587

type MailerConfig struct {
	Enabled    bool
	Account    string
	Password   string
	From       string
	Recipients []string
}

type GmailAlerter struct {
	mailClient gomail.SendCloser
	from       string
	recipients []string
}

func NewGmailAlerter(config MailerConfig) (*GmailAlerter, error) {
	d := gomail.NewDialer(gmailSMTP, gmailSMTPPort, config.Account, config.Password)
	sender, err := d.Dial()
	if err != nil {
		return nil, err
	}

	return &GmailAlerter{
		mailClient: sender,
		from:       config.From,
		recipients: config.Recipients,
	}, nil
}

// Alerter interface implementation
func (config *GmailAlerter) SendAlert(title, content string) error {
	log.Printf("Sending alert via gmail to: %v\n", config.recipients)

	if err := config.sendEmail(title, content); err != nil {
		return fmt.Errorf("failed to send alert via email: %v", err)
	}
	return nil
}

func (config *GmailAlerter) sendEmail(subject, body string) error {
	m := gomail.NewMessage()
	m.SetHeader("From", config.from)
	m.SetHeader("To", config.recipients...)
	m.SetHeader("Subject", subject)
	m.SetBody("text/plain", body)

	return gomail.Send(config.mailClient, m)
}
