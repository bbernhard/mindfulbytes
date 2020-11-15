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
}

type Api struct {
	redisConn redis.Conn
	imageMagickWrapper *utils.ImageMagickWrapper
	plugins *utils.Plugins
	tmpDir string
}

func NewApi(redisConn redis.Conn, imageMagickWrapper *utils.ImageMagickWrapper, plugins *utils.Plugins, tmpDir string) *Api {
	return &Api{
		redisConn: redisConn,
		imageMagickWrapper: imageMagickWrapper,
		plugins: plugins,
		tmpDir: tmpDir,
	}
}


func (a *Api) GetDataForDate(plugins []string, date string) ([]Entry, error) {
	allEntries := []Entry{}
	for _, plugin := range plugins {
		key := plugin + ":date:" + date

		bytes, err := redis.Bytes(a.redisConn.Do("GET", key))
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
	
	allEntries := []Entry{}

	for _, plugin := range plugins {
		key := plugin + ":fulldate:" + day
		
		bytes, err := redis.Bytes(a.redisConn.Do("GET", key))
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

func (a *Api) GetImage(plugin string, imageId string, convert bool, size string, caption string) ([]byte, error) {
	uri := imageId

	p, err := a.plugins.GetPlugin(plugin)
	if err != nil {
		return []byte(""), &ItemNotFoundError{Description: "No plugin with that name found: " + err.Error()}
	}
	
	tmpFileName, err := uuid.NewV4()
	if err != nil {
		return []byte(""), err
	}

	tmpDestination := a.tmpDir + "/" + tmpFileName.String()
	err = a.plugins.ExecFetch(uri, tmpDestination, p.Exec.FetchExec)
	if err != nil {
		return []byte(""), &InternalServerError{Description: "Couldn't fetch image: " + err.Error()}
	}

	tmpFilesToCleanup := []string{tmpDestination}

	if(convert) {
		u, err := uuid.NewV4()
		if err != nil {
			removeFiles(tmpFilesToCleanup) //no need to check return code, it's just cleanup
			return []byte(""), err
		}
		convertedTmpDestination, err := a.imageMagickWrapper.ConvertToEPaper(tmpDestination, u.String(), size, caption)
		if err != nil {
			removeFiles(tmpFilesToCleanup) //no need to check return code, it's just cleanup
			return []byte(""), err
		}
		tmpFilesToCleanup = append(tmpFilesToCleanup, convertedTmpDestination)
		tmpDestination = convertedTmpDestination
	}

	imgBytes, err := ioutil.ReadFile(tmpDestination)
	if err != nil {
		removeFiles(tmpFilesToCleanup) //no need to check return code, it's just cleanup
		return []byte(""), &InternalServerError{Description: "Couldn't read image: " + err.Error()}
	}

	removeFiles(tmpFilesToCleanup) //no need to check return code, it's just cleanup

	return imgBytes, nil
}

func (a *Api) GetDates(plugins []string) ([]string, error) {

	dates := []string{}
	for _, plugin := range plugins {
		key := plugin + ":date:*"

		res, err := redis.Strings(a.redisConn.Do("KEYS", key))
		if err != nil {
			if err == redis.ErrNil {
				return []string{}, &ItemNotFoundError{Description:"No item with that key found"}
			}

			return []string{}, &InternalServerError{Description: "Couldn't get key: " + err.Error()}
		}

		
		for _, elem := range res {
			date := strings.TrimLeft(elem, plugin + ":date:")
			if !utils.StringInSlice(date, dates) {
				dates = append(dates, date)
			}
		}
	}

	return dates, nil
}

func (a *Api) GetFullDates(plugins []string) ([]string, error) {
	
	fullDates := []string{}

	for _, plugin := range plugins {
		key := plugin + ":fulldate:*"

		res, err := redis.Strings(a.redisConn.Do("KEYS", key))
		if err != nil {
			if err == redis.ErrNil {
				return []string{}, &ItemNotFoundError{Description:"No item with that key found"}
			}

			return []string{}, &InternalServerError{Description: "Couldn't get key: " + err.Error()}
		}

		for _, elem := range res {
			fullDate := strings.TrimLeft(elem, plugin + ":fulldate:")
			if !utils.StringInSlice(fullDate, fullDates) {
				fullDates = append(fullDates, fullDate)
			}
		}
	}

	return fullDates, nil
}

