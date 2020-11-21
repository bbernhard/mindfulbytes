package main

import (
	"flag"
	"github.com/bbernhard/mindfulbytes/utils"
	"github.com/gomodule/redigo/redis"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.Info("Starting Plugin Runner")

	configDir := flag.String("config-dir", "../config/", "Config Directory")
	redisAddress := flag.String("redis-address", ":6379", "Address to the Redis server")
	redisMaxConnections := flag.Int("redis-max-connections", 500, "Max connections to Redis")

	flag.Parse()

	log.SetLevel(log.DebugLevel)

	//create redis pool
	redisPool := redis.NewPool(func() (redis.Conn, error) {
		c, err := redis.Dial("tcp", *redisAddress)

		if err != nil {
			log.Fatal("Couldn't dial redis: ", err.Error())
		}

		return c, err
	}, *redisMaxConnections)
	defer redisPool.Close()

	redisConn := redisPool.Get()
	defer redisConn.Close()

	psc := redis.PubSubConn{Conn: redisConn}
	defer psc.Close()

	log.Info("Subscribing")
	if err := psc.Subscribe(redis.Args{}.AddFlat([]string{"imgreader"})...); err != nil {
		log.Fatal("Couldn't subscribe to topic 'imgreader': ", err.Error())
	}

	done := make(chan error, 1)

	go func() {
		for {
			switch n := psc.Receive().(type) {
			case error:
				done <- n
				return
			case redis.Message:
				log.Info(n.Channel)
			case redis.Subscription:
				switch n.Count {
				case 0:
					// Return from the goroutine when all channels are unsubscribed.
					done <- nil
					return
				}
			}
		}
	}()

	plugins := utils.NewPlugins("./plugins/", *configDir)
	err := plugins.Load()
	if err != nil {
		log.Fatal(err)
	}

	for _, plugin := range plugins.GetPlugins() {
		if plugin.Config.Enabled {
			err = plugins.ExecCrawl(plugin.Exec.CrawlExec)
			if err != nil {
				log.Error(err)
			}
		}
	}
}
