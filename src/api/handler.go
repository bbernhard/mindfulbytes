package api

import (
	"github.com/gin-gonic/gin"
	"github.com/bbernhard/mindfulbytes/utils"
	log "github.com/sirupsen/logrus"
	"strings"
	"strconv"
	"time"
	"github.com/gofrs/uuid"
	"github.com/gabriel-vasile/mimetype"
	"bytes"
)

type ImageFetchError struct {
	Description string
	StatusCode int
}

func (e *ImageFetchError) Error() string {
	return e.Description
}

func parseGetImageRequest(c *gin.Context, apiClient *Api, plugins []string, imageId string) (string, string, utils.ConvertOptions, error) {
	mode := c.DefaultQuery("mode", "rgb")
	
	grayscale := false
	if mode == "grayscale" {
		grayscale = true
	}

	caption := c.DefaultQuery("caption", "")

	size := c.DefaultQuery("size", "")
	if size != "" {
		sizes := strings.Split(size, "x")

		if len(sizes) != 2 {
			return "", "", utils.ConvertOptions{}, &ImageFetchError{StatusCode: 400, Description: "Couldn't process request - invalid image size"}
		}

		_, err := strconv.Atoi(sizes[0])
		if err != nil {
			return "", "", utils.ConvertOptions{}, &ImageFetchError{StatusCode: 400, Description: "Couldn't process request - invalid image width"}
		}

		_, err = strconv.Atoi(sizes[1])
		if err != nil {
			return "", "", utils.ConvertOptions{}, &ImageFetchError{StatusCode: 400, Description: "Couldn't process request - invalid image height"}
		}
	}

	format := c.DefaultQuery("format", "jpg")
	language := c.DefaultQuery("language", "en")

	plugin := ""
	if len(plugins) > 0 {
		plugin = plugins[0]
	}

	fullDate := ""
	if imageId == "today-or-random" {
		currentDate := time.Now()
		currentDateStr := currentDate.Format("01-02")
		todaysEntries, err := apiClient.GetDataForDate(plugins, currentDateStr)
		if err != nil {
			return "", "", utils.ConvertOptions{}, &ImageFetchError{StatusCode: 500, Description: "Couldn't process request - please try again later"}
		}
		if len(todaysEntries) > 0 {
			randomNum:= utils.GetRandomNumber(len(todaysEntries))
			imageId = todaysEntries[randomNum].Uuid
			plugin = todaysEntries[randomNum].Plugin
			fullDate = todaysEntries[randomNum].FullDate
		} else {
			imageId = "random"
		}
	}


	if imageId == "random" {
		fullDates, err := apiClient.GetFullDates(plugins)
		if err != nil {
			log.Error(err.Error())
			return "", "", utils.ConvertOptions{}, &ImageFetchError{StatusCode: 500, Description: "Couldn't process request - please try again later"}
		}

		if len(fullDates) == 0 {
			return "", "", utils.ConvertOptions{}, &ImageFetchError{StatusCode: 400, Description: "No images for plugin(s) " + strings.Join(plugins, ",") + " found"}
		}

		randomNum := utils.GetRandomNumber(len(fullDates))
		randomFullDate := fullDates[randomNum]
 
		dataEntries, err := apiClient.GetDataForFullDate(plugins, randomFullDate)
		if err != nil {
			log.Error(err.Error())
			return "", "", utils.ConvertOptions{}, &ImageFetchError{StatusCode: 500, Description: "Couldn't process request - please try again later"}
		}

		if len(dataEntries) == 0 {
			return "", "", utils.ConvertOptions{}, &ImageFetchError{StatusCode: 404, Description: "No images found"}
		}

		randomNum = utils.GetRandomNumber(len(dataEntries))

		imageId = dataEntries[randomNum].Uuid
		plugin = dataEntries[randomNum].Plugin
		fullDate = dataEntries[randomNum].FullDate
	}

	if fullDate != "" && caption != "" {
		d, err := utils.ConvertFullDateToTime(fullDate)
		if err != nil {
			return "", "", utils.ConvertOptions{}, &ImageFetchError{StatusCode: 500, Description: "Couldn't process request - please try again later"}
		}

		caption, err = utils.ReplaceTagsInMessage(caption, d, language)
		if err != nil {
			return "", "", utils.ConvertOptions{}, &ImageFetchError{StatusCode: 500, Description: "Couldn't process request - please try again later"}
		}
	}

	if plugin == "" {
		return "", "", utils.ConvertOptions{}, &ImageFetchError{StatusCode: 404, Description: "No plugin specified"}
	}

	convertOptions := utils.ConvertOptions{Size: size, Caption: caption, Grayscale: grayscale, Format: format}
	return plugin, imageId, convertOptions, nil
}

