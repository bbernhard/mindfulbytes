package config

import (
	"io/ioutil"
	"gopkg.in/yaml.v2"
)

type Notification struct {
	Recipients []string `yaml:"recipients"`
	Message string `yaml:"message"`
	Enabled bool `yaml:"enabled"`
	Topics []string `yaml:"topics"`
	Interval string `yaml:"interval"`
	Settings map[string]string `yaml:"settings"`
}

type Config struct {
	Notifications map[string]Notification `yaml:"notifications"`
}

func ParseConfig(pathToConfigFile string) (Config, error) {
	var c Config
	
	data, err := ioutil.ReadFile(pathToConfigFile)
	if err != nil {
		return c, err
	}
	
	err = yaml.Unmarshal([]byte(data), &c)
	if err != nil {
		return c, err
	}

	return c, nil
}
