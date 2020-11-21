package main

import (
	"flag"
	"time"
	"github.com/bbernhard/mindfulbytes/config"
	"github.com/bbernhard/mindfulbytes/notifications"
	"github.com/bbernhard/mindfulbytes/utils"
	log "github.com/sirupsen/logrus"
	"github.com/gomodule/redigo/redis"
	"errors"
)

func handleNotification(name string, notification config.Notification) error {
	var err error = nil
	if notification.Enabled {
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

		if err == nil {
			log.Debug("Handled ", name, " notification")
		} else {
			log.Error("Couldn't handle ", name, " notification: ", err.Error())
		}
	} else {
		log.Debug("Nothing to do for ", name, " notification as notification is disabled")
	}
	return err
}


func main() {
	configFile := flag.String("config-file", "../config/config.yaml", "Path to config file")
	redisAddress := flag.String("redis-address", ":6379", "Address to the Redis server")
	redisMaxConnections := flag.Int("redis-max-connections", 500, "Max connections to Redis")

	flag.Parse()

	log.SetLevel(log.DebugLevel)
	log.Info("Starting notifier")

	//create redis pool
	redisPool := redis.NewPool(func() (redis.Conn, error) {
		c, err := redis.Dial("tcp", *redisAddress)

		if err != nil {
			log.Fatal("Couldn't dial redis: ", err.Error())
		}

		return c, err
	}, *redisMaxConnections)

	config, err := config.ParseConfig(*configFile)
	if err != nil {
		log.Fatal("Couldn't parse config: ", err.Error())
	}

	tickers := []*time.Ticker{}
	for name, notification := range config.Notifications {
		log.Debug("Handling notification ", name)
		t, err := utils.ScheduleNotification(handleNotification, name, notification, redisPool)
		if err != nil {
			log.Fatal(err.Error())
		}
		tickers = append(tickers, t)
	}

	select {} //wait forever
}
