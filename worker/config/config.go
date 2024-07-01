package config

import (
	apiconfig "github.com/arryved/app-ctrl/api/config"
	"io/ioutil"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type Config struct {
	// Port, Scheme for app-controld API
	AppControlDPort    int    `yaml:"appControlDPort"`
	AppControlDScheme  string `yaml:"appControlDScheme"`
	AppControlDPSKPath string `yaml:"appControlDPSKPath"`

	// App template map for kube
	AppTemplates map[string]string `yaml:"appTemplates"`

	// Arryved env short name
	Env string `yaml:"env"`

	// kubeconfig yaml path
	KubeConfigPath string `yaml:"kubeConfigPath"`

	// Min log level
	LogLevel string `yaml:"logLevel"`

	// Max number of concurrent job threads
	MaxJobThreads int `yaml:"maxJobThreads"`

	// Config for work queue client
	Queue apiconfig.QueueConfig `yaml:"queue"`

	// Google Service Account Key Path
	ServiceAccountKeyPath string `yaml:"serviceAccountKeyPath"`
}

func (c *Config) setDefaults() {
	if c.AppControlDPort == 0 {
		c.AppControlDPort = 1024
	}
	if c.AppControlDScheme == "" {
		c.AppControlDScheme = "https"
	}
	if c.AppControlDPSKPath == "" {
		c.AppControlDPSKPath = "./var/app-controld-psk"
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	if c.KubeConfigPath == "" {
		c.KubeConfigPath = "/usr/local/etc/app-control-api-kubeconfig.yml"
	}
	if c.MaxJobThreads == 0 {
		c.MaxJobThreads = 8
	}
	if c.ServiceAccountKeyPath == "" {
		c.ServiceAccountKeyPath = "/usr/local/etc/app-control-api-svc-acct-key.json"
	}
	log.Debugf("config %v", c)
}

// load and merge settings from file if it exists
func (c *Config) loadFile(configPath string) {
	file, err := ioutil.ReadFile(configPath)
	if err != nil {
		log.Warnf("Could not load config file at path='%s', err=%v", configPath, err)
		return
	}

	err = yaml.Unmarshal(file, c)
	if err != nil {
		log.Warnf("Could not load config file at path='%s', err=%v", configPath, err)
		return
	}

	log.Infof("Loaded config yaml from path='%s'", configPath)
	// TODO run validate()
}

// Load the config from provided path
func Load(configPath string) *Config {
	config := Config{}

	config.loadFile(configPath)
	config.setDefaults()

	return &config
}
