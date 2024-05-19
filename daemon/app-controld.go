package main

import (
	"flag"

	log "github.com/sirupsen/logrus"

	"github.com/arryved/app-ctrl/daemon/api"
	"github.com/arryved/app-ctrl/daemon/cli"
	"github.com/arryved/app-ctrl/daemon/config"
	"github.com/arryved/app-ctrl/daemon/model"
	"github.com/arryved/app-ctrl/daemon/runners"
)

const (
	config_path_default = "/usr/local/etc/app-controld.yml"
)

func main() {
	configPath := flag.String("config", config_path_default, "path to config file")
	flag.Parse()

	log.Infof("Using configPath=%s", *configPath)
	cfg := config.Load(*configPath)

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

	// thread-safe status map
	statusCache := model.NewStatusCache()

	// thread-safe deploy map
	deployCache := model.NewDeployCache()

	// CLI executor
	executor := &cli.Executor{}

	// start background runners
	go runners.StatusRunner(cfg, statusCache)
	go runners.DeployRunner(cfg, deployCache, executor)

	// start app-controld API
	api := api.New(cfg, statusCache, deployCache)
	api.Start()
}
