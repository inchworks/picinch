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

type Emailer struct {
	dialer *mail.Dialer
	local string
	sender string
	templates map[string]*template.Template
}

// New returns an Emailer, used to send emails.
// ## localHost specifies an optional domain naame, used to set message IDs. I'm not sure if this is needed for an email client.
func New(host string, port int, username, password, sender string, localHost string, templates map[string]*template.Template) *Emailer {

	em := &Emailer{
		local: localHost,
		sender: sender,
		templates: templates,
	}

	em.dialer = mail.NewDialer(host, port, username, password)
	em.dialer.Timeout = 5 * time.Second

	return em
}

// Send constructs and sends an email.
func (m *Emailer) Send(recipient, templateName string, data interface{}) error {

	var err error

	// get template from cache
	ts, ok := m.templates[templateName]
	if !ok {
		return fmt.Errorf("The template %s does not exist", templateName)
	}

	subject := new(bytes.Buffer)
	err = ts.ExecuteTemplate(subject, "subject", data)
	if err != nil {
		return err
	}

	plainBody := new(bytes.Buffer)
	err = ts.ExecuteTemplate(plainBody, "plainBody", data)
	if err != nil {
		return err
	}

	htmlBody := new(bytes.Buffer)
	err = ts.ExecuteTemplate(htmlBody, "htmlBody", data)
	if err != nil {
		return err
	}

	msg := mail.NewMessage()
	msg.SetHeader("To", recipient)
	msg.SetHeader("From", m.sender)
	msg.SetHeader("Subject", subject.String())

	if m.local != "" {
		// for messsage ID required by RFC 2822
		now := time.Now()
		msg.SetHeader("Message-Id", fmt.Sprintf("<%d.%d@picinch.%s>", now.Unix(), now.Nanosecond(), m.local))
	}

	msg.SetBody("text/plain", plainBody.String())
	msg.AddAlternative("text/html", htmlBody.String())

	err = m.dialer.DialAndSend(msg)
	if err != nil {
		return err
	}

	return nil
}
