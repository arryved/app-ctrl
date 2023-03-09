package api

import (
	"fmt"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/arryved/app-ctrl/daemon/config"
	"github.com/arryved/app-ctrl/daemon/model"
)

type Api struct {
	cfg           *config.Config
	StatusChannel chan map[string]model.Status
}

func (a *Api) Start() error {
	cfg := a.cfg

	mux := http.NewServeMux()

	log.Info("making status cache")
	cache := make(map[string]model.Status)

	mux.HandleFunc("/status", ConfiguredHandlerStatus(cfg, a.StatusChannel, cache))

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      mux,
		ReadTimeout:  time.Duration(cfg.ReadTimeoutS) * time.Second,
		WriteTimeout: time.Duration(cfg.WriteTimeoutS) * time.Second,
	}

	log.Infof("Starting HTTPS listener on port=%d", cfg.Port)
	err := srv.ListenAndServeTLS(
		cfg.CrtPath,
		cfg.KeyPath,
	)
	if err != nil {
		log.Errorf("Error seen when starting listener: %v", err)
	}

	log.Info("Finishing up")
	return err
}

func New(cfg *config.Config, ch chan map[string]model.Status) *Api {
	api := &Api{
		cfg:           cfg,
		StatusChannel: ch,
	}
	return api
}
