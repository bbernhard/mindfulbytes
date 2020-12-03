package main

import (
	"flag"
	"github.com/bbernhard/mindfulbytes/utils"
	"github.com/gomodule/redigo/redis"
	log "github.com/sirupsen/logrus"
	"time"
)

func handlePluginExec(plugin utils.Plugin, plugins *utils.Plugins) error {
	if plugin.Config.Enabled {
		err := plugins.ExecCrawl(plugin.Exec.CrawlExec)
		if err != nil {
			log.Error(err)
			return err
		}
	} else {
		log.Debug("Not running plugin ", plugin.Name, " as it is disabled")
	}
	
	return nil
}

func main() {
	log.Info("Starting Plugin Runner")

	configDir := flag.String("config-dir", "../config/", "Config Directory")
	redisAddress := flag.String("redis-address", ":6379", "Address to the Redis server")
	redisMaxConnections := flag.Int("redis-max-connections", 500, "Max connections to Redis")

	flag.Parse()

	log.SetLevel(log.DebugLevel)
	log.SetOutput(&utils.LogOutputSplitter{})

	//create redis pool
	redisPool := redis.NewPool(func() (redis.Conn, error) {
		c, err := redis.Dial("tcp", *redisAddress)

		if err != nil {
			log.Fatal("Couldn't dial redis: ", err.Error())
		}

		return c, err
	}, *redisMaxConnections)
	defer redisPool.Close()

	plugins := utils.NewPlugins("./plugins/", *configDir)
	err := plugins.Load()
	if err != nil {
		log.Fatal(err)
	}

	tickers := []*time.Ticker{}
	for _, plugin := range plugins.GetPlugins() {
		t, err := utils.SchedulePluginExecution(handlePluginExec, plugin, plugins, redisPool)
		if err !=nil {
			log.Fatal(err.Error())
		}
		tickers = append(tickers, t)
		
	}

	select {} //wait forever
}
