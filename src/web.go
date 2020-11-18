package main

import (
	"flag"
	"github.com/bbernhard/mindfulbytes/api"
	"github.com/bbernhard/mindfulbytes/utils"
	"github.com/gin-gonic/gin"
	"github.com/gomodule/redigo/redis"
	log "github.com/sirupsen/logrus"
	timeago "github.com/xeonx/timeago"
	"html/template"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
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

func deliverImage(c *gin.Context, apiClient *api.Api, plugins []string, imageId string) {
	grayscale := false
	if c.DefaultQuery("grayscale", "false") == "true" {
		grayscale = true
	}

	caption := c.DefaultQuery("caption", "")

	autoCaption := c.DefaultQuery("auto_caption", "false")

	if autoCaption == "true" && caption != "" {
		c.JSON(400, gin.H{"error": "Auto caption and caption cannot be set at the same time"})
		return
	}

	if autoCaption == "true" && imageId != "random" {
		c.JSON(400, gin.H{"error": "Auto caption not yet supported here"})
		return
	}

	size := c.DefaultQuery("size", "")
	if size != "" {
		sizes := strings.Split(size, "x")

		if len(sizes) != 2 {
			c.JSON(400, gin.H{"error": "Couldn't process request - invalid image size"})
			return
		}

		_, err := strconv.Atoi(sizes[0])
		if err != nil {
			c.JSON(400, gin.H{"error": "Couldn't process request - invalid image width"})
			return
		}

		_, err = strconv.Atoi(sizes[1])
		if err != nil {
			c.JSON(400, gin.H{"error": "Couldn't process request - invalid image height"})
			return
		}
	}

	format := c.DefaultQuery("format", "jpg")

	plugin := ""
	var imgBytes []byte
	if imageId == "random" {
		fullDates, err := apiClient.GetFullDates(plugins)
		if err != nil {
			log.Error(err.Error())
			c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
			return
		}

		if len(fullDates) == 0 {
			c.JSON(400, gin.H{"error": "No images for plugin(s) " + strings.Join(plugins, ",") + " found"})
			return
		}

		randomNum := getRandomNumber(len(fullDates))
		randomFullDate := fullDates[randomNum]

		if autoCaption == "true" {
			timeLayout := "2006-01-02"

			timeagoGermanConfig := timeago.NoMax(timeago.German)
			timeagoGermanConfig.DefaultLayout = timeLayout

			fullDate, err := time.Parse(timeLayout, fullDates[randomNum])
			if err != nil {
				log.Error(err.Error())
				c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
				return
			}
			caption = timeagoGermanConfig.FormatReference(fullDate, time.Now())
		}

		dataEntries, err := apiClient.GetDataForFullDate(plugins, randomFullDate)
		if err != nil {
			log.Error(err.Error())
			c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
			return
		}

		if len(dataEntries) == 0 {
			c.JSON(404, gin.H{"error": "No images found"})
			return
		}

		randomNum = getRandomNumber(len(dataEntries))

		imageId = dataEntries[randomNum].Uuid
		plugin = dataEntries[randomNum].Plugin
	} else {
		plugin = plugins[0]
	}

	if plugin == "" {
		c.JSON(404, gin.H{"error": "No plugin specified"})
		return
	}

	convertOptions := utils.ConvertOptions{Size: size, Caption: caption, Grayscale: grayscale, Format: format}
	imgBytes, mimeType, err := apiClient.GetImage(plugin, imageId, convertOptions)
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

	c.Writer.Header().Set("Content-Type", mimeType)
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(imgBytes)))
	_, err = c.Writer.Write(imgBytes)
	if err != nil {
		log.Error("Couldn't serve image: ", err.Error())
		return
	}
}

