package main

import (
	"flag"
	"time"
	"github.com/bbernhard/mindfulbytes/config"
	"github.com/bbernhard/mindfulbytes/notifications"
	"github.com/bbernhard/mindfulbytes/utils"
	log "github.com/sirupsen/logrus"
	"errors"
)

type funcDef func(string, config.Notification) error

func handleNotification(name string, notification config.Notification) error {
	var err error
	if name == "signalmessenger" {
		log.Debug("Handling signalmessenger notification")
		signalMessengerNotifier := notifications.NewSignalMessenger(notification)
		err = signalMessengerNotifier.Notify()
	} else if name == "email" {
		log.Debug("Handling email notification")
		email := notifications.NewEmail(notification)
		err = email.Notify()
	} else {
		err = errors.New("No notification with name " + name + " found")
	}

	if err != nil {
		log.Debug("Handled ", name, " notification")
	}

	return err
}

func schedule(f funcDef, name string, notification config.Notification, interval time.Duration) *time.Ticker {
    ticker := time.NewTicker(interval)
    go func() {
        for range ticker.C {
            f(name, notification)
        }
    }()
    return ticker
}


func main() {
	configFile := flag.String("config-file", "../config/config.yaml", "Path to config file")

	flag.Parse()

	log.SetLevel(log.DebugLevel)
	log.Info("Starting notifier")

	config, err := config.ParseConfig(*configFile)
	if err != nil {
		log.Fatal("Couldn't parse config: ", err.Error())
	}

	tickers := []*time.Ticker{}
	for name, notification := range config.Notifications {
		log.Debug("Handling notification ", name)
		if notification.Enabled {
			log.Debug("Notification ", name, " enabled")
			
			duration, err := utils.FuzzyTimeToDuration(notification.Interval)
			if err != nil {
				log.Fatal("Couldn't initialize ", name, " notification: ", err.Error())
			}

			err = handleNotification(name, notification)
			if err != nil {
					log.Error("Error: ", err.Error())
			}
			//t := schedule(handleNotification, name, notification, 300 * time.Second)
			t := schedule(handleNotification, name, notification, duration)
			tickers = append(tickers, t)
		}
	}

	select {} //wait forever
}
