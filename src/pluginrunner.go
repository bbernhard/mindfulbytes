package main

import (
	log "github.com/sirupsen/logrus"
	"github.com/gomodule/redigo/redis"
	"flag"
)


/*func execPlugin(command string, args []string, baseDir string) error {
	log.Info(args)
	cmd := exec.Command(command, args...)
	cmd.Dir = baseDir
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Start()
	
	if err != nil {
		return err
	}

	// Wait for the process to finish
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()
	select {
	case err := <-done:
		return err
	}

	return nil
}*/


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

	plugins, err := loadPlugins("./plugins/", *configDir)
	if err != nil {
		log.Fatal(err)
	}


	err = execPlugin(plugins[0].MetaData.Command, plugins[0].MetaData.CommandArgs, plugins[0].MetaData.Directory)
	if err != nil {
		log.Fatal(err)
	}

}
