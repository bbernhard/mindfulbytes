package notifications

import (
	"github.com/jordan-wright/email"
	"github.com/bbernhard/mindfulbytes/config"
	"github.com/bbernhard/mindfulbytes/api"
	"net/smtp"
	"bytes"
	"github.com/gabriel-vasile/mimetype"
)

type Email struct {
	notification config.Notification
	externalApiClient *api.ExternalApiClient
}

func NewEmail(notification config.Notification) *Email {
	return &Email {
		notification: notification,
		externalApiClient : api.NewExternalApiClient("http://127.0.0.1:8085", 1),
	}
}

func (e *Email) IsEnabled() bool {
	return e.notification.Enabled
}

func (s *Email) Notify() error {
	if s.IsEnabled() {
		recipients := s.notification.Recipients
		host := s.notification.Settings["host"]
		port := s.notification.Settings["port"]
		password := s.notification.Settings["password"]
		sender := s.notification.Settings["sender"]
		
		topics := s.notification.Topics
		for _, topic := range topics {

			data, err := s.externalApiClient.GetRandomImage(topic)
			if err != nil {
				return err
			}

			mime, err := mimetype.DetectReader(bytes.NewReader(data))
			if err != nil {
				return err
			}

			e := email.NewEmail()
			e.From = sender
			e.To = recipients
			e.Subject = "MindfulBytes"

			_, err = e.Attach(bytes.NewReader(data), "mindfulybtes"+mime.Extension(), mime.String())
			if err != nil {
				return err
			}

			err = e.Send(host+":"+port, smtp.PlainAuth("", sender, password, host))
			if err != nil {
				return err
			}

		}
	}

	return nil
}
