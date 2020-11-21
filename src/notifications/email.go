package notifications

import (
	"github.com/jordan-wright/email"
	"github.com/bbernhard/mindfulbytes/config"
	"github.com/bbernhard/mindfulbytes/api"
	"github.com/bbernhard/mindfulbytes/utils"
	"net/smtp"
	"bytes"
	"time"
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

			imageData, err := s.externalApiClient.GetImageTodayOrRandomWithData(topic)
			if err != nil {
				return err
			}

			layout := "2006-01-02"
			date, err := time.Parse(layout, imageData.FullDate)
			if err != nil {
				return err
			}

			message, err := utils.ReplaceTagsInMessage(s.notification.Message, date, "en") //TODO change default lang
			if err != nil {
				return err
			}

			mime, err := mimetype.DetectReader(bytes.NewReader(imageData.Image))
			if err != nil {
				return err
			}

			e := email.NewEmail()
			e.From = sender
			e.To = recipients
			e.Text = []byte(message)
			e.Subject = "MindfulBytes"

			_, err = e.Attach(bytes.NewReader(imageData.Image), "mindfulbytes"+mime.Extension(), mime.String())
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
