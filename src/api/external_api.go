package api

import (
	"github.com/go-resty/resty/v2"
	"strconv"
	"errors"
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
