package utils

import (
	"time"
	"errors"
	"strconv"
	"strings"
	"regexp"
	"os"
	"bytes"
	"math/rand"
	"github.com/bbernhard/mindfulbytes/config"
	"github.com/gomodule/redigo/redis"
	log "github.com/sirupsen/logrus"
	timeago "github.com/xeonx/timeago"
)

type scheduleNotificationFuncDef func(string, config.Notification) error
type schedulePluginExecFuncDef func(plugin Plugin, plugins *Plugins) error

func GetRandomNumber(max int) int {
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	return r1.Intn(max)
}

func StringInSlice(a string, list []string) bool {
    for _, b := range list {
        if b == a {
            return true
        }
    }
    return false
}

type LogOutputSplitter struct{}

func (splitter *LogOutputSplitter) Write(p []byte) (n int, err error) {
	if bytes.Contains(p, []byte("level=error")) {
		return os.Stderr.Write(p)
	}
	return os.Stdout.Write(p)
}

func FuzzyTimeToDuration(t string) (time.Duration, error) {
	if t == "daily" {
		return time.ParseDuration("24h")
	} else if t == "weekly" {
		return time.ParseDuration("168h")
	} else if t == "monthly" {
		return time.ParseDuration("720h")
	}

	return time.ParseDuration("24h")
}

func GetTimeagoConfigForLanguage(language string) timeago.Config {
	var timeagoConfig timeago.Config
	timeLayout := "2006-01-02"
	if language == "en" {
		timeagoConfig = timeago.NoMax(timeago.English)
	} else if language == "ge" {
		timeagoConfig = timeago.NoMax(timeago.German)
	} else {
		timeagoConfig = timeago.NoMax(timeago.English)
	}
	timeagoConfig.DefaultLayout = timeLayout
	return timeagoConfig
}

func ReplaceTagsInMessage(template string, timestamp time.Time, language string) (string, error) {
	s := template
	r := regexp.MustCompile("(\\{\\{[ ]*[a-z]*[ ]*\\}\\})*")
	allSubmatches := r.FindAllStringSubmatch(template, -1)
	for _, subMatch := range allSubmatches {
		if len(subMatch) > 0 {
			tag := subMatch[0]
			strippedTag := strings.Replace(tag, " ", "", -1)
			if strippedTag == "{{timeago}}" {
				timeagoConfig := GetTimeagoConfigForLanguage(language)
				t := timeagoConfig.FormatReference(timestamp, time.Now())
				s = strings.Replace(s, tag, t, -1)
			}
		}
	}
	return s, nil
}

func GetLastSuccessfulCrawlExecutionTimestamp(redisPool *redis.Pool, pluginName string) (time.Time, error) {
	key := pluginName + ":settings:crawl:lastsuccess"
	return getUnixTimestampFromRedis(redisPool, key)
}

func getUnixTimestampFromRedis(redisPool *redis.Pool, key string) (time.Time, error) {
	redisConnection := redisPool.Get()
	defer redisConnection.Close()

	bytes, err := redis.Bytes(redisConnection.Do("GET", key))
	if err != nil {
		if err == redis.ErrNil {
			return time.Time{}, nil
		}
		return time.Time{}, err
	}

	unixTimestamp := string(bytes)
	if unixTimestamp != "" {
		unixTimestampInt, err := strconv.ParseInt(unixTimestamp, 10, 64)
    	if err != nil {
        	return time.Time{}, err
    	}
		return time.Unix(unixTimestampInt, 0), nil
	}

	return time.Time{}, nil
}

func updateUnixTimestampInRedis(redisPool *redis.Pool, key string, timestamp time.Time) error {
	redisConnection := redisPool.Get()
	defer redisConnection.Close()

	unixTimestamp := int64(timestamp.Unix())
	_, err := redisConnection.Do("SET", key, unixTimestamp)
	return err
}

func SetLastSuccessfulCrawlExecutionTimestamp(redisPool *redis.Pool, pluginName string, timestamp time.Time) error {
	key := pluginName + ":settings:crawl:lastsuccess"
	return updateUnixTimestampInRedis(redisPool, key, timestamp)
}

func GetLastSuccessfulNotificationTimestamp(redisPool *redis.Pool, notificationName string) (time.Time, error) {
	key := notificationName + ":settings:notification:lastsuccess"
	return getUnixTimestampFromRedis(redisPool, key)
}

func SetLastSuccessfulNotificationTimestamp(redisPool *redis.Pool, notificationName string, timestamp time.Time) error {
	key := notificationName + ":settings:notification:lastsuccess"
	return updateUnixTimestampInRedis(redisPool, key, timestamp)
}

