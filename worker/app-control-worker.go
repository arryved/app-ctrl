package main

import (
	"flag"
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/arryved/app-ctrl/api/queue"
	"github.com/arryved/app-ctrl/worker/config"
	"github.com/arryved/app-ctrl/worker/worker"
)

const (
	config_path_default = "/usr/local/etc/app-control-worker.yml"
)

func main() {
	configPath := flag.String("config", config_path_default, "path to config file")
	flag.Parse()

	log.Infof("using configPath=%s", *configPath)
	cfg := config.Load(*configPath)

	// set log level
	level, err := log.ParseLevel(cfg.LogLevel)
	if err != nil {
		log.Warnf("Could not parse log level %s, %v, defaulting to InfoLevel", cfg.LogLevel, err)
		log.SetLevel(log.InfoLevel)
	} else {
		log.SetLevel(level)
	}

	client, err := queue.NewClient(cfg.Queue)
	if err != nil {
		msg := fmt.Sprintf("Could not get queue client, err=%s", err.Error())
		log.Error(msg)
		panic(msg)
	}
	jobQueue := queue.NewQueue(cfg.Queue, client)

	// TODO - ship logs to fluentd/log aggregation
	// TODO - collect and expose metrics

	// start app-control-worker thread(s)
	worker := worker.New(cfg, jobQueue)
	worker.Start()
}
