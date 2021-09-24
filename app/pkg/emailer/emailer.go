// Copyright © Rob Burke inchworks.com, 2021.

// This file is part of PicInch.
//
// PicInch is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// PicInch is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with PicInch.  If not, see <https://www.gnu.org/licenses/>.

package emailer

// Sends notification emails from the gallery.

// Adapted from Let's Go Further! by Alex Edwards, with thanks.

import (
	"bytes"
	"fmt"
	"html/template"
	"time"

	"github.com/go-mail/mail/v2"
)

type Emailer interface {
	Send(recipient, templateName string, data interface{}) error
}

type Dialer struct {
	dialer *mail.Dialer
	local string
	sender string
	templates map[string]*template.Template
}

type parts struct {
	subject string
	plain string
	html string
}

// New returns an Emailer, used to send emails.
// ## localHost specifies an optional domain name, used to set message IDs. I'm not sure if this is needed for an email client.
func NewDialer(host string, port int, username, password, sender string, localHost string, templates map[string]*template.Template) *Dialer {

	em := &Dialer{
		local: localHost,
		sender: sender,
		templates: templates,
	}

	em.dialer = mail.NewDialer(host, port, username, password)
	em.dialer.Timeout = 10 * time.Second

	return em
}

// Send constructs and sends an email.
func (m *Dialer) Send(recipient, templateName string, data interface{}) error {

	// use templates for message parts
	parts, err := execute(m.templates, templateName, data)
	if err != nil {
		return err
	}

	// construct message
	msg := mail.NewMessage()
	msg.SetHeader("To", recipient)
	msg.SetHeader("From", m.sender)
	msg.SetHeader("Subject", parts.subject)

    // messsage ID required by RFC 2822, unless provided by the SMTP relay service
	if m.local != "" {
		
		now := time.Now()
		msg.SetHeader("Message-Id", fmt.Sprintf("<%d.%d@picinch.%s>", now.Unix(), now.Nanosecond(), m.local))
	}

	msg.SetBody("text/plain", parts.plain)
	msg.AddAlternative("text/html", parts.html)

	// send message
	err = m.dialer.DialAndSend(msg)
	if err != nil {
		return err
	}

	return nil
}

// execute generates the email subject and body using templates.
func execute(templates map[string]*template.Template, templateName string, data interface{}) (*parts, error) {

	var err error

	// get template from cache
	ts, ok := templates[templateName]
	if !ok {
		return nil, fmt.Errorf("The template %s does not exist", templateName)
	}

	subject := new(bytes.Buffer)
	err = ts.ExecuteTemplate(subject, "subject", data)
	if err != nil {
		return nil, err
	}

	plainBody := new(bytes.Buffer)
	err = ts.ExecuteTemplate(plainBody, "plainBody", data)
	if err != nil {
		return nil, err
	}

	htmlBody := new(bytes.Buffer)
	err = ts.ExecuteTemplate(htmlBody, "htmlBody", data)
	if err != nil {
		return nil, err
	}

	return &parts{ subject.String(), plainBody.String(), htmlBody.String() }, nil
}
