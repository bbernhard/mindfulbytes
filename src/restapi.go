package main

import (
	"flag"
	"github.com/bbernhard/mindfulbytes/api"
	"github.com/bbernhard/mindfulbytes/utils"
	"github.com/gin-gonic/gin"
	"github.com/gomodule/redigo/redis"
	log "github.com/sirupsen/logrus"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	_ "github.com/bbernhard/mindfulbytes/docs"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var assetVersion string = ""



func GetParamFromUrlParams(c *gin.Context, name string, defaultIfNotFound string) string {
	params := c.Request.URL.Query()

	param := defaultIfNotFound
	if temp, ok := params[name]; ok {
		param = temp[0]
	}

	return param
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

// @title MindfulBytes REST API
// @version 1.0
// @description This is the MindfulBytes API documentation.

// @tag.name General
// @tag.description List general information.

// @host 127.0.0.1:8085
// @BasePath /
func main() {
	configDir := "../config/"

	redisAddress := flag.String("redis-address", "127.0.0.1:6379", "Address to the Redis server")
	redisMaxConnections := flag.Int("redis-max-connections", 500, "Max connections to Redis")
	baseUrl := flag.String("base-url", "http://127.0.0.1:8085", "Base URL")
	tmpDir := flag.String("tmp-dir", "/tmp", "Tmp directory")

	flag.Parse()

	log.SetLevel(log.DebugLevel)
	log.SetOutput(&utils.LogOutputSplitter{})
	
	if *tmpDir == "" {
		log.Fatal("Please provide a valid tmp-dir")
	}

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

	plugins := utils.NewPlugins("./plugins/", configDir)
	err := plugins.Load()
	if err != nil {
		log.Fatal(err)
	}

	imageMagickWrapper := utils.NewImageMagickWrapper("/usr/bin/magick", *tmpDir+"/")
	apiClient := api.NewApi(redisPool, imageMagickWrapper, plugins, *tmpDir)
	requestHandler := api.NewRequestHandler(apiClient, plugins)

	var tmpl *template.Template
	funcMap := template.FuncMap{}

	tmpl, err = GetTemplates("../html", funcMap)
	if err != nil {
		log.Fatal("Couldn't parse templates: ", err.Error())
	}

	router := gin.Default()

	router.Static("./js", "../js")   //serve javascript files
	router.Static("./css", "../css") //serve css files

	//router.Static("./img", "../img")

	router.SetHTMLTemplate(tmpl)
	router.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", gin.H{
			"assetVersion": assetVersion,
			"baseUrl":      *baseUrl,
		})
	})

	v1 := router.Group("/v1")
	{
		topicsGroup := v1.Group("/topics")
		{
			topicsGroup.GET("", requestHandler.GetTopics)
			topicsGroup.GET("/:topic/dates", requestHandler.GetDatesForTopic)
			topicsGroup.GET("/:topic/fulldates", requestHandler.GetFullDatesForTopic)
			topicsGroup.GET("/:topic/fulldates/:fulldate", requestHandler.GetFullDateDataForTopic)
			topicsGroup.GET("/:topic/dates/:date", requestHandler.GetDateDataForTopic)
			topicsGroup.GET("/:topic/images/random", requestHandler.GetRandomImageForTopic)
			topicsGroup.GET("/:topic/images/today-or-random", requestHandler.GetTodayOrRandomImageForTopic)
			topicsGroup.POST("/:topic/images/today-or-random/cache", requestHandler.CacheTodayOrRandomImage)
		}

		pluginsGroup := v1.Group("/plugins")
		{
			pluginsGroup.GET("", requestHandler.GetPlugins)
			pluginsGroup.GET("/:plugin/dates", requestHandler.GetDatesForPlugin)
			pluginsGroup.GET("/:plugin/dates/:date", requestHandler.GetDateDataForPlugin)
			pluginsGroup.GET("/:plugin/fulldates", requestHandler.GetFullDatesForPlugin)
			pluginsGroup.GET("/:plugin/fulldates/:fulldate", requestHandler.GetFullDateDataForPlugin)
			pluginsGroup.GET("/:plugin/images/:imageid", requestHandler.GetImageForPlugin)
		}

		cacheGroup := v1.Group("/cache")
		{
			cacheGroup.GET("/:cacheid", requestHandler.GetCachedEntry)
			cacheGroup.GET("", requestHandler.GetCacheEntries)
		}
	}

	swaggerUrl := ginSwagger.URL("http://127.0.0.1:8085/swagger/doc.json")
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, swaggerUrl))

	router.Run(":8085")
}
