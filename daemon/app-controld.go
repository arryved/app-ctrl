package main

import (
	"flag"

	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"

	"github.com/arryved/app-ctrl/daemon/api"
	"github.com/arryved/app-ctrl/daemon/config"
	"github.com/arryved/app-ctrl/daemon/model"
)

const (
	config_path_default = "/usr/local/etc/app-controld.yml"
)

func main() {
	configPath := flag.String("config", config_path_default, "path to config file")
	flag.Parse()

	log.Infof("using configPath=%s", *configPath)
	cfg := config.Load(*configPath)

	// set log level
	level, err := logrus.ParseLevel(cfg.LogLevel)
	if err != nil {
		log.Warnf("Could not parse log level %s, %v, defaulting to InfoLevel", cfg.LogLevel, err)
		log.SetLevel(log.InfoLevel)
	} else {
		log.SetLevel(level)
	}

	// TODO - ship logs to fluentd/log aggregation
	// TODO - collect and expose metrics

	// start background status runner
	statusCh := make(chan map[string]model.Status, 1)
	go api.StatusRunner(cfg, statusCh)

	// start app-controld API
	api := api.New(cfg, statusCh)
	api.Start()
}
