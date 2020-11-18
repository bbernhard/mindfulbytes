package main

import (
	"github.com/studio-b12/gowebdav"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
	"flag"
	"github.com/gomodule/redigo/redis"
	"github.com/gofrs/uuid"
	"encoding/json"
	"encoding/binary"
	"bytes"
	"os"
)

var TOPIC string = "imgreader-nc"

type FileInfo struct {
	Path string
	ModificationTime time.Time
}

type DataEntry struct {
	Uri string `json:"uri"`
	Uuid string `json:"uuid"`
}

type OutputSplitter struct{}

func (splitter *OutputSplitter) Write(p []byte) (n int, err error) {
	if bytes.Contains(p, []byte("level=error")) {
		return os.Stderr.Write(p)
	}
	return os.Stdout.Write(p)
}

func getRedisAddress() string {
	redisAddress := os.Getenv("REDIS_ADDRESS")
	if redisAddress == "" {
		return ":6379"
	}

	return redisAddress
}

func getFilesRecursively(client *gowebdav.Client, dir string, totalFiles *[]FileInfo) error {
	files, err := client.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, file := range files {
		fullPath := dir 
		if !strings.HasSuffix(fullPath, "/") {
			fullPath += "/"
		}
		fullPath += file.Name()

		log.Debug("Fetching file ", fullPath)
		
		if file.IsDir() {
			getFilesRecursively(client, fullPath, totalFiles)
		} else {
			//we are only interested in images
			contentType := file.(gowebdav.File).ContentType()
			contentTypeParts := strings.Split(contentType, "/")
			if len(contentTypeParts) < 2 {
				log.Debug("Skipping ", fullPath, " as we've got an invalid content type (content type: ", contentType, ")")
				continue
			}

			if contentTypeParts[0] == "image" {
				*totalFiles = append(*totalFiles, FileInfo{Path: fullPath, ModificationTime: file.ModTime()})
			} else {
				log.Debug("Skipping ", fullPath, " as we've got an invalid content type (content type: ", contentType, ")")
			}
		}
	}

	return nil
}


func crawl(redisAddress string, redisMaxConnections int, nextcloudWebDavUrl string, nextcloudAppToken string, nextcloudRootDir string) {
	//create redis pool
	redisPool := redis.NewPool(func() (redis.Conn, error) {
		c, err := redis.Dial("tcp", redisAddress)

		if err != nil {
			log.Fatal("[Main] Couldn't dial redis: ", err.Error())
		}

		return c, err
	}, redisMaxConnections)
	defer redisPool.Close()

	redisConn := redisPool.Get()
	defer redisConn.Close()

	c := gowebdav.NewClient(nextcloudWebDavUrl, "", "")
	c.SetHeader("Authorization", "Bearer " + nextcloudAppToken)
	
	files := []FileInfo{}
	err := getFilesRecursively(c, nextcloudRootDir, &files)
	if err != nil {
		log.Fatal("Couldn't get files: ", err.Error())
	}

	//delete all existing keys
	existingKeys, err := redis.Strings(redisConn.Do("KEYS", TOPIC+":*"))
	if err != nil {
		log.Fatal("Couldn't clear existing keys in redis")
	}
	for _, existingKey := range existingKeys {
		_, err = redisConn.Do("DEL", existingKey)
		if err != nil {
			log.Fatal("Couldn't delete key in redis: ", err.Error())
		}
	}


	imagesPerDate := make(map[string][]DataEntry)
	imagesPerFullDate := make(map[string][]DataEntry)
	for _, file := range files {
		log.Debug("Processing file ", file.Path) 
		u, err := uuid.NewV4()
		if err != nil {
			log.Fatal("Couldn't create UUID: ", err.Error())
		}

		date := file.ModificationTime.Format("01-02")
		fullDate := file.ModificationTime.Format("2006-01-02")
		dataEntry := DataEntry{Uri: file.Path, Uuid: u.String()}

		if _, ok := imagesPerDate[date]; ok {
			imagesPerDate[date] = append(imagesPerDate[date], dataEntry)
		} else {
			imagesPerDate[date] = []DataEntry{dataEntry}
		}

		if _, ok := imagesPerFullDate[fullDate]; ok {
			imagesPerFullDate[fullDate] = append(imagesPerFullDate[fullDate], dataEntry)
		} else {
			imagesPerFullDate[fullDate] = []DataEntry{dataEntry}
		}
		_, err = redisConn.Do("SET", TOPIC+":image:" + u.String(), file.Path)
		if err != nil {
			log.Fatal("Couldn't set data in redis: ", err.Error())
		}
	}

	for key, value := range imagesPerDate {
		serializedData, err := json.Marshal(value)
		if err != nil {
			log.Fatal("Couldn't marshal entry: ", err.Error())
		}

		_, err = redisConn.Do("SET", TOPIC+":date:" + key, serializedData)
		if err != nil {
			log.Fatal("Couldn't set data in redis: ", err.Error())
		}
	}

	for key, value := range imagesPerFullDate {
		serializedData, err := json.Marshal(value)
		if err != nil {
			log.Fatal("Couldn't marshal entry: ", err.Error())
		}

		_, err = redisConn.Do("SET", TOPIC+":fulldate:" + key, serializedData)
		if err != nil {
			log.Fatal("Couldn't set data in redis: ", err.Error())
		}
	}
}

