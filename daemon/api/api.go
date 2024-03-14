package api

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

	common "github.com/arryved/app-ctrl/api/api"
	"github.com/arryved/app-ctrl/daemon/config"
	"github.com/arryved/app-ctrl/daemon/model"
)

type Api struct {
	cfg         *config.Config
	StatusCache *model.StatusCache
}

func (a *Api) Start() error {
	cfg := a.cfg

	mux := http.NewServeMux()
	mux.HandleFunc("/status", NewConfiguredHandlerStatus(cfg, a.StatusCache))
	mux.HandleFunc("/healthz", NewConfiguredHandlerHealthz(cfg, a.StatusCache))

	tlsConfig := &tls.Config{
		CipherSuites:             common.CipherSuitesFromConfig(cfg.TLS.Ciphers),
		MinVersion:               common.TLSVersionFromConfig(cfg.TLS.MinVersion),
		PreferServerCipherSuites: true,
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Port),
		Handler:      mux,
		ReadTimeout:  time.Duration(cfg.ReadTimeoutS) * time.Second,
		TLSConfig:    tlsConfig,
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

func New(cfg *config.Config, cache *model.StatusCache) *Api {
	api := &Api{
		cfg:         cfg,
		StatusCache: cache,
	}
	return api
}
