package api

import (
	"github.com/go-resty/resty/v2"
	"strconv"
	"errors"
	"encoding/json"
	"time"
	"github.com/bbernhard/mindfulbytes/utils"
)


type ExternalApiClient struct {
	restClient *resty.Client
	fullAddress string
	address string
	version int64
}

func NewExternalApiClient(address string, version int64) *ExternalApiClient {
	return &ExternalApiClient{
		address: address,
		fullAddress: address + "/v" + strconv.FormatInt(version, 10),
		version: version,
		restClient: resty.New(),
	}
}

func (e *ExternalApiClient) GetRandomImage(topic string) ([]byte, error) {
	url := e.fullAddress + "/topics/" + topic + "/images/random"
	resp, err := e.restClient.R().
    	SetHeader("Content-Type", "application/json").
		Get(url)
	if err != nil {
		return []byte{}, err
	}

	if resp.StatusCode() != 200 {
		return []byte{}, errors.New("Couldn't get random image")
	}
	return resp.Body(), nil
}

func (e *ExternalApiClient) GetTodayOrRandomImage(topic string) ([]byte, error) {
	url := e.fullAddress + "/topics/" + topic + "/images/today-or-random"
	resp, err := e.restClient.R().
    	SetHeader("Content-Type", "application/json").
		Get(url)
	if err != nil {
		return []byte{}, err
	}

	if resp.StatusCode() != 200 {
		return []byte{}, errors.New("Couldn't get today or random image")
	}
	return resp.Body(), nil
}


func (e *ExternalApiClient) GetDataForDate(date string, topic string) ([]Entry, error) {
	url := e.fullAddress + "/topics/" + topic + "/dates/" + date
	resp, err := e.restClient.R().
    	SetHeader("Content-Type", "application/json").
		Get(url)
	if err != nil {
		return []Entry{}, err
	}

	if resp.StatusCode() != 200 {
		return []Entry{}, errors.New("Couldn't get data for date")
	}
	
	var entries []Entry
	err = json.Unmarshal(resp.Body(), &entries)
	if err != nil {
		return entries, err
	}

	return entries, nil
}

func (e *ExternalApiClient) GetFullDates(topic string) ([]string, error) {
	url := e.fullAddress + "/topics/" + topic + "/fulldates"
	resp, err := e.restClient.R().
    	SetHeader("Content-Type", "application/json").
		Get(url)
	if err != nil {
		return []string{}, err
	}

	if resp.StatusCode() != 200 {
		return []string{}, errors.New("Couldn't get fulldates")
	}
	
	var entries []string
	err = json.Unmarshal(resp.Body(), &entries)
	if err != nil {
		return entries, err
	}

	return entries, nil
}

func (e *ExternalApiClient) GetDataForFullDate(fullDate string, topic string) ([]Entry, error) {
	url := e.fullAddress + "/topics/" + topic + "/fulldates/" + fullDate
	resp, err := e.restClient.R().
    	SetHeader("Content-Type", "application/json").
		Get(url)
	if err != nil {
		return []Entry{}, err
	}

	if resp.StatusCode() != 200 {
		return []Entry{}, errors.New("Couldn't get date for fulldate")
	}
	
	var entries []Entry
	err = json.Unmarshal(resp.Body(), &entries)
	if err != nil {
		return entries, err
	}

	return entries, nil
}

func (e *ExternalApiClient) GetImage(plugin string, imageId string) ([]byte, error) {
	url := e.fullAddress + "/plugins/" + plugin + "/images/" + imageId
	resp, err := e.restClient.R().
    	SetHeader("Content-Type", "application/json").
		Get(url)
	if err != nil {
		return []byte{}, err
	}

	if resp.StatusCode() != 200 {
		return []byte{}, errors.New("Couldn't get image")
	}

	return resp.Body(), nil
}

type ImageData struct {
	Image []byte
	FullDate string
}

func (e *ExternalApiClient) GetImageTodayOrRandomWithData(topic string) (ImageData, error) {
	imageData := ImageData{}

	date := time.Now().Format("01-02")

	entries, err := e.GetDataForDate(date, topic)
	if err != nil {
		return imageData, err
	}

	imageId := ""
	plugin := ""
	if len(entries) == 0 {
		fullDates, err := e.GetFullDates(topic)
		if err != nil {
			return imageData, err
		}
		if len(fullDates) > 0 {
			randomNum := utils.GetRandomNumber(len(fullDates))
			fullDateDataEntries, err := e.GetDataForFullDate(fullDates[randomNum], topic)
			if err != nil {
				return imageData, err
			}
			entry := fullDateDataEntries[utils.GetRandomNumber(len(fullDateDataEntries))]
			imageId = entry.Uuid
			plugin = entry.Plugin
			imageData.FullDate = fullDates[randomNum]
		} else {
			return imageData, errors.New("No images found")
		}
	} else {
		randomNum := utils.GetRandomNumber(len(entries))
		imageId = entries[randomNum].Uuid
		plugin = entries[randomNum].Plugin
		imageData.FullDate = entries[randomNum].FullDate
	}

	imageData.Image, err = e.GetImage(plugin, imageId)
	return imageData, nil
}
