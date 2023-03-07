package main

import (
	"flag"

	log "github.com/sirupsen/logrus"

	"github.com/arryved/app-ctrl/daemon/api"
	"github.com/arryved/app-ctrl/daemon/config"
)

const (
	config_path_default = "/usr/local/etc/app-controld.yml"
)

func main() {
	configPath := flag.String("config", config_path_default, "path to config file")
	flag.Parse()

	log.Infof("using configPath=%s", *configPath)
	cfg := config.Load(*configPath)

	api := api.New(cfg)
	api.Start()
}
