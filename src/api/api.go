package api

import (
	"github.com/gomodule/redigo/redis"
	"github.com/bbernhard/mindfulbytes/utils"
	"io/ioutil"
	"strings"
	"github.com/gofrs/uuid"
	log "github.com/sirupsen/logrus"
	"os"
	"encoding/json"
	"github.com/gabriel-vasile/mimetype"
	"bytes"
	//
)

type InternalServerError struct {
	Description string
}

func (e *InternalServerError) Error() string {
	return e.Description
}

type ItemNotFoundError struct {
	Description string
}

func (e *ItemNotFoundError) Error() string {
	return e.Description
}

type Entry struct {
	Uri string `json:"uri"`
	Uuid string `json:"uuid"`
	Plugin string `json:"plugin"`
	FullDate string `json:"fulldate,omitempty"`
}

type Api struct {
	redisPool *redis.Pool
	imageMagickWrapper *utils.ImageMagickWrapper
	plugins *utils.Plugins
	tmpDir string
}

type CacheEntryRequest struct {
	UrlPath string `json:"urlpath"`
	ExpiresInSeconds int `json:"expires"`
}

func NewApi(redisPool *redis.Pool, imageMagickWrapper *utils.ImageMagickWrapper, plugins *utils.Plugins, tmpDir string) *Api {
	return &Api{
		redisPool: redisPool,
		imageMagickWrapper: imageMagickWrapper,
		plugins: plugins,
		tmpDir: tmpDir,
	}
}


func (a *Api) GetDataForDate(plugins []string, date string) ([]Entry, error) {
	redisConn := a.redisPool.Get()
	defer redisConn.Close()
	
	allEntries := []Entry{}
	for _, plugin := range plugins {
		key := plugin + ":date:" + date

		bytes, err := redis.Bytes(redisConn.Do("GET", key))
		if err != nil {
			if err == redis.ErrNil {
				if len(plugins) == 1 {
					return allEntries, &ItemNotFoundError{Description:"No item with that key found"}
				}
				continue
			}
			return allEntries, &InternalServerError{Description: "Couldn't get key: " + err.Error()}
		}

		var entries []Entry
		err = json.Unmarshal(bytes, &entries)
		if err != nil {
			return allEntries, &InternalServerError{Description: "Couldn't parse json: " + err.Error()}
		}
		
		for i := 0; i < len(entries); i++ {
			entry := &entries[i]
			entry.Plugin = plugin
		}

		allEntries = append(allEntries, entries...)
	}

	return allEntries, nil
}

func (a *Api) GetDataForFullDate(plugins []string, day string) ([]Entry, error) {
	redisConn := a.redisPool.Get()
	defer redisConn.Close()

	allEntries := []Entry{}

	for _, plugin := range plugins {
		key := plugin + ":fulldate:" + day
		
		bytes, err := redis.Bytes(redisConn.Do("GET", key))
		if err != nil {
			if err == redis.ErrNil {
				if len(plugins) == 1 {
					return allEntries, &ItemNotFoundError{Description:"No item with that key found"}
				}
				continue
			}
			return allEntries, &InternalServerError{Description: "Couldn't get key: " + err.Error()}
		}

		var entries []Entry
		err = json.Unmarshal(bytes, &entries)
		if err != nil {
			return allEntries, &InternalServerError{Description: "Couldn't parse json: " + err.Error()}
		}
		
		for i := 0; i < len(entries); i++ {
			entry := &entries[i]
			entry.Plugin = plugin
		}

		allEntries = append(allEntries, entries...)
	}

	return allEntries, nil
}

func removeFiles(files []string) error {
	var err error = nil
	for _, file := range files {
		e := os.Remove(file)
		if e != nil {
			log.Error("Couldn't remove file ", file, ": ", err.Error())
			err = e
		}
	}

	return err
}

