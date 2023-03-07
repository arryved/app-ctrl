package api

import (
	"fmt"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/arryved/app-ctrl/daemon/config"
)

type Api struct {
	cfg *config.Config
}

func (a *Api) Start() error {
	cfg := a.cfg

	mux := http.NewServeMux()
	registerHandlers(mux, cfg)

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
	return err
}

func New(cfg *config.Config) *Api {
	api := &Api{
		cfg: cfg,
	}
	return api
}