func GetTemplates(path string, funcMap template.FuncMap) (*template.Template, error) {
	templ := template.New("main").Funcs(funcMap)
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if strings.Contains(path, ".html") {
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

	redisAddress := flag.String("redis-address", "127.0.0.1:6379", "Address to the Redis server")
	redisMaxConnections := flag.Int("redis-max-connections", 500, "Max connections to Redis")
	tmpDir := flag.String("tmp-dir", "/tmp", "Tmp directory")

	if *tmpDir == "" {
		log.Fatal("Please provide a valid tmp-dir")
	}

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

	plugins := utils.NewPlugins("./plugins/", configDir)
	err := plugins.Load()
	if err != nil {
		log.Fatal(err)
	}

	imageMagickWrapper := utils.NewImageMagickWrapper("/usr/bin/magick", *tmpDir+"/")
	apiClient := api.NewApi(redisConn, imageMagickWrapper, plugins, *tmpDir)

	var tmpl *template.Template
	funcMap := template.FuncMap{}

	tmpl, err = GetTemplates("../html", funcMap)
	if err != nil {
		log.Fatal("Couldn't parse templates: ", err.Error())
	}

	router := gin.Default()

	router.Static("./js", "../js")   //serve javascript files
	router.Static("./css", "../css") //serve css files

	router.Static("./img", "../img")

	router.SetHTMLTemplate(tmpl)
	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"assetVersion": assetVersion,
			"baseUrl":      "http://127.0.0.1:8085",
		})
	})

	router.GET("/v1/topics", func(c *gin.Context) {
		topics := plugins.GetTopics()

		c.JSON(200, topics)
	})

	router.GET("/v1/topics/:topic/dates", func(c *gin.Context) {
		topic := c.Param("topic")

		topics := plugins.GetTopics()
		plugins, exists := topics[topic]
		if !exists {
			c.JSON(404, gin.H{"error": "No plugins for that topic found"})
			return
		}

		dates, err := apiClient.GetDates(plugins)
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

	router.GET("/v1/topics/:topic/fulldates", func(c *gin.Context) {
		topic := c.Param("topic")

		topics := plugins.GetTopics()
		plugins, exists := topics[topic]
		if !exists {
			c.JSON(404, gin.H{"error": "No plugins for that topic found"})
			return
		}

		fullDates, err := apiClient.GetFullDates(plugins)
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

		c.JSON(200, fullDates)
	})

	router.GET("/v1/topics/:topic/fulldates/:fulldate", func(c *gin.Context) {
		topic := c.Param("topic")
		fullDate := c.Param("fulldate")

		topics := plugins.GetTopics()
		plugins, exists := topics[topic]
		if !exists {
			c.JSON(404, gin.H{"error": "No plugins for that topic found"})
			return
		}

		data, err := apiClient.GetDataForFullDate(plugins, fullDate)
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

	router.GET("/v1/topics/:topic/dates/:date", func(c *gin.Context) {
		topic := c.Param("topic")
		date := c.Param("date")

		topics := plugins.GetTopics()
		plugins, exists := topics[topic]
		if !exists {
			c.JSON(404, gin.H{"error": "No plugins for that topic found"})
			return
		}

		data, err := apiClient.GetDataForDate(plugins, date)
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

	router.GET("/v1/topics/:topic/images/random", func(c *gin.Context) {
		topic := c.Param("topic")

		topics := plugins.GetTopics()
		plugins, exists := topics[topic]
		if !exists {
			c.JSON(404, gin.H{"error": "No plugins for that topic found"})
			return
		}

		deliverImage(c, apiClient, plugins, "random")
	})

	router.GET("/v1/plugins", func(c *gin.Context) {
		type PluginEntry struct {
			Name string `json:"name"`
		}

		pluginEntries := []PluginEntry{}
		for _, plugin := range plugins.GetPlugins() {
			pluginEntry := PluginEntry{Name: plugin.Name}
			pluginEntries = append(pluginEntries, pluginEntry)
		}

		c.JSON(200, pluginEntries)
	})

	router.GET("/v1/plugins/:plugin/fulldates/:fulldate", func(c *gin.Context) {
		plugin := c.Param("plugin")
		fullDate := c.Param("fulldate")
		data, err := apiClient.GetDataForFullDate([]string{plugin}, fullDate)
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

	router.GET("/v1/plugins/:plugin/fulldates", func(c *gin.Context) {
		plugin := c.Param("plugin")

		fullDates, err := apiClient.GetFullDates([]string{plugin})
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

		c.JSON(200, fullDates)
	})

	router.GET("/v1/plugins/:plugin/dates", func(c *gin.Context) {
		plugin := c.Param("plugin")

		dates, err := apiClient.GetDates([]string{plugin})
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

	router.GET("/v1/plugins/:plugin/dates/:date", func(c *gin.Context) {
		plugin := c.Param("plugin")
		date := c.Param("date")

		data, err := apiClient.GetDataForDate([]string{plugin}, date)
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

		deliverImage(c, apiClient, []string{plugin}, imageId)
	})

	router.Run(":8085")
}
