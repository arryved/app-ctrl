package main

import (
	"flag"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/arryved/app-ctrl/api/api"
	"github.com/arryved/app-ctrl/api/config"
)

const (
	config_path_default = "/usr/local/etc/app-control-api.yml"
)

func main() {
	configPath := flag.String("config", config_path_default, "path to config file")
	flag.Parse()

	log.Infof("using configPath=%s", *configPath)
	cfg := config.Load(*configPath)
	os.Setenv("GOOGLE_APPLICATION_CREDENTIALS", cfg.ServiceAccountKeyPath)

	// set log level
	level, err := log.ParseLevel(cfg.LogLevel)
	if err != nil {
		log.Warnf("Could not parse log level %s, %v, defaulting to InfoLevel", cfg.LogLevel, err)
		log.SetLevel(log.InfoLevel)
	} else {
		log.SetLevel(level)
	}

	// TODO - ship logs to fluentd/log aggregation
	// TODO - collect and expose metrics

	// start app-control-api listener
	api := api.New(cfg)
	api.Start()
}
