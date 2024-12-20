package config

import (
	"io/ioutil"

	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type Config struct {
	// Port, Scheme for app-controld API
	AppControlDPort    int    `yaml:"appControlDPort"`
	AppControlDScheme  string `yaml:"appControlDScheme"`
	AppControlDPSKPath string `yaml:"appControlDPSKPath"`

	// Port for HTTPS API listener
	Port int `yaml:"port"`

	// HTTPS Timeouts
	ReadTimeoutS  int `yaml:"readTimeoutS"`
	WriteTimeoutS int `yaml:"writeTimeoutS"`

	// kubeconfig yaml path
	KubeConfigPath string `yaml:"kubeConfigPath"`

	// TLS material locations
	KeyPath string `yaml:"keyPath"`
	CrtPath string `yaml:"crtPath"`

	// Min log level
	LogLevel string `yaml:"logLevel"`

	// TLS Settings
	TLS *TLSConfig `yaml:"tls"`

	// Service Account Key file
	ServiceAccountKeyPath string `yaml:"serviceAccountKeyPath"`

	// Layout of the app clusters TODO - (statically configured for now, add discovery later)
	Topology Topology `yaml:"topology"`

	// Config for work queue client
	Queue QueueConfig `yaml:"queue"`

	// RBAC
	AuthnEnabled    bool                        `yaml:"authnEnabled"`
	RBACEnabled     bool                        `yaml:"rbacEnabled"`
	RoleMemberships map[Role][]GroupUrn         `yaml:"roleMemberships"`
	AccessEntries   []AccessEntry               `yaml:"accessEntries"`
	UsersByGroups   map[GroupUrn][]PrincipalUrn `yaml:"usersByGroups"`

	// ServiceAccounts for Secret access
	SecretsServiceAccounts []string `yaml:"secretsServiceAccounts"`
}

type GroupUrn string
type ResourceUrn string
type PrincipalUrn string

type AccessEntry struct {
	Role       Role        `yaml:"role"`
	Permission Permission  `yaml:"permission"`
	Target     ResourceUrn `yaml:"target"`
}

type Role string

const (
	Operator  Role = "operator"
	Developer Role = "developer"
	Manager   Role = "manager"
)

type Permission string

const (
	Deploy        Permission = "deploy"
	Restart       Permission = "restart"
	SecretsList   Permission = "secretsList"
	SecretsRead   Permission = "secretsRead"
	SecretsCreate Permission = "secretsCreate"
	SecretsUpdate Permission = "secretsUpdate"
	SecretsDelete Permission = "secretsDelete"
)

type RoleMemberships map[Role][]string

// map of environment to clusters
type Topology map[string]Environment

type Environment struct {
	Clusters []Cluster `yaml:"clusters"`
}

type Cluster struct {
	Id      ClusterId       `yaml:"id"`
	Hosts   map[string]Host `yaml:"hosts"`
	Kind    string          `yaml:"kind"`
	Repo    string          `yaml:"repo"`
	Runtime string          `yaml:"runtime"`
}

// uniquely identifies them, enforce this constraint as needed (using as a map key, for instance)
type ClusterId struct {
	App     string `json:"app"     yaml:"app"`
	Region  string `json:"region"  yaml:"region"`
	Variant string `json:"variant" yaml:"variant"`
}

type Host struct {
	Canary bool `yaml:"canary"`
}

type TLSConfig struct {
	// list of allowed ciphers
	Ciphers []string

	// minimum TLS version to use
	MinVersion string
}

type QueueConfig struct {
	Project      string
	Topic        string
	Subscription string
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
	if c.AppControlDPSKPath == "" {
		c.AppControlDPSKPath = "./var/app-controld-psk"
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
	if c.ServiceAccountKeyPath == "" {
		c.ServiceAccountKeyPath = "/usr/local/etc/app-control-api-svc-acct-key.json"
	}
	if c.TLS == nil {
		c.TLS = &TLSConfig{
			Ciphers: []string{
				"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
				"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
			},
			MinVersion: "1.2",
		}
	}
	for envName, env := range c.Topology {
		for j, cluster := range env.Clusters {
			if cluster.Id.Variant == "" {
				cluster.Id.Variant = "default"
				c.Topology[envName].Clusters[j].Id.Variant = "default"
			}
		}
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

// TODO provide a validate() method to enforce values, uniqueness constraints, etc.
