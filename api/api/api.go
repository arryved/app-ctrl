package api

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/arryved/app-ctrl/api/config"
)

type Api struct {
	cfg *config.Config
}

func (a *Api) Start() error {
	cfg := a.cfg
	mux := http.NewServeMux()
	mux.HandleFunc("/status/", ConfiguredHandlerStatus(cfg))

	tlsConfig := &tls.Config{
		CipherSuites:             CipherSuitesFromConfig(cfg.TLS.Ciphers),
		MinVersion:               TLSVersionFromConfig(cfg.TLS.MinVersion),
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

func New(cfg *config.Config) *Api {
	api := &Api{cfg: cfg}
	return api
}

func CipherSuitesFromConfig(configuredCiphers []string) []uint16 {
	result := []uint16{}
	for _, cipherSuite := range tls.CipherSuites() {
		if contains(configuredCiphers, cipherSuite.Name) {
			result = append(result, cipherSuite.ID)
		}
	}
	return result
}

func TLSVersionFromConfig(versionString string) uint16 {
	version := tls.VersionTLS12

	switch versionString {
	case "1.0":
		return tls.VersionTLS10
	case "1.1":
		return tls.VersionTLS11
	case "1.2":
		return tls.VersionTLS12
	case "1.3":
		return tls.VersionTLS13
	}

	return uint16(version)
}

func contains(list []string, match string) bool {
	for _, element := range list {
		if element == match {
			return true
		}
	}
	return false
}
