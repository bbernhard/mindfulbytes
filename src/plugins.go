package main

import (
	"io/ioutil"
	"gopkg.in/yaml.v2"
	"path/filepath"
	"os"
	"github.com/go-cmd/cmd"
	log "github.com/sirupsen/logrus"
)

type PluginMetaData struct {
	Version string `yaml:"version"`
	Description string `yaml:"description"` 
	Command string `yaml:"command"`
	CommandArgs []string
	Directory string
}

type Notification struct {
	Recipients []string `yaml:"recipients"`
}

type PluginConfig struct {
	Args map[string]string `yaml:"args"`
	Notifications []Notification `yaml:"notifications"`
}

type Plugin struct {
	MetaData PluginMetaData
	Config PluginConfig
	Name string
}

func getArgs(pluginMetaData PluginMetaData, pluginConfig PluginConfig) []string {
	args := []string{}
	for key, value := range pluginConfig.Args	{
		args = append(args, "--" + key)
		args = append(args, value)
	}

	return args
}

func parsePluginMetaDataFile(path string) (PluginMetaData, error) {
	var t PluginMetaData

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return t, err
	}
	
	err = yaml.Unmarshal([]byte(data), &t)
	if err != nil {
		return t, err
	}

	return t, nil
}

func parsePluginConfigFile(path string) (PluginConfig, error) {
	var t PluginConfig

	data, err := ioutil.ReadFile(path)
	if err != nil {
		return t, err
	}
	
	err = yaml.Unmarshal([]byte(data), &t)
	if err != nil {
		return t, err
	}

	return t, nil
}

func execPlugin(command string, args []string, baseDir string) error {
	log.Debug("Executing command ", command, " with arguments ", args)
	
	cmdOptions := cmd.Options{
		Buffered:  false,
		Streaming: true,
	}
	
	c := cmd.NewCmdOptions(cmdOptions, command, args...)
	
	c.Dir = baseDir
	statusChannel := c.Start()

	communicationChanel := make(chan struct{})
	go func() {
		defer close(communicationChanel)
		// Done when both channels have been closed
		// https://dave.cheney.net/2013/04/30/curious-channels
		for c.Stdout != nil || c.Stderr != nil {
			select {
			case line, open := <-c.Stdout:
				if !open {
					c.Stdout = nil
					continue
				}
				log.Debug(line)
			case line, open := <-c.Stderr:
				if !open {
					c.Stderr = nil
					continue
				}
				log.Error(os.Stderr, line)
			}
		}
	}()
	status := <-statusChannel
	if status.Error != nil {
		return status.Error
	}
	<-communicationChanel
	log.Debug("Execution of command ", command, " with arguments ", args, " done")
	return nil
}

func loadPlugins(pluginDir string, configDir string) ([]Plugin, error) {
	pluginEntries := []Plugin{}
	err := filepath.Walk(pluginDir, func(path string, info os.FileInfo, err error) error {
		if filepath.Ext(path) == ".yaml" {
			log.Info("here")
			pluginMetaData, fError := parsePluginMetaDataFile(path)
			if fError != nil {
				return fError
			}
			
			pluginMetaDataDir, err := filepath.Abs(filepath.Dir(path) + "/")
			if err != nil {
				return err
			}
			pluginMetaData.Directory = pluginMetaDataDir

			pluginDir := filepath.Dir(path)
			pluginName := filepath.Base(pluginDir)
			
			configPath := configDir + pluginName + "/config.yaml"
			pluginConfig, fError := parsePluginConfigFile(configPath)
			if fError != nil {
				return fError
			}
			pluginMetaData.CommandArgs = getArgs(pluginMetaData, pluginConfig)

			pluginEntries = append(pluginEntries, Plugin{Config: pluginConfig, MetaData: pluginMetaData, Name: pluginName})
		}
		return nil
	})

	return pluginEntries, err
}

