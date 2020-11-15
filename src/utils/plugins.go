package utils

import (
	"io/ioutil"
	"gopkg.in/yaml.v2"
	"path/filepath"
	"os"
	"github.com/go-cmd/cmd"
	log "github.com/sirupsen/logrus"
	"errors"
)

type Arg struct {
	Type string `yaml:"type"`
	Format string `yaml:"format"`
}

type PluginMetaData struct {
	Version string `yaml:"version"`
	Name string `yaml:"name"`
	Description string `yaml:"description"` 
	Command string `yaml:"command"`
	CrawlArgs map[string]Arg `yaml:"crawl-args"`
	FetchArgs map[string]Arg `yaml:"fetch-args"`
	Topics []string `yaml:"topics"`
}

type Notification struct {
	Recipients []string `yaml:"recipients"`
}

type PluginConfig struct {
	Args map[string]string `yaml:"args"`
	Notifications []Notification `yaml:"notifications"`
}

type CrawlExec struct {
	Command string
	CommandArgs []string
	BaseDir string
}

type FetchExec struct {
	Command string
	BaseDir string
	StaticArgs []string
	DynamicArgsPrefix string
}

type Exec struct {
	FetchExec FetchExec
	CrawlExec CrawlExec
}

type Plugin struct {
	MetaData PluginMetaData
	Config PluginConfig
	Exec Exec
	Name string
}

func buildDynamicFetchArgs(id string, destination string, argPrefix string) []string {
	return []string{"fetch", argPrefix+"id", id, argPrefix+"destination", destination}
}

//it's not necessary for a plugin to specify the fetch-args in the meta.yml file, in case 
//it only needs the default arguments (i.e 'id' and 'destination'). But as those arguments
//are not specified in the meta.yml file we also do not know if it's a short argument ('-')
//or a long one ('--'). So we look at the other arguments to use the format that's used there.
//this assumes, that the plugin writer uses a consistent argument style. In case we haven't 
//found any arguments we default to '-'
func getFetchArgPrefix(pluginMetaData PluginMetaData) string {
	for _, arg := range pluginMetaData.FetchArgs {
		if arg.Format == "long" {
			return "--"
		}
		return "-"
	}

	for _, arg := range pluginMetaData.CrawlArgs {
		if arg.Format == "long" {
			return "--"
		}
		return "-"
	}

	return "-"
}

func getFetchArgs(pluginMetaData PluginMetaData, pluginConfig PluginConfig) ([]string, error) {
	args := []string{}
	for key, value := range pluginConfig.Args {
		log.Info("key = ", key)
		argDetails, ok := pluginMetaData.FetchArgs[key]
		if ok {
			prefix := ""
			if argDetails.Format == "short" {
				prefix = "-"
			} else if argDetails.Format == "long" {
				prefix = "--"
			} else {
				return args, errors.New("Invalid format specified for parameter '" + key + "' in plugin " + pluginMetaData.Description)
			}

			args = append(args, prefix + key)
			args = append(args, value)
		}
	}

	return args, nil
}

func getCrawlArgs(pluginMetaData PluginMetaData, pluginConfig PluginConfig) ([]string, error) {
	args := []string{"crawl"}
	for key, value := range pluginConfig.Args {
		argDetails, ok := pluginMetaData.CrawlArgs[key]
		if !ok {
			return args, errors.New("No format specified for parameter '" + key + "' in plugin " + pluginMetaData.Description)
		}

		prefix := ""
		if argDetails.Format == "short" {
			prefix = "-"
		} else if argDetails.Format == "long" {
			prefix = "--"
		} else {
			return args, errors.New("Invalid format specified for parameter '" + key + "' in plugin " + pluginMetaData.Description)
		}

		args = append(args, prefix + key)
		args = append(args, value)
	}

	return args, nil
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

func execCrawl(crawlExec CrawlExec) error {
	return execPlugin(crawlExec.Command, crawlExec.CommandArgs, crawlExec.BaseDir)
}

func execFetch(id string, destination string, fetchExec FetchExec) error {
	allArgs := buildDynamicFetchArgs(id, destination, fetchExec.DynamicArgsPrefix)
	allArgs = append(allArgs, fetchExec.StaticArgs...)
	log.Info("all args = ", allArgs)
	return execPlugin(fetchExec.Command, allArgs, fetchExec.BaseDir)
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
			pluginMetaData, fError := parsePluginMetaDataFile(path)
			if fError != nil {
				return fError
			}
			
			pluginMetaDataDir, err := filepath.Abs(filepath.Dir(path) + "/")
			if err != nil {
				return err
			}

			pluginDir := filepath.Dir(path)
			pluginName := filepath.Base(pluginDir)
			
			configPath := configDir + pluginName + "/config.yaml"
			
			pluginConfig, fError := parsePluginConfigFile(configPath)
			if fError != nil {
				return fError
			}

			exec := Exec{}
			exec.CrawlExec.CommandArgs, err = getCrawlArgs(pluginMetaData, pluginConfig)
			if err != nil {
				return err
			}
			exec.CrawlExec.BaseDir = pluginMetaDataDir
			exec.CrawlExec.Command = pluginMetaData.Command
			
			exec.FetchExec.BaseDir = pluginMetaDataDir
			exec.FetchExec.Command = pluginMetaData.Command
			exec.FetchExec.StaticArgs, err = getFetchArgs(pluginMetaData, pluginConfig)
			if err != nil {
				return err
			}

			exec.FetchExec.DynamicArgsPrefix = getFetchArgPrefix(pluginMetaData)

			pluginEntries = append(pluginEntries, Plugin{Config: pluginConfig, MetaData: pluginMetaData, Name: pluginName, Exec: exec})
		}
		return nil
	})

	return pluginEntries, err
}

type Plugins struct {
	pluginDir string
	configDir string
	plugins []Plugin
}

func NewPlugins(pluginDir string, configDir string) *Plugins {
	return &Plugins{
		pluginDir: pluginDir,
		configDir: configDir,
	}
}

func (p *Plugins) Load() error {
	var err error
	p.plugins, err = loadPlugins(p.pluginDir, p.configDir)
	return err
}

func (p *Plugins) GetPlugins() []Plugin {
	return p.plugins
}

func (p *Plugins) GetPlugin(name string) (Plugin, error) {
	for _, plugin := range p.plugins {
		if plugin.Name == name {
			return plugin, nil
		}
	}

	return Plugin{}, errors.New("No plugin with that name found")
}

func (p *Plugins) GetTopics() map[string][]string {
	topics := make(map[string][]string)
	for _, plugin := range p.plugins {
		for _, topic := range plugin.MetaData.Topics {
			existingTopics, ok := topics[topic]
			if ok {
				if !StringInSlice(plugin.MetaData.Name, existingTopics) {
					topics[topic] = append(topics[topic], plugin.MetaData.Name)
				}
			} else {
				topics[topic] = []string{plugin.MetaData.Name}
			}
		}
	}
	return topics
}

func (p *Plugins) ExecFetch(id string, destination string, fetchExec FetchExec) error {
	return execFetch(id, destination, fetchExec)
}

func (p *Plugins) ExecCrawl(crawlExec CrawlExec) error {
	return execCrawl(crawlExec)
}
