package main

import (
	"github.com/gin-gonic/gin"
	"net/http"
	"html/template"
	"os"
	"strings"
	log "github.com/sirupsen/logrus"
	"path/filepath"
	"flag"
	"github.com/gomodule/redigo/redis"
	"github.com/bbernhard/mindfulbytes/api"
	"strconv"
	"time"
	"math/rand"
	"encoding/json"
)

var assetVersion string = ""

func getRandomNumber(max int) int {
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)
	return r1.Intn(max)
}

func GetParamFromUrlParams(c *gin.Context, name string, defaultIfNotFound string) string {
    params := c.Request.URL.Query()

    param := defaultIfNotFound
    if temp, ok := params[name]; ok {
        param = temp[0]
    }

    return param
}

func GetTemplates(path string, funcMap template.FuncMap)  (*template.Template, error) {
    templ := template.New("main").Funcs(funcMap)
    err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
        if strings.Contains(path, ".html") || strings.Contains(path, ".js") {
            _, err = templ.ParseFiles(path)
            if err != nil {
                return err
            }
        }

        return err
    })

    return templ, err
}

func main() {
	configDir := "../config/"

	redisAddress := flag.String("redis-address", ":6379", "Address to the Redis server")
	redisMaxConnections := flag.Int("redis-max-connections", 500, "Max connections to Redis")

	flag.Parse()

	assetVersion = strconv.FormatInt(int64(time.Now().Unix()), 10)

	//create redis pool
	redisPool := redis.NewPool(func() (redis.Conn, error) {
		c, err := redis.Dial("tcp", *redisAddress)

		if err != nil {
			log.Fatal("[Main] Couldn't dial redis: ", err.Error())
		}

		return c, err
	}, *redisMaxConnections)
	defer redisPool.Close()

	redisConn := redisPool.Get()
	defer redisConn.Close()


	apiClient := api.NewApi(redisConn)

	
	var tmpl *template.Template
	var err error
	funcMap := template.FuncMap{
	}

	tmpl, err = GetTemplates("../html", funcMap)
	if err != nil {
		log.Fatal("Couldn't parse templates: ", err.Error())
	}
	
	plugins, err := loadPlugins("./plugins/", configDir)
	if err != nil {
		log.Fatal(err)
	}

	router := gin.Default()

	router.Static("./js", "../js") //serve javascript files
	router.Static("./css", "../css") //serve css files

	router.Static("./img", "../img")

	
	router.SetHTMLTemplate(tmpl)
	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H {
			"assetVersion": assetVersion,
			"baseUrl": "http://127.0.0.1:8085", 
		})
	})

	router.GET("/v1/plugins", func(c *gin.Context) {
		type PluginEntry struct {
			Name string `json:"name"`
		}

		pluginEntries := []PluginEntry{}
		for _, plugin := range plugins {
			pluginEntry := PluginEntry{Name: plugin.Name}
			pluginEntries = append(pluginEntries, pluginEntry) 
		}

		c.JSON(200, pluginEntries)
	})

	router.GET("/v1/plugins/:plugin/dates/:date", func(c *gin.Context) {
		plugin := c.Param("plugin")
		date := c.Param("date")
		data, err := apiClient.GetDataForDate(plugin, date)
		if err != nil {
			switch err.(type) {
				case *api.InternalServerError:
					log.Error(err.Error())
					c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
					return
				case *api.ItemNotFoundError:
					c.JSON(404, gin.H{"error": "No item for that date found"})
					return
			}
		}
		c.String(200, data)
	})

	router.GET("/v1/plugins/:plugin/dates", func(c *gin.Context) {
		plugin := c.Param("plugin")
		
		dates, err := apiClient.GetDates(plugin)
		if err != nil {
			switch err.(type) {
				case *api.InternalServerError:
					log.Error(err.Error())
					c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
					return
				case *api.ItemNotFoundError:
					c.JSON(404, gin.H{"error": "No item for that date found"})
					return
			}
		}

		c.JSON(200, dates)
	})

	router.GET("/v1/plugins/:plugin/days/:day", func(c *gin.Context) {
		plugin := c.Param("plugin")
		day := c.Param("day")
		

		data, err := apiClient.GetDataForDay(plugin, day)
		if err != nil {
			switch err.(type) {
				case *api.InternalServerError:
					log.Error(err.Error())
					c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
					return
				case *api.ItemNotFoundError:
					c.JSON(404, gin.H{"error": "No item for that date found"})
					return
			}
		}

		c.JSON(200, data)
	})

	router.GET("/v1/plugins/:plugin/images/:imageid", func(c *gin.Context) {
		plugin := c.Param("plugin")
		imageId := c.Param("imageid")

		var imgBytes []byte
		if imageId == "random" {
			dates, err := apiClient.GetDates(plugin)
			if err != nil {
				log.Error(err.Error())
				c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
				return
			}

			if len(dates) == 0 {
				c.JSON(400, gin.H{"error": "No images for plugin " + plugin + " found"})
				return
			}

			type PluginDataEntry struct {
				Uuid string `json:"uuid"`
			}

			randomNum := getRandomNumber(len(dates))
			randomDate := dates[randomNum]

			data, err := apiClient.GetDataForDate(plugin, randomDate)
			if err != nil {
				log.Error(err.Error())
				c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
				return
			}

			var dataEntry PluginDataEntry
			err = json.Unmarshal([]byte(data), &dataEntry)
			if err != nil {
				log.Error(err.Error())
				c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
				return
			}
			imageId = dataEntry.Uuid
		}

		imgBytes, err := apiClient.GetImage(plugin, imageId)
		if err != nil {
			switch err.(type) {
				case *api.InternalServerError:
					log.Error(err.Error())
					c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
					return
				case *api.ItemNotFoundError:
					c.JSON(404, gin.H{"error": "No item for that date found"})
					return
			}
		}

		format := http.DetectContentType(imgBytes)

		c.Writer.Header().Set("Content-Type", format)
		c.Writer.Header().Set("Content-Length", strconv.Itoa(len(imgBytes)))
		_, err = c.Writer.Write(imgBytes)
		if err != nil {
			log.Error("Couldn't serve image: ", err.Error())
			return
		}
	})


	router.Run(":8085")
}