func SchedulePluginExecution(f schedulePluginExecFuncDef, plugin Plugin, plugins *Plugins, redisPool *redis.Pool) (*time.Ticker, error) {
	defaultInterval, err := FuzzyTimeToDuration(plugin.Config.Refresh)
	if err != nil {
		return &time.Ticker{}, errors.New("Couldn't initialize " + plugin.Name + " crawl: " + err.Error())
	}

	lastSuccessfulExecutionTimestamp, err := GetLastSuccessfulCrawlExecutionTimestamp(redisPool, plugin.Name)
	if err != nil {
		return &time.Ticker{}, errors.New("Couldn't initialize " + plugin.Name + " crawl: " + err.Error())
	}

	var interval time.Duration
	if(lastSuccessfulExecutionTimestamp.IsZero()) { //hasn't run before, so trigger a run
		log.Debug("Run crawl now for plugin ", plugin.Name, " as it hasn't been run before")
		interval, err = time.ParseDuration("1s")
		if err != nil {
			return &time.Ticker{}, errors.New("Couldn't trigger crawl for plugin " + plugin.Name + ": " + err.Error())
		}
	} else {
		d := time.Now().Sub(lastSuccessfulExecutionTimestamp)
		if(d.Seconds() > defaultInterval.Seconds()) {
			log.Debug("Run crawl now for plugin ", plugin.Name, " as the last run was more than ", defaultInterval.Seconds(), " seconds ago")
			interval, err = time.ParseDuration("1s")
			if err != nil {
				return &time.Ticker{}, errors.New("Couldn't trigger crawl for plugin " + plugin.Name + ": " + err.Error())
			}
		} else {
			nextRunInSeconds := strconv.FormatInt(int64(defaultInterval.Seconds() - d.Seconds()), 10)
			log.Debug("Schedule crawl for plugin ", plugin.Name, " to run in ", nextRunInSeconds, " seconds")
			interval, err = time.ParseDuration(nextRunInSeconds+ "s")
			if err != nil {
				return &time.Ticker{}, errors.New("Couldn't trigger crawl for plugin " + plugin.Name + ": " + err.Error())
			}
		}
	}

	errorInterval, err := time.ParseDuration("1h")
	if err != nil {
		return &time.Ticker{}, errors.New("Couldn't trigger crawl for plugin " + plugin.Name + ": " + err.Error())
	}

	ticker := time.NewTicker(interval)
    go func() {
        for {
            select {
            case <-ticker.C:
                err := f(plugin, plugins)
				ticker.Stop()

				if err != nil { //something went wrong, schedule another run in an hour
					log.Debug("Schedule another crawl for plugin ", plugin.Name, " in ", errorInterval.Seconds(), " seconds, as the last try failed")
					ticker = time.NewTicker(errorInterval)
				} else { //execution was successful
                	//update last execution timestamp
					err = SetLastSuccessfulCrawlExecutionTimestamp(redisPool, plugin.Name, time.Now())
					if err != nil {
						log.Debug("Schedule another crawl for plugin ", plugin.Name, " in ", errorInterval.Seconds(), " seconds, as last execution failed")
						ticker = time.NewTicker(errorInterval)
					} else {
						log.Debug("Schedule another crawl for plugin ", plugin.Name, " in ", defaultInterval.Seconds(), " seconds")
						ticker = time.NewTicker(defaultInterval)
					}
				}
            }
        }
    }()
    return ticker, nil
}


func ScheduleNotification(f scheduleNotificationFuncDef, name string, notification config.Notification,
		redisPool *redis.Pool) (*time.Ticker, error) {
	defaultInterval, err := FuzzyTimeToDuration(notification.Interval)
	if err != nil {
		return &time.Ticker{}, errors.New("Couldn't initialize " + name + " notification: " + err.Error())
	}

	lastSuccessfulNotificationTimestamp, err := GetLastSuccessfulNotificationTimestamp(redisPool, name)
	if err != nil {
		return &time.Ticker{}, errors.New("Couldn't initialize " + name + " notification: " + err.Error())
	}

	var interval time.Duration
	if(lastSuccessfulNotificationTimestamp.IsZero()) { //hasn't run before, so trigger a run
		log.Debug("Run ", name, " notification now, as it hasn't been run before")
		interval, err = time.ParseDuration("1s")
		if err != nil {
			return &time.Ticker{}, errors.New("Couldn't trigger " + name + " notification: " + err.Error())
		}
	} else {
		d := time.Now().Sub(lastSuccessfulNotificationTimestamp)
		if(d.Seconds() > defaultInterval.Seconds()) {
			log.Debug("Run ", name, " notification now as the last run was more than ", defaultInterval.Seconds(), " seconds ago")
			interval, err = time.ParseDuration("1s")
			if err != nil {
				return &time.Ticker{}, errors.New("Couldn't trigger " + name + " notification: " + err.Error())
			}
		} else {
			nextRunInSeconds := strconv.FormatInt(int64(defaultInterval.Seconds() - d.Seconds()), 10)
			log.Debug("Schedule ", name, " notification to run in ", nextRunInSeconds, " seconds")
			interval, err = time.ParseDuration(nextRunInSeconds+ "s")
			if err != nil {
				return &time.Ticker{}, errors.New("Couldn't trigger " + name + " notification: " + err.Error())
			}
		}
	}

	errorInterval, err := time.ParseDuration("1h")
	if err != nil {
		return &time.Ticker{}, errors.New("Couldn't trigger " + name + " notification: " + err.Error())
	}

	ticker := time.NewTicker(interval)
    go func() {
        for {
            select {
            case <-ticker.C:
                err := f(name, notification)
				ticker.Stop()

				if err != nil { //something went wrong, schedule another try in an hour
					log.Debug("Schedule another ", name, " notification in ", errorInterval.Seconds(), " seconds, as the last try failed")
					ticker = time.NewTicker(errorInterval)
				} else { //notification was successful
					err = SetLastSuccessfulNotificationTimestamp(redisPool, name, time.Now())
					if err != nil {
						log.Debug("Schedule another ", name, " notification in ", errorInterval.Seconds(), " seconds, as last try failed")
						ticker = time.NewTicker(errorInterval)
					} else {
						log.Debug("Schedule another ", name, " notification in ", defaultInterval.Seconds(), " seconds")
						ticker = time.NewTicker(defaultInterval)
					}
				}
            }
        }
    }()
    return ticker, nil
	
	/*ticker := time.NewTicker(interval)
    go func() {
        for range ticker.C {
            f(name, notification)
        }
    }()
    return ticker*/
}
