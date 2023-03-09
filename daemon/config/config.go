package config

import (
	"io/ioutil"
	"strings"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type Config struct {
	// Port for HTTPS API listener
	Port int `yaml:"port"`

	// HTTPS Timeouts
	ReadTimeoutS  int `yaml:"readTimeoutS"`
	WriteTimeoutS int `yaml:"writeTimeoutS"`

	// TLS material locations
	KeyPath string `yaml:"keyPath"`
	CrtPath string `yaml:"crtPath"`

	// APT binary path
	AptPath string `yaml:"aptPath"`

	// Known apps
	AppDefs map[string]AppDef `yaml:"appDefs"`

	// Min log level
	LogLevel string `yaml:"logLevel"`

	// Status polling pause interval
	PollIntervalSec int `yaml:"pollIntervalSec"`
}

type AppDef struct {
	// Type of app (see enum below)
	Type AppType `yaml:"type"`

	// Healthz checks for app
	Healthz []Healthz `yaml:"healthz"`

	// Varz checks for app; used to get running version
	Varz *Varz `yaml:"varz"`
}

type Healthz struct {
	// Port number to check
	Port int

	// Whether or not to negotiate SSL/TLS
	TLS bool
}

type Varz struct {
	// Port number to check
	Port int

	// Whether or not to negotiate SSL/TLS
	TLS bool
}

// AppType Enum
type AppType int

const (
	Unknown AppType = iota

	// An app that runs as an OS daemon
	Daemon

	// An app that starts from a script an serves traffic
	Online

	// A runtime executable or library
	Runtime
)

// String representations of AppType enum values
func (t AppType) String() string {
	switch t {
	case Daemon:
		return "DAEMON"
	case Online:
		return "ONLINE"
	case Runtime:
		return "RUNTIME"
	}

	return "UNKNOWN"
}

// Custom yaml deserializer
func (t *AppType) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var token string

	err := unmarshal(&token)
	if err != nil {
		log.Errorf("Could not unmarshal AppType=%v", token)
		return err
	}

	token = strings.TrimSpace(token)
	switch token {
	case "DAEMON":
		*t = Daemon
		return nil
	case "ONLINE":
		*t = Online
		return nil
	case "RUNTIME":
		*t = Runtime
		return nil
	}

	*t = Unknown
	return nil
}

// Load the config from provided path
func Load(configPath string) *Config {
	config := Config{}

	config.loadFile(configPath)
	config.setDefaults()

	return &config
}

func (c *Config) setDefaults() {
	if c.Port == 0 {
		c.Port = 1024
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
	if c.AptPath == "" {
		c.AptPath = "/usr/bin/apt"
	}
	if c.AppDefs == nil {
		c.AppDefs = make(map[string]AppDef)
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	if c.PollIntervalSec == 0 {
		c.PollIntervalSec = 5
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