func (a *Api) GetImage(plugin string, imageId string, convertOptions utils.ConvertOptions) ([]byte, string, error) {
	uri := imageId

	p, err := a.plugins.GetPlugin(plugin)
	if err != nil {
		return []byte(""), "", &ItemNotFoundError{Description: "No plugin with that name found: " + err.Error()}
	}
	
	tmpFileName, err := uuid.NewV4()
	if err != nil {
		return []byte(""), "", err
	}

	tmpDestination := a.tmpDir + "/" + tmpFileName.String()
	err = a.plugins.ExecFetch(uri, tmpDestination, p.Exec.FetchExec)
	if err != nil {
		return []byte(""), "", &InternalServerError{Description: "Couldn't fetch image: " + err.Error()}
	}

	tmpFilesToCleanup := []string{tmpDestination}

	u, err := uuid.NewV4()
	if err != nil {
		removeFiles(tmpFilesToCleanup) //no need to check return code, it's just cleanup
		return []byte(""), "", err
	}

	convertedTmpDestination, err := a.imageMagickWrapper.Convert(tmpDestination, u.String(), convertOptions)
	if err != nil {
		removeFiles(tmpFilesToCleanup) //no need to check return code, it's just cleanup
		return []byte(""), "", err
	}
	tmpFilesToCleanup = append(tmpFilesToCleanup, convertedTmpDestination)
	tmpDestination = convertedTmpDestination

	imgBytes, err := ioutil.ReadFile(tmpDestination)
	if err != nil {
		removeFiles(tmpFilesToCleanup) //no need to check return code, it's just cleanup
		return []byte(""), "", &InternalServerError{Description: "Couldn't read image: " + err.Error()}
	}

	removeFiles(tmpFilesToCleanup) //no need to check return code, it's just cleanup

	mime, err := mimetype.DetectReader(bytes.NewReader(imgBytes))
	if err != nil {
		return []byte(""), "", err
	}


	return imgBytes, mime.String(), nil
}

func (a *Api) GetDates(plugins []string) ([]string, error) {
	redisConn := a.redisPool.Get()
	defer redisConn.Close()
	
	dates := []string{}
	for _, plugin := range plugins {
		key := plugin + ":date:*"

		res, err := redis.Strings(redisConn.Do("KEYS", key))
		if err != nil {
			if err == redis.ErrNil {
				return []string{}, &ItemNotFoundError{Description:"No item with that key found"}
			}

			return []string{}, &InternalServerError{Description: "Couldn't get key: " + err.Error()}
		}

		
		for _, elem := range res {
			date := strings.TrimPrefix(elem, plugin + ":date:")
			if !utils.StringInSlice(date, dates) {
				dates = append(dates, date)
			}
		}
	}

	return dates, nil
}

func (a *Api) GetFullDates(plugins []string) ([]string, error) {
	redisConn := a.redisPool.Get()
	defer redisConn.Close()

	fullDates := []string{}

	for _, plugin := range plugins {
		key := plugin + ":fulldate:*"

		res, err := redis.Strings(redisConn.Do("KEYS", key))
		if err != nil {
			if err == redis.ErrNil {
				return []string{}, &ItemNotFoundError{Description:"No item with that key found"}
			}

			return []string{}, &InternalServerError{Description: "Couldn't get key: " + err.Error()}
		}

		for _, elem := range res {
			fullDate := strings.TrimPrefix(elem, plugin + ":fulldate:")
			if !utils.StringInSlice(fullDate, fullDates) {
				fullDates = append(fullDates, fullDate)
			}
		}
	}

	return fullDates, nil
}

func (a *Api) GetCachedEntry(cacheId string) ([]byte, error) {
	key := "cache:" + cacheId

	redisConn := a.redisPool.Get()
	defer redisConn.Close()
	
	bytes, err := redis.Bytes(redisConn.Do("GET", key))
	if err != nil {
		if err == redis.ErrNil {
			return []byte{}, &ItemNotFoundError{Description:"No item with that key found"}
		}
		return []byte{}, &InternalServerError{Description: "Couldn't get key: " + err.Error()}
	}

	return bytes, nil
}

func (a *Api) CacheEntry(cacheId string, data []byte, expiresInSeconds int) error {
	key := "cache:" + cacheId
	
	redisConn := a.redisPool.Get()
	defer redisConn.Close()

	_, err := redisConn.Do("SETEX", key, expiresInSeconds, data)
	return err
}

func (a *Api) GetCacheEntries() ([]string, error) {
	redisConn := a.redisPool.Get()
	defer redisConn.Close()

	cacheEntries := []string{}

	key := "cache:*"

	res, err := redis.Strings(redisConn.Do("KEYS", key))
	if err != nil {
		if err == redis.ErrNil {
			return []string{}, nil 
		}

		return []string{}, &InternalServerError{Description: "Couldn't get keys: " + err.Error()}
	}

	for _, elem := range res {
		cacheEntry := strings.TrimPrefix(elem, "cache:")
		cacheEntries = append(cacheEntries, cacheEntry)
	}


	return cacheEntries, nil
}
