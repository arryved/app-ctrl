package config

import (
	"io/ioutil"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type Config struct {
	// Port, Scheme for app-controld API
	AppControlDPort   int    `yaml:"appControlDPort"`
	AppControlDScheme string `yaml:"appControlDScheme"`

	// Port for HTTPS API listener
	Port int `yaml:"port"`

	// HTTPS Timeouts
	ReadTimeoutS  int `yaml:"readTimeoutS"`
	WriteTimeoutS int `yaml:"writeTimeoutS"`

	// TLS material locations
	KeyPath string `yaml:"keyPath"`
	CrtPath string `yaml:"crtPath"`

	// Min log level
	LogLevel string `yaml:"logLevel"`

	// Layout of the app clusters TODO - (statically configured for now, add discovery later)
	Topology Topology `yaml:"topology"`
}

// map of environment to clusters
type Topology map[string]Environment

type Environment struct {
	Clusters map[string]Cluster `yaml:"clusters"`
}

type Cluster struct {
	Hosts map[string]Host `yaml:"hosts"`
}

type Host struct {
	Canary bool `yaml:"canary"`
}

// Load the config from provided path
func Load(configPath string) *Config {
	config := Config{}

	config.loadFile(configPath)
	config.setDefaults()

	return &config
}

func (c *Config) setDefaults() {
	if c.AppControlDPort == 0 {
		c.AppControlDPort = 1024
	}
	if c.AppControlDScheme == "" {
		c.AppControlDScheme = "https"
	}
	if c.Port == 0 {
		c.Port = 1026
	}
	if c.KeyPath == "" {
		c.KeyPath = "./var/service.key"
	}
	if c.CrtPath == "" {
		c.CrtPath = "./var/service.crt"
	}
	if c.ReadTimeoutS == 0 {
		c.ReadTimeoutS = 10
	}
	if c.WriteTimeoutS == 0 {
		c.WriteTimeoutS = 10
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}

	log.Infof("Applied defaults to all unset fields")
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
}
