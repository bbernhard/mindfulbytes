package notifications

import (
	"github.com/bbernhard/mindfulbytes/config"
	"github.com/bbernhard/mindfulbytes/utils"
	"github.com/bbernhard/mindfulbytes/api"
	log "github.com/sirupsen/logrus"
	"encoding/base64"
	"github.com/go-resty/resty/v2"
	"errors"
	"time"
)

type SignalMessenger struct {
	notification config.Notification
	externalApiClient *api.ExternalApiClient
	signalMessengerRestClient *resty.Client
	signalMessengerRestApiBaseUrl string
}

func NewSignalMessenger(notification config.Notification) *SignalMessenger {
	return &SignalMessenger {
		notification: notification,
		externalApiClient : api.NewExternalApiClient("http://127.0.0.1:8085", 1),
		signalMessengerRestClient : resty.New(),
		signalMessengerRestApiBaseUrl: notification.Settings["url"]+"/v2",
	}
}

func (s *SignalMessenger) IsEnabled() bool {
	return s.notification.Enabled
}

func (s *SignalMessenger) sendMessage(message string, attachment []byte, recipients []string) error {
	type Body struct {
		Message string `json:"message"`
		Number string `json:"number"`
		Recipients []string `json:"recipients"`
		Base64EncodedAttachments []string `json:"base64_attachments"`
	}

	base64EncodedAttachment := base64.StdEncoding.EncodeToString(attachment)

	body := &Body{Message: message, Number: s.notification.Settings["number"], Recipients: s.notification.Recipients}
	body.Base64EncodedAttachments = append(body.Base64EncodedAttachments, base64EncodedAttachment)

	url := s.signalMessengerRestApiBaseUrl + "/send"
	log.Info(url)
	resp, err := s.signalMessengerRestClient.R().
					SetHeader("Content-Type", "application/json").
					SetBody(body).
					Post(url)
	if err != nil {
		return err
	}

	if resp.StatusCode() != 201 {
		return errors.New("Couldn't send message")
	}

	return nil
}

func (s *SignalMessenger) Notify() error {
	if s.IsEnabled() {
		recipients := s.notification.Recipients
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

			message, err := utils.ReplaceTagsInMessage(s.notification.Message, date, s.notification.DefaultLanguage) 
			if err != nil {
				return err
			}

			err = s.sendMessage(message, imageData.Image, recipients)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