func fetch(redisAddress string, redisMaxConnections int, nextcloudWebDavUrl string, nextcloudAppToken string, id string, destination string) {
	//create redis pool
	redisPool := redis.NewPool(func() (redis.Conn, error) {
		c, err := redis.Dial("tcp", redisAddress)

		if err != nil {
			log.Fatal("[Main] Couldn't dial redis: ", err.Error())
		}

		return c, err
	}, redisMaxConnections)
	defer redisPool.Close()

	redisConn := redisPool.Get()
	defer redisConn.Close()


	key := TOPIC+":image:"+id

	webDavFilePathBytes, err := redis.Bytes(redisConn.Do("GET", key))
	if err != nil {
		log.Fatal("Couldn't read data from redis: ", err.Error())
	}

	webDavFilePath := string(webDavFilePathBytes)
	log.Info(webDavFilePath)
	
	c := gowebdav.NewClient(nextcloudWebDavUrl, "", "")
	c.SetHeader("Authorization", "Bearer " + nextcloudAppToken)

	bytes, err := c.Read(webDavFilePath)
	if err != nil {
		log.Fatal("Couldn't read file ", webDavFilePath, ": ", err.Error())
	}

	f, err := os.Create(destination)
	if err != nil {
		log.Fatal("Couldn't create file ", destination, ": ", err.Error())
	}

	err = binary.Write(f, binary.LittleEndian, bytes)
	if err != nil {
		log.Fatal("Couldn't write file ", destination, ": ", err.Error())
	}
}

func main() {
	crawlCommand := flag.NewFlagSet("crawl", flag.ExitOnError)

	nextcloudWebDavUrlCrawlCmd := crawlCommand.String("nextcloud-webdav-url", "", "Nextcloud Webdav URL")
	nextcloudAppTokenCrawlCmd := crawlCommand.String("nextcloud-token", "", "Nextcloud App Token")
	nextcloudRootDir := crawlCommand.String("nextcloud-root-dir", "", "Nextcloud Root Directory")

	fetchCommand := flag.NewFlagSet("fetch", flag.ExitOnError)
	fetchId := fetchCommand.String("id", "", "Identifier")
	nextcloudWebDavUrlFetchCmd := fetchCommand.String("nextcloud-webdav-url", "", "Nextcloud Webdav URL")
	nextcloudAppTokenFetchCmd := fetchCommand.String("nextcloud-token", "", "Nextcloud App Token")
	destinationFetchCmd := fetchCommand.String("destination", "", "Destination")

	redisMaxConnections := 10

	flag.Parse()

	log.SetLevel(log.DebugLevel)
	log.SetOutput(&OutputSplitter{})

	if len(os.Args) == 1 {
		log.Fatal("Please use either 'crawl' or 'fetch'")
	}

	switch os.Args[1] {
		case "crawl":
			crawlCommand.Parse(os.Args[2:])
			if *nextcloudWebDavUrlCrawlCmd == "" {
				log.Fatal("Please provide a valid Nextcloud webdav URL")
			}


			if *nextcloudAppTokenCrawlCmd == "" {
				log.Fatal("Please provide a valid Nextcloud App token")
			}

			if *nextcloudRootDir == "" {
				log.Fatal("Please specify the Nextcloud root directory")
			}

			crawl(getRedisAddress(), redisMaxConnections, *nextcloudWebDavUrlCrawlCmd, *nextcloudAppTokenCrawlCmd, *nextcloudRootDir)

		case "fetch":
			fetchCommand.Parse(os.Args[2:])
			if *fetchId == "" {
				log.Fatal("Please provide a id")
			}

			if *nextcloudWebDavUrlFetchCmd == "" {
				log.Fatal("Please provide a valid Nextcloud webdav URL")
			}

			if *nextcloudAppTokenFetchCmd == "" {
				log.Fatal("Please provide a valid Nextcloud App token")
			}

			if *destinationFetchCmd == "" {
				log.Fatal("Please provide a destination")
			}

			fetch(getRedisAddress(), redisMaxConnections, *nextcloudWebDavUrlFetchCmd, *nextcloudAppTokenFetchCmd, *fetchId, *destinationFetchCmd)
		default:
			log.Fatal(os.Args[1], " is not valid command.")
	}

}