func getImage(apiClient *Api, plugin string, imageId string, convertOptions utils.ConvertOptions) ([]byte, string, error) {
	imgBytes, mimeType, err := apiClient.GetImage(plugin, imageId, convertOptions)
	return imgBytes, mimeType, err
}

func deliverImage(c *gin.Context, apiClient *Api, plugins []string, imageId string) {
	plugin, imageId, convertOptions, err := parseGetImageRequest(c, apiClient, plugins, imageId)
	if err != nil {
		switch err.(type) {
		case *ImageFetchError:
			c.JSON(err.(*ImageFetchError).StatusCode, gin.H{"error": err.Error()})
		default:
			c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
			return
		}
	}


	imgBytes, mimeType, err := getImage(apiClient, plugin, imageId, convertOptions)
	if err != nil {
		switch err.(type) {
		case *InternalServerError:
			log.Error(err.Error())
			c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
			return
		case *ItemNotFoundError:
			c.JSON(404, gin.H{"error": "No item for that date found"})
			return
		default:
			c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
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


type RequestHandler struct {
	apiClient *Api
	plugins *utils.Plugins
}

func NewRequestHandler(apiClient *Api, plugins *utils.Plugins) *RequestHandler {
	return &RequestHandler{
		apiClient: apiClient,
		plugins: plugins,
	}
}

// @Summary List all topics
// @Tags General
// @Description List all registered topics.
// @Produce  json
// @Success 200 {object} []string
// @Router /v1/topics [get]
func (h *RequestHandler) GetTopics(c *gin.Context) {
	topics := h.plugins.GetTopics()

	c.JSON(200, topics)
}

// @Summary List all dates for topic
// @Tags General
// @Description List all dates for a specific topic.
// @Produce  json
// @Success 200 {object} []string
// @Param topic path string true "Topic"
// @Router /v1/topics/{topic}/dates [get]
func (h *RequestHandler) GetDatesForTopic(c *gin.Context) {
	topic := c.Param("topic")

	topics := h.plugins.GetTopics()
	plugins, exists := topics[topic]
	if !exists {
		c.JSON(404, gin.H{"error": "No plugins for that topic found"})
		return
	}

	dates, err := h.apiClient.GetDates(plugins)
	if err != nil {
		switch err.(type) {
		case *InternalServerError:
			log.Error(err.Error())
			c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
			return
		case *ItemNotFoundError:
			c.JSON(404, gin.H{"error": "No item for that date found"})
			return
		default:
			c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
			return
		}
	}

	c.JSON(200, dates)
}

// @Summary List all dates (YYYY-MM-DD) for topic
// @Tags General
// @Description List all dates (in the form YYYY-MM-DD) for a given topic.
// @Produce  json
// @Success 200 {object} []string
// @Param topic path string true "Topic"
// @Router /v1/topics/{topic}/fulldates [get]
func (h *RequestHandler) GetFullDatesForTopic(c *gin.Context) {
	topic := c.Param("topic")

	topics := h.plugins.GetTopics()
	plugins, exists := topics[topic]
	if !exists {
		c.JSON(404, gin.H{"error": "No plugins for that topic found"})
		return
	}

	fullDates, err := h.apiClient.GetFullDates(plugins)
	if err != nil {
		switch err.(type) {
		case *InternalServerError:
			log.Error(err.Error())
			c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
			return
		case *ItemNotFoundError:
			c.JSON(404, gin.H{"error": "No item for that date found"})
			return
		default:
			c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
			return
		}
	}

	c.JSON(200, fullDates)
}

// @Summary List all entries for a given date (YYYY-MM-DD) and topic
// @Tags General
// @Description List all entries for a given date (YYYY-MM-DD) and topic.
// @Produce  json
// @Success 200 {object} []Entry
// @Param topic path string true "Topic"
// @Param fulldate path string true "Date (YYYY-MM-DD)"
// @Router /v1/topics/{topic}/fulldates/{fulldate} [get]
func (h *RequestHandler) GetFullDateDataForTopic(c *gin.Context) {
	topic := c.Param("topic")
	fullDate := c.Param("fulldate")

	topics := h.plugins.GetTopics()
	plugins, exists := topics[topic]
	if !exists {
		c.JSON(404, gin.H{"error": "No plugins for that topic found"})
		return
	}

	data, err := h.apiClient.GetDataForFullDate(plugins, fullDate)
	if err != nil {
		switch err.(type) {
		case *InternalServerError:
			log.Error(err.Error())
			c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
			return
		case *ItemNotFoundError:
			c.JSON(404, gin.H{"error": "No item for that date found"})
			return
		default:
			c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
			return
		}
	}

	c.JSON(200, data)
}

// @Summary List all entries for a given date (MM-DD) and topic
// @Tags General
// @Description List all entries for a given date (MM-DD) and topic.
// @Produce  json
// @Success 200 {object} []Entry
// @Param topic path string true "Topic"
// @Param date path string true "Date (MM-DD)"
// @Router /v1/topics/{topic}/dates/{date} [get]
func (h *RequestHandler) GetDateDataForTopic(c *gin.Context) {
	topic := c.Param("topic")
	date := c.Param("date")

	topics := h.plugins.GetTopics()
	plugins, exists := topics[topic]
	if !exists {
		c.JSON(404, gin.H{"error": "No plugins for that topic found"})
		return
	}

	data, err := h.apiClient.GetDataForDate(plugins, date)
	if err != nil {
		switch err.(type) {
		case *InternalServerError:
			log.Error(err.Error())
			c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
			return
		case *ItemNotFoundError:
			c.JSON(404, gin.H{"error": "No item for that date found"})
			return
		default:
			c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
			return
		}
	}

	c.JSON(200, data)
}

// @Summary Get random image for given topic
// @Tags General
// @Description Get random image for given topic. 
// @Produce  json
// @Success 200 {object} []byte
// @Param topic path string true "Topic"
// @Router /v1/topics/{topic}/images/random [get]
func (h *RequestHandler) GetRandomImageForTopic(c *gin.Context) {
	topic := c.Param("topic")

	topics := h.plugins.GetTopics()
	plugins, exists := topics[topic]
	if !exists {
		c.JSON(404, gin.H{"error": "No plugins for that topic found"})
		return
	}

	deliverImage(c, h.apiClient, plugins, "random")
}

// @Summary Get image for given topic that was created at this day x years ago or a random image.
// @Tags General
// @Description Get Image for given topic that was created at this day x years ago. If no image is found, a random image is picked.
// @Produce  json
// @Success 200 {object} []byte
// @Param topic path string true "Topic"
// @Router /v1/topics/{topic}/images/today-or-random [get]
func (h *RequestHandler) GetTodayOrRandomImageForTopic(c *gin.Context) {
	topic := c.Param("topic")

	topics := h.plugins.GetTopics()
	plugins, exists := topics[topic]
	if !exists {
		c.JSON(404, gin.H{"error": "No plugins for that topic found"})
		return
	}

	deliverImage(c, h.apiClient, plugins, "today-or-random")
}

type PluginEntry struct {
	Name string `json:"name"`
}

// @Summary List all plugins 
// @Tags General
// @Description List all plugins.
// @Produce  json
// @Success 200 {object} []PluginEntry
// @Router /v1/plugins [get]
func (h *RequestHandler) GetPlugins(c *gin.Context) {
	pluginEntries := []PluginEntry{}
	for _, plugin := range h.plugins.GetPlugins() {
		pluginEntry := PluginEntry{Name: plugin.Name}
		pluginEntries = append(pluginEntries, pluginEntry)
	}

	c.JSON(200, pluginEntries)
}

// @Summary List all entries for a given date (YYYY-MM-DD) and plugin 
// @Tags General
// @Description List all entries for a given date (YYYY-MM-DD) and plugin.
// @Produce  json
// @Success 200 {object} []Entry
// @Param plugin path string true "Plugin"
// @Param fulldate path string true "Date (YYYY-MM-DD)"
// @Router /v1/plugins/{plugin}/fulldates/{fulldate} [get]
func (h *RequestHandler) GetFullDateDataForPlugin(c *gin.Context) {
	plugin := c.Param("plugin")
	fullDate := c.Param("fulldate")
	data, err := h.apiClient.GetDataForFullDate([]string{plugin}, fullDate)
	if err != nil {
		switch err.(type) {
		case *InternalServerError:
			log.Error(err.Error())
			c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
			return
		case *ItemNotFoundError:
			c.JSON(404, gin.H{"error": "No item for that date found"})
			return
		default:
			c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
			return
		}
	}

	c.JSON(200, data)
}


// @Summary List all dates (YYYY-MM-DD) for plugin 
// @Tags General
// @Description List all dates (YYYY-MM-DD) for a specific plugin.
// @Produce  json
// @Success 200 {object} []string
// @Param plugin path string true "Plugin"
// @Router /v1/plugins/{plugin}/fulldates [get]
func (h *RequestHandler) GetFullDatesForPlugin(c *gin.Context) {
	plugin := c.Param("plugin")

	fullDates, err := h.apiClient.GetFullDates([]string{plugin})
	if err != nil {
		switch err.(type) {
		case *InternalServerError:
			log.Error(err.Error())
			c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
			return
		case *ItemNotFoundError:
			c.JSON(404, gin.H{"error": "No item for that date found"})
			return
		default:
			c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
			return
		}
	}

	c.JSON(200, fullDates)
}

// @Summary List all dates (MM-DD) for plugin 
// @Tags General
// @Description List all dates (MM-DD) for a specific plugin.
// @Produce  json
// @Success 200 {object} []string
// @Param plugin path string true "Plugin"
// @Router /v1/plugins/{plugin}/dates [get]
func (h *RequestHandler) GetDatesForPlugin(c *gin.Context) {
	plugin := c.Param("plugin")

	dates, err := h.apiClient.GetDates([]string{plugin})
	if err != nil {
		switch err.(type) {
		case *InternalServerError:
			log.Error(err.Error())
			c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
			return
		case *ItemNotFoundError:
			c.JSON(404, gin.H{"error": "No item for that date found"})
			return
		default:
			c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
			return
		}
	}

	c.JSON(200, dates)
}

// @Summary List all entries for a given date (MM-DD) and plugin 
// @Tags General
// @Description List all entries for a given date (MM-DD) and plugin.
// @Produce  json
// @Success 200 {object} []Entry
// @Param plugin path string true "Plugin"
// @Param date path string true "Date"
// @Router /v1/plugins/{plugin}/dates/{date} [get]
func (h *RequestHandler) GetDateDataForPlugin(c *gin.Context) {
	plugin := c.Param("plugin")
	date := c.Param("date")

	data, err := h.apiClient.GetDataForDate([]string{plugin}, date)
	if err != nil {
		switch err.(type) {
		case *InternalServerError:
			log.Error(err.Error())
			c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
			return
		case *ItemNotFoundError:
			c.JSON(404, gin.H{"error": "No item for that date found"})
			return
		default:
			c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
			return
		}
	}

	c.JSON(200, data)
}

// @Summary Get image with given identifier in plugin 
// @Tags General
// @Description Get image with given identifier in plugin.
// @Produce  json
// @Success 200 {object} []byte
// @Param plugin path string true "Plugin"
// @Param imageid path string true "Image UUID"
// @Router /v1/plugins/{plugin}/images/{imageid} [get]
func (h *RequestHandler) GetImageForPlugin(c *gin.Context) {
	plugin := c.Param("plugin")
	imageId := c.Param("imageid")

	deliverImage(c, h.apiClient, []string{plugin}, imageId)
}

func (h *RequestHandler) CacheTodayOrRandomImage(c *gin.Context) {
	h.CacheImage(c, "today-or-random")
}

func (h *RequestHandler) CacheImage(c *gin.Context, imageId string) {
	topic := c.Param("topic")

	topics := h.plugins.GetTopics()
	plugins, exists := topics[topic]
	if !exists {
		c.JSON(404, gin.H{"error": "No plugins for that topic found"})
		return
	}
	
	plugin, imageId, convertOptions, err := parseGetImageRequest(c, h.apiClient, plugins, imageId)
	if err != nil {
		switch err.(type) {
		case *ImageFetchError:
			c.JSON(err.(*ImageFetchError).StatusCode, gin.H{"error": err.Error()})
		default:
			c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
			return
		}
	}

	cacheId, err := uuid.NewV4()
	if err != nil {
		c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
		return
	}

	c.JSON(201, gin.H{"cacheid": cacheId.String()})

	go func(apiClient *Api, plugin string, imageId string, convertOptions utils.ConvertOptions, cacheId string) {
		imgBytes, _, err := getImage(apiClient, plugin, imageId, convertOptions)
		if err == nil {
			err = apiClient.CacheEntry(cacheId, imgBytes)
			if err != nil {
				log.Error("Couldn't create cache entry with id ", cacheId, ": ", err.Error())
			}
		} else {
			log.Error("Couldn't create cache entry with id ", cacheId, ": ", err.Error())
		}
	}(h.apiClient, plugin, imageId, convertOptions, cacheId.String())

}

func (h *RequestHandler) GetCachedEntry(c *gin.Context) {
	cacheId := c.Param("cacheid")
	imgBytes, err := h.apiClient.GetCachedEntry(cacheId)
	if err != nil {
		switch err.(type) {
		case *InternalServerError:
			log.Error(err.Error())
			c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
			return
		case *ItemNotFoundError:
			c.JSON(404, gin.H{"error": "No cache entry with that id found"})
			return
		default:
			c.JSON(500, gin.H{"error": "Couldn't process request - please try again later"})
			return
		}
	}

	mime, err := mimetype.DetectReader(bytes.NewReader(imgBytes))
	if err != nil {
		log.Error(err.Error())
		c.JSON(500, gin.H{"error": "Couldn't detect MIME type for cache entry with id " + cacheId})
		return
	}
	
	c.Writer.Header().Set("Content-Type", mime.String())
	c.Writer.Header().Set("Content-Length", strconv.Itoa(len(imgBytes)))
	_, err = c.Writer.Write(imgBytes)
	if err != nil {
		log.Error("Couldn't serve image: ", err.Error())
		return
	}

}
