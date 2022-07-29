package emailer

// Sends notification emails from the gallery, using Mailgun's API.

import (
	"context"
	"html/template"
	"time"

	"github.com/mailgun/mailgun-go/v4"
)

type Gunner struct {
	mg        *mailgun.MailgunImpl
	local     string
	sender    string
	replyTo   string
	templates map[string]*template.Template
}

// New returns a Mailgunner, used to send emails via Mailgun.
func NewGunner(domain, apiKey string, sender string, replyTo string, templates map[string]*template.Template) *Gunner {

	emr := &Gunner{
		mg:        mailgun.NewMailgun(domain, apiKey),
		sender:    sender,
		replyTo:   replyTo,
		templates: templates,
	}
	emr.mg.SetAPIBase(mailgun.APIBaseEU)

	return emr
}

// Send constructs and sends an email via Mailgun.
func (emr *Gunner) Send(recipient, templateName string, data interface{}) error {

	// use templates for message parts
	parts, err := execute(emr.templates, templateName, data)
	if err != nil {
		return err
	}

	// construct message
	m := emr.mg.NewMessage(emr.sender, parts.subject, parts.plain, recipient)
	m.SetHtml(parts.html)
	if emr.replyTo != "" {
		m.SetReplyTo(emr.replyTo)
	}

	// send message
	// ## need retries?
	ctx, cancel := context.WithTimeout(context.Background(), 10 * time.Second)
	defer cancel()

	// ## are mes and id useful?
	_, _, err = emr.mg.Send(ctx, m)

	return err
}
