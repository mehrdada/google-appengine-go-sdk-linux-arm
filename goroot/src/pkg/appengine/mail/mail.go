// Copyright 2011 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

/*
Package mail provides the means of sending email from an
App Engine application.

Example:
	msg := &mail.Message{
		Sender:  "romeo@montague.com",
		To:      []string{"Juliet <juliet@capulet.org>"},
		Subject: "See you tonight",
		Body:    "Don't forget our plans. Hark, 'til later.",
	}
	if err := mail.Send(c, msg); err != nil {
		c.Errorf("Alas, my user, the email failed to sendeth: %v", err)
	}
*/
package mail

import (
	"net/mail"

	"appengine"
	"appengine_internal"
	"code.google.com/p/goprotobuf/proto"

	base_proto "appengine_internal/base"
	mail_proto "appengine_internal/mail"
)

// A Message represents an email message.
// Addresses may be of any form permitted by RFC 822.
type Message struct {
	// Sender must be set, and must be either an application admin
	// or the currently signed-in user.
	Sender  string
	ReplyTo string // may be empty

	// At least one of these slices must have a non-zero length.
	To, Cc, Bcc []string

	Subject string

	// At least one of Body or HTMLBody must be non-empty.
	Body     string
	HTMLBody string

	Attachments []Attachment

	// Extra mail headers.
	// See https://developers.google.com/appengine/docs/go/mail/overview
	// for permissible headers.
	Headers mail.Header
}

// An Attachment represents an email attachment.
type Attachment struct {
	// Name must be set to a valid file name.
	Name string
	Data []byte
}

// Send sends an email message.
func Send(c appengine.Context, msg *Message) error {
	req := &mail_proto.MailMessage{
		Sender:  &msg.Sender,
		To:      msg.To,
		Cc:      msg.Cc,
		Bcc:     msg.Bcc,
		Subject: &msg.Subject,
	}
	if msg.ReplyTo != "" {
		req.ReplyTo = &msg.ReplyTo
	}
	if msg.Body != "" {
		req.TextBody = &msg.Body
	}
	if msg.HTMLBody != "" {
		req.HtmlBody = &msg.HTMLBody
	}
	if len(msg.Attachments) > 0 {
		req.Attachment = make([]*mail_proto.MailAttachment, len(msg.Attachments))
		for i, att := range msg.Attachments {
			req.Attachment[i] = &mail_proto.MailAttachment{
				FileName: &att.Name,
				Data:     att.Data,
			}
		}
	}
	for key, vs := range msg.Headers {
		for _, v := range vs {
			req.Header = append(req.Header, &mail_proto.MailHeader{
				Name:  proto.String(key),
				Value: proto.String(v),
			})
		}
	}
	res := &base_proto.VoidProto{}
	if err := c.Call("mail", "Send", req, res, nil); err != nil {
		return err
	}
	return nil
}

func init() {
	appengine_internal.RegisterErrorCodeMap("mail", mail_proto.MailServiceError_ErrorCode_name)
}
